package captureanalysis

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

const (
	DessertRoundFixtureSchema = "dessert-mode1-round-v1"
	dessertGameSyncRPC        = "actDessert.gameSync"
)

var dessertFixtureRadiiPX = [...]float64{19.5, 26.5, 36.5, 45.5, 56, 63.5, 79, 105, 111.5, 135.5, 187}

// DessertRoundFixture is a sanitized, identity-free replay corpus. It contains
// only game physics and aggregate counters; capture correlation
// keys, batch IDs, user IDs, timestamps, URLs, and source paths are excluded.
type DessertRoundFixture struct {
	Schema               string                     `json:"schema"`
	Mode                 int32                      `json:"mode"`
	ContinuousTrajectory bool                       `json:"continuous_trajectory"`
	Initial              DessertFixtureState        `json:"initial"`
	Checkpoints          []DessertFixtureCheckpoint `json:"checkpoints"`
	Final                DessertFixtureFinal        `json:"final"`
}

type DessertFixtureCheckpoint struct {
	Sequence   int                    `json:"sequence"`
	Operation  string                 `json:"operation"`
	DropLevel  int32                  `json:"drop_level,omitempty"`
	MergeLevel int32                  `json:"merge_level,omitempty"`
	Submitted  DessertFixtureState    `json:"submitted"`
	Server     DessertFixtureCounters `json:"server"`
}

type DessertFixtureState struct {
	Step         int32                `json:"step"`
	Score        int32                `json:"score"`
	CurrentLevel int32                `json:"current_level"`
	GameStatus   int32                `json:"game_status"`
	Running      bool                 `json:"running"`
	FirstMerge   map[int32]int32      `json:"first_merge"`
	TotalGain    map[int32]int32      `json:"total_gain"`
	LevelMap     map[int32]int32      `json:"level_map"`
	Bodies       []DessertFixtureBody `json:"bodies"`
}

// DessertFixtureBody is one member of an unordered body multiset. It has no
// synthetic ID: object identity cannot be proved across captured frames.
type DessertFixtureBody struct {
	Level           int32                 `json:"level"`
	Position        DessertFixtureVector2 `json:"position"`
	LinearVelocity  DessertFixtureVector2 `json:"linear_velocity"`
	AngularVelocity float64               `json:"angular_velocity"`
	NodeAngleDeg    float64               `json:"node_angle_deg"`
	Scale           DessertFixtureVector3 `json:"scale"`
	Awake           bool                  `json:"awake"`
	Falling         bool                  `json:"falling"`
}

type DessertFixtureVector2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type DessertFixtureVector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type DessertFixtureCounters struct {
	Step         int32           `json:"step"`
	Score        int32           `json:"score"`
	CurrentLevel int32           `json:"current_level"`
	GameStatus   int32           `json:"game_status"`
	Running      bool            `json:"running"`
	FirstMerge   map[int32]int32 `json:"first_merge"`
	TotalGain    map[int32]int32 `json:"total_gain"`
	LevelMap     map[int32]int32 `json:"level_map"`
	DropCount    int32           `json:"drop_count"`
	TotalScore   int32           `json:"total_score"`
	Energy       int32           `json:"energy"`
	Currency     int32           `json:"currency"`
	Points       int32           `json:"points"`
	RewardBoxes  int32           `json:"reward_boxes"`
}

type DessertFixtureFinal struct {
	Drops              int32                  `json:"drops"`
	Merges             int32                  `json:"merges"`
	MergeLevelCounts   map[int32]int32        `json:"merge_level_counts"`
	Submitted          DessertFixtureState    `json:"submitted"`
	Server             DessertFixtureCounters `json:"server"`
	BodyLevelCounts    map[int32]int32        `json:"body_level_counts"`
	StepLagCheckpoints int32                  `json:"step_lag_checkpoints"`
}

type dessertFixturePending struct {
	flowID  string
	key     string
	batchID int32
	order   int
	action  string
	merge   int32
	state   DessertFixtureState
}

type dessertFixturePair struct {
	request  dessertFixturePending
	response json.RawMessage
}

