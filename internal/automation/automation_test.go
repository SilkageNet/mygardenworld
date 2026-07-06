package automation

import (
	"strconv"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func applyMap(t *testing.T, s *state.State, top map[string]any) {
	t.Helper()
	s.ApplyVMap(top)
}

func emptyLands(n int) map[string]any {
	lands := make(map[string]any, n)
	for i := 0; i < n; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	return lands
}

func cultivate(flowers ...int32) map[string]any {
	out := make(map[string]any, len(flowers))
	for _, id := range flowers {
		out[itoa32(id)] = map[string]any{"1": id, "2": 1, "4": 2}
	}
	return out
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func itoa32(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func oppositeQuality(flowerID int32) int32 {
	q := flowerQuality(flowerID)
	if q == 1 {
		return 2
	}
	return q - 1
}

func hasReasonContaining(reasons []string, part string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, part) {
			return true
		}
	}
	return false
}

func TestRecommend_State2WaitsForHarvestGrace(t *testing.T) {
	now := time.Date(2026, 7, 3, 15, 3, 59, 0, time.UTC)
	land := state.LandView{
		Observed:   true,
		FlowerID:   23001,
		State:      2,
		NextTimeMs: now.Add(-1 * time.Second).UnixMilli(),
	}

	if kind, reason := Recommend(land, now); kind != KindWait {
		t.Fatalf("state=2 should wait inside harvest grace, got kind=%s reason=%s", kind, reason)
	}

	land.NextTimeMs = now.Add(-harvestReadyGrace - time.Second).UnixMilli()
	if kind, reason := Recommend(land, now); kind != KindHarvest {
		t.Fatalf("state=2 should harvest after harvest grace, got kind=%s reason=%s", kind, reason)
	}
}

func TestRecommend_State3HarvestsImmediately(t *testing.T) {
	now := time.Date(2026, 7, 3, 15, 3, 59, 0, time.UTC)
	land := state.LandView{
		Observed: true,
		FlowerID: 23001,
		State:    3,
	}

	if kind, reason := Recommend(land, now); kind != KindHarvest {
		t.Fatalf("state=3 should harvest immediately, got kind=%s reason=%s", kind, reason)
	}
}

func TestBuildPlan_HarvestGroupsReadyLands(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	s := state.New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3},
			"1002": map[string]any{"0": 23002, "1": 3},
			"1003": map[string]any{"0": 23003, "1": 1},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind != clientproto.RPCUsrLandHarvest.String() {
			continue
		}
		if len(op.LandIDs) != 2 || op.LandIDs[0] != 1001 || op.LandIDs[1] != 1002 {
			t.Fatalf("harvest LandIDs=%v, want [1001 1002]", op.LandIDs)
		}
		return
	}
	t.Fatalf("missing harvest op: %+v", result.Operations)
}

func TestBuildPlan_ResidentNormalDisabledDoesNotDemandOrSubmit(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = false

	result := BuildPlan(s, p, time.Now())
	for _, demand := range result.Demands {
		if demand.GoalID == GoalResidentOrder {
			t.Fatalf("resident demand should not be generated when normal order is disabled: %+v", demand)
		}
	}
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() {
			t.Fatalf("resident submit should not be generated when normal order is disabled: %+v", op)
		}
	}
}

func TestBuildPlan_ResidentNormalLimitBlocksSubmit(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260702": map[string]any{"1": 20260702, "9": 5}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 5

	result := BuildPlan(s, p, time.Now())
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should be blocked after daily limit: %+v", op)
		}
		if op.Domain == "order.resident" && !op.Executable && hasReasonContaining(op.BlockedReasons, "5/5") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing resident limit block: %+v", result.Operations)
	}
}

func TestBuildPlan_ResidentServerDailyLimitMarkerBlocksSubmit(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260705": map[string]any{"1": 20260705, "9": 1259}}},
	})
	s.MarkResidentOrderDailyLimitReached(now)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true

	result := BuildPlan(s, p, now)
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should be blocked after server daily limit: %+v", op)
		}
		if op.Domain == "order.resident" && !op.Executable && hasReasonContaining(op.BlockedReasons, "今日完成订单次数已达上限") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing resident server limit block: %+v", result.Operations)
	}
}

func TestBuildPlan_ResidentQualityMismatchBlocksSubmit(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.Qualities = []int32{oppositeQuality(23005)}

	result := BuildPlan(s, p, time.Now())
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should be blocked by quality policy: %+v", op)
		}
		if op.Domain == "order.resident" && !op.Executable && hasReasonContaining(op.BlockedReasons, "不在策略范围") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing resident quality block: %+v", result.Operations)
	}
}

func TestBuildPlan_ResidentMissingStatisticsStillSubmitsWithDiagnosticReason(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 1

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			if !strings.Contains(op.Reason, "namespace 124") {
				t.Fatalf("resident submit should mention missing stats namespace: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing resident submit when statistics are absent: %+v", result.Operations)
}

func TestBuildPlan_ResidentCooldownOmitsSubmitUntilReady(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{
			"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}, "4": now.Add(30 * time.Second).UnixMilli(), "5": now.UnixMilli()},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 10

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.TargetID == 1 {
			t.Fatalf("resident submit should be omitted during cooldown: %+v", op)
		}
	}

	result = BuildPlan(s, p, now.Add(31*time.Second))
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.TargetID == 1 && op.Executable {
			return
		}
	}
	t.Fatalf("missing resident submit after cooldown: %+v", result.Operations)
}

func TestBuildPlan_CustomerArtDemandDrivesPlanting(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 0, "23007": 0, "23008": 0},
			"34": 12,
		}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(3)}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true

	result := BuildPlan(s, p, time.Now())
	var planted bool
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.FlowerID != 0 {
			planted = true
			break
		}
	}
	if !planted {
		t.Fatalf("expected customer art flower demand to produce plant op, ops=%+v demands=%+v", result.Operations, result.Demands)
	}
}

