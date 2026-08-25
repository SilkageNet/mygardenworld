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
	friendTouchSkipEnterCooldown = 5 * time.Minute
	// Idle friend-list sync stays below ordinary work so it only runs when the
	// planner has nothing higher-priority left after login.
	friendListIdlePriority = int32(1500)
	// Observed client biEnter point type for opening a friend garden.
	frdStealEnterPointType = 22
)

type friendTouchTarget struct {
	UID   int64
	Count int32
}

func friendTouchOperations(s *state.State, policy *pb.FriendTouchPolicy, now time.Time) []PlannedOp {
	ops := friendListIdleSyncOperations(s, now)
	if policy == nil || !policy.GetEnabled() {
		return ops
	}
	if op, ok := PlanOneFriendTouch(s, policy, now); ok {
		return append(ops, op)
	}
	return ops
}

// friendListIdleSyncOperations pulls the friend roster and missing names after
// login. It does not require friend-touch to be enabled; low priority keeps it
// out of the way until other work is idle.
func friendListIdleSyncOperations(s *state.State, now time.Time) []PlannedOp {
	if s == nil {
		return nil
	}
	view := s.FriendTouch(now)
	goal := Goal{ID: "basic.friend_list", Category: CategoryBasic, Domain: "basic.friend_list", Label: "好友列表", Priority: 15}
	if !view.FriendsObserved {
		op := friendListSyncOp(clientproto.RPCFrdEnter.String(), goal, "friend", "登录后空闲同步好友列表", nil, friendListIdlePriority+1)
		return []PlannedOp{op}
	}
	profileUIDs := friendListMissingProfileUIDs(view)
	if len(profileUIDs) == 0 {
		return nil
	}
	op := friendListSyncOp(clientproto.RPCOpptGetDetailOppts.String(), goal, "profile", "登录后空闲同步好友名称", profileUIDs, friendListIdlePriority)
	return []PlannedOp{op}
}

