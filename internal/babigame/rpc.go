package babigame

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RPCName is the server-side interface name without the client-side "gs."
// prefix. game.js calls request2("gs.usrLand.plant", ...); the websocket
// payload uses "usrLand.plant".
type RPCName string

func (n RPCName) String() string { return string(n) }

// RPCID is the usual numeric identifier used by game RPCs for item, task,
// land, activity, slot, and similar ids observed in game.js.
type RPCID = int32

// RPCUID is a user/account identifier. These values can be larger than
// content ids, so uid-like request fields use int64.
type RPCUID = int64

// RPCInt is a numeric argument that is not itself an id, such as count, index,
// type, mode, or a 0/1 flag that game.js sends numerically.
type RPCInt = int32

// RPCBool is a boolean argument that game.js sends as true/false.
type RPCBool = bool

// RPCString is a text argument in an RPC request.
type RPCString = string

// RPCIDList and RPCUIDList are the common list forms used by request bodies.
type RPCIDList = []int32
type RPCUIDList = []int64
type RPCStringList = []string

// RPCObject is a structured JSON object whose keys vary by caller.
type RPCObject = map[string]any

// RPCArray is a structured JSON array whose element shape varies by caller.
type RPCArray = []any

// RPCValue is intentionally dynamic JSON. Use it only when game.js forwards a
// caller-supplied scalar/object/array without a stable DTO.
type RPCValue = any

// RPCPoint is the anti-cheat point payload shape used by *Rqst verification
// RPCs. Captures show it as [pointType, fingerprint].
type RPCPoint = []any

// RPCResult is the normalized result of one websocket RPC.
type RPCResult struct {
	Name     RPCName
	Payload  json.RawMessage
	Envelope WSResponseD
}

// HasPayload reports whether the server returned a non-empty v payload.
func (r RPCResult) HasPayload() bool { return HasPayload(r.Payload) }

// StateDelta is the normal response payload shape for game RPCs: a map keyed
// by numeric namespace id ("7", "100", "114", ...), with each namespace value
// left raw so the state package can merge only the fragments it understands.
type StateDelta map[string]json.RawMessage

// Namespace returns one namespace fragment from a decoded StateDelta.
func (d StateDelta) Namespace(key string) json.RawMessage {
	if d == nil {
		return nil
	}
	return d[key]
}

// RawRequest is the fallback request container for RPCs whose exact request
// object could not be recovered from static game.js analysis. It is still a
// typed per-method parameter in the facade, but callers provide JSON-shaped
// fields explicitly.
type RawRequest map[string]any

// RPCRequestShape records how confident the static extractor was about a
// request object.
type RPCRequestShape string

const (
	// RPCRequestEmpty means game.js called the RPC with an empty object.
	RPCRequestEmpty RPCRequestShape = "empty"
	// RPCRequestFields means game.js exposed a stable object literal, and
	// RequestFields lists the JSON keys observed in that literal.
	RPCRequestFields RPCRequestShape = "fields"
	// RPCRequestRaw means game.js passed a prebuilt value or dynamic route, so
	// callers should provide a RawRequest-compatible JSON object.
	RPCRequestRaw RPCRequestShape = "raw"
)

// RPCSpec describes one RPC observed in game.js. It is metadata for discovery
// and tooling; the typed facade methods remain the preferred call sites.
type RPCSpec struct {
	Name           RPCName
	Group          string
	Method         string
	RequestShape   RPCRequestShape
	RequestFields  []string
	ResponseSchema string
}

// RPCResponse is a typed view over RPCResult. Payload always keeps the raw v
// bytes, while Data contains the same payload decoded into T when present.
type RPCResponse[T any] struct {
	RPCResult
	Data T
}

// RPCServerError wraps a successful websocket round trip whose server envelope
// carried an m/error field.
type RPCServerError struct {
	Name     RPCName
	Envelope WSResponseD
}

func (e *RPCServerError) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Envelope.ErrorMsg()
	if msg == "" {
		msg = string(e.Envelope.M)
	}
	return fmt.Sprintf("rpc %s: server: %s", e.Name, msg)
}

// RPCClient is a route-aware facade over the websocket transport. It exists so higher
// layers can call game interfaces by the names found in game.js without
// repeating route, timeout, and payload-apply plumbing.
type RPCClient struct {
	client                *Client
	routeArg              string
	defaultTimeout        time.Duration
	applyV                func(json.RawMessage)
	serverErrorsAsResults bool
}

// RPCClientOption configures a new RPCClient.
type RPCClientOption func(*RPCClient)

// WithDefaultTimeout sets the timeout used by RPC calls when a request does not
// override it.
func WithDefaultTimeout(timeout time.Duration) RPCClientOption {
	return func(c *RPCClient) {
		if timeout > 0 {
			c.defaultTimeout = timeout
		}
	}
}

// WithRouteArg overrides the standard session route argument.
func WithRouteArg(routeArg string) RPCClientOption {
	return func(c *RPCClient) {
		c.routeArg = routeArg
	}
}

// WithoutRoute sends two-tuple RPC payloads for interfaces that do not use
// the standard route token.
func WithoutRoute() RPCClientOption {
	return func(c *RPCClient) {
		c.routeArg = ""
	}
}

// WithApplyV installs an optional state-apply hook. It is called after
// successful server responses with a non-empty payload.
func WithApplyV(apply func(json.RawMessage)) RPCClientOption {
	return func(c *RPCClient) {
		c.applyV = apply
	}
}

