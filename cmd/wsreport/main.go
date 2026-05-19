package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const xorMask byte = 0x77

type captureFrame struct {
	TS        string          `json:"ts"`
	FlowID    string          `json:"flow_id"`
	Host      string          `json:"host"`
	Path      string          `json:"path"`
	Direction string          `json:"direction"`
	Opcode    string          `json:"opcode"`
	Length    int             `json:"length"`
	SHA256    string          `json:"sha256"`
	KeyHits   []string        `json:"key_hits"`
	Payload   capturePayload  `json:"payload"`
	JSON      json.RawMessage `json:"json"`
}

type capturePayload struct {
	Text   string `json:"text"`
	IsText bool   `json:"is_text"`
	Base64 string `json:"base64"`
}

type wsEnvelope struct {
	E string          `json:"e"`
	D json.RawMessage `json:"d"`
}

type wsRequestD struct {
	R string `json:"r"`
	P struct {
		Sign string `json:"sign"`
		A    string `json:"a"`
		L    string `json:"l"`
	} `json:"p"`
	K string `json:"k"`
}

type wsResponseD struct {
	R json.RawMessage `json:"r,omitempty"`
	V any             `json:"v,omitempty"`
	D any             `json:"d,omitempty"`
	T json.RawMessage `json:"t,omitempty"`
	K string          `json:"k,omitempty"`
	M any             `json:"m,omitempty"`
}

type report struct {
	Source             string                 `json:"source"`
	GeneratedAt        string                 `json:"generatedAt"`
	FrameCount         int                    `json:"frameCount"`
	TransactionCount   int                    `json:"transactionCount"`
	PushCount          int                    `json:"pushCount"`
	TextEventCount     int                    `json:"textEventCount"`
	UnmatchedRequests  int                    `json:"unmatchedRequests"`
	UnmatchedResponses int                    `json:"unmatchedResponses"`
	RPCCounts          map[string]int         `json:"rpcCounts"`
	CategoryCounts     map[string]int         `json:"categoryCounts"`
	NamespaceCounts    map[string]int         `json:"namespaceCounts"`
	NamespaceInfo      map[string]string      `json:"namespaceInfo"`
	Transactions       []*transaction         `json:"transactions"`
	Pushes             []pushEvent            `json:"pushes"`
	TextEvents         []textEvent            `json:"textEvents"`
	ParseErrors        []string               `json:"parseErrors"`
	Metadata           map[string]interface{} `json:"metadata"`
}

type transaction struct {
	K              string         `json:"k"`
	Seq            string         `json:"seq"`
	RPC            string         `json:"rpc"`
	Category       string         `json:"category"`
	RequestTS      string         `json:"requestTs"`
	ResponseTS     string         `json:"responseTs"`
	LatencyMS      *int64         `json:"latencyMs,omitempty"`
	Status         string         `json:"status"`
	Namespaces     []string       `json:"namespaces"`
	Request        *requestInfo   `json:"request,omitempty"`
	Responses      []responseInfo `json:"responses"`
	AnnotationID   string         `json:"annotationId"`
	SearchHaystack string         `json:"searchHaystack"`
}

type requestInfo struct {
	FrameIndex int    `json:"frameIndex"`
	TS         string `json:"ts"`
	Route      string `json:"route"`
	K          string `json:"k"`
	Sign       string `json:"sign"`
	Lang       string `json:"lang"`
	RPC        string `json:"rpc"`
	Category   string `json:"category"`
	Args       any    `json:"args"`
	RouteArg   any    `json:"routeArg"`
	Decoded    any    `json:"decoded"`
	DecodeText string `json:"decodeText"`
	Error      string `json:"error,omitempty"`
}

type responseInfo struct {
	FrameIndex      int               `json:"frameIndex"`
	TS              string            `json:"ts"`
	K               string            `json:"k"`
	Namespaces      []string          `json:"namespaces"`
	NamespaceBriefs map[string]string `json:"namespaceBriefs"`
	V               any               `json:"v"`
	D               any               `json:"d"`
	M               any               `json:"m"`
	R               any               `json:"r"`
	T               any               `json:"t"`
	Empty           bool              `json:"empty"`
	Error           string            `json:"error,omitempty"`
}

type pushEvent struct {
	FrameIndex int    `json:"frameIndex"`
	TS         string `json:"ts"`
	Direction  string `json:"direction"`
	Opcode     string `json:"opcode"`
	Length     int    `json:"length"`
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	Objects    []any  `json:"objects"`
}

