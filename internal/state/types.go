package state

import (
	"encoding/json"
	"time"
)

const FlowerSeedLow = 23000

// FlowerSeedHigh is the exclusive upper itemId for flower seeds.
const FlowerSeedHigh = 24000

// LandView mirrors the G.ILand schema: per-land state observed on the wire.
//
//	field 0 = flowerId
//	field 1 = state (1=just planted, 3=initial bloom ready, 2=regrowing)
//	field 2 = lvl (planted flower cultivation level)
//	field 3 = harvestCnt (times this plant has been harvested)
//	field 4 = stealUids (uids that already stole from this plot)
//	field 5 = nextTime (ms; next state transition - regrow ready)
//	field 6 = elvesId (flower elf occupying this plot)
//	field 7 = plantTime (ms; last plant/state-change tick)
//	field 8 = elvesStealUids (uids that already attempted the flower elf)
type LandView struct {
	FlowerID       int     `json:"flower_id,omitempty"`
	State          int     `json:"state,omitempty"`
	Lvl            int     `json:"lvl,omitempty"`
	HarvestCnt     int     `json:"harvest_cnt,omitempty"`
	StealUIDs      []int64 `json:"steal_uids,omitempty"`
	NextTimeMs     int64   `json:"next_time_ms,omitempty"`
	ElvesID        int     `json:"elves_id,omitempty"`
	PlantTimeMs    int64   `json:"plant_time_ms,omitempty"`
	ElvesStealUIDs []int64 `json:"elves_steal_uids,omitempty"`

	// Observed = the server has confirmed this land's state at least once
	// (including the empty-after-harvest state). Distinguishes "land we have
	// never seen" from "land we know is empty and ready to plant".
	Observed bool `json:"observed,omitempty"`
}

// IsPlanted returns true when a flower id is set on the land.
func (l LandView) IsPlanted() bool { return l.FlowerID != 0 }

// FromPrimary builds a LandView from the raw `100.1.<id>` JSON dict. Server
// responses use numeric-string keys ("0".."9"), per the G.ILand schema.
func FromPrimary(raw map[string]any) LandView {
	v := LandView{Observed: true}
	v.FlowerID = readInt(raw, "0")
	v.State = readInt(raw, "1")
	v.Lvl = readInt(raw, "2")
	v.HarvestCnt = readInt(raw, "3")
	v.StealUIDs = readInt64Slice(raw, "4")
	v.NextTimeMs = readInt64(raw, "5")
	v.ElvesID = readInt(raw, "6")
	v.PlantTimeMs = readInt64(raw, "7")
	v.ElvesStealUIDs = readInt64Slice(raw, "8")
	return v
}

// EmptyObserved is what we record after a harvest clears the land
// (server sends `100.1.<id> = {}`) - we still mark it observed so the
// automation engine knows the slot is plant-ready, not unknown.
func EmptyObserved() LandView { return LandView{Observed: true} }

// ToJSON returns the LandView as JSON for event emission.
func (l LandView) ToJSON() map[string]any {
	return map[string]any{
		"flowerId":       l.FlowerID,
		"state":          l.State,
		"lvl":            l.Lvl,
		"harvestCnt":     l.HarvestCnt,
		"stealUids":      l.StealUIDs,
		"nextTime":       l.NextTimeMs,
		"elvesId":        l.ElvesID,
		"plantTime":      l.PlantTimeMs,
		"elvesStealUids": l.ElvesStealUIDs,
		"observed":       l.Observed,
	}
}

// CultivateView mirrors the G.ICultivate schema from namespace 101.
//
//	field 1 = flowerId
//	field 2 = lvl (cultivation level; catalog max via FlowerMaxLevel)
//	field 3 = culTime (ms; cultivation completion timestamp)
//	field 4 = status (0=idle, 1=cultivating, 2=received/ready for upgrade)
//	field 5 = uTime (ms; last update)
type CultivateView struct {
	FlowerID  int32 `json:"flower_id"`
	Lvl       int32 `json:"lvl"`
	CulTimeMs int64 `json:"cul_time_ms"`
	Status    int32 `json:"status"`
	UTimeMs   int64 `json:"u_time_ms"`
}

// FlowerOrder represents a resident order box from namespace 105 (orderFlower).
type FlowerOrder struct {
	BoxID        int32           `json:"box_id"`
	NPCID        int32           `json:"npc_id,omitempty"`
	DialogID     int32           `json:"dialog_id,omitempty"`
	Requires     []FlowerRequire `json:"requires"`
	IsVideo      int32           `json:"is_video,omitempty"`
	CdTimeMs     int64           `json:"cd_time_ms,omitempty"`
	CTimeMs      int64           `json:"c_time_ms,omitempty"`
	PlaceIdx     int32           `json:"place_idx,omitempty"`
	VideoRewards []ItemCount     `json:"video_rewards,omitempty"`
}

func (o *FlowerOrder) CooldownReady(now time.Time) bool {
	if o == nil || o.CdTimeMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(o.CdTimeMs))
}

func (o *FlowerOrder) CooldownRemaining(now time.Time) time.Duration {
	if o == nil || o.CdTimeMs <= 0 {
		return 0
	}
	remaining := time.UnixMilli(o.CdTimeMs).Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CustomerOrder represents a customer order from namespace 109 (orderCustomer).
type CustomerOrder struct {
	NPCID        int32           `json:"npc_id"`
	Requires     []FlowerRequire `json:"requires,omitempty"`
	ItemRequires []ItemRequire   `json:"item_requires,omitempty"`
	FinishCnt    int32           `json:"finish_cnt,omitempty"`
}

// CustomerOrderSummary tracks namespace 109 metadata outside the active order
// map. nextGenTime is the server/client cooldown used before genOrder.
type CustomerOrderSummary struct {
	Observed      bool  `json:"observed,omitempty"`
	NextGenTimeMs int64 `json:"next_gen_time_ms,omitempty"`
	UpdatedAtMs   int64 `json:"updated_at_ms,omitempty"`
	CreatedAtMs   int64 `json:"created_at_ms,omitempty"`
	CreateCount   int32 `json:"create_count,omitempty"`
	ActiveCount   int32 `json:"active_count,omitempty"`
}

// ResidentSpecialOrder represents satin/decorate resident orders embedded in
// namespace 105 (orderSatin / orderDecorate). Field 0 is [[flowerId,count],...]
// in the live mini client (same shape as ordinary flower-order requires).
type ResidentSpecialOrder struct {
	Observed     bool            `json:"observed,omitempty"`
	Requires     []FlowerRequire `json:"requires,omitempty"`
	NPCID        int32           `json:"npc_id,omitempty"`
	DialogID     int32           `json:"dialog_id,omitempty"`
	FinishCnt    int32           `json:"finish_cnt,omitempty"`
	IsVideo      int32           `json:"is_video,omitempty"`
	VideoRewards []ItemCount     `json:"video_rewards,omitempty"`
	CdTimeMs     int64           `json:"cd_time_ms,omitempty"`
	CTimeMs      int64           `json:"c_time_ms,omitempty"`
}

func (o ResidentSpecialOrder) CooldownReady(now time.Time) bool {
	if o.CdTimeMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(o.CdTimeMs))
}

