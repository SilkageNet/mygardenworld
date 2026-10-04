package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestRPCObservationWindowCountsResponsesNotPlans(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.Now()
	r.observeGameRPCAt("", babigame.WSResponseD{}, now)
	r.observeGameRPCAt("old", babigame.WSResponseD{}, now.Add(-time.Minute-time.Nanosecond))
	r.observeGameRPCAt("usr.heartTick", babigame.WSResponseD{}, now.Add(-time.Minute))
	r.observeGameRPCAt("fmlRace.getTaskList", babigame.WSResponseD{}, now)
	r.observeGameRPCAt("fmlRace.getTaskList", babigame.WSResponseD{M: json.RawMessage(`{"code":5000}`)}, now)
	r.observeGameRPCAt("pearlPlace.hire", babigame.WSResponseD{M: json.RawMessage(`{"code":"pearl_tips4"}`)}, now)
	got := r.rpcObservations.snapshot(now)
	if got.Responses != 4 || got.Limited || len(got.ByRPC) != 3 {
		t.Fatalf("unexpected snapshot %+v", got)
	}
	if rpc := got.ByRPC["fmlRace.getTaskList"]; rpc.Responses != 2 || rpc.Errors != 1 || rpc.ErrorCodes[5000] != 1 {
		t.Fatalf("unexpected RPC counts %+v", rpc)
	}
	if got.ByRPC["pearlPlace.hire"].Errors != 1 {
		t.Fatal("text-only error not counted")
	}
	if got := r.rpcObservations.snapshot(now.Add(time.Minute + time.Nanosecond)); got.Responses != 0 {
		t.Fatal("expired observations retained")
	}
}

func TestRPCObservationWindowIsBoundedAndSnapshotIsIndependent(t *testing.T) {
	var w rpcObservationWindow
	now := time.Now()
	for range rpcObservationLimit + 1 {
		w.record("test", 5000, true, now)
	}
	got := w.snapshot(now)
	if len(w.items) != rpcObservationLimit || got.Responses != rpcObservationLimit || !got.Limited {
		t.Fatal("unbounded/misleading observation window")
	}
	got.ByRPC["test"].ErrorCodes[5000] = 0
	if w.snapshot(now).ByRPC["test"].ErrorCodes[5000] != rpcObservationLimit {
		t.Fatal("snapshot changed retained observations")
	}
	w.record("new", 0, false, now.Add(time.Minute+time.Nanosecond))
	if got := w.snapshot(now.Add(time.Minute + time.Nanosecond)); got.Responses != 1 || got.Limited {
		t.Fatal("old cap/window leaked into new interval")
	}
}

func TestProtectionEventIncludesBoundedRPCSummary(t *testing.T) {
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	events, unsubscribe := r.bus.SubscribeLive(10)
	defer unsubscribe()
	now := time.Now()
	r.observeGameRPCAt("usr.heartTick", babigame.WSResponseD{}, now)
	for _, rpc := range []string{"fmlRace.getTaskList", "fmlRace.getTaskList", "usr.lazySync"} {
		r.observeGameRPCAt(rpc, babigame.WSResponseD{M: json.RawMessage(`{"code":5000}`)}, now)
	}
	select {
	case event := <-events:
		var payload struct {
			Responses rpcResponseSnapshot `json:"recent_rpc_responses"`
		}
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			t.Fatal(err)
		}
		if event.Kind != "account_request_paused" || payload.Responses.Responses != 4 || payload.Responses.ByRPC["fmlRace.getTaskList"].Errors != 2 {
			t.Fatalf("missing response summary: %+v", event)
		}
	default:
		t.Fatal("missing protection event")
	}
}
