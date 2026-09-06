package babigame

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	clientproto "github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/coder/websocket"
)

// Client is the WebSocket-side counterpart of HTTPClient. One per logged-in
// account. Holds the wss connection, dispatches RPC responses to callers
// waiting on (k -> chan v), and surfaces server pushes via subscriber callbacks.
type Client struct {
	Cfg     Config
	Session *Session

	mu       sync.Mutex
	conn     *websocket.Conn
	pending  map[string]pendingRPC
	closed   atomic.Bool
	closedCh chan struct{}

	seq atomic.Int64

	subMu                  sync.RWMutex
	nsHandlers             map[string][]NamespaceHandler
	binHandlers            []BinaryHandler
	sessionExpiredHandlers []SessionExpiredHandler

	// Heartbeat configuration.
	HeartbeatInterval time.Duration

	// DebugWriter, when non-nil, receives all WS frames (send + recv) as JSONL.
	DebugWriter *DebugFrameWriter

	// Configure before Connect. BeforeRPC can reject any game RPC (including
	// heartbeat/login); OnRPCResponse runs before the matching caller resumes.
	// The observer must not call RPC or block for network IO.
	BeforeRPC     func(context.Context, string) error
	OnRPCResponse func(string, WSResponseD)
}

// NamespaceHandler receives the namespace value (`v.<ns_key>`) plus the full
// envelope d field for context. Called synchronously from the reader loop;
// callbacks must not block long.
type NamespaceHandler func(nsKey string, value json.RawMessage, env WSResponseD)

// BinaryHandler receives every embedded JSON in a server-pushed binary frame
// (G.ISysMsg world events, etc).
type BinaryHandler func(items []json.RawMessage)

// SessionExpiredHandler receives the first server response that indicates the
// active websocket session was invalidated.
type SessionExpiredHandler func(env WSResponseD)

type rpcResult struct {
	v   json.RawMessage
	d   WSResponseD
	err error
}

// pendingRPC records who owns application of a matching response payload.
// Namespace subscribers remain the fallback for pushes and RPC clients without
// an apply hook; calls with a hook (or explicit manual apply) suppress that
// fallback so additive namespace deltas are never merged twice.
type pendingRPC struct {
	name               string
	result             chan rpcResult
	dispatchNamespaces bool
}

// NewClient prepares a Client around a logged-in Session. Connect() must be
// called before any RPC.
func NewClient(session *Session) *Client {
	return &Client{
		Cfg:               session.Cfg,
		Session:           session,
		pending:           make(map[string]pendingRPC),
		closedCh:          make(chan struct{}),
		nsHandlers:        make(map[string][]NamespaceHandler),
		HeartbeatInterval: 25 * time.Second,
	}
}

// OnNamespace registers a callback for a top-level v key (e.g. "100" for
// land state, "7" for inventory). Multiple handlers are allowed.
func (c *Client) OnNamespace(nsKey string, h NamespaceHandler) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.nsHandlers[nsKey] = append(c.nsHandlers[nsKey], h)
}

// OnBinary registers a callback for binary server pushes.
func (c *Client) OnBinary(h BinaryHandler) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.binHandlers = append(c.binHandlers, h)
}

// OnSessionExpired registers a callback for server-side session invalidation.
func (c *Client) OnSessionExpired(h SessionExpiredHandler) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.sessionExpiredHandlers = append(c.sessionExpiredHandlers, h)
}

// Connect dials the wss URL and starts the reader / heartbeat goroutines.
func (c *Client) Connect(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, c.Session.WSURL(), &websocket.DialOptions{
		HTTPHeader: nil,
	})
	if err != nil {
		return fmt.Errorf("ws dial %s: %w", c.Session.WSURL(), err)
	}
	conn.SetReadLimit(8 * 1024 * 1024)
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	go c.reader()
	if c.HeartbeatInterval > 0 {
		go c.heartbeat()
	}
	return nil
}

// Close terminates the connection and fails every pending RPC.
func (c *Client) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(c.closedCh)
	c.mu.Lock()
	conn := c.conn
	pending := c.pending
	c.pending = make(map[string]pendingRPC)
	c.mu.Unlock()
	for _, call := range pending {
		call.result <- rpcResult{err: errors.New("websocket closed")}
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "client close")
	}
	return nil
}