type dessertFixtureExpected struct {
	drops  int
	merges int
}

// ExtractDessertRoundFixture scans websocket.jsonl directly and extracts the
// capture-proven 100-drop/81-merge ordinary-mode round.
func ExtractDessertRoundFixture(sessionDir string, channel babigame.Channel) (DessertRoundFixture, error) {
	f, err := os.Open(filepath.Join(sessionDir, "websocket.jsonl"))
	if err != nil {
		return DessertRoundFixture{}, fmt.Errorf("open websocket jsonl: %w", err)
	}
	defer func() { _ = f.Close() }()
	return extractDessertRoundFixture(f, channel, dessertFixtureExpected{drops: 100, merges: 81})
}

// WriteDessertRoundFixture writes stable compact JSON without embedding the
// input or output path in the fixture itself.
func WriteDessertRoundFixture(sessionDir, outputPath string, channel babigame.Channel) (DessertRoundFixture, error) {
	fixture, err := ExtractDessertRoundFixture(sessionDir, channel)
	if err != nil {
		return DessertRoundFixture{}, err
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		return DessertRoundFixture{}, fmt.Errorf("marshal dessert fixture: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return DessertRoundFixture{}, fmt.Errorf("write dessert fixture: %w", err)
	}
	return fixture, nil
}

func extractDessertRoundFixture(r io.Reader, channel babigame.Channel, expected dessertFixtureExpected) (DessertRoundFixture, error) {
	if channel == babigame.ChannelUnspecified {
		channel = babigame.ChannelIOS
	}
	cfg, err := babigame.ConfigForChannel(channel)
	if err != nil {
		return DessertRoundFixture{}, err
	}
	pending := make(map[string]dessertFixturePending)
	pairs := make([]dessertFixturePair, 0, expected.drops+expected.merges)
	requestOrder := 0
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		var frame wsJSONLRecord
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			return DessertRoundFixture{}, fmt.Errorf("websocket line %d: %w", lineNo, err)
		}
		if frame.OpcodeText != "text" {
			continue
		}
		text := frame.Text
		if text == "" && frame.Base64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(frame.Base64)
			if err != nil {
				return DessertRoundFixture{}, fmt.Errorf("websocket line %d base64: %w", lineNo, err)
			}
			text = string(decoded)
		}
		if text == "" || text == `"connectionEnabled"` {
			continue
		}
		if frame.Direction == "client_to_server" {
			req, relevant, err := decodeDessertFixtureRequest(text, cfg)
			if err != nil {
				return DessertRoundFixture{}, fmt.Errorf("websocket line %d request: %w", lineNo, err)
			}
			if !relevant {
				continue
			}
			requestOrder++
			req.flowID, req.order = frame.FlowID, requestOrder
			correlation := dessertCorrelation(frame.FlowID, req.key)
			if _, duplicate := pending[correlation]; duplicate {
				return DessertRoundFixture{}, fmt.Errorf("websocket line %d: duplicate dessert correlation", lineNo)
			}
			pending[correlation] = req
			continue
		}
		if frame.Direction != "server_to_client" {
			continue
		}
		env, err := babigame.ParseTextFrame(text)
		if err != nil || env.E != "response" {
			continue
		}
		var response babigame.WSResponseD
		if json.Unmarshal(env.D, &response) != nil || response.K == "" {
			continue
		}
		correlation := dessertCorrelation(frame.FlowID, response.K)
		req, ok := pending[correlation]
		if !ok {
			continue
		}
		delete(pending, correlation)
		if response.IsError() {
			return DessertRoundFixture{}, fmt.Errorf("dessert response %d is a server error", req.order)
		}
		if len(response.V) == 0 || bytes.Equal(bytes.TrimSpace(response.V), []byte("null")) {
			return DessertRoundFixture{}, fmt.Errorf("dessert response %d has no namespace payload", req.order)
		}
		pairs = append(pairs, dessertFixturePair{request: req, response: response.V})
	}
	if err := scanner.Err(); err != nil {
		return DessertRoundFixture{}, fmt.Errorf("scan websocket jsonl: %w", err)
	}
	if len(pending) != 0 {
		return DessertRoundFixture{}, fmt.Errorf("capture ended with %d unmatched dessert requests", len(pending))
	}
	return buildDessertRoundFixture(pairs, expected)
}