func TestBuildPlan_CustomerArtBlockedByMissingVase(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 2, "23007": 2, "23008": 2},
			"34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3001": map[string]any{"1": 3001}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true

	result := BuildPlan(s, p, time.Now())
	var blocked bool
	for _, op := range result.Operations {
		if !op.Executable && hasReasonContaining(op.BlockedReasons, "花瓶") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("expected missing vase block, ops=%+v", result.Operations)
	}
}

func TestBuildPlan_CustomerRejectUnavailableWhenUnlockedRequirementsMissing(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 2, "23007": 2, "23008": 2},
			"34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3001": map[string]any{"1": 3001}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if !op.Executable || op.TargetID != 7 || !strings.Contains(op.Reason, "花瓶") {
				t.Fatalf("reject op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer reject op: %+v", result.Operations)
}

func TestBuildPlan_CustomerMissingRecipeBlocksWithoutReject(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 399999, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	result := BuildPlan(s, p, time.Now())
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && op.Executable {
			t.Fatalf("missing recipe should not execute reject: %+v", op)
		}
		if !op.Executable && hasReasonContaining(op.BlockedReasons, "配方") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing recipe block not found: %+v", result.Operations)
	}
}

func TestBuildPlan_CustomerArtConfigLevelDoesNotReject(t *testing.T) {
	recipe, ok := state.FlowerArtRecipeByID(301606)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(301606) ok=false")
	}
	stock := make(map[string]any, len(recipe.Flowers))
	for _, flowerID := range recipe.Flowers {
		stock[itoa32(flowerID)] = int32(1)
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": stock,
			"34": recipe.Level - 1,
		}},
		"101": map[string]any{"0": cultivate(recipe.Flowers...)},
		"102": map[string]any{"0": map[string]any{itoa32(recipe.VaseID): map[string]any{"1": recipe.VaseID}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"6": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if strings.Contains(op.Reason, "等级不足") || hasReasonContaining(op.BlockedReasons, "等级不足") {
			t.Fatalf("flower art cfg lvl should not be treated as player level gate: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			t.Fatalf("customer order should not be rejected by flower art cfg lvl: %+v", op)
		}
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && op.ItemID == recipe.ArtID {
			if !op.Executable || op.Count != 1 {
				t.Fatalf("customer craft op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer craft op: %+v", result.Operations)
}

func TestBuildPlan_CustomerEmptyOrdersGenerateWhenCooldownReady(t *testing.T) {
	now := time.Date(2026, 7, 3, 9, 0, 0, 0, time.Local)
	s := state.New()
	applyMap(t, s, map[string]any{
		"109": map[string]any{"0": map[string]any{
			"1": map[string]any{},
			"2": now.Add(-2 * time.Second).UnixMilli(),
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerGenOrder.String() {
			if !op.Executable || op.Action != "generate" {
				t.Fatalf("customer gen op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer gen op: %+v", result.Operations)
}

func TestBuildPlan_CustomerEmptyOrdersRespectGenerationCooldown(t *testing.T) {
	now := time.Date(2026, 7, 3, 9, 0, 0, 0, time.Local)
	s := state.New()
	applyMap(t, s, map[string]any{
		"109": map[string]any{"0": map[string]any{
			"1": map[string]any{},
			"2": now.Add(time.Minute).UnixMilli(),
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerGenOrder.String() {
			t.Fatalf("customer gen should wait for cooldown: %+v", op)
		}
	}
}

func TestPlan_FarmLaneBeatsCustomerOrderGeneration(t *testing.T) {
	now := time.Date(2026, 7, 3, 9, 0, 0, 0, time.Local)
	s := state.New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23005, "1": 3, "5": now.Add(-time.Minute).UnixMilli()},
		}},
		"109": map[string]any{"0": map[string]any{
			"1": map[string]any{},
			"2": now.Add(-2 * time.Second).UnixMilli(),
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoEnabled = true
	p.Order.Customer.Enabled = true

	op := Plan(s, p, now)
	if op == nil || op.Kind != clientproto.RPCUsrLandHarvest.String() || op.Lane != LaneFarm {
		t.Fatalf("Plan()=%+v, want farm harvest before customer gen", op)
	}
}

func TestBuildPlan_FarmLaneBeatsDailyTaskClaim(t *testing.T) {
	now := time.Date(2026, 7, 5, 11, 30, 0, 0, time.UTC)
	s := state.New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(1)},
		"101": map[string]any{"0": cultivate(23001)},
		"22": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"4": 569},
				"3": map[string]any{},
				"100": map[string]any{
					"40001": map[string]any{"0": 40001, "1": 569, "2": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoEnabled = true
	p.Basic.Task.DailyEnabled = true

	result := BuildPlan(s, p, now)
	if len(result.Operations) == 0 {
		t.Fatal("BuildPlan produced no operations")
	}
	first := result.Operations[0]
	if first.Lane != LaneFarm || first.Kind != clientproto.RPCUsrLandPlant.String() {
		t.Fatalf("first operation=%+v, want farm plant before daily task", first)
	}
	var daily *PlannedOp
	for i := range result.Operations {
		if result.Operations[i].Kind == clientproto.RPCTaskDlyRecv.String() {
			daily = &result.Operations[i]
			break
		}
	}
	if daily == nil || daily.Lane != LaneSide {
		t.Fatalf("daily task op=%+v, want side lane", daily)
	}
}

func TestBuildPlan_FlowerRackRespectsCustomerLedgerAllocation(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 1}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerRackSell.String() {
			t.Fatalf("customer art allocation should not be sold on rack: %+v demands=%+v", op, result.Demands)
		}
	}
}

func TestBuildPlan_FlowerRackUsesFixedRackCount(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 20,
		}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerRackSell.String() {
			if op.ItemID != 300208 || op.Count != 12 {
				t.Fatalf("fixed rack sell mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing rack sell op: %+v", result.Operations)
}

func TestBuildPlan_FlowerRackCraftsWhenNoStock(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 4, "23007": 4, "23008": 4},
			"34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true
	p.Order.FlowerArt.CraftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			if op.ItemID != 300208 || op.Count != 4 || op.VaseID != 3002 || !op.Executable {
				t.Fatalf("rack craft mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing rack craft op: %+v", result.Operations)
}

func TestBuildPlan_FlowerRackUsesCurrentCraftableCount(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 1, "23007": 3, "23008": 3},
			"34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true
	p.Order.FlowerArt.CraftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			if op.ItemID != 300208 || op.Count != 1 || !op.Executable {
				t.Fatalf("rack craft should use current craftable count: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing partial rack craft op: %+v", result.Operations)
}

func TestBuildPlan_FlowerRackMissingMaterialsSkipsPlanting(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 0, "23007": 0, "23008": 0},
			"34": 12,
		}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(3)}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true
	p.Order.FlowerArt.CraftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.GoalID == GoalFlowerArt && op.FlowerID != 0 {
			t.Fatalf("flower rack missing materials should not drive planting: op=%+v ops=%+v demands=%+v", op, result.Operations, result.Demands)
		}
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			t.Fatalf("flower rack should skip uncraftable art instead of blocking craft: %+v", op)
		}
	}
}

func TestBuildPlan_FlowerRackMissingVaseSkipsCraft(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 4, "23007": 4, "23008": 4},
			"34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3001": map[string]any{"1": 3001}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true
	p.Order.FlowerArt.CraftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			t.Fatalf("flower rack missing vase should skip craft instead of blocking: %+v", op)
		}
	}
}

func TestBuildPlan_FlowerRackClaimUsesRecvSellMoney(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	listedAt := now.Add(-time.Duration(state.FlowerRackSellDurationMs()) * time.Millisecond).UnixMilli()
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"2": map[string]any{"1": 2, "2": 300208, "3": 1, "4": listedAt, "5": listedAt},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if strings.Contains(op.Kind, "OneKey") || strings.Contains(op.Kind, "oneKey") {
			t.Fatalf("OneKey operation should not be generated: %+v", op)
		}
		if op.Kind == clientproto.RPCFlowerRackRecvSellMoney.String() {
			if !op.Executable || op.TargetID != 2 {
				t.Fatalf("recvSellMoney op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing recvSellMoney op: %+v", result.Operations)
}

func TestPlan_FlowerRackClaimBeatsHarvest(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	listedAt := now.Add(-time.Duration(state.FlowerRackSellDurationMs()) * time.Millisecond).UnixMilli()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1001": map[string]any{"1": 23005, "2": 3, "4": now.Add(-time.Minute).UnixMilli()},
		}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 1, "4": listedAt, "5": listedAt},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoEnabled = true
	p.Order.FlowerArt.SellEnabled = true

	op := Plan(s, p, now)
	if op == nil || op.Kind != clientproto.RPCFlowerRackRecvSellMoney.String() {
		t.Fatalf("Plan()=%+v, want rack claim before harvest", op)
	}
}

func TestBuildPlan_DoesNotGenerateOneKeyOperations(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 10},
		}},
		"19": map[string]any{"1": []any{
			map[string]any{"1": 101, "2": 201, "13": [][]int32{{1, 5}}, "20": 0},
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23005, "1": 3, "5": now.Add(-time.Minute).UnixMilli()},
			"1002": map[string]any{"0": 23007, "1": 1},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.MailEnabled = true
	p.Plant.Planting.AutoEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if strings.Contains(op.Kind, "OneKey") || strings.Contains(op.Kind, "oneKey") {
			t.Fatalf("OneKey operation should not be generated: %+v", op)
		}
	}
}

