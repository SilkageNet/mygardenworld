package apiserver

import (
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/runner"
)

func buildDomainStatuses(policy *pb.Policy, diag runner.Diagnostics, connected bool) []*pb.DomainStatus {
	policy = automation.DefaultPolicyIfNil(policy)
	observed := setOfStrings(diag.ObservedNamespaces)
	blocked := append([]string(nil), diag.BlockedReasons...)
	statuses := []*pb.DomainStatus{
		domainStatus("basic", "basic", basicEnabled(policy.GetBasic()), observedAny(observed, "7", "22", "33", "116", "117", "119", "129"), blocked, "", connected),
		domainStatus("plant", "plant", plantEnabled(policy.GetPlant()), observedAny(observed, "100", "101", "104", "105", "109", "114"), blocked, "", connected),
		domainStatus("order", "order", orderEnabled(policy.GetOrder()), observedAny(observed, "104", "105", "107", "108", "109"), blocked, "", connected),
		domainStatus("union", "union", unionEnabled(policy.GetUnion()), observedAny(observed, "25", "152"), blocked, "", connected),
		domainStatus("activity", "activity", activityEnabled(policy.GetActivity()), observedActivity(observed), blocked, "", connected),
	}
	applyOperationCooldownsToDomainStatuses(statuses, diag.OperationCooldowns)
	return statuses
}

func featureCapabilitiesProto() []*pb.FeatureCapability {
	specs := automation.FeatureCatalog()
	out := make([]*pb.FeatureCapability, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if _, exists := seen[spec.ID]; exists {
			continue
		}
		seen[spec.ID] = struct{}{}
		out = append(out, &pb.FeatureCapability{
			Id:             spec.ID,
			Label:          spec.Label,
			Category:       spec.Category,
			Domain:         spec.Domain,
			Action:         spec.Action,
			Status:         planStatusProto(spec.Status),
			Executable:     spec.Executable,
			SyncOnly:       spec.SyncOnly,
			BlockedReasons: append([]string(nil), spec.BlockedReasons...),
		})
	}
	return out
}

func domainStatus(category, domain string, enabled bool, observed bool, blocked []string, lastErr string, connected bool) *pb.DomainStatus {
	status := pb.PlanStatus_PLAN_STATUS_SKIPPED
	var reasons []string
	if enabled {
		status = pb.PlanStatus_PLAN_STATUS_READY
		if !connected {
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
			reasons = append(reasons, "WebSocket 未连接")
		}
		if len(blocked) > 0 {
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
			reasons = append(reasons, blocked...)
		}
		if !observed && connected {
			status = pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		}
	}
	return &pb.DomainStatus{
		Category:       category,
		Domain:         domain,
		Lane:           defaultDomainLane(category),
		Observed:       observed,
		Status:         status,
		BlockedReasons: reasons,
		LastError:      lastErr,
	}
}

func defaultDomainLane(category string) pb.ExecutionLane {
	if category == automation.CategoryPlant {
		return pb.ExecutionLane_EXECUTION_LANE_FARM
	}
	return pb.ExecutionLane_EXECUTION_LANE_SIDE
}

func applyOperationCooldownsToDomainStatuses(statuses []*pb.DomainStatus, cooldowns []runner.OperationCooldownSnapshot) {
	for _, status := range statuses {
		var selected runner.OperationCooldownSnapshot
		for _, cd := range cooldowns {
			if cd.Category != status.GetCategory() {
				continue
			}
			if selected.Until.IsZero() || cd.Until.Before(selected.Until) {
				selected = cd
			}
		}
		if selected.Until.IsZero() {
			continue
		}
		status.Lane = executionLaneProto(selected.Lane)
		status.CooldownUntilMs = selected.Until.UnixMilli()
		status.CooldownReason = selected.Reason
		status.Status = pb.PlanStatus_PLAN_STATUS_BLOCKED
		if selected.Reason != "" && !containsString(status.BlockedReasons, selected.Reason) {
			status.BlockedReasons = append(status.BlockedReasons, selected.Reason)
		}
	}
}

func accountHealth(connected bool, diag runner.Diagnostics) string {
	switch {
	case diag.SessionInvalidatedReason != "":
		return "session_expired"
	case len(diag.BlockedReasons) > 0:
		return "blocked"
	case connected:
		return "online"
	default:
		return "offline"
	}
}

