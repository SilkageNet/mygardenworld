package automation

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	elvesPlantPriority        = int32(9800)
	elvesPlantHarvestPriority = int32(10100)
	elvesNightHarvestPriority = int32(10200)
	elvesNightHarvestHour     = 22
	elvesNightHarvestGoal     = "elves_night_harvest"
	elvesFriendStealPriority  = int32(5540)
	// While secondary is planted but elves have not spawned yet, re-enter often
	// enough to catch the spawn. Once elves are visible (or the garden is idle),
	// stay on a slow poll — no need to hammer homes that already show elvesId.
	friendStealElvesPlantingRefresh = 10 * time.Second
	friendStealElvesIdleEnter       = 5 * time.Minute
)

// FriendStealElvesReenterAfter is how long to wait before re-entering a friend
// garden when no stealable elves were found on the current visit.
func FriendStealElvesReenterAfter(lands map[int32]state.LandView) time.Duration {
	if state.FriendLandsPlantingElves(lands) {
		return friendStealElvesPlantingRefresh
	}
	return friendStealElvesIdleEnter
}

func friendStealElvesVisitFresh(view state.FriendTouchView, uid int64, now time.Time) bool {
	if view.VisitUID != uid || view.VisitObservedAtMs <= 0 {
		return false
	}
	ageMs := now.UnixMilli() - view.VisitObservedAtMs
	ttl := FriendStealElvesReenterAfter(view.VisitLands)
	return ageMs < ttl.Milliseconds()
}

func elvesPlantPolicy(plant *pb.PlantPolicy) *pb.ElvesPlantPolicy {
	if plant == nil {
		return nil
	}
	return plant.GetElvesPlant()
}

func elvesPlantActive(p *pb.ElvesPlantPolicy) bool {
	return p != nil && p.GetEnabled() && p.GetMainFlowerId() > 0 && p.GetSecondaryFlowerId() > 0
}

func elvesPlantMainLandCount(p *pb.ElvesPlantPolicy, totalLands int) int {
	if p == nil {
		return 0
	}
	n := int(p.GetMainLandCount())
	if n <= 0 {
		n = 4
	}
	if totalLands > 0 && n >= totalLands {
		n = totalLands - 1
		if n < 0 {
			n = 0
		}
	}
	return n
}

// elvesPlantDelayedHarvest reports whether secondary harvest is owned by the
// elves-plant module (independent of auto_harvest).
func elvesPlantDelayedHarvest(p *pb.ElvesPlantPolicy) bool {
	return elvesPlantActive(p) && p.GetHarvestDelaySeconds() > 0
}

// elvesNightHarvestEnabled is the 22:00 own-land elf harvest switch. It does
// not require ElvesPlantPolicy.enabled.
func elvesNightHarvestEnabled(p *pb.ElvesPlantPolicy) bool {
	return p != nil && p.GetNightHarvestEnabled()
}

// elvesNightHarvestOpen is 22:00–24:00 Asia/Shanghai. A late tick still
// collects; after midnight the window closes until the next evening.
func elvesNightHarvestOpen(now time.Time) bool {
	return now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Hour() >= elvesNightHarvestHour
}

