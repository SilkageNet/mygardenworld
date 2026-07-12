package captureanalysis

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

const dessertRoundFixtureSHA256 = "5ea5ad9b2ff60b8fab83b8d00f01f183addb63fd2356e1d6d35e7038bc20b64b"

func TestDessertRoundFixtureEvidenceInvariants(t *testing.T) {
	path := filepath.Join("..", "dessertphysics", "testdata", "mode1_round_100.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != dessertRoundFixtureSHA256 {
		t.Fatalf("fixture sha256=%s want %s", got, dessertRoundFixtureSHA256)
	}
	evidence, err := babigame.ReadDessertCaptureEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Replay.FixtureSHA256 != dessertRoundFixtureSHA256 {
		t.Fatalf("evidence fixture sha256=%s want %s", evidence.Replay.FixtureSHA256, dessertRoundFixtureSHA256)
	}
	var fixture DessertRoundFixture
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != DessertRoundFixtureSchema || fixture.Mode != 1 || fixture.ContinuousTrajectory {
		t.Fatalf("fixture identity/trajectory flags=%+v", fixture)
	}
	if len(fixture.Initial.Bodies) != 0 || fixture.Initial.Step != 0 || fixture.Initial.Score != 0 || !fixture.Initial.Running {
		t.Fatalf("initial state is not an empty running board: %+v", fixture.Initial)
	}
	if len(fixture.Checkpoints) != 181 || fixture.Final.Drops != 100 || fixture.Final.Merges != 81 || fixture.Final.StepLagCheckpoints != 18 {
		t.Fatalf("fixture totals=%+v checkpoints=%d", fixture.Final, len(fixture.Checkpoints))
	}
	if fixture.Checkpoints[0].Server.DropCount != 1 || fixture.Checkpoints[0].Server.Energy != 99 || fixture.Checkpoints[0].Server.Currency != 1 {
		t.Fatalf("first response does not establish the server counter baseline: %+v", fixture.Checkpoints[0].Server)
	}
	if fixture.Final.Server.Step != 100 || fixture.Final.Server.DropCount != 100 || fixture.Final.Server.TotalScore != 2220 || fixture.Final.Server.Energy != 0 {
		t.Fatalf("final authoritative counters=%+v", fixture.Final.Server)
	}

	levels := make(map[int32]int32)
	drops, merges := 0, 0
	mergeLevels := make(map[int32]int32)
	for index, checkpoint := range fixture.Checkpoints {
		if checkpoint.Sequence != index+1 {
			t.Fatalf("sequence[%d]=%d", index, checkpoint.Sequence)
		}
		for _, body := range checkpoint.Submitted.Bodies {
			assertFiniteDessertBody(t, body)
		}
		current := dessertBodyLevelCounts(checkpoint.Submitted.Bodies)
		dropLevel, err := dessertValidateLevelTransition(levels, current, checkpoint.Operation, checkpoint.MergeLevel)
		if err != nil {
			t.Fatalf("checkpoint %d topology: %v", checkpoint.Sequence, err)
		}
		if dropLevel != checkpoint.DropLevel {
			t.Fatalf("checkpoint %d drop level=%d want %d", checkpoint.Sequence, checkpoint.DropLevel, dropLevel)
		}
		levels = current
		switch checkpoint.Operation {
		case "drop":
			drops++
		case "merge":
			merges++
			mergeLevels[checkpoint.MergeLevel]++
		default:
			t.Fatalf("checkpoint %d operation=%q", checkpoint.Sequence, checkpoint.Operation)
		}
	}
	if drops != 100 || merges != 81 || !dessertFixtureMapsEqual(mergeLevels, fixture.Final.MergeLevelCounts) || !dessertFixtureMapsEqual(levels, fixture.Final.BodyLevelCounts) {
		t.Fatalf("replayed totals drops=%d merges=%d mergeLevels=%v levels=%v", drops, merges, mergeLevels, levels)
	}
}

func TestDessertRoundFixtureContainsOnlyWhitelistedSanitizedShape(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "dessertphysics", "testdata", "mode1_round_100.json"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{`"batch`, `"uid`, `"key`, `"time`, `"path`, `"url`, `"flow`, `"token`, `"session`, `"source`, `"issyn"`, `"_linetime"`} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("fixture contains forbidden field prefix %q", forbidden)
		}
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	assertDessertFixtureStrings(t, raw)
}