type textEvent struct {
	FrameIndex int    `json:"frameIndex"`
	TS         string `json:"ts"`
	Direction  string `json:"direction"`
	Length     int    `json:"length"`
	Text       string `json:"text"`
}

type pageData struct {
	DataJSON template.JS
}

func main() {
	in := flag.String("in", "", "input websocket_frames.jsonl")
	out := flag.String("out", "", "output HTML path")
	flag.Parse()

	if *in == "" {
		fmt.Fprintln(os.Stderr, "usage: wsreport -in websocket_frames.jsonl [-out report.html]")
		os.Exit(2)
	}
	if *out == "" {
		base := strings.TrimSuffix(filepath.Base(*in), filepath.Ext(*in))
		*out = filepath.Join("debug", "reports", base+"_report.html")
	}

	rep, err := buildReport(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeHTML(*out, rep); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(*out)
}

func buildReport(path string) (*report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rep := &report{
		Source:          path,
		GeneratedAt:     time.Now().Format(time.RFC3339),
		RPCCounts:       map[string]int{},
		CategoryCounts:  map[string]int{},
		NamespaceCounts: map[string]int{},
		NamespaceInfo:   namespaceDescriptions(),
		Metadata:        map[string]interface{}{},
	}
	byK := map[string]*transaction{}
	order := []string{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var cf captureFrame
		if err := json.Unmarshal(line, &cf); err != nil {
			rep.ParseErrors = append(rep.ParseErrors, fmt.Sprintf("line %d: %v", lineNo, err))
			continue
		}
		rep.FrameCount++
		if cf.Host != "" {
			rep.Metadata["host"] = cf.Host
		}
		if cf.Path != "" {
			rep.Metadata["path"] = cf.Path
		}

		if cf.Opcode == "binary" {
			rep.Pushes = append(rep.Pushes, parsePush(lineNo, cf))
			continue
		}
		if !cf.Payload.IsText {
			rep.Pushes = append(rep.Pushes, parsePush(lineNo, cf))
			continue
		}
		text := cf.Payload.Text
		raw := strings.TrimPrefix(text, "$#|#$")
		var env wsEnvelope
		if err := decodeJSON([]byte(raw), &env); err != nil {
			rep.TextEvents = append(rep.TextEvents, textEvent{FrameIndex: lineNo, TS: cf.TS, Direction: cf.Direction, Length: cf.Length, Text: text})
			continue
		}
		switch env.E {
		case "request":
			req, err := parseRequest(lineNo, cf, env.D)
			if err != nil {
				rep.ParseErrors = append(rep.ParseErrors, fmt.Sprintf("line %d request: %v", lineNo, err))
				continue
			}
			tx := ensureTx(byK, &order, req.K)
			tx.Request = req
			tx.K = req.K
			tx.RPC = req.RPC
			tx.Category = req.Category
			tx.RequestTS = req.TS
			tx.Seq = keySeq(req.K)
		case "response":
			resp, err := parseResponse(lineNo, cf, env.D)
			if err != nil {
				rep.ParseErrors = append(rep.ParseErrors, fmt.Sprintf("line %d response: %v", lineNo, err))
				continue
			}
			tx := ensureTx(byK, &order, resp.K)
			tx.Responses = append(tx.Responses, resp)
			if tx.ResponseTS == "" {
				tx.ResponseTS = resp.TS
			}
			tx.Namespaces = mergeStrings(tx.Namespaces, resp.Namespaces)
		default:
			rep.TextEvents = append(rep.TextEvents, textEvent{FrameIndex: lineNo, TS: cf.TS, Direction: cf.Direction, Length: cf.Length, Text: text})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	for _, k := range order {
		tx := byK[k]
		if tx.Request == nil {
			rep.UnmatchedResponses++
			tx.RPC = "(missing request)"
			tx.Category = "unmatched"
		}
		if len(tx.Responses) == 0 {
			rep.UnmatchedRequests++
			tx.Status = "pending"
		} else if tx.Request == nil {
			tx.Status = "orphan-response"
		} else {
			tx.Status = "matched"
		}
		if tx.Request != nil && len(tx.Responses) > 0 {
			if ms, ok := latencyMS(tx.Request.TS, tx.Responses[0].TS); ok {
				tx.LatencyMS = &ms
			}
		}
		if tx.RPC == "" {
			tx.RPC = "(unknown)"
		}
		if tx.Category == "" {
			tx.Category = categoryForRPC(tx.RPC)
		}
		tx.AnnotationID = "tx:" + tx.K
		tx.SearchHaystack = searchText(tx)
		rep.Transactions = append(rep.Transactions, tx)
		rep.RPCCounts[tx.RPC]++
		rep.CategoryCounts[tx.Category]++
		for _, ns := range tx.Namespaces {
			rep.NamespaceCounts[ns]++
		}
	}
	sort.Slice(rep.Transactions, func(i, j int) bool {
		return rep.Transactions[i].RequestTS < rep.Transactions[j].RequestTS
	})
	rep.TransactionCount = len(rep.Transactions)
	rep.PushCount = len(rep.Pushes)
	rep.TextEventCount = len(rep.TextEvents)
	return rep, nil
}

func ensureTx(m map[string]*transaction, order *[]string, k string) *transaction {
	if k == "" {
		k = "(empty-key)"
	}
	if tx, ok := m[k]; ok {
		return tx
	}
	tx := &transaction{K: k}
	m[k] = tx
	*order = append(*order, k)
	return tx
}

func parseRequest(lineNo int, cf captureFrame, raw json.RawMessage) (*requestInfo, error) {
	var d wsRequestD
	if err := decodeJSON(raw, &d); err != nil {
		return nil, err
	}
	decodedText, decoded, err := decodeGWArray(d.P.A)
	req := &requestInfo{
		FrameIndex: lineNo,
		TS:         cf.TS,
		Route:      d.R,
		K:          d.K,
		Sign:       d.P.Sign,
		Lang:       d.P.L,
		Decoded:    decoded,
		DecodeText: decodedText,
	}
	if err != nil {
		req.Error = err.Error()
		return req, nil
	}
	if arr, ok := decoded.([]any); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			req.RPC = s
		}
		if len(arr) > 1 {
			req.Args = arr[1]
		}
		if len(arr) > 2 {
			req.RouteArg = arr[2]
		}
	}
	if req.RPC == "" {
		req.RPC = "(unknown)"
	}
	req.Category = categoryForRPC(req.RPC)
	return req, nil
}

func parseResponse(lineNo int, cf captureFrame, raw json.RawMessage) (responseInfo, error) {
	var d wsResponseD
	if err := decodeJSON(raw, &d); err != nil {
		return responseInfo{}, err
	}
	resp := responseInfo{
		FrameIndex:      lineNo,
		TS:              cf.TS,
		K:               d.K,
		V:               d.V,
		D:               d.D,
		M:               d.M,
		R:               rawJSONValue(d.R),
		T:               rawJSONValue(d.T),
		NamespaceBriefs: map[string]string{},
	}
	resp.Namespaces = namespaceKeys(d.V)
	for _, ns := range resp.Namespaces {
		resp.NamespaceBriefs[ns] = valueBrief(getObjectValue(d.V, ns))
	}
	resp.Empty = len(resp.Namespaces) == 0 && isEmptyValue(d.V)
	resp.Error = errorMessage(d.M)
	return resp, nil
}

func parsePush(lineNo int, cf captureFrame) pushEvent {
	ev := pushEvent{FrameIndex: lineNo, TS: cf.TS, Direction: cf.Direction, Opcode: cf.Opcode, Length: cf.Length}
	if cf.Opcode == "binary" {
		raw, err := base64.StdEncoding.DecodeString(cf.Payload.Base64)
		if err != nil {
			ev.Kind = "binary-decode-error"
			ev.Summary = err.Error()
			return ev
		}
		objs := scanEmbeddedJSON(raw)
		ev.Objects = objs
		ev.Kind, ev.Summary = summarizePush(objs)
		return ev
	}
	ev.Kind = "text-event"
	ev.Summary = cf.Payload.Text
	ev.Objects = []any{cf.Payload.Text}
	return ev
}

func decodeGWArray(encoded string) (string, any, error) {
	var nums []int
	if err := json.Unmarshal([]byte(encoded), &nums); err != nil {
		return "", nil, fmt.Errorf("parse encoded array: %w", err)
	}
	runes := make([]rune, 0, len(nums))
	for _, n := range nums {
		runes = append(runes, rune(byte(n)^xorMask))
	}
	text := string(runes)
	var decoded any
	if err := decodeJSON([]byte(text), &decoded); err != nil {
		return text, nil, fmt.Errorf("decode request json: %w", err)
	}
	return text, decoded, nil
}

func scanEmbeddedJSON(raw []byte) []any {
	out := []any{}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' && raw[i] != '[' {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(raw[i:]))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			continue
		}
		out = append(out, v)
		i += int(dec.InputOffset()) - 1
	}
	return out
}

