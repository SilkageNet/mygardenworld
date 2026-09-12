package automation

import (
	"fmt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func passClaimOperations(s *state.State, task *pb.BasicTaskPolicy) []PlannedOp {
	if task == nil {
		return nil
	}
	var ops []PlannedOp
	ops = append(ops, flowerPassClaimOperations(s, task)...)
	ops = append(ops, flowerElvesPassClaimOperations(s, task)...)
	return ops
}

func flowerPassClaimOperations(s *state.State, task *pb.BasicTaskPolicy) []PlannedOp {
	taskEnabled := task.GetFlowerPassTaskRewardEnabled()
	levelEnabled := task.GetFlowerPassRewardEnabled()
	if !taskEnabled && !levelEnabled {
		return nil
	}
	goal := Goal{ID: "basic.flower_pass", Category: CategoryBasic, Domain: "basic.flower_pass", Label: "花之密令", Priority: 61}
	if !s.FlowerPassObserved() {
		planned := op(clientproto.RPCFlowerPassEnter.String(), goal, "sync", "花之密令未同步，先进入", 6110, 0, 0, 0)
		planned.FeatureID = "basic.flower_pass"
		return []PlannedOp{planned}
	}
	if taskEnabled {
		if bid, ids := s.ReadyFlowerPassTaskIDs(); bid > 0 && len(ids) > 0 {
			planned := op(clientproto.RPCFlowerPassTaskDone.String(), goal, "claim", "花之密令任务奖励可领取", 6105, bid, ids[0], 0)
			planned.FeatureID = "basic.flower_pass"
			return []PlannedOp{planned}
		}
	}
	if levelEnabled {
		if !s.FlowerPassEnterSynced() || !s.FlowerPassRwdMapObserved() {
			planned := op(clientproto.RPCFlowerPassEnter.String(), goal, "sync", "花之密令奖励状态未同步，先进入", 6102, 0, 0, 0)
			planned.FeatureID = "basic.flower_pass"
			return []PlannedOp{planned}
		}
		if bid, levels := s.ReadyFlowerPassFreeLevels(); bid > 0 && len(levels) > 0 {
			planned := op(clientproto.RPCFlowerPassRecvOneKey.String(), goal, "claim",
				fmt.Sprintf("花之密令免费等级可一键领取 (%d 档)", len(levels)), 6100, bid, 0, 0)
			planned.FeatureID = "basic.flower_pass"
			return []PlannedOp{planned}
		}
	}
	return nil
}

func flowerElvesPassClaimOperations(s *state.State, task *pb.BasicTaskPolicy) []PlannedOp {
	taskEnabled := task.GetElvesPassTaskRewardEnabled()
	levelEnabled := task.GetElvesPassRewardEnabled()
	if !taskEnabled && !levelEnabled {
		return nil
	}
	goal := Goal{ID: "basic.elves_pass", Category: CategoryBasic, Domain: "basic.elves_pass", Label: "花灵密令", Priority: 61}
	if !s.FlowerElvesPassObserved() {
		planned := op(clientproto.RPCFlowerElvesPassEnter.String(), goal, "sync", "花灵密令未同步，先进入", 6095, 0, 0, 0)
		planned.FeatureID = "basic.elves_pass"
		return []PlannedOp{planned}
	}
	if taskEnabled {
		if bid, ids := s.ReadyFlowerElvesPassTaskIDs(); bid > 0 && len(ids) > 0 {
			planned := op(clientproto.RPCFlowerElvesPassTaskDone.String(), goal, "claim", "花灵密令任务奖励可领取", 6090, bid, ids[0], 0)
			planned.FeatureID = "basic.elves_pass"
			return []PlannedOp{planned}
		}
	}
	if levelEnabled {
		if !s.FlowerElvesPassEnterSynced() || !s.FlowerElvesPassRwdMapObserved() {
			planned := op(clientproto.RPCFlowerElvesPassEnter.String(), goal, "sync", "花灵密令奖励状态未同步，先进入", 6087, 0, 0, 0)
			planned.FeatureID = "basic.elves_pass"
			return []PlannedOp{planned}
		}
		if bid, levels := s.ReadyFlowerElvesPassFreeLevels(); bid > 0 && len(levels) > 0 {
			planned := op(clientproto.RPCFlowerElvesPassRecvOneKey.String(), goal, "claim",
				fmt.Sprintf("花灵密令免费等级可一键领取 (%d 档)", len(levels)), 6085, bid, 0, 0)
			planned.FeatureID = "basic.elves_pass"
			return []PlannedOp{planned}
		}
	}
	return nil
}
