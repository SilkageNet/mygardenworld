package automation

import (
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func guardRaceHeldOperations(s *state.State, policy *pb.Policy, ops []PlannedOp, now time.Time) {
	v := s.FmlRaceAt(now)
	if !v.Taken.HasTask {
		return
	}
	farmDriven := raceDrivesFarm(s, policy, now)
	for i := range ops {
		op := &ops[i]
		switch op.Kind {
		case clientproto.RPCFmlRaceTakeTask.String(), clientproto.RPCFmlRaceDelTask.String(),
			clientproto.RPCFmlRaceEnter.String(), clientproto.RPCFmlRaceGetTaskList.String(),
			clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String():
			continue
		}
		// Farm work can be forced by a held task even with ordinary toggles off.
		// Cancel and replan at expiry rather than reuse that old authorization.
		if op.GoalID == raceActionGoal || (farmDriven && strings.HasPrefix(op.Kind, "usrLand.")) ||
			(op.GoalID == GoalCustomerOrder && RaceHoldsUnfinishedCustomerOrder(v)) {
			op.RaceHoldTaskMsID, op.RaceBatchID = v.Taken.TaskMsId, v.BatchID
		}
	}
}

func ValidateRaceHeldOperation(s *state.State, op *PlannedOp, now time.Time) error {
	if op == nil || op.RaceHoldTaskMsID == 0 {
		return nil
	}
	v := s.FmlRaceAt(now)
	if !v.ActiveAt(now) || !v.Taken.HasTask || v.Taken.TaskMsId != op.RaceHoldTaskMsID || v.BatchID != op.RaceBatchID {
		return fmt.Errorf("竞赛任务已过期或变更，取消旧任务操作并重新规划")
	}
	return nil
}
