package automation

import (
	"encoding/json"
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
	// Domain tests that provide guild namespace 25 exercise an account that is
	// already a member unless they explicitly provide the authoritative 25.1
	// membership record. No-guild/unknown-membership gates have dedicated
	// fixtures and must not be inferred from unrelated guild fragments.
	if guild, ok := top["25"].(map[string]any); ok {
		if _, explicit := guild["1"]; !explicit {
			guild["1"] = map[string]any{"0": int64(999), "1": 88}
		}
	}
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

func cultivateAtLevel(level int32, flowers ...int32) map[string]any {
	out := make(map[string]any, len(flowers))
	for _, id := range flowers {
		out[itoa32(id)] = map[string]any{"1": id, "2": level, "4": 2}
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

	if kind, reason := Recommend(land, now, 0); kind != KindWait {
		t.Fatalf("state=2 should wait inside harvest grace, got kind=%s reason=%s", kind, reason)
	}

	land.NextTimeMs = now.Add(-harvestReadyGrace - time.Second).UnixMilli()
	if kind, reason := Recommend(land, now, 0); kind != KindHarvest {
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

	if kind, reason := Recommend(land, now, 0); kind != KindHarvest {
		t.Fatalf("state=3 should harvest immediately, got kind=%s reason=%s", kind, reason)
	}
}

func TestRecommend_HarvestDelayHoldsState3(t *testing.T) {
	now := time.Date(2026, 7, 3, 15, 3, 59, 0, time.UTC)
	delay := 30 * time.Second
	land := state.LandView{
		Observed:    true,
		FlowerID:    23001,
		State:       3,
		PlantTimeMs: now.Add(-10 * time.Second).UnixMilli(),
	}
	if kind, reason := Recommend(land, now, delay); kind != KindWait {
		t.Fatalf("state=3 should wait inside harvest delay, got kind=%s reason=%s", kind, reason)
	}
	land.PlantTimeMs = now.Add(-delay - time.Second).UnixMilli()
	if kind, reason := Recommend(land, now, delay); kind != KindHarvest {
		t.Fatalf("state=3 should harvest after delay, got kind=%s reason=%s", kind, reason)
	}
}

func TestRecommend_HarvestDelayOverridesState2Grace(t *testing.T) {
	now := time.Date(2026, 7, 3, 15, 3, 59, 0, time.UTC)
	delay := 30 * time.Second
	land := state.LandView{
		Observed:   true,
		FlowerID:   23001,
		State:      2,
		NextTimeMs: now.Add(-10 * time.Second).UnixMilli(),
	}
	if kind, reason := Recommend(land, now, delay); kind != KindWait {
		t.Fatalf("state=2 should wait for configured delay beyond grace, got kind=%s reason=%s", kind, reason)
	}
	land.NextTimeMs = now.Add(-delay - time.Second).UnixMilli()
	if kind, reason := Recommend(land, now, delay); kind != KindHarvest {
		t.Fatalf("state=2 should harvest after configured delay, got kind=%s reason=%s", kind, reason)
	}
}

func TestBuildPlan_HarvestRespectsDelay(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	s := state.New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3, "7": now.Add(-10 * time.Second).UnixMilli()},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoHarvestEnabled = true
	p.Plant.Planting.HarvestDelaySeconds = 30

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandHarvest.String() {
			t.Fatalf("harvest should wait for delay, got %+v", op)
		}
	}

	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3, "7": now.Add(-31 * time.Second).UnixMilli()},
		}},
	})
	result = BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandHarvest.String() {
			return
		}
	}
	t.Fatalf("missing delayed harvest op: %+v", result.Operations)
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

func TestBuildPlan_ResidentVideoOrderDoesNotDemandOrSubmit(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{
			"0": 801, "2": [][]int32{{23005, 1}}, "3": 1, "7": map[string]any{"1": 8},
		}}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, demand := range result.Demands {
		if demand.GoalID == GoalResidentOrder {
			t.Fatalf("video resident order must not create planting demand: %+v", demand)
		}
	}
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() {
			t.Fatalf("video resident order must not be auto-submitted: %+v", op)
		}
	}
}

func TestBuildPlan_ResidentNormalLimitBlocksSubmit(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "9": 5}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 5

	result := BuildPlan(s, p, now)
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
	for _, demand := range result.Demands {
		if demand.GoalID == GoalResidentOrder {
			t.Fatalf("resident demand should stop after daily limit: %+v", demand)
		}
	}
}

func TestBuildPlan_ResidentNormalLimitSurvivesSparseStatisticsDelta(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "9": 5}}},
	})
	// Sparse unrelated counter update previously wiped finish count to 0.
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"8": 1}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 5

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should stay blocked after sparse stats delta: %+v", op)
		}
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

func TestBuildPlan_ResidentSatinDecorateServerDailyLimitMarkerBlocksSubmit(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23001": 5, "23003": 5}}},
		"105": map[string]any{"0": map[string]any{
			"6": map[string]any{"0": []any{[]any{23001, 2}}, "1": 201, "3": 1},
			"7": map[string]any{"0": []any{[]any{23003, 2}}, "1": 202, "3": 1},
		}},
		"124": map[string]any{"0": map[string]any{"20260705": map[string]any{"1": 20260705, "14": 0, "16": 0}}},
	})
	s.MarkResidentSatinDailyLimitReached(now)
	s.MarkResidentDecorateDailyLimitReached(now)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = false
	p.Order.Resident.SatinEnabled = true
	p.Order.Resident.SatinDailyLimit = 120
	p.Order.Resident.DecorateEnabled = true
	p.Order.Resident.DecorateDailyLimit = 120

	result := BuildPlan(s, p, now)
	var satinBlocked, decorateBlocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() && op.Executable {
			t.Fatalf("satin submit should be blocked after server daily limit: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String() && op.Executable {
			t.Fatalf("decorate submit should be blocked after server daily limit: %+v", op)
		}
		if op.Domain == "order.resident.satin" && !op.Executable && hasReasonContaining(op.BlockedReasons, "今日完成订单次数已达上限") {
			satinBlocked = true
		}
		if op.Domain == "order.resident.decorate" && !op.Executable && hasReasonContaining(op.BlockedReasons, "今日完成订单次数已达上限") {
			decorateBlocked = true
		}
	}
	if !satinBlocked || !decorateBlocked {
		t.Fatalf("missing satin/decorate server limit blocks (satin=%v decorate=%v): %+v", satinBlocked, decorateBlocked, result.Operations)
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

func TestBuildPlan_ResidentLocalFinishBiasEnforcesLimitWithoutStatistics(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 2

	s.NoteResidentOrderFinished(now, nil)
	s.NoteResidentOrderFinished(now, nil)

	result := BuildPlan(s, p, now)
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should be blocked by local finish bias: %+v", op)
		}
		if op.Domain == "order.resident" && !op.Executable && hasReasonContaining(op.BlockedReasons, "2/2") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing local bias limit block: %+v", result.Operations)
	}
}

func TestBuildPlan_ResidentFinishBiasClearedWhenStatisticsAdvance(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	s.NoteResidentOrderFinished(now, nil)
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "9": 1}}},
	})
	if got := s.ResidentOrderFinishNum(now); got != 1 {
		t.Fatalf("ResidentOrderFinishNum=%d, want 1 after authoritative stats clear bias", got)
	}
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 2

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			return
		}
	}
	t.Fatalf("expected resident submit under limit after stats reconcile: %+v", result.Operations)
}

func TestResidentOrderFinishCountsMergeBiasAndResetByGameDay(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, location)
	s := state.New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{
			"1": 20260724, "9": 1, "14": 1, "16": 1,
		}}},
	})

	s.NoteResidentOrderFinished(now, nil)
	s.NoteResidentSatinOrderFinished(now, nil)
	s.NoteResidentDecorateOrderFinished(now, nil)
	if got := s.ResidentOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentOrderFinishNum=%d, want observed 1 + local 1", got)
	}
	if got := s.ResidentSatinOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentSatinOrderFinishNum=%d, want observed 1 + local 1", got)
	}
	if got := s.ResidentDecorateOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentDecorateOrderFinishNum=%d, want observed 1 + local 1", got)
	}
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"8": 4}}},
	})
	if got := s.ResidentSatinOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentSatinOrderFinishNum after sparse stats=%d, want 2", got)
	}
	if got := s.ResidentDecorateOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentDecorateOrderFinishNum after sparse stats=%d, want 2", got)
	}

	policy := DefaultPolicy().GetOrder().GetResident()
	policy.SatinDailyLimit = 2
	policy.DecorateDailyLimit = 2
	if reason, reached := residentSatinDailyLimitReached(s, policy, now); !reached || !strings.Contains(reason, "2/2") {
		t.Fatalf("satin limit=(%q,%t), want 2/2 reached", reason, reached)
	}
	if reason, reached := residentDecorateDailyLimitReached(s, policy, now); !reached || !strings.Contains(reason, "2/2") {
		t.Fatalf("decorate limit=(%q,%t), want 2/2 reached", reason, reached)
	}

	nextDay := now.Add(24 * time.Hour)
	if got := s.ResidentOrderFinishNum(nextDay); got != 0 {
		t.Fatalf("ResidentOrderFinishNum next day=%d, want 0", got)
	}
	if got := s.ResidentSatinOrderFinishNum(nextDay); got != 0 {
		t.Fatalf("ResidentSatinOrderFinishNum next day=%d, want 0", got)
	}
	if got := s.ResidentDecorateOrderFinishNum(nextDay); got != 0 {
		t.Fatalf("ResidentDecorateOrderFinishNum next day=%d, want 0", got)
	}
}

func TestResidentSpecialFinishResponseCounterPreventsDoubleCount(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	raw := json.RawMessage(`{"124":{"0":{"20260724":{"1":20260724,"14":2,"16":3}}}}`)
	s.ApplyV(raw)
	s.NoteResidentSatinOrderFinished(now, raw)
	s.NoteResidentDecorateOrderFinished(now, raw)
	if got := s.ResidentSatinOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentSatinOrderFinishNum=%d, want authoritative 2", got)
	}
	if got := s.ResidentDecorateOrderFinishNum(now); got != 3 {
		t.Fatalf("ResidentDecorateOrderFinishNum=%d, want authoritative 3", got)
	}
}

