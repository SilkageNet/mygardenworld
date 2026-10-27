package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func TestCyclicNoteScopedOtherModulesPreservePolicyAndStop(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		taskID, taskType, target int32
	}{
		{"customer", 2007, cyclicNoteTaskTypeCustomerOrder, 25},
		{"rack", 2001, cyclicNoteTaskTypeFlowerRack, 135},
		{"pearl", 1006, cyclicNoteTaskTypePearlHire, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().Add(2 * time.Second)
			s := cyclicNotePlannerState(t, now, 2, []any{tc.taskID, nil, nil}, map[string]any{itoa32(tc.taskID): tc.target - 1}, map[string]any{}, 0, []any{})
			applyMap(t, s, map[string]any{
				"7":   map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"23005": 10, "300208": 20, "1003": 3}}},
				"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
				"109": map[string]any{"0": map[string]any{"1": map[string]any{"7": map[string]any{"0": [][]int32{{23005, 1}}, "1": 7}}}},
				"115": map[string]any{"0": map[string]any{"1": map[string]any{"2": int64(0), "3": nil, "4": 0, "9": int64(1)}}, "1": map[string]any{"5": map[string]any{}}},
				"24":  map[string]any{"0": map[string]any{"0": int64(9001)}, "1": []any{map[string]any{"0": int64(9001), "1": int64(2001)}}},
				"28":  map[string]any{"5": []any{map[string]any{"0": int64(2001), "1": "safe", "4": 12}}},
			})
			applyMap(t, s, map[string]any{"115": map[string]any{"5": map[string]any{"2001": int64(0)}}})
			p := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
			p.Plant.Planting.AutoEnabled = false
			p.Order.Customer.Enabled = false
			p.Order.FlowerArt.SellEnabled = false
			p.Basic.Pearl.AutoHireEnabled = false
			p.Basic.Pearl.MaxHireTicketUsage = 2
			before := proto.Clone(p)
			op := requireSingleCyclicNoteBusinessOp(t, BuildPlan(s, p, now).Operations, tc.taskType)
			if op.CyclicNoteDrive == nil || !runnableBusinessOperation(op) || CyclicNoteOperationCount(&op) != 1 {
				t.Fatalf("unbounded or missing task authorization: %+v", op)
			}
			if !proto.Equal(before, p) {
				t.Fatal("changed daily policy")
			}
			if tc.taskType == cyclicNoteTaskTypePearlHire {
				scoped, err := CyclicNotePearlPolicy(s, p, &op, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := ValidateSafePearlHire(s, scoped, &op, now); err != nil {
					t.Fatal(err)
				}
				for _, limit := range []func(*pb.PearlPolicy){func(p *pb.PearlPolicy) { p.MaxHireTicketUsage = 0 }, func(p *pb.PearlPolicy) { p.MaxHireLevel = 1 }} {
					limited := proto.Clone(p).(*pb.Policy)
					limit(limited.Basic.Pearl)
					safe, _ := CyclicNotePearlPolicy(s, limited, &op, now)
					if err := ValidateSafePearlHire(s, safe, &op, now); err == nil {
						t.Fatal("activity bypassed hire safety")
					}
				}
			}
			applyMap(t, s, map[string]any{"23": map[string]any{"3": map[string]any{"9001|0": map[string]any{"3": map[string]any{itoa32(tc.taskID): tc.target}, "5": map[string]any{}}}}})
			for _, op := range BuildPlan(s, p, now).Operations {
				if cyclicNoteOperationMatches(tc.taskType, op.Kind) && runnableBusinessOperation(op) {
					t.Fatalf("extra work after task completion: %+v", op)
				}
			}
		})
	}
}

func TestCyclicNoteScopedPlantStopsAtDeficitWithoutChangingDailyPolicy(t *testing.T) {
	now := time.Now()
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 78}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"4": 10000, "32": map[string]any{"23001": 10}}},
		"100": map[string]any{"1": emptyLands(10)}, "101": map[string]any{"0": cultivate(23001)},
	})
	p := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	p.Plant.Planting.AutoEnabled = false
	p.Plant.Planting.AutoHarvestEnabled = false
	before := proto.Clone(p)
	op := requireSingleCyclicNoteBusinessOp(t, BuildPlan(s, p, now).Operations, cyclicNoteTaskTypePlantAny)
	if len(op.LandIDs) != 2 || op.CyclicNoteDrive == nil || !runnableBusinessOperation(op) {
		t.Fatalf("wanted exactly two authorized plantings: %+v", op)
	}
	if err := ValidateCyclicNoteTaskOperation(s, p, &op, now); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(before, p) {
		t.Fatal("daily policy mutated")
	}
	applyMap(t, s, map[string]any{"23": map[string]any{"3": map[string]any{"9001|0": map[string]any{"3": map[string]any{"4003": 80}, "5": map[string]any{}}}}})
	if err := ValidateCyclicNoteTaskOperation(s, p, &op, now); err == nil {
		t.Fatal("completed stale operation accepted")
	}
	for _, next := range BuildPlan(s, p, now).Operations {
		if isPlantOperation(next.Kind) {
			t.Fatalf("continued planting after completion: %+v", next)
		}
	}
}

