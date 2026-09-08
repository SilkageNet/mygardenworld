package runner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestUrgentRaceReservesOnlyFinalPacingWindow(t *testing.T) {
	now := time.Now()
	r := newSideLaneTestRunner()
	r.pacer = newRequestPacer(RequestPacing{})
	take := runnableLaneOp("fmlRace.takeTask", automation.LaneSide, "union.race.take")
	take.PreemptFarm, take.Action = true, "take"
	farm := runnableLaneOp("usrLand.harvest", automation.LaneFarm, "")
	r.pacer.lastScope[take.Kind] = now
	candidates := []automation.PlannedOp{farm, take}
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now), farm.OperationID)
	if op := r.selectRunnableOperation(candidates, now.Add(7*time.Second)); op != nil {
		t.Fatalf("ordinary work stole urgent slot: %+v", op)
	}
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(8*time.Second)), take.OperationID)
}

func TestExpiredReusedRacePoolIsRejectedAfterQueueing(t *testing.T) {
	r := newOperationEventTestRunner()
	op := &automation.PlannedOp{Kind: clientproto.RPCFmlRaceTakeTask.String(), TaskMsID: 1}
	timing := &raceTiming{reusedPool: true}
	ctx := context.WithValue(t.Context(), raceMutationContextKey{}, op)
	ctx = context.WithValue(ctx, raceTimingKey{}, timing)
	r.state.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036}]}}`))
	if err := r.validateRaceMutationBeforeSend(ctx, op.Kind); err == nil {
		t.Fatal("expired/missing evidence reached mutation admission")
	}
	if !r.state.FmlRace().TaskPoolStale || !timing.admitted.IsZero() {
		t.Fatal("rejection must require a new full list without admission")
	}
}

func TestFullRaceListValidationRejectsMalformedPayload(t *testing.T) {
	for _, tc := range []struct {
		payload string
		want    bool
	}{
		{`{"25":{"114":[]}}`, true}, {`{"25":{"114":null}}`, true},
		{`{"25":{"114":{}}}`, false}, {`{"25":{"114":"bad"}}`, false}, {`{"25":{}}`, false},
	} {
		if got := fmlRaceTaskListPresent(json.RawMessage(tc.payload)); got != tc.want {
			t.Fatalf("%s valid=%v", tc.payload, got)
		}
	}
}

func TestScheduledHarvestChunkPreservesOriginalAndRealBatch(t *testing.T) {
	for _, kind := range []string{clientproto.RPCUsrLandHarvest.String(), clientproto.RPCFmlLandHarvest.String(), clientproto.RPCUsrLandPlantBatch.String()} {
		op := &automation.PlannedOp{Kind: kind, LandIDs: []int32{1, 2, 3}}
		chunk := scheduledOperationChunk(op)
		want := 3
		if kind == clientproto.RPCUsrLandHarvest.String() {
			want = 1
		}
		if len(chunk.LandIDs) != want || len(op.LandIDs) != 3 {
			t.Fatalf("chunk=%+v original=%+v", chunk, op)
		}
		if want == 1 {
			chunk.LandIDs[0] = 99
			if op.LandIDs[0] != 1 {
				t.Fatal("chunk aliases original")
			}
		}
	}
}

func TestDecisionWakeCoalesces(t *testing.T) {
	r := &Runner{}
	r.wakeDecision() // nil test/lifecycle channel must not block
	r.decisionWake = make(chan struct{}, 1)
	for range 100 {
		r.wakeDecision()
	}
	if len(r.decisionWake) != 1 {
		t.Fatal("wake not coalesced")
	}
}

func TestReusableRaceTakePool(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name                     string
		age                      time.Duration
		stale, missing, deletion bool
		want                     bool
	}{
		{name: "fresh", age: time.Second, want: true},
		{name: "boundary", age: 3 * time.Second, want: true},
		{name: "old", age: 3*time.Second + time.Millisecond},
		{name: "clock rollback", age: -time.Second},
		{name: "stale", stale: true},
		{name: "push only", missing: true},
		{name: "deletion always refreshes", deletion: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := state.FmlRaceView{TasksObserved: true, TaskPoolStale: tc.stale, FullTasksSyncedAtMs: now.Add(-tc.age).UnixMilli()}
			if tc.missing {
				view.FullTasksSyncedAtMs = 0
			}
			op := &automation.PlannedOp{Kind: clientproto.RPCFmlRaceTakeTask.String()}
			if tc.deletion {
				op.Kind = clientproto.RPCFmlRaceDelTask.String()
			}
			if got := reusableRaceTakePool(view, op, time.UnixMilli(now.UnixMilli())); got != tc.want {
				t.Fatalf("reuse=%v want=%v", got, tc.want)
			}
		})
	}
}