func TestBuildPlan_ResidentLaggingStatisticsBiasStillEnforcesLimit(t *testing.T) {
	// Regression: once namespace 124 is observed, finish responses that omit
	// field 9 must still advance the local bias so NormalDailyLimit trips.
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "9": 1}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 3

	s.NoteResidentOrderFinished(now, nil)
	s.NoteResidentOrderFinished(now, nil)
	if got := s.ResidentOrderFinishNum(now); got != 3 {
		t.Fatalf("ResidentOrderFinishNum=%d, want 1(stats)+2(bias)", got)
	}

	result := BuildPlan(s, p, now)
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should be blocked by lagging-stats bias: %+v", op)
		}
		if op.Domain == "order.resident" && !op.Executable && hasReasonContaining(op.BlockedReasons, "3/3") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing lagging-stats limit block: %+v", result.Operations)
	}
}

func TestBuildPlan_ResidentPriorDayStatisticsBiasEnforcesLimitAfterRollover(t *testing.T) {
	// Regression: after 00:05 the prior game-day snapshot must not report
	// finished=0 forever; local bias for the new day still enforces the cap.
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 25, 0, 10, 0, 0, loc)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 2}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}}}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "9": 80}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = true
	p.Order.Resident.NormalDailyLimit = 2

	s.NoteResidentOrderFinished(now, nil)
	s.NoteResidentOrderFinished(now, nil)
	if got := s.ResidentOrderFinishNum(now); got != 2 {
		t.Fatalf("ResidentOrderFinishNum=%d, want 2 from new-day bias (prior DayID ignored)", got)
	}

	result := BuildPlan(s, p, now)
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.Executable {
			t.Fatalf("resident submit should be blocked after game-day rollover bias: %+v", op)
		}
		if op.Domain == "order.resident" && !op.Executable && hasReasonContaining(op.BlockedReasons, "2/2") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing post-rollover limit block: %+v", result.Operations)
	}
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

func TestBuildPlan_ResidentSatinDecorateZeroLimitDefaultsAndCooldown(t *testing.T) {
	now := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23001": 5, "23003": 5}}},
		"105": map[string]any{"0": map[string]any{
			"6": map[string]any{
				"0": []any{[]any{23001, 2}}, "1": 201, "3": 1,
				"6": now.Add(61 * time.Second).UnixMilli(),
			},
			"7": map[string]any{
				"0": []any{[]any{23003, 2}}, "1": 202, "3": 1,
			},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Resident.NormalEnabled = false
	p.Order.Resident.SatinEnabled = true
	p.Order.Resident.SatinDailyLimit = 0 // UI used to show 120 while policy stayed 0
	p.Order.Resident.DecorateEnabled = true
	p.Order.Resident.DecorateDailyLimit = 0

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() && op.Executable {
			t.Fatalf("satin submit should wait ~61s cooldown: %+v", op)
		}
		if strings.Contains(strings.Join(op.BlockedReasons, " "), "上限必须大于 0") {
			t.Fatalf("zero policy limit should default instead of blocking: %+v", op)
		}
	}
	var decorateReady bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String() && op.Executable {
			decorateReady = true
		}
	}
	if !decorateReady {
		t.Fatalf("decorate submit missing with zero policy limit: %+v", result.Operations)
	}

	result = BuildPlan(s, p, now.Add(61*time.Second))
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() && op.Executable {
			return
		}
	}
	t.Fatalf("missing satin submit after 61s cooldown: %+v", result.Operations)
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
	p.Plant.Planting.DemandPriorityEnabled = true

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
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && !op.Executable {
			if !hasReasonContaining(op.BlockedReasons, "reject_unavailable_enabled 未开启") || !hasReasonContaining(op.BlockedReasons, "库存不足且无法制作") {
				t.Fatalf("expected inventory-shortage block, ops=%+v", op)
			}
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
			if !op.Executable || op.TargetID != 7 || !strings.Contains(op.Reason, "库存不足且无法制作") {
				t.Fatalf("reject op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer reject op: %+v", result.Operations)
}

func TestBuildPlan_CustomerMissingRecipeRejectsWhenEnabled(t *testing.T) {
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
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if !op.Executable || !strings.Contains(op.Reason, "库存不足且无法制作") {
				t.Fatalf("missing recipe should reject when enabled: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer reject op: %+v", result.Operations)
}

func TestBuildPlan_CustomerArtCraftsFromInventoryWithoutCultivation(t *testing.T) {
	recipe, ok := state.FlowerArtRecipeByID(300208)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300208) ok=false")
	}
	stock := make(map[string]any, len(recipe.Flowers))
	for _, flowerID := range recipe.Flowers {
		stock[itoa32(flowerID)] = int32(1)
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": stock,
			"34": 12,
		}},
		"102": map[string]any{"0": map[string]any{itoa32(recipe.VaseID): map[string]any{"1": recipe.VaseID}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			t.Fatalf("customer order should craft from inventory instead of rejecting: %+v", op)
		}
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && op.ItemID == recipe.ArtID {
			if !op.Executable || op.Count != 1 || op.VaseID != recipe.VaseID || op.Priority != orderSchedulePriority(orderStageCustomerCraft) {
				t.Fatalf("customer craft op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer craft op: %+v", result.Operations)
}

func TestBuildPlan_CustomerFinishWhenArtInStock(t *testing.T) {
	recipe, ok := state.FlowerArtRecipeByID(300505)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300505) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{itoa32(recipe.ArtID): 1},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			t.Fatalf("should finish from stock before crafting: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			t.Fatalf("should finish from stock before rejecting: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() {
			if !op.Executable || op.TargetID != 7 {
				t.Fatalf("finish op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer finish op: %+v", result.Operations)
}

func TestBuildPlan_CustomerDailyLimitBlocksFinishAndGenerate(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	recipe, ok := state.FlowerArtRecipeByID(300505)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300505) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{itoa32(recipe.ArtID): 1},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "11": 2}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.DailyLimit = 2

	result := BuildPlan(s, p, now)
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
			t.Fatalf("customer finish should be blocked by daily limit: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerGenOrder.String() && op.Executable {
			t.Fatalf("customer generate should be blocked by daily limit: %+v", op)
		}
		if op.Domain == "order.customer" && !op.Executable && hasReasonContaining(op.BlockedReasons, "2/2") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing customer daily limit block: %+v", result.Operations)
	}
	for _, demand := range result.Demands {
		if demand.GoalID == GoalCustomerOrder {
			t.Fatalf("customer demands should be omitted at daily limit: %+v", demand)
		}
	}
}

func TestBuildPlan_CustomerLocalFinishBiasEnforcesLimitWithoutStatistics(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	recipe, ok := state.FlowerArtRecipeByID(300505)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300505) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{itoa32(recipe.ArtID): 1},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.DailyLimit = 2

	s.NoteCustomerOrderFinished(now, nil)
	s.NoteCustomerOrderFinished(now, nil)

	result := BuildPlan(s, p, now)
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
			t.Fatalf("customer finish should be blocked by local finish bias: %+v", op)
		}
		if op.Domain == "order.customer" && !op.Executable && hasReasonContaining(op.BlockedReasons, "2/2") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("missing local bias limit block: %+v", result.Operations)
	}
}

func TestBuildPlan_CustomerDailyLimitZeroMeansUnlimited(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	recipe, ok := state.FlowerArtRecipeByID(300505)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300505) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{itoa32(recipe.ArtID): 1},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
		"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "11": 99}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.DailyLimit = 0

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
			return
		}
	}
	t.Fatalf("expected customer finish when daily limit is 0 (unlimited): %+v", result.Operations)
}

func TestBuildPlan_CustomerMinFlowerArtRejectsBelowThreshold(t *testing.T) {
	recipe2, ok := state.FlowerArtRecipeByID(300207)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300207) ok=false")
	}
	recipe3, ok := state.FlowerArtRecipeByID(300208)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300208) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{
				itoa32(recipe2.ArtID): 2,
				itoa32(recipe3.ArtID): 3,
			},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"1": map[string]any{"0": 2, "1": recipe2.ArtID, "2": 2, "3": 1},
			"3": map[string]any{"0": 2, "1": recipe3.ArtID, "2": 3, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.MinFlowerArtCount = 3

	result := BuildPlan(s, p, time.Now())
	var rejectedBelow, finishedOK bool
	for _, op := range result.Operations {
		switch {
		case op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && op.Executable && op.TargetID == 1:
			if !strings.Contains(op.Reason, "花艺件数 2 < 最低要求 3") {
				t.Fatalf("reject reason mismatch: %+v", op)
			}
			rejectedBelow = true
		case op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable && op.TargetID == 3:
			finishedOK = true
		case op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable && op.TargetID == 1:
			t.Fatalf("should not finish below-threshold order: %+v", op)
		}
	}
	if !rejectedBelow || !finishedOK {
		t.Fatalf("want reject npc=1 and finish npc=3, ops=%+v", result.Operations)
	}
	for _, demand := range result.Demands {
		if demand.GoalID == GoalCustomerOrder && demand.EntityID == "1" {
			t.Fatalf("below-threshold order must not create demands: %+v", demand)
		}
	}
}

func TestBuildPlan_CustomerMinFlowerArtTwoAcceptsTwoAndThree(t *testing.T) {
	recipe2, ok := state.FlowerArtRecipeByID(300207)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300207) ok=false")
	}
	recipe3, ok := state.FlowerArtRecipeByID(300208)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300208) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{
				itoa32(recipe2.ArtID): 2,
				itoa32(recipe3.ArtID): 3,
			},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"1": map[string]any{"0": 2, "1": recipe2.ArtID, "2": 2, "3": 1},
			"3": map[string]any{"0": 2, "1": recipe3.ArtID, "2": 3, "3": 1},
			"5": map[string]any{"0": 2, "1": recipe2.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.MinFlowerArtCount = 2

	result := BuildPlan(s, p, time.Now())
	var finishedTwo, finishedThree, rejectedOne bool
	for _, op := range result.Operations {
		switch {
		case op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable && op.TargetID == 1:
			finishedTwo = true
		case op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable && op.TargetID == 3:
			finishedThree = true
		case op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && op.Executable && op.TargetID == 5:
			rejectedOne = true
		case op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable && op.TargetID == 5:
			t.Fatalf("should not finish 1-art order when min=2: %+v", op)
		}
	}
	if !finishedTwo || !finishedThree || !rejectedOne {
		t.Fatalf("want finish 2+3 and reject 1, ops=%+v", result.Operations)
	}
}

