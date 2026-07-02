package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// TestRecommend covers the per-land state machine. The mapping is anchored
// in scripts/tools/garden_bot.py and the analysis report; any future change
// here is a behavior change and must come with a capture-driven justification.
func TestRecommend(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cases := []struct {
		name   string
		land   state.LandView
		want   string
		reason string // substring expected
	}{
		{
			name: "never observed",
			land: state.LandView{Observed: false},
			want: KindUnknown,
		},
		{
			name: "observed empty (post harvest, fresh slot)",
			land: state.LandView{Observed: true},
			want: KindPlant,
		},
		{
			name: "state=1, awaiting first water",
			land: state.LandView{Observed: true, FlowerID: 23001, State: 1},
			want: KindWater,
		},
		{
			name: "state=3, initial bloom ready",
			land: state.LandView{Observed: true, FlowerID: 23001, State: 3},
			want: KindHarvest,
		},
		{
			name: "state=2 + nextTime in past -> harvest",
			land: state.LandView{
				Observed: true, FlowerID: 23001, State: 2,
				NextTimeMs: now.Add(-1 * time.Minute).UnixMilli(),
			},
			want: KindHarvest,
		},
		{
			name: "state=2 + nextTime in future -> wait",
			land: state.LandView{
				Observed: true, FlowerID: 23001, State: 2,
				NextTimeMs: now.Add(time.Hour).UnixMilli(),
			},
			want: KindWait,
		},
		{
			name: "state=2 + nextTime unset -> wait (server hasn't scheduled regrow yet)",
			land: state.LandView{Observed: true, FlowerID: 23001, State: 2, NextTimeMs: 0},
			want: KindWait,
		},
		{
			name: "unrecognized state -> wait (don't act on what we don't understand)",
			land: state.LandView{Observed: true, FlowerID: 23001, State: 99},
			want: KindWait,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Recommend(tc.land, now)
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestFeatureRegistryEnrichesPlannedOps(t *testing.T) {
	harvest := landOp("usrLand.harvest", "farm.harvest", "harvest", "ready land", 1000, []int32{1001}, 0)
	if harvest.FeatureID != "plant.harvest" || harvest.Label != "收获" || harvest.Status != PlanStatusExecutable || !harvest.Executable || harvest.SyncOnly {
		t.Fatalf("harvest feature metadata = %+v", harvest)
	}

	weekly := markerOp(CategoryBasic, "basic.task.weekly", "claim", "weekly task rewards enabled", 620)
	if weekly.FeatureID != "basic.task_weekly" || weekly.Status != PlanStatusManaged || !weekly.Executable || len(weekly.BlockedReasons) != 0 {
		t.Fatalf("weekly feature metadata = %+v", weekly)
	}

	welfare := markerOp(CategoryBasic, "basic.welfare", "claim", "welfare enabled", 632)
	if welfare.FeatureID != "basic.welfare" || welfare.Status != PlanStatusAdapterMissing || welfare.Executable || len(welfare.BlockedReasons) == 0 {
		t.Fatalf("welfare feature metadata = %+v", welfare)
	}

	palace := markerOp(CategoryOrder, "order.palace", "finish", "palace orders enabled", 760)
	if palace.FeatureID != "order.palace" || palace.Status != PlanStatusSyncOnly || !palace.SyncOnly || palace.Executable {
		t.Fatalf("palace feature metadata = %+v", palace)
	}
}

func TestPlanOperationsExposeFeatureStatuses(t *testing.T) {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.WeeklyTaskEnabled = true
	policy.Basic.WelfareEnabled = true
	policy.Order.Palace.Enabled = true
	policy.Activity.Enabled = true
	policy.Activity.Modules["fishFun"].Enabled = true

	ops := PlanOperations(state.New(), policy, time.Now())
	weekly := findPlannedDomain(ops, "basic.task.weekly")
	if weekly == nil || weekly.Status != PlanStatusManaged || !weekly.Executable {
		t.Fatalf("weekly plan = %+v", weekly)
	}
	welfare := findPlannedDomain(ops, "basic.welfare")
	if welfare == nil || welfare.Status != PlanStatusAdapterMissing || len(welfare.BlockedReasons) == 0 {
		t.Fatalf("welfare plan = %+v", welfare)
	}
	palace := findPlannedDomain(ops, "order.palace")
	if palace == nil || palace.Status != PlanStatusSyncOnly || !palace.SyncOnly {
		t.Fatalf("palace plan = %+v", palace)
	}
	fish := findPlannedDomain(ops, "activity.fishFun")
	if fish == nil || fish.Status != PlanStatusAdapterMissing || len(fish.BlockedReasons) == 0 {
		t.Fatalf("fishFun plan = %+v", fish)
	}
}

func findPlannedDomain(ops []PlannedOp, domain string) *PlannedOp {
	for i := range ops {
		if ops[i].Domain == domain {
			return &ops[i]
		}
	}
	return nil
}

// applyLands stuffs raw LandView entries into a State for plan tests.
// Bypasses ApplyV because we want to control fields precisely.
func applyLands(s *state.State, lands map[int32]state.LandView) {
	// state.State has no setter; use ApplyV with a synthesized 100.0.1
	// roster shape. Each value is the LandView marshaled into the wire's
	// numeric-key shape.
	roster := map[string]any{}
	for id, l := range lands {
		entry := map[string]any{}
		if l.FlowerID != 0 {
			entry["0"] = l.FlowerID
		}
		entry["1"] = l.State
		entry["2"] = l.Lvl
		entry["3"] = l.HarvestCnt
		if l.NextTimeMs != 0 {
			entry["5"] = l.NextTimeMs
		}
		if l.PlantTimeMs != 0 {
			entry["7"] = l.PlantTimeMs
		}
		if !l.Observed {
			continue // skip unobserved entries entirely
		}
		if !l.IsPlanted() && l.State == 0 {
			roster[itoa(id)] = map[string]any{} // observed-empty
		} else {
			roster[itoa(id)] = entry
		}
	}
	s.ApplyVMap(map[string]any{
		"100": map[string]any{
			"0": map[string]any{"1": roster},
		},
	})
}

func itoa(n int32) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func setInventory(s *state.State, kv map[int32]int32) {
	cell := map[string]any{}
	for id, count := range kv {
		cell[itoa(id)] = count
	}
	s.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{"32": cell},
		},
	})
}

