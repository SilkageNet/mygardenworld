package babigame

import (
	"encoding/json"
	"testing"
)

func TestHasPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "empty", raw: "", want: false},
		{name: "whitespace", raw: " \n\t ", want: false},
		{name: "null", raw: "null", want: false},
		{name: "empty object", raw: "{}", want: false},
		{name: "empty array", raw: "[]", want: false},
		{name: "empty string", raw: `""`, want: false},
		{name: "object", raw: `{"100":{"0":{}}}`, want: true},
		{name: "array", raw: `[0]`, want: true},
		{name: "number", raw: `0`, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPayload(json.RawMessage(tt.raw)); got != tt.want {
				t.Fatalf("HasPayload(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestWSResponseDSessionExpired(t *testing.T) {
	var d WSResponseD
	if err := json.Unmarshal([]byte(`{"m":{"type":90000,"msg":"会话已过期，请重新登录"},"k":"ws|1|2"}`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.IsError() {
		t.Fatal("expected response to be an error")
	}
	if got := d.ErrorType(); got != 90000 {
		t.Fatalf("ErrorType() = %d, want 90000", got)
	}
	if got := d.ErrorMsg(); got != "会话已过期，请重新登录" {
		t.Fatalf("ErrorMsg() = %q", got)
	}
	if !d.IsSessionExpired() {
		t.Fatal("expected session-expired response")
	}
	if d.IsSessionDisplaced() {
		t.Fatal("generic expiry must not be treated as a displaced session")
	}
}

func TestWSResponseDSessionDisplaced(t *testing.T) {
	var d WSResponseD
	if err := json.Unmarshal([]byte(`{"m":{"type":90000,"msg":"账号已在其他设备登录，您已被挤下线"}}`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.IsSessionExpired() || !d.IsSessionDisplaced() {
		t.Fatalf("session predicates expired=%t displaced=%t, want both true", d.IsSessionExpired(), d.IsSessionDisplaced())
	}
}

func TestSessionDisplacementFromBinary(t *testing.T) {
	tests := []struct {
		name  string
		items []json.RawMessage
		want  bool
	}{
		{
			name: "observed named event with encoded content",
			items: []json.RawMessage{
				json.RawMessage(`{"t":"notify","i":"bst"}`),
				json.RawMessage(`{"name":"usr_kick","content":"{\"4\":0,\"5\":0}","dsName":"G.IKickData"}`),
			},
			want: true,
		},
		{
			name:  "compact event",
			items: []json.RawMessage{json.RawMessage(`{"0":"usr_kick","1":{"4":0,"5":0}}`)},
			want:  true,
		},
		{
			name:  "server gm kick",
			items: []json.RawMessage{json.RawMessage(`{"name":"usr_kick","content":{"isGM":1}}`)},
			want:  false,
		},
		{
			name:  "maintenance close",
			items: []json.RawMessage{json.RawMessage(`{"name":"usr_kick","content":{"isClose":1}}`)},
			want:  false,
		},
		{
			name:  "kick all",
			items: []json.RawMessage{json.RawMessage(`{"name":"usr_kickAll","content":{}}`)},
			want:  false,
		},
		{
			name:  "malformed safety field",
			items: []json.RawMessage{json.RawMessage(`{"name":"usr_kick","content":{"isGM":"unknown"}}`)},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := SessionDisplacementFromBinary(tt.items)
			if got != tt.want {
				t.Fatalf("SessionDisplacementFromBinary()=%t, want %t", got, tt.want)
			}
		})
	}
}

func TestWSResponseDGatewayBlockReason(t *testing.T) {
	var d WSResponseD
	if err := json.Unmarshal([]byte(`{"m":{"code":105,"param":{"expiredTime":1783612800000,"reason":"IDC IP封禁","targetType":0}},"k":"ws|1|2"}`), &d); err != nil {
		t.Fatal(err)
	}
	if got := d.ErrorCode(); got != 105 {
		t.Fatalf("ErrorCode() = %d, want 105", got)
	}
	want := "IDC IP封禁（解封时间：2026-07-10 00:00:00）"
	if got := d.ErrorMsg(); got != want {
		t.Fatalf("ErrorMsg() = %q, want %q", got, want)
	}
	if !d.IsSessionExpired() {
		t.Fatal("expected gateway block response to invalidate the session")
	}
	if d.IsSessionDisplaced() {
		t.Fatal("gateway block must not be treated as a displaced session")
	}
}

func TestErrorCodeOfLangJS(t *testing.T) {
	d := WSResponseD{M: json.RawMessage(`{"codeOfLangJs":"fmlRace_tips1","msg":"已接取其他任务"}`)}
	if got := d.ErrorCodeOfLangJS(); got != "fmlRace_tips1" {
		t.Fatalf("ErrorCodeOfLangJS=%q", got)
	}
	if d.ErrorMsg() != "已接取其他任务" {
		t.Fatalf("ErrorMsg=%q", d.ErrorMsg())
	}
}

func TestClientDispatchTextFiresSessionExpired(t *testing.T) {
	c := NewClient(&Session{Cfg: testConfig(t)})
	called := false
	c.OnSessionExpired(func(env WSResponseD) {
		called = true
		if env.ErrorType() != 90000 {
			t.Fatalf("ErrorType() = %d, want 90000", env.ErrorType())
		}
	})

	c.dispatchText([]byte(`{"e":"response","d":{"m":{"type":90000,"msg":"会话已过期，请重新登录"},"r":1,"t":2,"k":"ws|1|1"}}`))

	if !called {
		t.Fatal("session-expired handler was not called")
	}
}

func TestClientDispatchTextFiresSessionExpiredForGatewayBlock(t *testing.T) {
	c := NewClient(&Session{Cfg: testConfig(t)})
	called := false
	c.OnSessionExpired(func(env WSResponseD) {
		called = true
		if got := env.ErrorMsg(); got != "IDC IP封禁（解封时间：2026-07-10 00:00:00）" {
			t.Fatalf("ErrorMsg() = %q", got)
		}
	})

	c.dispatchText([]byte(`{"e":"response","d":{"m":{"code":105,"param":{"expiredTime":1783612800000,"reason":"IDC IP封禁","targetType":0}},"r":1,"t":2,"k":"ws|1|1"}}`))

	if !called {
		t.Fatal("session-expired handler was not called")
	}
}

func TestClientDispatchTextUsesSingleNamespaceApplyOwner(t *testing.T) {
	tests := []struct {
		name               string
		pending            bool
		dispatchNamespaces bool
		wantNamespaceCalls int
	}{
		{name: "server push uses subscriber", wantNamespaceCalls: 1},
		{name: "rpc without apply hook uses subscriber", pending: true, dispatchNamespaces: true, wantNamespaceCalls: 1},
		{name: "caller owned rpc suppresses subscriber", pending: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(&Session{Cfg: testConfig(t)})
			calls := 0
			client.OnNamespace("7", func(_ string, _ json.RawMessage, _ WSResponseD) { calls++ })
			var resultCh chan rpcResult
			if tt.pending {
				resultCh = make(chan rpcResult, 1)
				client.pending["ws|1|1"] = pendingRPC{result: resultCh, dispatchNamespaces: tt.dispatchNamespaces}
			}

			client.dispatchText([]byte(`{"e":"response","d":{"v":{"7":{"2":{"0":{"11":188}}}},"r":1,"t":2,"k":"ws|1|1"}}`))

			if calls != tt.wantNamespaceCalls {
				t.Fatalf("namespace calls=%d, want %d", calls, tt.wantNamespaceCalls)
			}
			if tt.pending {
				select {
				case result := <-resultCh:
					if string(result.v) != `{"7":{"2":{"0":{"11":188}}}}` {
						t.Fatalf("pending payload=%s", result.v)
					}
				default:
					t.Fatal("pending RPC did not receive its response")
				}
			}
		})
	}
}