func dessertCorrelation(flowID, key string) string { return flowID + "\x00" + key }

func decodeDessertFixtureRequest(text string, cfg babigame.Config) (dessertFixturePending, bool, error) {
	env, err := babigame.ParseTextFrame(text)
	if err != nil || env.E != "request" {
		return dessertFixturePending{}, false, nil
	}
	var out babigame.WSEnvelopeOutD
	if err := json.Unmarshal(env.D, &out); err != nil {
		return dessertFixturePending{}, false, nil
	}
	clear, err := babigame.GWDecode(out.P.A, cfg.GWXorMask)
	if err != nil {
		return dessertFixturePending{}, false, nil
	}
	var tuple []json.RawMessage
	if json.Unmarshal(clear, &tuple) != nil || len(tuple) < 2 {
		return dessertFixturePending{}, false, nil
	}
	var rpc string
	if json.Unmarshal(tuple[0], &rpc) != nil || rpc != dessertGameSyncRPC {
		return dessertFixturePending{}, false, nil
	}
	if out.K == "" {
		return dessertFixturePending{}, true, fmt.Errorf("gameSync request has no correlation key")
	}
	req, err := parseDessertGameSyncArgs(tuple[1])
	if err != nil {
		return dessertFixturePending{}, true, err
	}
	req.key = out.K
	return req, true, nil
}

