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
	friendTouchPriority     = int32(5530)
	friendTouchOtherInfoTTL = 30 * time.Second
	friendTouchVisitTTL     = 30 * time.Second
	friendTouchSyncBatch    = 50
)

type friendTouchTarget struct {
	UID   int64
	Count int32
}

func friendTouchOperations(s *state.State, policy *pb.FriendStealPolicy, now time.Time) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	if planned, ok := PlanOneFriendTouch(s, policy, now); ok {
		return []PlannedOp{planned}
	}
	return nil
}

// PlanOneFriendTouch advances the observed friend-garden state machine by at
// most one operation. Unknown quota, friend membership, or flower state is
// never interpreted as permission to mutate server state.
func PlanOneFriendTouch(s *state.State, policy *pb.FriendStealPolicy, now time.Time) (PlannedOp, bool) {
	if s == nil || policy == nil || !policy.GetEnabled() {
		return PlannedOp{}, false
	}
	if policy.GetStealElves() {
		return unsupportedFriendElves(), true
	}
	cfg, ok := state.FriendTouchConfigFromCatalog()
	if !ok {
		return blockedFriendTouch("好友摸花目录常量缺失，无法确认次数与友情币成本"), true
	}
	view := s.FriendTouch(now)
	goal := friendTouchGoal()
	if !view.FriendsObserved {
		return friendTouchSyncOp(clientproto.RPCFrdEnter.String(), goal, "friend", "好友列表未同步，先拉取服务端好友关系", nil, friendTouchPriority+6), true
	}

	targets := friendTouchTargets(policy, view, cfg, now)
	if len(targets) == 0 {
		return PlannedOp{}, false
	}
	profileUIDs := friendTouchProfileUIDs(view, targets)
	if len(profileUIDs) > 0 {
		return friendTouchSyncOp(clientproto.RPCOpptGetDetailOppts.String(), goal, "profile", "摸花目标好友名称未同步", firstUIDs(profileUIDs), friendTouchPriority+5), true
	}
	otherUIDs := friendTouchOtherInfoUIDs(view, targets, now)
	if len(otherUIDs) > 0 {
		return friendTouchSyncOp(clientproto.RPCFrdExtGetFrdOtherInfoByUids.String(), goal, "availability", "好友可摸状态未同步或已过期", firstUIDs(otherUIDs), friendTouchPriority+4), true
	}

	selection := friendFlowerSelection(policy)
	for _, target := range targets {
		info := view.OtherInfo[target.UID]
		if !friendTouchInfoFresh(info.ObservedAt, now) || !info.IsSteal {
			continue
		}
		if !friendTouchStealMapFresh(view, now) {
			if !friendTouchVisitFresh(view, target.UID, now) {
				return friendTouchEnterOp(goal, view, target.UID, "进入好友花园并同步今日已摸次数"), true
			}
			return blockedFriendTouch("frdSteal 今日已摸次数未随进入好友花园回包同步，拒绝假定为 0"), true
		}

		stolen := view.StealMap[target.UID]
		if target.Count-stolen <= 0 {
			continue
		}
		bought := int32(0)
		if friendTouchBuyMapFresh(view, now) {
			bought = view.StealCntBuyMap[target.UID]
		}
		left := cfg.StealMax + bought - stolen
		if left <= 0 {
			if !policy.GetAutoBuyTimes() {
				continue
			}
			if !friendTouchBuyMapFresh(view, now) {
				return blockedFriendTouch("今日额外摸花购买次数未同步，拒绝重复消耗友情币"), true
			}
			if planned, ok := planFriendTouchBuy(s, policy, cfg, goal, view, target.UID); ok {
				return planned, true
			}
			continue
		}
		if s.FriendTouchSkipEnter(target.UID, now) {
			continue
		}
		if !friendTouchVisitFresh(view, target.UID, now) {
			return friendTouchEnterOp(goal, view, target.UID, "进入好友花园，准备单次摸花"), true
		}
		landID, ok := state.PickFriendStealLandIDWithSelection(view.VisitLands, s.Inventory(), s.RoleID(), now, selection)
		if !ok {
			continue
		}
		reason := friendTouchStealReason(friendTouchMode(policy), friendTouchLabel(view, target.UID), target.Count, stolen)
		planned := friendTouchBaseOp(clientproto.RPCFrdStealSteal.String(), goal, "steal", reason, friendTouchPriority)
		planned.OperationID = clientproto.RPCFrdStealSteal.String() + ":" + strconv.FormatInt(target.UID, 10) + ":" + strconv.FormatInt(int64(landID), 10)
		planned.TargetUID = target.UID
		planned.TargetID = landID
		planned.Count = 1
		return planned, true
	}
	return PlannedOp{}, false
}

