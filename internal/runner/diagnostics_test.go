package runner

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestDiagnosticsTracksOperationLifecycleAndBlocks(t *testing.T) {
	r := &Runner{state: state.New()}
	now := time.Now()
	r.setNextDecisionAt(now.Add(4 * time.Second))

	finish := r.beginOperation("usrLand.waterBatch")
	diag := r.Diagnostics(now)
	if diag.CurrentOperation != "usrLand.waterBatch" {
		t.Fatalf("CurrentOperation=%q, want usrLand.waterBatch", diag.CurrentOperation)
	}
	if diag.NextDecisionAt.IsZero() {
		t.Fatal("NextDecisionAt was not set")
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
