package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestIsRaceTakeAlreadyTakenError(t *testing.T) {
	tips := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"codeOfLangJs":"fmlRace_tips1","msg":"已接取其他任务"}`)},
	}
	if !isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), tips) {
		t.Fatal("expected match")
	}
	other := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"codeOfLangJs":"fmlRace_other","msg":"其他"}`)},
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), other) {
		t.Fatal("must not match other codes")
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceFinishTask.String(), tips) {
		t.Fatal("must not match other RPCs")
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("已接取其他任务")) {
		t.Fatal("plain error without codeOfLangJs must not match")
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), nil) {
		t.Fatal("nil error must not match")
	}
}

func TestHandleOperationErrorRaceTakeTips1SoftRecover(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":1,"4":4001,"10":9}]}}`))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}

	tipsErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"codeOfLangJs":"fmlRace_tips1","msg":"已接取其他任务"}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "union",
		Domain:   "union.race",
		Action:   "take",
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              tipsErr,
		finishedAt:       time.Now(),
	})
	if got != nil {
		t.Fatalf("tips1 must soft-recover (nil), got %v", got)
	}
	view := r.state.FmlRace()
	if !view.TasksObserved || !view.TaskPoolStale {
		t.Fatalf("tips1 must retain the observed snapshot and mark it stale: %+v", view)
	}
}

func TestIsRaceTakeClaimedByOtherError(t *testing.T) {
	claimed := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"msg":"任务已被其他成员接取"}`)},
	}
	if !isRaceTakeClaimedByOtherError(clientproto.RPCFmlRaceTakeTask.String(), claimed) {
		t.Fatal("expected match")
	}
	if !isRaceTakeClaimedByOtherError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("rpc fmlRace.takeTask: server: 任务已被其他成员接取")) {
		t.Fatal("plain wrapped message must match")
	}
	if isRaceTakeClaimedByOtherError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("已接取其他任务")) {
		t.Fatal("must not match tips1 text")
	}
	if isRaceTakeClaimedByOtherError(clientproto.RPCFmlRaceFinishTask.String(), claimed) {
		t.Fatal("must not match other RPCs")
	}
}

func TestIsRaceTakeQuotaExceededError(t *testing.T) {
	quota := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"msg":"任务接取次数已达上限"}`)},
	}
	if !isRaceTakeQuotaExceededError(clientproto.RPCFmlRaceTakeTask.String(), quota) {
		t.Fatal("expected match")
	}
	if !isRaceTakeQuotaExceededError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("rpc fmlRace.takeTask: server: 任务接取次数已达上限")) {
		t.Fatal("plain wrapped message must match")
	}
	if isRaceTakeQuotaExceededError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("任务已被其他成员接取")) {
		t.Fatal("must not match claimed-by-other")
	}
}

func TestHandleOperationErrorRaceTakeClaimedByOtherSoftRecover(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":55,"4":4001,"5":3036,"10":9,"11":0}]}}`))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}
	if got := r.state.FmlRace().Tasks[0].UID; got != 0 {
		t.Fatalf("seed UID=%d, want 0", got)
	}

	claimedErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"msg":"任务已被其他成员接取"}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "union",
		Domain:   "union.race",
		Action:   "take",
		TaskMsID: 55,
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              claimedErr,
		finishedAt:       time.Now(),
	})
	if got != nil {
		t.Fatalf("claimed-by-other must soft-recover (nil), got %v", got)
	}
	view := r.state.FmlRace()
	if !view.TasksObserved || !view.TaskPoolStale {
		t.Fatalf("must retain the observed snapshot and mark it stale: %+v", view)
	}
	if view.Tasks[0].UID == 0 {
		t.Fatal("must MarkFmlRacePoolTaskClaimed so UID!=0")
	}
}

func TestHandleOperationErrorRaceTakeQuotaExceededSoftRecover(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":1,"4":4001,"10":9}]}}`))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}

	quotaErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"msg":"任务接取次数已达上限"}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "union",
		Domain:   "union.race",
		Action:   "take",
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              quotaErr,
		finishedAt:       time.Now(),
	})
	if got != nil {
		t.Fatalf("quota exceeded must soft-recover (nil), got %v", got)
	}
	view := r.state.FmlRace()
	if !view.TasksObserved || !view.TaskPoolStale {
		t.Fatalf("must retain the observed snapshot and mark it stale: %+v", view)
	}
	if !view.TakeQuotaExhausted {
		t.Fatal("must MarkFmlRaceTakeQuotaExhausted")
	}
}