func setCultivations(s *state.State, kv map[int32]state.CultivateView) {
	cell := map[string]any{}
	for id, cv := range kv {
		cell[itoa(id)] = map[string]any{
			"2": cv.Lvl,
			"3": cv.CulTimeMs,
			"4": cv.Status,
			"5": cv.UTimeMs,
		}
	}
	s.ApplyVMap(map[string]any{
		"101": map[string]any{
			"0": cell,
		},
	})
}

func setFlowerOrders(s *state.State, orders map[int32][]state.FlowerRequire) {
	boxes := map[string]any{}
	for boxID, reqs := range orders {
		rawReqs := make([][]int32, 0, len(reqs))
		for _, req := range reqs {
			rawReqs = append(rawReqs, []int32{req.FlowerID, req.Count})
		}
		boxes[itoa(boxID)] = map[string]any{"2": rawReqs}
	}
	s.ApplyVMap(map[string]any{
		"105": map[string]any{
			"0": map[string]any{"1": boxes},
		},
	})
}

func setCustomerOrders(s *state.State, orders map[int32][]state.FlowerRequire) {
	orderMap := map[string]any{}
	for npcID, reqs := range orders {
		rawReqs := make([][]int32, 0, len(reqs))
		for _, req := range reqs {
			rawReqs = append(rawReqs, []int32{req.FlowerID, req.Count})
		}
		orderMap[itoa(npcID)] = map[string]any{"0": rawReqs, "1": npcID}
	}
	s.ApplyVMap(map[string]any{
		"109": map[string]any{
			"0": map[string]any{"1": orderMap},
		},
	})
}

func setMainTask(s *state.State, taskID, finished int32) {
	s.ApplyVMap(map[string]any{
		"22": map[string]any{
			"0": map[string]any{"1": taskID, "2": finished},
		},
	})
}

func setWaterDrops(s *state.State, count int) {
	setWaterDropsWithNext(s, count, 0)
}

func setWaterDropsWithNext(s *state.State, count int, nextMs int64) {
	s.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"32": map[string]any{"7": count},
				"33": map[string]any{"7": map[string]any{"1": 130, "5": nextMs}},
			},
		},
	})
}

func setNoble(s *state.State, vip int32) {
	s.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{"36": vip},
		},
	})
}

