package state

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
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

// FlowerLvlYield is the harvest profile from one c_flowerLvl row: flowers per
// harvest action (cropGets), flowers reserved from friend picks (left), and
// harvest rounds per planting (frequencys).
type FlowerLvlYield struct {
	Level      int32
	CropGets   int32
	Left       int32
	Frequencys int32
}

// FlowersPerPlant returns cropGets * frequencys, or 0 when either side is unset.
func (y FlowerLvlYield) FlowersPerPlant() int32 {
	if y.CropGets <= 0 || y.Frequencys <= 0 {
		return 0
	}
	return y.CropGets * y.Frequencys
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

// RandomEventInfo describes one strictly validated c_randomEvent row. The
// runtime position is a zero-based index into PlaceCount and DialogIDs is the
// complete allow-list accepted for that event. CostFree is kept explicit so a
// future client table that adds a paid random-event path fails closed.
type RandomEventInfo struct {
	EventID    int32
	PlaceCount int32
	DialogIDs  []int32
	CostFree   bool
}

// ZooEventInfo describes one c_zooEvent row relevant to conservative event
// automation.
type ZooEventInfo struct {
	EventID       int32
	Name          string
	Type          int32
	SharedID      int32
	Code          string
	NoHandle      bool
	Result        bool
	Reward1       []ItemCount
	HasReward1    bool
	Reward2       []ItemCount
	HasReward2    bool
	MoodChange    int32
	SatietyChange int32
	SouvenirID    int32
	Text          string
}

// ZooSouvenirCollectInfo describes one c_zooSouvenirCollect milestone.
// Index is the value sent in zoo.recvSouvenirRwd.idxList.
type ZooSouvenirCollectInfo struct {
	Index       int32
	Required    int32
	Description string
	Reward      []ItemCount
}

// PearlProductionTiming is the client-configured duration of one pearl labor
// shift and of one production cycle. Both catalog values are stored in
// seconds; namespace 115 timestamps are milliseconds.
type PearlProductionTiming struct {
	HireTimeSeconds int64
	GatherCDSeconds int64
}

// PearlHireSlotConfig describes one configured production slot. A slot still
// has to exist in namespace 115.0 before it is considered unlocked.
type PearlHireSlotConfig struct {
	PlaceID           int32
	MonthlyCardUnlock bool
}

// PearlHireConfig contains the decoded client constants that make ticket-only
// automatic hiring safe.
type PearlHireConfig struct {
	TicketItemID    int32
	RestTimeSeconds int64
	EnemyMaxDays    int64
	Slots           map[int32]PearlHireSlotConfig
}

// CyclicNoteCatalog describes the static c_act/c_actCyclicNote metadata shared
// by every 花笺集芳 batch. Runtime batch ids and dates still come exclusively
// from namespace 23.
type CyclicNoteCatalog struct {
	TmpType        int32
	Name           string
	CurrencyItemID int32
	TaskSlotCount  int32
}

// CyclicStoryCatalog describes the static c_act metadata shared by every
// 莳花纪闻 batch. Runtime batch ids and dates still come from namespace 23.
type CyclicStoryCatalog struct {
	TmpType        int32
	Name           string
	CurrencyItemID int32
}

// CyclicStoryOrderInfo is one fully validated c_actCyclicStory order row.
// Unknown or malformed rows retain OrderID while CatalogKnown remains false.
type CyclicStoryOrderInfo struct {
	OrderID      int32
	Group        int32
	Cost         int32
	Weight       int32
	Reward       []ItemCount
	CatalogKnown bool
}

// CyclicNoteTaskInfo is one fully validated c_actCyclicNote task joined with
// its c_task_type description. Unknown or malformed rows retain TaskID while
// CatalogKnown remains false, so runtime task-list positions can still be
// displayed without guessing their semantics.
type CyclicNoteTaskInfo struct {
	TaskID       int32
	TaskType     int32
	Param        int32
	Title        string
	Target       int32
	Reward       []ItemCount
	FinishCost   []ItemCount
	CatalogKnown bool
}

// CyclicNoteMilestoneInfo is one namespace 23 template box definition.
// Index is the value stored in the runtime claimed-box list; Target is score.
type CyclicNoteMilestoneInfo struct {
	Index  int32
	Target int32
	Reward []ItemCount
}

// SignTypeRewardInfo is one c_signReward row. ID is the runtime signId and
// Type identifies the corresponding namespace 140.0 map entry.
type SignTypeRewardInfo struct {
	ID     int32
	Type   int32
	Reward []ItemCount
}

// FmlBuildOption describes one c_fmlBld donation/build option.
type FmlBuildOption struct {
	ID              int32
	Name            string
	ItemID          int32
	Cost            int32
	Type            int32
	DailyLimit      int32
	GroupDailyLimit int32
	Rewards         []ItemCount
	// ShareID > 0 means rewarded-video / share flow (c_share), not a bare fml.bld.
	ShareID int32
}

// ShareRewardConfig describes one c_share entry used to interpret the
// namespace 31 usage map. LimitType 1 is the client's daily-reset mode.
type ShareRewardConfig struct {
	ID        int32
	Name      string
	Limit     int32
	LimitType int32
	Cooldown  int32
	Rewards   []ItemCount
}

// FmlLandLvl describes one c_fmlLandLvl growth tier (seconds per flower + stock cap).
type FmlLandLvl struct {
	Level   int32
	TimeSec int32
	Stock   int32
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

type fmlPositionCatalogRow struct {
	ID         int32  `json:"id"`
	Name       string `json:"name"`
	RaceDelete int32  `json:"p_raceDelete"`
}

func fmlPositionRow(position int32) (fmlPositionCatalogRow, bool) {
	if position <= 0 {
		return fmlPositionCatalogRow{}, false
	}
	raw, ok := StaticRow("c_fmlPos", position)
	if !ok {
		return fmlPositionCatalogRow{}, false
	}
	var row fmlPositionCatalogRow
	if json.Unmarshal(raw, &row) != nil || row.ID != position {
		return fmlPositionCatalogRow{}, false
	}
	return row, true
}

// FmlPositionLabel returns the client-visible guild position name.
func FmlPositionLabel(position int32) string {
	row, ok := fmlPositionRow(position)
	if !ok {
		return ""
	}
	return strings.TrimSpace(row.Name)
}

// FmlPositionAllowsRaceDelete reports the client-configured p_raceDelete
// capability for one c_fmlPos row. Unknown or malformed positions fail closed.
func FmlPositionAllowsRaceDelete(position int32) bool {
	row, ok := fmlPositionRow(position)
	return ok && row.RaceDelete == 1
}

// RandomEventDefinition returns a catalog row when its identity, positions,
// and dialogs are complete. CostFree remains false when a future row carries
// an explicit cost; random-event rewards do not count as costs.
func RandomEventDefinition(eventID int32) (RandomEventInfo, bool) {
	if eventID <= 0 {
		return RandomEventInfo{}, false
	}
	table, ok := catalog.Tables["c_randomEvent"]
	if !ok {
		return RandomEventInfo{}, false
	}
	raw, ok := table.Rows[strconv.FormatInt(int64(eventID), 10)]
	if !ok {
		return RandomEventInfo{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return RandomEventInfo{}, false
	}
	rowID, idOK := readStoryMainInt32(row["id"])
	if !idOK || rowID != eventID {
		return RandomEventInfo{}, false
	}
	var places [][]int32
	if json.Unmarshal(row["place"], &places) != nil || len(places) == 0 {
		return RandomEventInfo{}, false
	}
	for _, place := range places {
		if len(place) != 2 {
			return RandomEventInfo{}, false
		}
	}
	var dialogs []int32
	if json.Unmarshal(row["dialog"], &dialogs) != nil || len(dialogs) == 0 {
		return RandomEventInfo{}, false
	}
	seen := make(map[int32]struct{}, len(dialogs))
	for _, dialogID := range dialogs {
		if dialogID <= 0 {
			return RandomEventInfo{}, false
		}
		if _, duplicate := seen[dialogID]; duplicate {
			return RandomEventInfo{}, false
		}
		seen[dialogID] = struct{}{}
	}
	if randomEventRowHasConfiguredCost(row) {
		return RandomEventInfo{EventID: eventID, PlaceCount: int32(len(places)), DialogIDs: append([]int32(nil), dialogs...)}, true
	}
	return RandomEventInfo{
		EventID: eventID, PlaceCount: int32(len(places)), DialogIDs: append([]int32(nil), dialogs...), CostFree: true,
	}, true
}

func randomEventRowHasConfiguredCost(row map[string]json.RawMessage) bool {
	for field, raw := range row {
		name := strings.ToLower(field)
		if !strings.Contains(name, "cost") && !strings.Contains(name, "consume") {
			continue
		}
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("0")) || bytes.Equal(trimmed, []byte("[]")) || bytes.Equal(trimmed, []byte("{}")) {
			continue
		}
		return true
	}
	return false
}

// SignTypeRewardByID returns a strictly validated c_signReward row.
func SignTypeRewardByID(id int32) (SignTypeRewardInfo, bool) {
	raw, ok := StaticRow("c_signReward", id)
	if !ok {
		return SignTypeRewardInfo{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return SignTypeRewardInfo{}, false
	}
	rowID, idOK := readStoryMainInt32(row["id"])
	typeID, typeOK := readStoryMainInt32(row["type"])
	reward, rewardOK := parseStoryMainCost(row["reward"])
	if !idOK || rowID != id || id <= 0 || !typeOK || typeID <= 0 || !rewardOK || len(reward) == 0 {
		return SignTypeRewardInfo{}, false
	}
	return SignTypeRewardInfo{ID: rowID, Type: typeID, Reward: reward}, true
}

// CyclicNoteCatalogConfig returns the validated static configuration shared
// by cyclic-note batches. The batch itself is deliberately not hard-coded:
// namespace 23 selects it dynamically by TmpType.
func CyclicNoteCatalogConfig() (CyclicNoteCatalog, bool) {
	const cyclicNoteTmpType int32 = 4002
	raw, ok := StaticRow("c_act", cyclicNoteTmpType)
	if !ok {
		return CyclicNoteCatalog{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return CyclicNoteCatalog{}, false
	}
	id, idOK := readStoryMainInt32(row["id"])
	var name string
	nameOK := json.Unmarshal(row["name"], &name) == nil && strings.TrimSpace(name) != ""
	currencyID, currencyOK := readSinglePositiveCatalogID(row["scoreId"])
	slotCount, slotsOK := cyclicNoteTaskSlotCount()
	if !idOK || id != cyclicNoteTmpType || !nameOK || !currencyOK || !slotsOK {
		return CyclicNoteCatalog{}, false
	}
	return CyclicNoteCatalog{
		TmpType:        cyclicNoteTmpType,
		Name:           strings.TrimSpace(name),
		CurrencyItemID: currencyID,
		TaskSlotCount:  slotCount,
	}, true
}

// CyclicStoryCatalogConfig returns the validated static configuration shared
// by cyclic-story batches. The batch itself is deliberately not hard-coded:
// namespace 23 selects it dynamically by TmpType.
func CyclicStoryCatalogConfig() (CyclicStoryCatalog, bool) {
	const cyclicStoryTmpType int32 = 4003
	raw, ok := StaticRow("c_act", cyclicStoryTmpType)
	if !ok {
		return CyclicStoryCatalog{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return CyclicStoryCatalog{}, false
	}
	id, idOK := readStoryMainInt32(row["id"])
	var name string
	nameOK := json.Unmarshal(row["name"], &name) == nil && strings.TrimSpace(name) != ""
	currencyID, currencyOK := readSinglePositiveCatalogID(row["scoreId"])
	if !idOK || id != cyclicStoryTmpType || !nameOK || !currencyOK {
		return CyclicStoryCatalog{}, false
	}
	return CyclicStoryCatalog{
		TmpType:        cyclicStoryTmpType,
		Name:           strings.TrimSpace(name),
		CurrencyItemID: currencyID,
	}, true
}

// CyclicStoryOrderInfoByID joins a c_actCyclicStory row. Unknown or malformed
// rows retain OrderID while CatalogKnown remains false.
func CyclicStoryOrderInfoByID(orderID int32) CyclicStoryOrderInfo {
	unknown := CyclicStoryOrderInfo{OrderID: orderID}
	if orderID <= 0 {
		return unknown
	}
	raw, ok := StaticRow("c_actCyclicStory", orderID)
	if !ok {
		return unknown
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return unknown
	}
	id, idOK := readStoryMainInt32(row["id"])
	group, groupOK := readStoryMainInt32(row["group"])
	cost, costOK := readStoryMainInt32(row["cost"])
	weight, weightOK := readStoryMainInt32(row["weight"])
	reward, rewardOK := parseStoryMainCost(row["items"])
	if !idOK || id != orderID || !groupOK || group <= 0 || !costOK || cost <= 0 ||
		!weightOK || weight <= 0 || !rewardOK || len(reward) == 0 {
		return unknown
	}
	return CyclicStoryOrderInfo{
		OrderID: orderID, Group: group, Cost: cost, Weight: weight,
		Reward: cloneCyclicNoteItems(reward), CatalogKnown: true,
	}
}

// CyclicNoteTaskInfoByID joins a c_actCyclicNote row with c_task_type. The
// returned TaskID is retained even on failure so callers can preserve and
// report unknown runtime tasks without treating them as actionable.
func CyclicNoteTaskInfoByID(taskID int32) CyclicNoteTaskInfo {
	unknown := CyclicNoteTaskInfo{TaskID: taskID}
	if taskID <= 0 {
		return unknown
	}
	raw, ok := StaticRow("c_actCyclicNote", taskID)
	if !ok {
		return unknown
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return unknown
	}
	id, idOK := readStoryMainInt32(row["id"])
	taskType, typeOK := readStoryMainInt32(row["type"])
	target, targetOK := readStoryMainInt32(row["value"])
	color, colorOK := readStoryMainInt32(row["color"])
	group, groupOK := readStoryMainInt32(row["group"])
	param := int32(0)
	if rawParam, present := row["param"]; present {
		var paramOK bool
		param, paramOK = readStoryMainInt32(rawParam)
		if !paramOK {
			return unknown
		}
	}
	reward, rewardOK := parseStoryMainCost(row["reward"])
	finishCost, finishOK := parseStoryMainCost(row["finishCost"])
	if !idOK || id != taskID || !typeOK || taskType <= 0 || !targetOK || target <= 0 ||
		!colorOK || color <= 0 || !groupOK || group <= 0 || !rewardOK || !finishOK {
		return unknown
	}

	typeRaw, ok := StaticRow("c_task_type", taskType)
	if !ok {
		return unknown
	}
	var typeRow map[string]json.RawMessage
	if json.Unmarshal(typeRaw, &typeRow) != nil {
		return unknown
	}
	typeID, typeIDOK := readStoryMainInt32(typeRow["id"])
	var description string
	if !typeIDOK || typeID != taskType || json.Unmarshal(typeRow["desc"], &description) != nil || strings.TrimSpace(description) == "" {
		return unknown
	}
	title := strings.TrimSpace(description)
	title = strings.ReplaceAll(title, "${value}", strconv.FormatInt(int64(target), 10))
	title = strings.ReplaceAll(title, "${param}", strconv.FormatInt(int64(param), 10))
	return CyclicNoteTaskInfo{
		TaskID:       taskID,
		TaskType:     taskType,
		Param:        param,
		Title:        title,
		Target:       target,
		Reward:       reward,
		FinishCost:   finishCost,
		CatalogKnown: true,
	}
}

// ParseCyclicNoteTemplateBoxes decodes field 9 of a namespace 23.1 template.
// Each runtime row is [milestoneIndex, scoreTarget, "item,count;..."] rather
// than the array-of-pairs representation used by static catalog rewards.
// Explicit null and [] are valid observed-empty replacements.
func ParseCyclicNoteTemplateBoxes(raw json.RawMessage) ([]CyclicNoteMilestoneInfo, bool) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, true
	}
	var rows []json.RawMessage
	if len(trimmed) == 0 || json.Unmarshal(trimmed, &rows) != nil {
		return nil, false
	}
	out := make([]CyclicNoteMilestoneInfo, 0, len(rows))
	seenIndexes := make(map[int32]struct{}, len(rows))
	seenTargets := make(map[int32]struct{}, len(rows))
	for _, rawRow := range rows {
		var fields []json.RawMessage
		if json.Unmarshal(rawRow, &fields) != nil || len(fields) != 3 {
			return nil, false
		}
		index, indexOK := readStoryMainInt32(fields[0])
		target, targetOK := readStoryMainInt32(fields[1])
		var rewardText string
		if !indexOK || index <= 0 || !targetOK || target <= 0 || json.Unmarshal(fields[2], &rewardText) != nil {
			return nil, false
		}
		if _, duplicate := seenIndexes[index]; duplicate {
			return nil, false
		}
		if _, duplicate := seenTargets[target]; duplicate {
			return nil, false
		}
		reward, ok := parseCyclicNoteRewardText(rewardText)
		if !ok {
			return nil, false
		}
		seenIndexes[index] = struct{}{}
		seenTargets[target] = struct{}{}
		out = append(out, CyclicNoteMilestoneInfo{Index: index, Target: target, Reward: reward})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	for i := 1; i < len(out); i++ {
		if out[i].Target <= out[i-1].Target {
			return nil, false
		}
	}
	return out, true
}

func cyclicNoteTaskSlotCount() (int32, bool) {
	raw, ok := StaticRow("c_actCyclicNote", -1)
	if !ok {
		return 0, false
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(raw, &meta) != nil {
		return 0, false
	}
	var rows []json.RawMessage
	if json.Unmarshal(meta["$taskNum"], &rows) != nil || len(rows) == 0 || len(rows) > math.MaxInt32 {
		return 0, false
	}
	for idx, rawRow := range rows {
		var fields []json.RawMessage
		if json.Unmarshal(rawRow, &fields) != nil || len(fields) != 3 {
			return 0, false
		}
		slotID, slotOK := readStoryMainInt32(fields[0])
		unlockType, unlockOK := readStoryMainInt32(fields[1])
		unlockCost, costOK := readStoryMainInt32(fields[2])
		if !slotOK || slotID != int32(idx+1) || !unlockOK || unlockType < 0 || !costOK || unlockCost < 0 {
			return 0, false
		}
		if (unlockType == 0 && unlockCost != 0) || (unlockType != 0 && unlockCost <= 0) {
			return 0, false
		}
	}
	return int32(len(rows)), true
}

func readSinglePositiveCatalogID(raw json.RawMessage) (int32, bool) {
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil || len(values) != 1 {
		return 0, false
	}
	id, ok := readStoryMainInt32(values[0])
	return id, ok && id > 0
}

func parseCyclicNoteRewardText(text string) ([]ItemCount, bool) {
	if text == "" || strings.TrimSpace(text) != text {
		return nil, false
	}
	aggregated := make(map[int32]int64)
	for _, stackText := range strings.Split(text, ";") {
		parts := strings.Split(stackText, ",")
		if len(parts) != 2 {
			return nil, false
		}
		itemID64, itemErr := strconv.ParseInt(parts[0], 10, 32)
		count64, countErr := strconv.ParseInt(parts[1], 10, 32)
		if itemErr != nil || countErr != nil || itemID64 <= 0 || count64 <= 0 ||
			strconv.FormatInt(itemID64, 10) != parts[0] || strconv.FormatInt(count64, 10) != parts[1] {
			return nil, false
		}
		itemID := int32(itemID64)
		aggregated[itemID] += count64
		if aggregated[itemID] > math.MaxInt32 {
			return nil, false
		}
	}
	ids := make([]int32, 0, len(aggregated))
	for itemID := range aggregated {
		ids = append(ids, itemID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]ItemCount, 0, len(ids))
	for _, itemID := range ids {
		out = append(out, ItemCount{ItemID: itemID, Count: int32(aggregated[itemID])})
	}
	return out, true
}

// PearlProductionTimingFromCatalog returns the timing constants used by the
// client-side PearlPlaceCtrl production formula.
func PearlProductionTimingFromCatalog() (PearlProductionTiming, bool) {
	pearlRaw, ok := StaticRow("c_pearl", -1)
	if !ok {
		return PearlProductionTiming{}, false
	}
	var pearl struct {
		HireTimeSeconds int64 `json:"$hireTime"`
	}
	if json.Unmarshal(pearlRaw, &pearl) != nil || pearl.HireTimeSeconds <= 0 {
		return PearlProductionTiming{}, false
	}

	eventRaw, ok := StaticRow("c_pearlEvent", -1)
	if !ok {
		return PearlProductionTiming{}, false
	}
	var event struct {
		GatherCDSeconds int64 `json:"$gatherCd"`
	}
	if json.Unmarshal(eventRaw, &event) != nil || event.GatherCDSeconds <= 0 {
		return PearlProductionTiming{}, false
	}

	return PearlProductionTiming{
		HireTimeSeconds: pearl.HireTimeSeconds,
		GatherCDSeconds: event.GatherCDSeconds,
	}, true
}

// PearlHireConfigFromCatalog returns the exact decoded ticket, protection,
// enemy-age and slot privilege gates used by the official client.
func PearlHireConfigFromCatalog() (PearlHireConfig, bool) {
	raw, ok := StaticRow("c_pearl", -1)
	if !ok {
		return PearlHireConfig{}, false
	}
	var base struct {
		TicketItemID    int32 `json:"$hireItem"`
		RestTimeSeconds int64 `json:"$restTime"`
		EnemyMaxDays    int64 `json:"$enemyTimeMax"`
		PlaceMax        int32 `json:"$placeMax"`
	}
	if json.Unmarshal(raw, &base) != nil || base.TicketItemID <= 0 ||
		base.RestTimeSeconds <= 0 || base.EnemyMaxDays <= 0 || base.PlaceMax <= 0 {
		return PearlHireConfig{}, false
	}
	config := PearlHireConfig{
		TicketItemID:    base.TicketItemID,
		RestTimeSeconds: base.RestTimeSeconds,
		EnemyMaxDays:    base.EnemyMaxDays,
		Slots:           make(map[int32]PearlHireSlotConfig, base.PlaceMax),
	}
	for placeID := int32(1); placeID <= base.PlaceMax; placeID++ {
		rawSlot, exists := StaticRow("c_pearl", placeID)
		if !exists {
			return PearlHireConfig{}, false
		}
		var slot struct {
			MonthlyCardUnlock int32 `json:"mCardUnlock"`
		}
		if json.Unmarshal(rawSlot, &slot) != nil {
			return PearlHireConfig{}, false
		}
		config.Slots[placeID] = PearlHireSlotConfig{
			PlaceID:           placeID,
			MonthlyCardUnlock: slot.MonthlyCardUnlock != 0,
		}
	}
	return config, true
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

// PlayerMaxLevel returns the configured player level cap from c_lvl.$max.
func PlayerMaxLevel() int32 {
	raw, ok := StaticRow("c_lvl", -1)
	if !ok {
		return 0
	}
	var row struct {
		Max int32 `json:"$max"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return row.Max
}

// PlayerLevelExpRequired returns the within-level experience needed to advance
// from the given player level. Level 65 (the configured cap) has no exp row.
func PlayerLevelExpRequired(level int32) (required int32, ok bool) {
	if level <= 0 {
		return 0, false
	}
	raw, exists := StaticRow("c_lvl", level)
	if !exists {
		return 0, false
	}
	var row struct {
		Exp int32 `json:"exp"`
	}
	if json.Unmarshal(raw, &row) != nil || row.Exp <= 0 {
		return 0, false
	}
	return row.Exp, true
}

// ExperienceToNextLevel returns remaining XP to the next player level.
// Namespace 7.0.35 experience is progress within the current level; c_lvl[level].exp
// is the requirement to advance. maxed is true at the configured level cap.
func ExperienceToNextLevel(level, experience int32) (remaining, required int32, maxed bool) {
	if level <= 0 {
		return 0, 0, false
	}
	if maxLevel := PlayerMaxLevel(); maxLevel > 0 && level >= maxLevel {
		return 0, 0, true
	}
	required, ok := PlayerLevelExpRequired(level)
	if !ok {
		return 0, 0, true
	}
	if experience >= required {
		return 0, required, false
	}
	return required - experience, required, false
}

// MainTaskDefinition is one claimable row in the client-configured main-task
// chain. Rows after c_task_main.$endId may remain in the decoded table but are
// not part of the currently active chain.
type MainTaskDefinition struct {
	TaskID     int32
	Target     int32
	NextTaskID int32
	EndTaskID  int32
}

// ResolveMainTaskDefinition resolves a current task against the exact chain
// from $initId through $endId. The client treats every positive curTaskId
// greater than $endId as terminal, even when a later row still exists.
func ResolveMainTaskDefinition(taskID int32) (definition MainTaskDefinition, valid, complete bool) {
	definitions, endTaskID, ok := mainTaskCatalogDefinitions()
	if !ok || taskID <= 0 {
		return MainTaskDefinition{}, false, false
	}
	if taskID > endTaskID {
		return MainTaskDefinition{TaskID: taskID, EndTaskID: endTaskID}, true, true
	}
	definition, ok = definitions[taskID]
	return definition, ok, false
}

// MainTaskEndID returns the last claimable task configured by the client.
func MainTaskEndID() (int32, bool) {
	_, endTaskID, ok := mainTaskCatalogDefinitions()
	return endTaskID, ok
}

var (
	mainTaskCatalogOnce            sync.Once
	mainTaskCatalogDefinitionsByID map[int32]MainTaskDefinition
	mainTaskCatalogEndTaskID       int32
	mainTaskCatalogValid           bool
)

func mainTaskCatalogDefinitions() (map[int32]MainTaskDefinition, int32, bool) {
	mainTaskCatalogOnce.Do(func() {
		mainTaskCatalogDefinitionsByID, mainTaskCatalogEndTaskID, mainTaskCatalogValid = loadMainTaskCatalogDefinitions()
	})
	return mainTaskCatalogDefinitionsByID, mainTaskCatalogEndTaskID, mainTaskCatalogValid
}

func loadMainTaskCatalogDefinitions() (map[int32]MainTaskDefinition, int32, bool) {
	table, ok := catalog.Tables["c_task_main"]
	if !ok {
		return nil, 0, false
	}
	rawMeta, ok := table.Rows["-1"]
	if !ok {
		return nil, 0, false
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(rawMeta, &meta) != nil {
		return nil, 0, false
	}
	initTaskID, initOK := readStoryMainInt32(meta["$initId"])
	endTaskID, endOK := readStoryMainInt32(meta["$endId"])
	if !initOK || !endOK || initTaskID <= 0 || endTaskID <= 0 {
		return nil, 0, false
	}

	definitions := make(map[int32]MainTaskDefinition)
	seen := make(map[int32]struct{})
	taskID := initTaskID
	for range table.Rows {
		if _, duplicate := seen[taskID]; duplicate {
			return nil, 0, false
		}
		seen[taskID] = struct{}{}
		raw, exists := table.Rows[strconv.FormatInt(int64(taskID), 10)]
		if !exists {
			return nil, 0, false
		}
		var row map[string]json.RawMessage
		if json.Unmarshal(raw, &row) != nil {
			return nil, 0, false
		}
		rowID, idOK := readStoryMainInt32(row["id"])
		target, targetOK := readStoryMainInt32(row["value"])
		nextTaskID, nextOK := readStoryMainInt32(row["nextId"])
		if !idOK || rowID != taskID || !targetOK || target <= 0 || !nextOK || nextTaskID <= 0 {
			return nil, 0, false
		}
		definitions[taskID] = MainTaskDefinition{
			TaskID: taskID, Target: target, NextTaskID: nextTaskID, EndTaskID: endTaskID,
		}
		if taskID == endTaskID {
			if nextTaskID <= endTaskID {
				return nil, 0, false
			}
			return definitions, endTaskID, true
		}
		taskID = nextTaskID
	}
	return nil, 0, false
}

const (
	mainTaskTypeHarvestFlower   int32 = 3004
	mainTaskTypeCultivateFlower int32 = 3005
)

// MainTaskFlowerRequirement returns the flower id and missing count for a
// current main task when the static task row is an explicit flower-harvest
// task. A cultivation task also carries a flower param, but it is not an
// inventory deficit and must never drive planting.
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
	return mainTaskFlowerTargetByType(taskID, mainTaskTypeHarvestFlower)
}

// MainTaskCultivateTarget returns the flower and target count for a main task
// whose observed client definition requires successful cultivation.
func MainTaskCultivateTarget(taskID int32) (flowerID, target int32, ok bool) {
	return mainTaskFlowerTargetByType(taskID, mainTaskTypeCultivateFlower)
}

func mainTaskFlowerTargetByType(taskID, taskType int32) (flowerID, target int32, ok bool) {
	definition, valid, complete := ResolveMainTaskDefinition(taskID)
	if !valid || complete {
		return 0, 0, false
	}
	raw, ok := StaticRow("c_task_main", taskID)
	if !ok {
		return 0, 0, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return 0, 0, false
	}
	rowType, validType := readStoryMainInt32(row["type"])
	param, validParam := readStoryMainInt32(row["param"])
	if !validType || rowType != taskType || !validParam || !isFlowerItemID(param) {
		return 0, 0, false
	}
	return param, definition.Target, true
}

// MainTaskTitle returns the client-visible description for a main task.
func MainTaskTitle(taskID int32) string {
	_, valid, complete := ResolveMainTaskDefinition(taskID)
	if !valid || complete {
		return ""
	}
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

// StoryMainSection returns the exact chapter section at the player's current
// next-unlock index. Missing referenced rows or malformed/empty costs are
// rejected rather than returned as a partial definition.
func StoryMainSection(chapter, sectionIdx int32) (StoryMainSectionInfo, bool) {
	if chapter <= 0 || sectionIdx < 0 {
		return StoryMainSectionInfo{}, false
	}
	ch, ok := storyMainChapter(chapter)
	if !ok {
		return StoryMainSectionInfo{}, false
	}
	idx := int(sectionIdx)
	if idx < 0 || idx >= len(ch.SectionIDs) || ch.SectionIDs[idx] <= 0 {
		return StoryMainSectionInfo{}, false
	}
	sectionID := ch.SectionIDs[idx]
	rawSection, ok := StaticRow("c_storyMainSection", sectionID)
	if !ok {
		return StoryMainSectionInfo{}, false
	}
	var sec struct {
		ID   int32           `json:"id"`
		Name string          `json:"name"`
		Cost json.RawMessage `json:"cost"`
	}
	if json.Unmarshal(rawSection, &sec) != nil || sec.ID != sectionID {
		return StoryMainSectionInfo{}, false
	}
	cost, ok := parseStoryMainCost(sec.Cost)
	if !ok {
		return StoryMainSectionInfo{}, false
	}
	return StoryMainSectionInfo{
		Chapter:     chapter,
		SectionIdx:  sectionIdx,
		SectionID:   sectionID,
		ChapterName: strings.TrimSpace(ch.Name),
		SectionName: strings.TrimSpace(sec.Name),
		Cost:        cost,
	}, true
}

type storyMainChapterDefinition struct {
	ID         int32
	Name       string
	SectionIDs []int32
}

func storyMainChapter(chapter int32) (storyMainChapterDefinition, bool) {
	raw, ok := StaticRow("c_storyMainChapter", chapter)
	if !ok {
		return storyMainChapterDefinition{}, false
	}
	var row struct {
		ID         int32   `json:"id"`
		Name       string  `json:"name"`
		SectionIDs []int32 `json:"sectionId"`
	}
	if json.Unmarshal(raw, &row) != nil || row.ID != chapter || len(row.SectionIDs) == 0 {
		return storyMainChapterDefinition{}, false
	}
	seen := make(map[int32]struct{}, len(row.SectionIDs))
	for _, sectionID := range row.SectionIDs {
		if sectionID <= 0 {
			return storyMainChapterDefinition{}, false
		}
		if _, exists := seen[sectionID]; exists {
			return storyMainChapterDefinition{}, false
		}
		seen[sectionID] = struct{}{}
	}
	return storyMainChapterDefinition{ID: row.ID, Name: row.Name, SectionIDs: row.SectionIDs}, true
}

func parseStoryMainCost(raw json.RawMessage) ([]ItemCount, bool) {
	var stacks []json.RawMessage
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &stacks) != nil || len(stacks) == 0 {
		return nil, false
	}
	aggregated := make(map[int32]int64, len(stacks))
	for _, rawStack := range stacks {
		var parts []json.RawMessage
		if json.Unmarshal(rawStack, &parts) != nil || len(parts) != 2 {
			return nil, false
		}
		itemID, validItem := readStoryMainInt32(parts[0])
		count, validCount := readStoryMainInt32(parts[1])
		if !validItem || !validCount || itemID <= 0 || count <= 0 {
			return nil, false
		}
		aggregated[itemID] += int64(count)
		if aggregated[itemID] > math.MaxInt32 {
			return nil, false
		}
	}
	ids := make([]int32, 0, len(aggregated))
	for itemID := range aggregated {
		ids = append(ids, itemID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]ItemCount, 0, len(ids))
	for _, itemID := range ids {
		out = append(out, ItemCount{ItemID: itemID, Count: int32(aggregated[itemID])})
	}
	return out, true
}

func readStoryMainInt32(raw json.RawMessage) (int32, bool) {
	n, ok := readStoryMainInt64(raw)
	if !ok || n < math.MinInt32 || n > math.MaxInt32 {
		return 0, false
	}
	return int32(n), true
}

func readStoryMainInt64(raw json.RawMessage) (int64, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' {
		return 0, false
	}
	var value json.Number
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	n, err := strconv.ParseInt(value.String(), 10, 64)
	return n, err == nil
}

// StoryMainTerminal returns the only catalog-derived completed progress pair.
// With the current decoded client this is 165:0, immediately after chapter
// 164's final section 17306.
func StoryMainTerminal() (chapter, sectionIdx int32, ok bool) {
	table, exists := StaticTableByName("c_storyMainChapter")
	if !exists {
		return 0, 0, false
	}
	rawSentinel, exists := table.Rows["-1"]
	if !exists {
		return 0, 0, false
	}
	var sentinel struct {
		MaxChapter int32 `json:"$max"`
	}
	if json.Unmarshal(rawSentinel, &sentinel) != nil || sentinel.MaxChapter <= 0 {
		return 0, 0, false
	}
	var lastChapter int32
	for idText := range table.Rows {
		value, err := strconv.ParseInt(idText, 10, 32)
		if err != nil || value <= 0 {
			continue
		}
		if value > int64(lastChapter) {
			lastChapter = int32(value)
		}
	}
	if lastChapter != sentinel.MaxChapter {
		return 0, 0, false
	}
	last, valid := storyMainChapter(lastChapter)
	if !valid || lastChapter == math.MaxInt32 {
		return 0, 0, false
	}
	if _, valid = StoryMainSection(lastChapter, int32(len(last.SectionIDs)-1)); !valid {
		return 0, 0, false
	}
	return lastChapter + 1, 0, true
}

// ResolveStoryMainProgress classifies a server chapter/index pair against the
// exact decoded catalog.
func ResolveStoryMainProgress(chapter, sectionIdx int32) (StoryMainSectionInfo, bool, bool) {
	terminalChapter, terminalSection, terminalOK := StoryMainTerminal()
	if !terminalOK {
		return StoryMainSectionInfo{}, false, false
	}
	if chapter == terminalChapter && sectionIdx == terminalSection {
		return StoryMainSectionInfo{Chapter: chapter, SectionIdx: sectionIdx}, true, true
	}
	section, valid := StoryMainSection(chapter, sectionIdx)
	return section, valid, false
}

// NextStoryMainProgress returns the exact state expected after unlocking the
// supplied current section.
func NextStoryMainProgress(chapter, sectionIdx int32) (nextChapter, nextSectionIdx int32, complete, ok bool) {
	currentChapter, valid := storyMainChapter(chapter)
	if !valid || sectionIdx < 0 || int(sectionIdx) >= len(currentChapter.SectionIDs) {
		return 0, 0, false, false
	}
	if int(sectionIdx)+1 < len(currentChapter.SectionIDs) {
		return chapter, sectionIdx + 1, false, true
	}
	terminalChapter, terminalSection, terminalOK := StoryMainTerminal()
	if terminalOK && chapter+1 == terminalChapter {
		return terminalChapter, terminalSection, true, true
	}
	if _, valid = StoryMainSection(chapter+1, 0); !valid {
		return 0, 0, false, false
	}
	return chapter + 1, 0, false, true
}

// ZooEventInfoByID returns a conservative view of a zoo event row.
func ZooEventInfoByID(eventID int32) (ZooEventInfo, bool) {
	raw, ok := StaticRow("c_zooEvent", eventID)
	if !ok {
		return ZooEventInfo{}, false
	}
	var row struct {
		Name          string          `json:"name"`
		Type          int32           `json:"type"`
		SharedID      int32           `json:"sharedId"`
		Code          string          `json:"code"`
		NoHandle      json.RawMessage `json:"noHandle"`
		Result        json.RawMessage `json:"result"`
		Reward1       json.RawMessage `json:"reward1"`
		Reward2       json.RawMessage `json:"reward2"`
		MoodChange    int32           `json:"moodChange"`
		SatietyChange int32           `json:"satietyChange"`
		SouvenirID    int32           `json:"souvenirId"`
		Text          string          `json:"text"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ZooEventInfo{}, false
	}
	return ZooEventInfo{
		EventID:       eventID,
		Name:          strings.TrimSpace(row.Name),
		Type:          row.Type,
		SharedID:      row.SharedID,
		Code:          strings.TrimSpace(row.Code),
		NoHandle:      rawTruthy(row.NoHandle),
		Result:        rawTruthy(row.Result),
		Reward1:       readItemCountsRaw(row.Reward1),
		HasReward1:    rawTruthy(row.Reward1),
		Reward2:       readItemCountsRaw(row.Reward2),
		HasReward2:    rawTruthy(row.Reward2),
		MoodChange:    row.MoodChange,
		SatietyChange: row.SatietyChange,
		SouvenirID:    row.SouvenirID,
		Text:          strings.TrimSpace(row.Text),
	}, true
}

// ZooSouvenirCollectInfoByIndex returns one decoded collection milestone.
func ZooSouvenirCollectInfoByIndex(index int32) (ZooSouvenirCollectInfo, bool) {
	if index <= 0 {
		return ZooSouvenirCollectInfo{}, false
	}
	raw, ok := StaticRow("c_zooSouvenirCollect", index)
	if !ok {
		return ZooSouvenirCollectInfo{}, false
	}
	var row struct {
		ID     int32           `json:"id"`
		Value  int32           `json:"value"`
		Desc   string          `json:"desc"`
		Reward json.RawMessage `json:"reward"`
	}
	if json.Unmarshal(raw, &row) != nil || row.ID != index || row.Value <= 0 {
		return ZooSouvenirCollectInfo{}, false
	}
	return ZooSouvenirCollectInfo{
		Index:       index,
		Required:    row.Value,
		Description: strings.TrimSpace(row.Desc),
		Reward:      readItemCountsRaw(row.Reward),
	}, true
}

// ZooSouvenirCollectMilestones returns all valid collection milestones sorted
// by server reward index.
func ZooSouvenirCollectMilestones() []ZooSouvenirCollectInfo {
	table, ok := catalog.Tables["c_zooSouvenirCollect"]
	if !ok {
		return nil
	}
	out := make([]ZooSouvenirCollectInfo, 0, len(table.Rows))
	for rawIndex := range table.Rows {
		value, err := strconv.ParseInt(rawIndex, 10, 32)
		if err != nil || value <= 0 {
			continue
		}
		if info, ok := ZooSouvenirCollectInfoByIndex(int32(value)); ok {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

// FmlRaceBaseTaskNum returns c_fmlRace(raceLvl).taskNum — the free task quota
// for a guild race tier. Missing/invalid tiers fall back to level 1 (client
// default: raceLvl||1). Returns 0 only when the catalog row is unavailable.
func FmlRaceBaseTaskNum(raceLvl int32) int32 {
	if raceLvl <= 0 {
		raceLvl = 1
	}
	raw, ok := StaticRow("c_fmlRace", raceLvl)
	if !ok && raceLvl != 1 {
		raw, ok = StaticRow("c_fmlRace", 1)
	}
	if !ok {
		return 0
	}
	var row struct {
		TaskNum int32 `json:"taskNum"`
	}
	if json.Unmarshal(raw, &row) != nil || row.TaskNum <= 0 {
		return 0
	}
	return row.TaskNum
}

// FmlRaceTotalTaskNum is the client-visible max free task count for a guild
// race tier: c_fmlRace(raceLvl).taskNum (甲=18, 乙=15, 丙=12, 丁=9).
// Purchased extras (buyTaskNum) are not included. Unknown raceLvl returns 0
// so the UI does not fall back to 丁级's 9.
func FmlRaceTotalTaskNum(raceLvl, _ int32) int32 {
	if raceLvl <= 0 {
		return 0
	}
	return FmlRaceBaseTaskNum(raceLvl)
}

// FmlRaceTaskTypeByID returns c_fmlRaceTask.type for a catalog task id.
// When the row is missing, the id itself is returned so callers that already
// store a type id (e.g. tests) keep working.
func FmlRaceTaskTypeByID(taskID int32) int32 {
	if taskID <= 0 {
		return 0
	}
	raw, ok := StaticRow("c_fmlRaceTask", taskID)
	if !ok {
		return taskID
	}
	var row struct {
		Type int32 `json:"type"`
	}
	if json.Unmarshal(raw, &row) != nil || row.Type <= 0 {
		return taskID
	}
	return row.Type
}

// FmlRaceTaskUpgradeCost returns the client-visible diamond cost for upgrading
// the currently held guild-race task. The mini client computes the next score
// from an explicit upgradePoint mapping when present, otherwise from the
// task-group multiplier, then charges 3 * max(nextScore-currentScore, 1).
func FmlRaceTaskUpgradeCost(taskID, currentScore int32) (int32, bool) {
	if taskID <= 0 || currentScore <= 0 {
		return 0, false
	}
	raw, ok := StaticRow("c_fmlRaceTask", taskID)
	if !ok {
		return 0, false
	}
	var task struct {
		Group        int32     `json:"group"`
		UpgradePoint [][]int32 `json:"upgradePoint"`
	}
	if json.Unmarshal(raw, &task) != nil || task.Group <= 0 {
		return 0, false
	}
	nextScore := int32(0)
	for _, point := range task.UpgradePoint {
		if len(point) >= 2 && point[0] == currentScore && point[1] > 0 {
			nextScore = point[1]
			break
		}
	}
	if nextScore == 0 {
		configRaw, ok := StaticRow("c_fmlRaceTask", -1)
		if !ok {
			return 0, false
		}
		var config struct {
			UpgradeMultiple []int32 `json:"$upgradeMultiple"`
		}
		if json.Unmarshal(configRaw, &config) != nil || int(task.Group) > len(config.UpgradeMultiple) {
			return 0, false
		}
		multiplier := config.UpgradeMultiple[task.Group-1]
		if multiplier <= 0 {
			return 0, false
		}
		next := (int64(currentScore)*int64(multiplier) + 9999) / 10000
		if next <= 0 || next > math.MaxInt32 {
			return 0, false
		}
		nextScore = int32(next)
	}
	delta := int64(nextScore) - int64(currentScore)
	if delta < 1 {
		delta = 1
	}
	cost := delta * 3
	if cost > math.MaxInt32 {
		return 0, false
	}
	return int32(cost), true
}

// FmlBuildOptionByID returns the client-visible cost for one guild build
// option. Video/share tiers expose ShareID and have no item cost.
func FmlBuildOptionByID(id int32) (FmlBuildOption, bool) {
	raw, ok := StaticRow("c_fmlBld", id)
	if !ok {
		return FmlBuildOption{}, false
	}
	var row struct {
		Name       string          `json:"name"`
		Items      [][]int32       `json:"items"`
		Type       int32           `json:"type"`
		DailyCount int32           `json:"dailyCount"`
		ShareID    int32           `json:"shareId"`
		Rewards    json.RawMessage `json:"rwds"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return FmlBuildOption{}, false
	}
	out := FmlBuildOption{
		ID:         id,
		Name:       strings.TrimSpace(row.Name),
		Type:       row.Type,
		DailyLimit: row.DailyCount,
		ShareID:    row.ShareID,
		Rewards:    readItemCountsRaw(row.Rewards),
	}
	if masterRaw, exists := StaticRow("c_fmlBld", -1); exists {
		var master struct {
			DailyCountMap map[string]int32 `json:"$dailyCountMap"`
		}
		if json.Unmarshal(masterRaw, &master) == nil {
			out.GroupDailyLimit = master.DailyCountMap[strconv.FormatInt(int64(row.Type), 10)]
		}
	}
	if len(row.Items) > 0 && len(row.Items[0]) >= 2 {
		out.ItemID = row.Items[0][0]
		if id == 2 {
			out.ItemID = 11
		}
		out.Cost = row.Items[0][1]
	}
	return out, true
}

// ShareRewardConfigByID returns the counter and reward semantics for one
// video/share action.
func ShareRewardConfigByID(id int32) (ShareRewardConfig, bool) {
	raw, ok := StaticRow("c_share", id)
	if !ok {
		return ShareRewardConfig{}, false
	}
	var row struct {
		Name      string          `json:"name"`
		Limit     int32           `json:"limit"`
		LimitType int32           `json:"limitType"`
		Cooldown  int32           `json:"cd"`
		Rewards   json.RawMessage `json:"award"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ShareRewardConfig{}, false
	}
	return ShareRewardConfig{
		ID:        id,
		Name:      strings.TrimSpace(row.Name),
		Limit:     row.Limit,
		LimitType: row.LimitType,
		Cooldown:  row.Cooldown,
		Rewards:   readItemCountsRaw(row.Rewards),
	}, true
}

// FmlLandLvlByID returns growth timing/stock for one guild-land level.
func FmlLandLvlByID(level int32) (FmlLandLvl, bool) {
	raw, ok := StaticRow("c_fmlLandLvl", level)
	if !ok {
		return FmlLandLvl{}, false
	}
	var row struct {
		Time  int32 `json:"time"`
		Stock int32 `json:"stock"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return FmlLandLvl{}, false
	}
	if row.Time <= 0 {
		return FmlLandLvl{}, false
	}
	return FmlLandLvl{Level: level, TimeSec: row.Time, Stock: row.Stock}, true
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
	if name := strings.TrimSpace(item.DisplayName); usableCatalogName(name) {
		return name
	}
	name := strings.TrimSpace(item.Name)
	if !usableCatalogName(name) {
		return ""
	}
	return name
}

func usableCatalogName(name string) bool {
	switch strings.TrimSpace(name) {
	case "", "0", "待定":
		return false
	default:
		return true
	}
}

// MsgCodeText returns the Chinese tip text for a server m.code from c_msgCode.
// Empty means the catalog has no usable row for that code.
func MsgCodeText(code int32) string {
	if code == 0 {
		return ""
	}
	raw, ok := StaticRow("c_msgCode", code)
	if !ok {
		return ""
	}
	var row struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ""
	}
	return strings.TrimSpace(row.Text)
}

// ItemLabel returns ItemName, or "#<id>" when the catalog has no usable name.
// Race task cards and op descriptions use this so unresolved flowers stay visible.
func ItemLabel(id int32) string {
	if name := ItemName(id); name != "" {
		return name
	}
	if id > 0 {
		return fmt.Sprintf("#%d", id)
	}
	return ""
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

// FlowerLvlYieldByID returns cropGets/left/frequencys for a flower at cultivation
// level. Prefer the per-flower c_flowerLvl row (key flowerId*100+level); when
// that row is absent or lacks yield fields (newer flowers like 梦紫郁金香),
// fall back to the shared c_flowerLvlCfg row for the same level — matching
// client defaults so race plant sizing does not degrade to 1 flower/plant.
func FlowerLvlYieldByID(flowerID, level int32) (FlowerLvlYield, bool) {
	if level <= 0 {
		return FlowerLvlYield{}, false
	}
	if flowerID > 0 {
		if raw, ok := StaticRow("c_flowerLvl", flowerID*100+level); ok {
			if y, ok := parseFlowerLvlYield(raw, level); ok {
				return y, true
			}
		}
	}
	raw, ok := StaticRow("c_flowerLvlCfg", level)
	if !ok {
		return FlowerLvlYield{}, false
	}
	return parseFlowerLvlYield(raw, level)
}

func parseFlowerLvlYield(raw json.RawMessage, level int32) (FlowerLvlYield, bool) {
	var row struct {
		CropGets   int32 `json:"cropGets"`
		Left       int32 `json:"left"`
		Frequencys int32 `json:"frequencys"`
	}
	if json.Unmarshal(raw, &row) != nil || row.CropGets <= 0 || row.Left < 0 || row.Frequencys <= 0 {
		return FlowerLvlYield{}, false
	}
	return FlowerLvlYield{Level: level, CropGets: row.CropGets, Left: row.Left, Frequencys: row.Frequencys}, true
}

// FlowerLvlCDSeconds returns the catalog grow CD (seconds between harvests)
// for a flower at cultivation level. Matches client G.CFG.getFlowerLvlCfg:
//  1. Prefer the per-level c_flowerLvl row (key flowerId*100+level).
//  2. Else, when a base c_flowerLvl(flowerId) row exists (newer flowers like
//     花笼流芳), scale with calFlowerLvlTime: round(base.cd * cfg(level).cd / cfg(1).cd).
//  3. Else fall back to bare c_flowerLvlCfg(level).cd.
func FlowerLvlCDSeconds(flowerID, level int32) (int32, bool) {
	if level <= 0 {
		return 0, false
	}
	if flowerID > 0 {
		if raw, ok := StaticRow("c_flowerLvl", flowerID*100+level); ok {
			if cd, ok := parseFlowerLvlCD(raw); ok {
				return cd, true
			}
		}
		if cd, ok := flowerLvlCDFromBaseRow(flowerID, level); ok {
			return cd, true
		}
	}
	raw, ok := StaticRow("c_flowerLvlCfg", level)
	if !ok {
		return 0, false
	}
	return parseFlowerLvlCD(raw)
}

// flowerLvlCDFromBaseRow mirrors client scaling for flowers that only publish
// a base c_flowerLvl(flowerId) row (cd/gldCost), not per-level flowerId*100+lvl rows.
func flowerLvlCDFromBaseRow(flowerID, level int32) (int32, bool) {
	baseRaw, ok := StaticRow("c_flowerLvl", flowerID)
	if !ok {
		return 0, false
	}
	baseCD, ok := parseFlowerLvlCD(baseRaw)
	if !ok {
		return 0, false
	}
	levelRaw, ok := StaticRow("c_flowerLvlCfg", level)
	if !ok {
		return 0, false
	}
	levelCD, ok := parseFlowerLvlCD(levelRaw)
	if !ok {
		return 0, false
	}
	baseCfgRaw, ok := StaticRow("c_flowerLvlCfg", 1)
	if !ok {
		return 0, false
	}
	baseCfgCD, ok := parseFlowerLvlCD(baseCfgRaw)
	if !ok || baseCfgCD <= 0 {
		return 0, false
	}
	// calFlowerLvlTime(base, levelCfg, cfg1) => Math.round(base * (levelCfg / cfg1))
	return int32(math.Round(float64(baseCD) * float64(levelCD) / float64(baseCfgCD))), true
}

func parseFlowerLvlCD(raw json.RawMessage) (int32, bool) {
	var row struct {
		CD int32 `json:"cd"`
	}
	if json.Unmarshal(raw, &row) != nil || row.CD <= 0 {
		return 0, false
	}
	return row.CD, true
}

// FlowerUpgradeCostForLevel returns the cost to upgrade a flower from its
// current cultivation level. It mirrors client getFlowerLvlCfg:
//  1. Prefer the per-level c_flowerLvl row (key flowerId*100+level).
//  2. Otherwise combine the flower's base c_flowerLvl row with the shared
//     c_flowerLvlCfg ratio. The shared gldCost is a multiplier input, not the
//     final price: round(base.gldCost * cfg(level).gldCost / cfg(1).gldCost).
//
// Essence item id is always flowerId-1000 (for example 23006 -> 22006).
func FlowerUpgradeCostForLevel(flowerID, level int32) (FlowerUpgradeCost, bool) {
	if level <= 0 {
		return FlowerUpgradeCost{}, false
	}
	itemID := FlowerBouquetItemID(flowerID)
	if itemID == 0 {
		return FlowerUpgradeCost{}, false
	}
	if cost, ok := flowerUpgradeCostFromRaw(StaticRow("c_flowerLvl", flowerID*100+level)); ok {
		cost.ItemID = itemID
		return cost, true
	}
	if cost, ok := flowerUpgradeCostFromBaseRow(flowerID, level); ok {
		cost.ItemID = itemID
		return cost, true
	}
	return FlowerUpgradeCost{}, false
}

func flowerUpgradeCostFromBaseRow(flowerID, level int32) (FlowerUpgradeCost, bool) {
	baseRaw, ok := StaticRow("c_flowerLvl", flowerID)
	if !ok {
		return FlowerUpgradeCost{}, false
	}
	baseGold, ok := parseFlowerLvlGold(baseRaw)
	if !ok {
		return FlowerUpgradeCost{}, false
	}
	levelCost, ok := flowerUpgradeCostFromRaw(StaticRow("c_flowerLvlCfg", level))
	if !ok {
		return FlowerUpgradeCost{}, false
	}
	baseCfgRaw, ok := StaticRow("c_flowerLvlCfg", 1)
	if !ok {
		return FlowerUpgradeCost{}, false
	}
	baseCfgGold, ok := parseFlowerLvlGold(baseCfgRaw)
	if !ok || baseCfgGold <= 0 {
		return FlowerUpgradeCost{}, false
	}
	levelCost.Gold = int32(math.Round(float64(baseGold) * float64(levelCost.Gold) / float64(baseCfgGold)))
	return levelCost, levelCost.Gold > 0
}

func parseFlowerLvlGold(raw json.RawMessage) (int32, bool) {
	var row struct {
		Gold int32 `json:"gldCost"`
	}
	if json.Unmarshal(raw, &row) != nil || row.Gold <= 0 {
		return 0, false
	}
	return row.Gold, true
}

// flowerUpgradeCostFromRaw parses lvlUpCost/gldCost from either a per-level
// c_flowerLvl row (lvlUpCost is [itemId, count]) or a shared c_flowerLvlCfg
// row (lvlUpCost is a bare count). Shared gldCost still needs base-row scaling.
// ItemID is left zero for the caller to fill.
func flowerUpgradeCostFromRaw(raw json.RawMessage, ok bool) (FlowerUpgradeCost, bool) {
	if !ok {
		return FlowerUpgradeCost{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return FlowerUpgradeCost{}, false
	}
	count := parseLvlUpCostCount(row["lvlUpCost"])
	if count <= 0 {
		return FlowerUpgradeCost{}, false
	}
	var gold int32
	if rawGold, ok := row["gldCost"]; ok {
		_ = json.Unmarshal(rawGold, &gold)
	}
	return FlowerUpgradeCost{Count: count, Gold: gold}, true
}

func parseLvlUpCostCount(raw json.RawMessage) int32 {
	if len(raw) == 0 {
		return 0
	}
	var pair []int32
	if json.Unmarshal(raw, &pair) == nil {
		if len(pair) >= 2 {
			return pair[1]
		}
		if len(pair) == 1 {
			return pair[0]
		}
	}
	var count int32
	if json.Unmarshal(raw, &count) == nil {
		return count
	}
	return 0
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
	// catalog_data.json has already gone through the client value-restoration
	// algorithm. In particular, c_flowerArt.vase and c_flowerArt.flowers are
	// the exact values sent by flowerArt.makeFlowerArt; applying the legacy
	// offset transform a second time corrupts recipes with higher flower ids.
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
