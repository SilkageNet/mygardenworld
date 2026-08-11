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

func TestRaceTaskSuccessMessage(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceFinishTask.String(),
		TaskID:   3036,
		FlowerID: 23001,
	}
	if got := raceTaskSuccessMessage(op); got != "种植收获 · 白百合" {
		t.Fatalf("raceTaskSuccessMessage = %q, want 种植收获 · 白百合", got)
	}
	if got := raceTaskSuccessMessage(&automation.PlannedOp{Kind: clientproto.RPCFmlRaceTakeTask.String()}); got != "完成" {
		t.Fatalf("raceTaskSuccessMessage empty = %q, want 完成", got)
	}
}

func TestRaceOperationEventLabels(t *testing.T) {
	cases := map[string]string{
		clientproto.RPCFmlRaceGetTaskList.String():  "同步竞赛任务",
		clientproto.RPCFmlRaceEnter.String():        "进入公会竞赛",
		clientproto.RPCFmlRaceTakeTask.String():     "接取竞赛任务",
		clientproto.RPCFmlRaceFinishTask.String():   "完成竞赛任务",
		clientproto.RPCFmlRaceUpgradeTask.String():  "升级竞赛任务",
		clientproto.RPCFmlRaceDelTask.String():      "删除竞赛任务",
		clientproto.RPCFmlRaceGiveUpTask.String():   "放弃竞赛任务",
	}
	for kind, want := range cases {
		if got := operationEventLabel(&automation.PlannedOp{Kind: kind}); got != want {
			t.Fatalf("operationEventLabel(%s)=%q, want %q", kind, got, want)
		}
	}
}

func TestRaceEventMetadata(t *testing.T) {
	cases := []struct {
		kind     string
		category string
		domain   string
		label    string
	}{
		{kind: "race_task_sync", category: "race", domain: "union.race.sync", label: "同步竞赛任务"},
		{kind: "race_enter", category: "race", domain: "union.race.enter", label: "进入公会竞赛"},
		{kind: "race_task_taken", category: "race", domain: "union.race.take", label: "接取竞赛任务"},
		{kind: "race_task_finished", category: "race", domain: "union.race.finish", label: "完成竞赛任务"},
		{kind: "race_task_upgraded", category: "race", domain: "union.race.upgrade", label: "升级竞赛任务"},
		{kind: "race_task_deleted", category: "race", domain: "union.race.delete", label: "删除竞赛任务"},
		{kind: "race_task_given_up", category: "race", domain: "union.race.giveUp", label: "放弃竞赛任务"},
	}
	for _, tt := range cases {
		if got := eventCategory(tt.kind); got != tt.category {
			t.Fatalf("eventCategory(%q)=%q, want %q", tt.kind, got, tt.category)
		}
		if got := eventDomain(tt.kind); got != tt.domain {
			t.Fatalf("eventDomain(%q)=%q, want %q", tt.kind, got, tt.domain)
		}
		if got := eventLabel(tt.kind); got != tt.label {
			t.Fatalf("eventLabel(%q)=%q, want %q", tt.kind, got, tt.label)
		}
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