// TestPlan_HarvestPriorityWithOneKey verifies the highest-priority kind
// (harvest) takes precedence and uses the one-key RPC when policy says so.
func TestPlan_HarvestPriorityWithOneKey(t *testing.T) {
	s := state.New()
	now := time.Now()
	pastMs := now.Add(-time.Minute).UnixMilli()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},                                                // plant candidate
		1002: {Observed: true, FlowerID: 23001, State: 1},                     // water candidate
		1003: {Observed: true, FlowerID: 23001, State: 3},                     // harvest (initial bloom)
		1004: {Observed: true, FlowerID: 23001, State: 2, NextTimeMs: pastMs}, // harvest (regrow)
	})
	setInventory(s, map[int32]int32{23001: 100})
	setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.HarvestPreferOneKey = true

	op := Plan(s, policy, now)
	if op == nil {
		t.Fatal("expected an operation")
	}
	if op.Kind != "usrLand.harvestOneKey" {
		t.Errorf("kind=%q, want usrLand.harvestOneKey", op.Kind)
	}
	if len(op.LandIDs) != 2 {
		t.Errorf("expected 2 lands in batch, got %d (%v)", len(op.LandIDs), op.LandIDs)
	}
}

// TestPlan_HarvestSingleWhenOneKeyDisabled verifies policy.prefer_one_key=false
// falls through to per-land usrLand.harvest.
func TestPlan_HarvestSingleWhenOneKeyDisabled(t *testing.T) {
	s := state.New()
	now := time.Now()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 3},
		1002: {Observed: true, FlowerID: 23001, State: 3},
	})
	setInventory(s, map[int32]int32{23001: 100})
	setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.HarvestPreferOneKey = false

	op := Plan(s, policy, now)
	if op == nil {
		t.Fatal("expected an op")
	}
	if op.Kind != "usrLand.harvest" {
		t.Errorf("kind=%q want usrLand.harvest", op.Kind)
	}
	if len(op.LandIDs) != 1 {
		t.Errorf("single op should target one land, got %d", len(op.LandIDs))
	}
}

// TestPlan_PlantBatchHonorsMaxBatch verifies the batch is capped by max_batch
// and uses the lowest-stock cultivated flower.
func TestPlan_PlantBatchHonorsMaxBatch(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
		1002: {Observed: true},
		1003: {Observed: true},
		1004: {Observed: true},
	})
	setInventory(s, map[int32]int32{23001: 50, 23005: 3})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23005: {Lvl: 1, Status: 2},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeLowStock
	policy.Plant.PlantMaxBatch = 3

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.Kind != "usrLand.plantBatch" {
		t.Errorf("kind=%q want usrLand.plantBatch", op.Kind)
	}
	if op.FlowerID != 23005 {
		t.Errorf("expected lowest-stock flower 23005, got %d", op.FlowerID)
	}
	if len(op.LandIDs) != 3 {
		t.Errorf("expected 3 lands (max_batch cap), got %d", len(op.LandIDs))
	}
}

// TestPlan_PlantsCultivatedFlowerWithZeroInventory captures the protocol fact
// that planting does not consume 230xx flower inventory. A cultivated flower
// can be planted even when current owned count is zero.
func TestPlan_PlantsCultivatedFlowerWithZeroInventory(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{23005: 0})
	setCultivations(s, map[int32]state.CultivateView{23005: {Lvl: 1, Status: 2}})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeHighValue

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23005 {
		t.Fatalf("flower=%d, want zero-stock cultivated flower 23005", op.FlowerID)
	}
}

// TestPlan_RespectsPlantAllowList verifies plant.allowed_flower_ids restricts
// flower selection - even when a non-allowed flower has lower stock.
func TestPlan_RespectsPlantAllowList(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{
		23001: 100, // not allowed
		23005: 50,  // allowed but higher
	})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23005: {Lvl: 1, Status: 2},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeSelected
	policy.Plant.AllowedFlowerIds = []int32{23005}

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23005 {
		t.Errorf("got flower=%d, want 23005 (allow-list)", op.FlowerID)
	}
}

// TestPlan_DisabledPolicyReturnsNil verifies the master switch wins over
// everything else.
func TestPlan_DisabledPolicyReturnsNil(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 3},
	})
	setInventory(s, map[int32]int32{23001: 100})
	setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})

	policy := DefaultPolicy() // automation_enabled=false by default
	if op := Plan(s, policy, time.Now()); op != nil {
		t.Errorf("disabled policy should return nil, got %+v", op)
	}
}

// TestPlan_NilPolicyReturnsNil avoids panics when callers haven't loaded a
// policy yet.
func TestPlan_NilPolicyReturnsNil(t *testing.T) {
	s := state.New()
	if op := Plan(s, (*pb.Policy)(nil), time.Now()); op != nil {
		t.Errorf("nil policy should return nil")
	}
}

