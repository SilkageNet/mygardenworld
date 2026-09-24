package state

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

const (
	// FlowerElvesItemType is c_item.type for flower-elf items (110xxx).
	FlowerElvesItemType int32 = 16
	// DefaultFlowerElvesMoneyItemID is c_flowerElves[-1].$moneyId (月令币 / 花灵币).
	DefaultFlowerElvesMoneyItemID int32 = 1014
	// DefaultElvesSpawnCap matches $harvestMax(18)+$addHarvestMax(12) when the
	// flower-elves pass bonus applies; used as the planting-elves round cap.
	DefaultElvesSpawnCap int32 = 30
	// FlowerElvesDoubleActTmpType is c_act id / IActInfo.tmpType for 花灵双倍.
	// Active batches expose eligible item ids via actTmp.ext.commonCfg.il and
	// multiplier via commonCfg.iv (client FlowerElvesPlaceCtrl.canRecvReward).
	FlowerElvesDoubleActTmpType int32 = 4401
	// UsrCountTypeFlowerElvesHarvest is IUsrCount.type for daily own-land
	// flower-elves harvest. Client flower-elves widget:
	// usrCtrl.getTdyCount(103) / ($harvestMax [+ $addHarvestMax]).
	UsrCountTypeFlowerElvesHarvest int32 = 103
)

// FlowerElvesGlobals are catalog constants from c_flowerElves id=-1.
type FlowerElvesGlobals struct {
	HarvestMax    int32
	AddHarvestMax int32
	SneakMax      int32
	SneakNumMax   int32
	ElvesLimit    int32
	MoneyItemID   int32
	// Friend help (花灵协助) knobs.
	FriendHelpNum   int32 // helpers required before recvAidEff
	FriendTimeMin   int32 // cooldown minutes between reqAid
	HelpMax         int32 // daily helpFrd cap
	NoticeTimeHours int32 // request visibility window
	FriendAddRate   int32 // percent buff from claimed aid
}

// FlowerElvesPlaceView is one dispatch slot from namespace 132.2 (placeMap).
type FlowerElvesPlaceView struct {
	PlaceID       int32
	ElvesID       int32
	ElvesNum      int32
	DispEndTimeMs int64
	GainMulti     int32
	Iid           int32
}

// FlowerElvesHouseView is the monitoring snapshot for the flower-elves house.
type FlowerElvesHouseView struct {
	PlacesObserved      bool
	MoneyItemID         int32
	MoneyCount          int32
	DispatchableCount   int32
	DispatchedCount     int32
	ElvesLimit          int32
	PlantedCount        int32
	PlantedCap          int32
	PlantedObserved     bool
	HarvestableCount    int32
	HarvestableCap      int32
	HarvestableObserved bool
	SlotCount           int32
	PendingRewardMoney  int32
	Places              []FlowerElvesPlaceView
	// Friend-aid buff (namespace 132.5).
	AidObserved      bool
	AidEffEndTimeMs  int64
	AidFriendAddRate int32 // catalog $friendAddRate percent
	AidReqOpen       bool
	AidHelperCount   int32
	AidPreReqTimeMs  int64
	AidReqReadyAtMs  int64 // 0 when reqAid is allowed now
	AidCanRecv       bool
	// Undispatched inventory breakdown (color × double eligibility).
	DoubleBuffActive bool
	InventoryGroups  []FlowerElvesInventoryGroup
}

// FlowerElvesInventoryItem is one undispatched elf stack in a color/double bucket.
type FlowerElvesInventoryItem struct {
	ItemID int32
	Name   string
	Count  int32
}

// FlowerElvesInventoryGroup is one monitor bucket for undispatched elves.
// Color 2/3/4 → 蓝/紫/金; Score is $elvesMoney[color-1] (×iv when double).
type FlowerElvesInventoryGroup struct {
	Color    int32
	Score    int32
	IsDouble bool
	Count    int32
	Items    []FlowerElvesInventoryItem
}

// UsrCountView is one G.IUsrCount row from IUsrTot.cntMap (namespace 7.4).
type UsrCountView struct {
	Type     int32
	TdyCnt   int32
	TotCnt   int32
	RTimeMs  int64
	Observed bool
}

// FlowerElvesBookPair is one c_flowerElvesBook main/secondary/elves row.
type FlowerElvesBookPair struct {
	BookID          int32
	MainFlowerID    int32
	SecondaryFlower int32
	FlowerElvesItem int32
}

