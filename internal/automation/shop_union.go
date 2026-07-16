package automation

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// raceTakeLeadWindow is how early automation may emit takeTask for a CD pool
// row that already meets filter rules (one planner tick).
const raceTakeLeadWindow = 4 * time.Second

// raceTaskPoolRefreshInterval is how often automation re-fetches the task pool
// when idle of giveUp/finish/take.
const raceTaskPoolRefreshInterval = 10 * time.Minute

func shopOperations(s *state.State, policy *pb.Policy) []PlannedOp {
	shop := policy.GetBasic().GetShop()
	var ops []PlannedOp
	ops = append(ops, giftbagOperations(s, shop)...)
	ops = append(ops, cultivateShopOperations(s, shop)...)
	vipShop := shop.GetVipShop()
	if vipShop.GetAutoBuy() {
		blocked := markerOp(CategoryBasic, "basic.shop.vip", "buy", "VIP 商店购买需要成本和状态确认", 115)
		blocked.Label = "VIP 商店"
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"VIP 商店商品状态、花坊币/元宝成本尚未完成协议确认"}
		ops = append(ops, blocked)
	}
	return ops
}

func giftbagOperations(s *state.State, shop *pb.ShopPolicy) []PlannedOp {
	if !shop.GetVideoFreeGiftEnabled() {
		return nil
	}
	goal := Goal{ID: "basic.shop.giftbag", Category: CategoryBasic, Domain: "basic.shop.giftbag", Label: "视频礼包", Priority: 54}
	if !s.ShopGiftbagObserved() {
		return []PlannedOp{domainOp(clientproto.RPCShopGiftbagEnter.String(), goal, "basic.shop.giftbag", "sync", "礼包商店状态未同步，先进入商店获取购买记录", 5480, 0, 0, 0)}
	}
	for _, offer := range s.ShopGiftbagOffers() {
		if !freeVideoGiftbag(offer) || offer.Remaining <= 0 {
			continue
		}
		buy := domainOp(clientproto.RPCShopGiftbagBuy.String(), goal, "basic.shop.video_gift", "claim", "视频免费礼包可领取", 5470, offer.ShopID, 0, 1)
		return []PlannedOp{buy}
	}
	return nil
}

func freeVideoGiftbag(offer state.ShopGiftbagOfferView) bool {
	return offer.Type == 1 && offer.ShareID > 0 && offer.RchgID == 0 &&
		offer.MoneyID == 0 && offer.Price == 0 && offer.PriceMax == 0
}

func cultivateShopOperations(s *state.State, shop *pb.ShopPolicy) []PlannedOp {
	cultivateShop := shop.GetCultivateShop()
	if !cultivateShop.GetAutoBuy() {
		return nil
	}
	goal := Goal{ID: "basic.shop.cultivate", Category: CategoryBasic, Domain: "basic.shop.cultivate", Label: "材料商店", Priority: 54}
	if !s.ShopCultivateObserved() {
		return []PlannedOp{domainOp(clientproto.RPCShopCultivateEnter.String(), goal, "basic.shop.cultivate", "sync", "材料商店状态未同步，先进入商店获取价格", 5450, 0, 0, 0)}
	}
	allowed := int32Set(cultivateShop.GetItemIds())
	inventory := s.Inventory()
	var firstBlocked *PlannedOp
	for _, offer := range s.ShopCultivateOffers() {
		if offer.ShopID <= 0 || offer.Remaining <= 0 {
			continue
		}
		if len(allowed) > 0 && !allowed[offer.ShopID] && !allowed[offer.ItemID] {
			continue
		}
		buy := domainOp(clientproto.RPCShopCultivateBuy.String(), goal, "basic.shop.cultivate", "buy", "材料商店白名单商品可购买", 5400, offer.ShopID, offer.ItemID, offer.ItemCount)
		buy.ItemCost = map[int32]int32{}
		if blocked := applyShopCultivateCostGate(&buy, offer, cultivateShop, s, inventory); len(blocked) > 0 {
			buy.Status = PlanStatusAdapterMissing
			buy.Executable = false
			buy.BlockedReasons = blocked
			if firstBlocked == nil {
				cp := buy
				firstBlocked = &cp
			}
			continue
		}
		if len(buy.ItemCost) == 0 {
			buy.ItemCost = nil
		}
		return []PlannedOp{buy}
	}
	if firstBlocked != nil {
		return []PlannedOp{*firstBlocked}
	}
	return nil
}

