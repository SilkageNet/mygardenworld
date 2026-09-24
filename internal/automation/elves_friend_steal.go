package automation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	elvesFriendStealPriority        = int32(5540)
	friendStealElvesPlantingRefresh = 10 * time.Second
	friendStealElvesIdleEnter       = 5 * time.Minute
)

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
	return now.UnixMilli()-view.VisitObservedAtMs < FriendStealElvesReenterAfter(view.VisitLands).Milliseconds()
}

func friendElvesProfileUIDs(view state.FriendTouchView) []int64 {
	out := make([]int64, 0, len(view.FriendUIDs))
	for _, uid := range view.FriendUIDs {
		profile, ok := view.Profiles[uid]
		if uid > 0 && (!ok || profile.ObservedAtMs <= 0 || strings.TrimSpace(profile.Name) == "") {
			out = append(out, uid)
		}
	}
	return out
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
	goal := Goal{ID: "farm.elves_friend_steal", Category: CategoryPlant, Domain: "farm.elves_steal", Label: "摸取花灵", Priority: 56}
	view := s.FriendTouch(now)
	// Sync friends even before any UID is selected so the policy UI picker can populate.
	if !view.FriendsObserved {
		planned := friendTouchSyncOp(clientproto.RPCFrdEnter.String(), goal, "friend", "好友列表未同步，先拉取好友关系", nil, elvesFriendStealPriority+6)
		planned.FeatureID = "plant.friend_steal_elves"
		return planned, true
	}
	// Keep filling picker labels even after some UIDs are already selected.
	if profileUIDs := friendElvesProfileUIDs(view); len(profileUIDs) > 0 {
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
		if s.FriendElvesSkipEnter(uid, now) {
			continue
		}
		if !friendStealElvesVisitFresh(view, uid, now) {
			planned := friendTouchGardenOp(goal, view, uid, "进入指定好友花园摸取花灵")
			planned.FeatureID = "plant.friend_steal_elves"
			return planned, true
		}
		landID, elvesID, ok := state.PickFriendStealElvesLandFor(view.VisitLands, now, s.RoleID(), func(landID int32, land state.LandView) bool {
			return s.FriendStealElvesLandSkipped(uid, landID, land.PlantTimeMs)
		})
		if !ok {
			// Waiting for spawn → 10s; elves already visible / idle → 5m.
			s.MarkFriendElvesSkipEnter(uid, now.Add(FriendStealElvesReenterAfter(view.VisitLands)))
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
