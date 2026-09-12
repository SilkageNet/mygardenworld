package automation

import (
	"encoding/json"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestPassClaimOperationsEnterWhenUnobserved(t *testing.T) {
	s := state.New()
	task := &pb.BasicTaskPolicy{FlowerPassTaskRewardEnabled: true}
	ops := flowerPassClaimOperations(s, task)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerPassEnter.String() || !ops[0].Executable {
		t.Fatalf("ops=%+v", ops)
	}
}

func TestPassClaimOperationsTaskDoneAndFreeRecv(t *testing.T) {
	s := state.New()
	applyPassV(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 2, "3": 0, "6": map[string]any{"1": []any{}}}},
			"1": map[string]any{"15": map[string]any{"1": 15, "5": []any{1001}, "6": map[string]any{"2010_0": 3}, "8": map[string]any{}}},
		},
	})
	task := &pb.BasicTaskPolicy{
		FlowerPassTaskRewardEnabled: true,
		FlowerPassRewardEnabled:     true,
	}
	ops := flowerPassClaimOperations(s, task)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerPassTaskDone.String() {
		t.Fatalf("want taskDone first, got %+v", ops)
	}
	if ops[0].TargetID != 15 || ops[0].ItemID != 1001 {
		t.Fatalf("taskDone targets %+v", ops[0])
	}

	applyPassV(t, s, map[string]any{
		"131": map[string]any{"1": map[string]any{"15": map[string]any{"8": map[string]any{"1001": 1}}}},
	})
	s.NoteFlowerPassEnterSynced()
	ops = flowerPassClaimOperations(s, task)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerPassRecvOneKey.String() {
		t.Fatalf("want recvOneKey, got %+v", ops)
	}
	if ops[0].TargetID != 15 {
		t.Fatalf("recvOneKey targets %+v", ops[0])
	}
}

func TestPassClaimOperationsEnterWhenRwdMapMissing(t *testing.T) {
	s := state.New()
	// Partial delta: lvl/exp without rwdMap (field 6) — must not plan free recv.
	applyPassV(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 5, "3": 10}},
			"1": map[string]any{"15": map[string]any{"1": 15, "5": []any{1001}, "8": map[string]any{"1001": 1}}},
		},
	})
	task := &pb.BasicTaskPolicy{FlowerPassRewardEnabled: true}
	ops := flowerPassClaimOperations(s, task)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerPassEnter.String() {
		t.Fatalf("want enter to sync rwdMap, got %+v", ops)
	}
	if bid, levels := s.ReadyFlowerPassFreeLevels(); bid != 15 || len(levels) != 0 {
		t.Fatalf("ready without rwdMap bid=%d levels=%v", bid, levels)
	}
}

func TestPassClaimOperationsEnterBeforeFreeRecvEvenIfRwdMapPresent(t *testing.T) {
	s := state.New()
	applyPassV(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 2, "6": map[string]any{"free": []any{}}}},
		},
	})
	task := &pb.BasicTaskPolicy{FlowerPassRewardEnabled: true}
	ops := flowerPassClaimOperations(s, task)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerPassEnter.String() {
		t.Fatalf("want enter before free recv, got %+v", ops)
	}
	s.NoteFlowerPassEnterSynced()
	ops = flowerPassClaimOperations(s, task)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerPassRecvOneKey.String() {
		t.Fatalf("want recvOneKey after enter, got %+v", ops)
	}
}

func TestPassClaimOperationsDisabled(t *testing.T) {
	s := state.New()
	applyPassV(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 2, "6": map[string]any{"1": []any{}}}},
			"1": map[string]any{"15": map[string]any{"5": []any{1001}, "6": map[string]any{"2010_0": 3}, "8": map[string]any{}}},
		},
	})
	if ops := flowerPassClaimOperations(s, &pb.BasicTaskPolicy{}); len(ops) != 0 {
		t.Fatalf("disabled should plan nothing, got %+v", ops)
	}
}

func applyPassV(t *testing.T, s *state.State, top map[string]any) {
	t.Helper()
	raw, err := json.Marshal(top)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s.ApplyV(raw)
}
