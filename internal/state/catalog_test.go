package state

import (
	"encoding/json"
	"testing"
)

func TestCultivateCostKnownFlower(t *testing.T) {
	costs, ok := CultivateCost(23006)
	if !ok {
		t.Fatal("CultivateCost(23006) ok=false")
	}
	want := []ItemCount{
		{ItemID: 1475, Count: 4},
		{ItemID: 1483, Count: 4},
		{ItemID: 1497, Count: 4},
		{ItemID: 1509, Count: 1},
	}
	if len(costs) != len(want) {
		t.Fatalf("len(costs)=%d want %d", len(costs), len(want))
	}
	for i := range want {
		if costs[i] != want[i] {
			t.Fatalf("costs[%d]=%+v want %+v", i, costs[i], want[i])
		}
	}
}

func TestCultivateCostUnknownFlower(t *testing.T) {
	if costs, ok := CultivateCost(99999); ok || costs != nil {
		t.Fatalf("CultivateCost(99999)=(%+v,%t), want (nil,false)", costs, ok)
	}
}

func TestCultivateCostReturnsCopy(t *testing.T) {
	costs, ok := CultivateCost(23006)
	if !ok {
		t.Fatal("CultivateCost(23006) ok=false")
	}
	costs[0].Count = 99
	again, _ := CultivateCost(23006)
	if again[0].Count != 4 {
		t.Fatalf("CultivateCost returned shared slice; got count %d", again[0].Count)
	}
}

func TestItemInfoByIDIncludesClientDetails(t *testing.T) {
	item, ok := ItemInfoByID(7)
	if !ok {
		t.Fatal("ItemInfoByID(7) ok=false")
	}
	if item.Name != "水滴" || item.Type != 0 || item.Color != 2 {
		t.Fatalf("ItemInfoByID(7)=%+v", item)
	}
}

func TestItemInfoByIDReturnsCopy(t *testing.T) {
	item, ok := ItemInfoByID(954)
	if !ok {
		t.Fatal("ItemInfoByID(954) ok=false")
	}
	if len(item.Items) == 0 || len(item.Items[0].Extra) == 0 {
		t.Fatalf("ItemInfoByID(954) missing item contents: %+v", item)
	}
	item.Items[0].Extra[0] = 0
	again, _ := ItemInfoByID(954)
	if again.Items[0].Extra[0] == 0 {
		t.Fatal("ItemInfoByID returned shared nested slice")
	}
}

func TestStaticTableAndRow(t *testing.T) {
	table, ok := StaticTableByName("c_task_dly")
	if !ok {
		t.Fatal("StaticTableByName(c_task_dly) ok=false")
	}
	if table.Columns["desc"] == "" || table.Columns["type"] == "" || len(table.Rows) == 0 {
		t.Fatalf("StaticTableByName(c_task_dly) missing decoded data: %+v", table.Columns)
	}

	rowJSON, ok := StaticRow("c_task_dly", 30160001)
	if !ok {
		t.Fatal("StaticRow(c_task_dly, 30160001) ok=false")
	}
	var row struct {
		Desc string `json:"desc"`
		Type int32  `json:"type"`
	}
	if err := json.Unmarshal(rowJSON, &row); err != nil {
		t.Fatal(err)
	}
	if row.Desc != "完成${value}次顾客订单" || row.Type == 0 {
		t.Fatalf("StaticRow(c_task_dly, 30160001)=%+v", row)
	}
}

func TestFmlBuildOptionByID(t *testing.T) {
	video, ok := FmlBuildOptionByID(1)
	if !ok || video.Cost != 0 || video.ItemID != 0 {
		t.Fatalf("video build option=%+v ok=%t", video, ok)
	}
	gold, ok := FmlBuildOptionByID(2)
	if !ok || gold.ItemID != 11 || gold.Cost <= 0 {
		t.Fatalf("gold build option=%+v ok=%t", gold, ok)
	}
	diamond, ok := FmlBuildOptionByID(3)
	if !ok || diamond.ItemID != 1 || diamond.Cost <= 0 {
		t.Fatalf("diamond build option=%+v ok=%t", diamond, ok)
	}
}

func TestFlowerBouquetItemID(t *testing.T) {
	tests := map[int32]int32{
		23006: 22006,
		23008: 22008,
		23009: 22009,
		23999: 0,
	}
	for flowerID, want := range tests {
		if got := FlowerBouquetItemID(flowerID); got != want {
			t.Fatalf("FlowerBouquetItemID(%d)=%d want %d", flowerID, got, want)
		}
	}
}

func TestFlowerUpgradeCostForLevel(t *testing.T) {
	cost, ok := FlowerUpgradeCostForLevel(23006, 4)
	if !ok {
		t.Fatal("FlowerUpgradeCostForLevel(23006, 4) ok=false")
	}
	if cost.ItemID != 22006 || cost.Count != 6 || cost.Gold != 1460 {
		t.Fatalf("FlowerUpgradeCostForLevel(23006,4)=%+v, want item 22006 count 6 gold 1460", cost)
	}
}

func TestFlowerUpgradeCostForMaxLevel(t *testing.T) {
	if cost, ok := FlowerUpgradeCostForLevel(23006, 20); ok {
		t.Fatalf("FlowerUpgradeCostForLevel(23006,20)=(%+v,true), want false", cost)
	}
}

func TestFlowerArtRecipeByID(t *testing.T) {
	recipe, ok := FlowerArtRecipeByID(300208)
	if !ok {
		t.Fatal("FlowerArtRecipeByID(300208) ok=false")
	}
	want := []int32{23008, 23007, 23005}
	if recipe.VaseID != 3002 || recipe.Level != 8 || recipe.SaleValue != 236 || len(recipe.Flowers) != len(want) {
		t.Fatalf("FlowerArtRecipeByID(300208)=%+v", recipe)
	}
	for i := range want {
		if recipe.Flowers[i] != want[i] {
			t.Fatalf("recipe.Flowers[%d]=%d want %d", i, recipe.Flowers[i], want[i])
		}
	}
}

func TestTaskTitles(t *testing.T) {
	if got := MainTaskTitle(350001); got != "35.累计上架10件花艺品" {
		t.Fatalf("MainTaskTitle(350001)=%q", got)
	}
	if got := DailyTaskTitle(30160001, 5); got != "完成5次顾客订单" {
		t.Fatalf("DailyTaskTitle(30160001,5)=%q", got)
	}
}

func TestFlowerMaxLevel(t *testing.T) {
	if got := FlowerMaxLevel(); got < 6 {
		t.Fatalf("FlowerMaxLevel()=%d want at least 6", got)
	}
}

func TestLandUnlockOpenLevelKnown(t *testing.T) {
	if level, ok := LandUnlockOpenLevel(1025); !ok || level != 42 {
		t.Fatalf("LandUnlockOpenLevel(1025)=(%d,%t), want (42,true)", level, ok)
	}
}

func TestLandUnlockOpenLevelUnknown(t *testing.T) {
	if level, ok := LandUnlockOpenLevel(1999); ok || level != 0 {
		t.Fatalf("LandUnlockOpenLevel(1999)=(%d,%t), want (0,false)", level, ok)
	}
}
