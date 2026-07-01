package captureproxy

import "testing"

func TestHostFilter(t *testing.T) {
	f := newHostFilter([]string{"*.babigame.cn", "example.com"})
	for _, host := range []string{"api.babigame.cn", "hy2gnhf118.babigame.cn:54825", "example.com"} {
		if !f.Match(host) {
			t.Fatalf("expected match for %s", host)
		}
	}
	for _, host := range []string{"babigame.cn.evil.test", "other.com"} {
		if f.Match(host) {
			t.Fatalf("unexpected match for %s", host)
		}
	}
}