func TestCyclicNoteScopedResidentIgnoresDailyPolicyButPreservesServerLimits(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	for _, mode := range []string{"disabled", "daily cap", "hard cap", "server rejection", "no stock", "video"} {
		t.Run(mode, func(t *testing.T) {
			s := cyclicNotePlannerState(t, now, 2, []any{1005, nil, nil}, map[string]any{"1005": 64}, map[string]any{}, 0, []any{})
			finished, stock, video := int32(254), int32(10), int32(0)
			if mode == "hard cap" {
				finished = state.ResidentOrderNormalDailyMax()
				if finished <= 0 {
					t.Fatal("missing catalog hard cap")
				}
			}
			if mode == "no stock" {
				stock = 0
			}
			if mode == "video" {
				video = 1
			}
			applyMap(t, s, map[string]any{
				"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": stock}}},
				"105": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 1, "2": [][]int32{{23005, 1}}, "3": video}}}},
				"124": map[string]any{"0": map[string]any{"20260724": map[string]any{"1": 20260724, "9": finished}}},
			})
			if mode == "server rejection" {
				s.MarkResidentOrderDailyLimitReached(now)
			}
			p := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
			p.Plant.Planting.AutoEnabled = false
			p.Order.Resident.NormalEnabled = mode != "disabled"
			p.Order.Resident.NormalDailyLimit = 160
			before := proto.Clone(p)
			var selected *PlannedOp
			for _, op := range BuildPlan(s, p, now).Operations {
				if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && runnableBusinessOperation(op) {
					cp := op
					selected = &cp
				}
			}
			want := mode == "disabled" || mode == "daily cap"
			if (selected != nil) != want {
				t.Fatalf("mode=%s selected=%+v want=%t", mode, selected, want)
			}
			if !proto.Equal(before, p) {
				t.Fatal("daily limits mutated")
			}
			if selected == nil {
				return
			}
			if selected.CyclicNoteDrive == nil || CyclicNoteOperationCount(selected) != 1 {
				t.Fatalf("not a bounded task action: %+v", selected)
			}
			applyMap(t, s, map[string]any{"23": map[string]any{"3": map[string]any{"9001|0": map[string]any{"3": map[string]any{"1005": 65}, "5": map[string]any{}}}}})
			for _, op := range BuildPlan(s, p, now).Operations {
				if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && runnableBusinessOperation(op) {
					t.Fatal("continued after completing activity")
				}
			}
		})
	}
}

func TestCyclicNoteScopedAuthorizationRevoked(t *testing.T) {
	for _, change := range []string{"activity off", "drive off", "automation off", "task replaced", "progress changed", "phase ended", "oversized"} {
		t.Run(change, func(t *testing.T) {
			now := time.Now()
			s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 78}, map[string]any{}, 0, []any{})
			p := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
			a := cyclicNoteTaskActionDemands(s, p, now)[0]
			op := PlannedOp{Kind: clientproto.RPCUsrLandPlant.String(), LandIDs: []int32{1001}, DemandID: a.Demand.ID, CyclicNoteDrive: &a.Drive}
			if err := ValidateCyclicNoteTaskOperation(s, p, &op, now); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "activity off":
				p.Activity.CyclicNote.Enabled = false
			case "drive off":
				p.Activity.CyclicNote.SatisfyTasks = false
			case "automation off":
				p.AutomationEnabled = false
			case "task replaced":
				applyMap(t, s, map[string]any{"23": map[string]any{"0": map[string]any{"9001": map[string]any{"14": map[string]any{"105": map[string]any{"0": []any{1005, nil, nil}}}}}}})
			case "progress changed":
				applyMap(t, s, map[string]any{"23": map[string]any{"3": map[string]any{"9001|0": map[string]any{"3": map[string]any{"4003": 79}}}}})
			case "phase ended":
				now = now.Add(time.Hour)
			case "oversized":
				op.LandIDs = []int32{1001, 1002, 1003}
			}
			if err := ValidateCyclicNoteTaskOperation(s, p, &op, now); err == nil {
				t.Fatal("stale authorization accepted")
			}
		})
	}
}