// PalaceOrderView is the tracked subset of namespace 108 (orderPalaceTot).
type PalaceOrderView struct {
	Observed bool  `json:"observed,omitempty"`
	UID      int64 `json:"uid,omitempty"`
	FlowerID int32 `json:"flower_id,omitempty"`
	Num      int32 `json:"num,omitempty"`
	IsFinish int32 `json:"is_finish,omitempty"`
	LTimeMs  int64 `json:"l_time_ms,omitempty"`
	UTimeMs  int64 `json:"u_time_ms,omitempty"`
	CTimeMs  int64 `json:"c_time_ms,omitempty"`
}

// TeamOrderView is the tracked subset of namespace 107 (orderTeamTot).
type TeamOrderView struct {
	Observed      bool  `json:"observed,omitempty"`
	UID           int64 `json:"uid,omitempty"`
	Status        int32 `json:"status,omitempty"`
	StartTimeMs   int64 `json:"start_time_ms,omitempty"`
	OrderNum      int32 `json:"order_num,omitempty"`
	FlowerID      int32 `json:"flower_id,omitempty"`
	Reward        int32 `json:"reward,omitempty"`
	RemainingNum  int32 `json:"remaining_num,omitempty"`
	RefreshNotCnt int32 `json:"refresh_not_cnt,omitempty"`
	UTimeMs       int64 `json:"u_time_ms,omitempty"`
	CTimeMs       int64 `json:"c_time_ms,omitempty"`
	ActiveTimeMs  int64 `json:"active_time_ms,omitempty"`
	ActiveCnt     int32 `json:"active_cnt,omitempty"`
	NPCID         int32 `json:"npc_id,omitempty"`
}

// FlowerRackSlot represents one shelf slot from namespace 104 (flowerRack).
type FlowerRackSlot struct {
	RackID        int32 `json:"rack_id"`
	ItemID        int32 `json:"item_id,omitempty"`
	Count         int32 `json:"count,omitempty"`
	ListedAtMs    int64 `json:"listed_at_ms,omitempty"`
	SellReadyAtMs int64 `json:"sell_ready_at_ms,omitempty"`
	UpdatedAtMs   int64 `json:"updated_at_ms,omitempty"`
}

// MailView is the tracked subset of namespace 19 (mailTot.list) needed for
// ordinary mail reward collection.
type MailView struct {
	MsID     int32           `json:"ms_id,omitempty"`
	AllID    int32           `json:"all_id,omitempty"`
	IsDel    int32           `json:"is_del,omitempty"`
	IsRead   int32           `json:"is_read,omitempty"`
	IsPick   int32           `json:"is_pick,omitempty"`
	ItemsRaw json.RawMessage `json:"items_raw,omitempty"`
}

// MailPickTarget is the RPC key pair for mail.pick.
type MailPickTarget struct {
	MsID  int32 `json:"ms_id,omitempty"`
	AllID int32 `json:"all_id,omitempty"`
}

// VaseView represents one unlocked vase from namespace 102 (vaseTot).
type VaseView struct {
	VaseID  int32 `json:"vase_id"`
	UTimeMs int64 `json:"u_time_ms,omitempty"`
	CTimeMs int64 `json:"c_time_ms,omitempty"`
}

// FlowerArtView is the tracked subset of namespace 106 (flowerArtTot).
type FlowerArtView struct {
	Exp          int32           `json:"exp,omitempty"`
	MakeList     []int32         `json:"make_list,omitempty"`
	MakeListRaw  json.RawMessage `json:"make_list_raw,omitempty"`
	SRecvList    []int32         `json:"s_recv_list,omitempty"`
	SRecvListRaw json.RawMessage `json:"s_recv_list_raw,omitempty"`
	UTimeMs      int64           `json:"u_time_ms,omitempty"`
	CTimeMs      int64           `json:"c_time_ms,omitempty"`
	Observed     bool            `json:"observed,omitempty"`
}

// CollectRewardView is the tracked subset of namespace 103 (collectRwdTot).
type CollectRewardView struct {
	Type               int32   `json:"type"`
	Lvl                int32   `json:"lvl,omitempty"`
	Exp                int32   `json:"exp,omitempty"`
	RecvIDs            []int32 `json:"recv_ids,omitempty"`
	ArtCreateRewardIDs []int32 `json:"art_create_reward_ids,omitempty"`
	UTimeMs            int64   `json:"u_time_ms,omitempty"`
	CTimeMs            int64   `json:"c_time_ms,omitempty"`
}

// FmlBuildView is the tracked subset of namespace 25 (fmlTot) needed for
// guild build automation.
type FmlBuildView struct {
	Observed               bool            `json:"observed,omitempty"`
	FmlID                  int32           `json:"fml_id,omitempty"`
	MembershipObserved     bool            `json:"membership_observed,omitempty"`
	MemberFmlID            int32           `json:"member_fml_id,omitempty"`
	MemberPositionObserved bool            `json:"member_position_observed,omitempty"`
	MemberPosition         int32           `json:"member_position,omitempty"`
	MemberPositionSyncAtMs int64           `json:"member_position_sync_at_ms,omitempty"`
	TodayBuildNum          int32           `json:"today_build_num,omitempty"`
	LastBuildTimeMs        int64           `json:"last_build_time_ms,omitempty"`
	FlowerTakeCnt          int32           `json:"flower_take_cnt,omitempty"` // 25.0.102 公会摸花次数上限
	RaceLvl                int32           `json:"race_lvl,omitempty"`        // 25.0.103 公会竞赛段位
	BuildCountsObserved    bool            `json:"build_counts_observed,omitempty"`
	BuildCounts            map[int32]int32 `json:"build_counts,omitempty"`
}

type FmlBuildOptionUsage struct {
	Observed   bool  `json:"observed,omitempty"`
	Count      int32 `json:"count,omitempty"`
	GroupCount int32 `json:"group_count,omitempty"`
}

// FmlLandView is one guild land slot from namespace 25.102.fmlLand.landMap.
type FmlLandView struct {
	LandID          int32 `json:"land_id"`
	Level           int32 `json:"level,omitempty"`
	FlowerID        int32 `json:"flower_id,omitempty"`
	StartTimeMs     int64 `json:"start_time_ms,omitempty"`
	MatureFlowerCnt int32 `json:"mature_flower_count,omitempty"`
	HarvestedCnt    int32 `json:"harvested_count,omitempty"`
	LastCalcTimeMs  int64 `json:"last_calc_time_ms,omitempty"`
}

// FmlForestEnergyView is the tracked subset of namespace 25.127
// (fmlForestEnergy) needed for no-cost energy collection.
type FmlForestEnergyView struct {
	Observed                bool            `json:"observed,omitempty"`
	UID                     int64           `json:"uid,omitempty"`
	FmlID                   int32           `json:"fml_id,omitempty"`
	EnergyByType            map[int32]int32 `json:"energy_by_type,omitempty"`
	DailyEnergyByType       map[int32]int32 `json:"daily_energy_by_type,omitempty"`
	PendingTempEnergyByType map[int32]int32 `json:"pending_temp_energy_by_type,omitempty"`
	PendingTempEnergyTotal  int32           `json:"pending_temp_energy_total,omitempty"`
	UpdatedAtMs             int64           `json:"updated_at_ms,omitempty"`
	LastDailyRefreshTimeMs  int64           `json:"last_daily_refresh_time_ms,omitempty"`
}

// FmlFlowerShareSlotView is one guild flower-share slot.
type FmlFlowerShareSlotView struct {
	SlotID           int32 `json:"slot_id"`
	FlowerID         int32 `json:"flower_id,omitempty"`
	ShareNum         int32 `json:"share_num,omitempty"`
	TakeNum          int32 `json:"take_num,omitempty"`
	ShareStartTimeMs int64 `json:"share_start_time_ms,omitempty"`
}