func applyShopCultivateCostGate(op *PlannedOp, offer state.ShopCultivateOfferView, policy *pb.ShopBuyPolicy, s *state.State, inventory map[int32]int32) []string {
	if offer.CostItemID <= 0 || offer.CostCount <= 0 {
		return []string{"材料商店价格未观测"}
	}
	switch offer.CostItemID {
	case 11:
		if policy.GetMaxSpendGold() <= 0 {
			return []string{"材料商店金币预算未设置"}
		}
		if int64(offer.CostCount) > policy.GetMaxSpendGold() {
			return []string{"材料商店金币成本超过策略上限"}
		}
		if s.Gold() < offer.CostCount {
			return []string{"金币不足"}
		}
		op.GoldCost = offer.CostCount
	case 1:
		op.DiamondCost = offer.CostCount
		if policy.GetMaxSpendDiamond() <= 0 {
			return []string{"材料商店元宝预算未设置"}
		}
		if int64(offer.CostCount) > policy.GetMaxSpendDiamond() {
			return []string{"材料商店元宝成本超过策略上限"}
		}
		return []string{"元宝成本操作尚未放开自动执行"}
	default:
		if inventory[offer.CostItemID] < offer.CostCount {
			return []string{"成本物品不足或未观测"}
		}
		op.ItemCost[offer.CostItemID] = offer.CostCount
	}
	return nil
}

func unionOperations(s *state.State, union *pb.UnionPolicy, now time.Time) []PlannedOp {
	if union == nil {
		return nil
	}
	uid := s.RoleID()
	var ops []PlannedOp
	ops = append(ops, unionBuildOperations(s, union.GetBuild())...)
	ops = append(ops, unionFlowerOperations(s, union.GetFlower())...)
	ops = append(ops, unionLandOperations(s, union.GetLand())...)
	ops = append(ops, unionRaceOperations(s, union.GetRace(), uid, now)...)
	ops = append(ops, unionForestOperations(s, union.GetForestEnabled())...)
	return ops
}

func unionFlowerOperations(s *state.State, policy *pb.UnionFlowerPolicy) []PlannedOp {
	if policy == nil || (!policy.GetShareEnabled() && !policy.GetTakeEnabled()) {
		return nil
	}
	goal := Goal{ID: "union.flower", Category: CategoryUnion, Domain: "union.flower", Label: "公会鲜花共享", Priority: 44}
	var ops []PlannedOp
	if policy.GetShareEnabled() {
		if !s.FmlFlowerShareObserved() {
			sync := domainOp(clientproto.RPCFmlFlowerShareRefresh.String(), goal, "union.flower.reward", "claim", "公会分享状态未观测，先刷新", 4470, 0, 0, 0)
			ops = append(ops, sync)
		} else if slotIDs := s.ReadyFmlFlowerShareRewardSlotIDs(); len(slotIDs) > 0 {
			claim := domainOp(clientproto.RPCFmlFlowerShareRecvRwd.String(), goal, "union.flower.reward", "claim", "公会分享槽位有可领取奖励", 4470, 0, 0, int32(len(slotIDs)))
			claim.SlotIDs = slotIDs
			ops = append(ops, claim)
		}
	}
	if policy.GetTakeEnabled() {
		if !s.OtherFmlFlowerSharesObserved() {
			sync := domainOp(clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String(), goal, "union.flower.take", "take", "公会摸花列表未观测，先同步", 4460, 0, 0, 0)
			ops = append(ops, sync)
		} else {
			for _, candidate := range s.FmlFlowerTakeCandidates() {
				if !unionFlowerTakeAllowed(candidate, policy) {
					continue
				}
				take := domainOp(clientproto.RPCFmlFlowerShareTake.String(), goal, "union.flower.take", "take", "公会成员分享鲜花可摸取", 4460, candidate.SlotID, 0, 1)
				take.TargetUID = candidate.UID
				take.FlowerID = candidate.FlowerID
				ops = append(ops, take)
				break
			}
		}
	}
	return ops
}