func TestBuildPlan_WaterClaimsRespectCapacityAndThreshold(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		drops     int32
		total     int32
		threshold int32
		wantOps   bool
	}{
		{name: "below capacity", drops: 12, total: 130, wantOps: true},
		{name: "at capacity", drops: 130, total: 130, wantOps: false},
		{name: "above threshold", drops: 80, total: 130, threshold: 50, wantOps: false},
		{name: "below threshold", drops: 20, total: 130, threshold: 50, wantOps: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{
				"7": map[string]any{"0": map[string]any{
					"32": map[string]any{"7": tt.drops},
					"33": map[string]any{"7": map[string]any{"1": tt.total}},
				}},
				"117": map[string]any{
					"1": 2,
					"2": now.UnixMilli(),
				},
			})
			p := DefaultPolicy()
			p.AutomationEnabled = true
			p.Basic.WaterwheelEnabled = true
			p.Basic.FreeWaterEnabled = true
			p.Basic.WaterClaimThreshold = tt.threshold

			result := BuildPlan(s, p, now)
			gotWaterwheel := false
			gotFreeWater := false
			for _, op := range result.Operations {
				gotWaterwheel = gotWaterwheel || op.Kind == clientproto.RPCWaterwheelRecv.String()
				gotFreeWater = gotFreeWater || op.Kind == clientproto.RPCFreeWaterRecv.String()
			}
			if gotWaterwheel {
				t.Fatalf("waterwheel should wait for local bucket generation, ops=%+v", result.Operations)
			}
			if gotFreeWater != tt.wantOps {
				t.Fatalf("free water claim = %t, want %t; ops=%+v", gotFreeWater, tt.wantOps, result.Operations)
			}
		})
	}
}