// Done returns a channel that is closed when the client is shutting down.
func (c *Client) Done() <-chan struct{} { return c.closedCh }

// Closed reports whether Close has started. It lets higher layers avoid
// treating a dead websocket as a live connection while reconnect handling runs.
func (c *Client) Closed() bool { return c.closed.Load() }

// rpc sends one websocket RPC and blocks until the matching response arrives.
// Higher layers should use RPCClient so route, timeout, and DTO handling stay
// centralized.
func (c *Client) rpc(ctx context.Context, name string, args any, routeArg string, timeout time.Duration, dispatchNamespaces bool) (json.RawMessage, WSResponseD, error) {
	if c.BeforeRPC != nil {
		if err := c.BeforeRPC(ctx, name); err != nil {
			return nil, WSResponseD{}, err
		}
	}
	if c.closed.Load() {
		return nil, WSResponseD{}, errors.New("client closed")
	}
	seq := c.seq.Add(1)
	frame, k, err := BuildRequest(name, args, routeArg, seq, c.Cfg)
	if err != nil {
		return nil, WSResponseD{}, err
	}
	ch := make(chan rpcResult, 1)
	c.mu.Lock()
	conn := c.conn
	if conn != nil {
		c.pending[k] = pendingRPC{name: name, result: ch, dispatchNamespaces: dispatchNamespaces}
	}
	c.mu.Unlock()
	if conn == nil {
		return nil, WSResponseD{}, errors.New("not connected")
	}
	if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
		c.mu.Lock()
		delete(c.pending, k)
		c.mu.Unlock()
		return nil, WSResponseD{}, fmt.Errorf("ws write: %w", err)
	}
	if c.DebugWriter != nil {
		c.DebugWriter.Log("ws_send", name, frame, "")
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case res := <-ch:
		return res.v, res.d, res.err
	case <-t.C:
		c.mu.Lock()
		delete(c.pending, k)
		c.mu.Unlock()
		return nil, WSResponseD{}, fmt.Errorf("rpc %s: timeout after %s", name, timeout)
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, k)
		c.mu.Unlock()
		return nil, WSResponseD{}, ctx.Err()
	case <-c.closedCh:
		return nil, WSResponseD{}, errors.New("client closed")
	}
}

// reader pulls every frame, dispatches to pending RPC waiters and namespace
// subscribers. Returns when the connection closes.
func (c *Client) reader() {
	defer func() { _ = c.Close() }()
	ctx := context.Background()
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch typ {
		case websocket.MessageText:
			if c.DebugWriter != nil {
				c.DebugWriter.Log("ws_recv", "", "", string(data))
			}
			c.dispatchText(data)
		case websocket.MessageBinary:
			items := ParseBinaryFrame(data)
			c.subMu.RLock()
			handlers := append([]BinaryHandler(nil), c.binHandlers...)
			c.subMu.RUnlock()
			for _, h := range handlers {
				h(items)
			}
		}
	}
}

// dispatchText handles "connectionEnabled", server response envelopes, and
// any other text frames. Anything that's not a recognized response is
// silently ignored (forward-compat).
func (c *Client) dispatchText(data []byte) {
	// The server emits the literal `"connectionEnabled"` at handshake time.
	if string(data) == `"connectionEnabled"` {
		return
	}
	env, err := ParseTextFrame(string(data))
	if err != nil {
		return
	}
	if env.E != "response" {
		return
	}
	var d WSResponseD
	if err := json.Unmarshal(env.D, &d); err != nil {
		return
	}
	// Resolve any waiter on this k.
	c.mu.Lock()
	call, ok := c.pending[d.K]
	if ok {
		delete(c.pending, d.K)
	}
	c.mu.Unlock()
	if c.OnRPCResponse != nil {
		c.OnRPCResponse(call.name, d)
	}
	if ok {
		call.result <- rpcResult{v: d.V, d: d}
	}
	if d.IsSessionExpired() {
		c.fireSessionExpired(d)
		return
	}
	// Don't fire namespace subscribers for error responses.
	if d.IsError() {
		return
	}
	if ok && !call.dispatchNamespaces {
		return
	}
	// Fire namespace subscribers.
	if len(d.V) == 0 {
		return
	}
	var nsMap map[string]json.RawMessage
	if err := json.Unmarshal(d.V, &nsMap); err != nil {
		// v can also be a string (some legacy responses serialize as a JSON string).
		return
	}
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	for nsKey, child := range nsMap {
		handlers := c.nsHandlers[nsKey]
		for _, h := range handlers {
			h(nsKey, child, d)
		}
	}
}

