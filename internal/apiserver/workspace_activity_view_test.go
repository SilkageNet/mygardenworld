package apiserver

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func TestFmlRaceProtoSurfacesPoolProgressAndSkipReason(t *testing.T) {
	view := state.FmlRaceView{Tasks: []state.FmlRaceTaskView{{
		MsId:      81,
		TaskId:    3030,
		TaskType:  3030,
		Score:     24,
		TargetCnt: 10,
		FinishCnt: 3,
	}}}
	policy := &pb.UnionRacePolicy{
		AvoidProgressedTasks: proto.Bool(true),
		TaskTypePriority:     map[int32]int32{3030: 4},
	}

	got := fmlRaceProto(view, state.New(), policy, 0, time.Now(), automation.RaceModuleGates{})
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
}