// TestPlan_WaterBatch verifies water-only state plans a batch.
func TestPlan_WaterBatch(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 1},
		1002: {Observed: true, FlowerID: 23001, State: 1},
		1003: {Observed: true, FlowerID: 23001, State: 1},
	})
	setInventory(s, map[int32]int32{23001: 100})
	setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})
	setWaterDrops(s, 100)

	policy := DefaultPolicy()
	policy.AutomationEnabled = true

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected water op")
	}
	if op.Kind != "usrLand.waterBatch" {
		t.Errorf("kind=%q want usrLand.waterBatch", op.Kind)
	}
	if len(op.LandIDs) != 3 {
		t.Errorf("got %d lands, want 3", len(op.LandIDs))
	}
}

func TestPlan_WaterOneKeyRequiresNobleAndPolicy(t *testing.T) {
	now := time.Now()
	newWaterState := func() *state.State {
		s := state.New()
		applyLands(s, map[int32]state.LandView{
			1001: {Observed: true, FlowerID: 23001, State: 1},
			1002: {Observed: true, FlowerID: 23001, State: 1},
		})
		setInventory(s, map[int32]int32{23001: 100})
		setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})
		setWaterDrops(s, 100)
		return s
	}

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.WaterPreferOneKeyIfNoble = true
	op := Plan(newWaterState(), policy, now)
	if op == nil {
		t.Fatal("expected water op")
	}
	if op.Kind == "usrLand.waterOneKey" {
		t.Fatalf("non-noble account planned one-key water: %+v", op)
	}

	nobleNoPolicy := newWaterState()
	setNoble(nobleNoPolicy, 1)
	policy.Plant.WaterPreferOneKeyIfNoble = false
	op = Plan(nobleNoPolicy, policy, now)
	if op == nil {
		t.Fatal("expected water op")
	}
	if op.Kind == "usrLand.waterOneKey" {
		t.Fatalf("noble account planned one-key water while policy is off: %+v", op)
	}

	nobleWithPolicy := newWaterState()
	setNoble(nobleWithPolicy, 1)
	policy.Plant.WaterPreferOneKeyIfNoble = true
	op = Plan(nobleWithPolicy, policy, now)
	if op == nil {
		t.Fatal("expected water op")
	}
	if op.Kind != "usrLand.waterOneKey" {
		t.Fatalf("kind=%q want usrLand.waterOneKey", op.Kind)
	}
}

func TestPlan_WaterAfterRecoveryTimestamp(t *testing.T) {
	s := state.New()
	now := time.Now()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 1},
		1002: {Observed: true, FlowerID: 23001, State: 1},
	})
	setInventory(s, map[int32]int32{23001: 100})
	setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})
	setWaterDropsWithNext(s, 0, now.Add(-time.Minute).UnixMilli())

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.MinWaterDrops = 0

	op := Plan(s, policy, now)
	if op == nil {
		t.Fatal("expected one water op after recovery timestamp")
	}
	if op.Kind != "usrLand.water" || len(op.LandIDs) != 1 {
		t.Fatalf("got %+v, want single water op", op)
	}
}

func TestPlan_WaterHonorsMinDrops(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 1},
		1002: {Observed: true, FlowerID: 23001, State: 1},
	})
	setInventory(s, map[int32]int32{23001: 100})
	setCultivations(s, map[int32]state.CultivateView{23001: {Lvl: 1, Status: 2}})
	setWaterDrops(s, 6)

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.MinWaterDrops = 5

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected one water op above the reserve threshold")
	}
	if len(op.LandIDs) != 1 {
		t.Fatalf("got %d lands, want exactly one usable drop beyond the reserve", len(op.LandIDs))
	}

	setWaterDrops(s, 5)
	if op := Plan(s, policy, time.Now()); op != nil {
		t.Fatalf("expected no water op at the reserve threshold, got %+v", op)
	}
}

func TestPlan_TaskPriorityPrioritizesOrderDeficit(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{23001: 100, 23005: 50})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23005: {Lvl: 1, Status: 2},
	})
	setFlowerOrders(s, map[int32][]state.FlowerRequire{
		1: {{FlowerID: 23005, Count: 80}},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeLowStock

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23005 {
		t.Fatalf("got flower=%d, want deficit flower 23005", op.FlowerID)
	}
}

func TestPlan_TaskPriorityIgnoresCustomerOrderDeficit(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{23001: 100, 23005: 50})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23005: {Lvl: 1, Status: 2},
	})
	setCustomerOrders(s, map[int32][]state.FlowerRequire{
		7: {{FlowerID: 23001, Count: 120}},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeLowStock

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23005 {
		t.Fatalf("got flower=%d, want low-stock flower 23005 instead of customer-order deficit", op.FlowerID)
	}
}

func TestPlan_TaskPriorityPrioritizesMainTaskFlower(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{23001: 100, 23058: 1})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23058: {Lvl: 1, Status: 2},
	})
	setMainTask(s, 10001, 1)

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeLowStock

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23058 {
		t.Fatalf("got flower=%d, want main-task flower 23058", op.FlowerID)
	}
}

