package babigame

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const dessertEvidenceManifestSHA256 = "03ecd1577321665b67736c291a2610c87d782e21a7f88f0f840cd8e420a125de"

//go:embed testdata/dessert_evidence_manifest.json
var dessertEvidenceManifestJSON []byte

//go:embed testdata/dessert_open_box_evidence.json
var dessertOpenBoxEvidenceFixtureJSON []byte

//go:embed testdata/dessert_lifecycle_evidence.json
var dessertLifecycleEvidenceFixtureJSON []byte

// A successful act.recvBoxes fixture has not been observed yet. Keeping the
// corpus variable explicit makes the progress-box gate impossible to open by
// changing only a manifest boolean.
var dessertRecvBoxesEvidenceFixtureJSON []byte

// DessertCaptureEvidence is the reviewed, sanitized protocol-evidence summary
// embedded in the daemon. It deliberately contains no account, batch, capture
// path, or wall-clock metadata.
type DessertCaptureEvidence struct {
	SchemaVersion int                         `json:"schema_version"`
	Activity      string                      `json:"activity"`
	Replay        DessertReplayEvidence       `json:"replay"`
	RewardBoxes   DessertRewardBoxEvidence    `json:"reward_boxes"`
	LiveAutoplay  DessertLiveAutoplayEvidence `json:"live_autoplay"`
}

type DessertReplayEvidence struct {
	Verified                  bool   `json:"verified"`
	RPC                       string `json:"rpc"`
	Mode                      int32  `json:"mode"`
	RequestCount              int    `json:"request_count"`
	ResponseCount             int    `json:"response_count"`
	DropCount                 int    `json:"drop_count"`
	MergeCount                int    `json:"merge_count"`
	FixtureSHA256             string `json:"fixture_sha256"`
	ResponseOrderVerified     bool   `json:"response_order_verified"`
	TopologyVerified          bool   `json:"topology_verified"`
	CheckpointRebuildVerified bool   `json:"checkpoint_rebuild_verified"`
	CausalSequenceVerified    bool   `json:"causal_sequence_verified"`
	TrajectoryVerified        bool   `json:"trajectory_verified"`
}

type DessertRewardBoxEvidence struct {
	RecvBoxesSuccess       bool   `json:"recv_boxes_success"`
	RecvBoxesFixtureSHA256 string `json:"recv_boxes_fixture_sha256,omitempty"`
	OpenBoxSuccess         bool   `json:"open_box_success"`
	OpenBoxRequestNum      int32  `json:"open_box_request_num"`
	OpenBoxFixtureSHA256   string `json:"open_box_fixture_sha256"`
}

type DessertLiveAutoplayEvidence struct {
	GameStartFromIdleSuccess bool   `json:"game_start_from_idle_success"`
	PhaseThreeSuccess        bool   `json:"phase_three_success"`
	NaturalEndSuccess        bool   `json:"natural_end_success"`
	TerminalGameOverSuccess  bool   `json:"terminal_game_over_success"`
	LifecycleFixtureSHA256   string `json:"lifecycle_fixture_sha256"`
}

type dessertOpenBoxEvidenceFixture struct {
	Schema  string `json:"schema"`
	Request struct {
		Num int32 `json:"num"`
	} `json:"request"`
	Before struct {
		RewardBoxes int32 `json:"reward_boxes"`
		TotalPoints int32 `json:"total_points"`
	} `json:"before"`
	After struct {
		RewardBoxes int32 `json:"reward_boxes"`
		TotalPoints int32 `json:"total_points"`
	} `json:"after"`
	Reward struct {
		ItemID int32 `json:"item_id"`
		Count  int32 `json:"count"`
	} `json:"reward"`
	ModeUnchanged bool `json:"mode_unchanged"`
}

type dessertLifecycleEvidenceFixture struct {
	Schema    string `json:"schema"`
	Mode      int32  `json:"mode"`
	GameStart struct {
		Before dessertLifecycleState `json:"before"`
		After  dessertLifecycleState `json:"after"`
	} `json:"game_start"`
	NaturalCheckpoint struct {
		OperationType int32   `json:"operation_type"`
		MergeLevel    int32   `json:"merge_level"`
		Step          int32   `json:"step"`
		Running       bool    `json:"running"`
		GameStatus    int32   `json:"game_status"`
		ObjectCount   int32   `json:"object_count"`
		MaxLineHoldMS float64 `json:"max_line_hold_ms"`
	} `json:"natural_checkpoint"`
	GameOver struct {
		Before           dessertLifecycleGameOverState `json:"before"`
		After            dessertLifecycleGameOverState `json:"after"`
		EconomyPreserved bool                          `json:"economy_preserved"`
	} `json:"game_over"`
}

type dessertLifecycleState struct {
	Step         int32 `json:"step"`
	Running      bool  `json:"running"`
	CurrentLevel int32 `json:"current_level"`
	ObjectCount  int32 `json:"object_count"`
}

