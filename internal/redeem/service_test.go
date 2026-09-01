package redeem

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestOfflineOnlineOnlyAccountKeepsRedeemAttemptPending(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "phone-first", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}
	policy := automation.DefaultPolicy()
	policy.Basic.RedeemConnectMode = pb.RedeemConnectMode_REDEEM_CONNECT_MODE_ONLINE_ONLY
	raw, err := policycfg.ToJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, account.ID, raw); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertRedeemCode(ctx, store.RedeemCodeInput{
		Code: "WAIT-ONLINE", Channel: "ios", SourceKey: "test",
	}); err != nil {
		t.Fatal(err)
	}
	manager := runner.NewManager(db, runner.NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	service, err := NewService(ctx, db, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.processNextAttempt(ctx); err != nil {
		t.Fatal(err)
	}
	if manager.Get(account.ID) != nil {
		t.Fatal("redeem worker created a runner for an offline ONLINE_ONLY account")
	}
	records, summary, err := db.ListRedeemAttempts(ctx, store.ListRedeemAttemptsOptions{AccountID: account.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Status != store.RedeemValidationPending || records[0].AttemptCount != 0 || summary.Pending != 1 {
		t.Fatalf("redeem attempts=%+v summary=%+v, want untouched pending attempt", records, summary)
	}
}

func TestEligibleAccountIDsDefaultToAutoConnect(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	auto, err := db.CreateAccount(ctx, user.ID, "auto", "ios", "auto", "secret")
	if err != nil {
		t.Fatal(err)
	}
	onlineOnly, err := db.CreateAccount(ctx, user.ID, "online-only", "ios", "online", "secret")
	if err != nil {
		t.Fatal(err)
	}
	policy := automation.DefaultPolicy()
	policy.Basic.RedeemConnectMode = pb.RedeemConnectMode_REDEEM_CONNECT_MODE_ONLINE_ONLY
	raw, err := policycfg.ToJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, onlineOnly.ID, raw); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertRedeemCode(ctx, store.RedeemCodeInput{Code: "ELIGIBLE", Channel: "ios", SourceKey: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	manager := runner.NewManager(db, runner.NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	service, err := NewService(ctx, db, manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.eligibleAccountIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != auto.ID {
		t.Fatalf("eligible accounts=%v, want default-AUTO account %d only", got, auto.ID)
	}
}

func TestValidateSourceEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		native     bool
		wantErrSub string
	}{
		{name: "native local HTTP", url: "http://127.0.0.1:8080", native: true},
		{name: "custom requires HTTPS", url: "http://example.com/codes", wantErrSub: "HTTPS"},
		{name: "custom rejects literal private address", url: "https://127.0.0.1/codes", wantErrSub: "private or local"},
		{name: "credentials rejected", url: "https://user:secret@example.com", native: true, wantErrSub: "credentials"},
		{name: "fragment rejected", url: "https://example.com/codes#private", native: true, wantErrSub: "fragments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSourceEndpoint(tt.url, tt.native)
			if tt.wantErrSub == "" && err != nil {
				t.Fatalf("ValidateSourceEndpoint() error = %v", err)
			}
			if tt.wantErrSub != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrSub)) {
				t.Fatalf("ValidateSourceEndpoint() error = %v, want substring %q", err, tt.wantErrSub)
			}
		})
	}
}

func TestValidateCustomParserConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		wantErrSub string
	}{
		{name: "valid TTL", config: `{"type":"json_array","code_field":"code","time_field":"created","time_format":"unix","default_ttl_seconds":300}`},
		{name: "valid permanent", config: `{"type":"json_array","code_field":"gift","permanent":true}`},
		{name: "missing code", config: `{}`, wantErrSub: "code_field"},
		{name: "missing expiration rule", config: `{"code_field":"code"}`, wantErrSub: "expiration rule"},
		{name: "ambiguous expiration rule", config: `{"code_field":"code","permanent":true,"default_ttl_seconds":300}`, wantErrSub: "expiration rule"},
		{name: "negative TTL", config: `{"code_field":"code","default_ttl_seconds":-1}`, wantErrSub: "non-negative"},
		{name: "unsupported format", config: `{"code_field":"code","time_format":"date","default_ttl_seconds":300}`, wantErrSub: "time_format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCustomParserConfig(tt.config)
			if tt.wantErrSub == "" && err != nil {
				t.Fatalf("ValidateCustomParserConfig() error = %v", err)
			}
			if tt.wantErrSub != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrSub)) {
				t.Fatalf("ValidateCustomParserConfig() error = %v, want substring %q", err, tt.wantErrSub)
			}
		})
	}
}