func TestBuildPlan_CustomerMinFlowerArtBypassedWhenRaceHoldsCustomerTask(t *testing.T) {
	recipe2, ok := state.FlowerArtRecipeByID(300207)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300207) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0": 999,
			"32": map[string]any{
				itoa32(recipe2.ArtID): 2,
			},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"1": map[string]any{"0": 2, "1": recipe2.ArtID, "2": 2, "3": 1},
		}}},
		// Unfinished customer-order race task (type 3016 via catalog 3019).
		"25": map[string]any{
			"111": map[string]any{"1": 1},
			"117": map[string]any{"5": 4},
			"110": map[string]any{"999": map[string]any{"7": map[string]any{"0": 71, "1": 3019, "2": 5, "3": 0}}},
			"114": []any{map[string]any{"0": 71, "4": 3019, "7": 5, "8": 0, "10": 24, "12": 999}},
		},
	})
	if !RaceHoldsUnfinishedCustomerOrder(s.FmlRace()) {
		t.Fatalf("expected unfinished customer race task, got %+v", s.FmlRace().Taken)
	}
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.MinFlowerArtCount = 3

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && op.Executable && op.TargetID == 1 {
			t.Fatalf("race-held customer task must bypass min flower-art reject: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable && op.TargetID == 1 {
			return
		}
	}
	t.Fatalf("expected finish of below-threshold order while race holds customer task: %+v", result.Operations)
}