type dessertLifecycleGameOverState struct {
	Step          int32 `json:"step"`
	Running       bool  `json:"running"`
	Score         int32 `json:"score"`
	CurrencyGain  int32 `json:"currency_gain"`
	RewardBoxGain int32 `json:"reward_box_gain"`
}

// ReadDessertCaptureEvidence verifies the embedded manifest byte-for-byte and
// then strictly decodes its schema. Any missing, unknown, or modified manifest
// is rejected rather than weakening an execution gate.
func ReadDessertCaptureEvidence() (DessertCaptureEvidence, error) {
	return readDessertCaptureEvidence(dessertEvidenceManifestJSON, dessertEvidenceManifestSHA256)
}

// DessertOpenRewardBoxEvidenceGate reports whether the independently observed
// single-box actDessert.openBox path has reviewed, embedded evidence.
func DessertOpenRewardBoxEvidenceGate() bool {
	evidence, err := ReadDessertCaptureEvidence()
	return err == nil && evidence.RewardBoxes.OpenBoxSuccess && evidence.RewardBoxes.OpenBoxRequestNum == 1 &&
		dessertEmbeddedEvidenceMatches(dessertOpenBoxEvidenceFixtureJSON, evidence.RewardBoxes.OpenBoxFixtureSHA256) &&
		validDessertOpenBoxEvidenceFixture(dessertOpenBoxEvidenceFixtureJSON)
}

// DessertProgressBoxEvidenceGate remains closed until a successful
// act.recvBoxes response fixture is embedded. Client code alone is not
// sufficient evidence for its receipt delta.
func DessertProgressBoxEvidenceGate() bool {
	evidence, err := ReadDessertCaptureEvidence()
	return err == nil && evidence.RewardBoxes.RecvBoxesSuccess &&
		dessertEmbeddedEvidenceMatches(dessertRecvBoxesEvidenceFixtureJSON, evidence.RewardBoxes.RecvBoxesFixtureSHA256)
}

// DessertRewardBoxEvidenceGate is retained for callers that require both
// independent reward paths. New planners should use the action-specific gate.
func DessertRewardBoxEvidenceGate() bool {
	return DessertOpenRewardBoxEvidenceGate() && DessertProgressBoxEvidenceGate()
}

// DessertLiveAutoplayEvidenceGate reports whether the capture corpus proves a
// causal physics trajectory and every live lifecycle prerequisite. Topology
// and deterministic checkpoint reconstruction alone can never enable live
// autoplay.
func DessertLiveAutoplayEvidenceGate() bool {
	evidence, err := ReadDessertCaptureEvidence()
	return err == nil && evidence.Replay.Verified &&
		evidence.Replay.ResponseOrderVerified &&
		evidence.Replay.TopologyVerified &&
		evidence.Replay.CheckpointRebuildVerified &&
		evidence.Replay.CausalSequenceVerified &&
		evidence.Replay.TrajectoryVerified &&
		evidence.LiveAutoplay.GameStartFromIdleSuccess &&
		evidence.LiveAutoplay.PhaseThreeSuccess &&
		evidence.LiveAutoplay.NaturalEndSuccess &&
		evidence.LiveAutoplay.TerminalGameOverSuccess &&
		dessertEmbeddedEvidenceMatches(dessertLifecycleEvidenceFixtureJSON, evidence.LiveAutoplay.LifecycleFixtureSHA256) &&
		validDessertLifecycleEvidenceFixture(dessertLifecycleEvidenceFixtureJSON)
}

func validDessertOpenBoxEvidenceFixture(raw []byte) bool {
	var fixture dessertOpenBoxEvidenceFixture
	if !strictDessertEvidenceDecode(raw, &fixture) {
		return false
	}
	return fixture.Schema == "dessert-open-box-v1" && fixture.Request.Num == 1 &&
		fixture.Before.RewardBoxes > 0 && fixture.After.RewardBoxes == fixture.Before.RewardBoxes-1 &&
		fixture.Reward.ItemID == 1344 && fixture.Reward.Count > 0 &&
		fixture.After.TotalPoints == fixture.Before.TotalPoints+fixture.Reward.Count && fixture.ModeUnchanged
}