func unionFlowerTakeAllowed(candidate state.FmlFlowerTakeCandidate, policy *pb.UnionFlowerPolicy) bool {
	if candidate.FlowerID <= 0 {
		return false
	}
	mode := policy.GetTakeMode()
	flowers := int32Set(policy.GetTakeFlowerIds())
	qualities := int32Set(policy.GetTakeQualities())
	switch mode {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return len(flowers) == 0 || flowers[candidate.FlowerID]
	case pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		return !flowers[candidate.FlowerID]
	case pb.SelectionMode_SELECTION_MODE_QUALITY:
		return len(qualities) == 0 || qualities[flowerQuality(candidate.FlowerID)]
	case pb.SelectionMode_SELECTION_MODE_ALL:
		return true
	default:
		if len(flowers) > 0 && !flowers[candidate.FlowerID] {
			return false
		}
		if len(qualities) > 0 && !qualities[flowerQuality(candidate.FlowerID)] {
			return false
		}
		return true
	}
}

func flowerQuality(flowerID int32) int32 {
	item, ok := state.ItemInfoByID(flowerID)
	if !ok {
		return 0
	}
	return item.Color
}

func unionBuildOperations(s *state.State, policy *pb.UnionBuildPolicy) []PlannedOp {
	if policy == nil || !unionBuildPolicyEnabled(policy) {
		return nil
	}
	goal := Goal{ID: "union.build", Category: CategoryUnion, Domain: "union.build", Label: "公会建设", Priority: 45}
	if !s.FmlBuildObserved() {
		blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设状态未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到公会 namespace 25，需先进入公会或补充 fml.enter 同步链路"}
		return []PlannedOp{blocked}
	}
	build := s.FmlBuild()
	if !build.BuildCountsObserved {
		blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设次数未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到 bldCountMap，无法确认今日建设次数"}
		return []PlannedOp{blocked}
	}
	inventory := s.Inventory()
	var firstBlocked *PlannedOp
	for _, id := range unionBuildOptionIDs(policy) {
		option, ok := state.FmlBuildOptionByID(id)
		if !ok {
			blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设档位配置缺失", 4500-id, id, 0, 0)
			blocked.Status = PlanStatusAdapterMissing
			blocked.Executable = false
			blocked.BlockedReasons = []string{"缺少 c_fmlBld 静态配置"}
			if firstBlocked == nil {
				firstBlocked = &blocked
			}
			continue
		}
		if option.DailyLimit > 0 && build.BuildCounts[id] >= option.DailyLimit {
			continue
		}
		reason := strings.TrimSpace(option.Name)
		if reason == "" {
			reason = fmt.Sprintf("公会建设 #%d", id)
		}
		buildOp := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", reason+"可执行", 4500-id, id, 0, 1)
		if blocked := applyUnionBuildCostGate(&buildOp, option, policy, s, inventory); len(blocked) > 0 {
			buildOp.Status = PlanStatusAdapterMissing
			buildOp.Executable = false
			buildOp.BlockedReasons = blocked
			if firstBlocked == nil {
				cp := buildOp
				firstBlocked = &cp
			}
			continue
		}
		if len(buildOp.ItemCost) == 0 {
			buildOp.ItemCost = nil
		}
		return []PlannedOp{buildOp}
	}
	if firstBlocked != nil {
		return []PlannedOp{*firstBlocked}
	}
	return nil
}

func unionBuildPolicyEnabled(policy *pb.UnionBuildPolicy) bool {
	return policy.GetFreeEnabled() || policy.GetGoldEnabled() || policy.GetDiamondEnabled()
}

func unionBuildOptionIDs(policy *pb.UnionBuildPolicy) []int32 {
	var ids []int32
	if policy.GetFreeEnabled() {
		ids = append(ids, 1)
	}
	if policy.GetGoldEnabled() {
		ids = append(ids, 2)
	}
	if policy.GetDiamondEnabled() {
		ids = append(ids, 3)
	}
	return ids
}