func TestAnnotateSequentialResourceBudgetBlocksCumulativeWaterDrops(t *testing.T) {
	now := time.Now()
	st := state.New()
	applyMap(t, st, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 5},
			"33": map[string]any{"7": map[string]any{"1": 65, "5": int64(0)}}}},
	})
	ops := []PlannedOp{
		{
			Kind:       clientproto.RPCUsrLandWaterBatch.String(),
			Executable: true,
			CostGates:  []CostGate{resourceGate("water_drop", GateResourceWaterDrop, "水滴", 7, 3, 5, "operation.resource")},
		},
		{
			Kind:       clientproto.RPCUsrLandWaterBatch.String(),
			Executable: true,
			CostGates:  []CostGate{resourceGate("water_drop", GateResourceWaterDrop, "水滴", 7, 3, 5, "operation.resource")},
		},
	}

	annotateSequentialResourceBudget(st, ops, now)
	if !ops[0].Executable {
		t.Fatalf("first op executable = false, want true: %+v", ops[0])
	}
	if ops[1].Executable || ops[1].Status != PlanStatusBlocked || !hasReasonContaining(ops[1].BlockedReasons, "队列资源不足") {
		t.Fatalf("second op = %+v, want queue resource block", ops[1])
	}
	if len(ops[1].CostGates) != 2 || ops[1].CostGates[1].Source != "operation.queue" {
		t.Fatalf("second op gates = %+v, want operation.queue gate", ops[1].CostGates)
	}
}

func TestAnnotateSequentialResourceBudgetBlocksCumulativeGoldAndItems(t *testing.T) {
	now := time.Now()
	st := state.New()
	applyMap(t, st, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"1001": 3},
			"44": 100,
		}},
	})
	ops := []PlannedOp{
		{
			Kind:       clientproto.RPCShopCultivateBuy.String(),
			Executable: true,
			CostGates: []CostGate{
				resourceGate("gold", GateResourceGold, "金币", 0, 70, 100, "operation.cost"),
				resourceGate("item:1001", GateResourceItem, "加速券", 1001, 2, 3, "operation.cost"),
			},
		},
		{
			Kind:       clientproto.RPCShopCultivateBuy.String(),
			Executable: true,
			CostGates: []CostGate{
				resourceGate("gold", GateResourceGold, "金币", 0, 40, 100, "operation.cost"),
				resourceGate("item:1001", GateResourceItem, "加速券", 1001, 2, 3, "operation.cost"),
			},
		},
	}

	annotateSequentialResourceBudget(st, ops, now)
	if !ops[0].Executable {
		t.Fatalf("first op executable = false, want true: %+v", ops[0])
	}
	if ops[1].Executable || ops[1].Status != PlanStatusBlocked {
		t.Fatalf("second op = %+v, want queue resource block", ops[1])
	}
	if !hasReasonContaining(ops[1].BlockedReasons, "金币") || !hasReasonContaining(ops[1].BlockedReasons, "加速卡") {
		t.Fatalf("second op reasons = %v, want gold and item queue blocks", ops[1].BlockedReasons)
	}
}

func TestBuildPlan_OrderPalaceAndTeamSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Palace.Enabled = true
	p.Order.Team.Enabled = true

	result := BuildPlan(s, p, time.Now())
	want := map[string]string{"order.palace": clientproto.RPCOrderPalaceEnter.String(), "order.team": clientproto.RPCOrderTeamRefreshOrder.String()}
	for _, op := range result.Operations {
		kind, ok := want[op.Domain]
		if !ok {
			continue
		}
		if op.Kind != kind || op.Executable || !op.SyncOnly || op.Status != PlanStatusSyncOnly {
			t.Fatalf("palace/team sync op mismatch: %+v", op)
		}
		delete(want, op.Domain)
	}
	if len(want) > 0 {
		t.Fatalf("missing sync ops %v: %+v", want, result.Operations)
	}
}

