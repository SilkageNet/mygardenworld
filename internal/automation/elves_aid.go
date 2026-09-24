package automation

import (
	"fmt"
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	elvesAidReceivePriority = int32(6075)
	elvesAidRequestPriority = int32(6070)
	elvesAidHelpPriority    = int32(5535)
	// Above elves plant (9800) so claim/sync runs before plant/water/speed-up.
	elvesPlantAidGatePriority = int32(9900)
)

// flowerElvesAidOperations plans request/receive/help. Each toggle is independent
// of ElvesPlantPolicy.enabled (auto plant) and FlowerElvesPolicy.enabled.
// When auto plant is on, pending recv/sync is also planned here (even if
// receive_aid is off) so planting can wait for the claim.
func flowerElvesAidOperations(s *state.State, plant *pb.PlantPolicy, now time.Time) []PlannedOp {
	p := plant.GetElves()
	if p == nil {
		if !elvesPlantActive(plant.GetElvesPlant()) {
			return nil
		}
		p = &pb.FlowerElvesPolicy{}
	}
	var ops []PlannedOp
	if planned, ok := planFlowerElvesAidReceive(s, p, elvesPlantActive(plant.GetElvesPlant()), now); ok {
		ops = append(ops, planned)
	}
	if planned, ok := planFlowerElvesAidRequest(s, p, now); ok {
		ops = append(ops, planned)
	}
	if planned, ok := PlanOneFlowerElvesAidHelp(s, p, now); ok {
		ops = append(ops, planned)
	}
	return ops
}

// elvesPlantBlockedByPendingAid is true while auto plant must wait: aid state
// unknown, or recvAidEff is ready. If the claimed buff is still active
// (effEndTime in the future), planting proceeds immediately.
func elvesPlantBlockedByPendingAid(s *state.State, now time.Time) bool {
	if s == nil {
		return false
	}
	if flowerElvesAidEffectActive(s, now) {
		return false
	}
	if !s.FlowerElvesAidObserved() {
		return true
	}
	return s.CanRecvFlowerElvesAid(now)
}

func flowerElvesAidEffectActive(s *state.State, now time.Time) bool {
	if s == nil || !s.FlowerElvesAidObserved() {
		return false
	}
	end := s.FlowerElvesAid().EffEndTime
	return end > 0 && now.UnixMilli() < end
}

func planFlowerElvesAidReceive(s *state.State, p *pb.FlowerElvesPolicy, plantOn bool, now time.Time) (PlannedOp, bool) {
	if s == nil || p == nil {
		return PlannedOp{}, false
	}
	if !p.GetReceiveAid() && !plantOn {
		return PlannedOp{}, false
	}
	// Auto-plant only forces claim/sync when there is no active buff yet.
	if !p.GetReceiveAid() && plantOn && flowerElvesAidEffectActive(s, now) {
		return PlannedOp{}, false
	}
	goal := Goal{ID: "farm.elves_aid", Category: CategoryPlant, Domain: "farm.elves_aid", Label: "花灵协助", Priority: 60}
	priority := elvesAidReceivePriority
	if plantOn && !flowerElvesAidEffectActive(s, now) {
		priority = elvesPlantAidGatePriority
	}
	if !s.FlowerElvesAidObserved() {
		reason := "花灵协助状态未同步"
		if plantOn {
			reason = "种植花灵前先同步花灵协助"
		}
		planned := op(clientproto.RPCFlowerElvesCheckConvert.String(), goal, "sync", reason, priority+1, 0, 0, 0)
		planned.FeatureID = "plant.elves_aid_receive"
		return planned, true
	}
	if !s.CanRecvFlowerElvesAid(now) {
		return PlannedOp{}, false
	}
	reason := "好友协助已满，领取花灵协助效果"
	if plantOn && !flowerElvesAidEffectActive(s, now) {
		reason = "种植花灵前先领取花灵协助"
	}
	planned := op(clientproto.RPCFlowerElvesAidRecvAidEff.String(), goal, "claim", reason, priority, 0, 0, 0)
	planned.FeatureID = "plant.elves_aid_receive"
	return planned, true
}

