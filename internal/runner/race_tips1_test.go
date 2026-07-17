package runner

import (
	"context"
	"encoding/json"
	"errors"
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