func buildDessertRoundFixture(pairs []dessertFixturePair, expected dessertFixtureExpected) (DessertRoundFixture, error) {
	if len(pairs) != expected.drops+expected.merges {
		return DessertRoundFixture{}, fmt.Errorf("dessert checkpoints=%d, want %d", len(pairs), expected.drops+expected.merges)
	}
	// pairs are appended in response-arrival order. Sparse server counters may
	// only be folded when relevant responses arrive in their request order.
	for index, pair := range pairs {
		if pair.request.order != index+1 {
			return DessertRoundFixture{}, fmt.Errorf("dessert responses arrived out of request order")
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].request.order < pairs[j].request.order })
	fixture := DessertRoundFixture{Schema: DessertRoundFixtureSchema, Mode: 1, ContinuousTrajectory: false}
	fixture.Checkpoints = make([]DessertFixtureCheckpoint, 0, len(pairs))
	mergeCounts := make(map[int32]int32)
	counters := DessertFixtureCounters{DropCount: -1, Energy: -1, Currency: -1, Points: -1, RewardBoxes: -1}
	var batchID int32
	var drops, merges, stepLag int
	previousLevels := make(map[int32]int32)
	for index, pair := range pairs {
		if pair.request.order != index+1 {
			return DessertRoundFixture{}, fmt.Errorf("dessert request ordering is not contiguous")
		}
		if batchID == 0 {
			batchID = pair.request.batchID
		} else if pair.request.batchID != batchID {
			return DessertRoundFixture{}, fmt.Errorf("capture contains more than one dessert batch")
		}
		next, err := parseDessertServerCounters(pair.response, batchID, pair.request.state, counters)
		if err != nil {
			return DessertRoundFixture{}, fmt.Errorf("dessert response %d: %w", pair.request.order, err)
		}
		counters = next
		if pair.request.state.Step != int32(drops) {
			stepLag++
		}
		currentLevels := dessertBodyLevelCounts(pair.request.state.Bodies)
		dropLevel, err := dessertValidateLevelTransition(previousLevels, currentLevels, pair.request.action, pair.request.merge)
		if err != nil {
			return DessertRoundFixture{}, fmt.Errorf("dessert checkpoint %d: %w", pair.request.order, err)
		}
		previousLevels = currentLevels
		checkpoint := DessertFixtureCheckpoint{Sequence: index + 1, Operation: pair.request.action, DropLevel: dropLevel, MergeLevel: pair.request.merge, Submitted: pair.request.state, Server: counters}
		fixture.Checkpoints = append(fixture.Checkpoints, checkpoint)
		if pair.request.action == "drop" {
			drops++
		} else {
			merges++
			mergeCounts[pair.request.merge]++
		}
	}
	if drops != expected.drops || merges != expected.merges {
		return DessertRoundFixture{}, fmt.Errorf("dessert actions drop=%d merge=%d, want %d/%d", drops, merges, expected.drops, expected.merges)
	}
	if len(fixture.Checkpoints) == 0 {
		return DessertRoundFixture{}, fmt.Errorf("dessert round is empty")
	}
	first := fixture.Checkpoints[0]
	firstBody := DessertFixtureBody{}
	if len(first.Submitted.Bodies) == 1 {
		firstBody = first.Submitted.Bodies[0]
	}
	firstRadius := float64(0)
	if first.DropLevel > 0 && int(first.DropLevel) <= len(dessertFixtureRadiiPX) {
		firstRadius = dessertFixtureRadiiPX[first.DropLevel-1]
	}
	if first.Operation != "drop" || first.Submitted.Step != 0 || first.Submitted.Score != 0 || !first.Submitted.Running || first.Submitted.GameStatus != 0 ||
		len(first.Submitted.FirstMerge) != 0 || len(first.Submitted.TotalGain) != 0 || len(first.Submitted.LevelMap) != 0 || len(first.Submitted.Bodies) != 1 ||
		firstRadius == 0 || !firstBody.Falling || firstBody.Level != first.DropLevel || firstBody.Position.X < -262+firstRadius || firstBody.Position.X > 262-firstRadius || firstBody.Position.Y != 360 ||
		firstBody.LinearVelocity.X != 0 || firstBody.LinearVelocity.Y != 0 || firstBody.AngularVelocity != 0 || firstBody.Scale.X != 1 || firstBody.Scale.Y != 1 {
		return DessertRoundFixture{}, fmt.Errorf("first checkpoint does not prove an empty initial board")
	}
	fixture.Initial = DessertFixtureState{CurrentLevel: first.Submitted.CurrentLevel, Running: true, FirstMerge: map[int32]int32{}, TotalGain: map[int32]int32{}, LevelMap: map[int32]int32{}, Bodies: []DessertFixtureBody{}}
	last := fixture.Checkpoints[len(fixture.Checkpoints)-1]
	fixture.Final = DessertFixtureFinal{Drops: int32(drops), Merges: int32(merges), MergeLevelCounts: mergeCounts, Submitted: last.Submitted, Server: last.Server, BodyLevelCounts: dessertBodyLevelCounts(last.Submitted.Bodies), StepLagCheckpoints: int32(stepLag)}
	return fixture, nil
}

func dessertBodyLevelCounts(bodies []DessertFixtureBody) map[int32]int32 {
	out := make(map[int32]int32)
	for _, body := range bodies {
		out[body.Level]++
	}
	return out
}

func dessertValidateLevelTransition(before, after map[int32]int32, action string, mergeLevel int32) (int32, error) {
	if action == "drop" {
		var addedLevel int32
		for level := int32(1); level <= 11; level++ {
			delta := after[level] - before[level]
			switch delta {
			case 0:
				continue
			case 1:
				if addedLevel != 0 {
					return 0, fmt.Errorf("drop adds more than one body level")
				}
				addedLevel = level
			default:
				return 0, fmt.Errorf("drop body-level delta is not exactly +1")
			}
		}
		if addedLevel == 0 || len(after) > 11 {
			return 0, fmt.Errorf("drop does not add exactly one body")
		}
		return addedLevel, nil
	}
	want := make(map[int32]int32, len(before)+1)
	for level, count := range before {
		want[level] = count
	}
	switch action {
	case "merge":
		source := mergeLevel - 1
		if want[source] < 2 {
			return 0, fmt.Errorf("merge level %d lacks two level-%d sources", mergeLevel, source)
		}
		want[source] -= 2
		if want[source] == 0 {
			delete(want, source)
		}
		want[mergeLevel]++
	default:
		return 0, fmt.Errorf("unsupported action %q", action)
	}
	if len(want) != len(after) {
		return 0, fmt.Errorf("body-level transition does not match %s", action)
	}
	for level, count := range want {
		if after[level] != count {
			return 0, fmt.Errorf("body-level transition does not match %s", action)
		}
	}
	return 0, nil
}