func planFlowerElvesAidRequest(s *state.State, p *pb.FlowerElvesPolicy, now time.Time) (PlannedOp, bool) {
	if s == nil || p == nil || !p.GetRequestAid() {
		return PlannedOp{}, false
	}
	goal := Goal{ID: "farm.elves_aid", Category: CategoryPlant, Domain: "farm.elves_aid", Label: "花灵协助", Priority: 60}
	if !s.FlowerElvesAidObserved() {
		planned := op(clientproto.RPCFlowerElvesCheckConvert.String(), goal, "sync", "花灵协助状态未同步", elvesAidRequestPriority+2, 0, 0, 0)
		planned.FeatureID = "plant.elves_aid_request"
		return planned, true
	}
	if !s.CanReqFlowerElvesAid(now) {
		return PlannedOp{}, false
	}
	planned := op(clientproto.RPCFlowerElvesAidReqAid.String(), goal, "request", "申请花灵好友协助", elvesAidRequestPriority, 0, 0, 0)
	planned.FeatureID = "plant.elves_aid_request"
	return planned, true
}

// PlanOneFlowerElvesAidHelp advances help-friend by at most one operation.
// Does not require ElvesPlantPolicy.enabled.
func PlanOneFlowerElvesAidHelp(s *state.State, p *pb.FlowerElvesPolicy, now time.Time) (PlannedOp, bool) {
	if s == nil || p == nil || !p.GetHelpFriend() {
		return PlannedOp{}, false
	}
	g, ok := state.FlowerElvesGlobalsFromCatalog()
	if !ok {
		return PlannedOp{}, false
	}
	helpMax := g.HelpMax
	if helpMax <= 0 {
		helpMax = 5
	}
	if s.FlowerElvesAidHelpCountToday(now) >= helpMax {
		return PlannedOp{}, false
	}
	goal := Goal{ID: "farm.elves_aid_help", Category: CategoryPlant, Domain: "farm.elves_aid", Label: "协助好友花灵", Priority: 56}
	view := s.FriendTouch(now)
	if !view.FriendsObserved {
		planned := friendTouchSyncOp(clientproto.RPCFrdEnter.String(), goal, "friend", "好友列表未同步，先拉取好友关系", nil, elvesAidHelpPriority+6)
		planned.FeatureID = "plant.elves_aid_help"
		return planned, true
	}
	if len(view.FriendUIDs) == 0 {
		return PlannedOp{}, false
	}
	targets := append([]int64(nil), view.FriendUIDs...)
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })

	stale := make([]int64, 0, len(targets))
	for _, uid := range targets {
		info := view.OtherInfo[uid]
		if !friendTouchInfoFresh(info.ObservedAt, now) {
			stale = append(stale, uid)
		}
	}
	if len(stale) > 0 {
		planned := friendTouchSyncOp(clientproto.RPCFrdExtGetFrdOtherInfoByUids.String(), goal, "availability", "同步好友花灵协助需求", firstUIDs(stale), elvesAidHelpPriority+4)
		planned.FeatureID = "plant.elves_aid_help"
		return planned, true
	}

	for _, uid := range targets {
		info := view.OtherInfo[uid]
		if !info.IsAid {
			continue
		}
		if s.FlowerElvesAidHelpedToday(uid, now) {
			continue
		}
		label := friendTouchLabel(view, uid)
		planned := friendTouchBaseOp(clientproto.RPCFlowerElvesAidHelpFrd.String(), goal, "help",
			fmt.Sprintf("协助好友 %s 花灵", label), elvesAidHelpPriority)
		planned.FeatureID = "plant.elves_aid_help"
		planned.TargetUID = uid
		planned.OperationID = clientproto.RPCFlowerElvesAidHelpFrd.String() + ":" + fmt.Sprintf("%d", uid)
		return planned, true
	}
	return PlannedOp{}, false
}