func friendTouchGoal() Goal {
	return Goal{ID: "farm.friend_steal", Category: CategoryPlant, Domain: "farm.friend_steal", Label: "好友摸花", Priority: 55}
}

func friendTouchEnterOp(goal Goal, view state.FriendTouchView, uid int64, reason string) PlannedOp {
	planned := friendTouchBaseOp(clientproto.RPCFrdStealEnterFrdSteal.String(), goal, "enter", fmt.Sprintf("%s %s", reason, friendTouchLabel(view, uid)), friendTouchPriority+3)
	planned.OperationID = clientproto.RPCFrdStealEnterFrdSteal.String() + ":" + strconv.FormatInt(uid, 10)
	planned.TargetUID = uid
	return planned
}

func friendTouchMode(policy *pb.FriendStealPolicy) pb.SelectionMode {
	if policy == nil {
		return pb.SelectionMode_SELECTION_MODE_ALL
	}
	switch policy.GetFriendMode() {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return pb.SelectionMode_SELECTION_MODE_SPECIFIC
	case pb.SelectionMode_SELECTION_MODE_ALL:
		return pb.SelectionMode_SELECTION_MODE_ALL
	default:
		if len(policy.GetFriendCounts()) > 0 {
			return pb.SelectionMode_SELECTION_MODE_SPECIFIC
		}
		return pb.SelectionMode_SELECTION_MODE_ALL
	}
}

func friendTouchTargets(policy *pb.FriendStealPolicy, view state.FriendTouchView, cfg state.FriendTouchConfig, now time.Time) []friendTouchTarget {
	friendSet := make(map[int64]struct{}, len(view.FriendUIDs))
	for _, uid := range view.FriendUIDs {
		if uid > 0 {
			friendSet[uid] = struct{}{}
		}
	}
	excluded := make(map[int64]struct{}, len(policy.GetExcludeUids()))
	for _, uid := range policy.GetExcludeUids() {
		if uid > 0 {
			excluded[uid] = struct{}{}
		}
	}
	maxBuy := policy.GetMaxBuyPerFriend()
	if maxBuy <= 0 || maxBuy > cfg.PickMax {
		maxBuy = cfg.PickMax
	}
	out := make([]friendTouchTarget, 0, len(view.FriendUIDs))
	if friendTouchMode(policy) == pb.SelectionMode_SELECTION_MODE_SPECIFIC {
		for uid, count := range policy.GetFriendCounts() {
			if _, isFriend := friendSet[uid]; !isFriend || count <= 0 {
				continue
			}
			if _, skip := excluded[uid]; skip {
				continue
			}
			if count > cfg.StealMax+maxBuy {
				count = cfg.StealMax + maxBuy
			}
			out = append(out, friendTouchTarget{UID: uid, Count: count})
		}
	} else {
		for _, uid := range view.FriendUIDs {
			if uid <= 0 {
				continue
			}
			if _, skip := excluded[uid]; skip {
				continue
			}
			count := cfg.StealMax
			if policy.GetAutoBuyTimes() {
				count += maxBuy
			} else if friendTouchBuyMapFresh(view, now) {
				count += view.StealCntBuyMap[uid]
			}
			out = append(out, friendTouchTarget{UID: uid, Count: count})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID < out[j].UID })
	return out
}