func parseDessertGameSyncArgs(raw json.RawMessage) (dessertFixturePending, error) {
	top, err := dessertObject(raw, "gameSync request")
	if err != nil {
		return dessertFixturePending{}, err
	}
	if err := dessertExactKeys(top, "gameSync request", "batchId", "gameType", "args"); err != nil {
		return dessertFixturePending{}, err
	}
	batchID, err := dessertPositiveInt32(top["batchId"], "batchId")
	if err != nil {
		return dessertFixturePending{}, err
	}
	gameType, err := dessertNonnegativeInt32(top["gameType"], "gameType")
	if err != nil || gameType != 1 {
		return dessertFixturePending{}, fmt.Errorf("gameType must be ordinary mode 1")
	}
	args, err := dessertObject(top["args"], "gameSync args")
	if err != nil {
		return dessertFixturePending{}, err
	}
	if err := dessertAllowedRequiredKeys(args, "gameSync args", []string{"operationType", "mergeLvl", "saveData"}, "_useItem2SelIdx", "operationType", "mergeLvl", "saveData"); err != nil {
		return dessertFixturePending{}, err
	}
	if rawIndex, ok := args["_useItem2SelIdx"]; ok {
		value, parseErr := dessertNonnegativeInt32(rawIndex, "_useItem2SelIdx")
		if parseErr != nil || value != 0 {
			return dessertFixturePending{}, fmt.Errorf("mode-one fixture cannot use item selector")
		}
	}
	operationType, err := dessertNonnegativeInt32(args["operationType"], "operationType")
	if err != nil {
		return dessertFixturePending{}, err
	}
	mergeLevel, err := dessertNonnegativeInt32(args["mergeLvl"], "mergeLvl")
	if err != nil {
		return dessertFixturePending{}, err
	}
	action := ""
	switch operationType {
	case 0:
		action = "merge"
		if mergeLevel < 2 || mergeLevel > 11 {
			return dessertFixturePending{}, fmt.Errorf("merge level %d is outside 2..11", mergeLevel)
		}
	case 1:
		action = "drop"
		if mergeLevel != 0 {
			return dessertFixturePending{}, fmt.Errorf("drop has nonzero merge level")
		}
	default:
		return dessertFixturePending{}, fmt.Errorf("unsupported operationType %d", operationType)
	}
	state, err := parseDessertSubmittedState(args["saveData"])
	if err != nil {
		return dessertFixturePending{}, err
	}
	return dessertFixturePending{batchID: batchID, action: action, merge: mergeLevel, state: state}, nil
}