func friendListMissingProfileUIDs(view state.FriendTouchView) []int64 {
	out := make([]int64, 0, len(view.FriendUIDs))
	for _, uid := range view.FriendUIDs {
		if uid <= 0 {
			continue
		}
		profile, exists := view.Profiles[uid]
		if !exists || profile.ObservedAtMs <= 0 || strings.TrimSpace(profile.Name) == "" {
			out = append(out, uid)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func friendListSyncOp(kind string, goal Goal, source, reason string, uids []int64, priority int32) PlannedOp {
	op := PlannedOp{
		OperationID: kind + ":friend_list:" + source,
		GoalID:      goal.ID,
		Kind:        kind,
		Lane:        LaneSide,
		FeatureID:   "basic.friend_list_sync",
		Category:    goal.Category,
		Label:       goal.Label,
		Domain:      goal.Domain,
		Action:      "sync",
		Reason:      reason,
		Priority:    priority,
		Status:      PlanStatusManaged,
		Executable:  true,
		TargetUIDs:  append([]int64(nil), uids...),
	}
	return op
}

// PlanOneFriendTouch advances the friend-touch state machine by at most one op.
func PlanOneFriendTouch(s *state.State, policy *pb.FriendTouchPolicy, now time.Time) (PlannedOp, bool) {
	if s == nil || policy == nil || !policy.GetEnabled() {
		return PlannedOp{}, false
	}
	cfg, ok := state.FriendTouchConfigFromCatalog()
	if !ok {
		return blockedFriendTouch("好友摘花目录常量缺失"), true
	}
	view := s.FriendTouch(now)
	goal := Goal{ID: "basic.friend_touch", Category: CategoryBasic, Domain: "basic.friend_touch", Label: "好友摸花", Priority: 55}
	mode := friendTouchMode(policy)
	excluded := friendTouchExcludeSet(policy)

	// Steal path may still sync urgently if idle sync has not finished yet.
	if !view.FriendsObserved {
		return friendTouchSyncOp(clientproto.RPCFrdEnter.String(), goal, "friend", "好友列表未同步，先拉取好友", nil, friendTouchPriority+6), true
	}

	targets := friendTouchTargets(policy, view, cfg, now)
	if len(targets) == 0 {
		return PlannedOp{}, false
	}

	profileUIDs := friendTouchProfileUIDs(view, targets, pb.SelectionMode_SELECTION_MODE_SPECIFIC, excluded, now)
	if len(profileUIDs) > 0 {
		return friendTouchSyncOp(clientproto.RPCOpptGetDetailOppts.String(), goal, "profile", "摘花目标好友名称未同步", profileUIDs, friendTouchPriority+5), true
	}

	otherUIDs := friendTouchOtherInfoUIDs(view, targets, pb.SelectionMode_SELECTION_MODE_SPECIFIC, excluded, now)
	if len(otherUIDs) > 0 {
		return friendTouchSyncOp(clientproto.RPCFrdExtGetFrdOtherInfoByUids.String(), goal, "steal_state", "好友摘花状态未同步", otherUIDs, friendTouchPriority+4), true
	}

	for _, target := range targets {
		if !friendTouchStealableTarget(view, target, cfg, now) {
			continue
		}
		stolen := friendTouchStolenCount(view, target.UID, now)
		remaining := target.Count - stolen
		if remaining <= 0 {
			continue
		}
		left := friendTouchStealLeft(view, target.UID, cfg, now)
		if left <= 0 {
			if op, ok := planFriendTouchBuy(s, policy, cfg, goal, view, target.UID, now); ok {
				return op, true
			}
			continue
		}
		label := friendTouchLabel(view, target.UID)
		if s.FriendTouchSkipEnter(target.UID, now) {
			continue
		}
		if !friendTouchVisitFresh(view, target.UID, now) {
			reason := fmt.Sprintf("进入好友 %s 花园，准备单次摸花", label)
			op := friendTouchBaseOp(clientproto.RPCFrdStealEnterFrdSteal.String(), goal, "enter", reason, friendTouchPriority+3)
			op.OperationID = clientproto.RPCFrdStealEnterFrdSteal.String() + ":" + strconv.FormatInt(target.UID, 10)
			op.TargetUID = target.UID
			return op, true
		}
		landID, ok := state.PickFriendStealLandID(view.VisitLands, s.Inventory(), s.RoleID(), now)
		if !ok {
			if len(view.VisitLands) > 0 {
				s.MarkFriendTouchSkipEnter(target.UID, now.Add(friendTouchSkipEnterCooldown))
			}
			continue
		}
		reason := friendTouchStealReason(mode, label, target.Count, stolen)
		op := friendTouchBaseOp(clientproto.RPCFrdStealSteal.String(), goal, "steal", reason, friendTouchPriority)
		op.OperationID = clientproto.RPCFrdStealSteal.String() + ":" + strconv.FormatInt(target.UID, 10) + ":" + strconv.FormatInt(int64(landID), 10)
		op.TargetUID = target.UID
		op.TargetID = landID
		op.Count = 1
		if policy.GetStealElves() {
			op.SlotID = 1
		}
		return op, true
	}
	return PlannedOp{}, false
}

func friendTouchStealableTarget(view state.FriendTouchView, target friendTouchTarget, cfg state.FriendTouchConfig, now time.Time) bool {
	if target.UID <= 0 {
		return false
	}
	stolen := friendTouchStolenCount(view, target.UID, now)
	if target.Count-stolen <= 0 {
		return false
	}
	if friendTouchStealLeft(view, target.UID, cfg, now) <= 0 {
		return false
	}
	info, ok := view.OtherInfo[target.UID]
	if !ok || !friendTouchInfoFresh(info.ObservedAt, now) || !info.IsSteal {
		return false
	}
	return true
}

func friendTouchMode(policy *pb.FriendTouchPolicy) pb.SelectionMode {
	if policy == nil {
		return pb.SelectionMode_SELECTION_MODE_ALL
	}
	switch policy.GetMode() {
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

func friendTouchExcludeSet(policy *pb.FriendTouchPolicy) map[int64]struct{} {
	out := make(map[int64]struct{})
	if policy == nil {
		return out
	}
	for _, uid := range policy.GetExcludeUids() {
		if uid > 0 {
			out[uid] = struct{}{}
		}
	}
	return out
}

func friendTouchTargets(policy *pb.FriendTouchPolicy, view state.FriendTouchView, cfg state.FriendTouchConfig, now time.Time) []friendTouchTarget {
	if policy == nil {
		return nil
	}
	excluded := friendTouchExcludeSet(policy)
	mode := friendTouchMode(policy)
	out := make([]friendTouchTarget, 0)
	switch mode {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		for uid, count := range policy.GetFriendCounts() {
			if uid <= 0 || count <= 0 {
				continue
			}
			if _, skip := excluded[uid]; skip {
				continue
			}
			out = append(out, friendTouchTarget{UID: uid, Count: count})
		}
	default:
		for _, uid := range view.FriendUIDs {
			if uid <= 0 {
				continue
			}
			if _, skip := excluded[uid]; skip {
				continue
			}
			max := cfg.StealMax + friendTouchBuyCount(view, uid, now)
			if max <= 0 {
				max = cfg.StealMax
			}
			out = append(out, friendTouchTarget{UID: uid, Count: max})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID < out[j].UID })
	return out
}

func friendTouchCandidateUIDs(view state.FriendTouchView, targets []friendTouchTarget, mode pb.SelectionMode, excluded map[int64]struct{}) []int64 {
	if mode == pb.SelectionMode_SELECTION_MODE_SPECIFIC {
		out := make([]int64, 0, len(targets))
		for _, target := range targets {
			out = append(out, target.UID)
		}
		return out
	}
	out := make([]int64, 0, len(view.FriendUIDs))
	for _, uid := range view.FriendUIDs {
		if uid <= 0 {
			continue
		}
		if _, skip := excluded[uid]; skip {
			continue
		}
		out = append(out, uid)
	}
	return out
}

func planFriendTouchBuy(s *state.State, policy *pb.FriendTouchPolicy, cfg state.FriendTouchConfig, goal Goal, view state.FriendTouchView, uid int64, now time.Time) (PlannedOp, bool) {
	if !policy.GetAutoBuyTimes() {
		return PlannedOp{}, false
	}
	maxBuy := policy.GetMaxBuyPerFriend()
	if maxBuy <= 0 {
		maxBuy = cfg.PickMax
	}
	bought := friendTouchBuyCount(view, uid, now)
	if bought >= maxBuy {
		return PlannedOp{}, false
	}
	costEach := cfg.PickAddCost
	if costEach <= 0 {
		costEach = 1
	}
	inv := s.Inventory()
	if inv[state.FriendCoinItemID()] < costEach {
		return PlannedOp{}, false
	}
	label := friendTouchLabel(view, uid)
	reason := fmt.Sprintf("为好友 %s 兑换 1 次摘花次数（友情币 %d）", label, costEach)
	op := friendTouchBaseOp(clientproto.RPCFrdExtBuyStealCnt.String(), goal, "buy", reason, friendTouchPriority+1)
	op.OperationID = clientproto.RPCFrdExtBuyStealCnt.String() + ":" + strconv.FormatInt(uid, 10)
	op.TargetUID = uid
	op.Count = 1
	op.ItemCost = map[int32]int32{state.FriendCoinItemID(): costEach}
	op.FeatureID = "basic.friend_touch_buy"
	return op, true
}

func friendTouchProfileUIDs(view state.FriendTouchView, targets []friendTouchTarget, mode pb.SelectionMode, excluded map[int64]struct{}, now time.Time) []int64 {
	candidates := friendTouchCandidateUIDs(view, targets, mode, excluded)
	out := make([]int64, 0, len(candidates))
	for _, uid := range candidates {
		profile, exists := view.Profiles[uid]
		if !exists || !cacheFresh(profile.ObservedAtMs, now) {
			out = append(out, uid)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func friendTouchOtherInfoUIDs(view state.FriendTouchView, targets []friendTouchTarget, mode pb.SelectionMode, excluded map[int64]struct{}, now time.Time) []int64 {
	candidates := friendTouchCandidateUIDs(view, targets, mode, excluded)
	out := make([]int64, 0, len(candidates))
	for _, uid := range candidates {
		info, exists := view.OtherInfo[uid]
		if !exists || !friendTouchInfoFresh(info.ObservedAt, now) {
			out = append(out, uid)
		}
	}
	return out
}

func friendTouchStolenCount(view state.FriendTouchView, uid int64, now time.Time) int32 {
	if !friendTouchStealMapFresh(view, now) {
		return 0
	}
	return view.StealMap[uid]
}

func friendTouchBuyCount(view state.FriendTouchView, uid int64, now time.Time) int32 {
	if !friendTouchBuyMapFresh(view, now) {
		return 0
	}
	return view.StealCntBuyMap[uid]
}

func friendTouchStealLeft(view state.FriendTouchView, uid int64, cfg state.FriendTouchConfig, now time.Time) int32 {
	max := cfg.StealMax + friendTouchBuyCount(view, uid, now)
	stolen := friendTouchStolenCount(view, uid, now)
	left := max - stolen
	if left < 0 {
		return 0
	}
	return left
}

func friendTouchStealMapFresh(view state.FriendTouchView, now time.Time) bool {
	if !view.StealObserved || view.StealRTimeMs <= 0 {
		return false
	}
	return calendarDayID(now) == calendarDayID(time.UnixMilli(view.StealRTimeMs))
}

func friendTouchBuyMapFresh(view state.FriendTouchView, now time.Time) bool {
	if view.StealCntBuyRTimeMs <= 0 {
		return len(view.StealCntBuyMap) > 0
	}
	return calendarDayID(now) == calendarDayID(time.UnixMilli(view.StealCntBuyRTimeMs))
}

func friendTouchInfoFresh(observedAtMs int64, now time.Time) bool {
	if observedAtMs <= 0 {
		return false
	}
	return now.UnixMilli()-observedAtMs <= friendTouchOtherInfoTTL.Milliseconds()
}

func friendTouchVisitFresh(view state.FriendTouchView, uid int64, now time.Time) bool {
	if view.VisitUID != uid || view.VisitObservedAtMs <= 0 {
		return false
	}
	return now.UnixMilli()-view.VisitObservedAtMs <= friendTouchVisitTTL.Milliseconds()
}

func friendTouchLabel(view state.FriendTouchView, uid int64) string {
	if profile, ok := view.Profiles[uid]; ok && profile.Name != "" {
		return profile.Name
	}
	return strconv.FormatInt(uid, 10)
}

func friendTouchStealReason(mode pb.SelectionMode, label string, targetCount, stolen int32) string {
	if mode == pb.SelectionMode_SELECTION_MODE_SPECIFIC {
		return fmt.Sprintf("向好友 %s 摘花（目标 %d 次，今日已摘 %d 次）", label, targetCount, stolen)
	}
	return fmt.Sprintf("向好友 %s 摘花（全部可摘，今日已摘 %d 次）", label, stolen)
}

func friendTouchSyncOp(kind string, goal Goal, source, reason string, uids []int64, priority int32) PlannedOp {
	op := friendTouchBaseOp(kind, goal, "sync", reason, priority)
	op.OperationID = kind + ":friend_touch:" + source
	op.TargetUIDs = append([]int64(nil), uids...)
	return op
}

func friendTouchBaseOp(kind string, goal Goal, action, reason string, priority int32) PlannedOp {
	return PlannedOp{
		OperationID: kind,
		GoalID:      goal.ID,
		Kind:        kind,
		Lane:        LaneSide,
		FeatureID:   "basic.friend_touch",
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
	op := markerOp(CategoryBasic, "basic.friend_touch", "steal", reason, friendTouchPriority)
	op.Status = PlanStatusBlocked
	op.Executable = false
	op.BlockedReasons = []string{reason}
	return op
}

func calendarDayID(now time.Time) int32 {
	local := now.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	return int32(local.Year()*10000 + int(local.Month())*100 + local.Day())
}