func summarizePush(objs []any) (string, string) {
	if len(objs) == 0 {
		return "binary", "no embedded JSON objects"
	}
	labels := []string{}
	for _, obj := range objs {
		if m, ok := obj.(map[string]any); ok {
			if t, ok := m["t"].(string); ok {
				labels = append(labels, t)
				continue
			}
			if sn, ok := m["sn"].(string); ok {
				labels = append(labels, sn)
				continue
			}
			keys := mapKeys(m)
			labels = append(labels, strings.Join(keys, ","))
		}
	}
	kind := strings.Join(labels, " / ")
	if strings.Contains(kind, "im_onMsg") {
		return "im_onMsg", briefJSONString(objs)
	}
	if strings.Contains(kind, "sysMsg") {
		return "sysMsg", briefJSONString(objs)
	}
	return kind, briefJSONString(objs)
}

func categoryForRPC(rpc string) string {
	switch {
	case rpc == "index.login":
		return "login/bootstrap"
	case strings.HasPrefix(rpc, "usrLand."):
		return "land"
	case strings.HasPrefix(rpc, "cultivate."):
		return "cultivation"
	case strings.HasPrefix(rpc, "waterwheel."):
		return "waterwheel"
	case strings.HasPrefix(rpc, "freeWater."):
		return "free-water"
	case strings.HasPrefix(rpc, "taskDly.") || strings.HasPrefix(rpc, "taskMain.") || strings.HasPrefix(rpc, "taskWeek."):
		return "tasks"
	case strings.HasPrefix(rpc, "order"):
		return "orders"
	case strings.HasPrefix(rpc, "im."):
		return "chat"
	case strings.HasPrefix(rpc, "frd."):
		return "friends"
	case strings.HasPrefix(rpc, "mail."):
		return "mail"
	case strings.HasPrefix(rpc, "act.") || strings.HasPrefix(rpc, "randomEvent."):
		return "activity"
	case strings.HasPrefix(rpc, "sdk.") || strings.HasPrefix(rpc, "rchg") || strings.HasPrefix(rpc, "sign."):
		return "reward/payment"
	case strings.HasPrefix(rpc, "usr."):
		return "user"
	default:
		return "other"
	}
}

