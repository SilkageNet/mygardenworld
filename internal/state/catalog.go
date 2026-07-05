package state

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Refresh with: go run ./cmd/gardencatalog --mini tmp/mini
//
//go:embed catalog_data.json
var catalogDataJSON []byte

// ItemStack describes an item/count tuple from client config. Some reward
// tables carry extra integers such as weights; those are preserved in Extra.
type ItemStack struct {
	ItemID int32   `json:"item_id"`
	Count  int32   `json:"count,omitempty"`
	Extra  []int32 `json:"extra,omitempty"`
}

// ItemInfo is the selected c_item row data used by the daemon and CLI.
type ItemInfo struct {
	ID          int32       `json:"id"`
	Name        string      `json:"name,omitempty"`
	ShortName   string      `json:"short_name,omitempty"`
	DisplayName string      `json:"display_name,omitempty"`
	Color       int32       `json:"color,omitempty"`
	Type        int32       `json:"type,omitempty"`
	UseType     int32       `json:"use_type,omitempty"`
	Items       []ItemStack `json:"items,omitempty"`
	Restore     []ItemStack `json:"restore,omitempty"`
}

// FlowerInfo is the selected c_flower row data.
type FlowerInfo struct {
	ID            int32       `json:"id"`
	SeedID        int32       `json:"seed_id,omitempty"`
	EliteID       int32       `json:"elite_id,omitempty"`
	Sort          int32       `json:"sort,omitempty"`
	Experience    int32       `json:"experience,omitempty"`
	Gold          int32       `json:"gold,omitempty"`
	CultivateCost []ItemStack `json:"cultivate_cost,omitempty"`
}

// FarmLandInfo is the selected c_farmLand row data.
type FarmLandInfo struct {
	ID        int32   `json:"id"`
	OpenLevel int32   `json:"open_level,omitempty"`
	Cost      []int32 `json:"cost,omitempty"`
	Wasteland []int32 `json:"wasteland,omitempty"`
}

type gameCatalog struct {
	Tables    map[string]StaticTable `json:"tables"`
	Items     map[int32]ItemInfo     `json:"items"`
	Flowers   map[int32]FlowerInfo   `json:"flowers"`
	FarmLands map[int32]FarmLandInfo `json:"farm_lands"`
}

// StaticTable is a fully decoded client config table. It keeps every decoded
// table from g-data available even when there is no hand-written typed view.
type StaticTable struct {
	Columns map[string]string          `json:"columns"`
	Rows    map[string]json.RawMessage `json:"rows"`
}

var catalog gameCatalog

func init() {
	if err := json.Unmarshal(catalogDataJSON, &catalog); err != nil {
		panic(err)
	}
}

// LandUnlockCostGold is the legacy observed gold cost for usrLand.unlockLand.
// Prefer FarmLandInfo when showing static client configuration.
const LandUnlockCostGold int32 = 800

// ItemCount describes an item requirement or reward count.
type ItemCount struct {
	ItemID int32
	Count  int32
}

// FlowerUpgradeCost describes the client-visible cost for one cultivate.upgrade.
type FlowerUpgradeCost struct {
	ItemID int32
	Count  int32
	Gold   int32
}

// FlowerArtRecipe describes the flower-art craft input from c_flowerArt.
type FlowerArtRecipe struct {
	ArtID     int32
	VaseID    int32
	Flowers   []int32
	Level     int32
	SaleValue int32
}

// RoadGrowTask describes a growth-road task row that the daemon can evaluate.
type RoadGrowTask struct {
	TaskID      int32
	Title       string
	TargetLevel int32
}

// WeeklyTask describes one c_task_week row that can be evaluated from
// namespace 22.100 progress and recv maps.
type WeeklyTask struct {
	TaskID       int32
	Title        string
	ProgressType int32
	Target       int32
}

// AchievementTask describes one c_task_ach row that can be evaluated from
// namespace 22.2 progress and recv maps.
type AchievementTask struct {
	TaskID       int32
	Title        string
	GroupID      int32
	StageIndex   int32
	ProgressType int32
	Target       int32
}

// StoryMainSectionInfo describes the currently unlockable story section.
type StoryMainSectionInfo struct {
	Chapter     int32
	SectionIdx  int32
	SectionID   int32
	ChapterName string
	SectionName string
	Cost        []ItemCount
}