func applyUnionBuildCostGate(op *PlannedOp, option state.FmlBuildOption, policy *pb.UnionBuildPolicy, s *state.State, inventory map[int32]int32) []string {
	if option.ItemID <= 0 || option.Cost <= 0 {
		return nil
	}
	switch option.ItemID {
	case 11:
		if policy.GetMaxSpendGold() <= 0 {
			return []string{"公会金币建设预算未设置"}
		}
		if int64(option.Cost) > policy.GetMaxSpendGold() {
			return []string{"公会金币建设成本超过策略上限"}
		}
		if s.Gold() < option.Cost {
			return []string{"金币不足"}
		}
		op.GoldCost = option.Cost
	case 1:
		op.DiamondCost = option.Cost
		if policy.GetMaxSpendDiamond() <= 0 {
			return []string{"公会元宝建设预算未设置"}
		}
		if int64(option.Cost) > policy.GetMaxSpendDiamond() {
			return []string{"公会元宝建设成本超过策略上限"}
		}
		if s.SpendableDiamonds() < option.Cost {
			return []string{"元宝不足"}
		}
		return []string{"元宝成本操作尚未放开自动执行"}
	default:
		if inventory[option.ItemID] < option.Cost {
			return []string{"公会建设成本物品不足或未观测"}
		}
		op.ItemCost = map[int32]int32{option.ItemID: option.Cost}
	}
	return nil
}

func unionLandOperations(s *state.State, policy *pb.UnionLandPolicy) []PlannedOp {
	if policy == nil || !policy.GetHarvestEnabled() {
		return nil
	}
	goal := Goal{ID: "union.land", Category: CategoryUnion, Domain: "union.land", Label: "公会土地", Priority: 44}
	if !s.FmlLandObserved() {
		blocked := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", "公会土地状态未观测", 4480, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到 25.102.fmlLand，需先进入公会土地或等待同步"}
		return []PlannedOp{blocked}
	}
	landIDs := s.ReadyFmlLandHarvestIDs()
	if len(landIDs) == 0 {
		return nil
	}
	harvest := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", "公会土地有成熟鲜花可收获", 4480, 0, 0, int32(len(landIDs)))
	harvest.LandIDs = landIDs
	return []PlannedOp{harvest}
}

func unionForestOperations(s *state.State, enabled bool) []PlannedOp {
	if !enabled {
		return nil
	}
	goal := Goal{ID: "union.forest", Category: CategoryUnion, Domain: "union.forest", Label: "能量森林", Priority: 43}
	if !s.FmlForestEnergyObserved() {
		sync := domainOp(clientproto.RPCFmlForestRefresh.String(), goal, "union.forest", "collect", "能量森林状态未观测，先刷新并自动收集", 4430, 1, 0, 0)
		return []PlannedOp{sync}
	}
	energy := s.FmlForestEnergy()
	types := s.ReadyFmlForestEnergyTypes()
	if len(types) == 0 {
		return nil
	}
	collect := domainOp(clientproto.RPCFmlForestRefresh.String(), goal, "union.forest", "collect", "能量森林有临时能量可收集", 4430, 1, 0, energy.PendingTempEnergyTotal)
	return []PlannedOp{collect}
}

