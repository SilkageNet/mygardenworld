package automation

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	pearlCandidateCacheTTL = 30 * time.Second
	pearlHirePriority      = int32(5540)
)

// PearlHireIntent contains only queue metadata. It intentionally has no gate
// overrides, so activity task drivers can reuse the helper without bypassing
// the user's pearl policy, ticket, slot, level, protection, or session lock.
type PearlHireIntent struct {
	GoalID   string
	DemandID string
	Category string
	Domain   string
	Label    string
	Reason   string
	Priority int32
}

// PlanOneSafePearlHire advances the ticket-only hire state machine by at most
// one synchronization or hire operation.
func PlanOneSafePearlHire(s *state.State, policy *pb.PearlPolicy, now time.Time, intent PearlHireIntent) (PlannedOp, bool) {
	return planOneSafePearlHire(s, policy, now, intent, nil)
}

func planOneSafePearlHire(s *state.State, policy *pb.PearlPolicy, now time.Time, intent PearlHireIntent, diagnostic *PearlHireDiagnostic) (PlannedOp, bool) {
	intent = normalizePearlHireIntent(intent)
	if s == nil || policy == nil || !policy.GetAutoHireEnabled() {
		return PlannedOp{}, false
	}
	if policy.GetMaxHireTicketUsage() <= 0 {
		return blockedPearlHire(intent, "同时在岗人数上限为 0，自动雇佣已禁用"), true
	}
	if policy.GetMaxHireLevel() < 0 {
		return blockedPearlHire(intent, "雇佣等级上限不能为负数"), true
	}
	if policy.GetDailyHireTicketLimit() < 0 {
		return blockedPearlHire(intent, "每日雇佣券上限不能为负数"), true
	}
	config, ok := state.PearlHireConfigFromCatalog()
	if !ok || config.TicketItemID != 1003 {
		return blockedPearlHire(intent, "珍珠雇佣目录常量缺失或雇佣券不是已实测的 item 1003"), true
	}
	view := s.PearlHireAt(now)
	if diagnostic != nil {
		diagnostic.ResourcesObserved = true
		diagnostic.Tickets = view.TicketCount
		diagnostic.UsedToday = view.TicketUsedToday
	}
	if view.RoleID <= 0 {
		return blockedPearlHire(intent, "自己的 UID 尚未可靠同步，无法排除 self 候选"), true
	}
	if view.SessionLocked {
		reason := view.SessionLockReason
		if reason == "" {
			reason = "当前会话因珍珠雇佣结果不明确，已停止自动雇佣"
		}
		return blockedPearlHire(intent, reason), true
	}
	if view.TicketCount < 1 {
		return blockedPearlHire(intent, "雇佣券 item 1003 不足，不会购买或使用金币回退"), true
	}
	if limit := policy.GetDailyHireTicketLimit(); limit > 0 && view.TicketUsedToday >= limit {
		return blockedPearlHire(intent, fmt.Sprintf("今日已使用雇佣券 %d 张，已达到每日上限 %d", view.TicketUsedToday, limit)), true
	}

	activeWorkers, activeUIDs, workersKnown := pearlActiveWorkers(view.Places, now)
	if diagnostic != nil {
		diagnostic.ActiveWorkers = activeWorkers
		diagnostic.WorkersKnown = workersKnown
	}
	if !workersKnown {
		return blockedPearlHire(intent, "珍珠槽位占用字段不完整，无法安全计算同时在岗人数"), true
	}
	if activeWorkers >= policy.GetMaxHireTicketUsage() {
		return blockedPearlHire(intent, fmt.Sprintf("当前在岗 %d 人，已达到策略上限 %d", activeWorkers, policy.GetMaxHireTicketUsage())), true
	}
	placeID, ok := firstSafePearlHireSlot(view, config, now)
	if !ok {
		return blockedPearlHire(intent, "没有已解锁且空闲的珍珠槽位；不会自动解锁槽位"), true
	}

	seen := make(map[int64]struct{})
	if !view.FriendsObserved {
		return pearlHireSyncOp(clientproto.RPCFrdEnter.String(), intent, "friend", "好友来源尚未同步，先按实测只请求好友列表", nil, pearlHirePriority+6), true
	}
	friendDiagnostic := diagnostic.source(0)
	friendUIDs := filterPearlCandidateUIDs(view.FriendUIDs, view, activeUIDs, seen, now, friendDiagnostic)
	if op, done := planPearlCandidateSource(view, config, policy, now, intent, "friend", "好友", friendUIDs, placeID, friendDiagnostic); done {
		return op, true
	}

	if !cacheFresh(view.RecommendObservedAtMs, now) {
		return pearlHireSyncOp(clientproto.RPCPearlGetRecommendList.String(), intent, "recommend", "推荐候选缓存缺失或已超过 30 秒，刷新推荐列表", nil, pearlHirePriority+3), true
	}
	recommendDiagnostic := diagnostic.source(1)
	recommendUIDs := filterPearlCandidateUIDs(view.RecommendUIDs, view, activeUIDs, seen, now, recommendDiagnostic)
	if op, done := planPearlCandidateSource(view, config, policy, now, intent, "recommend", "推荐", recommendUIDs, placeID, recommendDiagnostic); done {
		return op, true
	}

	if !view.EnemiesObserved {
		return pearlHireSyncOp(clientproto.RPCPearlRefresh.String(), intent, "enemy", "仇人来源尚未同步，先刷新自己的珍珠状态", nil, pearlHirePriority+2), true
	}
	enemyUIDs := pearlEnemyUIDs(view.Enemies, config.EnemyMaxDays, now)
	enemyDiagnostic := diagnostic.source(2)
	enemyUIDs = filterPearlCandidateUIDs(enemyUIDs, view, activeUIDs, seen, now, enemyDiagnostic)
	if op, done := planPearlCandidateSource(view, config, policy, now, intent, "enemy", "仇人", enemyUIDs, placeID, enemyDiagnostic); done {
		return op, true
	}
	return blockedPearlHire(intent, "好友、推荐和三日内仇人中暂无满足等级、保护期、失败冷却与当前会话跳过门槛的候选"), true
}

