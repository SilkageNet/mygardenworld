package outbound

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"testing"
	"time"
)

func TestEndpointRestrictionsAndDNSRebindingGate(t *testing.T) {
	for _, endpoint := range []string{"http://example.com/hook", "https://localhost/h", "https://user:pass@example.com", "https://example.com/#token", "https://127.0.0.1", "https://[::1]", "https://169.254.169.254", "https://100.100.100.200", "https://example.com:99999", "https://example.com:invalid"} {
		if ValidateEndpoint(endpoint) == nil {
			t.Errorf("accepted unsafe endpoint %q", endpoint)
		}
	}
	if err := ValidateEndpoint("https://example.com/hook?token=secret"); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "172.16.1.2", "192.168.1.2", "169.254.169.254", "100.100.100.200", "0.0.0.0", "224.0.0.1", "240.0.0.1", "192.0.2.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "64:ff9b::a9fe:a9fe", "2002:7f00:1::", "2001:db8::1"} {
		if publicAddress(netip.MustParseAddr(ip)) {
			t.Errorf("accepted non-public %s", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !publicAddress(netip.MustParseAddr(ip)) {
			t.Errorf("rejected public %s", ip)
		}
	}
	for _, answers := range [][]net.IPAddr{{{IP: net.ParseIP("127.0.0.1")}}, {{IP: net.ParseIP("1.1.1.1")}, {IP: net.ParseIP("10.0.0.1")}}} {
		lookup := func(context.Context, string) ([]net.IPAddr, error) { return answers, nil }
		if _, err := dialPublic(context.Background(), "tcp", "example.com:443", lookup); !errors.Is(err, ErrUnsafeEndpoint) {
			t.Fatal("DNS rebinding gate failed", err)
		}
	}
	client := NewClient(10 * time.Second)
	if client.Transport.(*http.Transport).Proxy != nil || client.Timeout > 10*time.Second {
		t.Fatal("unsafe transport options")
	}
	if client.CheckRedirect(&http.Request{}, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects allowed")
	}
}