// ZooEventInfo describes one c_zooEvent row relevant to conservative event
// automation.
type ZooEventInfo struct {
	EventID    int32
	Name       string
	Type       int32
	SharedID   int32
	NoHandle   bool
	Result     bool
	Reward1    []ItemCount
	Reward2    []ItemCount
	HasReward2 bool
	Text       string
}

// FmlBuildOption describes one c_fmlBld donation/build option.
type FmlBuildOption struct {
	ID         int32
	Name       string
	ItemID     int32
	Cost       int32
	DailyLimit int32
}

// ItemInfoByID returns a defensive copy of a c_item catalog row.
func ItemInfoByID(id int32) (ItemInfo, bool) {
	item, ok := catalog.Items[id]
	if !ok {
		return ItemInfo{}, false
	}
	return cloneItemInfo(item), true
}

// FlowerInfoByID returns a defensive copy of a c_flower catalog row.
func FlowerInfoByID(id int32) (FlowerInfo, bool) {
	flower, ok := catalog.Flowers[id]
	if !ok {
		return FlowerInfo{}, false
	}
	return cloneFlowerInfo(flower), true
}

// FarmLandInfoByID returns a defensive copy of a c_farmLand catalog row.
func FarmLandInfoByID(id int32) (FarmLandInfo, bool) {
	land, ok := catalog.FarmLands[id]
	if !ok {
		return FarmLandInfo{}, false
	}
	return cloneFarmLandInfo(land), true
}