func TestBuildPlan_CustomerCraftsAfterStaleArtStockCleared(t *testing.T) {
	recipe, ok := state.FlowerArtRecipeByID(300505)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300505) ok=false")
	}
	stock := make(map[string]any, len(recipe.Flowers)+1)
	stock[itoa32(recipe.ArtID)] = int32(1)
	for _, flowerID := range recipe.Flowers {
		stock[itoa32(flowerID)] = int32(1)
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": stock,
			"34": 12,
		}},
		"102": map[string]any{"0": map[string]any{itoa32(recipe.VaseID): map[string]any{"1": recipe.VaseID}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	before := BuildPlan(s, p, time.Now())
	for _, op := range before.Operations {
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
			goto cleared
		}
	}
	t.Fatalf("expected finish while stale art stock present: %+v", before.Operations)

cleared:
	s.MarkInventoryItemExhausted(recipe.ArtID)
	after := BuildPlan(s, p, time.Now())
	for _, op := range after.Operations {
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
			t.Fatalf("should not finish after stale art cleared: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && op.Executable {
			t.Fatalf("should craft materials instead of rejecting: %+v", op)
		}
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && op.ItemID == recipe.ArtID {
			if !op.Executable || op.Count != 1 {
				t.Fatalf("craft op mismatch after stock clear: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing craft after stale art cleared: %+v", after.Operations)
}

func TestBuildPlan_CustomerRejectsWhenArtMissingAndUncraftable(t *testing.T) {
	recipe, ok := state.FlowerArtRecipeByID(300505)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300505) ok=false")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{},
			"34": 12,
		}},
		"102": map[string]any{"0": map[string]any{itoa32(recipe.VaseID): map[string]any{"1": recipe.VaseID}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": recipe.ArtID, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
			t.Fatalf("should not finish without art stock: %+v", op)
		}
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && op.Executable {
			t.Fatalf("should not craft without flower materials: %+v", op)
		}
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if !op.Executable || !strings.Contains(op.Reason, "库存不足且无法制作") {
				t.Fatalf("reject op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing reject when uncraftable: %+v", result.Operations)
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

func TestBuildPlan_CustomerFlowerOrderBlockedWhenNoInventoryAndSwitchOff(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 23005, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = false

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if op.Executable {
				t.Fatalf("should not execute reject when switch off: %+v", op)
			}
			if !hasReasonContaining(op.BlockedReasons, "reject_unavailable_enabled 未开启") || !hasReasonContaining(op.BlockedReasons, "库存不足且无法制作") {
				t.Fatalf("expected reject_unavailable_enabled block: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer blocked op: %+v", result.Operations)
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
	p.Union.Race.Enabled = false

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
	p.Union.Race.Enabled = false

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
			if op.ItemID != 300208 || op.Count != 12 || op.Priority != orderSchedulePriority(orderStageFlowerRackSell) {
				t.Fatalf("fixed rack sell mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing rack sell op: %+v", result.Operations)
}

func TestBuildPlan_FlowerRackNightPauseSkipsListing(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
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
	p.Order.FlowerArt.SellNightPauseEnabled = true

	night := time.Date(2026, 7, 24, 3, 30, 0, 0, shanghai)
	for _, op := range BuildPlan(s, p, night).Operations {
		if op.Kind == clientproto.RPCFlowerRackSell.String() || op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			t.Fatalf("night pause should skip listing/craft-for-list: %+v", op)
		}
	}

	day := time.Date(2026, 7, 24, 8, 0, 0, 0, shanghai)
	found := false
	for _, op := range BuildPlan(s, p, day).Operations {
		if op.Kind == clientproto.RPCFlowerRackSell.String() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("listing should resume at 08:00")
	}
}

func TestBuildPlan_FlowerRackNightPauseKeepsClaim(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 24, 2, 0, 0, 0, shanghai)
	listedAt := now.Add(-time.Duration(state.FlowerRackSellDurationMs()) * time.Millisecond).UnixMilli()
	s := state.New()
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"2": map[string]any{"1": 2, "2": 300208, "3": 1, "4": listedAt, "5": listedAt},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = true
	p.Order.FlowerArt.SellNightPauseEnabled = true

	for _, op := range BuildPlan(s, p, now).Operations {
		if op.Kind == clientproto.RPCFlowerRackRecvSellMoney.String() {
			if !op.Executable || op.TargetID != 2 {
				t.Fatalf("recvSellMoney op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("night pause must still allow claiming rack proceeds")
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
			if op.ItemID != 300208 || op.Count != 4 || op.VaseID != 3002 || !op.Executable || op.Priority != orderSchedulePriority(orderStageFlowerRackCraft) {
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

func TestMaxCraftableCountKeepsZeroAsMinimum(t *testing.T) {
	recipe, ok := state.FlowerArtRecipeByID(300208)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300208) ok=false")
	}
	stock := map[int32]int32{}
	for _, flowerID := range recipe.Flowers {
		stock[flowerID] = 3
	}
	stock[recipe.Flowers[0]] = 0

	for i := 0; i < 20; i++ {
		if got := maxCraftableCount(recipe, NewInventoryLedger(stock)); got != 0 {
			t.Fatalf("maxCraftableCount()=%d, want 0", got)
		}
	}
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
			if !op.Executable || op.TargetID != 2 || op.Priority != orderSchedulePriority(orderStageFlowerRackClaim) {
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
	p.Union.Race.Enabled = false

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

func TestBuildPlan_WaterClaimsRespectThresholdNotRegenCapacity(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	cases := []struct {
		name           string
		drops          int32
		total          int32
		threshold      int32
		wantFreeWater  bool
		wantWaterwheel bool
	}{
		{name: "below threshold", drops: 12, total: 130, wantFreeWater: true, wantWaterwheel: true},
		{name: "at regen capacity still claims", drops: 130, total: 130, wantFreeWater: true, wantWaterwheel: true},
		{name: "above regen capacity still claims", drops: 200, total: 130, wantFreeWater: true, wantWaterwheel: true},
		{name: "above threshold", drops: 80, total: 130, threshold: 50, wantFreeWater: false, wantWaterwheel: false},
		{name: "below threshold with capacity full", drops: 40, total: 130, threshold: 50, wantFreeWater: true, wantWaterwheel: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{
				"7": map[string]any{"0": map[string]any{
					"32": map[string]any{"7": tt.drops},
					"33": map[string]any{"7": map[string]any{"1": tt.total}},
				}},
				"114": map[string]any{
					"1": 1,
					// Accrue local buckets via uTime catch-up after enter.
					"4": now.Add(-time.Hour).UnixMilli(),
				},
				"117": map[string]any{
					"1": []int32{},
					"2": now.UnixMilli(),
				},
			})
			s.MarkWaterwheelEntered(now)
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
				if op.Kind == clientproto.RPCFreeWaterRecv.String() {
					gotFreeWater = true
					if op.TargetID != 0 {
						t.Fatalf("free water target = %d, want first slot idx 0; op=%+v", op.TargetID, op)
					}
				}
			}
			if gotWaterwheel != tt.wantWaterwheel {
				t.Fatalf("waterwheel claim = %t, want %t; ops=%+v", gotWaterwheel, tt.wantWaterwheel, result.Operations)
			}
			if gotFreeWater != tt.wantFreeWater {
				t.Fatalf("free water claim = %t, want %t; ops=%+v", gotFreeWater, tt.wantFreeWater, result.Operations)
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

func TestBuildPlan_ShopCultivateEnterWhenLarTimeMissing(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 26, 14, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	// Buy ACK shape: bRecord only — observed but no larTime/resetTime.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5_000_000}},
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 1000}},
			"6": map[string]any{"10001": 1},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 50_000_000

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateEnter.String() || op.Action != "sync" || !op.Executable {
				t.Fatalf("incomplete shop state should re-enter, got %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate enter after incomplete observe: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagVideoGiftBlockedBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.video_gift" {
			if op.Executable || op.Status != PlanStatusAdapterMissing || op.TargetID != 1 || !hasReasonContaining(op.BlockedReasons, "SDK 广告") {
				t.Fatalf("unobserved video gift should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked video gift op: %+v", result.Operations)
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
			if op.Executable || op.Status != PlanStatusAdapterMissing || op.TargetID != 1 || !hasReasonContaining(op.BlockedReasons, "SDK 广告") {
				t.Fatalf("observed video gift should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked video gift op: %+v", result.Operations)
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

func TestBuildPlan_BenefitBoxClaimsWheneverClientCountIsReady(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	// drawCnt=0 with resetCntTime overnight ago: local accrual makes boxes ready.
	resetAt := time.Date(2026, 7, 29, 20, 0, 0, 0, shanghai)
	s := state.New()
	applyMap(t, s, map[string]any{
		"116": map[string]any{
			"0": map[string]any{
				"1": 0,
				"2": resetAt.UnixMilli(),
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Benefit = &pb.BenefitPolicy{BoxEnabled: true}

	cases := []struct {
		name string
		now  time.Time
	}{
		{name: "before former window", now: time.Date(2026, 7, 30, 4, 29, 0, 0, shanghai)},
		{name: "morning", now: time.Date(2026, 7, 30, 4, 45, 0, 0, shanghai)},
		{name: "daytime", now: time.Date(2026, 7, 30, 12, 0, 0, 0, shanghai)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildPlan(s, p, tc.now)
			var claim *PlannedOp
			for i := range result.Operations {
				if result.Operations[i].Kind == clientproto.RPCBenefitBoxDraw.String() {
					claim = &result.Operations[i]
					break
				}
			}
			if claim == nil {
				t.Fatalf("missing benefit box claim; ops=%+v", result.Operations)
			}
			if claim.Count != 8 {
				t.Fatalf("benefit box count=%d, want accrued 8; op=%+v", claim.Count, claim)
			}
		})
	}
}

func TestBuildPlan_BenefitBoxUnobservedIsVisibleAndFailClosed(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Benefit = &pb.BenefitPolicy{BoxEnabled: true}
	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.benefit" {
			if op.Kind == clientproto.RPCBenefitBoxDraw.String() || op.Executable || op.Status != PlanStatusBlocked || !hasReasonContaining(op.BlockedReasons, "116") {
				t.Fatalf("unobserved benefit box must be explicit and fail closed: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing unobserved benefit status: %+v", result.Operations)
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
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"1501": 5, "1502": 20}}},
		"33": map[string]any{
			"0": map[string]any{"0": 77900091102482},
			"1": map[string]any{
				"1": map[string]any{
					"1":  1,
					"2":  50,
					"3":  20,
					"4":  []int32{},
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
		"basic.zoo.feed":   clientproto.RPCZooAddFoodstuff.String(),
		"basic.zoo.stroke": clientproto.RPCZooStrokePet.String(),
	}
	seen := map[string]bool{}
	for _, op := range result.Operations {
		if kind, ok := want[op.Domain]; ok {
			seen[op.Domain] = true
			if op.Kind != kind || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("zoo op mismatch for %s: %+v", op.Domain, op)
			}
			if op.Domain == "basic.zoo.feed" && (op.Action != "stock" || op.ItemID != 1501 || op.Count != 5 || len(op.ItemCost) != 1 || op.ItemCost[1501] != 5) {
				t.Fatalf("zoo stock item/count/cost mismatch: %+v", op)
			}
		}
	}
	for domain := range want {
		if !seen[domain] {
			t.Fatalf("missing %s op: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_ZooBowlStockDoesNotWaitForStatusRefresh(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"1501": 30}}},
		"33": map[string]any{
			"0": map[string]any{"0": 77900091102482},
			"1": map[string]any{"1": map[string]any{
				"1": 1, "2": 50, "3": 20, "4": []int32{}, "5": 2,
				"12": now.Add(-time.Minute).UnixMilli(), "14": now.Add(-time.Second).UnixMilli(),
			}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoFeed = true
	p.Basic.Zoo.AutoStroke = true

	result := BuildPlan(s, p, now)
	var zooOps []PlannedOp
	for _, op := range result.Operations {
		if strings.HasPrefix(op.Domain, "basic.zoo") {
			zooOps = append(zooOps, op)
		}
	}
	if len(zooOps) != 2 || zooOps[0].Kind != clientproto.RPCZooAddFoodstuff.String() || zooOps[1].Kind != clientproto.RPCZooRefreshPetStatus.String() {
		t.Fatalf("zoo operations=%+v, want bowl stock followed by status refresh", zooOps)
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

func TestBuildPlan_ZooFoodBuySyncsThenBuysGoldOffer(t *testing.T) {
	now := time.Now()
	baseState := func(includeShop bool) *state.State {
		s := state.New()
		payload := map[string]any{
			"7": map[string]any{"0": map[string]any{"44": 5000, "32": map[string]any{"1501": 0, "1502": 0}}},
			"33": map[string]any{
				"0": map[string]any{"0": 1},
				"1": map[string]any{"1": map[string]any{"1": 1, "4": []int32{}}},
			},
		}
		if includeShop {
			payload["20"] = map[string]any{"0": map[string]any{
				"9": map[string]any{"1": 9, "3": now.UnixMilli(), "12": map[string]any{"90001": 0}},
			}}
		}
		applyMap(t, s, payload)
		return s
	}
	policy := func() *pb.Policy {
		p := DefaultPolicy()
		p.AutomationEnabled = true
		p.Basic.Zoo.Enabled = true
		p.Basic.Zoo.AutoFeed = true
		p.Basic.Zoo.AutoBuyFood = true
		p.Basic.Zoo.MaxSpendGold = 3000
		return p
	}

	result := BuildPlan(baseState(false), policy(), now)
	var sync *PlannedOp
	for i := range result.Operations {
		if result.Operations[i].Domain == "basic.zoo.buy_food" {
			sync = &result.Operations[i]
			break
		}
	}
	if sync == nil || sync.Kind != clientproto.RPCShopEnter.String() || sync.Action != "sync" || sync.TargetID != 9 || !sync.Executable {
		t.Fatalf("zoo food shop sync=%+v ops=%+v", sync, result.Operations)
	}

	result = BuildPlan(baseState(true), policy(), now)
	var buy *PlannedOp
	for i := range result.Operations {
		if result.Operations[i].Domain == "basic.zoo.buy_food" {
			buy = &result.Operations[i]
			break
		}
	}
	if buy == nil || buy.Kind != clientproto.RPCShopBuy.String() || buy.TargetID != 9 || buy.ItemID != 90001 || buy.Count != 30 || buy.GoldCost != 3000 || !buy.Executable {
		t.Fatalf("zoo food buy=%+v ops=%+v", buy, result.Operations)
	}
	if err := ValidateZooFoodPurchase(baseState(true), policy().GetBasic().GetZoo(), buy, now); err != nil {
		t.Fatalf("ValidateZooFoodPurchase()=%v", err)
	}
}

func TestBuildPlan_ZooFoodBuyHonorsDailyLimitAndBudget(t *testing.T) {
	now := time.Now()
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5000, "32": map[string]any{"1501": 0, "1502": 0}}},
		"20": map[string]any{"0": map[string]any{
			"9": map[string]any{"1": 9, "3": now.UnixMilli(), "12": map[string]any{"90001": 28}},
		}},
		"33": map[string]any{
			"0": map[string]any{"0": 1},
			"1": map[string]any{"1": map[string]any{"1": 1, "4": []int32{}}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoFeed = true
	p.Basic.Zoo.AutoBuyFood = true
	p.Basic.Zoo.MaxSpendGold = 100

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain != "basic.zoo.buy_food" {
			continue
		}
		if op.Kind != clientproto.RPCShopBuy.String() || op.Count != 1 || op.GoldCost != 100 {
			t.Fatalf("budgeted zoo food buy=%+v", op)
		}
		return
	}
	t.Fatalf("missing budgeted zoo food buy: %+v", result.Operations)
}

func TestZooFindPetFeatureRemainsBlocked(t *testing.T) {
	wantBlocked := map[string]bool{"basic.zoo_find_pet": false}
	for _, spec := range featureSpecs {
		if _, ok := wantBlocked[spec.ID]; !ok {
			continue
		}
		if spec.Executable || spec.Status != PlanStatusBlocked || len(spec.BlockedReasons) == 0 {
			t.Fatalf("unsafe zoo feature remains executable: %+v", spec)
		}
		wantBlocked[spec.ID] = true
	}
	for id, seen := range wantBlocked {
		if !seen {
			t.Fatalf("missing zoo feature %s", id)
		}
	}
	wantExecutable := map[string]bool{
		"basic.zoo_handle_event":    false,
		"basic.zoo_read_log":        false,
		"basic.zoo_souvenir_reward": false,
		"basic.zoo_souvenir_read":   false,
	}
	for _, spec := range featureSpecs {
		if _, ok := wantExecutable[spec.ID]; !ok {
			continue
		}
		if !spec.Executable || spec.Status != PlanStatusManaged || len(spec.BlockedReasons) != 0 {
			t.Fatalf("safe zoo log feature not executable: %+v", spec)
		}
		wantExecutable[spec.ID] = true
	}
	for id, seen := range wantExecutable {
		if !seen {
			t.Fatalf("missing zoo feature %s", id)
		}
	}
}

func TestBuildPlan_ZooObservedType2LogsBlockedAndOneReadPerTick(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1},
		"1": map[string]any{"7": map[string]any{"1": 7, "19": int64(1000)}},
		"2": map[string]any{
			"7|40": map[string]any{"1": 7, "2": 40, "5": 2096, "6": 2, "7": 0, "8": map[string]any{}, "9": map[string]any{}, "10": map[string]any{}, "11": map[string]any{}, "13": int64(1900)},
			"7|41": map[string]any{"1": 7, "2": 41, "5": 2001, "7": 1, "13": int64(1500)},
			"7|42": map[string]any{"1": 7, "2": 42, "5": 2096, "6": 2, "7": 0, "8": map[string]any{}, "9": map[string]any{}, "10": map[string]any{}, "11": map[string]any{}, "13": int64(2000)},
		},
	}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoEventEnabled = true

	result := BuildPlan(s, p, time.Now())
	blockedIndexes := map[int32]bool{}
	var read PlannedOp
	executable := 0
	for _, op := range result.Operations {
		if op.Domain != "basic.zoo.event" {
			continue
		}
		if op.Executable {
			executable++
		}
		if op.Action == "handle_event" {
			if op.Executable || op.Status != PlanStatusBlocked || !strings.Contains(op.Reason, "已观测客户端 handleEvent 分支") {
				t.Fatalf("observed type-2 log became executable: %+v", op)
			}
			blockedIndexes[op.ItemID] = true
		}
		if op.Action == "read_log" {
			read = op
		}
	}
	if !blockedIndexes[40] || !blockedIndexes[42] || len(blockedIndexes) != 2 {
		t.Fatalf("blocked handle indexes=%+v, want log indexes 40 and 42", blockedIndexes)
	}
	if executable != 1 || read.Kind != clientproto.RPCZooReadLog.String() || read.TargetID != 7 || read.ItemID != 41 || !read.Executable {
		t.Fatalf("read event op=%+v executable=%d, want one read using log idx 41", read, executable)
	}
	if read.ItemID == 2001 {
		t.Fatalf("read operation used event ID instead of log index: %+v", read)
	}
}

func TestBuildPlan_ZooBlockedLogDoesNotHideSafeRead(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1},
		"1": map[string]any{"7": map[string]any{"1": 7, "19": int64(1000)}},
		"2": map[string]any{
			"7|41": map[string]any{"1": 7, "2": 41, "5": 2001, "7": 1, "13": int64(1500)},
			"7|42": map[string]any{"1": 7, "2": 42, "5": 2096, "6": 2, "7": 0, "8": map[string]any{}, "9": map[string]any{"11": 1}, "10": map[string]any{}, "11": map[string]any{}, "13": int64(2000)},
		},
	}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoEventEnabled = true

	result := BuildPlan(s, p, time.Now())
	var blocked, read bool
	for _, op := range result.Operations {
		if op.Domain != "basic.zoo.event" {
			continue
		}
		if op.Action == "handle_event" && !op.Executable && op.Status == PlanStatusBlocked && strings.Contains(op.Reason, "消耗") {
			blocked = true
		}
		if op.Action == "read_log" && op.Executable && op.Kind == clientproto.RPCZooReadLog.String() {
			read = true
		}
	}
	if !blocked || !read {
		t.Fatalf("zoo event operations=%+v, want blocked diagnostic plus executable read", result.Operations)
	}
}

func TestBuildPlan_ZooSouvenirClaimsBeforeAcknowledging(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1, "13": []int32{1}},
		"2": map[string]any{},
		"4": map[string]any{
			"30201": map[string]any{"1": 30201, "2": 1},
			"32901": map[string]any{"1": 32901, "2": 0},
		},
	}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoEventEnabled = true

	result := BuildPlan(s, p, time.Now())
	var souvenirOps []PlannedOp
	for _, op := range result.Operations {
		if op.Domain == "basic.zoo.souvenir" && op.Executable {
			souvenirOps = append(souvenirOps, op)
		}
	}
	if len(souvenirOps) != 1 {
		t.Fatalf("souvenir ops=%+v, want one claim", souvenirOps)
	}
	claim := souvenirOps[0]
	if claim.Kind != clientproto.RPCZooRecvSouvenirRwd.String() || claim.Action != "claim" || claim.Priority != 5663 || claim.FeatureID != "basic.zoo_souvenir_reward" || claim.Count != 1 || len(claim.SlotIDs) != 1 || claim.SlotIDs[0] != 2 || claim.OperationID != clientproto.RPCZooRecvSouvenirRwd.String() {
		t.Fatalf("claim op=%+v", claim)
	}

	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"13": []int32{1, 2}}}})
	result = BuildPlan(s, p, time.Now())
	souvenirOps = nil
	for _, op := range result.Operations {
		if op.Domain == "basic.zoo.souvenir" && op.Executable {
			souvenirOps = append(souvenirOps, op)
		}
	}
	if len(souvenirOps) != 1 {
		t.Fatalf("souvenir ops after claim=%+v, want one read", souvenirOps)
	}
	read := souvenirOps[0]
	if read.Kind != clientproto.RPCZooReadSouvenir.String() || read.Action != "read" || read.Priority != 5662 || read.FeatureID != "basic.zoo_souvenir_read" || read.Count != 1 || len(read.SlotIDs) != 1 || read.SlotIDs[0] != 32901 || read.OperationID != clientproto.RPCZooReadSouvenir.String() {
		t.Fatalf("read op=%+v", read)
	}
}

func TestBuildPlan_ZooSouvenirBatchesRewardsAndReusesAutoEventPolicy(t *testing.T) {
	s := state.New()
	entries := map[string]any{}
	for _, id := range []int32{30201, 32901, 32902, 32903} {
		entries[strconv.FormatInt(int64(id), 10)] = map[string]any{"1": id, "2": 0}
	}
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1, "13": nil},
		"2": map[string]any{},
		"4": entries,
	}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true

	for _, enabled := range []bool{false, true} {
		p.Basic.Zoo.AutoEventEnabled = enabled
		result := BuildPlan(s, p, time.Now())
		var got []PlannedOp
		for _, op := range result.Operations {
			if op.Domain == "basic.zoo.souvenir" && op.Executable {
				got = append(got, op)
			}
		}
		if !enabled {
			if len(got) != 0 {
				t.Fatalf("auto_event=false produced souvenir ops: %+v", got)
			}
			continue
		}
		if len(got) != 1 || got[0].Kind != clientproto.RPCZooRecvSouvenirRwd.String() || len(got[0].SlotIDs) != 4 {
			t.Fatalf("batched rewards=%+v", got)
		}
		for i, want := range []int32{1, 2, 3, 4} {
			if got[0].SlotIDs[i] != want {
				t.Fatalf("reward batch=%v, want [1 2 3 4]", got[0].SlotIDs)
			}
		}
	}

	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"13": []int32{1, 2, 3, 4}}}})
	p.Basic.Zoo.AutoEventEnabled = true
	result := BuildPlan(s, p, time.Now())
	var read *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Domain == "basic.zoo.souvenir" && op.Executable {
			if read != nil {
				t.Fatalf("multiple souvenir ops after claims: %+v", result.Operations)
			}
			read = op
		}
	}
	if read == nil || read.Kind != clientproto.RPCZooReadSouvenir.String() || len(read.SlotIDs) != 4 {
		t.Fatalf("batched unread souvenirs=%+v", read)
	}
	for i, want := range []int32{30201, 32901, 32902, 32903} {
		if read.SlotIDs[i] != want {
			t.Fatalf("read batch=%v, want sorted souvenir IDs", read.SlotIDs)
		}
	}
}

func TestBuildPlan_ZooSouvenirUnknownRewardListBlocksWithoutReading(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1},
		"2": map[string]any{},
		"4": map[string]any{"32901": map[string]any{"1": 32901, "2": 0}},
	}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoEventEnabled = true

	result := BuildPlan(s, p, time.Now())
	var blocked bool
	for _, op := range result.Operations {
		if op.Domain != "basic.zoo.souvenir" {
			continue
		}
		if op.Action == "claim" && !op.Executable && op.Status == PlanStatusBlocked && strings.Contains(op.Reason, "领取列表未观测") {
			blocked = true
		}
		if op.Executable {
			t.Fatalf("unknown reward list produced executable souvenir RPC: %+v", op)
		}
	}
	if !blocked {
		t.Fatalf("unknown reward list ops=%+v, want blocked claim diagnostic", result.Operations)
	}
}

func TestBuildPlan_ZooLogPrecedesSouvenirReward(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1, "13": []int32{1}},
		"1": map[string]any{"7": map[string]any{"1": 7, "19": int64(1000)}},
		"2": map[string]any{"7|41": map[string]any{"1": 7, "2": 41, "5": 2096, "7": 1, "13": int64(1500)}},
		"4": map[string]any{
			"30201": map[string]any{"1": 30201, "2": 1},
			"32901": map[string]any{"1": 32901, "2": 0},
		},
	}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Zoo.Enabled = true
	p.Basic.Zoo.AutoEventEnabled = true
	p.Union.Race.Enabled = false

	result := BuildPlan(s, p, time.Now())
	readIndex, claimIndex := -1, -1
	for i, op := range result.Operations {
		switch op.Kind {
		case clientproto.RPCZooReadLog.String():
			readIndex = i
		case clientproto.RPCZooRecvSouvenirRwd.String():
			claimIndex = i
		}
	}
	if readIndex < 0 || claimIndex < 0 || readIndex >= claimIndex {
		t.Fatalf("operation order readLog=%d claim=%d ops=%+v", readIndex, claimIndex, result.Operations)
	}
	if planned := Plan(s, p, time.Now()); planned == nil || planned.Kind != clientproto.RPCZooReadLog.String() {
		t.Fatalf("first executable=%+v, want readLog before souvenir reward", planned)
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
			"0": map[string]any{"1": map[string]any{
				"1": 1, "3": now.Add(2 * time.Hour).UnixMilli(), "6": 1, "7": 0, "8": 2,
			}},
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
		"basic.pearl.place":   clientproto.RPCPearlPlaceRecvOneKey.String(),
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
			if op.Domain == "basic.pearl.place" && (op.TargetID != 0 || op.ItemID != 0 || op.Count != 0) {
				t.Fatalf("pearl one-key op must have no request fields: %+v", op)
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
	now := time.Date(2026, 7, 26, 14, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5000}},
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 3214}},
			"2": now.Add(-time.Hour).UnixMilli(),
			"3": now.Add(-time.Hour).UnixMilli(),
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 4000
	p.Basic.Shop.CultivateShop.ItemIds = []int32{1401}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateBuy.String() || op.TargetID != 10001 || op.ItemID != 1401 || op.GoldCost != 3214 || !op.Executable || op.SyncOnly {
				t.Fatalf("shop cultivate buy op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopCultivateBuyDespiteStaleResetMs(t *testing.T) {
	// Live failure mode (叶小楠): enter/refresh omit lResetTime while larTime is
	// current; a prior-day resetMs previously forced an infinite enter loop and
	// blocked buys even when infoMap still had remaining offers.
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 10, 15, 0, 0, 0, shanghai)
	staleReset := time.Date(2026, 8, 4, 0, 5, 0, 0, shanghai)
	freshLar := time.Date(2026, 8, 10, 14, 55, 0, 0, shanghai)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 50_000_000}},
		"113": map[string]any{
			"1": map[string]any{"10004": []int32{11, 5001}},
			"2": staleReset.UnixMilli(),
			"3": freshLar.UnixMilli(),
			"6": map[string]any{},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 50_000_000

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateBuy.String() || op.TargetID != 10004 || !op.Executable {
				t.Fatalf("expected buy despite stale resetMs, got %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopCultivateEntersWhenAutoCDReady(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 26, 17, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	lar := now.Add(-9001 * time.Second)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5000}},
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 3214}},
			"2": lar.UnixMilli(),
			"3": lar.UnixMilli(),
			"4": 0,
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 4000
	p.Basic.Shop.CultivateShop.ItemIds = []int32{1401}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateEnter.String() || op.Action != "sync" || !op.Executable {
				t.Fatalf("shop cultivate automatic enter op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate refresh op: %+v", result.Operations)
}

func TestBuildPlan_ShopCultivateAutoEnterIgnoresManualRefreshCount(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 26, 17, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	lar := now.Add(-9001 * time.Second)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5000}},
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 3214}},
			"2": lar.UnixMilli(),
			"3": lar.UnixMilli(),
			"4": 3, // $frTimes exhausted; further refresh costs yuanbao
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 4000
	p.Basic.Shop.CultivateShop.ItemIds = []int32{1401}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateEnter.String() || op.Action != "sync" || !op.Executable {
				t.Fatalf("automatic rotation must enter regardless of manual refresh count: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopCultivateDiamondCostBlocked(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 26, 14, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{1, 10}},
			"2": now.UnixMilli(),
			"3": now.UnixMilli(),
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendDiamond = 20
	p.Basic.Shop.CultivateShop.ItemIds = []int32{10001}

	result := BuildPlan(s, p, now)
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

	// Video tier (id=1) is adapter-missing; with gold enabled, planner skips to id=2.
	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Kind != clientproto.RPCFmlBld.String() || op.TargetID != 2 || op.GoldCost != 10000 || !op.Executable || op.SyncOnly {
				t.Fatalf("gold union build op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildPaidOptionsShareDailyLimit(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 20000}},
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"2": 0, "3": 5}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.GoldEnabled = true
	p.Union.Build.MaxSpendGold = 12000

	for _, op := range BuildPlan(s, p, time.Now()).Operations {
		if op.Domain == "union.build" {
			t.Fatalf("paid build group already used 5/5; unexpected op: %+v", op)
		}
	}
}

func TestBuildPlan_UnionBuildVideoBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"1": 0}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.FreeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Kind != clientproto.RPCFmlBld.String() || op.TargetID != 1 || op.Executable {
				t.Fatalf("video union build op should be blocked: %+v", op)
			}
			if !hasReasonContaining(op.BlockedReasons, "SDK 广告") {
				t.Fatalf("blocked reasons = %v, want SDK 广告", op.BlockedReasons)
			}
			return
		}
	}
	t.Fatalf("missing blocked video union build op: %+v", result.Operations)
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
			if op.Kind != clientproto.RPCFmlBld.String() || op.TargetID != 3 || op.DiamondCost != 10 || op.Executable || len(op.BlockedReasons) == 0 {
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
		"7": map[string]any{"0": map[string]any{"41": 5, "42": 1000}},
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
		if op.Kind != clientproto.RPCFmlBld.String() || op.TargetID != 3 || op.DiamondCost != 10 || op.Executable {
			t.Fatalf("diamond union build op mismatch: %+v", op)
		}
		if !hasReasonContaining(op.BlockedReasons, "元宝不足") {
			t.Fatalf("blocked reasons = %v, want 元宝不足", op.BlockedReasons)
		}
		for _, gate := range op.CostGates {
			if gate.ResourceKind == GateResourceDiamond {
				if gate.Available != 5 {
					t.Fatalf("diamond gate available = %d, want 5", gate.Available)
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
			if !strings.Contains(op.Reason, "土地#1") || !strings.Contains(op.Reason, "×4") {
				t.Fatalf("union land harvest reason should describe targets: %q", op.Reason)
			}
			return
		}
	}
	t.Fatalf("missing union land harvest op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandHarvestComputesStaleMatureCount(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23005,
						"2": now.Add(-45 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 0,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.harvest" {
			if op.Kind != clientproto.RPCFmlLandHarvest.String() || !op.Executable {
				t.Fatalf("stale mature count should still harvest: %+v", op)
			}
			if len(op.LandIDs) != 1 || op.LandIDs[0] != 1 {
				t.Fatalf("harvest land ids=%v, want [1]", op.LandIDs)
			}
			if !strings.Contains(op.Reason, "×3") {
				t.Fatalf("harvest reason should use computed pending: %q", op.Reason)
			}
			return
		}
	}
	t.Fatalf("missing union land harvest op with stale matureFlwCnt: %+v", result.Operations)
}

func TestBuildPlan_UnionLandHarvestRequiresObservedState(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"25": map[string]any{}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land" && op.Action == "sync" {
			if op.Kind != clientproto.RPCFmlEnter.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("unobserved union land should sync via fml.enter: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land sync op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantPrefersLowLevelLowStock(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 10,
			"23005": 2,
		}}},
		"101": map[string]any{"0": cultivate(23001, 23005)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
					"2": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.Kind != clientproto.RPCFmlLandPlant.String() || !op.Executable || op.FlowerID != 23005 {
				t.Fatalf("union land plant should prefer low-level low-stock 23005: %+v", op)
			}
			if len(op.LandIDs) != 2 || op.Count != 2 {
				t.Fatalf("union land plant should cover empty lands: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land plant op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantPrefersLowestLevelWhileBelow11(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 1,
			"23005": 1,
		}}},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 8, "4": 2},
			"23005": map[string]any{"1": 23005, "2": 3, "4": 2},
		}},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	// Maturity must not override leveling priority below 11.
	p.Union.Land.MinMaturityMinutes = 999

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			// Same quality (凡): still prefer the lower cultivate level.
			if op.FlowerID != 23005 {
				t.Fatalf("same-quality below-11 should prefer lowest cultivate level 23005, got %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land plant op for lowest level: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantPrefersHigherQualityWhileBelow11(t *testing.T) {
	// 花扇鹊嬉=华(4), 轻粉蓝铃花=普(2). Even with higher level / higher stock,
	// the华品 flower must be planted first while leveling below 11.
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23310": 50,
			"23603": 1,
		}}},
		"101": map[string]any{"0": map[string]any{
			"23310": map[string]any{"1": 23310, "2": 8, "4": 2},
			"23603": map[string]any{"1": 23603, "2": 2, "4": 2},
		}},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
					"2": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.MinMaturityMinutes = 999

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23310 {
				t.Fatalf("below-11 should prefer higher-quality 花扇鹊嬉(23310) over 轻粉蓝铃花(23603), got %+v", op)
			}
			if !strings.Contains(op.Reason, "品阶高") {
				t.Fatalf("reason should mention quality priority: %q", op.Reason)
			}
			return
		}
	}
	t.Fatalf("missing union land plant op for higher quality: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantHonorsReplantCooldownBelow11(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 50,
			"23005": 1, // lowest stock among equal low levels
		}}},
		"101": map[string]any{"0": cultivate(23001, 23005)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23001,
						// 5 minutes into 15-minute cycle → next mature in 10m (>2m grace)
						"2": now.Add(-5 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 0,
					},
					"2": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23005 {
				t.Fatalf("below-11 should force-plant lowest-stock 23005, got %+v", op)
			}
			if len(op.LandIDs) != 1 || op.LandIDs[0] != 2 {
				t.Fatalf("below-11 should preserve cooling land 1 and fill land 2, got %+v", op.LandIDs)
			}
			if !strings.Contains(op.Reason, "收获与冷却边界") {
				t.Fatalf("reason should mention safe replacement boundary: %q", op.Reason)
			}
			return
		}
	}
	t.Fatalf("missing empty-land below-11 plant op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantUsesLongMaturityWhenAllHighLevel(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 1,
			"23078": 5,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23001, 23078)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.MinMaturityMinutes = 20

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23078 {
				t.Fatalf("high-level auto-plant should pick long-maturity 23078, got %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land plant op for long maturity: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantRespectsFlowerIDs(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 1,
			"23117": 99,
		}}},
		"101": map[string]any{"0": cultivate(23001, 23117)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.FlowerIds = []int32{23117}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23117 {
				t.Fatalf("flower_ids allowlist should force 莹白露薇 23117, got %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land plant op for flower_ids: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantReplacesAfterHarvestCycle(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23117": 1,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23001, 23117)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23001,
						"2": now.Add(-70 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 999,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.FlowerIds = []int32{23117}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23117 || len(op.LandIDs) != 1 || op.LandIDs[0] != 1 {
				t.Fatalf("post-level-11 replace should plant 23117 on land 1: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing post-level-11 replace plant op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantNeverReplacesPendingHarvest(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23117": 1,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23117)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23307,
						"2": now.Add(-9 * time.Hour).UnixMilli(),
						"3": 3,
						"4": 0,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.FlowerIds = []int32{23117}

	for _, op := range BuildPlan(s, p, now).Operations {
		if op.Domain == "union.land.plant" {
			t.Fatalf("pending harvest was destructively replaced: %+v", op)
		}
	}
}

func TestBuildPlan_UnionLandAutoPlantReplacesHourly(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23078": 1,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23001, 23078)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23001,
						"2": now.Add(-2 * time.Hour).UnixMilli(),
						"3": 6,
						"4": 6,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.MinMaturityMinutes = 20

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23078 || len(op.LandIDs) != 1 || op.LandIDs[0] != 1 {
				t.Fatalf("level-11 replace should plant long-maturity 23078 on land 1: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing level-11 replace plant op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantReplacesBelow11AfterSafeBoundary(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23117": 1,
		}}},
		"101": map[string]any{"0": cultivate(23001, 23117)}, // level 1
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23001,
						// Past the 60m cooldown and 5m from next mature: safe to replace.
						"2": now.Add(-70 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 4,
					},
					"2": map[string]any{"0": 0}, // empty
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.FlowerIds = []int32{23117}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23117 {
				t.Fatalf("below-11 plant flower=%d, want 23117", op.FlowerID)
			}
			if len(op.LandIDs) != 2 || op.LandIDs[0] != 1 || op.LandIDs[1] != 2 {
				t.Fatalf("below-11 should replace eligible land 1 and fill land 2, got %+v", op.LandIDs)
			}
			if !strings.Contains(op.Reason, "收获与冷却边界") {
				t.Fatalf("reason should mention safe replacement boundary: %q", op.Reason)
			}
			return
		}
	}
	t.Fatalf("missing safe replacement plant op below level 11: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantWaitsNearMatureBelowLevel11(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23117": 1,
		}}},
		"101": map[string]any{"0": cultivate(23001, 23117)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23001,
						// Past cooldown; level-0 cycle is 900s and next mature is 60s away.
						"2": now.Add(-74 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 4,
					},
					"2": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.FlowerIds = []int32{23117}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23117 {
				t.Fatalf("plant flower=%d, want 23117", op.FlowerID)
			}
			if len(op.LandIDs) != 1 || op.LandIDs[0] != 2 {
				t.Fatalf("near-mature land 1 should wait for harvest; only fill land 2, got %+v", op.LandIDs)
			}
			return
		}
	}
	t.Fatalf("missing empty-only plant while waiting near mature: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantSkipsFreshPlant(t *testing.T) {
	// Same-flower occupied land stays untouched.
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23005": 1,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23005)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23005,
						"2": now.Add(-10 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 0,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.MinMaturityMinutes = 999

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			t.Fatalf("same flower should not replant: %+v", op)
		}
	}
}