// unionRaceOperations emits PlannedOps for the guild race task pool.
// Lifecycle:
//  1. enter + getTaskList (sync)
//  2. takeTask (接取)
//  3. raceTaskProgressDemands drives plant/harvest for 种植收获 (进行)
//  4. finishTask when FinishCnt >= TargetCnt (完成并领取积分)
//
// useSpeedupTicketInTask is honored by maintenanceOperations via
// raceSpeedupEnabled while an unfinished plant-harvest task is held.
func unionRaceOperations(s *state.State, policy *pb.UnionRacePolicy, uid int64, now time.Time) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	view := s.FmlRace()

	goal := Goal{ID: "union.race", Category: CategoryRace, Domain: "union.race", Label: "公会竞赛", Priority: 43}

	// Enter pushes CurFmlRaceBatch (111). Task pool / taken-task (114/110) require
	// a follow-up getTaskList. Neither sync step is gated by autoEnableModules.
	// Also re-fetch when a plant-harvest row is missing flower ParamID (once per
	// pool msId set — state.MissingParamRefreshFP prevents tight loops).
	if !view.Observed {
		return []PlannedOp{domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛进入同步", 4400, 0, 0, 0)}
	}
	if view.BatchActive && (!view.TasksObserved || raceTaskPoolNeedsParamRefresh(view)) {
		return []PlannedOp{domainOp(clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync", "公会竞赛拉取任务池与已接任务", 4398, 0, 0, 0)}
	}

	// autoEnableModules gates the sub-module execution (take/finish/upgrade/delete).
	// When off, the race module is active but does not auto-execute tasks.
	if !policy.GetAutoEnableModules() {
		return nil
	}
	if !view.BatchActive {
		return nil
	}
	var ops []PlannedOp

	// 1a. Abandon a taken task that cannot or should not be kept.
	// Score filter: min_task_score is a lower bound (Score <= minScore → give up).
	// Score==0 means the pool has not resolved the taken task yet — do not give up
	// for score alone.
	// Plant-harvest: give up when the target flower is unknown or not cultivated.
	if view.Taken.HasTask && view.Taken.FinishCnt < view.Taken.TargetCnt {
		reason := ""
		minScore := policy.GetMinTaskScore()
		switch {
		case minScore > 0 && view.Taken.Score > 0 && view.Taken.Score <= minScore:
			reason = "公会竞赛放弃不符合分数要求的已接任务"
		case raceTakenUncompletable(s, view.Taken):
			reason = "公会竞赛放弃无法完成的种植收获任务"
		case raceTakenPriorityZero(policy, view.Taken):
			reason = "公会竞赛放弃优先级为0的已接任务"
		}
		if reason != "" {
			op := domainOp(clientproto.RPCFmlRaceGiveUpTask.String(), goal, "union.race.giveUp", "giveUp", reason, 4395, 0, 0, 0)
			op.TaskMsID = view.Taken.TaskMsId
			op.TaskID = view.Taken.TaskType
			if op.TaskID == 0 {
				op.TaskID = view.Taken.TaskId
			}
			op.FlowerID = view.Taken.ParamID
			ops = append(ops, op)
		}
	}

	// 1b. Finish the current taken task if complete.
	if view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		op := domainOp(clientproto.RPCFmlRaceFinishTask.String(), goal, "union.race.finish", "finish", "公会竞赛任务已完成，提交领取积分", 4390, 0, 0, 0)
		op.TaskMsID = view.Taken.TaskMsId
		op.TaskID = view.Taken.TaskType
		if op.TaskID == 0 {
			op.TaskID = view.Taken.TaskId
		}
		op.FlowerID = view.Taken.ParamID
		ops = append(ops, op)
	}

	// 2. Select a task to take (only if not currently holding one).
	if !view.Taken.HasTask {
		selected := selectRaceTasks(s, view.Tasks, policy, uid, now)
		if len(selected) > 0 {
			best := selected[0]
			op := domainOp(clientproto.RPCFmlRaceTakeTask.String(), goal, "union.race.take", "take", "公会竞赛选择最优任务接取", 4380, 0, 0, 0)
			op.TaskMsID = best.MsId
			op.TaskID = best.TaskType
			if op.TaskID == 0 {
				op.TaskID = best.TaskId
			}
			op.FlowerID = best.ParamID
			ops = append(ops, op)
		}
	}

	hasPrimary := false
	for _, op := range ops {
		if isRacePrimaryMutatingOp(op) {
			hasPrimary = true
			break
		}
	}

	if !hasPrimary && raceTaskPoolTTLStale(view, now) && !raceHasNearTakeableCD(s, view.Tasks, policy, uid, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛定时刷新任务池", 4398, 0, 0, 0,
		)}
	}

	// 3. Optional: upgrade the currently held task. The observed client sends
	// an empty upgradeTask request, so this RPC cannot target an arbitrary pool
	// row. Its diamond cost must be known and pass the configured budget; the
	// global diamond gate still blocks automatic execution by default.
	if policy.GetUpgradeTask() && view.Taken.HasTask {
		if task, ok := raceTaskByMsID(view.Tasks, view.Taken.TaskMsId); ok && task.IsUpgrade == 0 {
			op := domainOp(clientproto.RPCFmlRaceUpgradeTask.String(), goal, "union.race.upgrade", "upgrade", "公会竞赛当前任务可升级", 4370, 0, 0, 0)
			op.TaskMsID = task.MsId
			op.TaskID = task.TaskType
			if op.TaskID == 0 {
				op.TaskID = task.TaskId
			}
			op.FlowerID = task.ParamID
			cost, costKnown := state.FmlRaceTaskUpgradeCost(task.TaskId, task.Score)
			switch {
			case !costKnown:
				op.Status = PlanStatusAdapterMissing
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级成本无法从客户端配置确认"}
			case policy.GetMaxSpendDiamond() <= 0:
				op.DiamondCost = cost
				op.Status = PlanStatusBlocked
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级元宝预算未设置"}
			case int64(cost) > policy.GetMaxSpendDiamond():
				op.DiamondCost = cost
				op.Status = PlanStatusBlocked
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级元宝成本超过策略上限"}
			default:
				op.DiamondCost = cost
			}
			ops = append(ops, op)
		}
	}

	// 4. Optional: delete low-score tasks.
	if policy.GetDeleteLowScoreTask() {
		maxDel := policy.GetDeleteTaskMaxScore()
		if maxDel > 0 {
			for _, task := range view.Tasks {
				if task.UID == 0 && task.Score <= maxDel {
					op := domainOp(clientproto.RPCFmlRaceDelTask.String(), goal, "union.race.delete", "delete", "公会竞赛低分任务清理", 4360, 0, 0, 0)
					op.TaskMsID = task.MsId
					op.TaskID = task.TaskType
					if op.TaskID == 0 {
						op.TaskID = task.TaskId
					}
					op.FlowerID = task.ParamID
					ops = append(ops, op)
					break // one delete per cycle
				}
			}
		}
	}

	return ops
}