// WithServerErrorsAsResults makes typed RPC calls return server-side m/error
// envelopes as successful round trips. This is useful for business loops that
// need to branch on WSResponseD without conflating domain errors with transport
// failures.
func WithServerErrorsAsResults() RPCClientOption {
	return func(c *RPCClient) {
		c.serverErrorsAsResults = true
	}
}

// NewRPCClient wraps a websocket client. If session is nil, client.Session is
// used when available.
func NewRPCClient(client *Client, session *Session, opts ...RPCClientOption) *RPCClient {
	if session == nil && client != nil {
		session = client.Session
	}
	routeArg := ""
	if session != nil {
		routeArg = session.RouteArg()
	}
	out := &RPCClient{
		client:         client,
		routeArg:       routeArg,
		defaultTimeout: 30 * time.Second,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(out)
		}
	}
	return out
}

// RequestOption configures a single RPC call.
type RequestOption func(*requestOptions)

type requestOptions struct {
	routeArg              string
	timeout               time.Duration
	applyV                *bool
	serverErrorsAsResults *bool
}

// WithTimeout overrides the RPC timeout for one call.
func WithTimeout(timeout time.Duration) RequestOption {
	return func(o *requestOptions) {
		if timeout > 0 {
			o.timeout = timeout
		}
	}
}

// WithRequestRoute overrides the route argument for one call.
func WithRequestRoute(routeArg string) RequestOption {
	return func(o *requestOptions) {
		o.routeArg = routeArg
	}
}

// WithoutRequestRoute sends a two-tuple payload for one call.
func WithoutRequestRoute() RequestOption {
	return func(o *requestOptions) {
		o.routeArg = ""
	}
}

// WithPayloadApply controls whether this call should run the client's applyV
// hook. The default is true.
func WithPayloadApply(enabled bool) RequestOption {
	return func(o *requestOptions) {
		o.applyV = &enabled
	}
}

// WithRequestServerErrorsAsResults overrides server error handling for one
// call. When enabled, the typed facade returns the envelope without converting
// m/errors to RPCServerError.
func WithRequestServerErrorsAsResults(enabled bool) RequestOption {
	return func(o *requestOptions) {
		o.serverErrorsAsResults = &enabled
	}
}

// rawCall sends an RPC and returns the envelope exactly as the server answered.
// Server-side m/errors are not converted into Go errors.
func (c *RPCClient) rawCall(ctx context.Context, name string, args any, opts ...RequestOption) (RPCResult, error) {
	if c == nil || c.client == nil {
		return RPCResult{}, errors.New("nil RPC client")
	}
	rpcName, err := NormalizeRPCName(name)
	if err != nil {
		return RPCResult{}, err
	}
	callOpts := c.requestOptions(opts...)
	v, d, err := c.client.rpc(ctx, rpcName.String(), args, callOpts.routeArg, callOpts.timeout)
	if err != nil {
		return RPCResult{Name: rpcName}, err
	}
	return RPCResult{Name: rpcName, Payload: v, Envelope: d}, nil
}

// call sends an RPC, treats server m/errors as Go errors, and applies returned
// namespace payloads through WithApplyV when configured.
func (c *RPCClient) call(ctx context.Context, name string, args any, opts ...RequestOption) (RPCResult, error) {
	result, err := c.rawCall(ctx, name, args, opts...)
	if err != nil {
		return result, err
	}
	callOpts := c.requestOptions(opts...)
	if result.Envelope.IsError() {
		if callOpts.shouldReturnServerErrors() {
			return result, nil
		}
		return result, &RPCServerError{Name: result.Name, Envelope: result.Envelope}
	}
	if c.applyV != nil && callOpts.shouldApply() && result.HasPayload() {
		c.applyV(result.Payload)
	}
	return result, nil
}

// callRPC sends one RPC and decodes the returned v payload into T. Most game
// RPCs use StateDelta because responses are namespace-keyed state fragments
// rather than endpoint-specific DTOs.
func callRPC[T any](ctx context.Context, c *RPCClient, name RPCName, args any, opts ...RequestOption) (RPCResponse[T], error) {
	result, err := c.call(ctx, name.String(), args, opts...)
	out := RPCResponse[T]{RPCResult: result}
	if err != nil {
		return out, err
	}
	if !result.HasPayload() {
		return out, nil
	}
	if err := json.Unmarshal(result.Payload, &out.Data); err != nil {
		return out, fmt.Errorf("rpc %s: decode payload: %w", result.Name, err)
	}
	return out, nil
}

func (c *RPCClient) requestOptions(opts ...RequestOption) requestOptions {
	out := requestOptions{
		routeArg:              c.routeArg,
		timeout:               c.defaultTimeout,
		serverErrorsAsResults: &c.serverErrorsAsResults,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

func (o requestOptions) shouldApply() bool {
	return o.applyV == nil || *o.applyV
}

func (o requestOptions) shouldReturnServerErrors() bool {
	return o.serverErrorsAsResults != nil && *o.serverErrorsAsResults
}

// NormalizeRPCName accepts either "usrLand.plant" or the client-side
// "gs.usrLand.plant" spelling from game.js.
func NormalizeRPCName(name string) (RPCName, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "gs.")
	if name == "" {
		return "", errors.New("empty RPC name")
	}
	if !strings.Contains(name, ".") {
		return "", fmt.Errorf("invalid RPC name %q", name)
	}
	return RPCName(name), nil
}
