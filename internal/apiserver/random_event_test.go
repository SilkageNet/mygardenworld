package apiserver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestPendingRandomEventsRemainVisibleWhenAutomationIsDisabled(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6004":{"0":6004,"1":2,"2":60040901}}}}}`))
	tasks := buildPendingTasksAtPolicy(s, time.Now(), false)
	var found *pb.PendingTaskView
	for _, task := range tasks {
		if task.GetCategory() == "地图随机事件" && task.GetId() == "6004" {
			found = task
			break
		}
	}
	if found == nil || found.GetStatus() != pb.PlanStatus_PLAN_STATUS_MANAGED ||
		!strings.Contains(found.GetTitle(), "位置 2") || !strings.Contains(found.GetTitle(), "自动处理已关闭") {
		t.Fatalf("disabled random event task=%+v", found)
	}
}

func TestPendingRandomEventsExposeMalformedAndUnsafeDiagnostics(t *testing.T) {
	t.Run("malformed map", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":[]}}}`))
		var found *pb.PendingTaskView
		for _, task := range buildPendingTasksAtPolicy(s, time.Now(), true) {
			if task.GetCategory() == "地图随机事件" {
				found = task
				break
			}
		}
		if found == nil || found.GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED || !strings.Contains(found.GetTitle(), "数据异常") {
			t.Fatalf("malformed task=%+v", found)
		}
	})

	t.Run("unsafe entry", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":1,"2":60080101}}}}}`))
		var found *pb.PendingTaskView
		for _, task := range buildPendingTasksAtPolicy(s, time.Now(), true) {
			if task.GetCategory() == "地图随机事件" {
				found = task
				break
			}
		}
		if found == nil || found.GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED || !strings.Contains(found.GetTitle(), "posIdx") {
			t.Fatalf("unsafe task=%+v", found)
		}
	})
}
