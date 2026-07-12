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

const dessertEvidenceManifestSHA256 = "48b491250c6433b6445e55acbf8b0cd998dcf263305bade6b7221ad761e0fb2f"

//go:embed testdata/dessert_evidence_manifest.json
var dessertEvidenceManifestJSON []byte

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
	Verified      bool   `json:"verified"`
	RPC           string `json:"rpc"`
	Mode          int32  `json:"mode"`
	RequestCount  int    `json:"request_count"`
	ResponseCount int    `json:"response_count"`
	DropCount     int    `json:"drop_count"`
	MergeCount    int    `json:"merge_count"`
}

type DessertRewardBoxEvidence struct {
	RecvBoxesSuccess bool `json:"recv_boxes_success"`
	OpenBoxSuccess   bool `json:"open_box_success"`
}

type DessertLiveAutoplayEvidence struct {
	GameStartFromIdleSuccess bool `json:"game_start_from_idle_success"`
	PhaseThreeSuccess        bool `json:"phase_three_success"`
	NaturalEndSuccess        bool `json:"natural_end_success"`
	TerminalGameOverSuccess  bool `json:"terminal_game_over_success"`
}

// ReadDessertCaptureEvidence verifies the embedded manifest byte-for-byte and
// then strictly decodes its schema. Any missing, unknown, or modified manifest
// is rejected rather than weakening an execution gate.
func ReadDessertCaptureEvidence() (DessertCaptureEvidence, error) {
	return readDessertCaptureEvidence(dessertEvidenceManifestJSON, dessertEvidenceManifestSHA256)
}

// DessertRewardBoxEvidenceGate reports whether both reward-box RPC success
// paths have reviewed capture evidence. It fails closed on every read error.
func DessertRewardBoxEvidenceGate() bool {
	evidence, err := ReadDessertCaptureEvidence()
	return err == nil && evidence.RewardBoxes.RecvBoxesSuccess && evidence.RewardBoxes.OpenBoxSuccess
}

// DessertLiveAutoplayEvidenceGate reports whether the replay and every live
// lifecycle prerequisite have reviewed capture evidence. A replay trace alone
// can never enable live autoplay.
func DessertLiveAutoplayEvidenceGate() bool {
	evidence, err := ReadDessertCaptureEvidence()
	return err == nil && evidence.Replay.Verified &&
		evidence.LiveAutoplay.GameStartFromIdleSuccess &&
		evidence.LiveAutoplay.PhaseThreeSuccess &&
		evidence.LiveAutoplay.NaturalEndSuccess &&
		evidence.LiveAutoplay.TerminalGameOverSuccess
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
	if evidence.SchemaVersion != 1 {
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
	return nil
}
