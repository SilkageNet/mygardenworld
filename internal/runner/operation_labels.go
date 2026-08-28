package runner

import (
	"fmt"
	"strings"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func operationEventLabel(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	switch {
	case op.Kind == clientproto.RPCOrderFlowerFinishOrder.String():
		return "普通居民订单"
	case op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String():
		return "绸缎订单"
	case op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String():
		return "建材订单"
	case op.Kind == clientproto.RPCOrderFlowerRecvOrderRwd.String():
		return "居民订单领奖"
	case op.Kind == clientproto.RPCFmlFlowerShareTake.String() || op.Domain == "union.flower.take":
		return "公会摸花"
	case op.Kind == clientproto.RPCFlowerRackSell.String():
		return "花艺上架"
	case op.Kind == clientproto.RPCFlowerRackRecvSellMoney.String():
		return "花艺售出"
	case op.Kind == clientproto.RPCWaterwheelRecv.String() || op.Domain == "basic.waterwheel":
		return "水车水滴"
	case op.Kind == clientproto.RPCFreeWaterRecv.String() || op.Domain == "basic.free_water":
		return "限时水滴"
	case op.Kind == clientproto.RPCPearlPlaceHire.String():
		return "雇佣劳工"
	case op.Kind == clientproto.RPCOpptGetDetailOppts.String():
		return "同步候选人"
	case op.Kind == clientproto.RPCPearlGetRecommendList.String():
		return "同步推荐"
	case op.Kind == clientproto.RPCPearlGetHireStateByUids.String():
		return "同步雇佣状态"
	case op.Kind == clientproto.RPCFrdEnter.String() && op.Domain == "basic.pearl.hire":
		return "同步好友候选人"
	case op.Domain == "basic.pearl.hire":
		return "雇佣劳工"
	case op.Kind == clientproto.RPCPearlPlaceRecvOneKey.String() || op.Domain == "basic.pearl.place":
		return "珍珠领取"
	case op.Kind == clientproto.RPCActCyclicStoryEnter.String(),
		op.Kind == clientproto.RPCActCyclicStoryRecvOrderRwd.String(),
		op.Kind == clientproto.RPCActCyclicStoryRecv.String(),
		op.Domain == "activity.actCyclicStory":
		return "莳花纪闻"
	case op.Kind == clientproto.RPCFmlRaceGetTaskList.String():
		return "同步竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String():
		return "同步竞赛已做次数"
	case op.Kind == clientproto.RPCFmlRaceEnter.String():
		return "进入公会竞赛"
	case op.Kind == clientproto.RPCFmlRaceTakeTask.String():
		return "接取竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceFinishTask.String():
		return "完成竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceUpgradeTask.String():
		return "升级竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceDelTask.String():
		return "删除竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceGiveUpTask.String():
		return "放弃竞赛任务"
	}
	return ""
}

func opDesc(op *automation.PlannedOp) string {
	desc := opKindDesc(op.Kind)
	if op.FlowerID == 0 || isRaceOpKind(op.Kind) {
		return desc
	}
	return fmt.Sprintf("%s %s(#%d)", desc, flowerName(int(op.FlowerID)), op.FlowerID)
}

func isRaceOpKind(kind string) bool {
	switch kind {
	case clientproto.RPCFmlRaceTakeTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceUpgradeTask.String(),
		clientproto.RPCFmlRaceDelTask.String(),
		clientproto.RPCFmlRaceGiveUpTask.String():
		return true
	default:
		return false
	}
}

func operationTargetSuffix(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	switch op.Kind {
	case clientproto.RPCFmlLandHarvest.String(),
		clientproto.RPCFmlLandHarvestAll.String(),
		clientproto.RPCFmlLandPlant.String():
		if op.Reason != "" {
			return " " + op.Reason
		}
	case clientproto.RPCFmlFlowerShareTake.String():
		parts := make([]string, 0, 2)
		if op.TargetUID > 0 {
			parts = append(parts, fmt.Sprintf("成员=%d", op.TargetUID))
		}
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("槽位=%d", op.TargetID))
		}
		if len(parts) > 0 {
			return " (" + strings.Join(parts, " ") + ")"
		}
	case clientproto.RPCPearlPlaceHire.String():
		parts := make([]string, 0, 2)
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("槽位=%d", op.TargetID))
		}
		if op.TargetUID > 0 {
			parts = append(parts, fmt.Sprintf("劳工=%d", op.TargetUID))
		}
		if len(parts) > 0 {
			return " (" + strings.Join(parts, " ") + ")"
		}
	}
	if suffix := landSuffix(op.LandIDs); suffix != "" {
		return suffix
	}
	switch op.Kind {
	case clientproto.RPCFmlRaceTakeTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceUpgradeTask.String(),
		clientproto.RPCFmlRaceDelTask.String(),
		clientproto.RPCFmlRaceGiveUpTask.String():
		if desc := automation.FormatRaceTaskOpDesc(op.TaskID, op.FlowerID); desc != "" {
			return " " + desc
		}
	case clientproto.RPCActCyclicNoteEnter.String():
		if op.BatchID > 0 {
			return fmt.Sprintf(" (活动批次=%d)", op.BatchID)
		}
	case clientproto.RPCActCyclicNoteRecvTaskRwd.String():
		if op.BatchID > 0 && op.SlotID > 0 && op.TaskID > 0 {
			return fmt.Sprintf(" (活动批次=%d 槽位=%d 任务=%d)", op.BatchID, op.SlotID, op.TaskID)
		}
	case clientproto.RPCActCyclicNoteRecv.String():
		if op.BatchID > 0 && op.MilestoneIndex > 0 {
			return fmt.Sprintf(" (活动批次=%d 里程碑=%d)", op.BatchID, op.MilestoneIndex)
		}
	case clientproto.RPCActCyclicStoryEnter.String():
		if op.BatchID > 0 {
			return fmt.Sprintf(" (活动批次=%d)", op.BatchID)
		}
	case clientproto.RPCActCyclicStoryRecvOrderRwd.String():
		if op.BatchID > 0 && op.TaskID > 0 {
			return fmt.Sprintf(" (活动批次=%d 订单槽=%d 订单=%d)", op.BatchID, op.SlotID, op.TaskID)
		}
	case clientproto.RPCActCyclicStoryRecv.String():
		if op.BatchID > 0 && op.MilestoneIndex > 0 {
			return fmt.Sprintf(" (活动批次=%d 里程碑=%d)", op.BatchID, op.MilestoneIndex)
		}
	case clientproto.RPCStoryMainUnlock.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (剧情小节=%d)", op.TargetID)
		}
	case clientproto.RPCTaskMainRecv.String(), clientproto.RPCTaskAchRecv.String(), clientproto.RPCTaskDlyRecv.String(), clientproto.RPCTaskWeekRecv.String(), clientproto.RPCRoadGrowRecv.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (任务=%d)", op.TargetID)
		}
	case clientproto.RPCRandomEventDoAffair.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (事件=%d)", op.TargetID)
		}
	case clientproto.RPCZooAddFoodstuff.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d 食物=%d×%d)", op.TargetID, op.ItemID, op.Count)
		}
	case clientproto.RPCZooRefreshPetStatus.String(), clientproto.RPCZooStrokePet.String(), clientproto.RPCZooFeedPets.String(), clientproto.RPCZooFindPet.String(), clientproto.RPCZooReadLog.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d)", op.TargetID)
		}
	case clientproto.RPCZooHandleEvent.String():
		if op.TargetID > 0 && op.ItemID > 0 {
			return fmt.Sprintf(" (宠物=%d 日志=%d)", op.TargetID, op.ItemID)
		}
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d)", op.TargetID)
		}
	case clientproto.RPCZooRecvSouvenirRwd.String():
		if len(op.SlotIDs) > 0 {
			return fmt.Sprintf(" (奖励档位=%v)", op.SlotIDs)
		}
	case clientproto.RPCZooReadSouvenir.String():
		if len(op.SlotIDs) > 0 {
			return fmt.Sprintf(" (纪念品=%v)", op.SlotIDs)
		}
	case clientproto.RPCFlowerArtMakeFlowerArt.String():
		if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
			return " " + desc
		}
	case clientproto.RPCFlowerRackSell.String():
		parts := make([]string, 0, 2)
		if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
			parts = append(parts, desc)
		} else if op.ItemID > 0 {
			parts = append(parts, fmt.Sprintf("花艺#%d×%d", op.ItemID, op.Count))
		}
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("花架=%d", op.TargetID))
		}
		if len(parts) > 0 {
			return " " + strings.Join(parts, " ")
		}
	case clientproto.RPCFlowerRackRecvSellMoney.String():
		parts := make([]string, 0, 2)
		if op.ItemID > 0 {
			if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
				parts = append(parts, desc)
			} else {
				parts = append(parts, fmt.Sprintf("花艺#%d×%d", op.ItemID, op.Count))
			}
		}
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("花架=%d", op.TargetID))
		}
		if len(parts) > 0 {
			return " " + strings.Join(parts, " ")
		}
	case clientproto.RPCCultivateUpgrade.String():
		if op.Count > 0 {
			return fmt.Sprintf(" lv%d-lv%d", op.Count, op.Count+1)
		}
	case clientproto.RPCBenefitBoxDraw.String():
		if op.Count > 0 {
			return fmt.Sprintf(" ×%d", op.Count)
		}
	}
	return ""
}

func landSuffix(landIDs []int32) string {
	if len(landIDs) == 0 {
		return ""
	}
	return fmt.Sprintf(" (田地=%v)", landIDs)
}

func (r *Runner) orderCustomerSuffix(op *automation.PlannedOp) string {
	switch op.Kind {
	case clientproto.RPCOrderCustomerFinishOrder.String(), clientproto.RPCOrderCustomerRejectOrder.String():
	default:
		return ""
	}
	if op.TargetID == 0 {
		return ""
	}
	orders := r.state.CustomerOrderDetails()
	order, ok := orders[op.TargetID]
	if !ok || order == nil {
		return fmt.Sprintf(" (NPC=%d)", op.TargetID)
	}
	summary := automation.FormatCustomerOrderRequires(r.state, order)
	if summary == "" {
		return fmt.Sprintf(" (NPC=%d)", op.TargetID)
	}
	return fmt.Sprintf(" (NPC=%d %s)", op.TargetID, summary)
}

func (r *Runner) opSuffix(op *automation.PlannedOp) string {
	if suffix := operationTargetSuffix(op); suffix != "" {
		return suffix
	}
	return r.orderCustomerSuffix(op)
}