func TestBuildPlan_OrderPalaceAndTeamSubmitWhenStockAvailable(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 3, "23007": 2}}},
		"107": map[string]any{"0": map[string]any{"1": 1, "3": 2, "4": 23007, "6": 2}},
		"108": map[string]any{"0": map[string]any{"0": map[string]any{"1": 23005, "2": 3, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Palace.Enabled = true
	p.Order.Team.Enabled = true

	result := BuildPlan(s, p, time.Now())
	want := map[string]string{"order.palace": clientproto.RPCOrderPalaceFinishOrder.String(), "order.team": clientproto.RPCOrderTeamSubmitOrder.String()}
	for _, op := range result.Operations {
		kind, ok := want[op.Domain]
		if !ok {
			continue
		}
		if op.Kind != kind || op.Executable || !op.SyncOnly || op.Status != PlanStatusSyncOnly {
			t.Fatalf("palace/team submit op mismatch: %+v", op)
		}
		delete(want, op.Domain)
	}
	if len(want) > 0 {
		t.Fatalf("missing submit ops %v: %+v", want, result.Operations)
	}
}

func TestBuildPlan_FlowerArtRewardsProduceClaimOps(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"103": map[string]any{
			"0": map[string]any{
				"11": map[string]any{"1": 11, "2": 0, "3": 5, "4": []int32{}},
				"13": map[string]any{"1": 13, "2": 0, "3": 70, "4": []int32{}, "7": []int32{}},
			},
		},
		"106": map[string]any{"0": map[string]any{"2": []int32{300101}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.CreateRewardEnabled = true
	p.Order.FlowerArt.CollectRewardEnabled = true

	result := BuildPlan(s, p, time.Now())
	var createReward, collectReward bool
	for _, op := range result.Operations {
		if op.Domain == "order.flower_art.create_reward" {
			createReward = true
			if op.Kind != clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String() || op.TargetID != 3001 || !op.Executable || op.SyncOnly {
				t.Fatalf("create reward op mismatch: %+v", op)
			}
		}
		if op.Domain == "order.flower_art.collect_reward" {
			collectReward = true
			if op.Kind != clientproto.RPCCollectRwdRecv.String() || op.TargetID != 11 || !op.Executable || op.SyncOnly {
				t.Fatalf("collect reward op mismatch: %+v", op)
			}
		}
	}
	if !createReward || !collectReward {
		t.Fatalf("missing reward ops create=%t collect=%t ops=%+v", createReward, collectReward, result.Operations)
	}
}

func TestBuildPlan_ShopCultivateEnterBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateEnter.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("shop cultivate sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate sync op: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagEnterBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.giftbag" {
			if op.Kind != clientproto.RPCShopGiftbagEnter.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("shop giftbag sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop giftbag sync op: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagVideoGift(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"112": map[string]any{
			"1": map[string]any{"1": 3},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.video_gift" {
			if op.Kind != clientproto.RPCShopGiftbagBuy.String() || op.TargetID != 1 || op.Count != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("shop giftbag buy op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop giftbag buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagPaidGiftIgnoredAndVipBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"112": map[string]any{
			"1": map[string]any{"1": 4},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true
	p.Basic.Shop.VipShop.AutoBuy = true

	result := BuildPlan(s, p, time.Now())
	var vipBlocked bool
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.video_gift" {
			t.Fatalf("paid or exhausted giftbag should not produce buy op: %+v", op)
		}
		if op.Domain == "basic.shop.vip" {
			vipBlocked = !op.Executable && len(op.BlockedReasons) > 0
		}
	}
	if !vipBlocked {
		t.Fatalf("missing blocked vip shop op: %+v", result.Operations)
	}
}

func TestBuildPlan_AntiScamBoxLifecycle(t *testing.T) {
	cases := []struct {
		name       string
		status     int32
		wantKind   string
		wantAction string
		wantOp     bool
	}{
		{name: "not answered", status: 0, wantKind: clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(), wantAction: "answer", wantOp: true},
		{name: "ready to claim", status: 1, wantKind: clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(), wantAction: "claim", wantOp: true},
		{name: "claimed", status: state.AntiFraudQAStatusClaimed, wantOp: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{
				"7": map[string]any{
					"13": map[string]any{
						"1": map[string]any{"104": tc.status},
					},
				},
			})
			p := DefaultPolicy()
			p.AutomationEnabled = true
			p.Basic.Benefit.AntiScamBoxEnabled = true

			result := BuildPlan(s, p, time.Now())
			for _, op := range result.Operations {
				if op.Domain != "basic.benefit.anti_scam" {
					continue
				}
				if !tc.wantOp {
					t.Fatalf("claimed anti-scam reward should not produce op: %+v", op)
				}
				if op.Kind != tc.wantKind || op.Action != tc.wantAction || op.FeatureID != "basic.anti_scam_box" || !op.Executable || op.SyncOnly {
					t.Fatalf("anti-scam op mismatch: %+v", op)
				}
				return
			}
			if tc.wantOp {
				t.Fatalf("missing anti-scam op: %+v", result.Operations)
			}
		})
	}
}

func TestBuildPlan_DoubleCoinBlockedUnlessActive(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Benefit.DoubleCoinEnabled = true

	s := state.New()
	result := BuildPlan(s, p, now)
	var blocked PlannedOp
	for _, op := range result.Operations {
		if op.Domain == "basic.benefit.double_coin" {
			blocked = op
			break
		}
	}
	if blocked.Domain == "" || blocked.Executable || blocked.Status != PlanStatusAdapterMissing || blocked.FeatureID != "basic.double_coin" || len(blocked.BlockedReasons) == 0 {
		t.Fatalf("double coin blocked op mismatch: %+v", blocked)
	}
	if got := Plan(s, p, now); got != nil && got.Domain == "basic.benefit.double_coin" {
		t.Fatalf("Plan returned blocked double coin op: %+v", got)
	}

	applyMap(t, s, map[string]any{
		"118": map[string]any{
			"1": 1,
			"2": now.Add(time.Hour).UnixMilli(),
		},
	})
	result = BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.benefit.double_coin" {
			t.Fatalf("active double coin should not produce op: %+v", op)
		}
	}
}

func TestBuildPlan_ZooSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoFeed = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.zoo" {
			if op.Kind != clientproto.RPCZooEnterZoo.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("zoo sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing zoo sync op: %+v", result.Operations)
}