func TestBuildPlan_UnionLandAutoPlantSkipsReplaceWithinReplantCooldown(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23078": 1,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23001, 23078)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23001,
						"2": now.Add(-10 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 0,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.MinMaturityMinutes = 20

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			t.Fatalf("occupied land within replant cooldown should not be replaced: %+v", op)
		}
	}
}

func TestBuildPlan_UnionLandAutoPlantFillsEmptyOnlyWithMixedFlowers(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23078": 1,
		}}},
		"101": map[string]any{"0": cultivateAtLevel(11, 23001, 23078)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
					"2": map[string]any{
						"0": 0,
						"1": 23001,
						"2": now.Add(-10 * time.Minute).UnixMilli(),
						"3": 0,
						"4": 0,
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.MinMaturityMinutes = 20

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.land.plant" {
			if op.FlowerID != 23078 || len(op.LandIDs) != 1 || op.LandIDs[0] != 1 {
				t.Fatalf("should only fill empty land 1, not replace land 2: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing empty-only plant with mixed flowers: %+v", result.Operations)
}

func TestBuildPlan_UnionLandAutoPlantRequiresEnabled(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": cultivate(23005)},
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = false
	p.Union.Land.AutoPlantEnabled = false

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if strings.HasPrefix(op.Domain, "union.land") {
			t.Fatalf("disabled union land should plan nothing: %+v", op)
		}
	}
}

func TestBuildPlan_UnionDomainWaitsForObservedMembership(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Flower.TakeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Category == CategoryUnion || op.Category == CategoryRace {
			t.Fatalf("unobserved membership must gate every guild operation: %+v", op)
		}
	}
}

