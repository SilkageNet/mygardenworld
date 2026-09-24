package babigame

import (
	"strings"
	"testing"
	"unicode"
)

func TestRandomGameSession1Shape(t *testing.T) {
	s := RandomGameSession1()
	if !strings.HasPrefix(s, "s1") {
		t.Fatalf("prefix: %q", s)
	}
	rest := s[2:]
	if len(rest) < 17+10 { // 17 uuid chars + unix ms (>=10 digits in 2026)
		t.Fatalf("too short: %q", s)
	}
	body := rest[:17]
	for _, r := range body {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			t.Fatalf("non-alnum in uuid part %q of %q", body, s)
		}
	}
	ms := rest[17:]
	for _, r := range ms {
		if r < '0' || r > '9' {
			t.Fatalf("non-digit ms suffix %q of %q", ms, s)
		}
	}
}

func TestCommonAppInfoOmitsEmptyAndLowercases(t *testing.T) {
	cfg, err := ConfigForChannel(ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	c := NewHTTPClient(cfg, "DEV-1", "uuid-1", "sess0")
	info := c.commonAppInfo("")
	if _, ok := info["_ip"]; ok {
		t.Fatalf("empty _ip should be omitted: %#v", info)
	}
	if got, _ := info["_package_name"].(string); got != strings.ToLower(cfg.PackageName) {
		t.Fatalf("package_name: %q", got)
	}
	if got, _ := info["_equipment_brand"].(string); got != "apple" {
		t.Fatalf("brand: %q", got)
	}
}