func TestBuildPlan_ZooFeedAndStroke(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"33": map[string]any{
			"0": map[string]any{"0": 77900091102482},
			"1": map[string]any{
				"1": map[string]any{
					"1":  1,
					"2":  50,
					"3":  20,
					"4":  []int32{1501},
					"5":  2,
					"12": now.Add(-time.Minute).UnixMilli(),
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoFeed = true
	p.Basic.Zoo.AutoStroke = true

	result := BuildPlan(s, p, now)
	want := map[string]string{
		"basic.zoo.feed":   clientproto.RPCZooFeedPets.String(),
		"basic.zoo.stroke": clientproto.RPCZooStrokePet.String(),
	}
	seen := map[string]bool{}
	for _, op := range result.Operations {
		if kind, ok := want[op.Domain]; ok {
			seen[op.Domain] = true
			if op.Kind != kind || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("zoo op mismatch for %s: %+v", op.Domain, op)
			}
		}
	}
	for domain := range want {
		if !seen[domain] {
			t.Fatalf("missing %s op: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_ZooCostAndEventBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"33": map[string]any{
			"0": map[string]any{"0": 1},
			"1": map[string]any{
				"1": map[string]any{
					"1": 1,
					"5": 5,
					"9": 4001,
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoBuyFood = true
	p.Basic.Zoo.AutoEventEnabled = true

	result := BuildPlan(s, p, time.Now())
	want := map[string]bool{"basic.zoo.buy_food": false, "basic.zoo.event": false}
	for _, op := range result.Operations {
		if _, ok := want[op.Domain]; ok {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("zoo blocked op mismatch: %+v", op)
			}
			want[op.Domain] = true
		}
	}
	for domain, seen := range want {
		if !seen {
			t.Fatalf("missing blocked %s op: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_StoryAchievementAndMapEvent(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.Local)
	t.Run("sync story and map before observed", func(t *testing.T) {
		s := state.New()
		p := DefaultPolicy()
		p.AutomationEnabled = true
		p.Basic.Task.StoryEnabled = true
		p.Basic.MapEventEnabled = true

		result := BuildPlan(s, p, now)
		seen := map[string]string{}
		for _, op := range result.Operations {
			seen[op.Domain+"."+op.Action] = op.Kind
		}
		if seen["basic.story.sync"] != clientproto.RPCStoryMainEnter.String() {
			t.Fatalf("missing story sync op: %+v", result.Operations)
		}
		if seen["basic.map_event.sync"] != clientproto.RPCRandomEventEnter.String() {
			t.Fatalf("missing map event sync op: %+v", result.Operations)
		}
	})

	t.Run("block story when cost missing", func(t *testing.T) {
		s := state.New()
		applyMap(t, s, map[string]any{
			"7": map[string]any{
				"0":   map[string]any{"32": map[string]any{"142": 1}},
				"101": map[string]any{"0": 1, "1": 1, "2": 0},
			},
		})
		p := DefaultPolicy()
		p.AutomationEnabled = true
		p.Basic.Task.StoryEnabled = true

		result := BuildPlan(s, p, now)
		for _, op := range result.Operations {
			if op.Domain == "basic.story" && op.Action == "unlock" {
				if op.Status != PlanStatusBlocked || op.Executable || len(op.BlockedReasons) == 0 {
					t.Fatalf("story blocked op mismatch: %+v", op)
				}
				return
			}
		}
		t.Fatalf("missing blocked story op: %+v", result.Operations)
	})

	t.Run("claim achievement and ready map event", func(t *testing.T) {
		s := state.New()
		applyMap(t, s, map[string]any{
			"22": map[string]any{"2": map[string]any{"1": map[string]any{"1": 3}, "3": map[string]any{}}},
			"129": map[string]any{"0": map[string]any{"1": map[string]any{
				"6002": map[string]any{"0": 6002, "1": 0, "2": 60020601},
			}}},
		})
		p := DefaultPolicy()
		p.AutomationEnabled = true
		p.Basic.Task.AchievementEnabled = true
		p.Basic.MapEventEnabled = true

		result := BuildPlan(s, p, now)
		seen := map[string]automationOp{}
		for _, op := range result.Operations {
			seen[op.Domain+"."+op.Action] = automationOp{kind: op.Kind, targetID: op.TargetID}
		}
		if got := seen["basic.task.achievement.claim"]; got.kind != clientproto.RPCTaskAchRecv.String() || got.targetID != 10001 {
			t.Fatalf("achievement op=%+v, want taskAch.recv 10001", got)
		}
		if got := seen["basic.map_event.claim"]; got.kind != clientproto.RPCRandomEventDoAffair.String() || got.targetID != 6002 {
			t.Fatalf("map event op=%+v, want doAffair 6002", got)
		}
	})
}

type automationOp struct {
	kind     string
	targetID int32
}

func TestBuildPlan_PearlRefreshBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.FreeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.pearl" {
			if op.Kind != clientproto.RPCPearlRefresh.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("pearl sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing pearl sync op: %+v", result.Operations)
}

func TestBuildPlan_PearlExecutableOps(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"115": map[string]any{
			"0": map[string]any{"1": map[string]any{"1": 1, "8": 2}},
			"1": map[string]any{"1": 0, "2": 1, "6": now.Add(-24 * time.Hour).UnixMilli()},
			"2": []int32{101, 102},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.FreeEnabled = true
	p.Basic.Pearl.DrawEnabled = true
	p.Basic.Pearl.ProtectEnabled = true

	result := BuildPlan(s, p, now)
	want := map[string]string{
		"basic.pearl.free":    clientproto.RPCPearlRecvDailyFree.String(),
		"basic.pearl.place":   clientproto.RPCPearlPlaceRecv.String(),
		"basic.pearl.protect": clientproto.RPCPearlSetProtectState.String(),
		"basic.pearl.draw":    clientproto.RPCPearlDraw.String(),
	}
	seen := map[string]bool{}
	for _, op := range result.Operations {
		if kind, ok := want[op.Domain]; ok {
			seen[op.Domain] = true
			if op.Kind != kind || !op.Executable || op.SyncOnly {
				t.Fatalf("pearl op mismatch for %s: %+v", op.Domain, op)
			}
			if op.Domain == "basic.pearl.place" && op.TargetID != 1 {
				t.Fatalf("pearl place target=%d, want 1", op.TargetID)
			}
			if op.Domain == "basic.pearl.protect" && op.TargetID != 1 {
				t.Fatalf("pearl protect target=%d, want 1", op.TargetID)
			}
			if op.Domain == "basic.pearl.draw" && op.Count != 1 {
				t.Fatalf("pearl draw count=%d, want 1", op.Count)
			}
		}
	}
	for domain := range want {
		if !seen[domain] {
			t.Fatalf("missing pearl op %s: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_PearlHireAndBuyTicketBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"115": map[string]any{"1": map[string]any{}}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.AutoHireEnabled = true
	p.Basic.Pearl.AutoBuyHireTicket = true
	p.Basic.Pearl.MaxSpendDiamond = 25

	result := BuildPlan(s, p, time.Now())
	var hireBlocked, buyBlocked bool
	for _, op := range result.Operations {
		switch op.Domain {
		case "basic.pearl.hire":
			hireBlocked = !op.Executable && len(op.BlockedReasons) > 0
		case "basic.pearl.buy_hire_ticket":
			buyBlocked = !op.Executable && len(op.BlockedReasons) > 0
		}
	}
	if !hireBlocked || !buyBlocked {
		t.Fatalf("expected pearl blocked ops hire=%t buy=%t ops=%+v", hireBlocked, buyBlocked, result.Operations)
	}
}

func TestBuildPlan_ShopCultivateBuyWithGoldBudget(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5000}},
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 3214}},
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 4000
	p.Basic.Shop.CultivateShop.ItemIds = []int32{1423}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateBuy.String() || op.TargetID != 10001 || op.ItemID != 1423 || op.GoldCost != 3214 || !op.Executable || op.SyncOnly {
				t.Fatalf("shop cultivate buy op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopCultivateDiamondCostBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{1, 10}},
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendDiamond = 20
	p.Basic.Shop.CultivateShop.ItemIds = []int32{10001}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("diamond shop cultivate op should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked shop cultivate op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildFreeAndGold(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 20000}},
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"1": 0, "2": 0}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.FreeEnabled = true
	p.Union.Build.GoldEnabled = true
	p.Union.Build.MaxSpendGold = 12000

	result := BuildPlan(s, p, time.Now())
	var freeSeen bool
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			freeSeen = true
			if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 1 || !op.Executable || op.SyncOnly || op.GoldCost != 0 {
				t.Fatalf("free union build op mismatch: %+v", op)
			}
			break
		}
	}
	if !freeSeen {
		t.Fatalf("missing free union build op: %+v", result.Operations)
	}

	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"1": 1, "2": 0}},
		},
	})
	result = BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 2 || op.GoldCost != 10095 || !op.Executable || op.SyncOnly {
				t.Fatalf("gold union build op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildDiamondBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"41": 200}},
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"3": 0}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.DiamondEnabled = true
	p.Union.Build.MaxSpendDiamond = 200

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 3 || op.DiamondCost != 106 || op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("diamond union build op should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildDiamondUsesVisibleBalanceOnly(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"41": 50, "42": 1000}},
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"3": 0}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.DiamondEnabled = true
	p.Union.Build.MaxSpendDiamond = 200

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain != "union.build" {
			continue
		}
		if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 3 || op.DiamondCost != 106 || op.Executable {
			t.Fatalf("diamond union build op mismatch: %+v", op)
		}
		if !hasReasonContaining(op.BlockedReasons, "元宝不足") {
			t.Fatalf("blocked reasons = %v, want 元宝不足", op.BlockedReasons)
		}
		for _, gate := range op.CostGates {
			if gate.ResourceKind == GateResourceDiamond {
				if gate.Available != 50 {
					t.Fatalf("diamond gate available = %d, want 50", gate.Available)
				}
				return
			}
		}
		t.Fatalf("missing diamond cost gate: %+v", op)
	}
	t.Fatalf("missing blocked union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildRequiresObservedCounts(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{"0": map[string]any{"0": 88}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.FreeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("union build without count map should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandHarvest(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"1": 23005, "3": 6, "4": 2},
					"2": map[string]any{"1": 23007, "3": 4, "4": 4},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.harvest" {
			if op.Kind != clientproto.RPCFmlLandHarvest.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union land harvest op mismatch: %+v", op)
			}
			if len(op.LandIDs) != 1 || op.LandIDs[0] != 1 || op.Count != 1 {
				t.Fatalf("union land harvest ids/count mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land harvest op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandHarvestRequiresObservedState(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.harvest" {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("unobserved union land should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked union land harvest op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerShareReward(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{
					"1": map[string]any{"0": 23005, "1": 10, "2": 3},
					"2": map[string]any{"0": 23007, "1": 10, "2": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.ShareEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.flower.reward" {
			if op.Kind != clientproto.RPCFmlFlowerShareRecvRwd.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union flower reward op mismatch: %+v", op)
			}
			if len(op.SlotIDs) != 1 || op.SlotIDs[0] != 1 || op.Count != 1 {
				t.Fatalf("union flower reward slot ids/count mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union flower reward op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerTakeSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			if op.Kind != clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union flower take sync op mismatch: %+v", op)
			}
			return
		}
		if op.Domain == "union.unknown" {
			t.Fatalf("take should not be folded into union.unknown: %+v", op)
		}
	}
	t.Fatalf("missing union flower take sync op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerTakeSpecificFlower(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"108": []any{
				map[string]any{
					"0": 77900091102483,
					"1": map[string]any{
						"1": map[string]any{"0": 23009, "1": 8, "2": 7},
					},
				},
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 6, "2": 1},
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	p.Union.Flower.TakeFlowerIds = []int32{23011}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			if op.Kind != clientproto.RPCFmlFlowerShareTake.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union flower take op mismatch: %+v", op)
			}
			if op.TargetUID != 77900091102484 || op.TargetID != 2 || op.FlowerID != 23011 || op.Count != 1 {
				t.Fatalf("union flower take target mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union flower take op: %+v", result.Operations)
}

func TestBuildPlan_UnionForestSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.ForestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.forest" {
			if op.Kind != clientproto.RPCFmlForestRefresh.String() || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("union forest sync op mismatch: %+v", op)
			}
			return
		}
		if op.Domain == "union.unknown" {
			t.Fatalf("forest should not be folded into union.unknown: %+v", op)
		}
	}
	t.Fatalf("missing union forest sync op: %+v", result.Operations)
}

func TestBuildPlan_UnionForestCollect(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"127": map[string]any{
				"1": 88,
				"8": map[string]any{
					"88": map[string]any{"1": 5},
					"99": map[string]any{"1": 4, "3": 2},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.ForestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.forest" {
			if op.Kind != clientproto.RPCFmlForestRefresh.String() || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("union forest collect op mismatch: %+v", op)
			}
			if op.Count != 11 {
				t.Fatalf("union forest collect count=%d, want 11: %+v", op.Count, op)
			}
			return
		}
	}
	t.Fatalf("missing union forest collect op: %+v", result.Operations)
}

