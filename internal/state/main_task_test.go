package state

import (
	"encoding/json"
	"testing"
)

func TestMainTaskCatalogActiveChainAndTerminal(t *testing.T) {
	definitions, endTaskID, ok := mainTaskCatalogDefinitions()
	if !ok {
		t.Fatal("mainTaskCatalogDefinitions ok=false")
	}
	if endTaskID != 6940001 || len(definitions) != 694 {
		t.Fatalf("end=%d definitions=%d, want 6940001/694", endTaskID, len(definitions))
	}

	definition, valid, complete := ResolveMainTaskDefinition(910001)
	if !valid || complete || definition.Target != 14 || definition.NextTaskID != 920001 || definition.EndTaskID != endTaskID {
		t.Fatalf("ResolveMainTaskDefinition(910001)=(%+v,%t,%t)", definition, valid, complete)
	}
	definition, valid, complete = ResolveMainTaskDefinition(endTaskID)
	if !valid || complete || definition.Target != 144 || definition.NextTaskID != 6950001 {
		t.Fatalf("ResolveMainTaskDefinition(end)=(%+v,%t,%t)", definition, valid, complete)
	}
	if _, valid, complete = ResolveMainTaskDefinition(6950001); !valid || !complete {
		t.Fatalf("ResolveMainTaskDefinition(6950001) valid=%t complete=%t", valid, complete)
	}
	if _, valid, complete = ResolveMainTaskDefinition(7000001); !valid || !complete {
		t.Fatalf("ResolveMainTaskDefinition(7000001) valid=%t complete=%t", valid, complete)
	}
	if _, valid, complete = ResolveMainTaskDefinition(12345); valid || complete {
		t.Fatalf("unknown task valid=%t complete=%t", valid, complete)
	}
}

func TestApplyMainTaskSparseCaptureFixture(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{"900001":1}}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool {
		return task.Observed && task.Valid && !task.Complete && task.TaskID == 910001 &&
			task.Finished == 14 && task.Target == 14 && task.NextTaskID == 920001 &&
			task.ProgressObserved && task.ReceiptObserved && !task.Receipted
	})
	snapshot, ok := s.MainTaskClaimSnapshot()
	if !ok || snapshot.TaskID != 910001 || snapshot.Target != 14 || snapshot.NextTaskID != 920001 {
		t.Fatalf("initial claim snapshot=(%+v,%t)", snapshot, ok)
	}

	// Observed production deltas carry only curValue/curRecord/uTime. Missing
	// curTaskId and recvMap must preserve the previous values.
	s.ApplyV(json.RawMessage(`{"22":{"0":{"2":15,"3":{"2":1000},"5":1000}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool {
		return task.TaskID == 910001 && task.Finished == 15 && task.ProgressObserved && task.ReceiptObserved
	})

	// taskMain.recv returns the next task, a reset progress, and a full receipt
	// map containing the task that was just claimed.
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":920001,"2":0,"3":{"2":2000},"4":{"900001":1,"910001":1},"5":2000}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool {
		return task.Valid && task.TaskID == 920001 && task.Finished == 0 && task.Target == 24 &&
			task.ProgressObserved && task.ReceiptObserved && !task.Receipted
	})
	if !s.MainTaskClaimApplied(snapshot) {
		t.Fatal("exact next task plus receipt did not confirm claim")
	}
}

func TestApplyMainTaskSwitchWithoutProgressFailsClosed(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":910001,"2":99,"4":{}}}}`))
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":920001}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool {
		return task.Valid && task.TaskID == 920001 && task.Finished == 0 && !task.ProgressObserved && task.ReceiptObserved
	})
	if snapshot, ok := s.MainTaskClaimSnapshot(); ok {
		t.Fatalf("claim snapshot from unknown switched progress: %+v", snapshot)
	}
}

