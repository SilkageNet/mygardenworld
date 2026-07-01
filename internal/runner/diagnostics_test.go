package runner

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsTracksOperationLifecycleAndBlocks(t *testing.T) {
	r := newStateHandlerTestRunner()
	now := time.Now()
	r.setNextDecisionAt(now.Add(4 * time.Second))
	r.setWaterBlockedUntil(now.Add(5 * time.Minute))

	finish := r.beginOperation("usrLand.waterBatch")
	diag := r.Diagnostics(now)
	if diag.CurrentOperation != "usrLand.waterBatch" {
		t.Fatalf("CurrentOperation=%q, want usrLand.waterBatch", diag.CurrentOperation)
	}
	if diag.NextDecisionAt.IsZero() {
		t.Fatal("NextDecisionAt was not set")
	}
	if !hasBlockedReason(diag.BlockedReasons, "缺水") {
		t.Fatalf("BlockedReasons=%v, want water block", diag.BlockedReasons)
	}

	finish(errors.New("rqst failed"))
	diag = r.Diagnostics(now)
	if diag.CurrentOperation != "" {
		t.Fatalf("CurrentOperation=%q, want cleared after finish", diag.CurrentOperation)
	}
	if diag.LastOperation != "usrLand.waterBatch" {
		t.Fatalf("LastOperation=%q, want usrLand.waterBatch", diag.LastOperation)
	}
	if !strings.Contains(diag.LastOperationError, "rqst failed") {
		t.Fatalf("LastOperationError=%q, want rqst failed", diag.LastOperationError)
	}
}

func hasBlockedReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, want) {
			return true
		}
	}
	return false
}