func TestBuildPlan_UnionLandAutoPlantSyncWithoutHarvest(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"25": map[string]any{}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land" && op.Action == "sync" {
			if op.Kind != clientproto.RPCFmlEnter.String() || !op.Executable {
				t.Fatalf("auto-plant alone should still sync: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land sync for auto-plant: %+v", result.Operations)
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
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 0,
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true

	now := state.FmlFlowerTakeWindowStart(time.Now()).Add(time.Minute)
	result := BuildPlan(s, p, now)
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
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 0,
				"3": time.Now().UnixMilli(),
			},
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

	now := state.FmlFlowerTakeWindowStart(time.Now()).Add(time.Minute)
	result := BuildPlan(s, p, now)
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

func TestBuildPlan_UnionFlowerTakePrefersLowestStock(t *testing.T) {
	// Both configured flowers are available; 23009 has lower FlowerID so the
	// old first-match path would take it, but inventory stock favors 23011.
	s := state.New()
	now := state.FmlFlowerTakeWindowStart(time.Now()).Add(time.Minute)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23009": 50,
			"23011": 3,
		}}},
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 0,
				"3": now.UnixMilli(),
			},
			"108": []any{
				map[string]any{
					"0": 77900091102483,
					"1": map[string]any{
						"1": map[string]any{"0": 23009, "1": 8, "2": 0},
					},
				},
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 6, "2": 0},
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	p.Union.Flower.TakeFlowerIds = []int32{23009, 23011}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			if op.Kind != clientproto.RPCFmlFlowerShareTake.String() || !op.Executable {
				t.Fatalf("union flower take op mismatch: %+v", op)
			}
			if op.FlowerID != 23011 || op.TargetUID != 77900091102484 || op.TargetID != 2 {
				t.Fatalf("expected lowest-stock 23011, got %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union flower take op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerTakeSkipsWhenDailyLimitReached(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 0,
			},
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 6, "2": 1},
					},
				},
			},
		},
	})
	now := state.FmlFlowerTakeWindowStart(time.Now()).Add(time.Hour)
	s.MarkFmlFlowerTakeDailyLimitReached(now)

	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			t.Fatalf("expected no union flower take after daily limit, got %+v", op)
		}
	}
}