// FmlFlowerShareView is namespace 25.107/25.108 guild flower sharing state.
type FmlFlowerShareView struct {
	Observed       bool                             `json:"observed,omitempty"`
	UID            int64                            `json:"uid,omitempty"`
	TdyTakeCnt     int32                            `json:"today_take_count,omitempty"`
	LastTakeTimeMs int64                            `json:"last_take_time_ms,omitempty"`
	UpdatedAtMs    int64                            `json:"updated_at_ms,omitempty"`
	CreatedAtMs    int64                            `json:"created_at_ms,omitempty"`
	Slots          map[int32]FmlFlowerShareSlotView `json:"slots,omitempty"`
}

// FmlFlowerTakeCandidate is one no-cost guild flower-share take candidate.
type FmlFlowerTakeCandidate struct {
	UID       int64 `json:"uid,omitempty"`
	SlotID    int32 `json:"slot_id"`
	FlowerID  int32 `json:"flower_id,omitempty"`
	ShareNum  int32 `json:"share_num,omitempty"`
	TakeNum   int32 `json:"take_num,omitempty"`
	Available int32 `json:"available,omitempty"`
}

// FmlRaceTaskView is a filtered view of an available race task from the pool.
type FmlRaceTaskView struct {
	MsId           int64  // task instance ID (used as taskMsId; millisecond-scale)
	TaskId         int32  // c_fmlRaceTask catalog row id (protocol field 4)
	TaskType       int32  // c_fmlRaceTask.type (priority / label key, e.g. 3036)
	Score          int32  // task score
	IsUpgrade      int32  // 1 if already upgraded
	UpgradeUid     int64  // UID of the member who upgraded (0 if none)
	UID            int64  // taker uid; non-zero means the task is already taken
	ParamID        int32  // first param id when present (flower/item); 0 if none
	TargetLabel    string // catalog name for ParamID; empty when unavailable
	AppearTime     int64  // protocol appearTime (ms); future = still on CD
	TargetCnt      int32  // protocol targetCnt (field 7); 0 if absent
	FinishCnt      int32  // protocol finishCnt (field 8); 0 if absent
	TakeLimitMin   int32  // protocol takeLimitMin (minutes allowed after take)
	TakeExpireTime int64  // protocol takeExpireTime (ms); deadline once taken
}

// FmlRaceTakenView is the user's currently taken task progress.
type FmlRaceTakenView struct {
	TaskMsId    int64
	TaskId      int32
	TaskType    int32
	Score       int32 // resolved from pool by MsId (0 if pool unavailable)
	TargetCnt   int32
	FinishCnt   int32
	ParamID     int32
	TargetLabel string
	// TakeLimitMin is the allowed duration in minutes after take (pool field 9).
	TakeLimitMin int32
	// TakenAtMs is local wall time when this TaskMsId hold was first observed.
	// Used with TakeLimitMin to compute ExpireTime when the server omits it.
	TakenAtMs int64
	// ExpireTime is the held-task deadline in ms. Prefer protocol
	// IFmlRaceTakeTask.expireTime / pool takeExpireTime; otherwise
	// TakenAtMs + TakeLimitMin minutes.
	ExpireTime int64
	HasTask    bool // true if the user currently holds a task
}

// FmlRaceView is the race-related slice of namespace 25.
type FmlRaceView struct {
	Observed      bool // true after a meaningful CurFmlRaceBatch (field 111) was synced
	TasksObserved bool // true after FmlRaceTaskList (field 114) has been received
	// TaskPoolStale is set when a local mutation, reconnect, or server rejection
	// requires a getTaskList round-trip before the cached pool may drive work.
	// It is deliberately separate from TasksObserved: a stale pool was still
	// observed and can be retained for display while mutations remain gated.
	TaskPoolStale bool
	// TaskPoolSyncAttemptAtMs is local wall time (ms) of the last successful
	// getTaskList round-trip. It advances even when the delta omits field 114,
	// allowing an unobserved channel response to retry with bounded backoff.
	TaskPoolSyncAttemptAtMs int64
	// TasksSyncedAtMs is local wall time (ms) when the observed task-pool snapshot
	// was last confirmed by getTaskList. A successful no-change delta advances it
	// when TasksObserved is already true.
	TasksSyncedAtMs int64
	BatchActive     bool              // true if status/time window indicates an active race
	BatchID         int64             // CurFmlRaceBatch.batchId (field 0; millisecond timestamp)
	BatchStatus     int32             // raw Status value from server (field 1 of CurFmlRaceBatch)
	BatchStartMs    int64             // race batch start time in ms (field 2)
	BatchEndMs      int64             // race batch end time in ms (field 3)
	Tasks           []FmlRaceTaskView // available task pool (field 114)
	Taken           FmlRaceTakenView  // current user's taken task (from field 110)
	// TaskQuotaObserved is true after field 110 (usr rcd) was applied.
	TaskQuotaObserved bool
	// FinishedTaskNum is IFmlRaceUsrRcd.fTaskNum (completed tasks this batch).
	FinishedTaskNum int32
	// BuyTaskNum is IFmlRaceUsrRcd.buyTaskNum (purchased extra slots).
	BuyTaskNum int32
	// RaceLvl is the guild race tier for this batch (甲=4/乙=3/…), from
	// CurFmlRaceRcd (117) / GroupFmlRaceRcdList (112) when present, else IFml.raceLvl (0.103).
	RaceLvl int32
	// RaceLvlObserved is true after RaceLvl > 0 was resolved from 117/112/0.103.
	RaceLvlObserved bool
	// RaceLvlSyncAtMs is local ms of the last enter attempt that sought raceLvl.
	RaceLvlSyncAtMs int64
	// Score is IFmlRaceUsrRcd.score — personal cumulative race points this batch.
	Score int32
	// ScoreTimeMs is IFmlRaceUsrRcd.scoreTime (ms) when Score was last set server-side.
	ScoreTimeMs int64
	// ScoreObserved is true after field 110/116 carried score (key "4") for self.
	ScoreObserved bool
	// Rank is 1-based personal standing within the guild member list from field 116
	// (sorted by score desc, then scoreTime asc). 0 when unknown.
	Rank int32
	// RankObserved is true after a usable FmlRaceUsrRankList (116) ranked self.
	RankObserved bool
	// RaceQuotaSyncAtMs is local ms of the last getFmlRaceUsrRankList attempt
	// that sought fTaskNum / personal score / rank.
	RaceQuotaSyncAtMs int64
	// MissingParamRefreshFP identifies the incomplete task rows for which a
	// successful getTaskList refresh has already been attempted. Empty means the
	// current incomplete rows have not yet received their one immediate refresh.
	MissingParamRefreshFP string
	// TakeQuotaExhausted is set when takeTask returns「任务接取次数已达上限」.
	// Cleared when the race batch identity changes. Blocks further take attempts
	// for the remainder of this batch without marking the account abnormal.
	TakeQuotaExhausted bool
	// LocalFinishCnt is a high-water harvest progress for the current taken
	// plant-harvest task. It advances from field 134 and from land HarvestCnt
	// deltas so the planner does not top-up-plant when FinishCnt lags or when
	// cleared lands drop out of the live planted set. Never used for finishTask
	// (server FinishCnt remains authoritative for claiming).
	LocalFinishCnt int32
	// LocalFinishTaskMsId is the TaskMsId LocalFinishCnt applies to; reset on
	// task change / clear.
	LocalFinishTaskMsId int64
}

