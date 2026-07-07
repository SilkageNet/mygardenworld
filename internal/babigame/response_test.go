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