func namespaceDescriptions() map[string]string {
	return map[string]string{
		"2":   "server config / timezone / session",
		"3":   "recharge state",
		"6":   "config versions / split map",
		"7":   "inventory, currencies, water drops",
		"16":  "VIP state",
		"19":  "mail list",
		"20":  "shop purchase records",
		"21":  "unknown captured bootstrap state",
		"22":  "daily/weekly task progress",
		"23":  "activity stats",
		"24":  "friend list",
		"27":  "IM channel",
		"28":  "friend/apply/blacklist adjunct",
		"31":  "share state",
		"100": "land state",
		"101": "cultivation state",
		"103": "collection/cultivation rewards",
		"105": "flower orders",
		"109": "customer orders",
		"112": "gift bag shop",
		"114": "waterwheel state",
		"117": "free-water reward state",
		"119": "high-frequency task counters",
		"124": "daily summary / popup rewards",
		"129": "random event state",
		"130": "cultivation and art rewards",
		"139": "activity/reward config state",
		"155": "unknown captured lazySync state",
		"161": "activity visibility/config list",
	}
}

func namespaceKeys(v any) []string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	keys := mapKeys(m)
	sort.Slice(keys, func(i, j int) bool {
		ai, ei := strconv.Atoi(keys[i])
		aj, ej := strconv.Atoi(keys[j])
		if ei == nil && ej == nil {
			return ai < aj
		}
		return keys[i] < keys[j]
	})
	return keys
}

func mergeStrings(dst []string, src []string) []string {
	seen := map[string]bool{}
	for _, v := range dst {
		seen[v] = true
	}
	for _, v := range src {
		if !seen[v] {
			dst = append(dst, v)
			seen[v] = true
		}
	}
	sort.Slice(dst, func(i, j int) bool {
		ai, ei := strconv.Atoi(dst[i])
		aj, ej := strconv.Atoi(dst[j])
		if ei == nil && ej == nil {
			return ai < aj
		}
		return dst[i] < dst[j]
	})
	return dst
}