// elvesNightHarvestLandIDs lists own lands that currently have a flower elf
// and are harvestable immediately (configured harvest delay is ignored).
func elvesNightHarvestLandIDs(s *state.State, p *pb.ElvesPlantPolicy, now time.Time) []int32 {
	if s == nil || !elvesNightHarvestEnabled(p) || !elvesNightHarvestOpen(now) {
		return nil
	}
	lands := s.Lands()
	ids := make([]int32, 0)
	for id, land := range lands {
		if land.ElvesID == 0 {
			continue
		}
		kind, _ := Recommend(land, now, 0)
		if kind != KindHarvest {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func filterOutLandIDs(landIDs, drop []int32) []int32 {
	if len(drop) == 0 || len(landIDs) == 0 {
		return landIDs
	}
	skip := make(map[int32]struct{}, len(drop))
	for _, id := range drop {
		skip[id] = struct{}{}
	}
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if _, ok := skip[id]; ok {
			continue
		}
		out = append(out, id)
	}
	return out
}

// elvesSecondaryFirstBloom is the post-water initial mature round. That bloom
// does not spawn flower elves; elves appear on the second (regrow) maturity.
func elvesSecondaryFirstBloom(land state.LandView) bool {
	return land.State == 3 && land.HarvestCnt == 0 && land.ElvesID == 0
}

// elvesPlantHarvestDelayForLand picks the harvest delay for a secondary land.
// First watering-round bloom: immediate. Later maturity (elves round): configured delay.
func elvesPlantHarvestDelayForLand(p *pb.ElvesPlantPolicy, land state.LandView) time.Duration {
	if !elvesPlantDelayedHarvest(p) {
		return 0
	}
	if elvesSecondaryFirstBloom(land) {
		return 0
	}
	return time.Duration(p.GetHarvestDelaySeconds()) * time.Second
}

func filterOutFlowerLandIDs(s *state.State, landIDs []int32, flowerID int32) []int32 {
	if flowerID <= 0 || len(landIDs) == 0 {
		return landIDs
	}
	lands := s.Lands()
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if int32(lands[id].FlowerID) == flowerID {
			continue
		}
		out = append(out, id)
	}
	return out
}

func filterOnlyFlowerLandIDs(s *state.State, landIDs []int32, flowerID int32) []int32 {
	if flowerID <= 0 || len(landIDs) == 0 {
		return nil
	}
	lands := s.Lands()
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if int32(lands[id].FlowerID) == flowerID {
			out = append(out, id)
		}
	}
	return out
}

// elvesPlantPlantOps fills empty lands: keep main_land_count main flowers, rest secondary.
func elvesPlantPlantOps(s *state.State, p *pb.ElvesPlantPolicy, empty []int32) []PlannedOp {
	if !elvesPlantActive(p) || len(empty) == 0 {
		return nil
	}
	lands := s.Lands()
	mainID := p.GetMainFlowerId()
	secID := p.GetSecondaryFlowerId()
	mainCount := 0
	for _, land := range lands {
		if int32(land.FlowerID) == mainID {
			mainCount++
		}
	}
	wantMain := elvesPlantMainLandCount(p, len(lands))
	needMain := wantMain - mainCount
	if needMain < 0 {
		needMain = 0
	}
	sort.Slice(empty, func(i, j int) bool { return empty[i] < empty[j] })
	var ops []PlannedOp
	cursor := 0
	if needMain > 0 && cursor < len(empty) {
		n := needMain
		if n > len(empty)-cursor {
			n = len(empty) - cursor
		}
		picks := append([]int32(nil), empty[cursor:cursor+n]...)
		cursor += n
		ops = append(ops, elvesPlantLandOp(picks, mainID, "种植花灵主花"))
	}
	if cursor < len(empty) {
		picks := append([]int32(nil), empty[cursor:]...)
		ops = append(ops, elvesPlantLandOp(picks, secID, "种植花灵副花"))
	}
	return ops
}

func elvesPlantLandOp(landIDs []int32, flowerID int32, reason string) PlannedOp {
	kind := clientproto.RPCUsrLandPlant.String()
	if len(landIDs) > 1 {
		kind = clientproto.RPCUsrLandPlantBatch.String()
	}
	return landOp(kind, "farm.plant", "plant", reason, elvesPlantPriority, landIDs, flowerID, "elves_plant", "elves_plant")
}

func elvesPlantSpeedUpOps(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	plant := policy.GetPlant()
	p := elvesPlantPolicy(plant)
	if !elvesPlantActive(p) || !p.GetUseSpeedUpTicket() {
		return nil
	}
	if elvesPlantBlockedByPendingAid(s, now) {
		return nil
	}
	capN := state.ResolveElvesSpawnCap(p.GetElvesSpawnCap())
	if s.ElvesProducedCount() >= capN {
		return nil
	}
	planting := plant.GetPlanting()
	batchMax := planting.GetSpeedUpTicketMax()
	if batchMax > 0 {
		remaining := batchMax - s.SpeedUpTicketsUsedToday(now)
		if remaining <= 0 {
			return nil
		}
		batchMax = remaining
	}
	secID := p.GetSecondaryFlowerId()
	lands, count := speedUpCandidates(s, now, secID, 0, batchMax)
	if count <= 0 {
		return nil
	}
	goal := Goal{ID: "farm.elves_plant", Category: CategoryPlant, Domain: "farm.speed_up", Label: "种植花灵加速", Priority: 55}
	speed := op(clientproto.RPCUsrLandSpeedUpBatch.String(), goal, "speed_up", "种植花灵副花加速", 7450, 0, 0, count)
	speed.LandIDs = lands
	speed.ItemCost = map[int32]int32{1001: count}
	speed.FeatureID = "plant.elves_plant_speed_up"
	return []PlannedOp{speed}
}

