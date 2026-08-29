package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestRedeemCodeDeduplicatesPerChannelAndSupportsPermanent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	instanceID, err := db.RedeemInstanceID(ctx)
	if err != nil || instanceID == "" {
		t.Fatalf("instance id=%q err=%v", instanceID, err)
	}
	expires := time.Now().Add(time.Hour)
	first, created, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
		Code: " A\u0301BC ", Channel: "ios", ExpiresAt: &expires, SourceKey: "public:test",
	})
	if err != nil || !created {
		t.Fatalf("first upsert created=%v err=%v", created, err)
	}
	duplicate, created, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
		Code: "ÁBC", Channel: "ios", ExpiresAt: nil, SourceKey: "peer:test",
	})
	if err != nil || created {
		t.Fatalf("duplicate upsert created=%v err=%v", created, err)
	}
	if duplicate.ID != first.ID || duplicate.ExpiresAt != nil {
		t.Fatalf("duplicate=%+v first=%+v", duplicate, first)
	}
	stableRevision := duplicate.Revision
	repeated, created, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
		Code: "ÁBC", Channel: "ios", ExpiresAt: nil, SourceKey: "peer:test",
	})
	if err != nil || created {
		t.Fatalf("repeated upsert created=%v err=%v", created, err)
	}
	if repeated.Revision != stableRevision {
		t.Fatalf("repeated observation advanced revision from %d to %d", stableRevision, repeated.Revision)
	}
	alipay, created, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
		Code: "ÁBC", Channel: "alipay", ExpiresAt: nil, SourceKey: "public:test",
	})
	if err != nil || !created || alipay.ID == first.ID {
		t.Fatalf("channel-scoped upsert=%+v created=%v err=%v", alipay, created, err)
	}
}

func TestRedeemInvalidStopsRemainingAccountAttempts(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
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
	for _, username := range []string{"one", "two"} {
		if _, err := db.CreateAccount(ctx, user.ID, username, "ios", username, "secret"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{Code: "BAD", Channel: "ios", SourceKey: "public:test"}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	attempt, err := db.NextRedeemAttempt(ctx)
	if err != nil || attempt == nil {
		t.Fatalf("attempt=%+v err=%v", attempt, err)
	}
	if err := db.CompleteRedeemAttempt(ctx, attempt.ID, RedeemValidationInvalid, "invalid", nil); err != nil {
		t.Fatal(err)
	}
	next, err := db.NextRedeemAttempt(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("unexpected remaining attempt=%+v", next)
	}
}

func TestRedeemValidationControlsSourceHealthAndPropagation(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	origin, err := db.UpsertRedeemSource(ctx, RedeemSourceInput{
		Name: "origin", Type: RedeemSourceMyGardenWorld, BaseURL: "https://origin.example.test",
		Enabled: true, PushEnabled: true, PollIntervalSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	target, err := db.UpsertRedeemSource(ctx, RedeemSourceInput{
		Name: "target", Type: RedeemSourceMyGardenWorld, BaseURL: "https://target.example.test",
		Enabled: true, PushEnabled: true, PollIntervalSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAccount(ctx, user.ID, "ios", "ios", "owner", "secret"); err != nil {
		t.Fatal(err)
	}

	complete := func(code, validation string) int64 {
		t.Helper()
		expires := time.Now().Add(time.Hour)
		entry, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
			Code: code, Channel: "ios", ExpiresAt: &expires, SourceID: &origin.ID,
			SourceKey: "source:origin", OriginInstanceID: "origin-instance",
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.EnsureRedeemAttempts(ctx); err != nil {
			t.Fatal(err)
		}
		attempt, err := db.NextRedeemAttempt(ctx)
		if err != nil || attempt == nil || attempt.CodeID != entry.ID {
			t.Fatalf("attempt=%+v entry=%+v err=%v", attempt, entry, err)
		}
		if err := db.CompleteRedeemAttempt(ctx, attempt.ID, validation, validation, nil); err != nil {
			t.Fatal(err)
		}
		return entry.ID
	}

	successID := complete("GOOD", RedeemValidationSuccess)
	invalidID := complete("BAD", RedeemValidationInvalid)
	_ = complete("OLD", RedeemValidationExpired)

	sources, err := db.ListRedeemSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sources[0].AcceptedCount != 1 || sources[0].InvalidCount != 1 {
		t.Fatalf("origin health accepted=%d invalid=%d", sources[0].AcceptedCount, sources[0].InvalidCount)
	}
	if sources[1].AcceptedCount != 0 || sources[1].InvalidCount != 0 {
		t.Fatalf("target health accepted=%d invalid=%d", sources[1].AcceptedCount, sources[1].InvalidCount)
	}
	var successToTarget, successToOrigin, invalidOutbox int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_exchange_outbox WHERE redeem_code_id = ? AND source_id = ?`, successID, target.ID).Scan(&successToTarget); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_exchange_outbox WHERE redeem_code_id = ? AND source_id = ?`, successID, origin.ID).Scan(&successToOrigin); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_exchange_outbox WHERE redeem_code_id = ?`, invalidID).Scan(&invalidOutbox); err != nil {
		t.Fatal(err)
	}
	if successToTarget != 1 || successToOrigin != 0 || invalidOutbox != 0 {
		t.Fatalf("outbox target=%d origin=%d invalid=%d", successToTarget, successToOrigin, invalidOutbox)
	}
}