func decodeJSON(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}

func rawJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := decodeJSON(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

func getObjectValue(v any, key string) any {
	if m, ok := v.(map[string]any); ok {
		return m[key]
	}
	return nil
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	if m, ok := v.(map[string]any); ok {
		return len(m) == 0
	}
	if a, ok := v.([]any); ok {
		return len(a) == 0
	}
	return false
}

func errorMessage(v any) string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return ""
	}
	if msg, ok := m["msg"].(string); ok {
		return msg
	}
	return briefJSONString(v)
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func keySeq(k string) string {
	parts := strings.Split(k, "|")
	if len(parts) == 3 {
		return parts[2]
	}
	return ""
}

func latencyMS(a, b string) (int64, bool) {
	ta, errA := parseCaptureTime(a)
	tb, errB := parseCaptureTime(b)
	if errA != nil || errB != nil {
		return 0, false
	}
	return tb.Sub(ta).Milliseconds(), true
}

func parseCaptureTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown time layout: %s", s)
}

func valueBrief(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return fmt.Sprintf("object: %d keys [%s]", len(x), strings.Join(limitStrings(mapKeys(x), 8), ", "))
	case []any:
		return fmt.Sprintf("array: %d items", len(x))
	case string:
		if len([]rune(x)) > 80 {
			return string([]rune(x)[:80]) + "..."
		}
		return strconv.Quote(x)
	default:
		return briefJSONString(x)
	}
}

func briefJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	s := string(b)
	if len([]rune(s)) > 180 {
		return string([]rune(s)[:180]) + "..."
	}
	return s
}

func limitStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	out := append([]string{}, in[:n]...)
	out = append(out, "...")
	return out
}

