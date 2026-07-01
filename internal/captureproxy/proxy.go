package captureproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/captureanalysis"
	"github.com/elazarl/goproxy"
)

// Proxy owns one capture session.
type Proxy struct {
	opts       Options
	sessionDir string
	ca         caFiles
	flowLog    *jsonlWriter
	wsLog      *jsonlWriter
	rpcLog     *jsonlWriter
	decoder    *captureanalysis.Decoder
	metaPath   string
	filter     hostFilter

	server *http.Server
}

// New creates a capture proxy session and prepares its output files.
func New(opts Options) (*Proxy, error) {
	opts = opts.Normalize()
	now := time.Now()
	sessionDir := joinPath(opts.OutDir, sessionSlug(opts.SessionName, now))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir session dir: %w", err)
	}
	ca, err := ensureCA(filepath.Join(opts.OutDir, "ca"))
	if err != nil {
		return nil, err
	}
	flowLog, err := newJSONLWriter(filepath.Join(sessionDir, "flows.jsonl"))
	if err != nil {
		return nil, err
	}
	wsLog, err := newJSONLWriter(filepath.Join(sessionDir, "websocket.jsonl"))
	if err != nil {
		_ = flowLog.Close()
		return nil, err
	}
	rpcLog, err := newJSONLWriter(filepath.Join(sessionDir, captureanalysis.RPCIndexFile))
	if err != nil {
		_ = flowLog.Close()
		_ = wsLog.Close()
		return nil, err
	}
	decoder, err := captureanalysis.NewDecoder("", rpcLog)
	if err != nil {
		_ = flowLog.Close()
		_ = wsLog.Close()
		_ = rpcLog.Close()
		return nil, err
	}
	p := &Proxy{
		opts:       opts,
		sessionDir: sessionDir,
		ca:         ca,
		flowLog:    flowLog,
		wsLog:      wsLog,
		rpcLog:     rpcLog,
		decoder:    decoder,
		metaPath:   filepath.Join(sessionDir, "session.json"),
		filter:     newHostFilter(opts.HostPatterns),
	}
	if err := p.writeMetadata("created", ""); err != nil {
		_ = flowLog.Close()
		_ = wsLog.Close()
		_ = rpcLog.Close()
		return nil, err
	}
	return p, nil
}

func (p *Proxy) SessionDir() string { return p.sessionDir }
func (p *Proxy) CACertPath() string { return p.ca.CertPath }
func (p *Proxy) ListenAddr() string {
	if p.server != nil {
		return p.server.Addr
	}
	return p.opts.Listen
}

// Run starts the proxy and blocks until ctx is canceled or the server fails.
func (p *Proxy) Run(ctx context.Context) error {
	gp := goproxy.NewProxyHttpServer()
	gp.Verbose = p.opts.Verbose
	gp.Logger = log.New(io.Discard, "", 0)
	if p.opts.Verbose {
		gp.Logger = log.New(os.Stderr, "gardencap proxy: ", log.LstdFlags)
	}
	gp.KeepAcceptEncoding = true
	gp.Tr.DisableCompression = true
	gp.CertStore = newMemoryCertStore()

	mitmAction := &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&p.ca.Cert),
	}
	gp.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if p.filter.Match(host) {
			p.flowLog.Write(map[string]any{
				"ts":      time.Now().Format(time.RFC3339Nano),
				"type":    "connect_mitm",
				"flow_id": fmt.Sprintf("%d", ctx.Session),
				"host":    host,
			})
			return mitmAction, host
		}
		p.flowLog.Write(map[string]any{
			"ts":      time.Now().Format(time.RFC3339Nano),
			"type":    "connect_tunnel",
			"flow_id": fmt.Sprintf("%d", ctx.Session),
			"host":    host,
		})
		return goproxy.OkConnect, host
	})
	gp.OnRequest().DoFunc(p.onRequest)
	gp.OnResponse().DoFunc(p.onResponse)
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if p.serveManagement(w, req) {
			return
		}
		gp.ServeHTTP(w, req)
	})

	ln, err := net.Listen("tcp", p.opts.Listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", p.opts.Listen, err)
	}
	addr := ln.Addr().String()
	p.server = &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 15 * time.Second}
	if err := p.writeMetadata("running", addr); err != nil {
		_ = ln.Close()
		return err
	}
	p.flowLog.Write(map[string]any{
		"ts":          time.Now().Format(time.RFC3339Nano),
		"type":        "session_start",
		"listen_addr": addr,
		"session_dir": p.sessionDir,
		"ca_cert":     p.ca.CertPath,
		"hosts":       p.opts.HostPatterns,
	})

	errCh := make(chan error, 1)
	go func() {
		err := p.server.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.server.Shutdown(shutdownCtx)
		<-errCh
		p.flowLog.Write(map[string]any{
			"ts":   time.Now().Format(time.RFC3339Nano),
			"type": "session_stop",
		})
		_ = p.writeMetadata("stopped", addr)
		return nil
	case err := <-errCh:
		if err != nil {
			_ = p.writeMetadata("failed", addr)
			return err
		}
		_ = p.writeMetadata("stopped", addr)
		return nil
	}
}

