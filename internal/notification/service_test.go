package notification

import (
	"context"
	"errors"
	"io"
	"net/http"
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
			if err := db.SaveNotificationSettings(ctx, u.ID, store.NotificationUpdate{Enabled: true, Endpoint: &endpoint, CooldownMinutes: 30, Provider: "custom"}); err != nil {
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

func TestDeliverNativeAcknowledgementAndRateLimit(t *testing.T) {
	for _, tc := range []struct{ name, body, status string }{
		{"accepted", `{"errcode":0}`, "sent"},
		{"rejected", `{"errcode":310000,"errmsg":"SECRET"}`, "failed"},
		{"limited", `{"errcode":410100,"errmsg":"SECRET"}`, "pending"},
		{"invalid", `{"errcode":null}`, "failed"},
		{"oversized", strings.Repeat(" ", 65537) + `{"errcode":0}`, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, now := context.Background(), time.Now().UTC()
			db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
			if err != nil {
				t.Fatal(err)
			}
			endpoint, secret := "https://oapi.dingtalk.com/robot/send?access_token=fixture", "SECRET"
			if err := db.SaveNotificationSettings(ctx, u.ID, store.NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "dingtalk", SigningSecret: &secret, CooldownMinutes: 30}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.QueueNotificationTest(ctx, u.ID, now); err != nil {
				t.Fatal(err)
			}
			service := New(db, nil)
			requests := 0
			service.client = &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				body, _ := io.ReadAll(r.Body)
				if r.URL.Query().Get("sign") == "" || r.Header.Get("X-Notification-ID") == "" || !strings.Contains(string(body), "测试通知") || strings.Contains(string(body), secret) {
					t.Fatal("incorrect signed delivery")
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			if err := service.deliverNext(ctx, now); err != nil {
				t.Fatal(err)
			}
			rows, err := db.NotificationDeliveries(ctx, u.ID, 0)
			if err != nil || len(rows) != 1 || rows[0].Status != tc.status || strings.Contains(rows[0].LastError, secret) {
				t.Fatalf("result=%+v err=%v", rows, err)
			}
			if tc.status == "pending" {
				if err := service.deliverNext(ctx, now.Add(9*time.Minute)); err != nil {
					t.Fatal(err)
				}
				if requests != 1 {
					t.Fatal("retry within platform's ten-minute limit")
				}
				if err := service.deliverNext(ctx, now.Add(10*time.Minute)); err != nil {
					t.Fatal(err)
				}
				if requests != 2 {
					t.Fatal("retry not released")
				}
			}
		})
	}
}