func searchText(tx *transaction) string {
	parts := []string{tx.K, tx.RPC, tx.Category, strings.Join(tx.Namespaces, " ")}
	if tx.Request != nil {
		parts = append(parts, briefJSONString(tx.Request.Args))
	}
	for _, resp := range tx.Responses {
		parts = append(parts, strings.Join(resp.Namespaces, " "), resp.Error)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func writeHTML(path string, rep *report) error {
	raw, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	tpl := template.Must(template.New("report").Parse(htmlTemplate))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tpl.Execute(f, pageData{DataJSON: template.JS(raw)})
}

const htmlTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>WebSocket Capture Report</title>
  <style>
    :root {
      --bg: #f7f8fa;
      --panel: #ffffff;
      --ink: #18202a;
      --muted: #667085;
      --line: #d9dee7;
      --accent: #167761;
      --accent-2: #9a5b16;
      --danger: #b42318;
      --code: #0f172a;
      --chip: #eef4ff;
    }
    * { box-sizing: border-box; }
    body { margin: 0; background: var(--bg); color: var(--ink); font: 14px/1.48 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    header { position: sticky; top: 0; z-index: 5; background: rgba(247,248,250,.94); border-bottom: 1px solid var(--line); backdrop-filter: blur(10px); }
    .wrap { max-width: 1440px; margin: 0 auto; padding: 18px 24px; }
    .topline { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; }
    h1 { margin: 0 0 4px; font-size: 22px; line-height: 1.2; }
    .source { color: var(--muted); overflow-wrap: anywhere; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 12px; }
    .toolbar { display: grid; grid-template-columns: minmax(220px, 1fr) 170px 170px 170px auto auto; gap: 10px; margin-top: 14px; align-items: center; }
    input, select, textarea, button { font: inherit; }
    input, select, textarea { width: 100%; border: 1px solid var(--line); background: #fff; color: var(--ink); border-radius: 6px; padding: 9px 10px; }
    button { border: 1px solid var(--line); background: #fff; color: var(--ink); border-radius: 6px; padding: 9px 12px; cursor: pointer; }
    button:hover { border-color: #9aa4b2; }
    main.wrap { padding-top: 20px; }
    .stats { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; margin-bottom: 18px; }
    .stat { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 12px; }
    .stat b { display: block; font-size: 22px; line-height: 1.1; }
    .stat span { color: var(--muted); font-size: 12px; }
    .tabs { display: flex; gap: 8px; margin: 8px 0 14px; flex-wrap: wrap; }
    .tab { border: 1px solid var(--line); background: #fff; border-radius: 6px; padding: 8px 11px; cursor: pointer; }
    .tab.active { background: var(--accent); color: #fff; border-color: var(--accent); }
    .section { display: none; }
    .section.active { display: block; }
    .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; align-items: start; }
    .panel, .tx, .push { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; margin-bottom: 12px; }
    .panel { padding: 14px; }
    .tx-head, .push-head { padding: 12px 14px; border-bottom: 1px solid var(--line); display: grid; grid-template-columns: 1fr auto; gap: 12px; align-items: start; }
    .tx-body, .push-body { padding: 14px; }
    .title { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
    .rpc { font-weight: 700; font-size: 15px; }
    .muted { color: var(--muted); }
    .mono { font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 12px; }
    .badge, .ns { display: inline-flex; align-items: center; border-radius: 999px; padding: 2px 8px; font-size: 12px; border: 1px solid var(--line); background: #fff; white-space: nowrap; }
    .badge.cat { background: #ecfdf3; border-color: #abefc6; color: #067647; }
    .badge.warn { background: #fffaeb; border-color: #fedf89; color: #93370d; }
    .badge.err { background: #fef3f2; border-color: #fecdca; color: var(--danger); }
    .ns { background: var(--chip); border-color: #c7d7fe; color: #344054; margin: 2px; }
    .chips { margin-top: 7px; }
    details { border: 1px solid var(--line); border-radius: 8px; margin-top: 10px; background: #fff; }
    summary { padding: 9px 10px; cursor: pointer; color: #344054; }
    pre { margin: 0; padding: 12px; overflow: auto; max-height: 520px; background: var(--code); color: #e2e8f0; border-radius: 0 0 8px 8px; font-size: 12px; line-height: 1.45; }
    table { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid var(--line); border-radius: 8px; overflow: hidden; }
    th, td { padding: 9px 10px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: top; }
    th { color: #344054; background: #f2f4f7; font-size: 12px; }
    .anno { display: grid; grid-template-columns: 170px 1fr; gap: 10px; margin-top: 10px; }
    .anno textarea { min-height: 38px; resize: vertical; }
    .small { font-size: 12px; }
    .empty { padding: 30px; text-align: center; color: var(--muted); background: #fff; border: 1px dashed var(--line); border-radius: 8px; }
    @media (max-width: 980px) {
      .toolbar { grid-template-columns: 1fr 1fr; }
      .stats { grid-template-columns: repeat(2, 1fr); }
      .grid2 { grid-template-columns: 1fr; }
      .topline { display: block; }
    }
  </style>
</head>
<body>
  <header>
    <div class="wrap">
      <div class="topline">
        <div>
          <h1>WebSocket Capture Report</h1>
          <div class="source" id="source"></div>
        </div>
        <div class="muted small" id="generated"></div>
      </div>
      <div class="toolbar">
        <input id="q" placeholder="搜索 RPC、k、参数、namespace" />
        <select id="cat"></select>
        <select id="rpc"></select>
        <select id="ns"></select>
        <button id="exportAnno">导出标注</button>
        <button id="clearFilters">清空筛选</button>
      </div>
    </div>
  </header>
  <main class="wrap">
    <div class="stats" id="stats"></div>
    <div class="tabs">
      <button class="tab active" data-tab="txs">RPC 关联</button>
      <button class="tab" data-tab="namespaces">Namespace</button>
      <button class="tab" data-tab="pushes">Push 事件</button>
      <button class="tab" data-tab="errors">解析问题</button>
    </div>
    <section class="section active" id="txs"><div id="txList"></div></section>
    <section class="section" id="namespaces"><div class="grid2"><div id="nsTable"></div><div id="rpcTable"></div></div></section>
    <section class="section" id="pushes"><div id="pushList"></div></section>
    <section class="section" id="errors"><div id="errorList"></div></section>
  </main>
  <script id="report-data" type="application/json">{{ .DataJSON }}</script>
  <script>
    const data = JSON.parse(document.getElementById('report-data').textContent);
    const annoKey = 'wsreport-annotations:' + data.source;
    let annotations = JSON.parse(localStorage.getItem(annoKey) || '{}');
    const $ = (id) => document.getElementById(id);
    const esc = (s) => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    const pretty = (v) => esc(JSON.stringify(v, null, 2));
    const entries = (obj) => Object.entries(obj || {}).sort((a,b) => String(a[0]).localeCompare(String(b[0]), undefined, {numeric:true}));

    $('source').textContent = data.source;
    $('generated').textContent = 'generated ' + data.generatedAt;
    $('stats').innerHTML = [
      ['Frames', data.frameCount],
      ['RPC pairs', data.transactionCount],
      ['Push events', data.pushCount],
      ['Text events', data.textEventCount],
      ['Unmatched req', data.unmatchedRequests],
      ['Unmatched resp', data.unmatchedResponses],
    ].map(([k,v]) => '<div class="stat"><b>' + esc(v) + '</b><span>' + esc(k) + '</span></div>').join('');

    function fillSelect(id, label, values) {
      $(id).innerHTML = '<option value="">' + esc(label) + '</option>' + values.map(v => '<option value="' + esc(v) + '">' + esc(v) + '</option>').join('');
    }
    fillSelect('cat', '全部分类', entries(data.categoryCounts).map(([k]) => k));
    fillSelect('rpc', '全部 RPC', entries(data.rpcCounts).map(([k,c]) => k + ' (' + c + ')'));
    [...$('rpc').options].forEach(o => { if (o.value) o.value = o.value.replace(/ \(\d+\)$/, ''); });
    fillSelect('ns', '全部 Namespace', entries(data.namespaceCounts).map(([k,c]) => k + ' (' + c + ')'));
    [...$('ns').options].forEach(o => { if (o.value) o.value = o.value.replace(/ \(\d+\)$/, ''); });

    document.querySelectorAll('.tab').forEach(btn => btn.addEventListener('click', () => {
      document.querySelectorAll('.tab').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.section').forEach(s => s.classList.remove('active'));
      btn.classList.add('active');
      $(btn.dataset.tab).classList.add('active');
    }));
    ['q','cat','rpc','ns'].forEach(id => $(id).addEventListener('input', renderTransactions));
    $('clearFilters').addEventListener('click', () => { ['q','cat','rpc','ns'].forEach(id => $(id).value = ''); renderTransactions(); });
    $('exportAnno').addEventListener('click', () => {
      const blob = new Blob([JSON.stringify(annotations, null, 2)], {type:'application/json'});
      const a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = 'wsreport-annotations.json';
      a.click();
      URL.revokeObjectURL(a.href);
    });

    function txMatches(tx) {
      const q = $('q').value.trim().toLowerCase();
      return (!q || tx.searchHaystack.includes(q) || JSON.stringify(annotations[tx.annotationId] || {}).toLowerCase().includes(q))
        && (!$('cat').value || tx.category === $('cat').value)
        && (!$('rpc').value || tx.rpc === $('rpc').value)
        && (!$('ns').value || (tx.namespaces || []).includes($('ns').value));
    }

    function renderTransactions() {
      const txs = data.transactions.filter(txMatches);
      $('txList').innerHTML = txs.length ? txs.map(renderTx).join('') : '<div class="empty">没有匹配的 RPC</div>';
      bindAnnotations();
    }

    function renderTx(tx) {
      const anno = annotations[tx.annotationId] || {};
      const latency = tx.latencyMs == null ? '' : '<span class="badge">' + esc(tx.latencyMs) + ' ms</span>';
      const statusClass = tx.status === 'matched' ? '' : ' warn';
      const ns = (tx.namespaces || []).map(n => '<span class="ns" title="' + esc(data.namespaceInfo[n] || '') + '">' + esc(n) + '</span>').join('');
      const responses = (tx.responses || []).map(r => {
        const responseNS = (r.namespaces || []).map(n => '<span class="ns">' + esc(n) + '</span>').join('');
        const err = r.error ? '<div class="panel"><span class="badge err">' + esc(r.error) + '</span></div>' : '';
        const briefs = Object.keys(r.namespaceBriefs || {}).length
          ? '<div class="panel small">' + entries(r.namespaceBriefs).map(([k,v]) => '<div><b>' + esc(k) + '</b> ' + esc(data.namespaceInfo[k] || '') + '<br><span class="muted">' + esc(v) + '</span></div>').join('<hr>') + '</div>'
          : '';
        return '<details><summary>Response frame #' + esc(r.frameIndex) + ' ' + (r.empty ? '(empty v)' : '') + ' ' + responseNS + '</summary>' +
          err + briefs + '<pre>' + pretty({v:r.v,d:r.d,m:r.m,r:r.r,t:r.t}) + '</pre></details>';
      }).join('');
      const reqFrame = tx.request && tx.request.frameIndex != null ? tx.request.frameIndex : '-';
      const reqRPC = tx.request ? tx.request.rpc : null;
      const reqArgs = tx.request ? tx.request.args : null;
      const reqRouteArg = tx.request ? tx.request.routeArg : null;
      const reqDecoded = tx.request ? tx.request.decoded : null;
      return '<article class="tx">' +
        '<div class="tx-head">' +
          '<div>' +
            '<div class="title">' +
              '<span class="rpc">' + esc(tx.rpc) + '</span>' +
              '<span class="badge cat">' + esc(tx.category) + '</span>' +
              '<span class="badge' + statusClass + '">' + esc(tx.status) + '</span>' +
              latency +
            '</div>' +
            '<div class="mono muted">' + esc(tx.k) + ' ' + (tx.seq ? 'seq=' + esc(tx.seq) : '') + '</div>' +
            '<div class="chips">' + ns + '</div>' +
          '</div>' +
          '<div class="small muted">request #' + esc(reqFrame) + '<br>response #' + esc((tx.responses || []).map(r => r.frameIndex).join(', ') || '-') + '</div>' +
        '</div>' +
        '<div class="tx-body">' +
          '<div class="anno">' +
            '<input data-anno="' + esc(tx.annotationId) + '" data-field="label" placeholder="标注: 真实含义 / 场景" value="' + esc(anno.label || '') + '" />' +
            '<textarea data-anno="' + esc(tx.annotationId) + '" data-field="note" placeholder="备注: 参数含义、返回字段、待验证点">' + esc(anno.note || '') + '</textarea>' +
          '</div>' +
          '<div class="grid2">' +
            '<details open><summary>Request args</summary><pre>' + pretty({rpc:reqRPC,args:reqArgs,routeArg:reqRouteArg,decoded:reqDecoded}) + '</pre></details>' +
            '<div>' + (responses || '<details open><summary>Response</summary><pre>{}</pre></details>') + '</div>' +
          '</div>' +
        '</div>' +
      '</article>';
    }

    function bindAnnotations() {
      document.querySelectorAll('[data-anno]').forEach(el => el.addEventListener('input', () => {
        const id = el.dataset.anno;
        annotations[id] = annotations[id] || {};
        annotations[id][el.dataset.field] = el.value;
        localStorage.setItem(annoKey, JSON.stringify(annotations));
      }));
    }

    function renderTables() {
      $('nsTable').innerHTML = table('Namespace 统计', ['Namespace','次数','已知含义'], entries(data.namespaceCounts).map(([k,c]) => [k,c,data.namespaceInfo[k] || 'unknown']));
      $('rpcTable').innerHTML = table('RPC 统计', ['RPC','次数','分类'], entries(data.rpcCounts).map(([k,c]) => [k,c,(data.transactions.find(t => t.rpc === k) || {}).category || '']));
    }

    function table(title, heads, rows) {
      return '<div class="panel"><h2 style="margin:0 0 10px;font-size:16px">' + esc(title) + '</h2><table><thead><tr>' +
        heads.map(h => '<th>' + esc(h) + '</th>').join('') +
        '</tr></thead><tbody>' +
        rows.map(r => '<tr>' + r.map(c => '<td>' + esc(c) + '</td>').join('') + '</tr>').join('') +
        '</tbody></table></div>';
    }

    function renderPushes() {
      $('pushList').innerHTML = data.pushes.length ? data.pushes.map(p =>
        '<article class="push">' +
          '<div class="push-head"><div><span class="badge cat">' + esc(p.kind) + '</span> <span class="mono muted">frame #' + esc(p.frameIndex) + '</span></div><div class="muted small">' + esc(p.ts) + ' · ' + esc(p.length) + ' bytes</div></div>' +
          '<div class="push-body">' +
            '<div class="muted small">' + esc(p.summary) + '</div>' +
            '<details><summary>Embedded JSON</summary><pre>' + pretty(p.objects) + '</pre></details>' +
          '</div>' +
        '</article>').join('') : '<div class="empty">没有 push 事件</div>';
    }

    function renderErrors() {
      const errors = data.parseErrors || [];
      $('errorList').innerHTML = errors.length ? '<pre>' + esc(errors.join('\n')) + '</pre>' : '<div class="empty">没有解析错误</div>';
    }

    renderTransactions();
    renderTables();
    renderPushes();
    renderErrors();
  </script>
</body>
</html>`