func raceTaskPoolTTLStale(view state.FmlRaceView, now time.Time) bool {
	if !view.BatchActive || !view.TasksObserved {
		return false
	}
	if view.TasksSyncedAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceTaskPoolRefreshInterval))
}

func raceHasNearTakeableCD(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time) bool {
	nowMs := now.UnixMilli()
	for _, t := range tasks {
		if t.AppearTime <= 0 || t.AppearTime <= nowMs {
			continue
		}
		if t.UID != 0 {
			continue
		}
		rem := time.Duration(t.AppearTime-nowMs) * time.Millisecond
		if rem >= raceTaskPoolRefreshInterval {
			continue
		}
		if raceTakeNonCDSkipReason(s, t, policy, uid) == "" {
			return true
		}
	}
	return false
}

func isRacePrimaryMutatingOp(op PlannedOp) bool {
	switch op.Kind {
	case clientproto.RPCFmlRaceGiveUpTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceTakeTask.String():
		return true
	default:
		return false
	}
}

// RaceTakeSkipReason returns the primary reason automation will not take this
// pool task, or "" if it is takeable (including preemptive CD within raceTakeLeadWindow).
// Priority matches docs/superpowers/specs/2026-07-15-race-task-take-skip-reason-design.md
// and CD copy branching in docs/superpowers/specs/2026-07-15-race-cd-skip-copy-design.md.
func RaceTakeSkipReason(s *state.State, t state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time) string {
	if t.UID != 0 {
		return "已被接取"
	}
	leadUntil := now.Add(raceTakeLeadWindow).UnixMilli()
	if t.AppearTime > 0 && t.AppearTime > leadUntil {
		hhmm := time.UnixMilli(t.AppearTime).Local().Format("15:04")
		if raceTakeNonCDSkipReason(s, t, policy, uid) != "" {
			return hhmm + " 后刷新"
		}
		return "冷却中，" + hhmm + " 后可接"
	}
	return raceTakeNonCDSkipReason(s, t, policy, uid)
}

// raceTakeNonCDSkipReason evaluates take filters other than far-CD AppearTime.
// Empty means those filters would allow take (ready / within-lead still apply outside).
func raceTakeNonCDSkipReason(s *state.State, t state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64) string {
	if policy.GetMinTaskScore() > 0 && t.Score <= policy.GetMinTaskScore() {
		return fmt.Sprintf("分数不足（≤%d）", policy.GetMinTaskScore())
	}
	if policy.GetOnlyUpgradeTask() && t.IsUpgrade == 0 {
		return "仅接已升级任务"
	}
	if policy.GetExcludeOthersUpgradeTask() && t.UpgradeUid != 0 && t.UpgradeUid != uid {
		return "他人已升级"
	}
	taskType := t.TaskType
	if taskType == 0 {
		taskType = t.TaskId
	}
	if raceTaskTypePriority(policy, taskType) <= 0 {
		return "优先级为0"
	}
	if taskType != raceTaskTypePlantHarvest {
		return "暂不支持自动完成"
	}
	if taskType == raceTaskTypePlantHarvest {
		if t.ParamID <= 0 || !flowerCultivated(s, t.ParamID) {
			return "目标花卉未培养"
		}
	}
	return ""
}