// ShopCultivateOfferView is one buyable material-shop offer from namespace 113.
type ShopCultivateOfferView struct {
	ShopID     int32 `json:"shop_id"`
	ItemID     int32 `json:"item_id,omitempty"`
	ItemCount  int32 `json:"item_count,omitempty"`
	CostItemID int32 `json:"cost_item_id,omitempty"`
	CostCount  int32 `json:"cost_count,omitempty"`
	Bought     int32 `json:"bought,omitempty"`
	BuyLimit   int32 `json:"buy_limit,omitempty"`
	Remaining  int32 `json:"remaining,omitempty"`
	Sort       int32 `json:"sort,omitempty"`
}

// ZooFoodShopView is the authoritative normal-food offer from generic shop 9.
// The offer metadata comes from c_shop_item_9; purchase progress comes from
// namespace 20 and is only usable after the current game day was synced.
type ZooFoodShopView struct {
	Observed       bool  `json:"observed"`
	NeedsEnter     bool  `json:"needs_enter"`
	ShopTempID     int32 `json:"shop_temp_id"`
	ShopItemID     int32 `json:"shop_item_id"`
	FoodstuffID    int32 `json:"foodstuff_id"`
	FoodstuffCount int32 `json:"foodstuff_count"`
	GoldCost       int32 `json:"gold_cost"`
	DailyBought    int32 `json:"daily_bought"`
	DailyLimit     int32 `json:"daily_limit"`
	DailyRemaining int32 `json:"daily_remaining"`
}

// ZooFoodBowlNeed is one deterministic, observed bowl-capacity deficit.
type ZooFoodBowlNeed struct {
	PetID int32 `json:"pet_id"`
	Count int32 `json:"count"`
}

// ShopGiftbagOfferView is one configured gift-bag shop item enriched with
// namespace 112 purchase records.
type ShopGiftbagOfferView struct {
	ShopID        int32       `json:"shop_id"`
	Type          int32       `json:"type,omitempty"`
	ShareID       int32       `json:"share_id,omitempty"`
	RchgID        int32       `json:"rchg_id,omitempty"`
	MoneyID       int32       `json:"money_id,omitempty"`
	Price         int32       `json:"price,omitempty"`
	PriceMax      int32       `json:"price_max,omitempty"`
	DailyLimit    int32       `json:"daily_limit,omitempty"`
	WeeklyLimit   int32       `json:"weekly_limit,omitempty"`
	MonthLimit    int32       `json:"month_limit,omitempty"`
	TotalLimit    int32       `json:"total_limit,omitempty"`
	DailyBought   int32       `json:"daily_bought,omitempty"`
	WeekBought    int32       `json:"week_bought,omitempty"`
	MonthBought   int32       `json:"month_bought,omitempty"`
	TotalBought   int32       `json:"total_bought,omitempty"`
	Remaining     int32       `json:"remaining,omitempty"`
	CooldownSec   int32       `json:"cooldown_sec,omitempty"`
	AvailableAtMs int64       `json:"available_at_ms,omitempty"`
	NextReward    ItemCount   `json:"next_reward,omitempty"`
	Sort          int32       `json:"sort,omitempty"`
	Rewards       []ItemCount `json:"rewards,omitempty"`
}

// ShareUsageView is one c_share counter from namespace 31 (IShareTot.map).
// Counts are normalized by ShareUsageAt when a daily record is from an older
// calendar day; the raw observation remains preserved for later sparse deltas.
type ShareUsageView struct {
	Observed      bool  `json:"observed,omitempty"`
	ShareID       int32 `json:"share_id"`
	UID           int64 `json:"uid,omitempty"`
	ShareCount    int32 `json:"share_count,omitempty"`
	ReceiveCount  int32 `json:"receive_count,omitempty"`
	ReceiveTimeMs int64 `json:"receive_time_ms,omitempty"`
	TotalCount    int32 `json:"total_count,omitempty"`
	UpdatedAtMs   int64 `json:"updated_at_ms,omitempty"`
	CreatedAtMs   int64 `json:"created_at_ms,omitempty"`
}

// PearlView is the tracked subset of namespace 115.1 (pearl).
type PearlView struct {
	ProtectState  int32 `json:"protect_state,omitempty"`
	ProtectNum    int32 `json:"protect_num,omitempty"`
	OwnerUID      int64 `json:"owner_uid,omitempty"`
	LaborEndTime  int64 `json:"labor_end_time_ms,omitempty"`
	RecvDailyDate int64 `json:"recv_daily_date_ms,omitempty"`
	HireState     int32 `json:"hire_state,omitempty"`
	SmallDrawCnt  int32 `json:"small_draw_cnt,omitempty"`
	UTimeMs       int64 `json:"u_time_ms,omitempty"`
	CTimeMs       int64 `json:"c_time_ms,omitempty"`
	Observed      bool  `json:"observed,omitempty"`
}

// PearlPlaceView is one pearl labor/production slot from namespace 115.0.
type PearlPlaceView struct {
	PlaceID                int32 `json:"place_id"`
	LaborUID               int64 `json:"labor_uid,omitempty"`
	LaborEndTime           int64 `json:"labor_end_time_ms,omitempty"`
	HireFailCnt            int32 `json:"hire_fail_cnt,omitempty"`
	EventID                int32 `json:"event_id,omitempty"`
	EveryMakeNum           int32 `json:"every_make_num,omitempty"`
	RecvCnt                int32 `json:"recv_cnt,omitempty"`
	SurplusRecvNum         int32 `json:"surplus_recv_num,omitempty"`
	UTimeMs                int64 `json:"u_time_ms,omitempty"`
	CTimeMs                int64 `json:"c_time_ms,omitempty"`
	LaborUIDObserved       bool  `json:"labor_uid_observed,omitempty"`
	LaborEndTimeObserved   bool  `json:"labor_end_time_observed,omitempty"`
	HireFailCntObserved    bool  `json:"hire_fail_cnt_observed,omitempty"`
	EveryMakeNumObserved   bool  `json:"every_make_num_observed,omitempty"`
	RecvCntObserved        bool  `json:"recv_cnt_observed,omitempty"`
	SurplusRecvNumObserved bool  `json:"surplus_recv_num_observed,omitempty"`
}

// PearlCandidateProfile is the namespace 28.5 summary needed to apply the
// configured hire-level ceiling. ObservedAtMs is local receipt time and is
// deliberately session-scoped: opponent details are only trusted for a short
// candidate-selection window.
type PearlCandidateProfile struct {
	UID           int64  `json:"uid"`
	Name          string `json:"name,omitempty"`
	Level         int32  `json:"level,omitempty"`
	LevelObserved bool   `json:"level_observed,omitempty"`
	ObservedAtMs  int64  `json:"observed_at_ms"`
}

// PearlCandidateHireState is one namespace 115.5 entry. LaborEndTimeMs is the
// candidate's last/current shift end; the client applies c_pearl.$restTime
// after this timestamp before allowing another hire.
type PearlCandidateHireState struct {
	UID            int64 `json:"uid"`
	LaborEndTimeMs int64 `json:"labor_end_time_ms,omitempty"`
	ObservedAtMs   int64 `json:"observed_at_ms"`
}

// PearlEnemyView is one namespace 115.1.5 enemy entry. EventTimeMs is used
// with c_pearl.$enemyTimeMax to discard old enemies.
type PearlEnemyView struct {
	UID         int64 `json:"uid"`
	EventTimeMs int64 `json:"event_time_ms"`
}

