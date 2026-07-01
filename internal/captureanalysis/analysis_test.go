package captureanalysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestBuildRPCIndexFromWebSocketJSONL(t *testing.T) {
	dir := writeSampleSession(t)
	rpcPath, err := BuildRPCIndex(dir, Options{Channel: babigame.ChannelIOS, Rewrite: true})
	if err != nil {
		t.Fatal(err)
	}
	records := readRPCIndex(t, rpcPath)
	if len(records) != 2 {
		t.Fatalf("records=%d want 2", len(records))
	}
	if records[0].Type != "rpc_request" || records[0].RPC != "usrLand.harvest" {
		t.Fatalf("bad request record: %+v", records[0])
	}
	if records[0].ArgShape != "object" || len(records[0].ArgKeys) != 1 || records[0].ArgKeys[0] != "landId" {
		t.Fatalf("bad request arg summary: %+v", records[0])
	}
	if records[1].Type != "rpc_response" || records[1].RPC != "usrLand.harvest" {
		t.Fatalf("bad response record: %+v", records[1])
	}
	if got := strings.Join(records[1].Namespaces, ","); got != "100,7" {
		t.Fatalf("namespaces=%q want 100,7", got)
	}
}

func TestAnalyzeSessionWritesReport(t *testing.T) {
	dir := writeSampleSession(t)
	report, err := AnalyzeSession(dir, Options{Channel: babigame.ChannelIOS, Rewrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.WS.ClientRPCs != 1 || report.WS.ServerResponses != 1 {
		t.Fatalf("unexpected ws summary: %+v", report.WS)
	}
	if report.Protocol["app_version"] != "1.1.17" {
		t.Fatalf("app_version protocol hint = %q, want 1.1.17", report.Protocol["app_version"])
	}
	sample := report.RPCSamples["usrLand.harvest"]
	if sample.Count != 1 || !sample.Known || sample.ArgShape != "object" {
		t.Fatalf("bad rpc sample: %+v", sample)
	}
	if len(report.UnknownRPCs) != 0 || len(report.UnknownNamespaces) != 0 {
		t.Fatalf("unexpected unknown diff: rpc=%v ns=%v", report.UnknownRPCs, report.UnknownNamespaces)
	}
	if _, err := os.Stat(filepath.Join(dir, ReportFile)); err != nil {
		t.Fatalf("analysis report missing: %v", err)
	}
}

func TestSanitizeScalarRedactsIdentityAndSessionKeys(t *testing.T) {
	sample := map[string]any{
		"open_id":    "openid-value",
		"roleId":     "role-value",
		"routeToken": "route-value",
		"session1":   "session-value",
		"idfa":       "device-value",
		"mobile":     "13800000000",
		"nested": map[string]any{
			"uid": "uid-value",
		},
	}

	got := sanitizeScalar("", sample).(map[string]any)
	for _, key := range []string{"open_id", "roleId", "routeToken", "session1", "idfa", "mobile"} {
		if got[key] != "<redacted>" {
			t.Fatalf("%s was not redacted: %+v", key, got[key])
		}
	}
	nested := got["nested"].(map[string]any)
	if nested["uid"] != "<redacted>" {
		t.Fatalf("nested uid was not redacted: %+v", nested)
	}
}

func TestSanitizeScalarKeepsBusinessIDs(t *testing.T) {
	sample := map[string]any{
		"itemId":   float64(23001),
		"flowerId": float64(23005),
		"landId":   float64(1001),
	}

	got := sanitizeScalar("", sample).(map[string]any)
	for key, want := range sample {
		if got[key] != want {
			t.Fatalf("%s=%v, want %v", key, got[key], want)
		}
	}
}

func writeSampleSession(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfg, err := babigame.ConfigForChannel(babigame.ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	frame, k, err := babigame.BuildRequest("usrLand.harvest", map[string]any{"landId": 1001}, "tok|2482", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	response := `{"e":"response","d":{"k":` + quoteJSON(k) + `,"v":{"7":{},"100":{}}}}`
	wsLines := []string{
		mustJSON(t, map[string]any{
			"ts":          "2026-07-01T12:00:00+08:00",
			"type":        "ws_frame",
			"flow_id":     "1",
			"frame_no":    1,
			"direction":   "client_to_server",
			"url":         "https://hy2gnhf113.babigame.cn:54821/?sgid=2482",
			"opcode_text": "text",
			"length":      len(frame),
			"text":        frame,
		}),
		mustJSON(t, map[string]any{
			"ts":          "2026-07-01T12:00:01+08:00",
			"type":        "ws_frame",
			"flow_id":     "1",
			"frame_no":    2,
			"direction":   "server_to_client",
			"url":         "https://hy2gnhf113.babigame.cn:54821/?sgid=2482",
			"opcode_text": "text",
			"length":      len(response),
			"text":        response,
		}),
	}
	if err := os.WriteFile(filepath.Join(dir, "websocket.jsonl"), []byte(strings.Join(wsLines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flowLines := []string{
		mustJSON(t, map[string]any{"type": "session_start"}),
		mustJSON(t, map[string]any{
			"type":    "http_request",
			"method":  "POST",
			"host":    "hygnhf2.babigame.cn",
			"url":     "https://hygnhf2.babigame.cn/pack/queryPackageConfig",
			"headers": map[string][]string{"User-Agent": {"HuaYuann/1.1.17 (iPhone)"}},
			"body":    map[string]any{"text": `{"version":"1001017","gameVersion":"1.0.0"}`},
		}),
		mustJSON(t, map[string]any{"type": "http_response", "method": "POST", "host": "hygnhf2.babigame.cn", "url": "https://hygnhf2.babigame.cn/gw", "status": 200}),
	}
	if err := os.WriteFile(filepath.Join(dir, "flows.jsonl"), []byte(strings.Join(flowLines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readRPCIndex(t *testing.T, path string) []RPCIndexRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []RPCIndexRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var rec RPCIndexRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		out = append(out, rec)
	}
	return out
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func quoteJSON(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
