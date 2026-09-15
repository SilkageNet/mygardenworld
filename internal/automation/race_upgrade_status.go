package automation

import (
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// RaceAutoUpgradeStatus explains the same task/cost gates used by execution.
// It is not permission to spend; send-time validation remains authoritative.
func RaceAutoUpgradeStatus(s *state.State, policy *pb.UnionRacePolicy, now time.Time) string {
	if !policy.GetUpgradeTask() {
		return "未开启"
	}
	if policy.GetMaxSpendDiamond() <= 0 {
		return "单次升级元宝上限为 0，禁止消费；请设置预算并保存"
	}
	if !policy.GetEnabled() {
		return "任务池同步未开启"
	}
	view := s.FmlRace()
	if !view.Taken.HasTask {
		return "等待接取任务"
	}
	if view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		return "任务已完成，不再消费元宝升级"
	}
	if !view.TasksObserved || view.TaskPoolStale || raceTaskPoolTTLStale(view, now) {
		return "等待当前任务池同步"
	}
	task, ok := raceTaskByMsID(view.Tasks, view.Taken.TaskMsId)
	if !ok {
		return "当前任务不在已同步任务池中，等待确认"
	}
	if task.IsUpgrade != 0 {
		return "当前任务已升级"
	}
	ops := raceUpgradeOperations(s, view, policy, Goal{}, now)
	if len(ops) == 0 {
		return "等待当前任务状态确认"
	}
	op := &ops[0]
	if len(op.BlockedReasons) > 0 {
		return strings.Join(op.BlockedReasons, "；")
	}
	if err := ValidateRaceUpgrade(s, policy, op, now); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("预计消耗 %d 元宝（上限 %d），等待调度并再次核验", op.DiamondCost, policy.GetMaxSpendDiamond())
}
