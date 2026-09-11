package main

import (
	"bytes"
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestDatabaseWaitDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name string
		wait time.Duration
		want bool
	}{
		{"idle", 0, false},
		{"normal contention", time.Second, false},
		{"slow waits", 10 * time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			log := slog.New(slog.NewTextHandler(&out, nil))
			before := sql.DBStats{WaitCount: 10, WaitDuration: time.Hour}
			after := sql.DBStats{WaitCount: 12, WaitDuration: time.Hour + tc.wait, OpenConnections: 1, InUse: 1}
			logDatabaseWaits(log, "writer", before, after)
			if got := out.String(); (got != "") != tc.want || (tc.want && (!strings.Contains(got, "pool=writer") || !strings.Contains(got, "wait_count_delta=2"))) {
				t.Fatalf("unexpected diagnostics: %s", got)
			}
		})
	}
}