func TestPlan_TaskPriorityUsesModeWithoutDeficit(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{23001: 100, 23004: 100, 23005: 100})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23004: {Lvl: 1, Status: 2},
		23005: {Lvl: 1, Status: 2},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeHighValue

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	want := highestValueFlower([]int32{23001, 23004, 23005})
	if op.FlowerID != want {
		t.Fatalf("got flower=%d, want highest value flower %d", op.FlowerID, want)
	}
}

func TestPlan_TaskPriorityCapsBatchByDeficit(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
		1002: {Observed: true},
		1003: {Observed: true},
	})
	setInventory(s, map[int32]int32{23005: 9})
	setCultivations(s, map[int32]state.CultivateView{
		23005: {Lvl: 1, Status: 2},
	})
	setFlowerOrders(s, map[int32][]state.FlowerRequire{
		1: {{FlowerID: 23005, Count: 10}},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeHighValue
	policy.Plant.PlantMaxBatch = 8

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.Kind != "usrLand.plant" {
		t.Fatalf("kind=%q want usrLand.plant for one missing flower", op.Kind)
	}
	if op.FlowerID != 23005 || len(op.LandIDs) != 1 {
		t.Fatalf("op=%+v, want one land planted with deficit flower 23005", op)
	}
}

func TestPlan_TaskPriorityPrioritizesZeroStockDeficit(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
		1002: {Observed: true},
	})
	setInventory(s, map[int32]int32{23007: 3000, 23008: 0})
	setCultivations(s, map[int32]state.CultivateView{
		23007: {Lvl: 1, Status: 2},
		23008: {Lvl: 1, Status: 2},
	})
	setFlowerOrders(s, map[int32][]state.FlowerRequire{
		6: {{FlowerID: 23008, Count: 2}},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeHighValue

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23008 {
		t.Fatalf("got flower=%d, want missing zero-stock order flower 23008", op.FlowerID)
	}
	if len(op.LandIDs) != 2 {
		t.Fatalf("lands=%v, want capped to deficit 2", op.LandIDs)
	}
}

func TestPlan_TaskPriorityCanBeDisabled(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
	})
	setInventory(s, map[int32]int32{23001: 1, 23005: 100})
	setCultivations(s, map[int32]state.CultivateView{
		23001: {Lvl: 1, Status: 2},
		23005: {Lvl: 1, Status: 2},
	})
	setFlowerOrders(s, map[int32][]state.FlowerRequire{
		1: {{FlowerID: 23005, Count: 120}},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.PlantingMode = PlantModeLowStock
	policy.Plant.TaskPriorityEnabled = false

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected plant op")
	}
	if op.FlowerID != 23001 {
		t.Fatalf("got flower=%d, want low-stock flower 23001 when task priority is disabled", op.FlowerID)
	}
}

func highestValueFlower(ids []int32) int32 {
	var best int32
	var bestGold int32
	var bestExp int32
	for _, id := range ids {
		info, _ := state.FlowerInfoByID(id)
		if best == 0 ||
			info.Gold > bestGold ||
			(info.Gold == bestGold && info.Experience > bestExp) ||
			(info.Gold == bestGold && info.Experience == bestExp && id < best) {
			best = id
			bestGold = info.Gold
			bestExp = info.Experience
		}
	}
	return best
}

func TestPlan_PlantSkippedWhenFlowerNotCultivated(t *testing.T) {
	s := state.New()
	applyLands(s, map[int32]state.LandView{
		1001: {Observed: true},
		1002: {Observed: true, FlowerID: 23001, State: 1},
	})
	setInventory(s, map[int32]int32{23005: 5})
	setCultivations(s, map[int32]state.CultivateView{23005: {Lvl: 1, Status: 1}})
	setWaterDrops(s, 100)

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.WaterEnabled = true

	op := Plan(s, policy, time.Now())
	if op == nil {
		t.Fatal("expected fallback to water op")
	}
	if op.Kind != "usrLand.water" && op.Kind != "usrLand.waterBatch" {
		t.Errorf("expected water op when flower is not cultivated; got %s", op.Kind)
	}
}