func TestExtractDessertFixturePairsInterleavedFramesByFlowAndKey(t *testing.T) {
	cfg, err := babigame.ConfigForChannel(babigame.ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	target, targetKey := syntheticDessertRequest(t, cfg, 0, []map[string]any{syntheticDessertBody(1, 10)})
	unrelated, unrelatedKey, err := babigame.BuildRequest("usrLand.harvest", map[string]any{"landId": 1001}, "", 2, cfg)
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		syntheticWSLine(t, "flow-a", "client_to_server", target),
		syntheticWSLine(t, "flow-a", "client_to_server", unrelated),
		syntheticWSLine(t, "flow-a", "server_to_client", syntheticResponse(t, unrelatedKey, map[string]any{"100": map[string]any{}})),
		syntheticWSLine(t, "flow-a", "server_to_client", syntheticDessertResponse(t, targetKey, 1, []map[string]any{syntheticDessertBody(1, 10)})),
	}
	fixture, err := extractDessertRoundFixture(strings.NewReader(strings.Join(lines, "\n")+"\n"), babigame.ChannelIOS, dessertFixtureExpected{drops: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.Checkpoints) != 1 || fixture.Checkpoints[0].Operation != "drop" || fixture.Checkpoints[0].Server.DropCount != 1 {
		t.Fatalf("fixture=%+v", fixture)
	}
}

func TestExtractDessertFixtureRejectsOutOfOrderRelevantResponses(t *testing.T) {
	cfg, err := babigame.ConfigForChannel(babigame.ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	body1 := syntheticDessertBody(1, 10)
	body2 := syntheticDessertBody(1, 20)
	request1, key1 := syntheticDessertRequest(t, cfg, 0, []map[string]any{body1})
	request2, key2 := syntheticDessertRequest(t, cfg, 1, []map[string]any{body1, body2})
	lines := []string{
		syntheticWSLine(t, "flow-a", "client_to_server", request1),
		syntheticWSLine(t, "flow-a", "client_to_server", request2),
		syntheticWSLine(t, "flow-a", "server_to_client", syntheticDessertResponse(t, key2, 2, []map[string]any{body1, body2})),
		syntheticWSLine(t, "flow-a", "server_to_client", syntheticDessertResponse(t, key1, 1, []map[string]any{body1})),
	}
	_, err = extractDessertRoundFixture(strings.NewReader(strings.Join(lines, "\n")+"\n"), babigame.ChannelIOS, dessertFixtureExpected{drops: 2})
	if err == nil || !strings.Contains(err.Error(), "out of request order") {
		t.Fatalf("err=%v", err)
	}
}

func TestDessertFixtureRejectsUnsafeBodyValues(t *testing.T) {
	for name, mutation := range map[string]func(map[string]any){
		"syncing body": func(body map[string]any) { body["isSyn"] = true },
		"nonfinite":    func(body map[string]any) { body["angularVelocity"] = json.RawMessage(`1e9999`) },
		"line timer":   func(body map[string]any) { body["_lineTime"] = 1 },
		"anisotropic scale": func(body map[string]any) {
			body["scale"].(map[string]any)["y"] = 0.75
		},
		"out of range scale": func(body map[string]any) {
			body["scale"].(map[string]any)["x"] = 1.1
			body["scale"].(map[string]any)["y"] = 1.1
		},
		"invalid depth scale": func(body map[string]any) {
			body["scale"].(map[string]any)["z"] = 1
		},
	} {
		t.Run(name, func(t *testing.T) {
			body := syntheticDessertBody(1, 10)
			mutation(body)
			save := syntheticDessertSave(0, []map[string]any{body})
			raw, _ := json.Marshal(save)
			if _, err := parseDessertSubmittedState(raw); err == nil {
				t.Fatal("unsafe body accepted")
			}
		})
	}
}

func TestDessertFixtureOptionalRegenerationMatchesCommittedCorpus(t *testing.T) {
	sessionDir := os.Getenv("DESSERT_CAPTURE_SESSION")
	if sessionDir == "" {
		t.Skip("set DESSERT_CAPTURE_SESSION to re-extract and compare the local authorized capture")
	}
	fixture, err := ExtractDessertRoundFixture(sessionDir, babigame.ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	want, err := os.ReadFile(filepath.Join("..", "dessertphysics", "testdata", "mode1_round_100.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("regenerated sanitized fixture differs from the committed corpus")
	}
}

func syntheticDessertRequest(t *testing.T, cfg babigame.Config, step int32, bodies []map[string]any) (string, string) {
	t.Helper()
	frame, key, err := babigame.BuildRequest(dessertGameSyncRPC, map[string]any{
		"batchId": 9001, "gameType": 1,
		"args": map[string]any{"_useItem2SelIdx": 0, "operationType": 1, "mergeLvl": 0, "saveData": syntheticDessertSave(step, bodies)},
	}, "", int64(step+10), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return frame, key
}

func syntheticDessertSave(step int32, bodies []map[string]any) map[string]any {
	totalGain := map[string]any{}
	levelMap := map[string]any{}
	if step > 0 {
		totalGain["1343"] = step
		levelMap["1"] = step
	}
	return map[string]any{
		"step": step, "itemUse": map[string]any{}, "map": bodies, "gameStatus": 0, "firstMerge": map[string]any{},
		"isRunning": true, "totalGain": totalGain, "curId": 1, "score": 0, "lvMap": levelMap,
	}
}

func syntheticDessertBody(level int32, x float64) map[string]any {
	return map[string]any{
		"lv": level, "pos": map[string]any{"x": x, "y": 360}, "linearVelocity": map[string]any{"x": 0, "y": 0},
		"angularVelocity": 0, "nodeAngle": 0, "scale": map[string]any{"x": 1, "y": 1, "z": 0.5},
		"isAwake": true, "isSyn": false, "isFallBall": true, "_lineTime": 0,
	}
}

func syntheticDessertResponse(t *testing.T, key string, step int32, bodies []map[string]any) string {
	t.Helper()
	mode := map[string]any{
		"0": step, "1": map[string]any{}, "2": bodies, "3": 0, "4": map[string]any{}, "5": true,
		"6": map[string]any{"1343": step}, "7": 1, "8": 0, "9": map[string]any{"1": step},
	}
	v := map[string]any{"23": map[string]any{"0": map[string]any{"9001": map[string]any{
		"11": step, "12": map[string]any{"1342": 100 - step, "1343": step},
		"14": map[string]any{"121": map[string]any{"0": 0, "1": map[string]any{"1": mode}}},
	}}}}
	return syntheticResponse(t, key, v)
}

func syntheticResponse(t *testing.T, key string, v map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"e": "response", "d": map[string]any{"k": key, "v": v}})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func syntheticWSLine(t *testing.T, flow, direction, text string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"flow_id": flow, "direction": direction, "opcode_text": "text", "text": text})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertFiniteDessertBody(t *testing.T, body DessertFixtureBody) {
	t.Helper()
	for name, value := range map[string]float64{
		"x": body.Position.X, "y": body.Position.Y, "vx": body.LinearVelocity.X, "vy": body.LinearVelocity.Y,
		"angular": body.AngularVelocity, "angle": body.NodeAngleDeg, "scale_x": body.Scale.X, "scale_y": body.Scale.Y, "scale_z": body.Scale.Z,
	} {
		if value != value || value > 1e308 || value < -1e308 {
			t.Fatalf("body %s is nonfinite: %v", name, value)
		}
	}
}

func dessertFixtureMapsEqual(left, right map[int32]int32) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func assertDessertFixtureStrings(t *testing.T, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			lower := strings.ToLower(key)
			for _, forbidden := range []string{"batch", "uid", "key", "time", "path", "url", "flow", "token", "session", "source"} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("forbidden key %q", key)
				}
			}
			assertDessertFixtureStrings(t, nested)
		}
	case []any:
		for _, nested := range typed {
			assertDessertFixtureStrings(t, nested)
		}
	case string:
		if typed != DessertRoundFixtureSchema && typed != "drop" && typed != "merge" {
			t.Fatalf("unexpected string value %q", typed)
		}
	}
}