func friendFlowerSelection(policy *pb.FriendStealPolicy) state.FriendStealSelection {
	selection := state.FriendStealSelection{}
	if policy == nil {
		return selection
	}
	switch policy.GetMode() {
	case pb.SelectionMode_SELECTION_MODE_QUALITY:
		selection.Qualities = append([]int32(nil), policy.GetQualities()...)
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		selection.FlowerIDs = append([]int32(nil), policy.GetFlowerIds()...)
	case pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		selection.ExcludeFlowerIDs = append([]int32(nil), policy.GetExcludeFlowerIds()...)
	}
	return selection
}

func planFriendTouchBuy(s *state.State, policy *pb.FriendStealPolicy, cfg state.FriendTouchConfig, goal Goal, view state.FriendTouchView, uid int64) (PlannedOp, bool) {
	maxBuy := policy.GetMaxBuyPerFriend()
	if maxBuy <= 0 || maxBuy > cfg.PickMax {
		maxBuy = cfg.PickMax
	}
	bought := view.StealCntBuyMap[uid]
	if bought >= maxBuy {
		return PlannedOp{}, false
	}
	costEach := cfg.PickAddCost
	if costEach <= 0 || s.Inventory()[state.FriendCoinItemID()] < costEach {
		return PlannedOp{}, false
	}
	reason := fmt.Sprintf("为好友 %s 兑换 1 次摸花次数（友情币 %d）", friendTouchLabel(view, uid), costEach)
	planned := friendTouchBaseOp(clientproto.RPCFrdExtBuyStealCnt.String(), goal, "buy", reason, friendTouchPriority+1)
	planned.OperationID = clientproto.RPCFrdExtBuyStealCnt.String() + ":" + strconv.FormatInt(uid, 10)
	planned.TargetUID = uid
	planned.Count = 1
	planned.ItemCost = map[int32]int32{state.FriendCoinItemID(): costEach}
	planned.FeatureID = "plant.friend_steal_buy"
	return planned, true
}

func friendTouchProfileUIDs(view state.FriendTouchView, targets []friendTouchTarget) []int64 {
	out := make([]int64, 0, len(targets))
	for _, target := range targets {
		profile, exists := view.Profiles[target.UID]
		if !exists || profile.ObservedAtMs <= 0 || strings.TrimSpace(profile.Name) == "" {
			out = append(out, target.UID)
		}
	}
	return out
}

func friendTouchOtherInfoUIDs(view state.FriendTouchView, targets []friendTouchTarget, now time.Time) []int64 {
	out := make([]int64, 0, len(targets))
	for _, target := range targets {
		info, exists := view.OtherInfo[target.UID]
		if !exists || !friendTouchInfoFresh(info.ObservedAt, now) {
			out = append(out, target.UID)
		}
	}
	return out
}

func firstUIDs(uids []int64) []int64 {
	if len(uids) > friendTouchSyncBatch {
		uids = uids[:friendTouchSyncBatch]
	}
	return append([]int64(nil), uids...)
}

func friendTouchStealMapFresh(view state.FriendTouchView, now time.Time) bool {
	return view.StealObserved && view.StealRTimeMs > 0 && calendarDayID(now) == calendarDayID(time.UnixMilli(view.StealRTimeMs))
}

func friendTouchBuyMapFresh(view state.FriendTouchView, now time.Time) bool {
	return view.StealCntBuyObserved && view.StealCntBuyRTimeMs > 0 && calendarDayID(now) == calendarDayID(time.UnixMilli(view.StealCntBuyRTimeMs))
}

func friendTouchInfoFresh(observedAtMs int64, now time.Time) bool {
	return observedAtMs > 0 && now.UnixMilli()-observedAtMs <= friendTouchOtherInfoTTL.Milliseconds()
}

func friendTouchVisitFresh(view state.FriendTouchView, uid int64, now time.Time) bool {
	return view.VisitUID == uid && view.VisitObservedAtMs > 0 && now.UnixMilli()-view.VisitObservedAtMs <= friendTouchVisitTTL.Milliseconds()
}

