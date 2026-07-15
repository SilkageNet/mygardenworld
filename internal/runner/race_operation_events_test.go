package runner

import (
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestRaceTakeOperationTargetSuffix(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		TaskID:   3036,
		FlowerID: 23001,
	}
	got := operationTargetSuffix(op)
	want := " 种植收获 · 白百合"
	if got != want {
		t.Fatalf("operationTargetSuffix = %q, want %q", got, want)
	}
}
