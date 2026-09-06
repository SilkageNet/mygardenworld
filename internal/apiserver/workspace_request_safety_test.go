package apiserver

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
)

func TestRaceRequestSafetyProjectsToExistingStatusAndButtons(t *testing.T) {
	for _, accountPause := range []bool{false, true} {
		view := &pb.FmlRaceView{AutoDeleteStatus: "等待删除", Tasks: []*pb.FmlRaceTask{{DeleteAllowed: true}}}
		id := "union.race.delete.interval"
		if accountPause {
			id = "account.request"
		}
		diag := runner.Diagnostics{OperationCooldowns: []runner.OperationCooldownSnapshot{{OperationID: id, Reason: "账号保护", Until: time.Now().Add(time.Minute)}}}
		applyRaceRequestSafety(view, &pb.UnionRacePolicy{DeleteLowScoreTask: true}, diag)
		if view.AutoDeleteStatus == "等待删除" || view.Tasks[0].DeleteAllowed || view.Tasks[0].DeleteBlockedReason == "" {
			t.Fatalf("safety not visible: %+v", view)
		}
		if (view.Tasks[0].TakeSkipReason != "") != accountPause {
			t.Fatal("delete-only spacing must not block taking tasks")
		}
		ops := plannedOperationsProto([]automation.PlannedOp{{Kind: "fmlRace.delTask", Executable: true}, {Kind: "farm.harvest", Executable: true}}, diag)
		if ops[0].CooldownUntilMs == 0 {
			t.Fatal("delete queue lost cooldown")
		}
		if accountPause {
			if ops[1].Executable || ops[1].Status != pb.PlanStatus_PLAN_STATUS_BLOCKED || len(ops[1].BlockedReasons) == 0 {
				t.Fatal("account pause left farm ready")
			}
		} else if !ops[1].Executable || ops[1].CooldownUntilMs != 0 {
			t.Fatal("delete spacing affected farm")
		}
	}
}