func friendTouchLabel(view state.FriendTouchView, uid int64) string {
	if profile, ok := view.Profiles[uid]; ok && strings.TrimSpace(profile.Name) != "" {
		return profile.Name
	}
	return strconv.FormatInt(uid, 10)
}

func friendTouchStealReason(mode pb.SelectionMode, label string, targetCount, stolen int32) string {
	if mode == pb.SelectionMode_SELECTION_MODE_SPECIFIC {
		return fmt.Sprintf("向好友 %s 摸花（目标 %d 次，今日已摸 %d 次）", label, targetCount, stolen)
	}
	return fmt.Sprintf("向好友 %s 摸花（今日已摸 %d 次）", label, stolen)
}

func friendTouchSyncOp(kind string, goal Goal, source, reason string, uids []int64, priority int32) PlannedOp {
	planned := friendTouchBaseOp(kind, goal, "sync", reason, priority)
	planned.OperationID = kind + ":friend_touch:" + source
	planned.TargetUIDs = append([]int64(nil), uids...)
	return planned
}

func friendTouchBaseOp(kind string, goal Goal, action, reason string, priority int32) PlannedOp {
	return PlannedOp{
		OperationID: kind,
		GoalID:      goal.ID,
		Kind:        kind,
		Lane:        LaneSide,
		FeatureID:   "plant.friend_steal",
		Category:    goal.Category,
		Label:       goal.Label,
		Domain:      goal.Domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		Status:      PlanStatusManaged,
		Executable:  true,
	}
}

func blockedFriendTouch(reason string) PlannedOp {
	planned := markerOp(CategoryPlant, "farm.friend_steal", "steal", reason, friendTouchPriority)
	planned.FeatureID = "plant.friend_steal"
	planned.Status = PlanStatusBlocked
	planned.Executable = false
	planned.BlockedReasons = []string{reason}
	return planned
}

func unsupportedFriendElves() PlannedOp {
	reason := "花灵可摸状态与成功回包尚未完成实测，暂不发送 stealElves=1"
	planned := markerOp(CategoryPlant, "farm.friend_steal", "steal_elves", reason, friendTouchPriority+1)
	planned.FeatureID = "plant.friend_steal_elves"
	planned.Status = PlanStatusAdapterMissing
	planned.Executable = false
	planned.BlockedReasons = []string{reason}
	return planned
}

// ValidateFriendTouchMutation re-plans immediately before a mutating RPC and
// rejects a stale queued target. This closes the decision-to-execution gap for
// quota, friendship, availability, and land changes.
func ValidateFriendTouchMutation(s *state.State, policy *pb.FriendStealPolicy, queued *PlannedOp, now time.Time) error {
	if queued == nil {
		return fmt.Errorf("好友摸花操作为空")
	}
	if queued.Kind != clientproto.RPCFrdStealSteal.String() && queued.Kind != clientproto.RPCFrdExtBuyStealCnt.String() {
		return nil
	}
	current, ok := PlanOneFriendTouch(s, policy, now)
	if !ok || !current.Executable || current.Status == PlanStatusBlocked || current.Status == PlanStatusAdapterMissing {
		return fmt.Errorf("好友摸花前置状态已变化")
	}
	if current.Kind != queued.Kind || current.TargetUID != queued.TargetUID || current.TargetID != queued.TargetID || current.Count != queued.Count || !sameFriendTouchItemCost(current.ItemCost, queued.ItemCost) {
		return fmt.Errorf("好友摸花目标已变化：计划=%s/%d/%d，当前=%s/%d/%d", queued.Kind, queued.TargetUID, queued.TargetID, current.Kind, current.TargetUID, current.TargetID)
	}
	return nil
}

func sameFriendTouchItemCost(a, b map[int32]int32) bool {
	if len(a) != len(b) {
		return false
	}
	for itemID, count := range a {
		if b[itemID] != count {
			return false
		}
	}
	return true
}

func calendarDayID(now time.Time) int32 {
	local := now.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	return int32(local.Year()*10000 + int(local.Month())*100 + local.Day())
}