func TestBuildPlan_LowStockFallbackBalancesMultipleFlowers(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 0,
			"23002": 1,
			"23003": 5,
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(6)}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true

	result := BuildPlan(s, p, time.Now())
	countByFlower := map[int32]int32{}
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" {
			countByFlower[op.FlowerID] += int32(len(op.LandIDs))
		}
	}
	if countByFlower[23001] == 0 || countByFlower[23002] == 0 {
		t.Fatalf("fallback should split across low-stock flowers, got %v ops=%+v", countByFlower, result.Operations)
	}
}

func TestBuildPlan_LowStockFallbackUsesAllEmptyLand(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 0,
			"23002": 2,
			"23003": 5,
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(6)}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true

	result := BuildPlan(s, p, time.Now())
	countByFlower := map[int32]int32{}
	var total int32
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" {
			count := int32(len(op.LandIDs))
			countByFlower[op.FlowerID] += count
			total += count
		}
	}
	if total != 6 {
		t.Fatalf("fallback should use all empty land, got total=%d counts=%v ops=%+v", total, countByFlower, result.Operations)
	}
	if countByFlower[23001] == 0 || countByFlower[23002] == 0 {
		t.Fatalf("fallback should prefer low-stock flowers, got %v", countByFlower)
	}
}

func TestBuildPlan_AutoReplantSpecificFlowersRestrictsOnlyFallback(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 0,
			"23002": 0,
			"23003": 0,
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(4)}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoReplantMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	p.Plant.Planting.AutoReplantFlowerIds = []int32{23002}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.FlowerID != 23002 {
			t.Fatalf("specific planting should only use selected flowers, op=%+v ops=%+v", op, result.Operations)
		}
	}
}