func TestIsRaceTakeOnCooldownError(t *testing.T) {
	cd := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"msg":"任务冷却中"}`)},
	}
	if !isRaceTakeOnCooldownError(clientproto.RPCFmlRaceTakeTask.String(), cd) {
		t.Fatal("expected match")
	}
	if !isRaceTakeOnCooldownError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("rpc fmlRace.takeTask: server: 任务冷却中")) {
		t.Fatal("plain wrapped message must match")
	}
	if isRaceTakeOnCooldownError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("任务已被其他成员接取")) {
		t.Fatal("must not match claimed-by-other")
	}
	if isRaceTakeOnCooldownError(clientproto.RPCFmlRaceFinishTask.String(), cd) {
		t.Fatal("must not match other RPCs")
	}
}

func TestIsRaceDeleteOnCooldownError(t *testing.T) {
	cd := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceDelTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"msg":"任务冷却中"}`)},
	}
	if !isRaceDeleteOnCooldownError(clientproto.RPCFmlRaceDelTask.String(), cd) {
		t.Fatal("expected delete cooldown match")
	}
	if isRaceDeleteOnCooldownError(clientproto.RPCFmlRaceTakeTask.String(), cd) {
		t.Fatal("must not match takeTask")
	}
}

func TestHandleOperationErrorRaceDeleteOnCooldownWaitsAppearTime(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.UnixMilli(1_000_000)
	appear := now.Add(20 * time.Second).UnixMilli()
	r.state.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"0":42,"1":1},"114":[{"0":814,"4":4007,"5":%d,"10":21,"14":0,"15":0}]}}`,
		appear,
	)))
	cdErr := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceDelTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"msg":"任务冷却中"}`)},
	}
	op := &automation.PlannedOp{
		OperationID: "fmlRace.delTask:814",
		CooldownKey: "union.race.delete:814",
		Kind:        clientproto.RPCFmlRaceDelTask.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryRace,
		Domain:      "union.race.delete",
		Action:      "delete",
		TaskMsID:    814,
	}

	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              cdErr,
		finishedAt:       now,
	})
	if got != nil {
		t.Fatalf("delete cooldown must soft-defer, got %v", got)
	}
	cd, cooling := r.operationCoolingDown(op, now.Add(time.Second))
	if !cooling {
		t.Fatal("expected per-task delete cooldown")
	}
	if want := time.UnixMilli(appear); !cd.Until.Equal(want) {
		t.Fatalf("cooldown until=%v, want %v", cd.Until, want)
	}
	other := &automation.PlannedOp{
		OperationID: "fmlRace.delTask:815",
		CooldownKey: "union.race.delete:815",
		Kind:        clientproto.RPCFmlRaceDelTask.String(),
		Lane:        automation.LaneSide,
		TaskMsID:    815,
	}
	if _, cooling := r.operationCoolingDown(other, now.Add(time.Second)); cooling {
		t.Fatal("one task's delete cooldown must not block another task")
	}
}

func TestHandleOperationErrorRaceGetTaskListFailureRetriesIn1s(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":814,"4":4001,"5":3036,"10":9}]}}`))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}

	syncErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceGetTaskList,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"code":221,"args":[]}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceGetTaskList.String(),
		Lane:     automation.LaneSide,
		Category: "race",
		Domain:   "union.race.sync",
		Action:   "sync",
	}
	now := time.Now()
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              syncErr,
		finishedAt:       now,
	})
	if got != nil {
		t.Fatalf("getTaskList failure must soft-recover (nil), got %v", got)
	}
	if r.state.FmlRace().Observed {
		t.Fatal("code 221 must MarkFmlRaceSessionStale (Observed=false)")
	}
	cd, cooling := r.operationCoolingDown(op, now.Add(500*time.Millisecond))
	if !cooling {
		t.Fatal("expected 1s sync cooldown")
	}
	if d := cd.Until.Sub(now); d < time.Second || d > 2*time.Second {
		t.Fatalf("cooldown duration=%v, want ~1s not 60s", d)
	}
}

func TestHandleOperationErrorRaceTake221ForcesEnter(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":814,"4":4001,"5":3036,"10":9}]}}`))

	takeErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"code":221,"args":[]}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "race",
		Domain:   "union.race.take",
		Action:   "take",
		TaskMsID: 814,
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              takeErr,
		finishedAt:       time.Now(),
	})
	if got != nil {
		t.Fatalf("take 221 must soft-recover (nil), got %v", got)
	}
	view := r.state.FmlRace()
	if view.Observed || !view.TasksObserved || !view.TaskPoolStale {
		t.Fatalf("take 221 must stale the session while retaining the pool snapshot: %+v", view)
	}
}