// AllFarmLands returns the known c_farmLand rows sorted by land id. The client
// config also contains a sentinel -1 row; it is intentionally omitted.
func AllFarmLands() []FarmLandInfo {
	out := make([]FarmLandInfo, 0, len(catalog.FarmLands))
	for _, land := range catalog.FarmLands {
		if land.ID <= 0 {
			continue
		}
		out = append(out, cloneFarmLandInfo(land))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// StaticTableByName returns a defensive copy of a decoded client config table.
func StaticTableByName(name string) (StaticTable, bool) {
	table, ok := catalog.Tables[name]
	if !ok {
		return StaticTable{}, false
	}
	return cloneStaticTable(table), true
}

// StaticRow returns one raw decoded row from a client config table.
func StaticRow(tableName string, id int32) (json.RawMessage, bool) {
	table, ok := catalog.Tables[tableName]
	if !ok {
		return nil, false
	}
	row, ok := table.Rows[strconv.FormatInt(int64(id), 10)]
	if !ok {
		return nil, false
	}
	return cloneRaw(row), true
}

// FlowerRackSellDurationMs returns the configured shelf sale window from
// c_flowerRack.$sellTime. The client config stores this value in seconds.
func FlowerRackSellDurationMs() int64 {
	raw, ok := StaticRow("c_flowerRack", -1)
	if !ok {
		return 0
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	var seconds int64
	if rawSellTime, ok := row["$sellTime"]; ok {
		_ = json.Unmarshal(rawSellTime, &seconds)
	}
	if seconds <= 0 {
		return 0
	}
	return seconds * 1000
}

// AllFlowerArtRecipes returns every decoded c_flowerArt recipe. The list is
// sorted from higher-value art to lower-value art for automation choices.
func AllFlowerArtRecipes() []FlowerArtRecipe {
	table, ok := StaticTableByName("c_flowerArt")
	if !ok {
		return nil
	}
	out := make([]FlowerArtRecipe, 0, len(table.Rows))
	for idStr := range table.Rows {
		id := atoiCatalogID(idStr)
		if id == 0 {
			continue
		}
		recipe, ok := FlowerArtRecipeByID(id)
		if ok {
			out = append(out, recipe)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SaleValue != out[j].SaleValue {
			return out[i].SaleValue > out[j].SaleValue
		}
		return out[i].ArtID > out[j].ArtID
	})
	return out
}

func FlowerName(id int32) string {
	return ItemName(id)
}

// FlowerValue returns the static gold value used to rank plant choices.
func FlowerValue(id int32) int32 {
	flower, ok := catalog.Flowers[id]
	if !ok {
		return 0
	}
	return flower.Gold
}

func isFlowerItemID(id int32) bool {
	return int(id) >= FlowerSeedLow && int(id) < FlowerSeedHigh
}

// IsFlowerItemID reports whether an item id is in the flower inventory range.
func IsFlowerItemID(id int32) bool {
	return isFlowerItemID(id)
}

// FlowerMaxLevel returns the configured cultivation upgrade cap. The
// c_flowerLvl sentinel row carries the current client-side maximum.
func FlowerMaxLevel() int32 {
	raw, ok := StaticRow("c_flowerLvl", -1)
	if !ok {
		return 0
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	var max int32
	if rawMax, ok := row["$lvlMax"]; ok {
		_ = json.Unmarshal(rawMax, &max)
	}
	return max
}

// MainTaskFlowerRequirement returns the flower id and missing count for a
// current main task when the static task row points at a flower item.
func MainTaskFlowerRequirement(taskID, finished int32) (flowerID, missing int32, ok bool) {
	flowerID, target, ok := MainTaskFlowerTarget(taskID)
	if !ok || target <= finished {
		return 0, 0, false
	}
	return flowerID, target - finished, true
}

// MainTaskFlowerTarget returns the flower item and target count for a main
// task row when the task is an explicit flower collection requirement.
func MainTaskFlowerTarget(taskID int32) (flowerID, target int32, ok bool) {
	raw, ok := StaticRow("c_task_main", taskID)
	if !ok {
		return 0, 0, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return 0, 0, false
	}
	var param int32
	if rawParam, ok := row["param"]; ok {
		_ = json.Unmarshal(rawParam, &param)
	}
	if !isFlowerItemID(param) {
		return 0, 0, false
	}
	if rawValue, ok := row["value"]; ok {
		if json.Unmarshal(rawValue, &target) != nil {
			var values []int32
			if json.Unmarshal(rawValue, &values) == nil && len(values) > 0 {
				target = values[0]
			}
		}
	}
	if target <= 0 {
		return 0, 0, false
	}
	return param, target, true
}

// MainTaskTitle returns the client-visible description for a main task.
func MainTaskTitle(taskID int32) string {
	return taskTitleFromTable("c_task_main", taskID, 0)
}

// DailyTaskTitle returns the client-visible description for a daily task.
func DailyTaskTitle(taskID, target int32) string {
	return taskTitleFromTable("c_task_dly", taskID, target)
}

// WeeklyTaskTitle returns the client-visible description for a weekly task.
func WeeklyTaskTitle(taskID, target int32) string {
	return taskTitleFromTable("c_task_week", taskID, target)
}

// DailyTaskProgressType returns the progress counter key used by c_task_dly.
func DailyTaskProgressType(taskID int32) (int32, bool) {
	raw, ok := StaticRow("c_task_dly", taskID)
	if !ok {
		return 0, false
	}
	var row struct {
		Type int32 `json:"type"`
	}
	if json.Unmarshal(raw, &row) != nil || row.Type == 0 {
		return 0, false
	}
	return row.Type, true
}

// WeeklyTaskDefinitions returns weekly task rows sorted by task id.
func WeeklyTaskDefinitions() []WeeklyTask {
	table, ok := StaticTableByName("c_task_week")
	if !ok {
		return nil
	}
	out := make([]WeeklyTask, 0, len(table.Rows))
	for idStr, raw := range table.Rows {
		taskID := atoiCatalogID(idStr)
		if taskID == 0 {
			continue
		}
		var row struct {
			Desc  string  `json:"desc"`
			Type  int32   `json:"type"`
			Value []int32 `json:"value"`
		}
		if json.Unmarshal(raw, &row) != nil || row.Type == 0 || len(row.Value) == 0 || row.Value[0] <= 0 {
			continue
		}
		title := strings.TrimSpace(row.Desc)
		if title != "" {
			title = strings.ReplaceAll(title, "${value}", strconv.FormatInt(int64(row.Value[0]), 10))
		}
		out = append(out, WeeklyTask{
			TaskID:       taskID,
			Title:        title,
			ProgressType: row.Type,
			Target:       row.Value[0],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out
}

// AchievementTaskDefinitions returns achievement task rows sorted by task id.
func AchievementTaskDefinitions() []AchievementTask {
	table, ok := StaticTableByName("c_task_ach")
	if !ok {
		return nil
	}
	out := make([]AchievementTask, 0, len(table.Rows))
	for idStr, raw := range table.Rows {
		taskID := atoiCatalogID(idStr)
		if taskID == 0 {
			continue
		}
		groupID := taskID / 10000
		stageIndex := taskID % 10000
		var row struct {
			Title string          `json:"title"`
			Type  int32           `json:"type"`
			Value json.RawMessage `json:"value"`
		}
		if json.Unmarshal(raw, &row) != nil || row.Type == 0 || groupID <= 0 || stageIndex <= 0 {
			continue
		}
		target := firstPositiveInt32(row.Value)
		if target <= 0 {
			continue
		}
		out = append(out, AchievementTask{
			TaskID:       taskID,
			Title:        strings.TrimSpace(row.Title),
			GroupID:      groupID,
			StageIndex:   stageIndex,
			ProgressType: row.Type,
			Target:       target,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out
}

// AchievementTaskTitle returns the client-visible achievement title.
func AchievementTaskTitle(taskID int32) string {
	raw, ok := StaticRow("c_task_ach", taskID)
	if !ok {
		return ""
	}
	var row struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ""
	}
	return strings.TrimSpace(row.Title)
}

// StoryMainSection returns the chapter section at the player's current index.
func StoryMainSection(chapter, sectionIdx int32) (StoryMainSectionInfo, bool) {
	if chapter <= 0 || sectionIdx < 0 {
		return StoryMainSectionInfo{}, false
	}
	rawChapter, ok := StaticRow("c_storyMainChapter", chapter)
	if !ok {
		return StoryMainSectionInfo{}, false
	}
	var ch struct {
		Name      string  `json:"name"`
		SectionID []int32 `json:"sectionId"`
	}
	if json.Unmarshal(rawChapter, &ch) != nil {
		return StoryMainSectionInfo{}, false
	}
	idx := int(sectionIdx)
	if idx < 0 || idx >= len(ch.SectionID) || ch.SectionID[idx] <= 0 {
		return StoryMainSectionInfo{}, false
	}
	sectionID := ch.SectionID[idx]
	rawSection, ok := StaticRow("c_storyMainSection", sectionID)
	if !ok {
		return StoryMainSectionInfo{
			Chapter:     chapter,
			SectionIdx:  sectionIdx,
			SectionID:   sectionID,
			ChapterName: strings.TrimSpace(ch.Name),
		}, true
	}
	var sec struct {
		Name string          `json:"name"`
		Cost json.RawMessage `json:"cost"`
	}
	if json.Unmarshal(rawSection, &sec) != nil {
		return StoryMainSectionInfo{}, false
	}
	return StoryMainSectionInfo{
		Chapter:     chapter,
		SectionIdx:  sectionIdx,
		SectionID:   sectionID,
		ChapterName: strings.TrimSpace(ch.Name),
		SectionName: strings.TrimSpace(sec.Name),
		Cost:        readItemCountsRaw(sec.Cost),
	}, true
}

// ZooEventInfoByID returns a conservative view of a zoo event row.
func ZooEventInfoByID(eventID int32) (ZooEventInfo, bool) {
	raw, ok := StaticRow("c_zooEvent", eventID)
	if !ok {
		return ZooEventInfo{}, false
	}
	var row struct {
		Name     string          `json:"name"`
		Type     int32           `json:"type"`
		SharedID int32           `json:"sharedId"`
		NoHandle json.RawMessage `json:"noHandle"`
		Result   json.RawMessage `json:"result"`
		Reward1  json.RawMessage `json:"reward1"`
		Reward2  json.RawMessage `json:"reward2"`
		Text     string          `json:"text"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ZooEventInfo{}, false
	}
	return ZooEventInfo{
		EventID:    eventID,
		Name:       strings.TrimSpace(row.Name),
		Type:       row.Type,
		SharedID:   row.SharedID,
		NoHandle:   rawTruthy(row.NoHandle),
		Result:     rawTruthy(row.Result),
		Reward1:    readItemCountsRaw(row.Reward1),
		Reward2:    readItemCountsRaw(row.Reward2),
		HasReward2: rawTruthy(row.Reward2),
		Text:       strings.TrimSpace(row.Text),
	}, true
}

// FmlBuildOptionByID returns the client-visible cost for one guild build
// option. The video/share option has no item cost.
func FmlBuildOptionByID(id int32) (FmlBuildOption, bool) {
	raw, ok := StaticRow("c_fmlBld", id)
	if !ok {
		return FmlBuildOption{}, false
	}
	var row struct {
		Name       string    `json:"name"`
		Items      [][]int32 `json:"items"`
		DailyCount int32     `json:"dailyCount"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return FmlBuildOption{}, false
	}
	out := FmlBuildOption{ID: id, Name: strings.TrimSpace(row.Name), DailyLimit: row.DailyCount}
	if len(row.Items) > 0 && len(row.Items[0]) >= 2 {
		out.ItemID = row.Items[0][0]
		if id == 2 {
			out.ItemID = 11
		}
		out.Cost = row.Items[0][1]
	}
	return out, true
}

func taskTitleFromTable(tableName string, taskID, target int32) string {
	raw, ok := StaticRow(tableName, taskID)
	if !ok {
		return ""
	}
	var row struct {
		Desc string `json:"desc"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ""
	}
	desc := strings.TrimSpace(row.Desc)
	if desc == "" {
		return ""
	}
	if target > 0 {
		desc = strings.ReplaceAll(desc, "${value}", strconv.FormatInt(int64(target), 10))
	}
	return desc
}

func firstPositiveInt32(raw json.RawMessage) int32 {
	if n, ok := readInt32Raw(raw); ok && n > 0 {
		return n
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) == nil {
		for _, value := range values {
			if n, ok := readInt32Raw(value); ok && n > 0 {
				return n
			}
		}
	}
	return 0
}

func rawTruthy(raw json.RawMessage) bool {
	return truthyRaw(raw)
}

// RoadGrowLevelTasks returns growth-road level rewards sorted by task id.
func RoadGrowLevelTasks() []RoadGrowTask {
	table, ok := StaticTableByName("c_task_roadGrow")
	if !ok {
		return nil
	}
	out := make([]RoadGrowTask, 0)
	for idStr, raw := range table.Rows {
		taskID := atoiCatalogID(idStr)
		if taskID == 0 {
			continue
		}
		var row map[string]json.RawMessage
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		var typ int32
		if rawType, ok := row["type"]; ok {
			_ = json.Unmarshal(rawType, &typ)
		}
		if typ != 2 {
			continue
		}
		var desc string
		if rawDesc, ok := row["desc"]; ok {
			_ = json.Unmarshal(rawDesc, &desc)
		}
		var target int32
		if _, err := fmt.Sscanf(desc, "等级达到%d级", &target); err != nil || target <= 0 {
			continue
		}
		out = append(out, RoadGrowTask{TaskID: taskID, Title: desc, TargetLevel: target})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TaskID < out[j].TaskID })
	return out
}

func ItemName(id int32) string {
	item, ok := catalog.Items[id]
	if !ok {
		return ""
	}
	if name := strings.TrimSpace(item.DisplayName); name != "" && name != "0" {
		return name
	}
	name := strings.TrimSpace(item.Name)
	if name == "0" {
		return ""
	}
	return name
}

func LandUnlockOpenLevel(landID int32) (int32, bool) {
	land, ok := catalog.FarmLands[landID]
	if !ok || land.OpenLevel == 0 {
		return 0, false
	}
	return land.OpenLevel, true
}

func FlowerBouquetItemID(flowerID int32) int32 {
	if _, ok := catalog.Flowers[flowerID]; !ok {
		return 0
	}
	itemID := flowerID - 1000
	if _, ok := catalog.Items[itemID]; !ok {
		return 0
	}
	return itemID
}

// FlowerUpgradeCostForLevel returns the cost to upgrade a flower from its
// current cultivation level. The client table key is flowerId*100+level.
// The table row carries the required essence count and gold, while the essence
// item itself is consistently flowerId-1000 (for example 23006 -> 22006).
func FlowerUpgradeCostForLevel(flowerID, level int32) (FlowerUpgradeCost, bool) {
	if level <= 0 {
		return FlowerUpgradeCost{}, false
	}
	itemID := FlowerBouquetItemID(flowerID)
	if itemID == 0 {
		return FlowerUpgradeCost{}, false
	}
	raw, ok := StaticRow("c_flowerLvl", flowerID*100+level)
	if !ok {
		return FlowerUpgradeCost{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return FlowerUpgradeCost{}, false
	}
	var count int32
	if rawCost, ok := row["lvlUpCost"]; ok {
		var pair []int32
		if json.Unmarshal(rawCost, &pair) == nil && len(pair) >= 2 {
			count = pair[1]
		}
	}
	var gold int32
	if rawGold, ok := row["gldCost"]; ok {
		_ = json.Unmarshal(rawGold, &gold)
	}
	if count <= 0 {
		return FlowerUpgradeCost{}, false
	}
	return FlowerUpgradeCost{ItemID: itemID, Count: count, Gold: gold}, true
}

// FlowerArtRecipeByID returns the craft recipe for a flower-art item.
func FlowerArtRecipeByID(artID int32) (FlowerArtRecipe, bool) {
	raw, ok := StaticRow("c_flowerArt", artID)
	if !ok {
		return FlowerArtRecipe{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return FlowerArtRecipe{}, false
	}
	var recipe FlowerArtRecipe
	recipe.ArtID = artID
	if rawLevel, ok := row["lvl"]; ok {
		_ = json.Unmarshal(rawLevel, &recipe.Level)
	}
	if rawVase, ok := row["vase"]; ok {
		_ = json.Unmarshal(rawVase, &recipe.VaseID)
	}
	if rawFlowers, ok := row["flowers"]; ok {
		_ = json.Unmarshal(rawFlowers, &recipe.Flowers)
	}
	if rawSell, ok := row["sPrice"]; ok {
		var prices []int32
		if json.Unmarshal(rawSell, &prices) == nil {
			for _, price := range prices {
				if price > recipe.SaleValue {
					recipe.SaleValue = price
				}
			}
		}
	}
	// The current client table stores display/template vase ids and shifted
	// flower ids, while the wire RPC uses the vase group (artID/100) and the
	// real 230xx flower ids. This transform is verified against captured
	// flowerArt.makeFlowerArt calls for 300103, 300207, and 300208.
	if artID >= 300000 {
		if vaseGroup := artID / 100; vaseGroup > 0 {
			recipe.VaseID = vaseGroup
		}
		if suffix := artID % 100; suffix > 0 {
			shift := int32(55) + suffix
			for i, flowerID := range recipe.Flowers {
				if flowerID-shift >= FlowerSeedLow && flowerID-shift < FlowerSeedHigh {
					recipe.Flowers[i] = flowerID - shift
				}
			}
		}
	}
	if recipe.VaseID == 0 || len(recipe.Flowers) == 0 {
		return FlowerArtRecipe{}, false
	}
	recipe.Flowers = cloneInt32s(recipe.Flowers)
	return recipe, true
}

// CultivateCost returns the material cost required to start cultivating a
// flower. The game client static table names this field c_flower.culCost.
func CultivateCost(flowerID int32) ([]ItemCount, bool) {
	flower, ok := catalog.Flowers[flowerID]
	if !ok || len(flower.CultivateCost) == 0 {
		return nil, false
	}
	out := make([]ItemCount, 0, len(flower.CultivateCost))
	for _, cost := range flower.CultivateCost {
		if cost.ItemID == 0 {
			continue
		}
		out = append(out, ItemCount{ItemID: cost.ItemID, Count: cost.Count})
	}
	return out, len(out) > 0
}

func atoiCatalogID(s string) int32 {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}

func cloneItemInfo(item ItemInfo) ItemInfo {
	item.Items = cloneItemStacks(item.Items)
	item.Restore = cloneItemStacks(item.Restore)
	return item
}

func cloneFlowerInfo(flower FlowerInfo) FlowerInfo {
	flower.CultivateCost = cloneItemStacks(flower.CultivateCost)
	return flower
}

func cloneFarmLandInfo(land FarmLandInfo) FarmLandInfo {
	land.Cost = cloneInt32s(land.Cost)
	land.Wasteland = cloneInt32s(land.Wasteland)
	return land
}

func cloneStaticTable(table StaticTable) StaticTable {
	out := StaticTable{
		Columns: make(map[string]string, len(table.Columns)),
		Rows:    make(map[string]json.RawMessage, len(table.Rows)),
	}
	for key, value := range table.Columns {
		out.Columns[key] = value
	}
	for key, value := range table.Rows {
		out.Rows[key] = cloneRaw(value)
	}
	return out
}

func cloneItemStacks(in []ItemStack) []ItemStack {
	if len(in) == 0 {
		return nil
	}
	out := make([]ItemStack, len(in))
	copy(out, in)
	for i := range out {
		out[i].Extra = cloneInt32s(out[i].Extra)
	}
	return out
}

func cloneInt32s(in []int32) []int32 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int32, len(in))
	copy(out, in)
	return out
}

func cloneRaw(in json.RawMessage) json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(in))
	copy(out, in)
	return out
}