func friendStealElvesOperations(s *state.State, plant *pb.PlantPolicy, now time.Time) []PlannedOp {
	if planned, ok := PlanOneFriendStealElves(s, plant, now); ok {
		return []PlannedOp{planned}
	}
	return nil
}

// PlanOneFriendStealElves advances designated-friend elf stealing by at most one op.
// Client (v184): stealElves=!!elvesId; icon requires getStealCntLeftNumByFrdUid>0 and
// empty elvesStealUids; daily cap is c_flowerElves.$sneakMax via IFrdSteal.stealElvesCnt.
func PlanOneFriendStealElves(s *state.State, plant *pb.PlantPolicy, now time.Time) (PlannedOp, bool) {
	p := elvesPlantPolicy(plant)
	if s == nil || p == nil || !p.GetStealFriendElvesEnabled() {
		return PlannedOp{}, false
	}
	goal := Goal{ID: "farm.elves_friend_steal", Category: CategoryElves, Domain: "farm.elves_steal", Label: "摸取花灵", Priority: 56}
	view := s.FriendTouch(now)
	// Sync friends even before any UID is selected so the policy UI picker can populate.
	if !view.FriendsObserved {
		planned := friendTouchSyncOp(clientproto.RPCFrdEnter.String(), goal, "friend", "好友列表未同步，先拉取好友关系", nil, elvesFriendStealPriority+6)
		planned.FeatureID = "plant.friend_steal_elves"
		return planned, true
	}
	// Keep filling picker labels even after some UIDs are already selected.
	if profileUIDs := missingFriendProfileUIDs(view); len(profileUIDs) > 0 {
		planned := friendTouchSyncOp(clientproto.RPCOpptGetDetailOppts.String(), goal, "profile", "同步好友名称供摸花灵选择", firstUIDs(profileUIDs), elvesFriendStealPriority+5)
		planned.FeatureID = "plant.friend_steal_elves"
		return planned, true
	}
	if len(p.GetFriendUids()) == 0 {
		return PlannedOp{}, false
	}
	cfg, ok := state.FriendTouchConfigFromCatalog()
	if !ok {
		return PlannedOp{}, false
	}
	elvesCfg, ok := state.FlowerElvesGlobalsFromCatalog()
	if !ok {
		return PlannedOp{}, false
	}
	if s.StealElvesCntAt(now) >= elvesCfg.SneakMax {
		return PlannedOp{}, false
	}
	targets := make([]int64, 0, len(p.GetFriendUids()))
	seen := map[int64]struct{}{}
	friendSet := map[int64]struct{}{}
	for _, uid := range view.FriendUIDs {
		friendSet[uid] = struct{}{}
	}
	for _, uid := range p.GetFriendUids() {
		if uid <= 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		if _, ok := friendSet[uid]; !ok {
			continue
		}
		targets = append(targets, uid)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
	if len(targets) == 0 {
		return PlannedOp{}, false
	}
	staleOther := make([]int64, 0, len(targets))
	for _, uid := range targets {
		info := view.OtherInfo[uid]
		if !friendTouchInfoFresh(info.ObservedAt, now) {
			staleOther = append(staleOther, uid)
		}
	}
	if len(staleOther) > 0 {
		planned := friendTouchSyncOp(clientproto.RPCFrdExtGetFrdOtherInfoByUids.String(), goal, "availability", "同步指定好友可摸状态", firstUIDs(staleOther), elvesFriendStealPriority+4)
		planned.FeatureID = "plant.friend_steal_elves"
		return planned, true
	}
	for _, uid := range targets {
		// Do NOT gate on OtherInfo.IsSteal: that flag is ordinary flower-steal
		// availability. Elf steals only need per-friend stealLeft + land.elvesId
		// (client: getStealCntLeftNumByFrdUid>0 and empty elvesStealUids).
		if !friendTouchStealMapFresh(view, now) {
			// frdHome.getFrdHomeInfo syncs lands only; stealMap/rTime come from
			// login/lazySync NS 111.0. Re-entering cannot populate today's quota.
			planned := blockedFriendTouch("frdSteal 今日已摸次数未同步（缺少 111.0.rTime），拒绝假定为 0")
			planned.FeatureID = "plant.friend_steal_elves"
			planned.GoalID = goal.ID
			planned.Label = goal.Label
			planned.Action = "steal_elves"
			return planned, true
		}
		bought := int32(0)
		if friendTouchBuyMapFresh(view, now) {
			bought = view.StealCntBuyMap[uid]
		}
		// Client hides elf-steal icon when getStealCntLeftNumByFrdUid<=0.
		if left := cfg.StealMax + bought - view.StealMap[uid]; left <= 0 {
			continue
		}
		if s.FriendTouchSkipEnter(uid, now) {
			continue
		}
		if !friendStealElvesVisitFresh(view, uid, now) {
			planned := friendTouchEnterOp(goal, view, uid, "进入指定好友花园摸取花灵")
			planned.FeatureID = "plant.friend_steal_elves"
			return planned, true
		}
		landID, elvesID, ok := state.PickFriendStealElvesLandFor(view.VisitLands, now, s.RoleID(), func(landID int32, land state.LandView) bool {
			return s.FriendStealElvesLandSkipped(uid, landID, land.PlantTimeMs)
		})
		if !ok {
			// Waiting for spawn → 10s; elves already visible / idle → 5m.
			s.MarkFriendTouchSkipEnter(uid, now.Add(FriendStealElvesReenterAfter(view.VisitLands)))
			continue
		}
		label := friendTouchLabel(view, uid)
		elvesLabel := state.ItemLabel(elvesID)
		reason := fmt.Sprintf("摸取好友 %s 花灵 land=%d", label, landID)
		if elvesLabel != "" {
			reason = fmt.Sprintf("摸取好友 %s 的花灵 %s（田地 #%d）", label, elvesLabel, landID)
		}
		planned := friendTouchBaseOp(clientproto.RPCFrdStealSteal.String(), goal, "steal_elves",
			reason, elvesFriendStealPriority)
		planned.OperationID = clientproto.RPCFrdStealSteal.String() + ":elves:" + strconv.FormatInt(uid, 10) + ":" + strconv.FormatInt(int64(landID), 10)
		planned.TargetUID = uid
		planned.TargetID = landID
		planned.ItemID = elvesID
		planned.Count = 1
		planned.FeatureID = "plant.friend_steal_elves"
		return planned, true
	}
	return PlannedOp{}, false
}

// ValidateFriendStealElvesMutation re-plans designated-friend elf steals immediately
// before RPC send so quota, visit lands, and sneakMax cannot go stale.
func ValidateFriendStealElvesMutation(s *state.State, plant *pb.PlantPolicy, queued *PlannedOp, now time.Time) error {
	if queued == nil {
		return fmt.Errorf("花灵摸取操作为空")
	}
	if queued.Kind != clientproto.RPCFrdStealSteal.String() || queued.Action != "steal_elves" {
		return nil
	}
	current, ok := PlanOneFriendStealElves(s, plant, now)
	if !ok || !current.Executable || current.Status == PlanStatusBlocked || current.Status == PlanStatusAdapterMissing {
		return fmt.Errorf("花灵摸取前置状态已变化")
	}
	if current.Kind != queued.Kind || current.TargetUID != queued.TargetUID || current.TargetID != queued.TargetID || current.Count != queued.Count || current.Action != queued.Action || current.ItemID != queued.ItemID {
		return fmt.Errorf("花灵摸取目标已变化：计划=%s/%d/%d/%d，当前=%s/%d/%d/%d", queued.Kind, queued.TargetUID, queued.TargetID, queued.ItemID, current.Kind, current.TargetUID, current.TargetID, current.ItemID)
	}
	return nil
}