func TestHandleOperationErrorRaceTakeOrdinaryFailureRetriesImmediately(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":814,"4":4001,"5":3036,"10":9,"11":0}]}}`))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}

	// Generic take reject (not code 221) — previously fell through to 60s backoff.
	takeErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"code":999,"msg":"接取失败"}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "race",
		Domain:   "union.race.take",
		Action:   "take",
		TaskMsID: 814,
	}
	now := time.Now()
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              takeErr,
		finishedAt:       now,
	})
	if got != nil {
		t.Fatalf("ordinary take failure must soft-recover (nil), got %v", got)
	}
	if view := r.state.FmlRace(); !view.TasksObserved || !view.TaskPoolStale {
		t.Fatalf("must mark the observed pool stale for immediate resync/retry: %+v", view)
	}
	if _, cooling := r.operationCoolingDown(op, now.Add(time.Second)); cooling {
		t.Fatal("must not apply 60s side-op backoff on take failure")
	}
}

func TestHandleOperationErrorRaceTakeOnCooldownWaitsAppearTime(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.UnixMilli(1_000_000)
	appear := now.Add(3 * time.Second).UnixMilli()
	r.state.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":814,"4":4007,"5":%d,"6":[23282],"10":21,"14":0,"15":0}]}}`,
		appear,
	)))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}

	cdErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"msg":"任务冷却中"}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "race",
		Domain:   "union.race.take",
		Action:   "take",
		TaskMsID: 814,
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              cdErr,
		finishedAt:       now,
	})
	if got != nil {
		t.Fatalf("task-cooldown must soft-recover (nil), got %v", got)
	}
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("cooldown retry must keep the current pool observed")
	}
	cd, cooling := r.operationCoolingDown(op, now)
	if !cooling {
		t.Fatal("expected take cooldown")
	}
	wantUntil := time.UnixMilli(appear)
	if !cd.Until.Equal(wantUntil) {
		t.Fatalf("cooldown until=%v, want %v (AppearTime)", cd.Until, wantUntil)
	}
	if d := cd.Until.Sub(now); d < 2*time.Second || d > 4*time.Second {
		t.Fatalf("cooldown duration=%v, want ~3s not 60s ordinary backoff", d)
	}
}

func TestHandleOperationSuccessMarksRaceProgressUnobserved(t *testing.T) {
	tests := []struct {
		name   string
		seed   string
		kind   string
		wantOK bool
	}{
		{
			name:   "flower art craft",
			seed:   `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3034,"2":5,"3":1}}},"114":[{"0":71,"4":3034,"7":5,"8":1,"10":24,"12":999}]}}`,
			kind:   clientproto.RPCFlowerArtMakeFlowerArt.String(),
			wantOK: true,
		},
		{
			name:   "flower art sell",
			seed:   `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3030,"2":5,"3":1}}},"114":[{"0":71,"4":3030,"7":5,"8":1,"10":24,"12":999}]}}`,
			kind:   clientproto.RPCFlowerRackSell.String(),
			wantOK: true,
		},
		{
			name:   "cultivate start",
			seed:   `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":4,"3":1}}},"114":[{"0":71,"4":3044,"7":4,"8":1,"10":36,"12":999}]}}`,
			kind:   clientproto.RPCCultivateCultivate.String(),
			wantOK: true,
		},
		{
			name:   "cultivate recv",
			seed:   `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":4,"3":1}}},"114":[{"0":71,"4":3044,"7":4,"8":1,"10":36,"12":999}]}}`,
			kind:   clientproto.RPCCultivateRecv.String(),
			wantOK: true,
		},
		{
			name:   "craft rpc while holding sell does not mark",
			seed:   `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3030,"2":5,"3":1}}},"114":[{"0":71,"4":3030,"7":5,"8":1,"10":24,"12":999}]}}`,
			kind:   clientproto.RPCFlowerArtMakeFlowerArt.String(),
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newOperationEventTestRunner()
			r.state.ApplyV(json.RawMessage(tc.seed))
			if !r.state.FmlRace().TasksObserved {
				t.Fatal("seed TasksObserved")
			}
			r.handleOperationSuccess(context.Background(), operationResult{
				operationAttempt: operationAttempt{
					op: &automation.PlannedOp{Kind: tc.kind, Category: "order", Domain: "test", Action: "test"},
				},
				finishedAt: time.Now(),
				raw:        json.RawMessage(`{}`),
			})
			got := r.state.FmlRace().TaskPoolStale
			if got != tc.wantOK {
				t.Fatalf("TaskPoolStale=%v, want %v (taken=%+v)", got, tc.wantOK, r.state.FmlRace().Taken)
			}
		})
	}
}
