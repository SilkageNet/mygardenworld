package store

import (
	"context"
	"path/filepath"
	"strconv"
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
	expires := time.Now().Add(time.Hour)
	if _, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
		Code: "GOOD", Channel: "ios", ExpiresAt: &expires, SourceID: &target.ID,
		SourceKey: "source:target", OriginInstanceID: "target-instance",
	}); err != nil {
		t.Fatal(err)
	}

	sources, err := db.ListRedeemSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sources[0].ObservedCount != 3 || sources[0].TrustedCount != 2 || sources[0].SuccessCount != 1 ||
		sources[0].AlreadyRedeemedCount != 0 || sources[0].ExpiredCount != 1 ||
		sources[0].InvalidCount != 1 || sources[0].PendingCount != 0 {
		t.Fatalf("origin stats=%+v", sources[0])
	}
	if sources[1].ObservedCount != 1 || sources[1].TrustedCount != 1 || sources[1].SuccessCount != 1 ||
		sources[1].InvalidCount != 0 || sources[1].PendingCount != 0 {
		t.Fatalf("target stats=%+v", sources[1])
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

func TestDueRedeemSourcesUsesTypedLastSyncTime(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	source, err := db.UpsertRedeemSource(ctx, RedeemSourceInput{
		Name: "scheduled", Type: RedeemSourceCustomHTTP,
		BaseURL: "https://example.test/codes.json", Channel: "ios",
		ParserConfigJSON: `{"type":"json_array","code_field":"code","permanent":true}`,
		Enabled:          true, PollIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateRedeemSourceSync(ctx, source.ID, "", "", ""); err != nil {
		t.Fatal(err)
	}
	source, err = db.GetRedeemSource(ctx, source.ID)
	if err != nil || source.LastSyncAt == nil {
		t.Fatalf("source=%+v err=%v", source, err)
	}
	before, err := db.DueRedeemSources(ctx, source.LastSyncAt.Add(59*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("source became due before interval: %+v", before)
	}
	due, err := db.DueRedeemSources(ctx, source.LastSyncAt.Add(60*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != source.ID {
		t.Fatalf("due sources=%+v, want source %d", due, source.ID)
	}
}

func TestListRedeemAttemptsScopesSummarizesFiltersAndPaginates(t *testing.T) {
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
	account, err := db.CreateAccount(ctx, user.ID, "ios", "ios", "owner", "secret")
	if err != nil {
		t.Fatal(err)
	}
	statuses := []string{
		RedeemValidationSuccess,
		RedeemValidationAlreadyRedeemed,
		RedeemValidationExpired,
		RedeemValidationInvalid,
		RedeemValidationPending,
		RedeemAttemptStatusRunning,
		RedeemValidationRetryable,
		RedeemValidationUnknown,
	}
	expires := time.Now().Add(time.Hour)
	for index := range statuses {
		if _, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
			Code:      "CODE-" + strconv.Itoa(index),
			Channel:   "ios",
			ExpiresAt: &expires,
			SourceKey: "test:" + strconv.Itoa(index),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	for index, status := range statuses {
		if _, err := db.ExecContext(ctx, `
UPDATE redeem_attempts
SET status = ?, message = ?, attempt_count = ?, attempted_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND redeem_code_id = (SELECT id FROM redeem_codes WHERE code = ?)`,
			status, status, index+1, account.ID, "CODE-"+strconv.Itoa(index)); err != nil {
			t.Fatal(err)
		}
	}

	all, summary, err := db.ListRedeemAttempts(ctx, ListRedeemAttemptsOptions{
		AccountID: account.ID,
		Limit:     20,
		Filter:    RedeemAttemptFilterAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(statuses) || all[0].Code != "CODE-7" || all[len(all)-1].Code != "CODE-0" {
		t.Fatalf("all attempts=%+v", all)
	}
	if summary.Total != 8 || summary.Success != 1 || summary.AlreadyRedeemed != 1 ||
		summary.Expired != 1 || summary.Invalid != 1 || summary.Pending != 1 ||
		summary.Running != 1 || summary.Retryable != 1 || summary.Unknown != 1 {
		t.Fatalf("summary=%+v", summary)
	}

	tests := []struct {
		filter string
		want   int
	}{
		{RedeemAttemptFilterRedeemed, 2},
		{RedeemAttemptFilterUnavailable, 2},
		{RedeemAttemptFilterAttention, 4},
	}
	for _, test := range tests {
		records, gotSummary, err := db.ListRedeemAttempts(ctx, ListRedeemAttemptsOptions{
			AccountID: account.ID,
			Limit:     20,
			Filter:    test.filter,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != test.want || gotSummary != summary {
			t.Fatalf("filter=%s records=%d summary=%+v", test.filter, len(records), gotSummary)
		}
	}

	first, _, err := db.ListRedeemAttempts(ctx, ListRedeemAttemptsOptions{AccountID: account.ID, Limit: 3})
	if err != nil || len(first) != 3 {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	second, _, err := db.ListRedeemAttempts(ctx, ListRedeemAttemptsOptions{
		AccountID: account.ID,
		BeforeID:  first[len(first)-1].ID,
		Limit:     3,
	})
	if err != nil || len(second) != 3 || second[0].ID >= first[len(first)-1].ID {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
}
