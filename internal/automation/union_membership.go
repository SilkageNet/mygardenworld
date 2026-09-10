package automation

import (
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// FmlMembershipNextSyncAt is shared by planning and recovery diagnostics.
// Unknown identity retries at 30s, 2m, then 5m; confirmed absence is checked
// only every 5m while guild automation is enabled. No retry reconnects a client.
func FmlMembershipNextSyncAt(build state.FmlBuildView) time.Time {
	if build.MembershipObserved && build.MemberFmlID > 0 {
		return time.Time{}
	}
	if build.MembershipSyncAtMs <= 0 {
		return time.Time{}
	}
	interval := 30 * time.Second
	if build.MembershipObserved || build.MembershipSyncAttempts >= 3 {
		interval = 5 * time.Minute
	} else if build.MembershipSyncAttempts == 2 {
		interval = 2 * time.Minute
	}
	return time.UnixMilli(build.MembershipSyncAtMs).Add(interval)
}

func unionMembershipOperations(build state.FmlBuildView, policy *pb.UnionPolicy, now time.Time) []PlannedOp {
	if !UnionAutomationEnabled(policy) || now.Before(FmlMembershipNextSyncAt(build)) {
		return nil
	}
	goal := Goal{ID: "union.membership", Category: CategoryUnion, Domain: "union.membership", Label: "公会身份同步", Priority: 43}
	op := domainOp(clientproto.RPCFmlEnter.String(), goal, "union.membership.sync", "sync",
		"公会身份待确认，重新读取当前成员信息", 4490, 0, 0, 0)
	op.CooldownKey = "union.membership.sync"
	return []PlannedOp{op}
}

// UnionAutomationEnabled allows membership recovery only for configured work,
// including land/forest users who do not participate in the guild race.
func UnionAutomationEnabled(policy *pb.UnionPolicy) bool {
	return policy.GetBuild().GetFreeEnabled() || policy.GetBuild().GetGoldEnabled() ||
		policy.GetFlower().GetShareEnabled() || policy.GetFlower().GetTakeEnabled() ||
		policy.GetLand().GetHarvestEnabled() || policy.GetLand().GetAutoPlantEnabled() ||
		policy.GetRace().GetEnabled() || policy.GetForestEnabled() || policy.GetRedPacketEnabled()
}