func raceTaskByMsID(tasks []state.FmlRaceTaskView, msID int64) (state.FmlRaceTaskView, bool) {
	for _, task := range tasks {
		if task.MsId == msID {
			return task, true
		}
	}
	return state.FmlRaceTaskView{}, false
}

// raceTaskPoolNeedsParamRefresh reports whether getTaskList should run again
// because at least one plant-harvest pool task has no flower ParamID and this
// incomplete pool identity has not yet been refresh-attempted.
func raceTaskPoolNeedsParamRefresh(view state.FmlRaceView) bool {
	if !state.FmlRacePlantHarvestMissingParam(view.Tasks) {
		return false
	}
	return state.FmlRaceTaskPoolMsFingerprint(view.Tasks) != view.MissingParamRefreshFP
}

// raceTaskTypePriority returns the configured priority for a race task type.
// Missing map entries fall back to defaultUnionRacePriority (0 = do not take).
func raceTaskTypePriority(policy *pb.UnionRacePolicy, taskType int32) int32 {
	if m := policy.GetTaskTypePriority(); m != nil {
		if p, ok := m[taskType]; ok {
			return p
		}
	}
	return defaultUnionRacePriority()[taskType]
}

// selectRaceTasks filters the available task pool via RaceTakeSkipReason, then
// sorts by configured priority.
//
// min_task_score is a lower bound: tasks with Score <= minScore are skipped.
// 0 means no score filtering. Combined with only_upgrade_task, only upgraded tasks
// above the threshold are eligible.
//
// task_type_priority: 0 (or missing → default 0) means do not take that type.
// Positive values rank candidates (higher first), then Score descending.
//
// Plant-harvest (3036): skip when ParamID is missing or the flower is not yet
// cultivated (Status==2 && Lvl>0). Seed stock / empty land are not required.
//
// AppearTime gating: ready tasks (appearTime already due) are preferred. CD tasks
// within raceTakeLeadWindow may be selected preemptively when no ready candidate
// remains; farther CD tasks are skipped.
func selectRaceTasks(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time) []state.FmlRaceTaskView {
	nowMs := now.UnixMilli()

	var ready, upcoming []state.FmlRaceTaskView
	for _, t := range tasks {
		if RaceTakeSkipReason(s, t, policy, uid, now) != "" {
			continue
		}
		if t.AppearTime > 0 && t.AppearTime > nowMs {
			// Within lead (otherwise skip reason would be non-empty).
			upcoming = append(upcoming, t)
			continue
		}
		ready = append(ready, t)
	}

	sortRaceTasks := func(list []state.FmlRaceTaskView) {
		sort.SliceStable(list, func(i, j int) bool {
			pi := int(raceTaskTypePriority(policy, list[i].TaskType))
			pj := int(raceTaskTypePriority(policy, list[j].TaskType))
			if pi != pj {
				return pi > pj
			}
			return list[i].Score > list[j].Score
		})
	}
	sortRaceTasks(ready)
	sortRaceTasks(upcoming)
	if len(ready) > 0 {
		return ready
	}
	return upcoming
}

// raceTakenUncompletable reports whether a held unfinished task can never be
// progressed by automation — today only plant-harvest with a missing/unplantable
// target flower.
func raceTakenUncompletable(s *state.State, taken state.FmlRaceTakenView) bool {
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	if taskType != raceTaskTypePlantHarvest {
		return false
	}
	return taken.ParamID <= 0 || !flowerCultivated(s, taken.ParamID)
}

// raceTakenPriorityZero reports whether a held task's type is configured at
// priority 0 (do not take / should give up).
func raceTakenPriorityZero(policy *pb.UnionRacePolicy, taken state.FmlRaceTakenView) bool {
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	return raceTaskTypePriority(policy, taskType) <= 0
}
