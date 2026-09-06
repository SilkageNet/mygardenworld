package automation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func TestRaceUpgradeBudgetAndIndependentSwitch(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		enabled, autoComplete bool
		budget                int64
		balance               int32
		allowed               bool
	}{
		{"independent", true, false, 27, 27, true},
		{"auto complete", true, true, 27, 27, true},
		{"disabled", false, true, 100, 100, false},
		{"zero prohibits", true, false, 0, 100, false},
		{"budget insufficient", true, false, 26, 100, false},
		{"balance insufficient", true, false, 100, 26, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
			s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1},"117":{"5":4},"114":[{"0":1,"4":4001,"6":[23001],"10":9,"12":999,"14":0}],"110":{"42":{"3":0,"4":10,"7":{"0":1,"1":4001,"2":10,"3":1,"4":[23001]}}},"116":[{"0":999,"1":42,"3":0,"4":10}]}}`))
			s.ApplyVMap(map[string]any{"7": map[string]any{"0": map[string]any{"41": tc.balance}}})
			p := testRacePolicy()
			p.UpgradeTask, p.AutoEnableModules, p.MaxSpendDiamond = tc.enabled, tc.autoComplete, tc.budget
			now := time.Now()
			ops := unionRaceOperations(s, p, 999, now, raceGatesOn())
			annotateOperationGates(s, ops, now)
			allowed := false
			for _, op := range ops {
				if op.Kind != clientproto.RPCFmlRaceUpgradeTask.String() {
					continue
				}
				allowed = operationConsumesQueueBudget(op)
				if allowed {
					if err := ValidateRaceUpgrade(s, p, &op, now); err != nil {
						t.Fatal(err)
					}
					changed := proto.Clone(p).(*pb.UnionRacePolicy)
					changed.UpgradeTask = false
					if ValidateRaceUpgrade(s, changed, &op, now) == nil {
						t.Fatal("disabled switch passed final validation")
					}
					op.RaceBatchID++
					if ValidateRaceUpgrade(s, p, &op, now) == nil {
						t.Fatal("changed batch passed validation")
					}
				}
			}
			if allowed != tc.allowed {
				t.Fatalf("allowed=%v, want %v; ops=%+v", allowed, tc.allowed, ops)
			}
			other := []PlannedOp{{Kind: clientproto.RPCFmlBld.String(), Executable: true, DiamondCost: 1}}
			annotateOperationGates(s, other, now)
			if other[0].Executable {
				t.Fatal("unrelated paid operation was enabled")
			}
		})
	}
}

func TestRaceUpgradeRejectsMissingEvidence(t *testing.T) {
	if ValidateRaceUpgrade(state.New(), &pb.UnionRacePolicy{Enabled: true, UpgradeTask: true, MaxSpendDiamond: 100}, &PlannedOp{Kind: clientproto.RPCFmlRaceUpgradeTask.String(), DiamondCost: 1}, time.Now()) == nil {
		t.Fatal("unguarded upgrade allowed")
	}
}

func TestRaceAutoDeleteExplainsScoreAndPermissionGates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		position int32
		limit    int32
		want     string
	}{
		{"eligible without auto-complete", 2, 29, "1 个任务符合"},
		{"lowered threshold", 2, 28, "暂无可删除"},
		{"ordinary member", 3, 29, "无删除权限"},
		{"zero threshold", 2, 0, "上限为 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			applyRaceState(s, [][5]int32{{1, 3036, 29, 0, 0}})
			applyRaceDeletePosition(s, tc.position)
			p := testRacePolicy()
			p.AutoEnableModules = false
			p.DeleteLowScoreTask = true
			p.DeleteTaskMaxScore = tc.limit
			if got := RaceAutoDeleteStatus(s, p, time.Now()); !strings.Contains(got, tc.want) {
				t.Fatalf("status=%q, want %q", got, tc.want)
			}
		})
	}
}