func TestBuildPlan_UnionFlowerTakeSkipsWhenTdyTakeCntExhausted(t *testing.T) {
	s := state.New()
	now := state.FmlFlowerTakeWindowStart(time.Now()).Add(time.Hour)
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"0": map[string]any{
				"0":   1001,
				"102": 2,
			},
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 2,
				"3": now.UnixMilli(),
			},
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 6, "2": 1},
					},
				},
			},
		},
	})
	if !s.FmlFlowerTakeExhausted(now) {
		t.Fatalf("FmlFlowerTakeExhausted=false, want true (tdy=%d limit=%d)", s.FmlFlowerShare().TdyTakeCnt, s.FmlFlowerTakeLimit())
	}

	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			t.Fatalf("expected no union flower take when tdyTakeCnt exhausted, got %+v", op)
		}
	}
}

func TestBuildPlan_UnionFlowerTakeContinuesWhenGuildLimitUnobserved(t *testing.T) {
	s := state.New()
	now := time.Now()
	if windowStart := state.FmlFlowerTakeWindowStart(now); now.Before(windowStart) {
		now = windowStart.Add(time.Minute)
	}
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 1,
				"3": now.UnixMilli(),
			},
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 6, "2": 1},
					},
				},
			},
		},
	})
	if s.FmlFlowerTakeExhausted(now) {
		t.Fatal("unobserved FlowerTakeCnt must not exhaust after a single take")
	}

	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_ALL

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" && op.Kind == clientproto.RPCFmlFlowerShareTake.String() {
			return
		}
	}
	t.Fatalf("expected continued take while guild limit unobserved, got %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerTakeSkipsBeforeWindow(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{"0": 1, "1": map[string]any{}, "2": 0},
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{"2": map[string]any{"0": 23011, "1": 6, "2": 1}},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_ALL

	before := state.FmlFlowerTakeWindowStart(time.Now()).Add(-time.Second)
	result := BuildPlan(s, p, before)
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			t.Fatalf("expected no take before 00:01, got %+v", op)
		}
	}
}

func TestBuildPlan_UnionFlowerTakeNoMatchDoesNotTake(t *testing.T) {
	s := state.New()
	now := state.FmlFlowerTakeWindowStart(time.Now()).Add(time.Minute)
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{"0": 1, "1": map[string]any{}, "2": 0, "3": now.UnixMilli()},
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{"2": map[string]any{"0": 23011, "1": 6, "2": 1}},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	p.Union.Flower.TakeFlowerIds = []int32{99999}

	result := BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			t.Fatalf("expected no take when no matching flower, got %+v", op)
		}
	}
}

func TestBuildPlan_UnionFlowerTakeHourlyResync(t *testing.T) {
	s := state.New()
	synced := time.Now().UnixMilli()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{"0": 1, "1": map[string]any{}, "2": 0, "3": synced},
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{"2": map[string]any{"0": 23011, "1": 6, "2": 1}},
				},
			},
		},
	})
	if got := s.OtherFmlFlowerSharesSyncedAtMs(); got <= 0 {
		t.Fatal("expected other-share syncedAt")
	}
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_ALL

	fresh := state.FmlFlowerTakeWindowStart(time.UnixMilli(synced)).Add(time.Minute)
	if fresh.UnixMilli() < synced {
		fresh = time.UnixMilli(synced).Add(time.Minute)
	}
	result := BuildPlan(s, p, fresh)
	foundTake := false
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" && op.Kind == clientproto.RPCFmlFlowerShareTake.String() {
			foundTake = true
		}
		if op.Kind == clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String() {
			t.Fatalf("fresh list should not resync, got %+v", op)
		}
	}
	if !foundTake {
		t.Fatalf("expected take while list fresh: %+v", result.Operations)
	}

	stale := time.UnixMilli(s.OtherFmlFlowerSharesSyncedAtMs()).Add(time.Hour + time.Second)
	if !state.FmlFlowerTakeWindowOpen(stale) {
		stale = state.FmlFlowerTakeWindowStart(stale).Add(time.Hour)
	}
	result = BuildPlan(s, p, stale)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String() {
			return
		}
	}
	t.Fatalf("expected hourly resync, got %+v", result.Operations)
}

func TestBuildPlan_UnionForestSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"25": map[string]any{}})
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

