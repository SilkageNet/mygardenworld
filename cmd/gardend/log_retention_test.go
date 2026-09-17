package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestLogRetentionDuration(t *testing.T) {
	tests := []struct {
		name    string
		days    int
		want    time.Duration
		wantErr bool
	}{
		{name: "disabled", days: 0},
		{name: "one day is valid", days: 1, want: 24 * time.Hour},
		{name: "default", days: defaultLogRetentionDays, want: 7 * 24 * time.Hour},
		{name: "negative", days: -1, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := logRetentionDuration(tc.days)
			if (err != nil) != tc.wantErr {
				t.Fatalf("logRetentionDuration(%d) error=%v, wantErr=%t", tc.days, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("logRetentionDuration(%d)=%s, want %s", tc.days, got, tc.want)
			}
		})
	}
}

func TestServeCommandDefaultsToSevenDayLogRetention(t *testing.T) {
	flag := newServeCmd().Flags().Lookup("log-retention-days")
	if flag == nil || flag.DefValue != "7" {
		t.Fatalf("log-retention-days flag=%+v, want default 7", flag)
	}
}

func TestRunLogCleanupLoopReportsDisabledPolicyWithoutOpeningDB(t *testing.T) {
	var output bytes.Buffer
	log := slog.New(slog.NewTextHandler(&output, nil))
	runLogCleanupLoop(context.Background(), nil, log, 0)
	if got := output.String(); !strings.Contains(got, "log retention disabled") || !strings.Contains(got, "retention_days=0") {
		t.Fatalf("disabled retention log=%q", got)
	}
}

func TestCleanExpiredLogsConsumesDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	log := slog.New(slog.NewTextHandler(&output, nil))

	cleanExpiredLogs(ctx, db, log, time.Now().UTC(), 24*time.Hour)
	if !strings.Contains(output.String(), "clean expired logs failed") {
		t.Fatalf("cleanup error was not reported: %q", output.String())
	}

	// Keep the no-op logger shape used by production callers covered as well.
	cleanExpiredLogs(ctx, db, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Now().UTC(), 24*time.Hour)
}

func TestKeepForeverStillReclaimsDeletedSpace(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	u, err := db.CreateUser(ctx, "owner", "o@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "main", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.LogEvent(ctx, store.EventLog{AccountID: a.ID, Kind: "keep", TS: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE deleted_fixture(data BLOB); INSERT INTO deleted_fixture VALUES(zeroblob(2097152)); DELETE FROM deleted_fixture`); err != nil {
		t.Fatal(err)
	}
	before, err := db.DatabaseSpace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	maintainDatabase(ctx, db, log, time.Date(2026, 9, 17, 12, 1, 0, 0, time.UTC), 0)
	after, err := db.DatabaseSpace(ctx)
	if err != nil || after.Pages >= before.Pages {
		t.Fatalf("before=%+v after=%+v err=%v", before, after, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM event_log`).Scan(&count); err != nil || count != 1 {
		t.Fatal("keep-forever log was deleted")
	}
}