// applyStoppedRunnerDiagnostics fills status fields when no live runner remains.
// Retained kick/expiry diagnostics keep the account in 异常 instead of bare offline.
func applyStoppedRunnerDiagnostics(out *pb.AccountStatus, policy *pb.Policy, diag runner.Diagnostics) {
	out.Diagnostics = runnerDiagnosticsProto(diag)
	out.DomainStatuses = buildDomainStatuses(policy, diag, false)
	out.Health = accountHealth(false, diag)
	if diag.LastOperationError != "" {
		out.LastError = diag.LastOperationError
		return
	}
	out.LastError = diag.SessionInvalidatedReason
}

func basicEnabled(p *pb.BasicPolicy) bool {
	task := p.GetTask()
	benefit := p.GetBenefit()
	sign := p.GetSign()
	return p != nil && (task.GetMainEnabled() || task.GetDailyEnabled() || task.GetWeeklyEnabled() ||
		task.GetStoryEnabled() || task.GetAchievementEnabled() || p.GetMailEnabled() ||
		sign.GetDailyEnabled() || sign.GetPatchEnabled() || p.GetFreeWaterEnabled() ||
		p.GetWaterwheelEnabled() || benefit.GetBoxEnabled() || benefit.GetDoubleCoinEnabled() ||
		benefit.GetShareRewardEnabled() || benefit.GetAntiScamBoxEnabled() ||
		p.GetMapEventEnabled() || p.GetRoadGrowRewardEnabled() ||
		p.GetPearl().GetFreeEnabled() || p.GetShop().GetVideoFreeGiftEnabled() ||
		p.GetZoo().GetEnabled())
}

func plantEnabled(p *pb.PlantPolicy) bool {
	planting := p.GetPlanting()
	cultivate := p.GetCultivate()
	return p != nil && (planting.GetAutoEnabled() || planting.GetAutoHarvestEnabled() ||
		planting.GetUseSpeedUpTicket() || planting.GetVideoSpeedUpEnabled() || planting.GetAutoUnlockLand() ||
		cultivate.GetEnabled() || cultivate.GetUpgradeEnabled() || cultivate.GetVideoSpeedUpEnabled() ||
		p.GetFriendSteal().GetEnabled() || p.GetElves().GetEnabled() || p.GetMarket().GetPutEnabled() ||
		p.GetMarket().GetAutoBuyFromFriend())
}

func orderEnabled(p *pb.OrderPolicy) bool {
	return p != nil && (p.GetCustomer().GetEnabled() || residentOrderEnabledProto(p.GetResident()) ||
		p.GetPalace().GetEnabled() || p.GetTeam().GetEnabled() || p.GetFlowerArt().GetSellEnabled() ||
		p.GetFlowerArt().GetCraftEnabled())
}

func residentOrderEnabledProto(p *pb.ResidentOrderPolicy) bool {
	return p != nil && (p.GetNormalEnabled() || p.GetDecorateEnabled() || p.GetSatinEnabled() ||
		p.GetRewardEnabled())
}

func unionEnabled(p *pb.UnionPolicy) bool {
	build := p.GetBuild()
	flower := p.GetFlower()
	race := p.GetRace()
	land := p.GetLand()
	return p != nil && (build.GetFreeEnabled() || build.GetGoldEnabled() || build.GetDiamondEnabled() ||
		flower.GetShareEnabled() || flower.GetTakeEnabled() || race.GetEnabled() ||
		land.GetAutoPlantEnabled() || land.GetHarvestEnabled() ||
		p.GetRedPacketEnabled() || p.GetForestEnabled())
}

func activityEnabled(p *pb.ActivityPolicy) bool {
	if p == nil {
		return false
	}
	// Parent ActivityPolicy.enabled is legacy/no-op; modules are independent.
	for _, module := range p.GetModules() {
		if module != nil && module.GetEnabled() {
			return true
		}
	}
	return false
}

func observedActivity(observed map[string]struct{}) bool {
	return observedAny(observed, "138", "139", "140", "152", "155", "160", "161", "162", babigame.CelebrityNamespaceLegacy, babigame.CelebrityNamespace)
}

func setOfStrings(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[x] = struct{}{}
	}
	return out
}

func observedAny(set map[string]struct{}, keys ...string) bool {
	for _, key := range keys {
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

func cloneInt32Map(in map[int32]int32) map[int32]int32 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int32]int32, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