func (p *Proxy) Close() error {
	var err error
	if p.flowLog != nil {
		err = p.flowLog.Close()
	}
	if p.wsLog != nil {
		if e := p.wsLog.Close(); err == nil {
			err = e
		}
	}
	if p.rpcLog != nil {
		if e := p.rpcLog.Close(); err == nil {
			err = e
		}
	}
	return err
}

type flowState struct {
	ID         string
	Start      time.Time
	Captured   bool
	Request    *http.Request
	RequestURL string
}

func (p *Proxy) onRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	flowID := fmt.Sprintf("%d", ctx.Session)
	captured := p.filter.Match(req.Host)
	state := &flowState{
		ID:         flowID,
		Start:      time.Now(),
		Captured:   captured,
		Request:    req,
		RequestURL: req.URL.String(),
	}
	ctx.UserData = state
	if !captured {
		return req, nil
	}
	ctx.RoundTripper = roundTripperFunc(func(r *http.Request, c *goproxy.ProxyCtx) (*http.Response, error) {
		res, err := c.Proxy.Tr.RoundTrip(r)
		if err != nil || res == nil {
			return res, err
		}
		if isWebSocketHandshakeResponse(res) {
			if rwc, ok := res.Body.(io.ReadWriteCloser); ok {
				res.Body = newWSReadWriteCloser(rwc, flowID, state.RequestURL, p.wsLog, p.decoder, p.opts.MaxWSFrameBytes)
			}
		}
		return res, nil
	})

	p.flowLog.Write(map[string]any{
		"ts":      state.Start.Format(time.RFC3339Nano),
		"type":    "http_request",
		"flow_id": flowID,
		"method":  req.Method,
		"url":     req.URL.String(),
		"host":    req.Host,
		"proto":   req.Proto,
		"headers": cloneHeader(req.Header),
		"body":    readRequestBody(req, p.opts.MaxBodyBytes),
	})
	return req, nil
}

func (p *Proxy) onResponse(res *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if res == nil {
		return res
	}
	state, _ := ctx.UserData.(*flowState)
	if state == nil || !state.Captured {
		return res
	}
	rec := map[string]any{
		"ts":          time.Now().Format(time.RFC3339Nano),
		"type":        "http_response",
		"flow_id":     state.ID,
		"method":      state.Request.Method,
		"url":         state.RequestURL,
		"status":      res.StatusCode,
		"status_text": res.Status,
		"proto":       res.Proto,
		"headers":     cloneHeader(res.Header),
		"duration_ms": time.Since(state.Start).Milliseconds(),
	}
	if isWebSocketHandshakeResponse(res) {
		rec["websocket"] = true
	} else {
		rec["body"] = readResponseBody(res, p.opts.MaxBodyBytes)
	}
	p.flowLog.Write(rec)
	return res
}

func (p *Proxy) writeMetadata(status, listenAddr string) error {
	if listenAddr == "" {
		listenAddr = p.opts.Listen
	}
	meta := map[string]any{
		"status":             status,
		"updated_at":         time.Now().Format(time.RFC3339Nano),
		"listen_addr":        listenAddr,
		"cert_page_url":      "http://" + listenAddr + "/",
		"session_dir":        p.sessionDir,
		"flows_jsonl":        filepath.Join(p.sessionDir, "flows.jsonl"),
		"websocket_jsonl":    filepath.Join(p.sessionDir, "websocket.jsonl"),
		"rpc_jsonl":          filepath.Join(p.sessionDir, captureanalysis.RPCIndexFile),
		"analysis_json":      filepath.Join(p.sessionDir, captureanalysis.ReportFile),
		"ca_cert":            p.ca.CertPath,
		"ca_key":             p.ca.KeyPath,
		"host_patterns":      p.opts.HostPatterns,
		"max_body_bytes":     p.opts.MaxBodyBytes,
		"max_ws_frame_bytes": p.opts.MaxWSFrameBytes,
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.metaPath, data, 0o644)
}

func cloneHeader(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, vs := range h {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func isWebSocketHandshakeResponse(res *http.Response) bool {
	if res == nil || res.StatusCode != http.StatusSwitchingProtocols {
		return false
	}
	return headerContains(res.Header, "Connection", "Upgrade") &&
		headerContains(res.Header, "Upgrade", "websocket")
}

func headerContains(h http.Header, name, value string) bool {
	for _, raw := range h.Values(name) {
		for _, part := range strings.Split(raw, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return true
			}
		}
	}
	return false
}

type roundTripperFunc func(*http.Request, *goproxy.ProxyCtx) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Response, error) {
	return f(req, ctx)
}
