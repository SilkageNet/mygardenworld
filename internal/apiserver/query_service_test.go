package apiserver

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestBuildLandViewsResolvesStaticLandStatus(t *testing.T) {
	lands := map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 1},
	}

	got := buildLandViews(lands, 1, time.Unix(0, 0))
	if len(got) < 64 {
		t.Fatalf("expected full static land roster, got %d", len(got))
	}

	byID := make(map[int32]string, len(got))
	observed := make(map[int32]bool, len(got))
	for _, land := range got {
		byID[land.GetLandId()] = land.GetLandStatus()
		observed[land.GetLandId()] = land.GetObserved()
	}

	if byID[1001] != "opened" || !observed[1001] {
		t.Fatalf("land 1001 = status %q observed %v, want opened true", byID[1001], observed[1001])
	}
	if byID[1002] != "wasteland" || observed[1002] {
		t.Fatalf("land 1002 = status %q observed %v, want wasteland false", byID[1002], observed[1002])
	}
	if byID[1058] != "locked" || observed[1058] {
		t.Fatalf("land 1058 = status %q observed %v, want locked false", byID[1058], observed[1058])
	}
}

func TestBuildPendingTasksGroupsTrackedTaskSources(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001":  2,
			"23058":  1,
			"300208": 1,
		}, "34": 14}},
		"105": map[string]any{
			"0": map[string]any{"1": map[string]any{
				"2": map[string]any{"2": [][]int32{{23001, 5}}},
			}},
		},
		"109": map[string]any{
			"0": map[string]any{"1": map[string]any{
				"1": map[string]any{"0": 2, "1": 300208, "2": 3, "3": 1},
			}},
		},
		"22": map[string]any{
			"0": map[string]any{"1": 10001, "2": 1},
			"1": map[string]any{
				"3": map[string]any{"102": 1},
				"100": map[string]any{
					"101": map[string]any{"0": 101, "1": 5, "2": 2, "4": 0},
					"102": map[string]any{"0": 102, "1": 1, "2": 1, "4": 1},
				},
			},
		},
		"119": map[string]any{"3": map[string]any{"20001": 1, "20002": 1, "20003": 1}},
		"129": map[string]any{"0": map[string]any{"1": map[string]any{
			"6002": map[string]any{"0": 6002, "1": 0, "2": 60020601},
		}}},
	})

	tasks := buildPendingTasks(st)
	byCategory := map[string]int{}
	for _, task := range tasks {
		byCategory[task.GetCategory()]++
	}
	if byCategory["居民订单"] != 1 || byCategory["顾客订单"] != 1 || byCategory["主线任务"] != 1 || byCategory["日常任务"] != 1 || byCategory["地图事件"] != 1 {
		t.Fatalf("task categories = %+v, want all tracked categories once", byCategory)
	}

	var flowerReq, artReq, vaseReq, recipeReq *pb.RequirementView
	for _, task := range tasks {
		for _, req := range task.GetRequirements() {
			switch req.GetItemId() {
			case 23001:
				flowerReq = req
			case 300208:
				artReq = req
			case 3069:
				vaseReq = req
			case 23071:
				recipeReq = req
			}
		}
	}
	if flowerReq == nil || !flowerReq.GetPlantingRelevant() || flowerReq.GetMissing() != 3 {
		t.Fatalf("flower requirement = %+v, want planting-relevant missing 3", flowerReq)
	}
	if artReq == nil || artReq.GetPlantingRelevant() || artReq.GetMissing() != 2 {
		t.Fatalf("art requirement = %+v, want non-planting missing 2", artReq)
	}
	if vaseReq == nil || vaseReq.GetPlantingRelevant() || vaseReq.GetMissing() != 2 {
		t.Fatalf("vase requirement = %+v, want non-planting missing 2", vaseReq)
	}
	if recipeReq == nil || !recipeReq.GetPlantingRelevant() || recipeReq.GetMissing() != 2 {
		t.Fatalf("recipe requirement = %+v, want planting-relevant missing 2", recipeReq)
	}
}