func parseDessertSubmittedState(raw json.RawMessage) (DessertFixtureState, error) {
	fields, err := dessertObject(raw, "saveData")
	if err != nil {
		return DessertFixtureState{}, err
	}
	if err := dessertExactKeys(fields, "saveData", "step", "itemUse", "map", "gameStatus", "firstMerge", "isRunning", "totalGain", "curId", "score", "lvMap"); err != nil {
		return DessertFixtureState{}, err
	}
	step, err := dessertNonnegativeInt32(fields["step"], "saveData.step")
	if err != nil {
		return DessertFixtureState{}, err
	}
	score, err := dessertNonnegativeInt32(fields["score"], "saveData.score")
	if err != nil {
		return DessertFixtureState{}, err
	}
	current, err := dessertPositiveInt32(fields["curId"], "saveData.curId")
	if err != nil || current > 11 {
		return DessertFixtureState{}, fmt.Errorf("saveData.curId must be within 1..11")
	}
	gameStatus, err := dessertNonnegativeInt32(fields["gameStatus"], "saveData.gameStatus")
	if err != nil {
		return DessertFixtureState{}, err
	}
	var running bool
	if err := json.Unmarshal(fields["isRunning"], &running); err != nil {
		return DessertFixtureState{}, fmt.Errorf("saveData.isRunning: %w", err)
	}
	itemUse, err := dessertInt32Map(fields["itemUse"], "saveData.itemUse", nil)
	if err != nil || len(itemUse) != 0 {
		return DessertFixtureState{}, fmt.Errorf("mode-one fixture contains item use")
	}
	firstMerge, err := dessertInt32Map(fields["firstMerge"], "saveData.firstMerge", dessertLevelKey)
	if err != nil {
		return DessertFixtureState{}, err
	}
	totalGain, err := dessertInt32Map(fields["totalGain"], "saveData.totalGain", dessertActivityItemKey)
	if err != nil {
		return DessertFixtureState{}, err
	}
	levelMap, err := dessertInt32Map(fields["lvMap"], "saveData.lvMap", dessertLevelKey)
	if err != nil {
		return DessertFixtureState{}, err
	}
	bodies, err := dessertBodies(fields["map"])
	if err != nil {
		return DessertFixtureState{}, err
	}
	return DessertFixtureState{Step: step, Score: score, CurrentLevel: current, GameStatus: gameStatus, Running: running, FirstMerge: firstMerge, TotalGain: totalGain, LevelMap: levelMap, Bodies: bodies}, nil
}

func dessertBodies(raw json.RawMessage) ([]DessertFixtureBody, error) {
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return nil, fmt.Errorf("saveData.map must be an array")
	}
	out := make([]DessertFixtureBody, 0, len(rows))
	for index, row := range rows {
		fields, err := dessertObject(row, fmt.Sprintf("saveData.map[%d]", index))
		if err != nil {
			return nil, err
		}
		if err := dessertAllowedRequiredKeys(fields, fmt.Sprintf("saveData.map[%d]", index), []string{"lv", "pos", "linearVelocity", "angularVelocity", "nodeAngle", "scale", "isAwake", "isSyn", "_lineTime"}, "lv", "pos", "linearVelocity", "angularVelocity", "nodeAngle", "scale", "isAwake", "isSyn", "isFallBall", "_lineTime"); err != nil {
			return nil, err
		}
		level, err := dessertPositiveInt32(fields["lv"], "body.lv")
		if err != nil || level > 11 {
			return nil, fmt.Errorf("body level must be within 1..11")
		}
		position, err := dessertVector2(fields["pos"], "body.pos")
		if err != nil {
			return nil, err
		}
		velocity, err := dessertVector2(fields["linearVelocity"], "body.linearVelocity")
		if err != nil {
			return nil, err
		}
		angular, err := dessertFiniteFloat(fields["angularVelocity"], "body.angularVelocity")
		if err != nil {
			return nil, err
		}
		angle, err := dessertFiniteFloat(fields["nodeAngle"], "body.nodeAngle")
		if err != nil {
			return nil, err
		}
		scale, err := dessertVector3(fields["scale"], "body.scale")
		if err != nil || scale.X != scale.Y || scale.X < 0.5 || scale.X > 1 || scale.Z != 0.5 {
			return nil, fmt.Errorf("body.scale must use the capture-proven uniform range 0.5..1 with z=0.5")
		}
		lineTime, err := dessertFiniteFloat(fields["_lineTime"], "body._lineTime")
		if err != nil || lineTime != 0 {
			return nil, fmt.Errorf("body._lineTime is not the capture-proven zero")
		}
		var awake, syn, falling bool
		if json.Unmarshal(fields["isAwake"], &awake) != nil || json.Unmarshal(fields["isSyn"], &syn) != nil {
			return nil, fmt.Errorf("body awake/sync flags must be booleans")
		}
		if syn {
			return nil, fmt.Errorf("body.isSyn=true is not safe for an offline physics fixture")
		}
		if rawFalling, ok := fields["isFallBall"]; ok && json.Unmarshal(rawFalling, &falling) != nil {
			return nil, fmt.Errorf("body.isFallBall must be boolean")
		}
		out = append(out, DessertFixtureBody{Level: level, Position: position, LinearVelocity: velocity, AngularVelocity: angular, NodeAngleDeg: angle, Scale: scale, Awake: awake, Falling: falling})
	}
	dessertSortBodies(out)
	return out, nil
}