// PearlHireView is a defensive snapshot of the session-scoped candidate
// state used by the pure automation planner.
type PearlHireView struct {
	RoleID                int64
	TicketCount           int32
	TicketUsedToday       int32
	NobleEligible         bool
	Places                map[int32]PearlPlaceView
	FriendUIDs            []int64
	FriendsObserved       bool
	Profiles              map[int64]PearlCandidateProfile
	HireStates            map[int64]PearlCandidateHireState
	RecommendUIDs         []int64
	RecommendObserved     bool
	RecommendObservedAtMs int64
	Enemies               []PearlEnemyView
	EnemiesObserved       bool
	FailedUntilMs         map[int64]int64
	SessionLocked         bool
	SessionLockReason     string
}

// PearlClaimSnapshot fixes the readiness instant and slots used by a
// pearlPlace.recvOneKey preflight. Rechecking at this same instant avoids a
// false postcondition failure when a new cycle matures during the RPC.
type PearlClaimSnapshot struct {
	At       time.Time
	PlaceIDs []int32
}

// PearlHireAttemptSnapshot fixes the exact resource and slot revision used by
// the pearlPlace.hire executor's postcondition.
type PearlHireAttemptSnapshot struct {
	At                   time.Time
	PlaceID              int32
	TargetUID            int64
	TicketCount          int32
	PreviousPlaceUTimeMs int64
}

// FlowerRequire is a single flower requirement in an order.
type FlowerRequire struct {
	FlowerID int32 `json:"flower_id"`
	Count    int32 `json:"count"`
}

// ItemRequire is a generic inventory item requirement in an order.
type ItemRequire struct {
	ItemID int32 `json:"item_id"`
	Count  int32 `json:"count"`
}

// PlantableFlower describes a cultivated flower currently available for
// planting.
type PlantableFlower struct {
	FlowerID   int32 `json:"flower_id"`
	Stock      int32 `json:"stock"`
	Lvl        int32 `json:"lvl,omitempty"`
	Gold       int32 `json:"gold,omitempty"`
	Experience int32 `json:"experience,omitempty"`
}

// DailyTaskView is the tracked subset of G.ITaskItem from namespace 22.
type DailyTaskView struct {
	TaskID       int32 `json:"task_id"`
	ProgressType int32 `json:"progress_type"`
	Target       int32 `json:"target"`
	Finished     int32 `json:"finished"`
	Status       int32 `json:"status"`
	Receipted    int32 `json:"receipted"`
}

// WeeklyTaskView is the tracked subset of c_task_week evaluated against
// namespace 22.100 progress and receipt maps.
type WeeklyTaskView struct {
	TaskID    int32 `json:"task_id"`
	Target    int32 `json:"target"`
	Finished  int32 `json:"finished"`
	Status    int32 `json:"status"`
	Receipted int32 `json:"receipted"`
}

// AchievementTaskView is the tracked subset of G.ITaskAch evaluated against
// c_task_ach and namespace 22.2 progress/receipt maps.
type AchievementTaskView struct {
	TaskID        int32 `json:"task_id"`
	GroupID       int32 `json:"group_id"`
	StageIndex    int32 `json:"stage_index"`
	ProgressType  int32 `json:"progress_type"`
	Target        int32 `json:"target"`
	Finished      int32 `json:"finished"`
	Status        int32 `json:"status"`
	Receipted     int32 `json:"receipted"`
	GroupReceived int32 `json:"group_received"`
	Current       bool  `json:"current"`
}

// MainTaskView is the tracked subset of G.ITaskMain from namespace 22.0.
type MainTaskView struct {
	Observed         bool  `json:"observed,omitempty"`
	Valid            bool  `json:"valid,omitempty"`
	Complete         bool  `json:"complete,omitempty"`
	TaskIDObserved   bool  `json:"task_id_observed,omitempty"`
	ProgressObserved bool  `json:"progress_observed,omitempty"`
	ReceiptObserved  bool  `json:"receipt_observed,omitempty"`
	TaskID           int32 `json:"task_id"`
	Finished         int32 `json:"finished"`
	Target           int32 `json:"target,omitempty"`
	NextTaskID       int32 `json:"next_task_id,omitempty"`
	Receipted        bool  `json:"receipted,omitempty"`
}

// MainTaskClaimSnapshot freezes the exact catalog task and observed server
// progress used by one empty-argument taskMain.recv request.
type MainTaskClaimSnapshot struct {
	TaskID     int32
	Target     int32
	NextTaskID int32
	Finished   int32
}

// CyclicNoteView is a defensive snapshot of the dynamically selected
// 花笺集芳 (tmpType 4002) activity in namespace 23.
type CyclicNoteView struct {
	Observed                  bool
	Found                     bool
	Valid                     bool
	TaskListObserved          bool
	TaskRecordObserved        bool
	MilestoneReceiptsObserved bool
	BatchID                   int32
	TmpID                     int32
	TmpType                   int32
	Status                    int32
	Phase                     int32
	VisibleStartMs            int64
	BeginMs                   int64
	EndMs                     int64
	GraceEndMs                int64
	PhaseEndMs                int64
	Name                      string
	Description               string
	Score                     int32
	Bag                       map[int32]int32
	CurrencyItemID            int32
	CurrencyBalance           int32
	FinishCount               int32
	FinishCountObserved       bool
	LastRefreshTimeMs         int64
	ClaimedMilestoneIndexes   []int32
	Tasks                     []CyclicNoteTaskSlotView
	Milestones                []CyclicNoteMilestoneView
}

// CyclicNoteTaskSlotView preserves the server task-list position. Locked or
// malformed entries are retained instead of being compacted away.
type CyclicNoteTaskSlotView struct {
	SlotID           int32
	Unlocked         bool
	TaskID           int32
	TaskType         int32
	Param            int32
	Title            string
	Target           int32
	Progress         int32
	ProgressObserved bool
	ReceiptObserved  bool
	Received         bool
	CatalogKnown     bool
	Reward           []ItemCount
	FinishCost       []ItemCount
}

// CyclicNoteMilestoneView is one tmp boxes threshold. Received is keyed by
// milestone index, not by target score.
type CyclicNoteMilestoneView struct {
	Index    int32
	Target   int32
	Received bool
	Reward   []ItemCount
}

// CyclicStoryView is a defensive snapshot of the dynamically selected
// 莳花纪闻 (tmpType 4003) activity in namespace 23.
type CyclicStoryView struct {
	Observed bool
	Found    bool
	// Valid requires score/bag/orders sync. Brand-new batches often omit those
	// until actCyclicStory.enter; use EnterReady for that bootstrap path.
	Valid bool
	// EnterReady means identity/template/phase are sound enough to call enter
	// even when score, bag, claimed boxes, or orders are still missing.
	EnterReady                bool
	OrdersObserved            bool
	OrdersValid               bool
	MilestoneReceiptsObserved bool
	BatchID                   int32
	TmpID                     int32
	TmpType                   int32
	Status                    int32
	Phase                     int32
	VisibleStartMs            int64
	BeginMs                   int64
	EndMs                     int64
	GraceEndMs                int64
	PhaseEndMs                int64
	Name                      string
	Description               string
	Score                     int32
	Bag                       map[int32]int32
	CurrencyItemID            int32
	CurrencyBalance           int32
	FinishCount               int32
	FinishCountObserved       bool
	ExpOrderNum               int32
	ExpOrderNumObserved       bool
	LastRefreshTimeMs         int64
	ClaimedMilestoneIndexes   []int32
	Orders                    []CyclicStoryOrderView
	Milestones                []CyclicNoteMilestoneView
}

