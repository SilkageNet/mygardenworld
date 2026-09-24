package babigame

import "testing"

func TestAdoptNotifyUUID(t *testing.T) {
	c := &HTTPClient{UUID: "client-rfc-uuid"}
	c.NotifyURL = "https://hygncdn.babigame.cn/index-gn-mix-450.0.15.html?mdgid=160&uuid=ServerIssuedUUID31charsXXXX&env=prod"
	adoptNotifyUUID(c)
	if c.UUID != "ServerIssuedUUID31charsXXXX" {
		t.Fatalf("uuid=%q", c.UUID)
	}

	c2 := &HTTPClient{UUID: "keep-me"}
	adoptNotifyUUID(c2)
	if c2.UUID != "keep-me" {
		t.Fatalf("empty notify should not change uuid: %q", c2.UUID)
	}

	c3 := &HTTPClient{UUID: "keep-me", NotifyURL: "https://example.com/x?env=prod"}
	adoptNotifyUUID(c3)
	if c3.UUID != "keep-me" {
		t.Fatalf("missing uuid query should not change: %q", c3.UUID)
	}
}