// ValidateSafePearlHire reruns the same planner gates immediately before an
// RPC. The requested UID/place must still be the unique next safe hire.
func ValidateSafePearlHire(s *state.State, policy *pb.PearlPolicy, op *PlannedOp, now time.Time) error {
	if op == nil {
		return fmt.Errorf("pearl hire operation is nil")
	}
	planned, ok := PlanOneSafePearlHire(s, policy, now, PearlHireIntent{
		GoalID: op.GoalID, DemandID: op.DemandID, Category: op.Category,
		Domain: op.Domain, Label: op.Label, Reason: op.Reason, Priority: op.Priority,
	})
	if !ok || planned.Kind != clientproto.RPCPearlPlaceHire.String() {
		if planned.Reason != "" {
			return fmt.Errorf("pearl hire preflight rejected: %s", planned.Reason)
		}
		return fmt.Errorf("pearl hire preflight rejected")
	}
	if planned.TargetID != op.TargetID || planned.TargetUID != op.TargetUID {
		return fmt.Errorf("pearl hire preflight candidate changed: place=%d uid=%d", planned.TargetID, planned.TargetUID)
	}
	if op.Count != 1 || op.GoldCost != 0 || op.DiamondCost != 0 ||
		len(op.ItemCost) != 1 || op.ItemCost[1003] != 1 {
		return fmt.Errorf("pearl hire preflight requires exact ItemCost{1003:1}")
	}
	return nil
}