// CyclicStoryOrderView is one orderInfo slot keyed by server orderIdx.
type CyclicStoryOrderView struct {
	OrderIdx     int32
	OrderID      int32
	FlowerID     int32
	OrderTime    int64
	ValidTime    int64
	Cost         int32
	CatalogKnown bool
	Reward       []ItemCount
	OnCooldown   bool
}

// CyclicStoryEnterSnapshot freezes the exact active batch before an enter RPC.
type CyclicStoryEnterSnapshot struct {
	At      time.Time
	BatchID int32
	Phase   int32
}

// CyclicStoryOrderClaimSnapshot freezes one order slot before recvOrderRwd.
type CyclicStoryOrderClaimSnapshot struct {
	At                  time.Time
	BatchID             int32
	OrderIdx            int32
	OrderID             int32
	FlowerID            int32
	Cost                int32
	FinishCount         int32
	FinishCountObserved bool
	Score               int32
}

// CyclicStoryMilestoneClaimSnapshot freezes one score milestone before recv.
type CyclicStoryMilestoneClaimSnapshot struct {
	At             time.Time
	BatchID        int32
	MilestoneIndex int32
	Target         int32
	Score          int32
}

// CyclicNoteEnterSnapshot freezes the exact active batch before an enter RPC.
// Enter is only safe while the batch is in its active or reward-grace phase
// and before its authoritative task list has been observed.
type CyclicNoteEnterSnapshot struct {
	At      time.Time
	BatchID int32
	Phase   int32
}

// CyclicNoteTaskClaimSnapshot freezes the unique task slot and the server
// counters used to validate one reward claim.
type CyclicNoteTaskClaimSnapshot struct {
	At                  time.Time
	BatchID             int32
	SlotID              int32
	TaskID              int32
	Target              int32
	Progress            int32
	FinishCount         int32
	FinishCountObserved bool
}

// CyclicNoteMilestoneClaimSnapshot freezes one score milestone before its
// reward claim. MilestoneIndex is the server-configured idx, not its position.
type CyclicNoteMilestoneClaimSnapshot struct {
	At             time.Time
	BatchID        int32
	MilestoneIndex int32
	Target         int32
	Score          int32
}

// StoryMainView is the tracked subset of 7.101 (G.IStoryMain).
type StoryMainView struct {
	Observed        bool        `json:"observed,omitempty"`
	Valid           bool        `json:"valid,omitempty"`
	Complete        bool        `json:"complete,omitempty"`
	UID             int64       `json:"uid,omitempty"`
	Chapter         int32       `json:"chapter,omitempty"`
	SectionIdx      int32       `json:"section_idx,omitempty"`
	ChapterObserved bool        `json:"chapter_observed,omitempty"`
	SectionObserved bool        `json:"section_observed,omitempty"`
	SectionID       int32       `json:"section_id,omitempty"`
	ChapterName     string      `json:"chapter_name,omitempty"`
	SectionName     string      `json:"section_name,omitempty"`
	Cost            []ItemCount `json:"cost,omitempty"`
	UTimeMs         int64       `json:"u_time_ms,omitempty"`
	CTimeMs         int64       `json:"c_time_ms,omitempty"`
}

// StoryUnlockSnapshot freezes the exact catalog target and inventory used by
// one empty-argument storyMain.unlock request.
type StoryUnlockSnapshot struct {
	Chapter    int32
	SectionIdx int32
	SectionID  int32
	Cost       []ItemCount
	Inventory  map[int32]int32
}

