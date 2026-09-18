package apiserver

import (
	"encoding/json"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func TestFmlRaceProtoSurfacesPoolProgressAndSkipReason(t *testing.T) {
	now := time.Now()
	view := state.FmlRaceView{Tasks: []state.FmlRaceTaskView{{
		MsId:       81,
		TaskId:     3030,
		TaskType:   3030,
		Score:      24,
		TargetCnt:  10,
		FinishCnt:  3,
		AppearTime: now.Add(time.Hour).UnixMilli(),
	}}}
	policy := &pb.UnionRacePolicy{
		AvoidProgressedTasks: proto.Bool(true),
		TaskTypePriority:     map[int32]int32{3030: 4},
	}

	got := fmlRaceProto(view, state.New(), policy, 0, now, automation.RaceModuleGates{})
	if len(got.GetTasks()) != 1 {
		t.Fatalf("tasks=%d, want 1", len(got.GetTasks()))
	}
	task := got.GetTasks()[0]
	if task.GetTargetCnt() != 10 || task.GetFinishCnt() != 3 {
		t.Fatalf("progress=%d/%d, want 3/10", task.GetFinishCnt(), task.GetTargetCnt())
	}
	if task.GetTakeSkipReason() != "已有进度（3/10）" {
		t.Fatalf("take skip reason=%q", task.GetTakeSkipReason())
	}
	if task.GetAppearTimeMs() != view.Tasks[0].AppearTime {
		t.Fatal("refresh deadline must remain available independently of the restriction")
	}
}

func TestFmlRaceProtoSeparatesTaskOccupancyFromUpgradeMember(t *testing.T) {
	now := time.Now()
	deadline := now.Add(time.Hour)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"101":{"0":{"23281":{"1":23281,"2":1,"4":2}}}}`))
	for _, tc := range []struct {
		name       string
		upgradeUID int64
		claimedUID int64
		want       string
	}{
		{"other upgrade member", 99, 0, "他人已升级"},
		{"no upgrade member", 0, 0, ""},
		{"own upgrade", 42, 0, ""},
		{"claimed without upgrade member", 0, 99, "已被接取"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := state.FmlRaceView{Tasks: []state.FmlRaceTaskView{{
				MsId: 39, TaskType: 3036, ParamID: 23281, Score: 42, TargetCnt: 560, IsUpgrade: 1,
				UID: tc.claimedUID, UpgradeUid: tc.upgradeUID, AppearTime: deadline.UnixMilli(),
			}}}
			policy := &pb.UnionRacePolicy{MinTaskScore: 22, ExcludeOthersUpgradeTask: true, TaskTypePriority: map[int32]int32{3036: 10}}
			for _, at := range []time.Time{now, deadline, deadline.Add(time.Second)} {
				want := tc.want
				if want == "" && at.Before(deadline) {
					want = "冷却中，" + deadline.Local().Format("15:04:05") + " 后可接"
				}
				got := fmlRaceProto(view, s, policy, 42, at, automation.RaceModuleGates{}).GetTasks()[0]
				if got.GetTakeSkipReason() != want || got.GetAppearTimeMs() != deadline.UnixMilli() {
					t.Fatalf("at %s: reason=%q deadline=%d", at, got.GetTakeSkipReason(), got.GetAppearTimeMs())
				}
			}
		})
	}
}

func TestFmlRaceProtoSurfacesManualDeleteAvailability(t *testing.T) {
	now := time.Now()
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"1":{"0":999,"1":42,"2":2}}}`))
	view := state.FmlRaceView{
		Observed:      true,
		BatchStatus:   1,
		BatchStartMs:  now.Add(-time.Hour).UnixMilli(),
		BatchEndMs:    now.Add(time.Hour).UnixMilli(),
		TasksObserved: true,
		Tasks: []state.FmlRaceTaskView{
			{MsId: 81, TaskId: 3030, TaskType: 3030, Score: 300},
			{MsId: 82, TaskId: 3030, TaskType: 3030, Score: 300, UID: 123},
		},
	}
	got := fmlRaceProto(view, s, &pb.UnionRacePolicy{Enabled: true}, 999, now, automation.RaceModuleGates{})
	if !got.GetTasks()[0].GetDeleteAllowed() || got.GetTasks()[0].GetDeleteBlockedReason() != "" {
		t.Fatalf("ready delete state=%+v", got.GetTasks()[0])
	}
	if got.GetTasks()[1].GetDeleteAllowed() || got.GetTasks()[1].GetDeleteBlockedReason() != "任务已被成员接取" {
		t.Fatalf("claimed delete state=%+v", got.GetTasks()[1])
	}
}
