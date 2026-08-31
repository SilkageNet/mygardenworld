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
		{ItemID: 1401, Count: 4},
		{ItemID: 1409, Count: 4},
		{ItemID: 1423, Count: 4},
		{ItemID: 1435, Count: 1},
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

func TestFmlPositionAllowsRaceDelete(t *testing.T) {
	if label := FmlPositionLabel(2); label != "副会长" {
		t.Fatalf("FmlPositionLabel(2)=%q want 副会长", label)
	}
	if label := FmlPositionLabel(999); label != "" {
		t.Fatalf("FmlPositionLabel(999)=%q want empty", label)
	}
	for _, position := range []int32{1, 2} {
		if !FmlPositionAllowsRaceDelete(position) {
			t.Fatalf("position %d should allow race task deletion", position)
		}
	}
	for _, position := range []int32{0, 3, 4, 5, 999} {
		if FmlPositionAllowsRaceDelete(position) {
			t.Fatalf("position %d must not allow race task deletion", position)
		}
	}
}

func TestFmlBuildOptionByID(t *testing.T) {
	video, ok := FmlBuildOptionByID(1)
	if !ok || video.Cost != 0 || video.ItemID != 0 || video.ShareID != 14 || video.Type != 1 || video.GroupDailyLimit != 1 || len(video.Rewards) != 1 || video.Rewards[0] != (ItemCount{ItemID: 1, Count: 10}) {
		t.Fatalf("video build option=%+v ok=%t", video, ok)
	}
	gold, ok := FmlBuildOptionByID(2)
	if !ok || gold.ItemID != 11 || gold.Cost <= 0 || gold.ShareID != 0 || gold.Type != 2 || gold.GroupDailyLimit != 5 {
		t.Fatalf("gold build option=%+v ok=%t", gold, ok)
	}
	diamond, ok := FmlBuildOptionByID(3)
	if !ok || diamond.ItemID != 1 || diamond.Cost <= 0 || diamond.ShareID != 0 || diamond.Type != 2 || diamond.GroupDailyLimit != 5 {
		t.Fatalf("diamond build option=%+v ok=%t", diamond, ok)
	}
}

func TestShareRewardConfigByID(t *testing.T) {
	pearl, ok := ShareRewardConfigByID(10)
	if !ok || pearl.Limit != 2 || pearl.LimitType != 1 || len(pearl.Rewards) != 1 || pearl.Rewards[0] != (ItemCount{ItemID: 1006, Count: 500}) {
		t.Fatalf("pearl share config=%+v ok=%t", pearl, ok)
	}
}

func TestFmlRaceTaskUpgradeCost(t *testing.T) {
	if cost, ok := FmlRaceTaskUpgradeCost(4001, 9); !ok || cost != 27 {
		t.Fatalf("FmlRaceTaskUpgradeCost(4001,9)=(%d,%t), want (27,true)", cost, ok)
	}
	if cost, ok := FmlRaceTaskUpgradeCost(1017, 5); !ok || cost != 9 {
		t.Fatalf("explicit upgradePoint cost=(%d,%t), want (9,true)", cost, ok)
	}
	if cost, ok := FmlRaceTaskUpgradeCost(99999, 9); ok || cost != 0 {
		t.Fatalf("unknown task upgrade cost=(%d,%t), want (0,false)", cost, ok)
	}
}

func TestFmlRaceBaseAndTotalTaskNum(t *testing.T) {
	if got := FmlRaceBaseTaskNum(0); got != 9 {
		t.Fatalf("default lvl1 base=%d, want 9", got)
	}
	if got := FmlRaceBaseTaskNum(3); got != 15 {
		t.Fatalf("乙级 base=%d, want 15", got)
	}
	if got := FmlRaceBaseTaskNum(4); got != 18 {
		t.Fatalf("甲级 base=%d, want 18", got)
	}
	if got := FmlRaceTotalTaskNum(0, 2); got != 0 {
		t.Fatalf("unknown raceLvl total=%d, want 0", got)
	}
	if got := FmlRaceTotalTaskNum(4, 2); got != 18 {
		t.Fatalf("total ignores buy=%d, want 18", got)
	}
	if got := FmlRaceTotalTaskNum(1, 0); got != 9 {
		t.Fatalf("total no buy=%d, want 9", got)
	}
}

