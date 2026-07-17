package state

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDessertCatalogConfigExactInvariants(t *testing.T) {
	config, ok := DessertCatalogConfig()
	if !ok {
		t.Fatal("DessertCatalogConfig ok=false")
	}
	if config.TmpType != 5601 || config.Name != "香卉甜糕" || config.EnergyItemID != 1342 || config.CurrencyItemID != 1343 ||
		config.PointItemID != 1344 || config.RemoveItemID != 1345 || config.SelectItemID != 1346 || config.RewardBoxItemID != 1347 ||
		config.InitialEnergy != 100 || config.TaskType != 18 || config.CelebrityReward != (ItemCount{ItemID: 1342, Count: 20}) {
		t.Fatalf("config=%+v", config)
	}
	if !reflect.DeepEqual(config.Multipliers, []int32{1, 5, 10, 25, 100}) ||
		!reflect.DeepEqual(config.UnlockScores, []int32{0, 4000, 24000, 64000, 160000}) ||
		!reflect.DeepEqual(config.FirstDrops, []int32{1, 1, 2, 3, 4}) || len(config.Levels) != 11 {
		t.Fatalf("mode/levels=%+v", config)
	}
	wantScores := []int32{0, 5, 10, 15, 25, 40, 50, 60, 80, 100, 150}
	wantResultScores := []int32{0, 0, 0, 0, 1, 2, 4, 8, 12, 16, 20}
	wantScales := []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1, 0.6, 0.7, 0.75, 0.9, 1}
	wantBoxes := []int32{0, 0, 0, 0, 0, 0, 1, 1, 2, 3, 4}
	for index, level := range config.Levels {
		if level.Level != int32(index+1) || level.Score != wantScores[index] || level.ResultScore != wantResultScores[index] ||
			level.Scale != wantScales[index] {
			t.Fatalf("level[%d]=%+v", index, level)
		}
		if wantBoxes[index] == 0 && len(level.Reward) != 0 {
			t.Fatalf("level[%d] unexpected reward=%v", index, level.Reward)
		}
		if wantBoxes[index] > 0 && !reflect.DeepEqual(level.Reward, []ItemCount{{ItemID: 1347, Count: wantBoxes[index]}}) {
			t.Fatalf("level[%d] reward=%v", index, level.Reward)
		}
	}

	config.Multipliers[0] = 999
	config.Levels[6].Reward[0].Count = 999
	again, ok := DessertCatalogConfig()
	if !ok || again.Multipliers[0] != 1 || again.Levels[6].Reward[0].Count != 1 {
		t.Fatalf("catalog defensive copy failed: %+v", again)
	}
}

func TestParseDessertTemplateTaskGroups(t *testing.T) {
	raw := json.RawMessage(`[{"5":[[1,18,1,null,"1342,100",1],[2,18,2,7,"1342,100",1]],"6":1}]`)
	groups, ok := ParseDessertTemplateTaskGroups(raw)
	if !ok || len(groups) != 1 || groups[0].TaskIndex != 0 || len(groups[0].Tasks) != 2 {
		t.Fatalf("groups=(%+v,%t)", groups, ok)
	}
	if groups[0].Tasks[0].HasParam || groups[0].Tasks[0].Title != "完成每日任务1次" || !groups[0].Tasks[0].CatalogKnown ||
		!groups[0].Tasks[1].HasParam || groups[0].Tasks[1].Param != 7 || groups[0].Tasks[1].Title != "完成每日任务2次" {
		t.Fatalf("tasks=%+v", groups[0].Tasks)
	}
	groups[0].Tasks[0].Reward[0].Count = 999
	again, _ := ParseDessertTemplateTaskGroups(raw)
	if again[0].Tasks[0].Reward[0].Count != 100 {
		t.Fatalf("task parser leaked backing storage: %+v", again)
	}

	for _, malformed := range []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`[{"5":{}}]`),
		json.RawMessage(`[{"5":[[1,18,1,null,"1342,100"],[1,18,2,null,"1342,100"]]}]`),
		json.RawMessage(`[{"5":[[1,18,1,null,"bad"]]}]`),
	} {
		if got, valid := ParseDessertTemplateTaskGroups(malformed); valid || got != nil {
			t.Fatalf("ParseDessertTemplateTaskGroups(%s)=(%+v,%t)", malformed, got, valid)
		}
	}
}
