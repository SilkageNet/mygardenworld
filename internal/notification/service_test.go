package notification

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestClassifyOnlyActionableEventsWithSafeMessages(t *testing.T) {
	for _, tc := range []struct {
		kind, action string
		wanted       bool
		severity     int
		recovered    bool
	}{
		{"account_request_paused", "blocked", true, 2, false},
		{"account_request_resumed", "resumed", true, 0, true},
		{"session_expired", "retry_scheduled", true, 1, false},
		{"session_expired", "blocked", true, 2, false},
		{"session", "session", true, 0, true},
		{"reputation_guard", "blocked", true, 2, false},
		{"reputation_guard", "check", false, 0, false},
		{"pearl_hire_locked", "blocked", true, 2, false},
		{"operation_failed", "failed", false, 0, false},
		{"operation_deferred", "wait", false, 0, false},
		{"pearl_hire_diagnostic", "", false, 0, false},
		{"ws_disconnected", "", false, 0, false},
	} {
		t.Run(tc.kind+"/"+tc.action, func(t *testing.T) {
			s := Classify(store.EventLog{Kind: tc.kind, Action: tc.action, Message: "SECRET", PayloadJSON: `{"password":"SECRET"}`})
			if (s != nil) != tc.wanted {
				t.Fatalf("signal=%+v", s)
			}
			if s != nil && (s.Severity != tc.severity || s.Recovered != tc.recovered || strings.Contains(s.Message, "SECRET")) {
				t.Fatalf("signal=%+v", s)
			}
		})
	}
}

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
	client := safeClient()
	if client.Transport.(*http.Transport).Proxy != nil || client.Timeout > 10*time.Second {
		t.Fatal("unsafe transport options")
	}
	if client.CheckRedirect(&http.Request{}, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects allowed")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDeliverRetriesWithStableIDAndRedactedErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    int
		network bool
		want    string
	}{
		{"success", 204, false, "sent"}, {"throttled", 429, false, "pending"}, {"server", 503, false, "pending"}, {"invalid", 400, false, "failed"}, {"redirect", 302, false, "failed"}, {"network", 0, true, "pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			now := time.Now().UTC()
			db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
			if err != nil {
				t.Fatal(err)
			}
			endpoint := "https://example.com/hook?token=SECRET"
			if err := db.SaveNotificationSettings(ctx, u.ID, true, &endpoint, 30); err != nil {
				t.Fatal(err)
			}
			if _, err := db.QueueNotificationTest(ctx, u.ID, now); err != nil {
				t.Fatal(err)
			}
			s := New(db, nil)
			var keys []string
			s.client = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(r.Body)
				key := r.Header.Get("X-Notification-ID")
				keys = append(keys, key)
				if key == "" || !strings.Contains(string(body), key) || strings.Contains(string(body), "SECRET") || r.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("unsafe request %s", body)
				}
				if tc.network {
					return nil, errors.New("SECRET upstream error")
				}
				return &http.Response{StatusCode: tc.code, Header: http.Header{"Retry-After": []string{"120"}}, Body: io.NopCloser(strings.NewReader("SECRET response"))}, nil
			})}
			if err := s.deliverNext(ctx, now); err != nil {
				t.Fatal(err)
			}
			rows, err := db.NotificationDeliveries(ctx, u.ID, 0)
			if err != nil || len(rows) != 1 || rows[0].Status != tc.want || strings.Contains(rows[0].LastError, "SECRET") {
				t.Fatalf("delivery %+v %v", rows, err)
			}
			if tc.want == "pending" {
				if err := s.deliverNext(ctx, now.Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				if len(keys) != 1 {
					t.Fatal("retry too soon")
				}
				if err := s.deliverNext(ctx, now.Add(121*time.Second)); err != nil {
					t.Fatal(err)
				}
				if len(keys) != 2 || keys[0] != keys[1] {
					t.Fatalf("unstable id %v", keys)
				}
			}
		})
	}
}

func TestRetryAfterBoundedAndSupportsDates(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"-1", 0}, {"120", 2 * time.Minute}, {"2147483647", 24 * time.Hour}, {"not-a-date", 0}, {now.Add(time.Minute).Format(http.TimeFormat), time.Minute},
	} {
		if got := retryAfter(tc.value, now); got != tc.want {
			t.Fatalf("%q: %v want %v", tc.value, got, tc.want)
		}
	}
}