func parseDessertServerCounters(raw json.RawMessage, batchID int32, submitted DessertFixtureState, before DessertFixtureCounters) (DessertFixtureCounters, error) {
	root, err := dessertObject(raw, "response.v")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	ns23, err := dessertObject(root["23"], "response.v.23")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	batches, err := dessertObject(ns23["0"], "response.v.23.0")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	batch, err := dessertObject(batches[strconv.FormatInt(int64(batchID), 10)], "response dessert batch")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	next := before
	if rawCount, ok := batch["11"]; ok {
		next.DropCount, err = dessertNonnegativeInt32(rawCount, "response drop count")
		if err != nil {
			return DessertFixtureCounters{}, err
		}
	}
	if rawBag, ok := batch["12"]; ok {
		bag, mapErr := dessertInt32Map(rawBag, "response activity bag", dessertActivityItemKey)
		if mapErr != nil {
			return DessertFixtureCounters{}, mapErr
		}
		next.Energy = bag[1342]
		next.Currency = bag[1343]
		next.Points = bag[1344]
		next.RewardBoxes = bag[1347]
	}
	extension, err := dessertObject(batch["14"], "response dessert extension")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	dessert, err := dessertObject(extension["121"], "response dessert extension 121")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	if rawScore, ok := dessert["0"]; ok {
		next.TotalScore, err = dessertNonnegativeInt32(rawScore, "response total score")
		if err != nil {
			return DessertFixtureCounters{}, err
		}
	}
	modes, err := dessertObject(dessert["1"], "response dessert modes")
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	serverState, err := parseDessertNumericMode(modes["1"])
	if err != nil {
		return DessertFixtureCounters{}, err
	}
	if !dessertBodiesEqual(serverState.Bodies, submitted.Bodies) {
		return DessertFixtureCounters{}, fmt.Errorf("response body multiset differs from submitted saveData")
	}
	// The server echoes the submitted unordered body multiset but advances the
	// mode counters. Preserve those authoritative post-operation counters once,
	// without duplicating the response map or inventing cross-frame body IDs.
	next.Step = serverState.Step
	next.Score = serverState.Score
	next.CurrentLevel = serverState.CurrentLevel
	next.GameStatus = serverState.GameStatus
	next.Running = serverState.Running
	next.FirstMerge = serverState.FirstMerge
	next.TotalGain = serverState.TotalGain
	next.LevelMap = serverState.LevelMap
	if next.DropCount < 0 || next.Energy < 0 || next.Currency < 0 || next.Points < 0 || next.RewardBoxes < 0 || next.TotalScore < 0 {
		return DessertFixtureCounters{}, fmt.Errorf("response counters are incomplete")
	}
	return next, nil
}

func parseDessertNumericMode(raw json.RawMessage) (DessertFixtureState, error) {
	fields, err := dessertObject(raw, "response mode 1")
	if err != nil {
		return DessertFixtureState{}, err
	}
	if err := dessertExactKeys(fields, "response mode 1", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9"); err != nil {
		return DessertFixtureState{}, err
	}
	named, err := json.Marshal(map[string]json.RawMessage{
		"step": fields["0"], "itemUse": fields["1"], "map": fields["2"], "gameStatus": fields["3"], "firstMerge": fields["4"],
		"isRunning": fields["5"], "totalGain": fields["6"], "curId": fields["7"], "score": fields["8"], "lvMap": fields["9"],
	})
	if err != nil {
		return DessertFixtureState{}, err
	}
	return parseDessertSubmittedState(named)
}

func dessertObject(raw json.RawMessage, label string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s is missing", label)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return nil, fmt.Errorf("%s must be an object", label)
	}
	return out, nil
}

func dessertExactKeys(values map[string]json.RawMessage, label string, keys ...string) error {
	return dessertAllowedRequiredKeys(values, label, keys, keys...)
}

