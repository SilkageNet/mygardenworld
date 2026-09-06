package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestPearlHireDiagnosticsThrottlesAndExplainsDisabledPolicy(t *testing.T) {
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	events, cancel := r.bus.SubscribeLive(20)
	defer cancel()
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.AutoHireEnabled = false
	snapshot := tickSnapshot{policy: p, client: &babigame.Client{}, session: &babigame.Session{}}
	now := time.Now()
	r.emitPearlHireDiagnostic(snapshot, now)
	if len(events) != 1 {
		t.Fatal("disabled policy was not logged")
	}
	event := <-events
	var d automation.PearlHireDiagnostic
	if err := json.Unmarshal([]byte(event.PayloadJSON), &d); err != nil || d.Status != "disabled" || event.Category != "basic" || event.Kind != "pearl_hire_diagnostic" {
		t.Fatalf("diagnostic event=%+v error=%v", event, err)
	}
	r.emitPearlHireDiagnostic(snapshot, now.Add(4*time.Second))
	r.emitPearlHireDiagnostic(snapshot, now.Add(10*time.Minute))
	if len(events) != 0 {
		t.Fatal("unchanged disabled policy spammed logs")
	}
	p.Basic.Pearl.AutoHireEnabled = true
	p.Basic.Pearl.MaxHireTicketUsage = 0
	now = now.Add(10*time.Minute + time.Second)
	r.emitPearlHireDiagnostic(snapshot, now)
	if len(events) != 1 {
		t.Fatal("policy change did not bypass sampling interval")
	}
	<-events
	r.emitPearlHireDiagnostic(snapshot, now.Add(4*time.Second))
	r.emitPearlHireDiagnostic(snapshot, now.Add(time.Minute))
	if len(events) != 0 {
		t.Fatal("unchanged active blocker spammed logs")
	}
	r.emitPearlHireDiagnostic(snapshot, now.Add(pearlDiagnosticRepeat))
	if len(events) != 1 {
		t.Fatal("active blocker lost its periodic diagnostic")
	}
}

func TestPearlHireDiagnosticsLogsOfflineTransitionWithoutRPC(t *testing.T) {
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	events, cancel := r.bus.SubscribeLive(20)
	defer cancel()
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.AutoHireEnabled = true
	p.Basic.Pearl.MaxHireTicketUsage = 1
	snapshot := tickSnapshot{policy: p, client: &babigame.Client{}, session: &babigame.Session{}}
	now := time.Now()
	r.emitPearlHireDiagnostic(snapshot, now)
	<-events
	snapshot.sessionInvalidated = true
	r.emitPearlHireDiagnostic(snapshot, now.Add(time.Second))
	if len(events) != 1 {
		t.Fatal("session loss was not logged promptly")
	}
	var d automation.PearlHireDiagnostic
	if err := json.Unmarshal([]byte((<-events).PayloadJSON), &d); err != nil || d.Status != "disconnected" || d.TargetUID != 0 || len(d.Sources) != 0 {
		t.Fatalf("offline diagnostics claimed current eligibility: %+v error=%v", d, err)
	}
}