func TestApplyMainTaskReceiptReplacementAndNull(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":920001,"2":24,"4":{"920001":0}}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool { return task.Receipted && task.ReceiptObserved })
	if snapshot, ok := s.MainTaskClaimSnapshot(); ok {
		t.Fatalf("zero-valued receipt key must fail closed: %+v", snapshot)
	}

	// A present map replaces rather than merges the prior full map.
	s.ApplyV(json.RawMessage(`{"22":{"0":{"4":{}}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool { return !task.Receipted && task.ReceiptObserved })
	if _, ok := s.MainTaskClaimSnapshot(); !ok {
		t.Fatal("replacement empty receipt map should make completed task claimable")
	}

	s.ApplyV(json.RawMessage(`{"22":{"0":{"4":null}}}`))
	assertMainTask(t, s, func(task MainTaskView) bool { return !task.Receipted && task.ReceiptObserved })
	if _, ok := s.MainTaskClaimSnapshot(); !ok {
		t.Fatal("null receipt map should be observed empty")
	}
}

func TestApplyMainTaskStrictFieldsAndTerminal(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		ok   func(MainTaskView) bool
	}{
		{name: "quoted task", raw: json.RawMessage(`{"22":{"0":{"1":"910001","2":14,"4":{}}}}`), ok: func(task MainTaskView) bool { return !task.TaskIDObserved && !task.Valid }},
		{name: "fraction progress", raw: json.RawMessage(`{"22":{"0":{"1":910001,"2":14.5,"4":{}}}}`), ok: func(task MainTaskView) bool { return task.TaskIDObserved && !task.ProgressObserved && task.Valid }},
		{name: "negative progress", raw: json.RawMessage(`{"22":{"0":{"1":910001,"2":-1,"4":{}}}}`), ok: func(task MainTaskView) bool { return !task.ProgressObserved }},
		{name: "malformed receipts", raw: json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{"910001":"1"}}}}`), ok: func(task MainTaskView) bool { return !task.ReceiptObserved }},
		{name: "fraction receipt", raw: json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{"910001":1.5}}}}`), ok: func(task MainTaskView) bool { return !task.ReceiptObserved }},
		{name: "overflow receipt", raw: json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{"910001":2147483648}}}}`), ok: func(task MainTaskView) bool { return !task.ReceiptObserved }},
		{name: "noncanonical receipt key", raw: json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{"0910001":1}}}}`), ok: func(task MainTaskView) bool { return !task.ReceiptObserved }},
		{name: "unknown below end", raw: json.RawMessage(`{"22":{"0":{"1":12345,"2":99,"4":{}}}}`), ok: func(task MainTaskView) bool { return !task.Valid && !task.Complete }},
		{name: "terminal", raw: json.RawMessage(`{"22":{"0":{"1":6950001}}}`), ok: func(task MainTaskView) bool { return task.Valid && task.Complete && task.TaskID == 6950001 }},
		{name: "later terminal", raw: json.RawMessage(`{"22":{"0":{"1":7000001}}}`), ok: func(task MainTaskView) bool { return task.Valid && task.Complete && task.TaskID == 7000001 }},
		{name: "null main", raw: json.RawMessage(`{"22":{"0":null}}`), ok: func(task MainTaskView) bool { return task.Observed && !task.Valid && !task.TaskIDObserved }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(tc.raw)
			assertMainTask(t, s, tc.ok)
		})
	}
}

func TestMainTaskClaimAppliedRequiresExactSwitchAndReceipt(t *testing.T) {
	initial := json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{}}}}`)
	tests := []struct {
		name     string
		response json.RawMessage
		want     bool
	}{
		{name: "exact", response: json.RawMessage(`{"22":{"0":{"1":920001,"2":0,"4":{"910001":1}}}}`), want: true},
		{name: "zero receipt still fail closed", response: json.RawMessage(`{"22":{"0":{"1":920001,"2":0,"4":{"910001":0}}}}`), want: true},
		{name: "switch only", response: json.RawMessage(`{"22":{"0":{"1":920001,"2":0}}}`)},
		{name: "receipt only", response: json.RawMessage(`{"22":{"0":{"4":{"910001":1}}}}`)},
		{name: "wrong switch", response: json.RawMessage(`{"22":{"0":{"1":930001,"2":0,"4":{"910001":1}}}}`)},
		{name: "next progress missing", response: json.RawMessage(`{"22":{"0":{"1":920001,"4":{"910001":1}}}}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(initial)
			snapshot, ok := s.MainTaskClaimSnapshot()
			if !ok {
				t.Fatal("claim snapshot unavailable")
			}
			s.ApplyV(tc.response)
			if got := s.MainTaskClaimApplied(snapshot); got != tc.want {
				t.Fatalf("MainTaskClaimApplied=%t want %t", got, tc.want)
			}
		})
	}
}

func TestMainTaskClaimAppliedAcceptsExactTerminal(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":6940001,"2":144,"4":{}}}}`))
	snapshot, ok := s.MainTaskClaimSnapshot()
	if !ok || snapshot.NextTaskID != 6950001 {
		t.Fatalf("terminal snapshot=(%+v,%t)", snapshot, ok)
	}
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":6950001,"4":{"6940001":1}}}}`))
	if !s.MainTaskClaimApplied(snapshot) {
		t.Fatal("exact terminal transition not confirmed")
	}
}

func assertMainTask(t *testing.T, s *State, accept func(MainTaskView) bool) {
	t.Helper()
	task, ok := s.MainTask()
	if !ok || !accept(task) {
		t.Fatalf("MainTask=(%+v,%t)", task, ok)
	}
}