func validDessertLifecycleEvidenceFixture(raw []byte) bool {
	var fixture dessertLifecycleEvidenceFixture
	if !strictDessertEvidenceDecode(raw, &fixture) {
		return false
	}
	start := fixture.GameStart
	checkpoint := fixture.NaturalCheckpoint
	gameOver := fixture.GameOver
	return fixture.Schema == "dessert-mode1-lifecycle-v1" && fixture.Mode == 1 &&
		start.Before.Step == 0 && !start.Before.Running && start.Before.CurrentLevel == 0 && start.Before.ObjectCount == 0 &&
		start.After.Step == 0 && start.After.Running && start.After.CurrentLevel > 0 && start.After.ObjectCount == 0 &&
		checkpoint.OperationType == 0 && checkpoint.MergeLevel == 0 && checkpoint.Step > 0 && checkpoint.Running &&
		checkpoint.GameStatus == 0 && checkpoint.ObjectCount > 0 && checkpoint.MaxLineHoldMS >= 5000 &&
		gameOver.Before.Step == checkpoint.Step && gameOver.Before.Running && gameOver.Before.Score > 0 &&
		gameOver.Before.CurrencyGain >= 0 && gameOver.Before.RewardBoxGain >= 0 &&
		gameOver.After.Step == 0 && !gameOver.After.Running && gameOver.After.Score == 0 &&
		gameOver.After.CurrencyGain == gameOver.Before.CurrencyGain &&
		gameOver.After.RewardBoxGain == gameOver.Before.RewardBoxGain && gameOver.EconomyPreserved
}

func strictDessertEvidenceDecode(raw []byte, destination any) bool {
	if len(raw) == 0 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(destination) != nil {
		return false
	}
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

func dessertEmbeddedEvidenceMatches(raw []byte, expectedSHA256 string) bool {
	if len(raw) == 0 || !validDessertEvidenceSHA256(expectedSHA256) {
		return false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]) == expectedSHA256
}

func validDessertEvidenceSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func readDessertCaptureEvidence(raw []byte, expectedSHA256 string) (DessertCaptureEvidence, error) {
	if len(raw) == 0 {
		return DessertCaptureEvidence{}, errors.New("dessert evidence manifest is missing")
	}
	if expectedSHA256 == "" {
		return DessertCaptureEvidence{}, errors.New("dessert evidence manifest digest is missing")
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != expectedSHA256 {
		return DessertCaptureEvidence{}, errors.New("dessert evidence manifest integrity check failed")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var evidence DessertCaptureEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return DessertCaptureEvidence{}, fmt.Errorf("decode dessert evidence manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return DessertCaptureEvidence{}, errors.New("dessert evidence manifest has trailing data")
		}
		return DessertCaptureEvidence{}, fmt.Errorf("decode dessert evidence trailing data: %w", err)
	}
	if err := validateDessertCaptureEvidence(evidence); err != nil {
		return DessertCaptureEvidence{}, err
	}
	return evidence, nil
}

func validateDessertCaptureEvidence(evidence DessertCaptureEvidence) error {
	if evidence.SchemaVersion != 2 {
		return fmt.Errorf("unsupported dessert evidence schema version %d", evidence.SchemaVersion)
	}
	if evidence.Activity != "dessert" {
		return errors.New("dessert evidence activity mismatch")
	}
	replay := evidence.Replay
	if !replay.Verified || replay.RPC != "actDessert.gameSync" || replay.Mode != 1 {
		return errors.New("dessert replay evidence identity is incomplete")
	}
	if replay.RequestCount != 181 || replay.ResponseCount != 181 ||
		replay.DropCount != 100 || replay.MergeCount != 81 {
		return errors.New("dessert replay evidence summary mismatch")
	}
	if replay.RequestCount != replay.ResponseCount || replay.DropCount+replay.MergeCount != replay.RequestCount {
		return errors.New("dessert replay evidence counts are inconsistent")
	}
	if !validDessertEvidenceSHA256(replay.FixtureSHA256) {
		return errors.New("dessert replay fixture digest is invalid")
	}
	if !replay.ResponseOrderVerified || !replay.TopologyVerified || !replay.CheckpointRebuildVerified {
		return errors.New("dessert replay checkpoint evidence is incomplete")
	}
	if replay.TrajectoryVerified && !replay.CausalSequenceVerified {
		return errors.New("dessert trajectory cannot be verified from a non-causal sequence")
	}
	rewards := evidence.RewardBoxes
	if rewards.OpenBoxSuccess {
		if rewards.OpenBoxRequestNum != 1 || !validDessertEvidenceSHA256(rewards.OpenBoxFixtureSHA256) {
			return errors.New("dessert single-box evidence is incomplete")
		}
	} else if rewards.OpenBoxRequestNum != 0 || rewards.OpenBoxFixtureSHA256 != "" {
		return errors.New("dessert open-box evidence is inconsistent")
	}
	if rewards.RecvBoxesSuccess && !validDessertEvidenceSHA256(rewards.RecvBoxesFixtureSHA256) {
		return errors.New("dessert progress-box evidence is incomplete")
	}
	live := evidence.LiveAutoplay
	if (live.NaturalEndSuccess || live.TerminalGameOverSuccess) && !live.GameStartFromIdleSuccess {
		return errors.New("dessert terminal lifecycle lacks a verified game start")
	}
	if !validDessertEvidenceSHA256(live.LifecycleFixtureSHA256) {
		return errors.New("dessert lifecycle fixture digest is invalid")
	}
	return nil
}
