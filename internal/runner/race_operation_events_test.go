package runner

import (
	"strings"
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

func TestUnionFlowerTakeOperationTargetSuffix(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCFmlFlowerShareTake.String(),
		TargetUID: 77900091102484,
		TargetID:  2,
		FlowerID:  23011,
	}
	got := operationTargetSuffix(op)
	want := " (成员=77900091102484 槽位=2)"
	if got != want {
		t.Fatalf("operationTargetSuffix = %q, want %q", got, want)
	}
}

func TestUnionFlowerTakeMessageSuffix(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCFmlFlowerShareTake.String(),
		TargetUID: 77900091102484,
		TargetID:  2,
		FlowerID:  23011,
	}
	got := unionFlowerTakeMessageSuffix(op)
	if !strings.Contains(got, "(#23011)") || !strings.Contains(got, "成员=77900091102484") || !strings.Contains(got, "槽位=2") {
		t.Fatalf("unionFlowerTakeMessageSuffix = %q", got)
	}
}

func TestOpKindDescUnionFlowerTake(t *testing.T) {
	if got := opKindDesc(clientproto.RPCFmlFlowerShareTake.String()); got != "公会摸花" {
		t.Fatalf("opKindDesc(take)=%q, want 公会摸花", got)
	}
}