// RandomEventView is one namespace 129.0.1 entry joined with c_randomEvent.
// PositionIndex and DialogID retain the client schema semantics; neither is a
// status field. Invalid entries remain visible for diagnostics but are never
// returned as executable candidates.
type RandomEventView struct {
	EventID       int32  `json:"event_id"`
	PositionIndex int32  `json:"position_index"`
	DialogID      int32  `json:"dialog_id"`
	CatalogKnown  bool   `json:"catalog_known"`
	CostFree      bool   `json:"cost_free"`
	Valid         bool   `json:"valid"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}

// RandomEventClaimSnapshot freezes the exact event verified immediately
// before randomEvent.doAffair.
type RandomEventClaimSnapshot struct {
	EventID       int32
	PositionIndex int32
	DialogID      int32
}

const (
	// SignTypeAntiFraud is c_open.sign_1, named "防诈骗签到" by the client.
	// It is separate from the ordinary sign.sign monthly-login flow.
	SignTypeAntiFraud int32 = 1
)

const (
	// The observed signType state machine is 0 (can sign), 1 (signed and can
	// receive), 2 (reward received). The client also uses status != 2 for its
	// red-dot condition.
	SignTypeStatusCanSign int32 = iota
	SignTypeStatusCanReceive
	SignTypeStatusReceived
)

// SignTypeView is one namespace 140.0.<type> channel/sign reward record.
// SignID selects a c_signReward row; it is not an activity batch id.
type SignTypeView struct {
	Observed          bool  `json:"observed,omitempty"`
	Valid             bool  `json:"valid,omitempty"`
	UID               int64 `json:"uid,omitempty"`
	UIDObserved       bool  `json:"uid_observed,omitempty"`
	Type              int32 `json:"type,omitempty"`
	TypeObserved      bool  `json:"type_observed,omitempty"`
	LastTimeMs        int64 `json:"last_time_ms,omitempty"`
	LastTimeObserved  bool  `json:"last_time_observed,omitempty"`
	SignID            int32 `json:"sign_id,omitempty"`
	SignIDObserved    bool  `json:"sign_id_observed,omitempty"`
	Status            int32 `json:"status,omitempty"`
	StatusObserved    bool  `json:"status_observed,omitempty"`
	UpdatedAtMs       int64 `json:"updated_at_ms,omitempty"`
	UpdatedAtObserved bool  `json:"updated_at_observed,omitempty"`
	CreatedAtMs       int64 `json:"created_at_ms,omitempty"`
	CreatedAtObserved bool  `json:"created_at_observed,omitempty"`
}

const (
	// BaseRewardAntiFraud is c_rwd[2], the one-time/base anti-fraud reward
	// checked by the client before it enters signType type=1 on later days.
	BaseRewardAntiFraud      int32 = 2
	BaseRewardStatusReceived int32 = 2
)

// BaseRewardView is one namespace 7.7.<type> G.IRwd record.
type BaseRewardView struct {
	Observed          bool  `json:"observed,omitempty"`
	Valid             bool  `json:"valid,omitempty"`
	UID               int64 `json:"uid,omitempty"`
	UIDObserved       bool  `json:"uid_observed,omitempty"`
	Type              int32 `json:"type,omitempty"`
	TypeObserved      bool  `json:"type_observed,omitempty"`
	Status            int32 `json:"status,omitempty"`
	StatusObserved    bool  `json:"status_observed,omitempty"`
	UpdatedAtMs       int64 `json:"updated_at_ms,omitempty"`
	UpdatedAtObserved bool  `json:"updated_at_observed,omitempty"`
	CreatedAtMs       int64 `json:"created_at_ms,omitempty"`
	CreatedAtObserved bool  `json:"created_at_observed,omitempty"`
}

const (
	// AntiFraudQAStatusClaimed is the client-observed terminal state for the
	// anti-fraud QA reward. Any other observed state keeps the red-dot entry
	// visible in game.js.
	AntiFraudQAStatusClaimed int32 = 2
)

// UsrExtraView is the tracked subset of 7.13.1 (G.IUsrExtra).
type UsrExtraView struct {
	Observed              bool  `json:"observed,omitempty"`
	AntiFraudQAStatus     int32 `json:"anti_fraud_qa_status,omitempty"`
	LastAntiFraudQATimeMs int64 `json:"last_anti_fraud_qa_time_ms,omitempty"`
}

// ReputationView is the tracked subset of 7.17.0 (G.IReputationUsr).
type ReputationView struct {
	Observed       bool  `json:"observed,omitempty"`
	UID            int64 `json:"uid,omitempty"`
	Score          int32 `json:"score,omitempty"`
	LastSyncTimeMs int64 `json:"last_sync_time_ms,omitempty"`
	LastViewTimeMs int64 `json:"last_view_time_ms,omitempty"`
	UTimeMs        int64 `json:"u_time_ms,omitempty"`
	CTimeMs        int64 `json:"c_time_ms,omitempty"`
}

// VideoDoubleView is the tracked subset of namespace 118 (G.IVideoDouble).
type VideoDoubleView struct {
	Observed    bool  `json:"observed,omitempty"`
	UID         int64 `json:"uid,omitempty"`
	VideoCount  int32 `json:"video_count,omitempty"`
	EndTimeMs   int64 `json:"end_time_ms,omitempty"`
	UpdatedAtMs int64 `json:"updated_at_ms,omitempty"`
	CreatedAtMs int64 `json:"created_at_ms,omitempty"`
}

// StatisticsView is the tracked subset of namespace 124 (statisticsTot).
type StatisticsView struct {
	Observed               bool  `json:"observed,omitempty"`
	DayID                  int32 `json:"day_id,omitempty"`
	Gold                   int32 `json:"gold,omitempty"`
	Experience             int32 `json:"experience,omitempty"`
	Diamonds               int32 `json:"diamonds,omitempty"`
	SpeedUpCard            int32 `json:"speed_up_card,omitempty"`
	FlowerShopCoin         int32 `json:"flower_shop_coin,omitempty"`
	FlowerHarvestNum       int32 `json:"flower_harvest_num,omitempty"`
	FlowerArtSellNum       int32 `json:"flower_art_sell_num,omitempty"`
	OrderFlowerFinishNum   int32 `json:"order_flower_finish_num,omitempty"`
	OrderPalaceFinishNum   int32 `json:"order_palace_finish_num,omitempty"`
	OrderCustomerFinishNum int32 `json:"order_customer_finish_num,omitempty"`
	OrderSatinFinishNum    int32 `json:"order_satin_finish_num,omitempty"`
	Satin                  int32 `json:"satin,omitempty"`
	OrderDecorateFinishNum int32 `json:"order_decorate_finish_num,omitempty"`
	Wood                   int32 `json:"wood,omitempty"`
	UTimeMs                int64 `json:"u_time_ms,omitempty"`
	CTimeMs                int64 `json:"c_time_ms,omitempty"`
}

// ZooView is the tracked subset of namespace 33.0 (G.IZoo).
type ZooView struct {
	Observed                  bool              `json:"observed,omitempty"`
	UID                       int64             `json:"uid,omitempty"`
	Comfort                   int32             `json:"comfort,omitempty"`
	PetIDs                    []int32           `json:"pet_ids,omitempty"`
	ReadLogTimeMs             int64             `json:"read_log_time_ms,omitempty"`
	UpdatedAtMs               int64             `json:"updated_at_ms,omitempty"`
	CreatedAtMs               int64             `json:"created_at_ms,omitempty"`
	AsleepBeginTimeMs         int64             `json:"asleep_begin_time_ms,omitempty"`
	LastSetSleepTimeMs        int64             `json:"last_set_sleep_time_ms,omitempty"`
	LastSetSleepTimeObserved  bool              `json:"last_set_sleep_time_observed,omitempty"`
	GuideIDs                  []int32           `json:"guide_ids,omitempty"`
	HasChangeSleep            int32             `json:"has_change_sleep,omitempty"`
	SouvenirRewardIDs         []int32           `json:"souvenir_reward_ids,omitempty"`
	SouvenirRewardIDsObserved bool              `json:"souvenir_reward_ids_observed,omitempty"`
	ZooDecorateMap            map[int32][]int32 `json:"zoo_decorate_map,omitempty"`
	ZooDecorateMapObserved    bool              `json:"zoo_decorate_map_observed,omitempty"`
}

// ZooSouvenirView is one namespace 33.4.<tempId> souvenir record.
// MapTempID is authoritative for requests; TempIDObserved records whether the
// sparse payload also carried a matching field 1 identity.
type ZooSouvenirView struct {
	MapTempID         int32 `json:"map_temp_id"`
	UID               int64 `json:"uid,omitempty"`
	UIDObserved       bool  `json:"uid_observed,omitempty"`
	TempID            int32 `json:"temp_id,omitempty"`
	TempIDObserved    bool  `json:"temp_id_observed,omitempty"`
	IsRead            int32 `json:"is_read,omitempty"`
	IsReadObserved    bool  `json:"is_read_observed,omitempty"`
	UpdatedAtMs       int64 `json:"updated_at_ms,omitempty"`
	UpdatedAtObserved bool  `json:"updated_at_observed,omitempty"`
	CreatedAtMs       int64 `json:"created_at_ms,omitempty"`
	CreatedAtObserved bool  `json:"created_at_observed,omitempty"`
}

// ZooPetView is one pet from namespace 33.1.<petId> (G.IZooPet).
type ZooPetView struct {
	PetID                  int32           `json:"pet_id"`
	UID                    int64           `json:"uid,omitempty"`
	MoodValue              int32           `json:"mood_value,omitempty"`
	MoodObserved           bool            `json:"mood_observed,omitempty"`
	SatietyValue           int32           `json:"satiety_value,omitempty"`
	SatietyObserved        bool            `json:"satiety_observed,omitempty"`
	FoodstuffIDs           []int32         `json:"foodstuff_ids,omitempty"`
	FoodstuffObserved      bool            `json:"foodstuff_observed,omitempty"`
	Status                 int32           `json:"status,omitempty"`
	StatusObserved         bool            `json:"status_observed,omitempty"`
	Name                   string          `json:"name,omitempty"`
	NameObserved           bool            `json:"name_observed,omitempty"`
	ConDozeCount           int32           `json:"con_doze_count,omitempty"`
	StrokeCd               int32           `json:"stroke_cd,omitempty"`
	StrokeCdObserved       bool            `json:"stroke_cd_observed,omitempty"`
	GoOutEventID           int32           `json:"go_out_event_id,omitempty"`
	SpecialEventIDs        []int32         `json:"special_event_ids,omitempty"`
	LastStrokeTimeMs       int64           `json:"last_stroke_time_ms,omitempty"`
	LastStrokeTimeObserved bool            `json:"last_stroke_time_observed,omitempty"`
	StrokeCdTimeMs         int64           `json:"stroke_cd_time_ms,omitempty"`
	StrokeCdTimeObserved   bool            `json:"stroke_cd_time_observed,omitempty"`
	GetHomeTimeMs          int64           `json:"get_home_time_ms,omitempty"`
	StatusCdTimeMs         int64           `json:"status_cd_time_ms,omitempty"`
	StatusCdTimeObserved   bool            `json:"status_cd_time_observed,omitempty"`
	GoOutCdTimeMs          int64           `json:"go_out_cd_time_ms,omitempty"`
	CalTimeMs              int64           `json:"cal_time_ms,omitempty"`
	CalTimeObserved        bool            `json:"cal_time_observed,omitempty"`
	HungerTimeMs           int64           `json:"hunger_time_ms,omitempty"`
	HungerTimeObserved     bool            `json:"hunger_time_observed,omitempty"`
	ReadLogTimeMs          int64           `json:"read_log_time_ms,omitempty"`
	ReadLogTimeObserved    bool            `json:"read_log_time_observed,omitempty"`
	Ext                    json.RawMessage `json:"ext,omitempty"`
	ExtObserved            bool            `json:"ext_observed,omitempty"`
	UpdatedAtMs            int64           `json:"updated_at_ms,omitempty"`
	CreatedAtMs            int64           `json:"created_at_ms,omitempty"`
	EventTriggerTimes      map[int32]int64 `json:"event_trigger_times,omitempty"`
}

// ZooDecorateSuitView is one namespace 33.6.<tempId> (G.IZooDecorateSuit).
type ZooDecorateSuitView struct {
	MapTempID         int32 `json:"map_temp_id"`
	UID               int64 `json:"uid,omitempty"`
	UIDObserved       bool  `json:"uid_observed,omitempty"`
	TempID            int32 `json:"temp_id,omitempty"`
	TempIDObserved    bool  `json:"temp_id_observed,omitempty"`
	ActCount          int32 `json:"act_count,omitempty"`
	ActCountObserved  bool  `json:"act_count_observed,omitempty"`
	UpdatedAtMs       int64 `json:"updated_at_ms,omitempty"`
	UpdatedAtObserved bool  `json:"updated_at_observed,omitempty"`
	CreatedAtMs       int64 `json:"created_at_ms,omitempty"`
	CreatedAtObserved bool  `json:"created_at_observed,omitempty"`
}

// ZooDecorateView is one namespace 33.5.<tempId> (G.IZooDecorate).
type ZooDecorateView struct {
	MapTempID         int32 `json:"map_temp_id"`
	UID               int64 `json:"uid,omitempty"`
	UIDObserved       bool  `json:"uid_observed,omitempty"`
	TempID            int32 `json:"temp_id,omitempty"`
	TempIDObserved    bool  `json:"temp_id_observed,omitempty"`
	IsRead            int32 `json:"is_read,omitempty"`
	IsReadObserved    bool  `json:"is_read_observed,omitempty"`
	Comfort           int32 `json:"comfort,omitempty"`
	ComfortObserved   bool  `json:"comfort_observed,omitempty"`
	UpdatedAtMs       int64 `json:"updated_at_ms,omitempty"`
	UpdatedAtObserved bool  `json:"updated_at_observed,omitempty"`
	CreatedAtMs       int64 `json:"created_at_ms,omitempty"`
	CreatedAtObserved bool  `json:"created_at_observed,omitempty"`
}

// ZooLogExtView is field 11 of one namespace 33.2 log entry.
type ZooLogExtView struct {
	UserName           string          `json:"user_name,omitempty"`
	UserNameObserved   bool            `json:"user_name_observed,omitempty"`
	PetName            string          `json:"pet_name,omitempty"`
	PetNameObserved    bool            `json:"pet_name_observed,omitempty"`
	PetID              int32           `json:"pet_id,omitempty"`
	PetIDObserved      bool            `json:"pet_id_observed,omitempty"`
	Consume            map[int32]int32 `json:"consume,omitempty"`
	ConsumeObserved    bool            `json:"consume_observed,omitempty"`
	Consume2           map[int32]int32 `json:"consume2,omitempty"`
	Consume2Observed   bool            `json:"consume2_observed,omitempty"`
	IsUserBack         int32           `json:"is_user_back,omitempty"`
	IsUserBackObserved bool            `json:"is_user_back_observed,omitempty"`
}

// ZooLogView is one namespace 33.2 server event log. Key is the opaque
// server-map key used only for sparse merge and deletion; PetID and Index are
// the authoritative operation identity carried by fields 1 and 2.
type ZooLogView struct {
	Key                   string          `json:"key"`
	Malformed             bool            `json:"malformed,omitempty"`
	MalformedReason       string          `json:"malformed_reason,omitempty"`
	UID                   int64           `json:"uid,omitempty"`
	UIDObserved           bool            `json:"uid_observed,omitempty"`
	PetID                 int32           `json:"pet_id,omitempty"`
	PetIDObserved         bool            `json:"pet_id_observed,omitempty"`
	Index                 int32           `json:"index,omitempty"`
	IndexObserved         bool            `json:"index_observed,omitempty"`
	MoodChangeValue       int32           `json:"mood_change_value,omitempty"`
	MoodChangeObserved    bool            `json:"mood_change_observed,omitempty"`
	SatietyChangeValue    int32           `json:"satiety_change_value,omitempty"`
	SatietyChangeObserved bool            `json:"satiety_change_observed,omitempty"`
	GoOutEventID          int32           `json:"go_out_event_id,omitempty"`
	GoOutEventIDObserved  bool            `json:"go_out_event_id_observed,omitempty"`
	EventType             int32           `json:"event_type,omitempty"`
	EventTypeObserved     bool            `json:"event_type_observed,omitempty"`
	ProType               int32           `json:"pro_type,omitempty"`
	ProTypeObserved       bool            `json:"pro_type_observed,omitempty"`
	Gain                  map[int32]int32 `json:"gain,omitempty"`
	GainObserved          bool            `json:"gain_observed,omitempty"`
	Consume               map[int32]int32 `json:"consume,omitempty"`
	ConsumeObserved       bool            `json:"consume_observed,omitempty"`
	Souvenir              map[int32]int32 `json:"souvenir,omitempty"`
	SouvenirObserved      bool            `json:"souvenir_observed,omitempty"`
	Ext                   ZooLogExtView   `json:"ext,omitempty"`
	ExtObserved           bool            `json:"ext_observed,omitempty"`
	UpdatedAtMs           int64           `json:"updated_at_ms,omitempty"`
	UpdatedAtObserved     bool            `json:"updated_at_observed,omitempty"`
	CreatedAtMs           int64           `json:"created_at_ms,omitempty"`
	CreatedAtObserved     bool            `json:"created_at_observed,omitempty"`
	InsertedAtMs          int64           `json:"inserted_at_ms,omitempty"`
	InsertedAtObserved    bool            `json:"inserted_at_observed,omitempty"`
}

// ZooFoodstuffPlan describes one inventory-backed bowl stocking operation.
// One operation deliberately contains only one food type.
type ZooFoodstuffPlan struct {
	PetID       int32 `json:"pet_id"`
	FoodstuffID int32 `json:"foodstuff_id"`
	Count       int32 `json:"count"`
}

// ZooEventAction is a conservative server-backed animal-event candidate.
type ZooEventAction struct {
	PetID         int32  `json:"pet_id"`
	EventID       int32  `json:"event_id"`
	TableID       int32  `json:"table_id,omitempty"`
	CreatedAtMs   int64  `json:"created_at_ms,omitempty"`
	Name          string `json:"name,omitempty"`
	Action        string `json:"action,omitempty"` // handle_event or read_log
	Agree         bool   `json:"agree,omitempty"`
	IsShareVideo  int32  `json:"is_share_video,omitempty"`
	Blocked       bool   `json:"blocked,omitempty"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}