func TestUnionForestRefreshBacksOffAfterUnusableAcknowledgement(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.Local)
	if ops := unionForestOperations(s, true, now); len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlForestRefresh.String() {
		t.Fatalf("initial forest sync=%+v, want one refresh", ops)
	}

	s.MarkFmlForestRefreshAttemptAt(now)
	if ops := unionForestOperations(s, true, now.Add(30*time.Second)); len(ops) != 0 {
		t.Fatalf("recent empty acknowledgement must back off, got %+v", ops)
	}
	if ops := unionForestOperations(s, true, now.Add(fmlForestRefreshRetryInterval)); len(ops) != 1 {
		t.Fatalf("forest sync must retry after backoff, got %+v", ops)
	}
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
	var batches []int32
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" {
			n := int32(len(op.LandIDs))
			countByFlower[op.FlowerID] += n
			batches = append(batches, n)
		}
	}
	// ALL mode: plant lowest stock in batches of 4, then re-rank.
	// A0 → 4, then B1 → 2 → A:4 B:2.
	if countByFlower[23001] != 4 || countByFlower[23002] != 2 {
		t.Fatalf("fallback batch split want A:4 B:2, got %v ops=%+v", countByFlower, result.Operations)
	}
	for _, n := range batches {
		if n <= 0 || n > autoReplantBatchSize {
			t.Fatalf("ALL auto-replant batch size=%d, want 1..%d: %+v", n, autoReplantBatchSize, batches)
		}
	}
}

func TestBuildPlan_AutoReplantAllModeQualityFilter(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23014": 200, // 蓝星花 凡
			"23077": 10,  // 迎春花 普, lower stock
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(4)}},
		"101": map[string]any{"0": cultivate(23014, 23077)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoReplantMode = pb.SelectionMode_SELECTION_MODE_ALL
	p.Union.Race.Enabled = false

	// Empty qualities = all → prefer lowest stock 迎春花.
	result := BuildPlan(s, p, time.Now())
	first := Plan(s, p, time.Now())
	if first == nil || first.FlowerID != 23077 {
		t.Fatalf("empty qualities should plant lowest-stock 23077, got %+v ops=%+v", first, result.Operations)
	}

	// Only 凡 (1) → 迎春花 excluded even though lower stock.
	p.Plant.Planting.AutoReplantQualities = []int32{1}
	first = Plan(s, p, time.Now())
	if first == nil || first.FlowerID != 23014 {
		t.Fatalf("quality=凡 should plant 23014, got %+v", first)
	}
}

func TestBuildPlan_AutoReplantMinLevelFilter(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23014": 200, // lv1, higher stock
			"23077": 10,  // lv11, lower stock
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(4)}},
		"101": map[string]any{"0": map[string]any{
			"23014": map[string]any{"1": 23014, "2": 1, "4": 2},
			"23077": map[string]any{"1": 23077, "2": 11, "4": 2},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoReplantMode = pb.SelectionMode_SELECTION_MODE_ALL
	p.Union.Race.Enabled = false

	// No min level → prefer lowest stock 23077.
	first := Plan(s, p, time.Now())
	if first == nil || first.FlowerID != 23077 {
		t.Fatalf("min_level=0 should plant lowest-stock 23077, got %+v", first)
	}

	// Min level 11 → still 23077 (only eligible flower at >=11).
	p.Plant.Planting.AutoReplantMinLevel = 11
	first = Plan(s, p, time.Now())
	if first == nil || first.FlowerID != 23077 {
		t.Fatalf("min_level=11 should plant 23077, got %+v", first)
	}

	// Min level 12 → 23077 excluded; fall back to 23014 only if it meets... it doesn't.
	// So no plant ops from auto-replant. Drop min to 1 and confirm 23014 when 23077 is too low.
	p.Plant.Planting.AutoReplantMinLevel = 12
	first = Plan(s, p, time.Now())
	if first != nil && first.Domain == "farm.plant" && first.FlowerID == 23077 {
		t.Fatalf("min_level=12 should not plant lv11 flower 23077, got %+v", first)
	}
	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.Executable {
			t.Fatalf("min_level=12 should yield no plantable candidates, got %+v", op)
		}
	}

	// Only lv1 flower meets... wait, raise 23014 to lv12 and confirm it is chosen.
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23014": map[string]any{"1": 23014, "2": 12, "4": 2},
			"23077": map[string]any{"1": 23077, "2": 11, "4": 2},
		}},
	})
	first = Plan(s, p, time.Now())
	if first == nil || first.FlowerID != 23014 {
		t.Fatalf("min_level=12 should plant lv12 23014, got %+v", first)
	}
}

func TestPlantAssignments_AutoReplantMinLevelDoesNotRestrictDemand(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 1, "4": 2},
			"23002": map[string]any{"1": 23002, "2": 15, "4": 2},
		}},
	})
	p := DefaultPolicy()
	p.Plant.Planting.DemandPriorityEnabled = true
	p.Plant.Planting.AutoReplantMinLevel = 11

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
		t.Fatalf("task demand should bypass min-level filter, assignments=%+v", assignments)
	}
}

func TestBuildPlan_LowStockAutoReplantKeepsStockOrderOverFlowerID(t *testing.T) {
	// High flower IDs used to starve: equal-priority plant ops were re-sorted by
	// OperationID (which embeds flower=ID), so 23014 ran before lower-stock 23077.
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23014": 167,
			"23077": 163,
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(8)}},
		"101": map[string]any{"0": cultivate(23014, 23077)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = false

	result := BuildPlan(s, p, time.Now())
	var plantOps []PlannedOp
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.Executable {
			plantOps = append(plantOps, op)
		}
	}
	if len(plantOps) == 0 {
		t.Fatalf("expected plant ops: %+v", result.Operations)
	}
	if plantOps[0].FlowerID != 23077 {
		t.Fatalf("first plant op should be lowest-stock 23077, got flower=%d ops=%+v", plantOps[0].FlowerID, plantOps)
	}
	if Plan(s, p, time.Now()).FlowerID != 23077 {
		t.Fatalf("Plan() should select lowest-stock 23077 first, got %+v", Plan(s, p, time.Now()))
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

func TestBuildPlan_LowStockFallbackCountsPlantedLands(t *testing.T) {
	s := state.New()
	lands := emptyLands(4)
	// Four lands already growing 23001 so effective stock A=0+4, B=1.
	for i := 0; i < 4; i++ {
		lands[itoa(2001+i)] = map[string]any{"0": 23001, "1": 2}
	}
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 0,
			"23002": 1,
			"23003": 5,
		}}},
		"100": map[string]any{"0": map[string]any{"1": lands}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true

	result := BuildPlan(s, p, time.Now())
	countByFlower := map[int32]int32{}
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.Executable {
			countByFlower[op.FlowerID] += int32(len(op.LandIDs))
		}
	}
	if countByFlower[23002] != 4 || countByFlower[23001] != 0 {
		t.Fatalf("planted lands should raise effective stock so B is planted next, got %v ops=%+v", countByFlower, result.Operations)
	}
}

func TestBuildPlan_ExcludeModeUsesLowestStockBatches(t *testing.T) {
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
	p.Plant.Planting.AutoReplantMode = pb.SelectionMode_SELECTION_MODE_EXCLUDE
	p.Plant.Planting.AutoReplantExcludeFlowerIds = []int32{23003}

	result := BuildPlan(s, p, time.Now())
	countByFlower := map[int32]int32{}
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" {
			countByFlower[op.FlowerID] += int32(len(op.LandIDs))
		}
	}
	if countByFlower[23003] != 0 {
		t.Fatalf("exclude mode planted excluded flower: %v", countByFlower)
	}
	if countByFlower[23001] != 4 || countByFlower[23002] != 2 {
		t.Fatalf("exclude mode batch split want A:4 B:2, got %v ops=%+v", countByFlower, result.Operations)
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
	p.Plant.Planting.DemandPriorityEnabled = true

	plan := planPlantAssignments(s, p.Plant, []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  1,
		Priority: 90,
		Label:    "顾客订单",
	}}, 2, false)
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
	p.Plant.Planting.DemandPriorityEnabled = true
	demands := []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  1,
		Priority: 90,
		Label:    "顾客订单",
	}}

	ops := farmOps(s, p.Plant, demands, time.Now(), false)
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
		return
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
	p.Plant.Planting.DemandPriorityEnabled = true
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
	p.Plant.Planting.DemandPriorityEnabled = true

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

func TestPlantAssignments_DemandPriorityDisabledSkipsDemandUsesStock(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 5,
			"23002": 0,
			"23003": 8,
		}}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	// Default: DemandPriorityEnabled=false → ignore order demand, plant lowest stock.

	assignments := plantAssignments(s, p.Plant, []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  6,
		Priority: 90,
		Label:    "顾客订单",
	}}, 4)
	if len(assignments) == 0 {
		t.Fatalf("expected auto-replant assignments: %+v", assignments)
	}
	for _, assignment := range assignments {
		if assignment.GoalID != GoalAutoReplant {
			t.Fatalf("demand priority off should not claim lands for orders, got %+v", assignments)
		}
		if assignment.FlowerID != 23002 {
			t.Fatalf("ALL mode should plant lowest stock 23002, got %+v", assignments)
		}
	}
}

func TestPlantAssignments_DemandPriorityEnabledFillsRemainderWithAutoReplant(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 10,
			"23002": 0,
			"23003": 8,
		}}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.Plant.Planting.DemandPriorityEnabled = true

	assignments := plantAssignments(s, p.Plant, []Demand{{
		ID:       "demand-23001",
		GoalID:   GoalCustomerOrder,
		Kind:     DemandKindFlower,
		ItemID:   23001,
		Missing:  2,
		Priority: 90,
		Label:    "顾客订单",
	}}, 6)
	if len(assignments) < 2 {
		t.Fatalf("want demand + auto-replant remainder, got %+v", assignments)
	}
	if assignments[0].GoalID != GoalCustomerOrder || assignments[0].FlowerID != 23001 || assignments[0].Count != 2 {
		t.Fatalf("demand should claim first: %+v", assignments)
	}
	var fallback int32
	for _, assignment := range assignments[1:] {
		if assignment.GoalID != GoalAutoReplant {
			t.Fatalf("remainder should be auto-replant, got %+v", assignments)
		}
		fallback += assignment.Count
		if assignment.FlowerID != 23002 {
			t.Fatalf("remainder should prefer lowest stock 23002, got %+v", assignments)
		}
	}
	if fallback != 4 {
		t.Fatalf("auto-replant remainder=%d, want 4: %+v", fallback, assignments)
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