func TestZooEventInfoDynamicReward(t *testing.T) {
	event, ok := ZooEventInfoByID(2096)
	if !ok {
		t.Fatal("ZooEventInfoByID(2096) ok=false")
	}
	if event.Name != "助人为乐" || event.Type != 2 || !event.HasReward1 {
		t.Fatalf("ZooEventInfoByID(2096)=%+v, want type=2 dynamic reward metadata", event)
	}
	if event.SharedID != 0 || event.HasReward2 || event.NoHandle || event.Result {
		t.Fatalf("ZooEventInfoByID(2096)=%+v, want no share, no alternate result", event)
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
	if cost.ItemID != 22006 || cost.Count != 6 || cost.Gold != 1400 {
		t.Fatalf("FlowerUpgradeCostForLevel(23006,4)=%+v, want item 22006 count 6 gold 1400", cost)
	}
	// Prefer per-flower gold when the row exists (cfg gold for lvl4 is 400).
	if cost.Gold == 400 {
		t.Fatalf("FlowerUpgradeCostForLevel(23006,4) used cfg gold=%d, want per-flower 1400", cost.Gold)
	}
}

func TestFlowerUpgradeCostForLevelScalesBaseRow(t *testing.T) {
	// Newer flowers publish a base c_flowerLvl row instead of one row per level.
	// The client scales that base gold by cfg(level)/cfg(1).
	tests := []struct {
		flowerID int32
		level    int32
		count    int32
		gold     int32
	}{
		{flowerID: 23590, level: 9, count: 120, gold: 129000},
		{flowerID: 23436, level: 9, count: 120, gold: 126000},
		{flowerID: 23526, level: 12, count: 500, gold: 486000},
	}
	for _, tt := range tests {
		cost, ok := FlowerUpgradeCostForLevel(tt.flowerID, tt.level)
		if !ok {
			t.Fatalf("FlowerUpgradeCostForLevel(%d, %d) ok=false", tt.flowerID, tt.level)
		}
		wantItem := tt.flowerID - 1000
		if cost.ItemID != wantItem || cost.Count != tt.count || cost.Gold != tt.gold {
			t.Fatalf("FlowerUpgradeCostForLevel(%d,%d)=%+v, want item %d count %d gold %d", tt.flowerID, tt.level, cost, wantItem, tt.count, tt.gold)
		}
	}
}

func TestFlowerLvlYieldByID(t *testing.T) {
	y1, ok := FlowerLvlYieldByID(23001, 1)
	if !ok || y1.CropGets != 2 || y1.Left != 1 || y1.Frequencys != 1 || y1.FlowersPerPlant() != 2 {
		t.Fatalf("FlowerLvlYieldByID(23001,1)=%+v ok=%v, want cropGets=2 left=1 frequencys=1", y1, ok)
	}
	y10, ok := FlowerLvlYieldByID(23001, 10)
	if !ok || y10.CropGets != 3 || y10.Left != 1 || y10.Frequencys != 3 || y10.FlowersPerPlant() != 9 {
		t.Fatalf("FlowerLvlYieldByID(23001,10)=%+v ok=%v, want cropGets=3 left=1 frequencys=3", y10, ok)
	}
	if _, ok := FlowerLvlYieldByID(23001, 0); ok {
		t.Fatal("FlowerLvlYieldByID(23001,0) should be false")
	}
	// 梦紫郁金香 has no per-flower c_flowerLvl yield rows; use c_flowerLvlCfg.
	y11, ok := FlowerLvlYieldByID(23436, 11)
	if !ok || y11.CropGets != 3 || y11.Left != 1 || y11.Frequencys != 4 || y11.FlowersPerPlant() != 12 {
		t.Fatalf("FlowerLvlYieldByID(23436,11)=%+v ok=%v, want cfg cropGets=3 left=1 frequencys=4", y11, ok)
	}
}

func TestFlowerLvlCDSeconds(t *testing.T) {
	cd, ok := FlowerLvlCDSeconds(23001, 1)
	if !ok || cd != 55 {
		t.Fatalf("FlowerLvlCDSeconds(23001,1)=%d ok=%v, want 55", cd, ok)
	}
	cd, ok = FlowerLvlCDSeconds(23078, 11)
	if !ok || cd != 2700 {
		t.Fatalf("FlowerLvlCDSeconds(23078,11)=%d ok=%v, want 2700", cd, ok)
	}
	if _, ok := FlowerLvlCDSeconds(23001, 0); ok {
		t.Fatal("FlowerLvlCDSeconds(23001,0) should be false")
	}
	// 花笼流芳 / 梦紫郁金香 only publish base c_flowerLvl(flowerId); client scales
	// with calFlowerLvlTime(base.cd, cfg(level).cd, cfg(1).cd).
	cd, ok = FlowerLvlCDSeconds(23331, 1)
	if !ok || cd != 3300 {
		t.Fatalf("FlowerLvlCDSeconds(23331,1)=%d ok=%v, want 3300", cd, ok)
	}
	cd, ok = FlowerLvlCDSeconds(23331, 11)
	if !ok || cd != 2475 {
		t.Fatalf("FlowerLvlCDSeconds(23331,11)=%d ok=%v, want 2475", cd, ok)
	}
	cd, ok = FlowerLvlCDSeconds(23436, 1)
	if !ok || cd != 3000 {
		t.Fatalf("FlowerLvlCDSeconds(23436,1)=%d ok=%v, want 3000", cd, ok)
	}
	cd, ok = FlowerLvlCDSeconds(23436, 20)
	if !ok || cd != 1750 {
		t.Fatalf("FlowerLvlCDSeconds(23436,20)=%d ok=%v, want 1750", cd, ok)
	}
}

func TestFlowerUpgradeCostForMaxLevel(t *testing.T) {
	if cost, ok := FlowerUpgradeCostForLevel(23006, 20); ok {
		t.Fatalf("FlowerUpgradeCostForLevel(23006,20)=(%+v,true), want false", cost)
	}
}

func TestFlowerArtRecipeByID(t *testing.T) {
	tests := []struct {
		artID     int32
		vaseID    int32
		level     int32
		saleValue int32
		flowers   []int32
	}{
		{artID: 300208, vaseID: 3002, level: 8, saleValue: 170, flowers: []int32{23008, 23007, 23005}},
		// This higher-id recipe catches accidental re-application of the old
		// row/suffix offset. Its decoded values are already wire-ready.
		{artID: 301612, vaseID: 3016, level: 30, saleValue: 320, flowers: []int32{23070, 23075, 23003}},
	}
	for _, tt := range tests {
		recipe, ok := FlowerArtRecipeByID(tt.artID)
		if !ok {
			t.Fatalf("FlowerArtRecipeByID(%d) ok=false", tt.artID)
		}
		if recipe.VaseID != tt.vaseID || recipe.Level != tt.level || recipe.SaleValue != tt.saleValue || len(recipe.Flowers) != len(tt.flowers) {
			t.Fatalf("FlowerArtRecipeByID(%d)=%+v", tt.artID, recipe)
		}
		for i := range tt.flowers {
			if recipe.Flowers[i] != tt.flowers[i] {
				t.Fatalf("FlowerArtRecipeByID(%d).Flowers[%d]=%d want %d", tt.artID, i, recipe.Flowers[i], tt.flowers[i])
			}
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

func TestMainTaskFlowerTargetsDistinguishHarvestAndCultivate(t *testing.T) {
	if flowerID, target, ok := MainTaskFlowerTarget(10001); !ok || flowerID != 23001 || target != 4 {
		t.Fatalf("MainTaskFlowerTarget(10001)=(%d,%d,%t)", flowerID, target, ok)
	}
	if _, _, ok := MainTaskCultivateTarget(10001); ok {
		t.Fatal("harvest task must not be treated as cultivation")
	}
	if flowerID, target, ok := MainTaskCultivateTarget(40001); !ok || flowerID != 23003 || target != 1 {
		t.Fatalf("MainTaskCultivateTarget(40001)=(%d,%d,%t)", flowerID, target, ok)
	}
	if _, _, ok := MainTaskFlowerTarget(40001); ok {
		t.Fatal("cultivation task must not be treated as flower inventory demand")
	}
}

func TestFlowerMaxLevel(t *testing.T) {
	if got := FlowerMaxLevel(); got < 6 {
		t.Fatalf("FlowerMaxLevel()=%d want at least 6", got)
	}
}

func TestPlayerMaxLevel(t *testing.T) {
	if got := PlayerMaxLevel(); got != 65 {
		t.Fatalf("PlayerMaxLevel()=%d want 65", got)
	}
}

func TestPlayerLevelExpRequired(t *testing.T) {
	if got, ok := PlayerLevelExpRequired(1); !ok || got != 40 {
		t.Fatalf("PlayerLevelExpRequired(1)=(%d,%t) want (40,true)", got, ok)
	}
	if got, ok := PlayerLevelExpRequired(10); !ok || got != 12000 {
		t.Fatalf("PlayerLevelExpRequired(10)=(%d,%t) want (12000,true)", got, ok)
	}
	if got, ok := PlayerLevelExpRequired(65); ok || got != 0 {
		t.Fatalf("PlayerLevelExpRequired(65)=(%d,%t) want (0,false)", got, ok)
	}
}

func TestExperienceToNextLevel(t *testing.T) {
	remaining, required, maxed := ExperienceToNextLevel(1, 10)
	if remaining != 30 || required != 40 || maxed {
		t.Fatalf("ExperienceToNextLevel(1,10)=(%d,%d,%t) want (30,40,false)", remaining, required, maxed)
	}
	remaining, required, maxed = ExperienceToNextLevel(1, 40)
	if remaining != 0 || required != 40 || maxed {
		t.Fatalf("ExperienceToNextLevel(1,40)=(%d,%d,%t) want (0,40,false)", remaining, required, maxed)
	}
	remaining, required, maxed = ExperienceToNextLevel(65, 0)
	if remaining != 0 || required != 0 || !maxed {
		t.Fatalf("ExperienceToNextLevel(65,0)=(%d,%d,%t) want (0,0,true)", remaining, required, maxed)
	}
}

func TestLandUnlockOpenLevelKnown(t *testing.T) {
	if level, ok := LandUnlockOpenLevel(1025); !ok || level != 13 {
		t.Fatalf("LandUnlockOpenLevel(1025)=(%d,%t), want (13,true)", level, ok)
	}
}

func TestLandUnlockOpenLevelUnknown(t *testing.T) {
	if level, ok := LandUnlockOpenLevel(1999); ok || level != 0 {
		t.Fatalf("LandUnlockOpenLevel(1999)=(%d,%t), want (0,false)", level, ok)
	}
}
