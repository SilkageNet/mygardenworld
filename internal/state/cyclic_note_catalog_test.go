package state

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCyclicNoteCatalogConfig(t *testing.T) {
	cfg, ok := CyclicNoteCatalogConfig()
	if !ok {
		t.Fatal("CyclicNoteCatalogConfig ok=false")
	}
	if cfg.TmpType != 4002 || cfg.Name != "花笺集芳" || cfg.CurrencyItemID != 1107 || cfg.TaskSlotCount != 3 {
		t.Fatalf("CyclicNoteCatalogConfig=%+v", cfg)
	}
}

func TestCyclicNoteTaskInfoByID(t *testing.T) {
	tests := []struct {
		id       int32
		taskType int32
		target   int32
		title    string
		reward   ItemCount
		cost     ItemCount
	}{
		{id: 4003, taskType: 3001, target: 80, title: "去种植任意鲜花80次", reward: ItemCount{ItemID: 1107, Count: 4}, cost: ItemCount{ItemID: 1, Count: 36}},
		{id: 2001, taskType: 3015, target: 135, title: "在花架出售135件花艺品", reward: ItemCount{ItemID: 1107, Count: 2}, cost: ItemCount{ItemID: 1, Count: 18}},
		{id: 2007, taskType: 3016, target: 25, title: "完成25次顾客订单", reward: ItemCount{ItemID: 1107, Count: 2}, cost: ItemCount{ItemID: 1, Count: 18}},
		{id: 1005, taskType: 1009, target: 65, title: "累计完成65次居民订单", reward: ItemCount{ItemID: 1107, Count: 1}, cost: ItemCount{ItemID: 1, Count: 9}},
		{id: 1006, taskType: 1010, target: 3, title: "累计采集珍珠雇佣3次", reward: ItemCount{ItemID: 1107, Count: 1}, cost: ItemCount{ItemID: 1, Count: 9}},
	}
	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			info := CyclicNoteTaskInfoByID(test.id)
			if !info.CatalogKnown || info.TaskID != test.id || info.TaskType != test.taskType || info.Param != 0 || info.Target != test.target || info.Title != test.title {
				t.Fatalf("CyclicNoteTaskInfoByID(%d)=%+v", test.id, info)
			}
			if !reflect.DeepEqual(info.Reward, []ItemCount{test.reward}) || !reflect.DeepEqual(info.FinishCost, []ItemCount{test.cost}) {
				t.Fatalf("CyclicNoteTaskInfoByID(%d) reward=%v finishCost=%v", test.id, info.Reward, info.FinishCost)
			}
		})
	}
}

func TestCyclicNoteTaskInfoUnknownAndDefensive(t *testing.T) {
	unknown := CyclicNoteTaskInfoByID(999999)
	if unknown.TaskID != 999999 || unknown.CatalogKnown {
		t.Fatalf("unknown task=%+v", unknown)
	}

	first := CyclicNoteTaskInfoByID(4003)
	first.Reward[0].Count = 999
	first.FinishCost[0].Count = 999
	again := CyclicNoteTaskInfoByID(4003)
	if again.Reward[0].Count != 4 || again.FinishCost[0].Count != 36 {
		t.Fatalf("task catalog returned shared slices: %+v", again)
	}
}

func TestParseCyclicNoteTemplateBoxes(t *testing.T) {
	raw := json.RawMessage(`[[1,60,"1,80;1002,350"],[2,120,"1,200;1001,600"],[3,265,"1,600;21541,1"]]`)
	boxes, ok := ParseCyclicNoteTemplateBoxes(raw)
	if !ok {
		t.Fatal("ParseCyclicNoteTemplateBoxes ok=false")
	}
	want := []CyclicNoteMilestoneInfo{
		{Index: 1, Target: 60, Reward: []ItemCount{{ItemID: 1, Count: 80}, {ItemID: 1002, Count: 350}}},
		{Index: 2, Target: 120, Reward: []ItemCount{{ItemID: 1, Count: 200}, {ItemID: 1001, Count: 600}}},
		{Index: 3, Target: 265, Reward: []ItemCount{{ItemID: 1, Count: 600}, {ItemID: 21541, Count: 1}}},
	}
	if !reflect.DeepEqual(boxes, want) {
		t.Fatalf("boxes=%+v want %+v", boxes, want)
	}

	boxes[0].Reward[0].Count = 999
	again, _ := ParseCyclicNoteTemplateBoxes(raw)
	if again[0].Reward[0].Count != 80 {
		t.Fatalf("template parser returned shared reward storage: %+v", again[0])
	}
}

func TestParseCyclicNoteTemplateBoxesObservedEmpty(t *testing.T) {
	for _, raw := range []json.RawMessage{json.RawMessage(`null`), json.RawMessage(`[]`)} {
		boxes, ok := ParseCyclicNoteTemplateBoxes(raw)
		if !ok || len(boxes) != 0 {
			t.Fatalf("ParseCyclicNoteTemplateBoxes(%s)=(%v,%t), want empty,true", raw, boxes, ok)
		}
	}
}

func TestParseCyclicNoteTemplateBoxesRejectsMalformed(t *testing.T) {
	tests := []string{
		`{}`,
		`[[1,60]]`,
		`[["1",60,"1,80"]]`,
		`[[1,60.0,"1,80"]]`,
		`[[1,60,"1,80"],[1,120,"1,200"]]`,
		`[[1,120,"1,80"],[2,60,"1,200"]]`,
		`[[1,60,"01,80"]]`,
		`[[1,60,"1,080"]]`,
		`[[1,60,"1,80;"]]`,
		`[[1,60,"1,0"]]`,
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if got, ok := ParseCyclicNoteTemplateBoxes(json.RawMessage(raw)); ok || got != nil {
				t.Fatalf("ParseCyclicNoteTemplateBoxes(%s)=(%v,%t), want nil,false", raw, got, ok)
			}
		})
	}
}

func TestParseCyclicNoteRewardTextAggregatesAndSorts(t *testing.T) {
	got, ok := parseCyclicNoteRewardText("2,3;1,2;2,4")
	want := []ItemCount{{ItemID: 1, Count: 2}, {ItemID: 2, Count: 7}}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCyclicNoteRewardText=%v,%t want %v,true", got, ok, want)
	}
}
