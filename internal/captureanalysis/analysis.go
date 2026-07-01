package captureanalysis

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

const (
	RPCIndexFile = "rpc.jsonl"
	ReportFile   = "analysis.json"
)

type RecordWriter interface {
	Write(any)
}

type Options struct {
	Channel babigame.Channel
	Rewrite bool
}

type WSFrame struct {
	TS         string
	FlowID     string
	FrameNo    int64
	Direction  string
	URL        string
	OpcodeText string
	Length     int
	Text       string
	Payload    []byte
}

type RPCIndexRecord struct {
	TS           string   `json:"ts"`
	Type         string   `json:"type"`
	FlowID       string   `json:"flow_id,omitempty"`
	FrameNo      int64    `json:"frame_no,omitempty"`
	Direction    string   `json:"direction,omitempty"`
	URL          string   `json:"url,omitempty"`
	OpcodeText   string   `json:"opcode_text,omitempty"`
	Length       int      `json:"length,omitempty"`
	K            string   `json:"k,omitempty"`
	RPC          string   `json:"rpc,omitempty"`
	ArgShape     string   `json:"arg_shape,omitempty"`
	ArgKeys      []string `json:"arg_keys,omitempty"`
	RoutePresent bool     `json:"route_present,omitempty"`
	Namespaces   []string `json:"namespaces,omitempty"`
	HasError     bool     `json:"has_error,omitempty"`
	ErrorType    int      `json:"error_type,omitempty"`
	BinaryType   string   `json:"binary_type,omitempty"`
	SchemaName   string   `json:"schema_name,omitempty"`
	Keys         []string `json:"keys,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type Decoder struct {
	cfg     babigame.Config
	writer  RecordWriter
	pending map[string]RPCIndexRecord
}

func NewDecoder(channel babigame.Channel, writer RecordWriter) (*Decoder, error) {
	if channel == babigame.ChannelUnspecified {
		channel = babigame.ChannelIOS
	}
	cfg, err := babigame.ConfigForChannel(channel)
	if err != nil {
		return nil, err
	}
	return &Decoder{cfg: cfg, writer: writer, pending: make(map[string]RPCIndexRecord)}, nil
}

func (d *Decoder) ProcessFrame(frame WSFrame) {
	if d == nil || d.writer == nil {
		return
	}
	switch frame.OpcodeText {
	case "text":
		d.processText(frame)
	case "binary":
		d.processBinary(frame)
	}
}

func (d *Decoder) processText(frame WSFrame) {
	text := frame.Text
	if text == "" && len(frame.Payload) > 0 {
		text = string(frame.Payload)
	}
	if text == "" {
		return
	}
	if frame.Direction == "client_to_server" {
		d.processClientText(frame, text)
		return
	}
	if frame.Direction == "server_to_client" {
		d.processServerText(frame, text)
	}
}

func (d *Decoder) processClientText(frame WSFrame, text string) {
	env, err := babigame.ParseTextFrame(text)
	if err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	if env.E != "request" {
		d.writeOtherText(frame, env.E)
		return
	}
	var out babigame.WSEnvelopeOutD
	if err := json.Unmarshal(env.D, &out); err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	clear, err := babigame.GWDecode(out.P.A, d.cfg.GWXorMask)
	if err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	var tuple []json.RawMessage
	if err := json.Unmarshal(clear, &tuple); err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	if len(tuple) == 0 {
		d.writeDecodeError(frame, fmt.Errorf("empty RPC tuple"))
		return
	}
	var rpc string
	if err := json.Unmarshal(tuple[0], &rpc); err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	argShape, argKeys := "", []string(nil)
	if len(tuple) > 1 {
		argShape, argKeys = summarizeArg(tuple[1])
	}
	rec := RPCIndexRecord{
		TS:           frame.TS,
		Type:         "rpc_request",
		FlowID:       frame.FlowID,
		FrameNo:      frame.FrameNo,
		Direction:    frame.Direction,
		URL:          frame.URL,
		OpcodeText:   frame.OpcodeText,
		Length:       frame.Length,
		K:            out.K,
		RPC:          rpc,
		ArgShape:     argShape,
		ArgKeys:      argKeys,
		RoutePresent: len(tuple) > 2 && string(tuple[2]) != "null",
	}
	if out.K != "" {
		d.pending[out.K] = rec
	}
	d.writer.Write(rec)
}

func (d *Decoder) processServerText(frame WSFrame, text string) {
	if text == `"connectionEnabled"` {
		d.writeOtherText(frame, "connectionEnabled")
		return
	}
	env, err := babigame.ParseTextFrame(text)
	if err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	if env.E != "response" {
		d.writeOtherText(frame, env.E)
		return
	}
	var in babigame.WSResponseD
	if err := json.Unmarshal(env.D, &in); err != nil {
		d.writeDecodeError(frame, err)
		return
	}
	req := d.pending[in.K]
	namespaces := namespaceKeys(in.V)
	rec := RPCIndexRecord{
		TS:         frame.TS,
		Type:       "rpc_response",
		FlowID:     frame.FlowID,
		FrameNo:    frame.FrameNo,
		Direction:  frame.Direction,
		URL:        frame.URL,
		OpcodeText: frame.OpcodeText,
		Length:     frame.Length,
		K:          in.K,
		RPC:        req.RPC,
		Namespaces: namespaces,
		HasError:   in.IsError(),
		ErrorType:  in.ErrorType(),
	}
	d.writer.Write(rec)
}

func (d *Decoder) processBinary(frame WSFrame) {
	payload := frame.Payload
	if len(payload) == 0 {
		return
	}
	items := babigame.ParseBinaryFrame(payload)
	if len(items) == 0 {
		return
	}
	for _, item := range items {
		binType, schema, keys := binaryItemSummary(item)
		d.writer.Write(RPCIndexRecord{
			TS:         frame.TS,
			Type:       "ws_binary_item",
			FlowID:     frame.FlowID,
			FrameNo:    frame.FrameNo,
			Direction:  frame.Direction,
			URL:        frame.URL,
			OpcodeText: frame.OpcodeText,
			Length:     frame.Length,
			BinaryType: binType,
			SchemaName: schema,
			Keys:       keys,
		})
	}
}

func (d *Decoder) writeDecodeError(frame WSFrame, err error) {
	d.writer.Write(RPCIndexRecord{
		TS:         frame.TS,
		Type:       "decode_error",
		FlowID:     frame.FlowID,
		FrameNo:    frame.FrameNo,
		Direction:  frame.Direction,
		URL:        frame.URL,
		OpcodeText: frame.OpcodeText,
		Length:     frame.Length,
		Error:      err.Error(),
	})
}

func (d *Decoder) writeOtherText(frame WSFrame, event string) {
	d.writer.Write(RPCIndexRecord{
		TS:         frame.TS,
		Type:       "ws_text_other",
		FlowID:     frame.FlowID,
		FrameNo:    frame.FrameNo,
		Direction:  frame.Direction,
		URL:        frame.URL,
		OpcodeText: frame.OpcodeText,
		Length:     frame.Length,
		BinaryType: event,
	})
}

type wsJSONLRecord struct {
	TS         string `json:"ts"`
	FlowID     string `json:"flow_id"`
	FrameNo    int64  `json:"frame_no"`
	Direction  string `json:"direction"`
	URL        string `json:"url"`
	OpcodeText string `json:"opcode_text"`
	Length     int    `json:"length"`
	Text       string `json:"text"`
	Base64     string `json:"base64"`
}

func BuildRPCIndex(sessionDir string, opts Options) (string, error) {
	rpcPath := filepath.Join(sessionDir, RPCIndexFile)
	if !opts.Rewrite {
		if st, err := os.Stat(rpcPath); err == nil && st.Size() > 0 {
			return rpcPath, nil
		}
	}
	wsPath := filepath.Join(sessionDir, "websocket.jsonl")
	in, err := os.Open(wsPath)
	if err != nil {
		return "", fmt.Errorf("open websocket jsonl: %w", err)
	}
	defer in.Close()
	out, err := newFileWriter(rpcPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	decoder, err := NewDecoder(opts.Channel, out)
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	var fallbackFrameNo int64
	for sc.Scan() {
		var rec wsJSONLRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		payload := []byte(rec.Text)
		if len(payload) == 0 && rec.Base64 != "" {
			payload, _ = base64.StdEncoding.DecodeString(rec.Base64)
		}
		if rec.FrameNo == 0 {
			fallbackFrameNo++
			rec.FrameNo = fallbackFrameNo
		}
		decoder.ProcessFrame(WSFrame{
			TS:         rec.TS,
			FlowID:     rec.FlowID,
			FrameNo:    rec.FrameNo,
			Direction:  rec.Direction,
			URL:        rec.URL,
			OpcodeText: rec.OpcodeText,
			Length:     rec.Length,
			Text:       rec.Text,
			Payload:    payload,
		})
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("scan websocket jsonl: %w", err)
	}
	return rpcPath, nil
}

type Report struct {
	SessionDir string             `json:"session_dir"`
	Generated  string             `json:"generated"`
	Files      map[string]int64   `json:"files"`
	HTTP       HTTPSummary        `json:"http"`
	WS         WSSummary          `json:"ws"`
	Top        map[string][]Count `json:"top"`
}

type HTTPSummary struct {
	Records int `json:"records"`
}

type WSSummary struct {
	IndexRecords    int `json:"index_records"`
	ClientRPCs      int `json:"client_rpcs"`
	ServerResponses int `json:"server_responses"`
	ServerErrors    int `json:"server_errors"`
	DecodeErrors    int `json:"decode_errors"`
	BinaryItems     int `json:"binary_items"`
}

type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func AnalyzeSession(sessionDir string, opts Options) (Report, error) {
	if _, err := BuildRPCIndex(sessionDir, opts); err != nil {
		return Report{}, err
	}
	report := Report{
		SessionDir: sessionDir,
		Generated:  time.Now().Format(time.RFC3339Nano),
		Files:      make(map[string]int64),
		Top:        make(map[string][]Count),
	}
	for _, name := range []string{"flows.jsonl", "websocket.jsonl", RPCIndexFile, "session.json"} {
		if st, err := os.Stat(filepath.Join(sessionDir, name)); err == nil {
			report.Files[name] = st.Size()
		}
	}
	if err := summarizeFlows(sessionDir, &report); err != nil {
		return Report{}, err
	}
	if err := summarizeRPCIndex(sessionDir, &report); err != nil {
		return Report{}, err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Report{}, err
	}
	if err := os.WriteFile(filepath.Join(sessionDir, ReportFile), data, 0o644); err != nil {
		return Report{}, err
	}
	return report, nil
}

type flowRecord struct {
	Type      string `json:"type"`
	Method    string `json:"method"`
	URL       string `json:"url"`
	Host      string `json:"host"`
	Status    int    `json:"status"`
	WebSocket bool   `json:"websocket"`
}

func summarizeFlows(sessionDir string, report *Report) error {
	f, err := os.Open(filepath.Join(sessionDir, "flows.jsonl"))
	if err != nil {
		return fmt.Errorf("open flows jsonl: %w", err)
	}
	defer f.Close()
	flowTypes := make(map[string]int)
	hosts := make(map[string]int)
	babigamePaths := make(map[string]int)
	statuses := make(map[string]int)
	wsUpgrades := make(map[string]int)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var rec flowRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		report.HTTP.Records++
		flowTypes[rec.Type]++
		if rec.Host != "" {
			hosts[rec.Host]++
		}
		if rec.Status != 0 {
			statuses[fmt.Sprintf("%d", rec.Status)]++
		}
		if rec.WebSocket {
			wsUpgrades[scrubURL(rec.URL)]++
		}
		if rec.URL != "" && isBabigameHost(rec.Host) {
			babigamePaths[rec.Method+" "+scrubURL(rec.URL)]++
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan flows jsonl: %w", err)
	}
	report.Top["http_flow_types"] = topCounts(flowTypes, 20)
	report.Top["http_hosts"] = topCounts(hosts, 30)
	report.Top["http_babigame_paths"] = topCounts(babigamePaths, 40)
	report.Top["http_statuses"] = topCounts(statuses, 20)
	report.Top["websocket_upgrades"] = topCounts(wsUpgrades, 20)
	return nil
}

func summarizeRPCIndex(sessionDir string, report *Report) error {
	f, err := os.Open(filepath.Join(sessionDir, RPCIndexFile))
	if err != nil {
		return fmt.Errorf("open rpc index: %w", err)
	}
	defer f.Close()
	recordTypes := make(map[string]int)
	rpcs := make(map[string]int)
	rpcNamespaces := make(map[string]int)
	namespaces := make(map[string]int)
	binaryItems := make(map[string]int)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		var rec RPCIndexRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		report.WS.IndexRecords++
		recordTypes[rec.Type]++
		switch rec.Type {
		case "rpc_request":
			report.WS.ClientRPCs++
			rpcs[rec.RPC]++
		case "rpc_response":
			report.WS.ServerResponses++
			if rec.HasError {
				report.WS.ServerErrors++
			}
			ns := "-"
			if len(rec.Namespaces) > 0 {
				ns = strings.Join(rec.Namespaces, ",")
				for _, key := range rec.Namespaces {
					namespaces[key]++
				}
			}
			rpc := rec.RPC
			if rpc == "" {
				rpc = "(unknown)"
			}
			rpcNamespaces[rpc+" -> "+ns]++
		case "decode_error":
			report.WS.DecodeErrors++
		case "ws_binary_item":
			report.WS.BinaryItems++
			key := binaryCountKey(rec)
			binaryItems[key]++
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan rpc index: %w", err)
	}
	report.Top["ws_index_types"] = topCounts(recordTypes, 20)
	report.Top["rpc_counts"] = topCounts(rpcs, 100)
	report.Top["rpc_namespaces"] = topCounts(rpcNamespaces, 120)
	report.Top["namespaces"] = topCounts(namespaces, 100)
	report.Top["binary_items"] = topCounts(binaryItems, 80)
	return nil
}

func PrintReport(report Report) {
	fmt.Printf("session: %s\n", report.SessionDir)
	fmt.Printf("flows: %d bytes, websocket: %d bytes, rpc index: %d bytes\n",
		report.Files["flows.jsonl"], report.Files["websocket.jsonl"], report.Files[RPCIndexFile])
	fmt.Printf("http records: %d\n", report.HTTP.Records)
	fmt.Printf("client RPCs: %d, responses: %d, server errors: %d, binary items: %d, decode errors: %d\n\n",
		report.WS.ClientRPCs, report.WS.ServerResponses, report.WS.ServerErrors, report.WS.BinaryItems, report.WS.DecodeErrors)
	printTop("HTTP hosts", report.Top["http_hosts"], 12)
	printTop("Babigame paths", report.Top["http_babigame_paths"], 12)
	printTop("RPC counts", report.Top["rpc_counts"], 20)
	printTop("RPC namespaces", report.Top["rpc_namespaces"], 20)
	printTop("Namespaces", report.Top["namespaces"], 20)
	printTop("Binary items", report.Top["binary_items"], 12)
	fmt.Printf("wrote: %s\n", filepath.Join(report.SessionDir, ReportFile))
}

func printTop(title string, counts []Count, limit int) {
	fmt.Println(title + ":")
	if len(counts) > limit {
		counts = counts[:limit]
	}
	for _, c := range counts {
		fmt.Printf("  %5d  %s\n", c.Count, c.Name)
	}
	fmt.Println()
}

func summarizeArg(raw json.RawMessage) (string, []string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "null", nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "object", keys
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return fmt.Sprintf("array[%d]", len(arr)), nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return "string", nil
	}
	return "scalar", nil
}

func namespaceKeys(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var ns map[string]json.RawMessage
	if json.Unmarshal(raw, &ns) != nil {
		return nil
	}
	keys := make([]string, 0, len(ns))
	for key := range ns {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func binaryItemSummary(raw json.RawMessage) (string, string, []string) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return "non_object", "", nil
	}
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	binType, schema := "", ""
	if rawType, ok := obj["t"]; ok {
		_ = json.Unmarshal(rawType, &binType)
	}
	if rawSchema, ok := obj["sn"]; ok {
		_ = json.Unmarshal(rawSchema, &schema)
	}
	return binType, schema, keys
}

func binaryCountKey(rec RPCIndexRecord) string {
	if rec.BinaryType != "" && rec.SchemaName != "" {
		return "t=" + rec.BinaryType + " sn=" + rec.SchemaName
	}
	if rec.BinaryType != "" {
		return "t=" + rec.BinaryType
	}
	if len(rec.Keys) > 0 {
		return "keys=" + strings.Join(rec.Keys, ",")
	}
	return "unknown"
}

func isBabigameHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ":443")
	return host == "babigame.cn" || strings.HasSuffix(host, ".babigame.cn")
}

func scrubURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme == "" && u.Host == "" {
		return u.Path
	}
	return u.Scheme + "://" + u.Host + u.Path
}

func topCounts(m map[string]int, limit int) []Count {
	out := make([]Count, 0, len(m))
	for name, count := range m {
		out = append(out, Count{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

type fileWriter struct {
	file *os.File
}

func newFileWriter(path string) (*fileWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &fileWriter{file: f}, nil
}

func (w *fileWriter) Write(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = w.file.Write(append(data, '\n'))
}

func (w *fileWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	return w.file.Close()
}