func planPearlCandidateSource(view state.PearlHireView, config state.PearlHireConfig, policy *pb.PearlPolicy, now time.Time, intent PearlHireIntent, source, sourceLabel string, uids []int64, placeID int32, diagnostic *PearlHireSourceDiagnostic) (PlannedOp, bool) {
	if len(uids) == 0 {
		return PlannedOp{}, false
	}
	staleProfiles := make([]int64, 0, len(uids))
	staleHireStates := make([]int64, 0, len(uids))
	var readyUID int64
	for _, uid := range uids {
		profile, exists := view.Profiles[uid]
		if !exists || !cacheFresh(profile.ObservedAtMs, now) {
			staleProfiles = append(staleProfiles, uid)
			diagnostic.record(uid, "profile_unavailable", 0)
			continue
		}
		if !profile.LevelObserved || profile.Level <= 0 {
			diagnostic.record(uid, "level_unknown", 0)
			continue
		}
		diagnostic.observeLevel(profile.Level, policy.GetMaxHireLevel())
		if maxLevel := policy.GetMaxHireLevel(); maxLevel > 0 && profile.Level > maxLevel {
			diagnostic.record(uid, "level_exceeded", profile.Level)
			continue
		}
		// An over-level candidate cannot be hired, so its missing protection
		// state must not hold up the remaining candidates or next source.
		hireState, exists := view.HireStates[uid]
		if !exists || !cacheFresh(hireState.ObservedAtMs, now) {
			staleHireStates = append(staleHireStates, uid)
			diagnostic.record(uid, "protection_unavailable", profile.Level)
			continue
		}
		restTimeMs := config.RestTimeSeconds * int64(time.Second/time.Millisecond)
		if hireState.LaborEndTimeMs > math.MaxInt64-restTimeMs {
			diagnostic.record(uid, "protection_invalid", profile.Level)
			continue
		}
		availableAt := hireState.LaborEndTimeMs + restTimeMs
		if hireState.LaborEndTimeMs > 0 && now.UnixMilli() < availableAt {
			diagnostic.record(uid, "protected", profile.Level)
			continue
		}
		diagnostic.record(uid, "eligible", profile.Level)
		if readyUID == 0 {
			readyUID = uid
		}
	}
	// Prefer an already fully validated candidate in this source over waiting
	// for unrelated incomplete profiles. Final execution rechecks all gates.
	if readyUID != 0 {
		uid := readyUID
		profile := view.Profiles[uid]
		reason := intent.Reason
		if reason == "" {
			reason = fmt.Sprintf("%s候选 %s 已通过等级、保护期和雇佣券门槛", sourceLabel, pearlCandidateLabel(profile))
		}
		op := pearlHireBaseOp(clientproto.RPCPearlPlaceHire.String(), intent, "hire", reason, intent.Priority)
		op.OperationID = clientproto.RPCPearlPlaceHire.String() + ":" + strconv.FormatInt(int64(placeID), 10) + ":" + strconv.FormatInt(uid, 10)
		op.TargetID = placeID
		op.TargetUID = uid
		op.Count = 1
		op.ItemCost = map[int32]int32{1003: 1}
		return op, true
	}
	if len(staleProfiles) > 0 {
		return pearlHireSyncOp(clientproto.RPCOpptGetDetailOppts.String(), intent, source, sourceLabel+"候选详情缺失或已超过 30 秒", staleProfiles, pearlHirePriority+5), true
	}
	if len(staleHireStates) > 0 {
		return pearlHireSyncOp(clientproto.RPCPearlGetHireStateByUids.String(), intent, source, sourceLabel+"候选保护状态缺失或已超过 30 秒", staleHireStates, pearlHirePriority+4), true
	}
	return PlannedOp{}, false
}

func pearlHireSyncOp(kind string, intent PearlHireIntent, source, reason string, uids []int64, priority int32) PlannedOp {
	op := pearlHireBaseOp(kind, intent, "sync", reason, priority)
	op.OperationID = kind + ":" + source
	op.TargetUIDs = append([]int64(nil), uids...)
	return op
}

func pearlHireBaseOp(kind string, intent PearlHireIntent, action, reason string, priority int32) PlannedOp {
	return PlannedOp{
		OperationID: kind,
		GoalID:      intent.GoalID, DemandID: intent.DemandID, Kind: kind,
		Lane: LaneSide, FeatureID: "basic.pearl_hire", Category: intent.Category,
		Label: intent.Label, Domain: intent.Domain, Action: action,
		Status: PlanStatusManaged, Executable: true, Reason: reason, Priority: priority,
	}
}

func blockedPearlHire(intent PearlHireIntent, reason string) PlannedOp {
	op := pearlHireBaseOp("basic.pearl.hire.blocked", intent, "hire", reason, intent.Priority)
	op.OperationID = "basic.pearl.hire.blocked"
	op.Status = PlanStatusBlocked
	op.Executable = false
	op.BlockedReasons = []string{reason}
	return op
}

func normalizePearlHireIntent(intent PearlHireIntent) PearlHireIntent {
	if intent.GoalID == "" {
		intent.GoalID = "basic.pearl"
	}
	if intent.Category == "" {
		intent.Category = CategoryBasic
	}
	if intent.Domain == "" {
		intent.Domain = "basic.pearl.hire"
	}
	if intent.Label == "" {
		intent.Label = "雇佣劳工"
	}
	if intent.Priority == 0 {
		intent.Priority = pearlHirePriority
	}
	return intent
}