// FrdStealRcdView is one IFrdStealRcd entry from frdSteal.getFrdStealRcdList.
type FrdStealRcdView struct {
	FrdUID   int64
	StealMap map[int32]int32 // itemId -> count
	CTimeMs  int64
}

// FlowerElvesGlobalsFromCatalog loads c_flowerElves global row.
func FlowerElvesGlobalsFromCatalog() (FlowerElvesGlobals, bool) {
	raw, ok := StaticRow("c_flowerElves", -1)
	if !ok {
		return FlowerElvesGlobals{}, false
	}
	var row struct {
		HarvestMax    int32 `json:"$harvestMax"`
		AddHarvestMax int32 `json:"$addHarvestMax"`
		SneakMax      int32 `json:"$sneakMax"`
		SneakNumMax   int32 `json:"$sneakNumMax"`
		ElvesLimit    int32 `json:"$elvesLimit"`
		MoneyID       int32 `json:"$moneyId"`
		FriendHelpNum int32 `json:"$friendHelpNum"`
		FriendTime    int32 `json:"$friendTime"`
		HelpMax       int32 `json:"$helpMax"`
		NoticeTime    int32 `json:"$noticeTime"`
		FriendAddRate int32 `json:"$friendAddRate"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return FlowerElvesGlobals{}, false
	}
	if row.HarvestMax <= 0 {
		row.HarvestMax = 18
	}
	if row.SneakMax <= 0 {
		row.SneakMax = 4
	}
	if row.SneakNumMax <= 0 {
		row.SneakNumMax = 1
	}
	if row.MoneyID <= 0 {
		row.MoneyID = DefaultFlowerElvesMoneyItemID
	}
	if row.FriendHelpNum <= 0 {
		row.FriendHelpNum = 1
	}
	if row.FriendTime <= 0 {
		row.FriendTime = 150
	}
	if row.HelpMax <= 0 {
		row.HelpMax = 5
	}
	if row.NoticeTime <= 0 {
		row.NoticeTime = 24
	}
	return FlowerElvesGlobals{
		HarvestMax:      row.HarvestMax,
		AddHarvestMax:   row.AddHarvestMax,
		SneakMax:        row.SneakMax,
		SneakNumMax:     row.SneakNumMax,
		ElvesLimit:      row.ElvesLimit,
		MoneyItemID:     row.MoneyID,
		FriendHelpNum:   row.FriendHelpNum,
		FriendTimeMin:   row.FriendTime,
		HelpMax:         row.HelpMax,
		NoticeTimeHours: row.NoticeTime,
		FriendAddRate:   row.FriendAddRate,
	}, true
}

// ActiveFlowerElvesMoneyItemID picks the seasonal shop currency from
// c_flowerElvesMoney for now (unix-second start/end windows). Overlapping
// windows prefer the latest startTime.
func ActiveFlowerElvesMoneyItemID(now time.Time) (int32, bool) {
	table, ok := StaticTableByName("c_flowerElvesMoney")
	if !ok || len(table.Rows) == 0 {
		return 0, false
	}
	nowSec := now.Unix()
	var bestID int32
	var bestStart int64
	found := false
	for key, raw := range table.Rows {
		if key == "-1" {
			continue
		}
		var row struct {
			ID        int32 `json:"id"`
			StartTime int64 `json:"startTime"`
			EndTime   int64 `json:"endTime"`
		}
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		id := row.ID
		if id <= 0 {
			id = atoiCatalogID(key)
		}
		if id <= 0 || row.StartTime <= 0 || row.EndTime <= 0 {
			continue
		}
		if nowSec < row.StartTime || nowSec >= row.EndTime {
			continue
		}
		if !found || row.StartTime >= bestStart {
			found = true
			bestStart = row.StartTime
			bestID = id
		}
	}
	return bestID, found
}

// ResolveElvesSpawnCap returns policy cap or the catalog/default 30.
func ResolveElvesSpawnCap(policyCap int32) int32 {
	if policyCap > 0 {
		return policyCap
	}
	g, ok := FlowerElvesGlobalsFromCatalog()
	if ok && g.HarvestMax > 0 {
		cap := g.HarvestMax + g.AddHarvestMax
		if cap > 0 {
			return cap
		}
	}
	return DefaultElvesSpawnCap
}

// FlowerElvesMoneyRatesFromCatalog returns c_flowerElvesBook[-1].$elvesMoney
// (per item color 1..N). Missing catalog falls back to [1,2,3,4].
func FlowerElvesMoneyRatesFromCatalog() []int32 {
	raw, ok := StaticRow("c_flowerElvesBook", -1)
	if !ok {
		return []int32{1, 2, 3, 4}
	}
	var row struct {
		ElvesMoney []int32 `json:"$elvesMoney"`
	}
	if json.Unmarshal(raw, &row) != nil || len(row.ElvesMoney) == 0 {
		return []int32{1, 2, 3, 4}
	}
	return append([]int32(nil), row.ElvesMoney...)
}

// FlowerElvesDispatchRewardMoney mirrors FlowerElvesPlaceCtrl.canRecvReward:
// elvesNum * $elvesMoney[color-1] * max(gainMulti, 1).
func FlowerElvesDispatchRewardMoney(elvesID, elvesNum, gainMulti int32) int32 {
	if elvesID <= 0 || elvesNum <= 0 {
		return 0
	}
	info, ok := ItemInfoByID(elvesID)
	if !ok {
		return 0
	}
	rates := FlowerElvesMoneyRatesFromCatalog()
	rate := rates[0]
	if info.Color >= 1 && int(info.Color) <= len(rates) {
		rate = rates[info.Color-1]
	}
	multi := gainMulti
	if multi <= 0 {
		multi = 1
	}
	return elvesNum * rate * multi
}

// FlowerElvesPlaceIDsFromCatalog lists c_flowerElves place slot ids (id>0).
func FlowerElvesPlaceIDsFromCatalog() []int32 {
	table, ok := StaticTableByName("c_flowerElves")
	if !ok || len(table.Rows) == 0 {
		return nil
	}
	ids := make([]int32, 0, len(table.Rows))
	for key := range table.Rows {
		id := atoiCatalogID(key)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// FlowerElvesBookByPair finds the book row for a main+secondary flower pair.
func FlowerElvesBookByPair(mainFlowerID, secondaryFlowerID int32) (FlowerElvesBookPair, bool) {
	if mainFlowerID <= 0 || secondaryFlowerID <= 0 {
		return FlowerElvesBookPair{}, false
	}
	for _, pair := range FlowerElvesBookPairs() {
		if pair.MainFlowerID == mainFlowerID && pair.SecondaryFlower == secondaryFlowerID {
			return pair, true
		}
	}
	return FlowerElvesBookPair{}, false
}

// FlowerElvesBookPairs lists all catalog book pairings.
func FlowerElvesBookPairs() []FlowerElvesBookPair {
	table, ok := StaticTableByName("c_flowerElvesBook")
	if !ok || len(table.Rows) == 0 {
		return nil
	}
	out := make([]FlowerElvesBookPair, 0, len(table.Rows))
	ids := make([]int32, 0, len(table.Rows))
	for key := range table.Rows {
		id := atoiCatalogID(key)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		raw, ok := table.Rows[strconv.FormatInt(int64(id), 10)]
		if !ok {
			continue
		}
		var row struct {
			ID              int32 `json:"id"`
			MainFlower      int32 `json:"mainFlower"`
			SecondaryFlower int32 `json:"secondaryFlower"`
			FlowerElves     int32 `json:"flowerElves"`
		}
		if json.Unmarshal(raw, &row) != nil || row.MainFlower <= 0 || row.SecondaryFlower <= 0 {
			continue
		}
		out = append(out, FlowerElvesBookPair{
			BookID:          id,
			MainFlowerID:    row.MainFlower,
			SecondaryFlower: row.SecondaryFlower,
			FlowerElvesItem: row.FlowerElves,
		})
	}
	return out
}

// SecondaryFlowersForMain returns vice/secondary flower ids that pair with main.
func SecondaryFlowersForMain(mainFlowerID int32) []int32 {
	if mainFlowerID <= 0 {
		return nil
	}
	seen := map[int32]struct{}{}
	var out []int32
	for _, pair := range FlowerElvesBookPairs() {
		if pair.MainFlowerID != mainFlowerID {
			continue
		}
		if _, ok := seen[pair.SecondaryFlower]; ok {
			continue
		}
		seen[pair.SecondaryFlower] = struct{}{}
		out = append(out, pair.SecondaryFlower)
	}
	return out
}

// ItemIsFlowerElves reports c_item.type == 16.
func ItemIsFlowerElves(itemID int32) bool {
	info, ok := ItemInfoByID(itemID)
	return ok && info.Type == FlowerElvesItemType
}

// FlowerElvesMoneyRateForColor returns $elvesMoney[color-1] (fallback 1).
func FlowerElvesMoneyRateForColor(color int32) int32 {
	rates := FlowerElvesMoneyRatesFromCatalog()
	if color >= 1 && int(color) <= len(rates) && rates[color-1] > 0 {
		return rates[color-1]
	}
	if len(rates) > 0 && rates[0] > 0 {
		return rates[0]
	}
	return 1
}

// flowerElvesDoubleBuffLocked returns the active 4401 item set and multiplier.
// Mirrors client getActiveActByTmpType(4401) + actTmp.ext.commonCfg.{il,iv}.
func (s *State) flowerElvesDoubleBuffLocked(nowMs int64) (map[int32]struct{}, int32, bool) {
	var selected *activityBatchState
	for _, batch := range s.activityBatches {
		if batch == nil || batch.TmpType != FlowerElvesDoubleActTmpType || batch.Status != 1 {
			continue
		}
		if batch.BeginMs > 0 && nowMs < batch.BeginMs {
			continue
		}
		if batch.EndMs > 0 && nowMs >= batch.EndMs {
			continue
		}
		if selected == nil || batch.BeginMs > selected.BeginMs || (batch.BeginMs == selected.BeginMs && batch.BatchID > selected.BatchID) {
			selected = batch
		}
	}
	if selected == nil || selected.TmpID <= 0 {
		return nil, 0, false
	}
	template := s.activityTemplates[selected.TmpID]
	if template == nil || !template.CommonCfgObserved {
		return nil, 0, true
	}
	multi := template.CommonCfgIV
	if multi <= 0 {
		multi = 2
	}
	set := make(map[int32]struct{}, len(template.CommonCfgIL))
	for _, id := range template.CommonCfgIL {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	return set, multi, true
}

func buildFlowerElvesInventoryGroups(inventory map[int32]int32, doubleIDs map[int32]struct{}, doubleMulti int32, doubleActive bool) []FlowerElvesInventoryGroup {
	type bucketKey struct {
		color    int32
		isDouble bool
	}
	type bucketAcc struct {
		count int32
		items map[int32]int32
	}
	acc := map[bucketKey]*bucketAcc{}
	ensure := func(color int32, isDouble bool) *bucketAcc {
		key := bucketKey{color: color, isDouble: isDouble}
		if b := acc[key]; b != nil {
			return b
		}
		b := &bucketAcc{items: map[int32]int32{}}
		acc[key] = b
		return b
	}
	for itemID, count := range inventory {
		if count <= 0 || !ItemIsFlowerElves(itemID) {
			continue
		}
		info, ok := ItemInfoByID(itemID)
		if !ok {
			continue
		}
		color := info.Color
		if color < 2 || color > 4 {
			continue
		}
		isDouble := false
		if doubleActive && doubleMulti > 1 {
			_, isDouble = doubleIDs[itemID]
		}
		b := ensure(color, isDouble)
		b.count += count
		b.items[itemID] += count
	}

	order := []bucketKey{
		{2, false}, {3, false}, {4, false},
		{2, true}, {3, true}, {4, true},
	}
	out := make([]FlowerElvesInventoryGroup, 0, len(order))
	for _, key := range order {
		b := acc[key]
		if b == nil {
			b = &bucketAcc{}
		}
		rate := FlowerElvesMoneyRateForColor(key.color)
		score := rate
		if key.isDouble {
			multi := doubleMulti
			if multi <= 1 {
				multi = 2
			}
			score = rate * multi
		}
		group := FlowerElvesInventoryGroup{
			Color:    key.color,
			Score:    score,
			IsDouble: key.isDouble,
			Count:    b.count,
		}
		if len(b.items) > 0 {
			ids := make([]int32, 0, len(b.items))
			for id := range b.items {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool {
				if b.items[ids[i]] != b.items[ids[j]] {
					return b.items[ids[i]] > b.items[ids[j]]
				}
				return ids[i] < ids[j]
			})
			group.Items = make([]FlowerElvesInventoryItem, 0, len(ids))
			for _, id := range ids {
				group.Items = append(group.Items, FlowerElvesInventoryItem{
					ItemID: id,
					Name:   ItemName(id),
					Count:  b.items[id],
				})
			}
		}
		out = append(out, group)
	}
	return out
}

func (s *State) noteElvesSpawnLocked(lands map[int32]LandView) {
	for lid, land := range lands {
		if land.ElvesID != 0 {
			s.noteElvesSpawnLandLocked(lid)
		}
	}
}

func (s *State) noteElvesSpawnLandLocked(landID int32) {
	if landID <= 0 {
		return
	}
	if s.elvesProducedLands == nil {
		s.elvesProducedLands = make(map[int32]struct{})
	}
	if _, ok := s.elvesProducedLands[landID]; ok {
		return
	}
	s.elvesProducedLands[landID] = struct{}{}
	if s.elvesRoundStartMs == 0 {
		s.elvesRoundStartMs = s.lastApplyMs
		if s.elvesRoundStartMs == 0 {
			s.elvesRoundStartMs = time.Now().UnixMilli()
		}
	}
}

// ElvesProducedCount is the sticky count of own lands that showed an elf this round.
func (s *State) ElvesProducedCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int32(len(s.elvesProducedLands))
}

// ElvesRoundStartMs is when the current planting-elves round first saw an elf.
func (s *State) ElvesRoundStartMs() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.elvesRoundStartMs
}

// ClearElvesRound resets sticky spawn tracking after a completed harvest round.
func (s *State) ClearElvesRound() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.elvesProducedLands = nil
	s.elvesRoundStartMs = 0
}

// EnsureElvesRoundStarted marks round start when planting elves without a prior elf.
func (s *State) EnsureElvesRoundStarted(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.elvesRoundStartMs == 0 {
		s.elvesRoundStartMs = now.UnixMilli()
	}
}

// CountStealRcdPicks sums flower-elf and secondary-flower items from steal records
// with cTime at or after sinceMs (0 = all records).
func (s *State) CountStealRcdPicks(elvesItemID, secondaryFlowerID int32, sinceMs int64) (elvesPicked, secondaryPicked int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rcd := range s.frdStealRcdList {
		if sinceMs > 0 && rcd.CTimeMs > 0 && rcd.CTimeMs < sinceMs {
			continue
		}
		for itemID, n := range rcd.StealMap {
			if n <= 0 {
				continue
			}
			if elvesItemID > 0 && itemID == elvesItemID {
				elvesPicked += n
				continue
			}
			if secondaryFlowerID > 0 && itemID == secondaryFlowerID {
				secondaryPicked += n
			}
		}
	}
	return elvesPicked, secondaryPicked
}

// ElvesPickedDone reports elvesPicked+secondaryPicked >= produced.
func ElvesPickedDone(elvesPicked, secondaryPicked, produced int32) bool {
	if produced <= 0 {
		return false
	}
	return elvesPicked+secondaryPicked >= produced
}

// StealElvesCnt returns today's friend elf-steal usage from IFrdSteal.
// Prefer StealElvesCntAt when a clock is available so day-rollover matches the client.
func (s *State) StealElvesCnt() int32 {
	return s.StealElvesCntAt(time.Now())
}

// StealElvesCntAt mirrors FrdStealCtrl.getStealElvesCnt: after rTime day refresh the
// count is treated as 0 even if the cached field is non-zero.
func (s *State) StealElvesCntAt(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.frdStealRTimeMs > 0 && !frdStealMapFresh(s.frdStealRTimeMs, now) {
		return 0
	}
	return s.frdStealElvesCnt
}

// FrdStealRcdObserved reports whether getFrdStealRcdList (or equivalent) applied.
func (s *State) FrdStealRcdObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.frdStealRcdObserved
}

// FriendLandsPlantingElves reports whether a friend garden is still waiting for
// flower elves to spawn: catalog secondary flower present and no plot has
// elvesId yet. Used to poll more often until spawn; once elves are visible,
// refresh slows down (steal immediately if possible; already-taken elves will
// not become stealable again until the next plant round).
func FriendLandsPlantingElves(lands map[int32]LandView) bool {
	if len(lands) == 0 {
		return false
	}
	secondaries := flowerElvesSecondaryFlowerSet()
	hasSecondary := false
	for _, land := range lands {
		if land.ElvesID != 0 {
			return false
		}
		if land.FlowerID == 0 {
			continue
		}
		if _, ok := secondaries[int32(land.FlowerID)]; ok {
			hasSecondary = true
		}
	}
	return hasSecondary
}

func flowerElvesSecondaryFlowerSet() map[int32]struct{} {
	pairs := FlowerElvesBookPairs()
	out := make(map[int32]struct{}, len(pairs))
	for _, pair := range pairs {
		if pair.SecondaryFlower > 0 {
			out[pair.SecondaryFlower] = struct{}{}
		}
	}
	return out
}

// PickFriendStealElvesLand picks a land with a still-stealable flower elf and
// returns its land id plus the planted elvesId.
func PickFriendStealElvesLand(lands map[int32]LandView, now time.Time) (landID int32, elvesID int32, ok bool) {
	return PickFriendStealElvesLandFor(lands, now, 0, nil)
}

// FriendStealElvesLandSkip reports whether a land should be ignored while
// choosing an elf-steal target (sticky server rejects / false flower steals).
type FriendStealElvesLandSkip func(landID int32, land LandView) bool

// PickFriendStealElvesLandFor picks a stealable elf land, optionally excluding
// plots this account already flower-stole and sticky per-land skips.
func PickFriendStealElvesLandFor(lands map[int32]LandView, now time.Time, selfUID int64, skip FriendStealElvesLandSkip) (landID int32, elvesID int32, ok bool) {
	if len(lands) == 0 {
		return 0, 0, false
	}
	ids := make([]int32, 0, len(lands))
	for id, land := range lands {
		if !land.HasStealableElvesFor(selfUID, now) {
			continue
		}
		if skip != nil && skip(id, land) {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return 0, 0, false
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	landID = ids[0]
	if land, found := lands[landID]; found {
		elvesID = int32(land.ElvesID)
	}
	return landID, elvesID, true
}

// PickFriendStealElvesLandID picks a land with a still-stealable flower elf.
func PickFriendStealElvesLandID(lands map[int32]LandView, now time.Time) (int32, bool) {
	landID, _, ok := PickFriendStealElvesLand(lands, now)
	return landID, ok
}

func (s *State) applyFlowerElvesLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if s.flowerElvesPlaces == nil {
		s.flowerElvesPlaces = make(map[int32]*FlowerElvesPlaceView)
	}
	if rawPlaces, ok := fields["2"]; ok {
		// Client ignores a null/falsy placeMap. Only null entries inside an
		// observed map carry deletion semantics.
		if !isJSONNull(rawPlaces) {
			var places map[string]json.RawMessage
			if err := json.Unmarshal(rawPlaces, &places); err == nil {
				for placeIDStr, rawPlace := range places {
					parsedID, err := strconv.ParseInt(placeIDStr, 10, 32)
					if err != nil || parsedID <= 0 {
						continue
					}
					placeID := int32(parsedID)
					if isJSONNull(rawPlace) {
						delete(s.flowerElvesPlaces, placeID)
						continue
					}
					view := s.flowerElvesPlaces[placeID]
					if view == nil {
						view = &FlowerElvesPlaceView{PlaceID: placeID}
						s.flowerElvesPlaces[placeID] = view
					}
					applyFlowerElvesPlaceFields(view, rawPlace)
				}
				s.flowerElvesPlacesObserved = true
			}
		}
	}
	s.applyFlowerElvesPassLocked(fields["3"], fields["4"])
	if rawAid, ok := fields["5"]; ok {
		s.applyFlowerElvesAidLocked(rawAid)
	}
}

func applyFlowerElvesPlaceFields(view *FlowerElvesPlaceView, raw json.RawMessage) {
	if view == nil || len(raw) == 0 || string(raw) == "{}" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
		view.PlaceID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.ElvesID = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.ElvesNum = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.DispEndTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.GainMulti = n
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.Iid = n
	}
	if view.ElvesID == 0 || view.ElvesNum == 0 {
		view.ElvesID = 0
		view.ElvesNum = 0
		view.DispEndTimeMs = 0
		view.GainMulti = 0
		view.Iid = 0
	}
}

// FlowerElvesHouse builds the flower-elves house monitoring snapshot.
func (s *State) FlowerElvesHouse() FlowerElvesHouseView {
	return s.FlowerElvesHouseAt(time.Now())
}

// FlowerElvesHouseAt builds the house snapshot at a fixed clock (tests / query).
func (s *State) FlowerElvesHouseAt(now time.Time) FlowerElvesHouseView {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := FlowerElvesGlobalsFromCatalog()
	moneyID := DefaultFlowerElvesMoneyItemID
	if activeID, activeOK := ActiveFlowerElvesMoneyItemID(now); activeOK {
		moneyID = activeID
	} else if ok && g.MoneyItemID > 0 {
		moneyID = g.MoneyItemID
	}
	elvesLimit := int32(0)
	sneakMax := int32(4)
	if ok {
		elvesLimit = g.ElvesLimit
		if g.SneakMax > 0 {
			sneakMax = g.SneakMax
		}
	}

	planted, plantedObs := s.getTdyCountLocked(UsrCountTypeFlowerElvesHarvest, now)
	stealElvesCnt := s.frdStealElvesCnt
	if s.frdStealRTimeMs > 0 && !frdStealMapFresh(s.frdStealRTimeMs, now) {
		stealElvesCnt = 0
	}
	out := FlowerElvesHouseView{
		PlacesObserved:      s.flowerElvesPlacesObserved,
		MoneyItemID:         moneyID,
		MoneyCount:          s.inventory[moneyID],
		ElvesLimit:          elvesLimit,
		PlantedCount:        planted,
		PlantedCap:          ResolveElvesSpawnCap(0),
		PlantedObserved:     plantedObs,
		HarvestableCount:    stealElvesCnt,
		HarvestableCap:      sneakMax,
		HarvestableObserved: s.frdStealObserved,
		AidObserved:         s.flowerElvesAidObserved,
		AidEffEndTimeMs:     s.flowerElvesAid.EffEndTime,
		AidReqOpen:          s.flowerElvesAid.ReqAid != 0,
		AidHelperCount:      int32(len(s.flowerElvesAid.AidMap)),
		AidPreReqTimeMs:     s.flowerElvesAid.PreReqAidTime,
	}
	if ok && g.FriendAddRate > 0 {
		out.AidFriendAddRate = g.FriendAddRate
	}
	if s.flowerElvesAidObserved {
		out.AidCanRecv = s.canRecvFlowerElvesAidLocked(now)
		if readyAt, blocked := s.flowerElvesAidReqReadyAtLocked(now); blocked {
			out.AidReqReadyAtMs = readyAt
		}
	}
	for itemID, count := range s.inventory {
		if count <= 0 || !ItemIsFlowerElves(itemID) {
			continue
		}
		out.DispatchableCount += count
	}
	doubleIDs, doubleMulti, doubleActive := s.flowerElvesDoubleBuffLocked(now.UnixMilli())
	out.DoubleBuffActive = doubleActive
	out.InventoryGroups = buildFlowerElvesInventoryGroups(s.inventory, doubleIDs, doubleMulti, doubleActive)

	placeIDs := FlowerElvesPlaceIDsFromCatalog()
	seen := map[int32]struct{}{}
	for _, id := range placeIDs {
		seen[id] = struct{}{}
	}
	for id := range s.flowerElvesPlaces {
		if _, ok := seen[id]; ok {
			continue
		}
		placeIDs = append(placeIDs, id)
	}
	sort.Slice(placeIDs, func(i, j int) bool { return placeIDs[i] < placeIDs[j] })
	out.SlotCount = int32(len(placeIDs))

	if s.flowerElvesPlacesObserved {
		out.Places = make([]FlowerElvesPlaceView, 0, len(placeIDs))
		for _, id := range placeIDs {
			cp := FlowerElvesPlaceView{PlaceID: id}
			if place := s.flowerElvesPlaces[id]; place != nil {
				cp = *place
				if cp.PlaceID <= 0 {
					cp.PlaceID = id
				}
			}
			out.Places = append(out.Places, cp)
			if cp.ElvesNum > 0 {
				out.DispatchedCount += cp.ElvesNum
			}
			out.PendingRewardMoney += FlowerElvesDispatchRewardMoney(cp.ElvesID, cp.ElvesNum, cp.GainMulti)
		}
	}
	if out.SlotCount == 0 {
		out.SlotCount = int32(len(out.Places))
	}
	return out
}
