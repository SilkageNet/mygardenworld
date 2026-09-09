package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestSchedulerWaitDiagnosticIsBoundedAndExplainsPacing(t *testing.T) {
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	r.pacer = newRequestPacer(RequestPacing{})
	r.sideLaneFirstWait = map[string]time.Time{}
	ch, cancel := r.bus.SubscribeLive(10)
	defer cancel()
	now := time.Unix(1_000_000, 0)
	side := runnableLaneOp("order.finish", automation.LaneSide, "order")
	side.Domain = "order.resident"
	r.sideLaneFirstWait["order"] = now
	ops := []automation.PlannedOp{side}
	r.emitSchedulerWaitDiagnostic(ops, nil, now.Add(time.Minute-time.Nanosecond))
	if len(ch) != 0 {
		t.Fatal("normal wait produced diagnostic")
	}
	recordTestPacingRequest(r, "other", now.Add(time.Minute))
	r.emitSchedulerWaitDiagnostic(ops, nil, now.Add(time.Minute))
	if len(ch) != 1 {
		t.Fatal("missing prolonged wait diagnostic")
	}
	event := <-ch
	var payload map[string]any
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if event.Kind != "scheduler_wait" || payload["send_delay_ms"] != float64(2000) || payload["wait_seconds"] != float64(60) {
		t.Fatalf("unexpected diagnostic: %+v", event)
	}
	for i := range 100 {
		r.emitSchedulerWaitDiagnostic(ops, nil, now.Add(time.Minute+time.Duration(i)*time.Millisecond))
	}
	if len(ch) != 0 {
		t.Fatal("wake flood produced log flood")
	}
	r.emitSchedulerWaitDiagnostic(ops, nil, now.Add(2*time.Minute))
	if len(ch) != 1 {
		t.Fatal("prolonged wait not reported again at interval")
	}
	<-ch
	delete(r.sideLaneFirstWait, "order")
	r.emitSchedulerWaitDiagnostic(ops, nil, now.Add(3*time.Minute))
	if len(ch) != 0 {
		t.Fatal("completed task still reported waiting")
	}
	r.resetSideLaneFairness()
	if !r.lastSchedulerWaitDiagnostic.IsZero() {
		t.Fatal("lifecycle kept diagnostic throttle")
	}
}
