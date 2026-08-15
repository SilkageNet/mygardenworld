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
	if r.state.FmlRace().TasksObserved {
		t.Fatal("tips1 must MarkFmlRaceTasksUnobserved")
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
	if view.TasksObserved {
		t.Fatal("must MarkFmlRaceTasksUnobserved")
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
	if view.TasksObserved {
		t.Fatal("must MarkFmlRaceTasksUnobserved")
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
	if r.state.FmlRace().TasksObserved {
		t.Fatal("must MarkFmlRaceTasksUnobserved so pool refresh runs before retry")
	}
	cd, cooling := r.operationCoolingDown(op, now)
	if !cooling {
		t.Fatal("expected take cooldown")
	}
	wantUntil := time.UnixMilli(appear).Add(10 * time.Millisecond)
	if !cd.Until.Equal(wantUntil) {
		t.Fatalf("cooldown until=%v, want %v (AppearTime+pad)", cd.Until, wantUntil)
	}
	if d := cd.Until.Sub(now); d < 2*time.Second || d > 4*time.Second {
		t.Fatalf("cooldown duration=%v, want ~3.01s not 60s ordinary backoff", d)
	}
}