func pearlActiveWorkers(places map[int32]state.PearlPlaceView, now time.Time) (int32, map[int64]struct{}, bool) {
	activeUIDs := make(map[int64]struct{})
	var active int32
	for _, place := range places {
		if !place.LaborUIDObserved || !place.LaborEndTimeObserved {
			return 0, nil, false
		}
		if place.LaborUID < 0 || place.LaborEndTime < 0 || (place.LaborUID == 0) != (place.LaborEndTime == 0) {
			return 0, nil, false
		}
		if place.LaborUID <= 0 {
			continue
		}
		activeUIDs[place.LaborUID] = struct{}{}
		if place.LaborEndTime > now.UnixMilli() {
			active++
		}
	}
	return active, activeUIDs, true
}

func firstSafePearlHireSlot(view state.PearlHireView, config state.PearlHireConfig, now time.Time) (int32, bool) {
	ids := make([]int32, 0, len(config.Slots))
	for id := range config.Slots {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		slotConfig := config.Slots[id]
		place, observed := view.Places[id]
		// c_pearl slot 4 requires a live recharge-card type 1. State does not
		// yet model namespace 21.5, and ordinary VIP is not equivalent, so
		// monthly-card slots remain permanently fail-closed in this commit.
		if !observed || !place.LaborUIDObserved || !place.LaborEndTimeObserved || slotConfig.MonthlyCardUnlock {
			continue
		}
		if !pearlPlaceHireSlotFree(place, now.UnixMilli()) {
			continue
		}
		return id, true
	}
	return 0, false
}

func pearlPlaceHireSlotFree(place state.PearlPlaceView, nowMs int64) bool {
	if place.LaborUID == 0 && place.LaborEndTime == 0 {
		return true
	}
	// The server can retain the previous UID after the shift has ended. Both
	// fields must be complete and the end instant must have passed.
	return place.LaborUID > 0 && place.LaborEndTime > 0 && place.LaborEndTime <= nowMs
}

func filterPearlCandidateUIDs(input []int64, view state.PearlHireView, activeUIDs map[int64]struct{}, seen map[int64]struct{}, now time.Time, diagnostic *PearlHireSourceDiagnostic) []int64 {
	if diagnostic != nil {
		diagnostic.Checked = true
		diagnostic.Candidates = len(input)
	}
	out := make([]int64, 0, len(input))
	for _, uid := range input {
		if uid <= 0 || uid == view.RoleID {
			diagnostic.record(uid, "self_or_invalid", 0)
			continue
		}
		if _, exists := activeUIDs[uid]; exists {
			diagnostic.record(uid, "in_slot", 0)
			continue
		}
		if until := view.FailedUntilMs[uid]; until > now.UnixMilli() {
			diagnostic.record(uid, "failure_cooldown", 0)
			continue
		}
		if _, skipped := view.SkippedUIDs[uid]; skipped {
			diagnostic.record(uid, "session_skip", 0)
			continue
		}
		if _, exists := seen[uid]; exists {
			diagnostic.record(uid, "duplicate", 0)
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

func pearlEnemyUIDs(enemies []state.PearlEnemyView, maxDays int64, now time.Time) []int64 {
	if maxDays <= 0 {
		return nil
	}
	cutoff := now.Add(-time.Duration(maxDays) * 24 * time.Hour).UnixMilli()
	out := make([]int64, 0, len(enemies))
	for _, enemy := range enemies {
		if enemy.UID > 0 && enemy.EventTimeMs >= cutoff && enemy.EventTimeMs <= now.UnixMilli() {
			out = append(out, enemy.UID)
		}
	}
	return out
}

func cacheFresh(observedAtMs int64, now time.Time) bool {
	if observedAtMs <= 0 || now.IsZero() {
		return false
	}
	age := now.Sub(time.UnixMilli(observedAtMs))
	return age >= 0 && age < pearlCandidateCacheTTL
}

func pearlCandidateLabel(profile state.PearlCandidateProfile) string {
	name := strings.TrimSpace(profile.Name)
	if name == "" {
		name = strconv.FormatInt(profile.UID, 10)
	}
	return fmt.Sprintf("%s(Lv.%d)", name, profile.Level)
}
