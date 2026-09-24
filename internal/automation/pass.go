package automation

import (
	"fmt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// Pass rewards are free claims only. The board and claimed-level map must be
// observed before a claim is planned; paid tiers are never selected.
func passClaimOperations(s *state.State, p *pb.FlowerElvesPolicy) []PlannedOp {
	if s == nil || p == nil {
		return nil
	}
	var ops []PlannedOp
	for _, board := range []struct {
		task, level                       bool
		observed, entered, rewards, found bool
		readyTask                         func() (int32, []int32)
		readyLevel                        func() (int32, []int32)
		enter, taskDone, recv             clientproto.RPCName
		feature, label                    string
		priority                          int32
	}{
		{p.GetFlowerPassTaskRewardEnabled(), p.GetFlowerPassRewardEnabled(), s.FlowerPassObserved(), s.FlowerPassEnterSynced(), s.FlowerPassRwdMapObserved(), s.FlowerPassView().Found, s.ReadyFlowerPassTaskIDs, s.ReadyFlowerPassFreeLevels, clientproto.RPCFlowerPassEnter, clientproto.RPCFlowerPassTaskDone, clientproto.RPCFlowerPassRecv, "plant.flower_pass", "花之密令", 6110},
		{p.GetPassTaskRewardEnabled(), p.GetPassRewardEnabled(), s.FlowerElvesPassObserved(), s.FlowerElvesPassEnterSynced(), s.FlowerElvesPassRwdMapObserved(), s.FlowerElvesPassView().Found, s.ReadyFlowerElvesPassTaskIDs, s.ReadyFlowerElvesPassFreeLevels, clientproto.RPCFlowerElvesPassEnter, clientproto.RPCFlowerElvesPassTaskDone, clientproto.RPCFlowerElvesPassRecv, "plant.elves_pass", "花灵密令", 6095},
	} {
		if !board.task && !board.level {
			continue
		}
		goal := Goal{ID: board.feature, Category: CategoryPlant, Domain: board.feature, Label: board.label, Priority: board.priority / 100}
		if !board.observed || (board.level && !board.entered) || (board.level && board.found && !board.rewards) {
			planned := op(board.enter.String(), goal, "sync", board.label+"奖励状态未同步", board.priority, 0, 0, 0)
			planned.FeatureID = board.feature
			ops = append(ops, planned)
			continue
		}
		if !board.found {
			continue
		}
		if board.task {
			if bid, ids := board.readyTask(); bid > 0 && len(ids) > 0 {
				planned := op(board.taskDone.String(), goal, "claim", board.label+"任务奖励可领取", board.priority-2, bid, ids[0], 0)
				planned.FeatureID = board.feature
				ops = append(ops, planned)
				continue
			}
		}
		if board.level {
			if bid, levels := board.readyLevel(); bid > 0 && len(levels) > 0 {
				planned := op(board.recv.String(), goal, "claim", fmt.Sprintf("%s免费等级 %d 可领取", board.label, levels[0]), board.priority-5, bid, levels[0], state.PassRwdTypeFree)
				planned.FeatureID = board.feature
				ops = append(ops, planned)
			}
		}
	}
	return ops
}