func TestPlanPlantAssignments_BlockedDemandDoesNotConsumeFallback(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": cultivate(23002)},
	})
	p := DefaultPolicy()

	plan := planPlantAssignments(s, p.Plant, []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  1,
		Priority: 90,
		Label:    "顾客订单",
	}}, 2)
	if len(plan.blockedDiagnostic) != 1 {
		t.Fatalf("blocked diagnostics len=%d, want 1: %+v", len(plan.blockedDiagnostic), plan)
	}
	blocked := plan.blockedDiagnostic[0]
	if blocked.FlowerID != 23001 || blocked.Count != 0 || blocked.Priority != blockedPlantDiagnosticPriority ||
		blocked.GoalID != GoalCustomerOrder || blocked.DemandID != "demand-23001" || blocked.Reason != blockedPlantDiagnosticReason {
		t.Fatalf("blocked diagnostic mismatch: %+v", blocked)
	}
	if len(plan.executable) == 0 {
		t.Fatalf("fallback auto-replant should still be executable: %+v", plan)
	}
	for _, assignment := range plan.executable {
		if assignment.GoalID != GoalAutoReplant || assignment.FlowerID != 23002 || assignment.Count <= 0 {
			t.Fatalf("blocked demand should not consume fallback slots, assignment=%+v plan=%+v", assignment, plan)
		}
	}
}

func TestFarmOps_BlockedDemandEmitsDiagnosticPlantOperation(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": emptyLands(2)}},
		"101": map[string]any{"0": cultivate(23002)},
	})
	p := DefaultPolicy()
	demands := []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  1,
		Priority: 90,
		Label:    "顾客订单",
	}}

	ops := farmOps(s, p.Plant, demands, time.Now())
	var blocked *PlannedOp
	var fallback *PlannedOp
	for i := range ops {
		op := &ops[i]
		switch {
		case op.Domain == "farm.plant" && op.FlowerID == 23001:
			blocked = op
		case op.Domain == "farm.plant" && op.FlowerID == 23002 && op.Executable:
			fallback = op
		}
	}
	if fallback == nil {
		t.Fatalf("blocked diagnostic should not prevent fallback planting, ops=%+v", ops)
	}
	if blocked == nil {
		t.Fatalf("expected blocked plant diagnostic op, ops=%+v", ops)
	}
	if blocked.Kind != clientproto.RPCUsrLandPlant.String() || blocked.Executable || blocked.Status != PlanStatusBlocked ||
		blocked.BlockingStage != "state" || blocked.Priority != blockedPlantDiagnosticPriority ||
		blocked.GoalID != GoalCustomerOrder || blocked.DemandID != "demand-23001" || blocked.FlowerID != 23001 ||
		len(blocked.LandIDs) != 0 || blocked.Reason != blockedPlantDiagnosticReason ||
		!hasReasonContaining(blocked.BlockedReasons, blockedPlantDiagnosticReason) {
		t.Fatalf("blocked plant diagnostic mismatch: %+v", *blocked)
	}
}

func TestPlantAssignments_AutoReplantRangeDoesNotRestrictDemand(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": cultivate(23001, 23002)},
	})
	p := DefaultPolicy()
	p.Plant.Planting.AutoReplantMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	p.Plant.Planting.AutoReplantFlowerIds = []int32{23002}

	assignments := plantAssignments(s, p.Plant, []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  6,
		Priority: 90,
		Label:    "顾客订单",
	}}, 3)
	if len(assignments) != 1 {
		t.Fatalf("assignments len=%d, want 1: %+v", len(assignments), assignments)
	}
	if assignments[0].FlowerID != 23001 || assignments[0].Count != 3 || assignments[0].GoalID != GoalCustomerOrder {
		t.Fatalf("task demand should bypass specific planting range, assignments=%+v", assignments)
	}
}

func TestPlantAssignments_TaskDemandFillsAvailableEmptyLand(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": cultivate(23001)},
	})
	p := DefaultPolicy()

	assignments := plantAssignments(s, p.Plant, []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  6,
		Priority: 90,
		Label:    "顾客订单",
	}}, 6)
	if len(assignments) != 1 || assignments[0].FlowerID != 23001 || assignments[0].Count != 6 {
		t.Fatalf("task demand should fill available empty land, got %+v", assignments)
	}
}

func TestNextLandUnlockCandidateDoesNotInventNextFourLands(t *testing.T) {
	s := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[itoa32(id)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 999999}},
	})

	if id, _, ok := nextLandUnlockCandidate(s); ok {
		t.Fatalf("nextLandUnlockCandidate()=(%d,true), want no guessed candidate", id)
	}
}

func TestNextLandUnlockCandidateUsesRuntimeLandConfig(t *testing.T) {
	s := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[itoa32(id)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 1500}},
	})
	s.SetFarmLands([]state.FarmLandInfo{{ID: 1025, OpenLevel: 13, Cost: []int32{37, 1500}}})

	id, cost, ok := nextLandUnlockCandidate(s)
	if !ok || id != 1025 {
		t.Fatalf("nextLandUnlockCandidate()=(%d,%t), want (1025,true)", id, ok)
	}
	if cost != 1474 {
		t.Fatalf("nextLandUnlockCandidate cost=%d, want 1474", cost)
	}
}