func dessertAllowedRequiredKeys(values map[string]json.RawMessage, label string, required []string, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range values {
		if _, ok := allowedSet[key]; !ok {
			return fmt.Errorf("%s contains unsupported field %q", label, key)
		}
	}
	for _, key := range required {
		if _, ok := values[key]; !ok {
			return fmt.Errorf("%s is missing field %q", label, key)
		}
	}
	return nil
}

func dessertPositiveInt32(raw json.RawMessage, label string) (int32, error) {
	value, err := dessertNonnegativeInt32(raw, label)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", label)
	}
	return value, nil
}

func dessertNonnegativeInt32(raw json.RawMessage, label string) (int32, error) {
	var value int64
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil || value < 0 || value > math.MaxInt32 {
		return 0, fmt.Errorf("%s must be a nonnegative int32", label)
	}
	return int32(value), nil
}

func dessertFiniteFloat(raw json.RawMessage, label string) (float64, error) {
	var value float64
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be finite", label)
	}
	return value, nil
}

func dessertVector2(raw json.RawMessage, label string) (DessertFixtureVector2, error) {
	fields, err := dessertObject(raw, label)
	if err != nil {
		return DessertFixtureVector2{}, err
	}
	if err := dessertExactKeys(fields, label, "x", "y"); err != nil {
		return DessertFixtureVector2{}, err
	}
	x, err := dessertFiniteFloat(fields["x"], label+".x")
	if err != nil {
		return DessertFixtureVector2{}, err
	}
	y, err := dessertFiniteFloat(fields["y"], label+".y")
	if err != nil {
		return DessertFixtureVector2{}, err
	}
	return DessertFixtureVector2{X: x, Y: y}, nil
}

func dessertVector3(raw json.RawMessage, label string) (DessertFixtureVector3, error) {
	fields, err := dessertObject(raw, label)
	if err != nil {
		return DessertFixtureVector3{}, err
	}
	if err := dessertExactKeys(fields, label, "x", "y", "z"); err != nil {
		return DessertFixtureVector3{}, err
	}
	x, err := dessertFiniteFloat(fields["x"], label+".x")
	if err != nil {
		return DessertFixtureVector3{}, err
	}
	y, err := dessertFiniteFloat(fields["y"], label+".y")
	if err != nil {
		return DessertFixtureVector3{}, err
	}
	z, err := dessertFiniteFloat(fields["z"], label+".z")
	if err != nil {
		return DessertFixtureVector3{}, err
	}
	return DessertFixtureVector3{X: x, Y: y, Z: z}, nil
}

func dessertInt32Map(raw json.RawMessage, label string, validKey func(int32) bool) (map[int32]int32, error) {
	values, err := dessertObject(raw, label)
	if err != nil {
		return nil, err
	}
	out := make(map[int32]int32, len(values))
	for key, rawValue := range values {
		parsed, parseErr := strconv.ParseInt(key, 10, 32)
		if parseErr != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != key {
			return nil, fmt.Errorf("%s has invalid key %q", label, key)
		}
		id := int32(parsed)
		if validKey != nil && !validKey(id) {
			return nil, fmt.Errorf("%s has unsupported key %d", label, id)
		}
		value, valueErr := dessertNonnegativeInt32(rawValue, label+"["+key+"]")
		if valueErr != nil {
			return nil, valueErr
		}
		out[id] = value
	}
	return out, nil
}

func dessertLevelKey(value int32) bool { return value >= 1 && value <= 11 }

func dessertActivityItemKey(value int32) bool {
	switch value {
	case 1342, 1343, 1344, 1347:
		return true
	default:
		return false
	}
}

func dessertSortBodies(bodies []DessertFixtureBody) {
	sort.Slice(bodies, func(i, j int) bool {
		left, _ := json.Marshal(bodies[i])
		right, _ := json.Marshal(bodies[j])
		return bytes.Compare(left, right) < 0
	})
}

func dessertBodiesEqual(left, right []DessertFixtureBody) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