func (c *Client) fireSessionExpired(d WSResponseD) {
	c.subMu.RLock()
	handlers := append([]SessionExpiredHandler(nil), c.sessionExpiredHandlers...)
	c.subMu.RUnlock()
	for _, h := range handlers {
		h(d)
	}
}

// heartbeat sends usr.heartTick on a tick. The server returns updated
// inventory deltas in namespace 7 - useful even if we never need to read them.
func (c *Client) heartbeat() {
	ticker := time.NewTicker(c.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.closedCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = CallRPC[clientproto.StateDelta](ctx, NewRPCClient(c, c.Session), clientproto.RPCUsrHeartTick, clientproto.UsrHeartTickRequest{}, WithTimeout(10*time.Second))
			cancel()
		}
	}
}

func (c *Client) indexLoginRequest(isSimulator int) clientproto.IndexLoginRequest {
	s := c.Session
	return clientproto.IndexLoginRequest{
		AID:         s.AID,
		GsIdx:       int32(s.GsIdx),
		Token:       s.RouteToken,
		OSType:      int32(s.Cfg.OSType),
		IsNative:    s.Cfg.IsNative,
		DeviceID:    s.DeviceID,
		IsSimulator: int32(isSimulator),
		DeviceInfo: clientproto.IndexDeviceInfo{
			OSType:         s.Cfg.SDKPlatform,
			DeviceID:       s.DeviceID,
			IsEmulator:     "0",
			OSVersion:      s.Cfg.OSVersion,
			Brand:          s.Cfg.DeviceBrand,
			Model:          s.Cfg.DeviceModel,
			NetworkType:    s.Cfg.NetworkType,
			SysLanguage:    s.Cfg.SysLanguage,
			ScreenWidthPx:  s.Cfg.ScreenWidthPx,
			ScreenHeightPx: s.Cfg.ScreenHeightPx,
			DeviceType:     s.Cfg.DeviceType,
			AppVersion:     s.Cfg.AppVersion,
		},
		Inviter: map[string]any{},
		Version: s.Cfg.ClientVersion,
		Area:    s.Cfg.Area,
		ChnID:   int32(s.Cfg.ChannelID),
	}
}

// Login issues index.login on the live WS, matching the official client's
// first post-connect initialization call.
func (c *Client) Login(ctx context.Context, isSimulator int) (json.RawMessage, error) {
	req := c.indexLoginRequest(isSimulator)
	resp, err := CallRPC[clientproto.StateDelta](ctx, NewRPCClient(c, c.Session), clientproto.RPCIndexLogin, req,
		WithTimeout(30*time.Second), WithPayloadApply(false))
	return resp.Payload, err
}

// ReLogin issues index.reLogin on the live WS. Idempotent on first call;
// the second call on the same WS won't refresh server-pushed state, so
// callers that want a refresh should reconnect instead.
func (c *Client) ReLogin(ctx context.Context, isSimulator int) (json.RawMessage, error) {
	req := clientproto.IndexReLoginRequest(c.indexLoginRequest(isSimulator))
	resp, err := CallRPC[clientproto.StateDelta](ctx, NewRPCClient(c, c.Session), clientproto.RPCIndexReLogin, req,
		WithTimeout(30*time.Second), WithPayloadApply(false))
	return resp.Payload, err
}

// LazySync pulls module init data (namespaces 111/122/129/139/155/161 in
// captures - generic activity / quests / mail). Doesn't refresh 100/7.
func (c *Client) LazySync(ctx context.Context) (json.RawMessage, error) {
	resp, err := CallRPC[clientproto.StateDelta](ctx, NewRPCClient(c, c.Session), clientproto.RPCUsrLazySync, clientproto.UsrLazySyncRequest{},
		WithTimeout(15*time.Second), WithPayloadApply(false))
	return resp.Payload, err
}
