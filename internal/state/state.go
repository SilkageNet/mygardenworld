// Package state tracks per-account land + inventory state. This is the Go
// port of GardenState from scripts/tools/garden_bot.py.
//
// The tracker is fed v-namespace fragments (typically `100` and `7`) from
// either index.reLogin responses (initial bulk) or per-RPC responses (delta
// after plant/water/harvest). It maintains a coherent in-memory view that
// the automation engine queries.
package state

import (
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

// FlowerSeedLow is the inclusive lower itemId for flower seeds.
const FlowerSeedLow = 23000

// FlowerSeedHigh is the exclusive upper itemId for flower seeds.
const FlowerSeedHigh = 24000

// LandView mirrors the G.ILand schema: per-land state observed on the wire.
//
//	field 0 = flowerId
//	field 1 = state (1=just planted, 3=initial bloom ready, 2=regrowing)
//	field 2 = lvl (land level)
//	field 3 = harvestCnt (times this plant has been harvested)
//	field 5 = nextTime (ms; next state transition - regrow ready)
//	field 7 = plantTime (ms; last plant/state-change tick)
type LandView struct {
	FlowerID    int   `json:"flower_id,omitempty"`
	State       int   `json:"state,omitempty"`
	Lvl         int   `json:"lvl,omitempty"`
	HarvestCnt  int   `json:"harvest_cnt,omitempty"`
	NextTimeMs  int64 `json:"next_time_ms,omitempty"`
	PlantTimeMs int64 `json:"plant_time_ms,omitempty"`

	// Observed = the server has confirmed this land's state at least once
	// (including the empty-after-harvest state). Distinguishes "land we have
	// never seen" from "land we know is empty and ready to plant".
	Observed bool `json:"observed,omitempty"`
}

// IsPlanted returns true when a flower id is set on the land.
func (l LandView) IsPlanted() bool { return l.FlowerID != 0 }

// FromPrimary builds a LandView from the raw `100.1.<id>` JSON dict. Server
// responses use numeric-string keys ("0".."7"), per the G.ILand schema.
func FromPrimary(raw map[string]any) LandView {
	v := LandView{Observed: true}
	v.FlowerID = readInt(raw, "0")
	v.State = readInt(raw, "1")
	v.Lvl = readInt(raw, "2")
	v.HarvestCnt = readInt(raw, "3")
	v.NextTimeMs = readInt64(raw, "5")
	v.PlantTimeMs = readInt64(raw, "7")
	return v
}

// EmptyObserved is what we record after a harvest clears the land
// (server sends `100.1.<id> = {}`) - we still mark it observed so the
// automation engine knows the slot is plant-ready, not unknown.
func EmptyObserved() LandView { return LandView{Observed: true} }

// ToJSON returns the LandView as JSON for event emission.
func (l LandView) ToJSON() map[string]any {
	return map[string]any{
		"flowerId":   l.FlowerID,
		"state":      l.State,
		"lvl":        l.Lvl,
		"harvestCnt": l.HarvestCnt,
		"nextTime":   l.NextTimeMs,
		"plantTime":  l.PlantTimeMs,
		"observed":   l.Observed,
	}
}

// CultivateView mirrors the G.ICultivate schema from namespace 101.
//
//	field 1 = flowerId
//	field 2 = lvl (cultivation level, 0-5)
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
	BoxID    int32           `json:"box_id"`
	Mode     int32           `json:"mode,omitempty"`
	Requires []FlowerRequire `json:"requires"`
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
// namespace 105. The observed protocol currently exposes counters and NPC data
// but not a reliable per-flower requirement list, so execution remains gated
// until requirements are explicit.
type ResidentSpecialOrder struct {
	Observed  bool  `json:"observed,omitempty"`
	Flowers   int32 `json:"flowers,omitempty"`
	NPCID     int32 `json:"npc_id,omitempty"`
	DialogID  int32 `json:"dialog_id,omitempty"`
	FinishCnt int32 `json:"finish_cnt,omitempty"`
	IsVideo   int32 `json:"is_video,omitempty"`
	VideoRwd  int32 `json:"video_rwd,omitempty"`
	CdTimeMs  int64 `json:"cd_time_ms,omitempty"`
	CTimeMs   int64 `json:"c_time_ms,omitempty"`
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
	Observed            bool            `json:"observed,omitempty"`
	FmlID               int32           `json:"fml_id,omitempty"`
	TodayBuildNum       int32           `json:"today_build_num,omitempty"`
	LastBuildTimeMs     int64           `json:"last_build_time_ms,omitempty"`
	BuildCountsObserved bool            `json:"build_counts_observed,omitempty"`
	BuildCounts         map[int32]int32 `json:"build_counts,omitempty"`
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

// ShopGiftbagOfferView is one configured gift-bag shop item enriched with
// namespace 112 purchase records.
type ShopGiftbagOfferView struct {
	ShopID      int32       `json:"shop_id"`
	Type        int32       `json:"type,omitempty"`
	ShareID     int32       `json:"share_id,omitempty"`
	RchgID      int32       `json:"rchg_id,omitempty"`
	MoneyID     int32       `json:"money_id,omitempty"`
	Price       int32       `json:"price,omitempty"`
	PriceMax    int32       `json:"price_max,omitempty"`
	DailyLimit  int32       `json:"daily_limit,omitempty"`
	WeeklyLimit int32       `json:"weekly_limit,omitempty"`
	MonthLimit  int32       `json:"month_limit,omitempty"`
	TotalLimit  int32       `json:"total_limit,omitempty"`
	DailyBought int32       `json:"daily_bought,omitempty"`
	WeekBought  int32       `json:"week_bought,omitempty"`
	MonthBought int32       `json:"month_bought,omitempty"`
	TotalBought int32       `json:"total_bought,omitempty"`
	Remaining   int32       `json:"remaining,omitempty"`
	Sort        int32       `json:"sort,omitempty"`
	Rewards     []ItemCount `json:"rewards,omitempty"`
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
	PlaceID        int32 `json:"place_id"`
	LaborUID       int64 `json:"labor_uid,omitempty"`
	LaborEndTime   int64 `json:"labor_end_time_ms,omitempty"`
	HireFailCnt    int32 `json:"hire_fail_cnt,omitempty"`
	EventID        int32 `json:"event_id,omitempty"`
	EveryMakeNum   int32 `json:"every_make_num,omitempty"`
	RecvCnt        int32 `json:"recv_cnt,omitempty"`
	SurplusRecvNum int32 `json:"surplus_recv_num,omitempty"`
	UTimeMs        int64 `json:"u_time_ms,omitempty"`
	CTimeMs        int64 `json:"c_time_ms,omitempty"`
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
	Gold       int32 `json:"gold,omitempty"`
	Experience int32 `json:"experience,omitempty"`
}

// DailyTaskView is the tracked subset of G.ITaskItem from namespace 22.
type DailyTaskView struct {
	TaskID    int32 `json:"task_id"`
	Target    int32 `json:"target"`
	Finished  int32 `json:"finished"`
	Status    int32 `json:"status"`
	Receipted int32 `json:"receipted"`
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

// MainTaskView is the tracked subset of G.ITaskMain from namespace 22.0.
type MainTaskView struct {
	TaskID   int32 `json:"task_id"`
	Finished int32 `json:"finished"`
}

// RandomEventView is the tracked subset of namespace 129 map events. Static
// client schema names 129.IRandomEventInfo.1/2 as posIdx/dialogId; current
// status/affair semantics are capture-derived and pending revalidation.
type RandomEventView struct {
	EventID int32 `json:"event_id"`
	Status  int32 `json:"status"`
	Affair  int32 `json:"affair"`
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
	OrderFlowerFinishNum   int32 `json:"order_flower_finish_num,omitempty"`
	OrderPalaceFinishNum   int32 `json:"order_palace_finish_num,omitempty"`
	OrderCustomerFinishNum int32 `json:"order_customer_finish_num,omitempty"`
	OrderSatinFinishNum    int32 `json:"order_satin_finish_num,omitempty"`
	OrderDecorateFinishNum int32 `json:"order_decorate_finish_num,omitempty"`
	FlowerArtSellNum       int32 `json:"flower_art_sell_num,omitempty"`
	UTimeMs                int64 `json:"u_time_ms,omitempty"`
	CTimeMs                int64 `json:"c_time_ms,omitempty"`
}

// ZooView is the tracked subset of namespace 33.0 (G.IZoo).
type ZooView struct {
	Observed          bool    `json:"observed,omitempty"`
	UID               int64   `json:"uid,omitempty"`
	Comfort           int32   `json:"comfort,omitempty"`
	PetIDs            []int32 `json:"pet_ids,omitempty"`
	ReadLogTimeMs     int64   `json:"read_log_time_ms,omitempty"`
	UpdatedAtMs       int64   `json:"updated_at_ms,omitempty"`
	SouvenirRewardIDs []int32 `json:"souvenir_reward_ids,omitempty"`
}

// ZooPetView is one pet from namespace 33.1.<petId> (G.IZooPet).
type ZooPetView struct {
	PetID          int32   `json:"pet_id"`
	UID            int64   `json:"uid,omitempty"`
	MoodValue      int32   `json:"mood_value,omitempty"`
	SatietyValue   int32   `json:"satiety_value,omitempty"`
	FoodstuffIDs   []int32 `json:"foodstuff_ids,omitempty"`
	Status         int32   `json:"status,omitempty"`
	StrokeCdTimeMs int64   `json:"stroke_cd_time_ms,omitempty"`
	StatusCdTimeMs int64   `json:"status_cd_time_ms,omitempty"`
	GoOutCdTimeMs  int64   `json:"go_out_cd_time_ms,omitempty"`
	GetHomeTimeMs  int64   `json:"get_home_time_ms,omitempty"`
	UpdatedAtMs    int64   `json:"updated_at_ms,omitempty"`
}

// State is the per-account in-memory tracker.
type State struct {
	mu sync.RWMutex

	lands              map[int32]LandView
	landRosterObserved bool
	farmLands          map[int32]FarmLandInfo
	farmLandObserved   bool
	inventory          map[int32]int32 // 7.0.32 sub-map: itemId -> count
	gold               int32           // 7.0.44 金币
	level              int32           // 7.0.34 等级
	experience         int32           // 7.0.35 经验
	vip                int32           // 7.0.36 VIP level
	vipExp             int32           // 7.0.37 VIP exp
	diamondsFree       int32           // 7.0.41 免费钻石
	diamondsPaid       int32           // 7.0.42 付费钻石
	roleID             int64

	rawNamespaces   map[string]json.RawMessage
	namespaceCounts map[string]int32
	unknownNSCounts map[string]int32

	hasWaterDropsItem  bool  // whether namespace 7 has carried itemId=7 at least once
	waterDropsTotal    int32 // 7.0.33.7.1 observed water-drop capacity / total
	waterDropsNextMs   int64 // 7.0.33.7.5 下次恢复时间 ms
	waterDropsInFlight int32 // drops committed to in-flight water RPCs

	wwClaimedCount int32 // 114.1 水车已领取总次数
	wwLastRecvTs   int64 // 114.4 uTime; used as the latest observed claim/update timestamp

	cultivations map[int32]*CultivateView // 101.0.<flowerId>

	customerOrderSummary  CustomerOrderSummary      // 109.0 顾客订单元信息
	customerOrders        map[int32]*CustomerOrder  // 109.0.1.<npcId> 当前活跃顾客订单
	flowerRack            map[int32]*FlowerRackSlot // 104.0.<rackId> 花艺货架
	mailObserved          bool
	mails                 map[string]*MailView // 19.1[] keyed by msId/allId
	vases                 map[int32]*VaseView  // 102.0.<vaseId> 已解锁花瓶
	vaseObserved          bool
	collectRewards        map[int32]*CollectRewardView // 103.0.<type> 图鉴/制作奖励状态
	collectRewardObserved bool
	flowerArt             FlowerArtView // 106.0 花艺制作/分享状态
	fmlBuild              FmlBuildView  // 25.0/25.133 公会建设状态
	fmlLandObserved       bool
	fmlLands              map[int32]*FmlLandView // 25.102.1.<landId> 公会土地
	fmlForestEnergy       FmlForestEnergyView    // 25.127 能量森林
	fmlFlowerShare        FmlFlowerShareView     // 25.107 自己的公会鲜花分享
	fmlOtherFlowerShares  map[int64]*FmlFlowerShareView
	fmlOtherShareObserved bool
	shopGiftbagDRecord    map[int32]int32 // 112.1 daily purchase counts
	shopGiftbagWRecord    map[int32]int32 // 112.2 weekly purchase counts
	shopGiftbagMRecord    map[int32]int32 // 112.3 monthly purchase counts
	shopGiftbagTRecord    map[int32]int32 // 112.4 total purchase counts
	shopGiftbagObserved   bool
	shopCultivateCosts    map[int32]ItemCount // 113.1.<shopId> material-shop dynamic cost
	shopCultivateBought   map[int32]int32     // 113.6.<shopId> bought count
	shopCultivateObserved bool
	pearl                 PearlView
	pearlPlaces           map[int32]*PearlPlaceView // 115.0.<placeId>
	pearlDrawCount        int32                     // count derived from 115.2 drawList
	pearlDrawRaw          json.RawMessage
	pearlObserved         bool

	flowerOrders               map[int32]*FlowerOrder // 105.0.1.<boxId> 当前活跃居民订单
	flowerOrderRewardsReceived map[int32]bool         // 105.0.2 已领取的居民订单阶段奖励 target
	residentSatinOrder         ResidentSpecialOrder
	residentDecorateOrder      ResidentSpecialOrder
	palaceOrder                PalaceOrderView // 108.0 宫廷订单
	teamOrder                  TeamOrderView   // 107.0 组团订单

	mainTask    *MainTaskView             // 22.0 当前主线任务
	dailyTasks  map[int32]*DailyTaskView  // 22.1.100.<taskId>
	weeklyTasks map[int32]*WeeklyTaskView // 22.100 + c_task_week

	roadGrowReceived map[int32]bool             // 119.3.<taskId> 成长之路已领取
	randomEvents     map[int32]*RandomEventView // 129.0.1.<eventId> 地图随机事件

	freeWaterObserved bool  // namespace 117 has been observed at least once
	freeWaterRecvIdx  int32 // 117.1 last observed free-water receive index
	freeWaterResetMs  int64 // 117.2 reset timestamp

	benefitBoxDrawCnt    int32 // 116.0.1 drawCnt
	benefitBoxResetCntMs int64 // 116.0.2 resetCntTime
	benefitBoxUTimeMs    int64 // 116.0.3 uTime
	benefitBoxObserved   bool  // namespace 116 has been observed at least once
	usrExtra             UsrExtraView
	videoDouble          VideoDoubleView
	statistics           StatisticsView
	zoo                  ZooView
	zooPets              map[int32]*ZooPetView
	zooObserved          bool

	// Recent server-side activity timestamp; updated on every apply.
	lastApplyMs int64

	// onChange (optional) is invoked on every accepted apply, with a
	// snapshot of changed-land ids and the new view. Useful for the runner
	// to push events to subscribers.
	onChange func(changed []LandChange)

	// onResourceChange (optional) is invoked when any resource field changes.
	onResourceChange func(ResourceSnapshot)

	// onInventoryChange (optional) is invoked when the tracked inventory map changes.
	onInventoryChange func(InventorySnapshot)
}

// LandChange is the diff produced by apply.
type LandChange struct {
	LandID int32
	Before LandView
	After  LandView
}

// ResourceSnapshot is the current state of player resources, emitted on change.
type ResourceSnapshot struct {
	Gold             int32 `json:"gold"`
	WaterDrops       int32 `json:"water_drops"`
	WaterDropsTotal  int32 `json:"water_drops_total"`
	WaterDropsNextMs int64 `json:"water_drops_next_ms"`
	Level            int32 `json:"level"`
	Experience       int32 `json:"experience"`
	Vip              int32 `json:"vip"`
	VipExp           int32 `json:"vip_exp"`
	NobleEligible    bool  `json:"noble_eligible"`
	DiamondsFree     int32 `json:"diamonds_free"`
	DiamondsPaid     int32 `json:"diamonds_paid"`
}

// InventorySnapshot is emitted when the tracked item inventory changes.
type InventorySnapshot struct {
	Inventory map[int32]int32      `json:"inventory"`
	Changes   []InventoryItemDelta `json:"changes,omitempty"`
}

// InventoryItemDelta describes one changed inventory entry.
type InventoryItemDelta struct {
	ItemID int32 `json:"item_id"`
	Before int32 `json:"before"`
	After  int32 `json:"after"`
}

// New creates an empty tracker.
func New() *State {
	return &State{
		lands:                      make(map[int32]LandView),
		farmLands:                  make(map[int32]FarmLandInfo),
		inventory:                  make(map[int32]int32),
		rawNamespaces:              make(map[string]json.RawMessage),
		namespaceCounts:            make(map[string]int32),
		unknownNSCounts:            make(map[string]int32),
		cultivations:               make(map[int32]*CultivateView),
		customerOrders:             make(map[int32]*CustomerOrder),
		flowerRack:                 make(map[int32]*FlowerRackSlot),
		mails:                      make(map[string]*MailView),
		vases:                      make(map[int32]*VaseView),
		collectRewards:             make(map[int32]*CollectRewardView),
		fmlBuild:                   FmlBuildView{BuildCounts: make(map[int32]int32)},
		fmlLands:                   make(map[int32]*FmlLandView),
		fmlForestEnergy:            FmlForestEnergyView{EnergyByType: make(map[int32]int32), DailyEnergyByType: make(map[int32]int32), PendingTempEnergyByType: make(map[int32]int32)},
		fmlFlowerShare:             FmlFlowerShareView{Slots: make(map[int32]FmlFlowerShareSlotView)},
		fmlOtherFlowerShares:       make(map[int64]*FmlFlowerShareView),
		shopGiftbagDRecord:         make(map[int32]int32),
		shopGiftbagWRecord:         make(map[int32]int32),
		shopGiftbagMRecord:         make(map[int32]int32),
		shopGiftbagTRecord:         make(map[int32]int32),
		shopCultivateCosts:         make(map[int32]ItemCount),
		shopCultivateBought:        make(map[int32]int32),
		pearlPlaces:                make(map[int32]*PearlPlaceView),
		flowerOrders:               make(map[int32]*FlowerOrder),
		flowerOrderRewardsReceived: make(map[int32]bool),
		dailyTasks:                 make(map[int32]*DailyTaskView),
		weeklyTasks:                make(map[int32]*WeeklyTaskView),
		roadGrowReceived:           make(map[int32]bool),
		randomEvents:               make(map[int32]*RandomEventView),
		zooPets:                    make(map[int32]*ZooPetView),
	}
}

// SetOnChange installs a callback fired whenever lands change. Called with
// the lock released.
func (s *State) SetOnChange(fn func(changed []LandChange)) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

// SetOnResourceChange installs a callback fired whenever resource fields change.
// Called with the lock released.
func (s *State) SetOnResourceChange(fn func(ResourceSnapshot)) {
	s.mu.Lock()
	s.onResourceChange = fn
	s.mu.Unlock()
}

// SetOnInventoryChange installs a callback fired whenever tracked item counts change.
// Called with the lock released.
func (s *State) SetOnInventoryChange(fn func(InventorySnapshot)) {
	s.mu.Lock()
	s.onInventoryChange = fn
	s.mu.Unlock()
}

// ApplyV merges a v-fragment from the server (full or partial) into state.
// Recognised top-level keys include inventory/resources, lands, cultivation,
// orders, tasks, waterwheel, and free-water reward state. Other keys are
// silently ignored - they're outside this tracker's scope.
//
// When the input is not a JSON object (e.g. some legacy responses serialize
// v as a JSON-stringified blob), ApplyV is a no-op.
func (s *State) ApplyV(rawV json.RawMessage) {
	if len(rawV) == 0 {
		return
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(rawV, &top); err != nil {
		return
	}
	s.applyTop(top)
}

// ApplyVMap is the post-decoded counterpart of ApplyV: pass an already-parsed
// `map[string]any`. The runner uses this when subscribing via
// Client.OnNamespace, where the fragment arrives pre-extracted.
func (s *State) ApplyVMap(top map[string]any) {
	conv := make(map[string]json.RawMessage, len(top))
	for k, v := range top {
		raw, _ := json.Marshal(v)
		conv[k] = raw
	}
	s.applyTop(conv)
}

func (s *State) applyTop(top map[string]json.RawMessage) {
	s.mu.Lock()
	now := time.Now().UnixMilli()
	s.lastApplyMs = now

	var changes []LandChange
	if s.rawNamespaces == nil {
		s.rawNamespaces = make(map[string]json.RawMessage)
	}
	if s.namespaceCounts == nil {
		s.namespaceCounts = make(map[string]int32)
	}
	if s.unknownNSCounts == nil {
		s.unknownNSCounts = make(map[string]int32)
	}
	for ns, raw := range top {
		s.rawNamespaces[ns] = append(json.RawMessage(nil), raw...)
		s.namespaceCounts[ns]++
		if !babigame.IsModeledNamespace(ns) {
			s.unknownNSCounts[ns]++
		}
	}

	// Capture resource state before NS7 apply for change detection.
	prevGold := s.gold
	prevWaterDrops, prevWaterDropsTotal, prevWaterDropsNext := s.currentWaterDropsLocked(), s.waterDropsTotal, s.waterDropsNextMs
	prevLevel, prevExp, prevVip, prevVipExp := s.level, s.experience, s.vip, s.vipExp
	prevDFree, prevDPaid := s.diamondsFree, s.diamondsPaid
	var prevInventory map[int32]int32
	if _, ok := top["7"]; ok {
		prevInventory = cloneInt32Map(s.inventory)
	}

	if rawNS100, ok := top["100"]; ok {
		var ns map[string]json.RawMessage
		if err := json.Unmarshal(rawNS100, &ns); err == nil {
			changes = append(changes, s.applyLandsLocked(ns)...)
		}
	}
	if rawNS7, ok := top["7"]; ok {
		var ns map[string]json.RawMessage
		if err := json.Unmarshal(rawNS7, &ns); err == nil {
			s.applyInventoryLocked(ns)
			s.applyUsrExtraLocked(ns)
		}
	}
	if rawNS19, ok := top["19"]; ok {
		s.applyMailLocked(rawNS19)
	}
	if rawNS114, ok := top["114"]; ok {
		s.applyWaterwheelLocked(rawNS114)
	}
	if rawNS115, ok := top["115"]; ok {
		s.applyPearlLocked(rawNS115)
	}
	if rawNS101, ok := top["101"]; ok {
		s.applyCultivationsLocked(rawNS101)
	}
	if rawNS102, ok := top["102"]; ok {
		s.applyVasesLocked(rawNS102)
	}
	if rawNS103, ok := top["103"]; ok {
		s.applyCollectRewardsLocked(rawNS103)
	}
	if rawNS109, ok := top["109"]; ok {
		s.applyCustomerOrdersLocked(rawNS109)
	}
	if rawNS104, ok := top["104"]; ok {
		s.applyFlowerRackLocked(rawNS104)
	}
	if rawNS105, ok := top["105"]; ok {
		s.applyFlowerOrdersLocked(rawNS105)
	}
	if rawNS106, ok := top["106"]; ok {
		s.applyFlowerArtLocked(rawNS106)
	}
	if rawNS107, ok := top["107"]; ok {
		s.applyTeamOrderLocked(rawNS107)
	}
	if rawNS108, ok := top["108"]; ok {
		s.applyPalaceOrderLocked(rawNS108)
	}
	if rawNS25, ok := top["25"]; ok {
		s.applyFmlLocked(rawNS25)
	}
	if rawNS112, ok := top["112"]; ok {
		s.applyShopGiftbagLocked(rawNS112)
	}
	if rawNS113, ok := top["113"]; ok {
		s.applyShopCultivateLocked(rawNS113)
	}
	if rawNS22, ok := top["22"]; ok {
		s.applyTasksLocked(rawNS22)
	}
	if rawNS117, ok := top["117"]; ok {
		s.applyFreeWaterLocked(rawNS117)
	}
	if rawNS116, ok := top["116"]; ok {
		s.applyBenefitBoxLocked(rawNS116)
	}
	if rawNS118, ok := top["118"]; ok {
		s.applyVideoDoubleLocked(rawNS118)
	}
	if rawNS119, ok := top["119"]; ok {
		s.applyRoadGrowLocked(rawNS119)
	}
	if rawNS124, ok := top["124"]; ok {
		s.applyStatisticsLocked(rawNS124)
	}
	if rawNS129, ok := top["129"]; ok {
		s.applyRandomEventsLocked(rawNS129)
	}
	if rawNS33, ok := top["33"]; ok {
		s.applyZooLocked(rawNS33)
	}

	resourcesChanged := s.gold != prevGold || s.currentWaterDropsLocked() != prevWaterDrops ||
		s.waterDropsTotal != prevWaterDropsTotal || s.waterDropsNextMs != prevWaterDropsNext || s.level != prevLevel ||
		s.experience != prevExp || s.vip != prevVip || s.vipExp != prevVipExp ||
		s.diamondsFree != prevDFree || s.diamondsPaid != prevDPaid
	var resourceSnap ResourceSnapshot
	var resourceCb func(ResourceSnapshot)
	if resourcesChanged {
		resourceSnap = ResourceSnapshot{
			Gold: s.gold, WaterDrops: s.currentWaterDropsLocked(), WaterDropsTotal: s.waterDropsTotal, WaterDropsNextMs: s.waterDropsNextMs,
			Level: s.level, Experience: s.experience, Vip: s.vip, VipExp: s.vipExp, NobleEligible: s.nobleEligibleLocked(),
			DiamondsFree: s.diamondsFree, DiamondsPaid: s.diamondsPaid,
		}
		resourceCb = s.onResourceChange
	}
	var inventorySnap InventorySnapshot
	var inventoryCb func(InventorySnapshot)
	if prevInventory != nil {
		changes := inventoryChanges(prevInventory, s.inventory)
		if len(changes) > 0 {
			inventorySnap = InventorySnapshot{
				Inventory: cloneInt32Map(s.inventory),
				Changes:   changes,
			}
			inventoryCb = s.onInventoryChange
		}
	}

	cb := s.onChange
	s.mu.Unlock()

	if cb != nil && len(changes) > 0 {
		cb(changes)
	}
	if resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil {
		inventoryCb(inventorySnap)
	}
}

func (s *State) applyLandsLocked(ns100 map[string]json.RawMessage) []LandChange {
	var changes []LandChange
	if raw0, ok := ns100["0"]; ok {
		var s0 map[string]json.RawMessage
		if err := json.Unmarshal(raw0, &s0); err == nil {
			if rawRole, ok := s0["0"]; ok {
				_ = json.Unmarshal(rawRole, &s.roleID)
			}
			if raw1, ok := s0["1"]; ok {
				var roster map[string]json.RawMessage
				if err := json.Unmarshal(raw1, &roster); err == nil {
					s.landRosterObserved = true
					next := make(map[int32]LandView, len(roster))
					for lidStr, rawEntry := range roster {
						lid := atoi32(lidStr)
						if lid < 1000 {
							continue
						}
						var entry map[string]any
						if len(rawEntry) > 0 && string(rawEntry) != "{}" {
							if err := json.Unmarshal(rawEntry, &entry); err != nil {
								continue
							}
						}
						var view LandView
						if len(entry) > 0 {
							view = FromPrimary(entry)
						} else {
							view = EmptyObserved()
						}
						next[lid] = view
						before, existed := s.lands[lid]
						if !existed || before != view {
							changes = append(changes, LandChange{LandID: lid, Before: before, After: view})
						}
					}
					for lid, before := range s.lands {
						if _, ok := next[lid]; !ok {
							changes = append(changes, LandChange{LandID: lid, Before: before, After: LandView{}})
						}
					}
					s.lands = next
				}
			}
		}
	}
	if raw1, ok := ns100["1"]; ok {
		var sub1 map[string]json.RawMessage
		if err := json.Unmarshal(raw1, &sub1); err == nil {
			for lidStr, rawEntry := range sub1 {
				lid := atoi32(lidStr)
				if lid < 1000 {
					continue
				}
				var entry map[string]any
				view := EmptyObserved()
				if len(rawEntry) > 0 && string(rawEntry) != "{}" {
					if err := json.Unmarshal(rawEntry, &entry); err == nil {
						view = FromPrimary(entry)
					}
				}
				if change, ok := s.upsertLandLocked(lid, view, "primary"); ok {
					changes = append(changes, change)
				}
			}
		}
	}
	return changes
}

func (s *State) upsertLandLocked(lid int32, next LandView, _ string) (LandChange, bool) {
	prev, existed := s.lands[lid]
	if existed && prev == next {
		return LandChange{}, false
	}
	s.lands[lid] = next
	return LandChange{LandID: lid, Before: prev, After: next}, true
}

func (s *State) applyInventoryLocked(ns7 map[string]json.RawMessage) {
	if raw0, ok := ns7["0"]; ok {
		var s0 map[string]json.RawMessage
		if err := json.Unmarshal(raw0, &s0); err == nil {
			if cell32, ok := s0["32"]; ok {
				s.applyInventoryCountsLocked(cell32, true)
			}
			if raw33, ok := s0["33"]; ok {
				s.applyWaterDropsLocked(raw33)
			}
			if raw44, ok := s0["44"]; ok {
				var n int32
				if json.Unmarshal(raw44, &n) == nil {
					s.gold = n
				}
			}
			if raw34, ok := s0["34"]; ok {
				var n int32
				if json.Unmarshal(raw34, &n) == nil {
					s.level = n
				}
			}
			if raw35, ok := s0["35"]; ok {
				var n int32
				if json.Unmarshal(raw35, &n) == nil {
					s.experience = n
				}
			}
			if raw36, ok := s0["36"]; ok {
				var n int32
				if json.Unmarshal(raw36, &n) == nil {
					s.vip = n
				}
			}
			if raw37, ok := s0["37"]; ok {
				var n int32
				if json.Unmarshal(raw37, &n) == nil {
					s.vipExp = n
				}
			}
			if raw41, ok := s0["41"]; ok {
				var n int32
				if json.Unmarshal(raw41, &n) == nil {
					s.diamondsFree = n
				}
			}
			if raw42, ok := s0["42"]; ok {
				var n int32
				if json.Unmarshal(raw42, &n) == nil {
					s.diamondsPaid = n
				}
			}
		}
	}

	if raw1, ok := ns7["1"]; ok && !s.hasWaterDropsItem {
		s.applyWaterDropsColdFallbackLocked(raw1)
	}

	if raw2, ok := ns7["2"]; ok {
		s.applyInventoryDeltaLocked(raw2)
	}
}

func (s *State) applyUsrExtraLocked(ns7 map[string]json.RawMessage) {
	raw13, ok := ns7["13"]
	if !ok {
		return
	}
	var usrExtTot map[string]json.RawMessage
	if err := json.Unmarshal(raw13, &usrExtTot); err != nil {
		return
	}
	rawExtra, ok := usrExtTot["1"]
	if !ok {
		return
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(rawExtra, &extra); err != nil {
		return
	}
	s.usrExtra.Observed = true
	if rawStatus, ok := extra["104"]; ok {
		var n int32
		if json.Unmarshal(rawStatus, &n) == nil {
			s.usrExtra.AntiFraudQAStatus = n
		}
	}
	if rawTime, ok := extra["105"]; ok {
		var n int64
		if json.Unmarshal(rawTime, &n) == nil {
			s.usrExtra.LastAntiFraudQATimeMs = n
		}
	}
}

func (s *State) applyZooLocked(raw json.RawMessage) {
	var ns33 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns33); err != nil {
		return
	}
	s.zooObserved = true
	if rawData, ok := ns33["0"]; ok {
		if zoo, ok := parseZooView(rawData); ok {
			s.zoo = zoo
		}
	}
	if rawPets, ok := ns33["1"]; ok {
		s.applyZooPetMapLocked(rawPets)
	}
}

func parseZooView(raw json.RawMessage) (ZooView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return ZooView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooView{}, false
	}
	view := ZooView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if rawPetIDs, ok := fields["3"]; ok {
		view.PetIDs = readInt32ListRaw(rawPetIDs)
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.ReadLogTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.Comfort = n
	}
	if n, ok := readInt64JSONField(fields, "8"); ok {
		view.UpdatedAtMs = n
	}
	if rawRewards, ok := fields["13"]; ok {
		view.SouvenirRewardIDs = readInt32ListRaw(rawRewards)
	}
	return view, true
}

func (s *State) applyZooPetMapLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var petMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &petMap); err != nil {
		return
	}
	if s.zooPets == nil {
		s.zooPets = make(map[int32]*ZooPetView)
	}
	for petIDStr, rawPet := range petMap {
		petID := atoi32(petIDStr)
		base := ZooPetView{PetID: petID}
		if old := s.zooPets[petID]; old != nil {
			base = cloneZooPetView(*old)
		}
		pet, ok := parseZooPetView(rawPet, base)
		if !ok || pet.PetID <= 0 {
			continue
		}
		cp := pet
		s.zooPets[pet.PetID] = &cp
	}
}

func parseZooPetView(raw json.RawMessage, base ZooPetView) (ZooPetView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return ZooPetView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooPetView{}, false
	}
	pet := base
	if n, ok := readInt64JSONField(fields, "0"); ok {
		pet.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
		pet.PetID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		pet.MoodValue = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		pet.SatietyValue = n
	}
	if rawFood, ok := fields["4"]; ok {
		pet.FoodstuffIDs = readInt32OrderedListRaw(rawFood)
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		pet.Status = n
	}
	if n, ok := readInt64JSONField(fields, "12"); ok {
		pet.StrokeCdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "13"); ok {
		pet.GetHomeTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "14"); ok {
		pet.StatusCdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "15"); ok {
		pet.GoOutCdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "23"); ok {
		pet.UpdatedAtMs = n
	}
	return pet, true
}

func (s *State) applyInventoryCountsLocked(raw json.RawMessage, absolute bool) {
	var inv map[string]any
	if err := json.Unmarshal(raw, &inv); err != nil {
		return
	}
	for k, v := range inv {
		id := atoi32(k)
		count := readInt32Any(v)
		if id == 0 {
			continue
		}
		if absolute {
			s.inventory[id] = count
		} else {
			s.inventory[id] += count
		}
		if id == 7 {
			s.hasWaterDropsItem = true
		}
	}
}

func (s *State) applyWaterDropsLocked(raw33 json.RawMessage) {
	var cell33 map[string]json.RawMessage
	if err := json.Unmarshal(raw33, &cell33); err != nil {
		return
	}
	raw7, ok := cell33["7"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw7, &inner); err != nil {
		return
	}
	if v, ok := inner["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.waterDropsTotal = n
		}
	}
	if v, ok := inner["5"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.waterDropsNextMs = n
		}
	}
}

func (s *State) applyWaterDropsColdFallbackLocked(raw1 json.RawMessage) {
	var cell1 map[string]json.RawMessage
	if err := json.Unmarshal(raw1, &cell1); err != nil {
		return
	}
	rawCurrent, ok := cell1["13"]
	if !ok {
		return
	}
	var n int32
	if json.Unmarshal(rawCurrent, &n) != nil {
		return
	}
	if s.waterDropsTotal > 0 && n > s.waterDropsTotal {
		return
	}
	s.inventory[7] = n
	s.hasWaterDropsItem = true
}

func (s *State) applyInventoryDeltaLocked(raw2 json.RawMessage) {
	var cell2 map[string]json.RawMessage
	if err := json.Unmarshal(raw2, &cell2); err != nil {
		return
	}
	if rawTotals, ok := cell2["2"]; ok {
		s.applyInventoryCountsLocked(rawTotals, true)
		return
	}
	if rawDelta, ok := cell2["0"]; ok {
		s.applyInventoryCountsLocked(rawDelta, false)
	}
}

func cloneInt32Map(src map[int32]int32) map[int32]int32 {
	dst := make(map[int32]int32, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneFmlFlowerShareView(src FmlFlowerShareView) FmlFlowerShareView {
	out := src
	out.Slots = make(map[int32]FmlFlowerShareSlotView, len(src.Slots))
	for slotID, slot := range src.Slots {
		out.Slots[slotID] = slot
	}
	return out
}

func cloneZooView(src ZooView) ZooView {
	out := src
	out.PetIDs = append([]int32(nil), src.PetIDs...)
	out.SouvenirRewardIDs = append([]int32(nil), src.SouvenirRewardIDs...)
	return out
}

func cloneZooPetView(src ZooPetView) ZooPetView {
	out := src
	out.FoodstuffIDs = append([]int32(nil), src.FoodstuffIDs...)
	return out
}

func inventoryChanges(before, after map[int32]int32) []InventoryItemDelta {
	seen := make(map[int32]struct{}, len(before)+len(after))
	var out []InventoryItemDelta
	for id, prev := range before {
		seen[id] = struct{}{}
		if next := after[id]; next != prev {
			out = append(out, InventoryItemDelta{ItemID: id, Before: prev, After: next})
		}
	}
	for id, next := range after {
		if _, ok := seen[id]; ok {
			continue
		}
		if next != 0 {
			out = append(out, InventoryItemDelta{ItemID: id, After: next})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}

func (s *State) applyWaterwheelLocked(raw json.RawMessage) {
	var ns114 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns114); err != nil {
		return
	}
	if v, ok := ns114["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.wwClaimedCount = n
		}
	}
	if v, ok := ns114["4"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.wwLastRecvTs = n
		}
	}
}

func (s *State) applyCultivationsLocked(raw json.RawMessage) {
	var ns101 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns101); err != nil {
		return
	}
	raw0, ok := ns101["0"]
	if !ok {
		return
	}
	var flowers map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &flowers); err != nil {
		return
	}
	for fid, rawEntry := range flowers {
		id := atoi32(fid)
		if id == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &fields); err != nil {
			continue
		}
		cv := s.cultivations[id]
		if cv == nil {
			cv = &CultivateView{FlowerID: id}
			s.cultivations[id] = cv
		}
		if v, ok := fields["2"]; ok {
			var n int32
			if json.Unmarshal(v, &n) == nil {
				cv.Lvl = n
			}
		}
		if v, ok := fields["3"]; ok {
			var n int64
			if json.Unmarshal(v, &n) == nil {
				cv.CulTimeMs = n
			}
		}
		if v, ok := fields["4"]; ok {
			var n int32
			if json.Unmarshal(v, &n) == nil {
				cv.Status = n
			}
		}
		if v, ok := fields["5"]; ok {
			var n int64
			if json.Unmarshal(v, &n) == nil {
				cv.UTimeMs = n
			}
		}
	}
}

func (s *State) applyCustomerOrdersLocked(raw json.RawMessage) {
	var ns109 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns109); err != nil {
		return
	}
	raw0, ok := ns109["0"]
	if !ok {
		return
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &outer); err != nil {
		return
	}
	s.customerOrderSummary.Observed = true
	if n, ok := readInt64JSONField(outer, "2"); ok {
		s.customerOrderSummary.NextGenTimeMs = n
	}
	if n, ok := readInt64JSONField(outer, "3"); ok {
		s.customerOrderSummary.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(outer, "4"); ok {
		s.customerOrderSummary.CreatedAtMs = n
	}
	if n, ok := readInt32JSONField(outer, "5"); ok {
		s.customerOrderSummary.CreateCount = n
	}
	raw1, ok := outer["1"]
	if !ok {
		s.customerOrderSummary.ActiveCount = int32(len(s.customerOrders))
		return
	}
	var orders map[string]json.RawMessage
	if err := json.Unmarshal(raw1, &orders); err != nil {
		s.customerOrderSummary.ActiveCount = int32(len(s.customerOrders))
		return
	}
	// Replace the full order set.
	// Older captures used fields 0=[[flowerId,count],...], 1=npcId, 3=finishCnt.
	// Current captures use fields 0=dialogId, 1=artId, 2=num, 3=pathId.
	s.customerOrders = make(map[int32]*CustomerOrder, len(orders))
	for npcID, rawOrder := range orders {
		id := atoi32(npcID)
		if id <= 0 {
			continue
		}
		order := &CustomerOrder{NPCID: id}
		storeID := id
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawOrder, &fields); err == nil {
			oldShape := false
			if rawReqs, ok := fields["0"]; ok {
				flowers, items := parseOrderRequires(rawReqs)
				order.Requires = append(order.Requires, flowers...)
				order.ItemRequires = append(order.ItemRequires, items...)
				oldShape = len(flowers) > 0 || len(items) > 0
			}
			if oldShape {
				if rawNPCID, ok := fields["1"]; ok {
					var n int32
					if json.Unmarshal(rawNPCID, &n) == nil && n > 0 {
						order.NPCID = n
						storeID = n
					}
				}
				if rawFinishCnt, ok := fields["3"]; ok {
					var n int32
					if json.Unmarshal(rawFinishCnt, &n) == nil {
						order.FinishCnt = n
					}
				}
				if rawItemID, rawItemOK := fields["1"]; rawItemOK {
					var itemID int32
					var count int32
					_ = json.Unmarshal(rawItemID, &itemID)
					if rawCount, ok := fields["2"]; ok {
						_ = json.Unmarshal(rawCount, &count)
					}
					if itemID > 0 && count > 0 && itemID != order.NPCID {
						order.ItemRequires = append(order.ItemRequires, ItemRequire{ItemID: itemID, Count: count})
					}
				}
			} else if rawItemID, ok := fields["1"]; ok {
				var itemID int32
				var count int32
				_ = json.Unmarshal(rawItemID, &itemID)
				if rawCount, ok := fields["2"]; ok {
					_ = json.Unmarshal(rawCount, &count)
				}
				if itemID > 0 && count > 0 {
					order.ItemRequires = []ItemRequire{{ItemID: itemID, Count: count}}
				}
			}
		}
		s.customerOrders[storeID] = order
	}
	s.customerOrderSummary.ActiveCount = int32(len(s.customerOrders))
}

func (s *State) applyFlowerRackLocked(raw json.RawMessage) {
	var ns104 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns104); err != nil {
		return
	}
	raw0, ok := ns104["0"]
	if !ok {
		return
	}
	var slots map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &slots); err != nil {
		return
	}
	for rackIDStr, rawSlot := range slots {
		rackID := atoi32(rackIDStr)
		if rackID <= 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawSlot, &fields); err != nil {
			continue
		}
		slot := s.flowerRack[rackID]
		if slot == nil {
			slot = &FlowerRackSlot{RackID: rackID}
			s.flowerRack[rackID] = slot
		}
		if rawRackID, ok := fields["1"]; ok {
			var n int32
			if json.Unmarshal(rawRackID, &n) == nil && n > 0 {
				slot.RackID = n
			}
		}
		if rawItemID, ok := fields["2"]; ok {
			var n int32
			if json.Unmarshal(rawItemID, &n) == nil {
				slot.ItemID = n
			}
		}
		if rawCount, ok := fields["3"]; ok {
			var n int32
			if json.Unmarshal(rawCount, &n) == nil {
				slot.Count = n
			}
		}
		if rawListedAt, ok := fields["4"]; ok {
			var n int64
			if json.Unmarshal(rawListedAt, &n) == nil {
				slot.ListedAtMs = n
			}
		}
		if rawUpdatedAt, ok := fields["5"]; ok {
			var n int64
			if json.Unmarshal(rawUpdatedAt, &n) == nil {
				slot.UpdatedAtMs = n
			}
		}
		if slot.ItemID == 0 || slot.Count == 0 {
			slot.ItemID = 0
			slot.Count = 0
			slot.SellReadyAtMs = 0
		} else if sellDurationMs := FlowerRackSellDurationMs(); sellDurationMs > 0 && slot.ListedAtMs > 0 {
			slot.SellReadyAtMs = slot.ListedAtMs + int64(slot.Count)*sellDurationMs
		}
	}
}

func (s *State) applyMailLocked(raw json.RawMessage) {
	var ns19 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns19); err != nil {
		return
	}
	s.mailObserved = true
	rawList, ok := ns19["1"]
	if !ok || len(rawList) == 0 || string(rawList) == "null" {
		return
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(rawList, &entries); err != nil {
		return
	}
	if s.mails == nil {
		s.mails = make(map[string]*MailView)
	}
	for _, rawEntry := range entries {
		mail, ok := parseMailView(rawEntry)
		if !ok {
			continue
		}
		key := mailKey(mail.MsID, mail.AllID)
		if key == "" {
			continue
		}
		next := mail
		s.mails[key] = &next
	}
}

func parseMailView(raw json.RawMessage) (MailView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return MailView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return MailView{}, false
	}
	view := MailView{}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.MsID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.AllID = n
	}
	if n, ok := readInt32JSONField(fields, "17"); ok {
		view.IsDel = n
	}
	if n, ok := readInt32JSONField(fields, "18"); ok {
		view.IsRead = n
	}
	if n, ok := readInt32JSONField(fields, "20"); ok {
		view.IsPick = n
	}
	if rawItems, ok := fields["13"]; ok {
		view.ItemsRaw = append(json.RawMessage(nil), rawItems...)
	}
	return view, view.MsID > 0 || view.AllID > 0
}

func mailKey(msID, allID int32) string {
	if msID <= 0 && allID <= 0 {
		return ""
	}
	return strconv.FormatInt(int64(msID), 10) + ":" + strconv.FormatInt(int64(allID), 10)
}

func mailHasItems(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return len(arr) > 0
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		return len(obj) > 0
	}
	return true
}

func (s *State) applyVasesLocked(raw json.RawMessage) {
	var ns102 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns102); err != nil {
		return
	}
	raw0, ok := ns102["0"]
	if !ok {
		return
	}
	var vaseMap map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &vaseMap); err != nil {
		return
	}
	s.vaseObserved = true
	next := make(map[int32]*VaseView, len(vaseMap))
	for vaseIDStr, rawVase := range vaseMap {
		vaseID := atoi32(vaseIDStr)
		if vaseID <= 0 {
			continue
		}
		view := &VaseView{VaseID: vaseID}
		if len(rawVase) > 0 && string(rawVase) != "{}" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawVase, &fields); err == nil {
				if rawID, ok := fields["1"]; ok {
					var n int32
					if json.Unmarshal(rawID, &n) == nil && n > 0 {
						view.VaseID = n
					}
				}
				if rawUTime, ok := fields["2"]; ok {
					_ = json.Unmarshal(rawUTime, &view.UTimeMs)
				}
				if rawCTime, ok := fields["3"]; ok {
					_ = json.Unmarshal(rawCTime, &view.CTimeMs)
				}
			}
		}
		next[view.VaseID] = view
	}
	s.vases = next
}

func (s *State) applyFlowerArtLocked(raw json.RawMessage) {
	var ns106 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns106); err != nil {
		return
	}
	raw0, ok := ns106["0"]
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &fields); err != nil {
		return
	}
	s.flowerArt.Observed = true
	if rawExp, ok := fields["1"]; ok {
		_ = json.Unmarshal(rawExp, &s.flowerArt.Exp)
	}
	if rawMakeList, ok := fields["2"]; ok {
		s.flowerArt.MakeListRaw = cloneRaw(rawMakeList)
		s.flowerArt.MakeList = readInt32ListRaw(rawMakeList)
	}
	if rawSRecvList, ok := fields["3"]; ok {
		s.flowerArt.SRecvListRaw = cloneRaw(rawSRecvList)
		s.flowerArt.SRecvList = readInt32ListRaw(rawSRecvList)
	}
	if rawUTime, ok := fields["4"]; ok {
		_ = json.Unmarshal(rawUTime, &s.flowerArt.UTimeMs)
	}
	if rawCTime, ok := fields["5"]; ok {
		_ = json.Unmarshal(rawCTime, &s.flowerArt.CTimeMs)
	}
}

func (s *State) applyStatisticsLocked(raw json.RawMessage) {
	var ns124 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns124); err != nil {
		return
	}
	raw0, ok := ns124["0"]
	if !ok {
		return
	}
	if view, ok := parseStatisticsView(raw0); ok {
		s.statistics = view
	}
}

func parseStatisticsView(raw json.RawMessage) (StatisticsView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return StatisticsView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return StatisticsView{}, false
	}
	if _, ok := fields["9"]; ok {
		return parseStatisticsFields(fields)
	}
	var best StatisticsView
	for dayIDStr, rawEntry := range fields {
		var entryFields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &entryFields); err != nil {
			continue
		}
		entry, ok := parseStatisticsFields(entryFields)
		if !ok {
			continue
		}
		if entry.DayID == 0 {
			entry.DayID = atoi32(dayIDStr)
		}
		if !best.Observed || entry.DayID >= best.DayID {
			best = entry
		}
	}
	if best.Observed {
		return best, true
	}
	return StatisticsView{}, false
}

func parseStatisticsFields(fields map[string]json.RawMessage) (StatisticsView, bool) {
	view := StatisticsView{Observed: true}
	seen := false
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.DayID = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.FlowerArtSellNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "9"); ok {
		view.OrderFlowerFinishNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "10"); ok {
		view.OrderPalaceFinishNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "11"); ok {
		view.OrderCustomerFinishNum = n
		seen = true
	}
	if n, ok := readInt64JSONField(fields, "12"); ok {
		view.UTimeMs = n
		seen = true
	}
	if n, ok := readInt64JSONField(fields, "13"); ok {
		view.CTimeMs = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "14"); ok {
		view.OrderSatinFinishNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "16"); ok {
		view.OrderDecorateFinishNum = n
		seen = true
	}
	return view, seen
}

func (s *State) applyFmlLocked(raw json.RawMessage) {
	var ns25 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns25); err != nil {
		return
	}
	s.fmlBuild.Observed = true
	if s.fmlBuild.BuildCounts == nil {
		s.fmlBuild.BuildCounts = make(map[int32]int32)
	}
	if rawFml, ok := ns25["0"]; ok {
		s.applyFmlObjectLocked(rawFml)
	}
	if rawBuild, ok := ns25["133"]; ok {
		s.applyFmlBuildObjectLocked(rawBuild)
	}
	if rawLand, ok := ns25["102"]; ok {
		s.applyFmlLandObjectLocked(rawLand)
	}
	if rawForestEnergy, ok := ns25["127"]; ok {
		s.applyFmlForestEnergyObjectLocked(rawForestEnergy)
	}
	if rawShare, ok := ns25["107"]; ok {
		if view, ok := parseFmlFlowerShare(rawShare); ok {
			s.fmlFlowerShare = view
		}
	}
	if rawOtherShares, ok := ns25["108"]; ok {
		s.applyOtherFmlFlowerSharesObjectLocked(rawOtherShares)
	}
}

func (s *State) applyFmlObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if id, ok := readInt32JSONField(fields, "0"); ok {
		s.fmlBuild.FmlID = id
	}
	if count, ok := readInt32JSONField(fields, "19", "113"); ok {
		s.fmlBuild.TodayBuildNum = count
	}
	if ts, ok := readInt64JSONField(fields, "20", "29"); ok {
		s.fmlBuild.LastBuildTimeMs = ts
	}
	if rawCounts, ok := fields["30"]; ok {
		s.setFmlBuildCountsLocked(rawCounts)
	}
}

func (s *State) applyFmlBuildObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if id, ok := readInt32JSONField(fields, "1"); ok {
		s.fmlBuild.FmlID = id
	}
	if ts, ok := readInt64JSONField(fields, "4"); ok {
		s.fmlBuild.LastBuildTimeMs = ts
	}
	if rawCounts, ok := fields["5"]; ok {
		s.setFmlBuildCountsLocked(rawCounts)
	}
}

func (s *State) setFmlBuildCountsLocked(raw json.RawMessage) {
	counts := readInt32RawMap(raw)
	s.fmlBuild.BuildCountsObserved = true
	s.fmlBuild.BuildCounts = counts
}

func (s *State) applyFmlLandObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	rawLandMap, ok := fields["1"]
	if !ok {
		return
	}
	var landMap map[string]json.RawMessage
	if err := json.Unmarshal(rawLandMap, &landMap); err != nil {
		return
	}
	next := make(map[int32]*FmlLandView, len(landMap))
	for landIDStr, rawLand := range landMap {
		landID := atoi32(landIDStr)
		if landID <= 0 {
			continue
		}
		view := &FmlLandView{LandID: landID}
		if len(rawLand) > 0 && string(rawLand) != "{}" {
			var landFields map[string]json.RawMessage
			if err := json.Unmarshal(rawLand, &landFields); err == nil {
				if n, ok := readInt32JSONField(landFields, "0"); ok {
					view.Level = n
				}
				if n, ok := readInt32JSONField(landFields, "1"); ok {
					view.FlowerID = n
				}
				if n, ok := readInt64JSONField(landFields, "2"); ok {
					view.StartTimeMs = n
				}
				if n, ok := readInt32JSONField(landFields, "3"); ok {
					view.MatureFlowerCnt = n
				}
				if n, ok := readInt32JSONField(landFields, "4"); ok {
					view.HarvestedCnt = n
				}
				if n, ok := readInt64JSONField(landFields, "5"); ok {
					view.LastCalcTimeMs = n
				}
			}
		}
		next[landID] = view
	}
	s.fmlLands = next
	s.fmlLandObserved = true
}

func (s *State) applyFmlForestEnergyObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	view := FmlForestEnergyView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.FmlID = n
	}
	if rawEnergy, ok := fields["2"]; ok {
		view.EnergyByType = readInt32RawMap(rawEnergy)
	}
	if rawDaily, ok := fields["6"]; ok {
		view.DailyEnergyByType = readInt32RawMap(rawDaily)
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "7"); ok {
		view.LastDailyRefreshTimeMs = n
	}
	if rawTemp, ok := fields["8"]; ok {
		view.PendingTempEnergyByType, view.PendingTempEnergyTotal = readNestedInt32RawMapTotals(rawTemp)
	}
	if view.EnergyByType == nil {
		view.EnergyByType = map[int32]int32{}
	}
	if view.DailyEnergyByType == nil {
		view.DailyEnergyByType = map[int32]int32{}
	}
	if view.PendingTempEnergyByType == nil {
		view.PendingTempEnergyByType = map[int32]int32{}
	}
	s.fmlForestEnergy = view
}

func (s *State) applyOtherFmlFlowerSharesObjectLocked(raw json.RawMessage) {
	next := make(map[int64]*FmlFlowerShareView)
	if len(raw) == 0 || string(raw) == "null" {
		s.fmlOtherFlowerShares = next
		s.fmlOtherShareObserved = true
		return
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, rawShare := range list {
			view, ok := parseFmlFlowerShare(rawShare)
			if !ok || view.UID == 0 {
				continue
			}
			cp := view
			next[view.UID] = &cp
		}
		s.fmlOtherFlowerShares = next
		s.fmlOtherShareObserved = true
		return
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return
	}
	for uidStr, rawShare := range values {
		view, ok := parseFmlFlowerShare(rawShare)
		if !ok {
			continue
		}
		if view.UID == 0 {
			view.UID = atoi64(uidStr)
		}
		if view.UID == 0 {
			continue
		}
		cp := view
		next[view.UID] = &cp
	}
	s.fmlOtherFlowerShares = next
	s.fmlOtherShareObserved = true
}

func parseFmlFlowerShare(raw json.RawMessage) (FmlFlowerShareView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return FmlFlowerShareView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return FmlFlowerShareView{}, false
	}
	view := FmlFlowerShareView{Observed: true, Slots: make(map[int32]FmlFlowerShareSlotView)}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if rawSlots, ok := fields["1"]; ok {
		view.Slots = parseFmlFlowerShareSlots(rawSlots)
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.TdyTakeCnt = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.LastTakeTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "5"); ok {
		view.CreatedAtMs = n
	}
	return view, true
}

func parseFmlFlowerShareSlots(raw json.RawMessage) map[int32]FmlFlowerShareSlotView {
	out := make(map[int32]FmlFlowerShareSlotView)
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	var slots map[string]json.RawMessage
	if err := json.Unmarshal(raw, &slots); err != nil {
		return out
	}
	for slotIDStr, rawSlot := range slots {
		slotID := atoi32(slotIDStr)
		if slotID <= 0 {
			continue
		}
		slot := FmlFlowerShareSlotView{SlotID: slotID}
		if len(rawSlot) > 0 && string(rawSlot) != "{}" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawSlot, &fields); err == nil {
				if n, ok := readInt32JSONField(fields, "0"); ok {
					slot.FlowerID = n
				}
				if n, ok := readInt32JSONField(fields, "1"); ok {
					slot.ShareNum = n
				}
				if n, ok := readInt32JSONField(fields, "2"); ok {
					slot.TakeNum = n
				}
				if n, ok := readInt64JSONField(fields, "3"); ok {
					slot.ShareStartTimeMs = n
				}
			}
		}
		out[slotID] = slot
	}
	return out
}

func (s *State) applyCollectRewardsLocked(raw json.RawMessage) {
	var ns103 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns103); err != nil {
		return
	}
	raw0, ok := ns103["0"]
	if !ok {
		return
	}
	var rewards map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &rewards); err != nil {
		return
	}
	next := make(map[int32]*CollectRewardView, len(rewards))
	for typeStr, rawReward := range rewards {
		typeID := atoi32(typeStr)
		if typeID == 0 {
			continue
		}
		view := &CollectRewardView{Type: typeID}
		if len(rawReward) > 0 && string(rawReward) != "{}" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawReward, &fields); err == nil {
				if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
					view.Type = n
				}
				if n, ok := readInt32JSONField(fields, "2"); ok {
					view.Lvl = n
				}
				if n, ok := readInt32JSONField(fields, "3"); ok {
					view.Exp = n
				}
				if rawRecv, ok := fields["4"]; ok {
					view.RecvIDs = readInt32ListRaw(rawRecv)
				}
				if n, ok := readInt64JSONField(fields, "5"); ok {
					view.UTimeMs = n
				}
				if n, ok := readInt64JSONField(fields, "6"); ok {
					view.CTimeMs = n
				}
				if rawArtCreate, ok := fields["7"]; ok {
					view.ArtCreateRewardIDs = readInt32ListRaw(rawArtCreate)
				}
			}
		}
		next[view.Type] = view
	}
	s.collectRewards = next
	s.collectRewardObserved = true
}

func (s *State) applyShopCultivateLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if s.shopCultivateCosts == nil {
		s.shopCultivateCosts = make(map[int32]ItemCount)
	}
	if s.shopCultivateBought == nil {
		s.shopCultivateBought = make(map[int32]int32)
	}
	if rawInfo, ok := fields["1"]; ok {
		var costs map[string]json.RawMessage
		if err := json.Unmarshal(rawInfo, &costs); err == nil {
			next := make(map[int32]ItemCount, len(costs))
			for shopIDStr, rawCost := range costs {
				shopID := atoi32(shopIDStr)
				if shopID == 0 {
					continue
				}
				parts := readInt32OrderedListRaw(rawCost)
				if len(parts) < 2 || parts[0] <= 0 || parts[1] <= 0 {
					continue
				}
				next[shopID] = ItemCount{ItemID: parts[0], Count: parts[1]}
			}
			s.shopCultivateCosts = next
		}
	}
	if rawBought, ok := fields["6"]; ok {
		s.shopCultivateBought = readInt32RawMap(rawBought)
	}
	s.shopCultivateObserved = true
}

func (s *State) applyShopGiftbagLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if rawDRecord, ok := fields["1"]; ok {
		s.shopGiftbagDRecord = readInt32RawMap(rawDRecord)
	}
	if rawWRecord, ok := fields["2"]; ok {
		s.shopGiftbagWRecord = readInt32RawMap(rawWRecord)
	}
	if rawMRecord, ok := fields["3"]; ok {
		s.shopGiftbagMRecord = readInt32RawMap(rawMRecord)
	}
	if rawTRecord, ok := fields["4"]; ok {
		s.shopGiftbagTRecord = readInt32RawMap(rawTRecord)
	}
	s.shopGiftbagObserved = true
}

func (s *State) applyPearlLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if s.pearlPlaces == nil {
		s.pearlPlaces = make(map[int32]*PearlPlaceView)
	}
	if rawPlaces, ok := fields["0"]; ok {
		var places map[string]json.RawMessage
		if err := json.Unmarshal(rawPlaces, &places); err == nil {
			for placeIDStr, rawPlace := range places {
				placeID := atoi32(placeIDStr)
				if placeID == 0 {
					continue
				}
				view := s.pearlPlaces[placeID]
				if view == nil {
					view = &PearlPlaceView{PlaceID: placeID}
					s.pearlPlaces[placeID] = view
				}
				applyPearlPlaceFields(view, rawPlace)
			}
		}
	}
	if rawPearl, ok := fields["1"]; ok {
		applyPearlFields(&s.pearl, rawPearl)
	}
	if rawDraw, ok := fields["2"]; ok {
		s.pearlDrawRaw = cloneRaw(rawDraw)
		s.pearlDrawCount = rawCollectionCount(rawDraw)
	}
	s.pearlObserved = true
}

func applyPearlFields(view *PearlView, raw json.RawMessage) {
	if view == nil {
		return
	}
	view.Observed = true
	if len(raw) == 0 || string(raw) == "{}" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.ProtectState = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.ProtectNum = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.OwnerUID = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.LaborEndTime = n
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.RecvDailyDate = n
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.HireState = n
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.SmallDrawCnt = n
	}
	if n, ok := readInt64JSONField(fields, "9"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "10"); ok {
		view.CTimeMs = n
	}
}

func applyPearlPlaceFields(view *PearlPlaceView, raw json.RawMessage) {
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
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.LaborUID = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.LaborEndTime = n
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.HireFailCnt = n
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.EventID = n
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.EveryMakeNum = n
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.RecvCnt = n
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.SurplusRecvNum = n
	}
	if n, ok := readInt64JSONField(fields, "9"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "10"); ok {
		view.CTimeMs = n
	}
}

func (s *State) applyFlowerOrdersLocked(raw json.RawMessage) {
	// NS105 structure: {"0": {"1": {boxId: {order...}}, ...}}
	var ns105 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns105); err != nil {
		return
	}
	raw0, ok := ns105["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &inner); err != nil {
		return
	}
	if raw1, ok := inner["1"]; ok {
		var boxes map[string]json.RawMessage
		if err := json.Unmarshal(raw1, &boxes); err == nil {
			s.flowerOrders = make(map[int32]*FlowerOrder, len(boxes))
			for boxIDStr, rawBox := range boxes {
				boxID := atoi32(boxIDStr)
				if boxID == 0 {
					continue
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(rawBox, &fields); err != nil {
					continue
				}
				order := &FlowerOrder{BoxID: boxID}
				if rawMode, ok := fields["0"]; ok {
					_ = json.Unmarshal(rawMode, &order.Mode)
				}
				// field "2" = [[flowerId, count], ...]
				if rawReqs, ok := fields["2"]; ok {
					order.Requires = parseFlowerRequires(rawReqs)
				}
				s.flowerOrders[boxID] = order
			}
		}
	}
	if rawReceived, ok := inner["2"]; ok {
		var ids []int32
		if json.Unmarshal(rawReceived, &ids) == nil {
			s.flowerOrderRewardsReceived = make(map[int32]bool, len(ids))
			for _, id := range ids {
				if id > 0 {
					s.flowerOrderRewardsReceived[id] = true
				}
			}
		}
	}
	if rawSatin, ok := inner["6"]; ok {
		s.residentSatinOrder = parseResidentSpecialOrder(rawSatin)
	}
	if rawDecorate, ok := inner["7"]; ok {
		s.residentDecorateOrder = parseResidentSpecialOrder(rawDecorate)
	}
}

func parseResidentSpecialOrder(raw json.RawMessage) ResidentSpecialOrder {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ResidentSpecialOrder{}
	}
	view := ResidentSpecialOrder{Observed: true}
	if n, ok := readInt32JSONField(fields, "0"); ok {
		view.Flowers = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.NPCID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.DialogID = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.FinishCnt = n
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.IsVideo = n
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.VideoRwd = n
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.CdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "7"); ok {
		view.CTimeMs = n
	}
	return view
}

func (s *State) applyTeamOrderLocked(raw json.RawMessage) {
	var ns107 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns107); err != nil {
		return
	}
	raw0, ok := ns107["0"]
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &fields); err != nil {
		return
	}
	view := TeamOrderView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.Status = n
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.StartTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.OrderNum = n
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.FlowerID = n
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.Reward = n
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.RemainingNum = n
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.RefreshNotCnt = n
	}
	if n, ok := readInt64JSONField(fields, "8"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "9"); ok {
		view.CTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "10"); ok {
		view.ActiveTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "11"); ok {
		view.ActiveCnt = n
	}
	if n, ok := readInt32JSONField(fields, "14"); ok {
		view.NPCID = n
	}
	s.teamOrder = view
}

func (s *State) applyPalaceOrderLocked(raw json.RawMessage) {
	var ns108 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns108); err != nil {
		return
	}
	raw0, ok := ns108["0"]
	if !ok {
		return
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &outer); err != nil {
		return
	}
	rawOrder := raw0
	if nested, ok := outer["0"]; ok {
		rawOrder = nested
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawOrder, &fields); err != nil {
		return
	}
	view := PalaceOrderView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.FlowerID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.Num = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.IsFinish = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.LTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "5"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.CTimeMs = n
	}
	s.palaceOrder = view
}

func (s *State) applyRoadGrowLocked(raw json.RawMessage) {
	var ns119 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns119) != nil {
		return
	}
	rawRecv, ok := ns119["3"]
	if !ok {
		return
	}
	var recv map[string]int32
	if json.Unmarshal(rawRecv, &recv) != nil {
		return
	}
	s.roadGrowReceived = make(map[int32]bool, len(recv))
	for id, v := range recv {
		if v != 0 {
			s.roadGrowReceived[atoi32(id)] = true
		}
	}
}

func (s *State) applyRandomEventsLocked(raw json.RawMessage) {
	var ns129 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns129) != nil {
		return
	}
	raw0, ok := ns129["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(raw0, &inner) != nil {
		return
	}
	rawEvents, ok := inner["1"]
	if !ok {
		return
	}
	var events map[string]json.RawMessage
	if json.Unmarshal(rawEvents, &events) != nil {
		return
	}
	s.randomEvents = make(map[int32]*RandomEventView, len(events))
	for idStr, rawEvent := range events {
		id := atoi32(idStr)
		if id == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEvent, &fields) != nil {
			continue
		}
		event := &RandomEventView{EventID: id}
		if rawID, ok := fields["0"]; ok {
			_ = json.Unmarshal(rawID, &event.EventID)
		}
		if rawStatus, ok := fields["1"]; ok {
			_ = json.Unmarshal(rawStatus, &event.Status)
		}
		if rawAffair, ok := fields["2"]; ok {
			_ = json.Unmarshal(rawAffair, &event.Affair)
		}
		s.randomEvents[id] = event
	}
}

func parseFlowerRequires(raw json.RawMessage) []FlowerRequire {
	flowers, _ := parseOrderRequires(raw)
	return flowers
}

func parseOrderRequires(raw json.RawMessage) ([]FlowerRequire, []ItemRequire) {
	var reqs [][]int32
	if json.Unmarshal(raw, &reqs) != nil {
		return nil, nil
	}
	flowers := make([]FlowerRequire, 0, len(reqs))
	items := make([]ItemRequire, 0, len(reqs))
	for _, req := range reqs {
		if len(req) >= 2 && req[0] > 0 && req[1] > 0 {
			if isFlowerItemID(req[0]) {
				flowers = append(flowers, FlowerRequire{FlowerID: req[0], Count: req[1]})
			} else {
				items = append(items, ItemRequire{ItemID: req[0], Count: req[1]})
			}
		}
	}
	return flowers, items
}

func (s *State) applyTasksLocked(raw json.RawMessage) {
	var ns22 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns22); err != nil {
		return
	}
	if rawMain, ok := ns22["0"]; ok {
		var main map[string]json.RawMessage
		if err := json.Unmarshal(rawMain, &main); err == nil {
			task := &MainTaskView{}
			if rawTaskID, ok := main["1"]; ok {
				var n int32
				if json.Unmarshal(rawTaskID, &n) == nil {
					task.TaskID = n
				}
			}
			if rawFinished, ok := main["2"]; ok {
				var n int32
				if json.Unmarshal(rawFinished, &n) == nil {
					task.Finished = n
				}
			}
			if task.TaskID > 0 {
				s.mainTask = task
			}
		}
	}
	if rawDaily, ok := ns22["1"]; ok {
		s.applyDailyTasksLocked(rawDaily)
	}
	if rawWeekly, ok := ns22["100"]; ok {
		s.applyWeeklyTasksLocked(rawWeekly)
	}
}

func (s *State) applyDailyTasksLocked(rawDaily json.RawMessage) {
	var daily map[string]json.RawMessage
	if err := json.Unmarshal(rawDaily, &daily); err != nil {
		return
	}

	progressMap := readInt32RawMap(daily["1"])
	recvMap := readInt32RawMap(daily["3"])

	rawTaskMap, ok := daily["100"]
	if !ok {
		return
	}
	var tasks map[string]json.RawMessage
	if err := json.Unmarshal(rawTaskMap, &tasks); err != nil {
		return
	}
	s.dailyTasks = make(map[int32]*DailyTaskView, len(tasks))
	for idStr, rawTask := range tasks {
		id := atoi32(idStr)
		if id == 0 {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal(rawTask, &fields); err != nil {
			continue
		}
		taskID := int32(readInt(fields, "0"))
		if taskID == 0 {
			taskID = id
		}
		finished := readInt32Any(fields["2"])
		if progressType, ok := DailyTaskProgressType(taskID); ok {
			if progress := progressMap[progressType]; progress > finished {
				finished = progress
			}
		}
		s.dailyTasks[id] = &DailyTaskView{
			TaskID:    taskID,
			Target:    readInt32Any(fields["1"]),
			Finished:  finished,
			Status:    readInt32Any(fields["4"]),
			Receipted: recvMap[id],
		}
	}
}

func (s *State) applyWeeklyTasksLocked(rawWeekly json.RawMessage) {
	var weekly map[string]json.RawMessage
	if err := json.Unmarshal(rawWeekly, &weekly); err != nil {
		return
	}
	rawProgress, progressObserved := weekly["1"]
	rawRecv, recvObserved := weekly["3"]
	progressMap := readInt32RawMap(rawProgress)
	recvMap := readInt32RawMap(rawRecv)
	defs := WeeklyTaskDefinitions()
	prev := s.weeklyTasks
	s.weeklyTasks = make(map[int32]*WeeklyTaskView, len(defs))
	for _, def := range defs {
		finished := progressMap[def.ProgressType]
		receipted := recvMap[def.TaskID]
		if old := prev[def.TaskID]; old != nil {
			if !progressObserved {
				finished = old.Finished
			}
			if !recvObserved {
				receipted = old.Receipted
			}
		}
		status := int32(2)
		if receipted != 0 {
			status = 3
		} else if def.Target > 0 && finished >= def.Target {
			status = 1
		}
		s.weeklyTasks[def.TaskID] = &WeeklyTaskView{
			TaskID:    def.TaskID,
			Target:    def.Target,
			Finished:  finished,
			Status:    status,
			Receipted: receipted,
		}
	}
}

func (s *State) applyFreeWaterLocked(raw json.RawMessage) {
	var ns117 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns117); err != nil {
		return
	}
	s.freeWaterObserved = true
	if v, ok := ns117["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.freeWaterRecvIdx = n
		}
	}
	if v, ok := ns117["2"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.freeWaterResetMs = n
		}
	}
}

func (s *State) applyBenefitBoxLocked(raw json.RawMessage) {
	// NS 116 client schema: {"0": {"1": drawCnt, "2": resetCntTime, "3": uTime, "4": cTime}}
	var ns116 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns116); err != nil {
		return
	}
	s.benefitBoxObserved = true
	raw0, ok := ns116["0"]
	if !ok {
		return
	}
	var sub map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &sub); err != nil {
		return
	}
	if v, ok := sub["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxDrawCnt = n
		}
	}
	if v, ok := sub["2"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxResetCntMs = n
		}
	}
	if v, ok := sub["3"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxUTimeMs = n
		}
	}
}

func (s *State) applyVideoDoubleLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	view := s.videoDouble
	view.Observed = true
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.VideoCount = n
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.EndTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.CreatedAtMs = n
	}
	s.videoDouble = view
}

// BenefitBoxDrawsRemaining returns the number of free draws available.
func (s *State) BenefitBoxDrawsRemaining() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxDrawCnt
}

// BenefitBoxReady returns true if there are draws available.
func (s *State) BenefitBoxReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxObserved && s.benefitBoxDrawCnt > 0
}

// UsrExtra returns the tracked account-extension state.
func (s *State) UsrExtra() UsrExtraView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usrExtra
}

// AntiFraudQAStatus returns the observed anti-fraud QA reward status.
func (s *State) AntiFraudQAStatus() (int32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usrExtra.AntiFraudQAStatus, s.usrExtra.Observed
}

// VideoDouble returns the tracked double-coin video reward state.
func (s *State) VideoDouble() VideoDoubleView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoDouble
}

// VideoDoubleObserved reports whether namespace 118 has been observed.
func (s *State) VideoDoubleObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoDouble.Observed
}

// VideoDoubleActive reports whether the client-observed double-coin timer is active.
func (s *State) VideoDoubleActive(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoDoubleActiveLocked(now)
}

func (s *State) videoDoubleActiveLocked(now time.Time) bool {
	return s.videoDouble.Observed && s.videoDouble.EndTimeMs > now.UnixMilli()
}

// ZooObserved reports whether namespace 33 has been observed.
func (s *State) ZooObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooObserved
}

// Zoo returns the tracked animal-home state.
func (s *State) Zoo() ZooView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneZooView(s.zoo)
}

// ZooPets returns a defensive copy of the pet map.
func (s *State) ZooPets() map[int32]ZooPetView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]ZooPetView, len(s.zooPets))
	for id, pet := range s.zooPets {
		if pet == nil {
			continue
		}
		out[id] = cloneZooPetView(*pet)
	}
	return out
}

// ReadyZooFeedPetIDs returns pets with bowl food that can currently eat.
func (s *State) ReadyZooFeedPetIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.zooPets))
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || len(pet.FoodstuffIDs) == 0 {
			continue
		}
		if zooPetCanEat(pet.Status) {
			out = append(out, petID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyZooStrokePetIDs returns pets that match the client's touch red-dot gate.
func (s *State) ReadyZooStrokePetIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.zooPets))
	nowMs := now.UnixMilli()
	moodMax := ZooMoodMax()
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || pet.Status <= 0 {
			continue
		}
		if !zooPetTouchable(pet.Status) {
			continue
		}
		if moodMax > 0 && pet.MoodValue >= moodMax {
			continue
		}
		if pet.StrokeCdTimeMs > 0 && nowMs < pet.StrokeCdTimeMs {
			continue
		}
		out = append(out, petID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ZooMoodMax returns the client-configured pet mood cap.
func ZooMoodMax() int32 {
	raw, ok := StaticRow("c_zoo", -1)
	if !ok {
		return 100
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return 100
	}
	if n, ok := readInt32JSONField(fields, "$moodMax1", "$moodMax"); ok && n > 0 {
		return n
	}
	return 100
}

func zooPetTouchable(status int32) bool {
	fields, ok := zooStateRow(status)
	if !ok {
		return true
	}
	if n, ok := readInt32JSONField(fields, "isTouch"); ok {
		return n != 0
	}
	return true
}

func zooPetCanEat(status int32) bool {
	fields, ok := zooStateRow(status)
	if !ok {
		return false
	}
	if n, ok := readInt32JSONField(fields, "isEat"); ok {
		return n != 0
	}
	return false
}

func zooStateRow(status int32) (map[string]json.RawMessage, bool) {
	raw, ok := StaticRow("c_zooState", status)
	if !ok {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	return fields, true
}

// Lands returns a copy of the land map.
func (s *State) Lands() map[int32]LandView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]LandView, len(s.lands))
	for k, v := range s.lands {
		out[k] = v
	}
	return out
}

// LandRosterObserved reports whether the cold-start `100.0.1` land roster has
// arrived. Once true, absence from Lands means the server did not include that
// land in the player's opened/owned land list.
func (s *State) LandRosterObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.landRosterObserved
}

// SetFarmLands replaces the per-account runtime c_farmLand view loaded from
// the current client resource pack. This intentionally does not fall back to
// embedded static data, because stale land tables cause wrong unlock decisions.
func (s *State) SetFarmLands(lands []FarmLandInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.farmLands = make(map[int32]FarmLandInfo, len(lands))
	for _, land := range lands {
		if land.ID <= 0 {
			continue
		}
		s.farmLands[land.ID] = cloneFarmLandInfo(land)
	}
	s.farmLandObserved = len(s.farmLands) > 0
}

// FarmLandConfigObserved reports whether the current client-side land table has
// been loaded for this running account session.
func (s *State) FarmLandConfigObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.farmLandObserved
}

// FarmLands returns the runtime c_farmLand rows loaded for this account,
// sorted by id. It returns nil until SetFarmLands succeeds.
func (s *State) FarmLands() []FarmLandInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.farmLandObserved {
		return nil
	}
	out := make([]FarmLandInfo, 0, len(s.farmLands))
	for _, land := range s.farmLands {
		if land.ID > 0 {
			out = append(out, cloneFarmLandInfo(land))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// FarmLand returns one runtime c_farmLand row for this account.
func (s *State) FarmLand(id int32) (FarmLandInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.farmLandObserved {
		return FarmLandInfo{}, false
	}
	land, ok := s.farmLands[id]
	if !ok {
		return FarmLandInfo{}, false
	}
	return cloneFarmLandInfo(land), true
}

// MarkLandsWatered forces the given lands to state=2 (growing) locally and
// spends one local water drop per land when item 7 is tracked. Some successful
// water RPC responses omit inventory deltas, so this keeps the next plan from
// reusing a stale water balance.
func (s *State) MarkLandsWatered(landIDs []int32) {
	now := time.Now()
	s.mu.Lock()
	prevWaterDrops := s.currentWaterDropsLocked()
	prevWaterNextMs := s.waterDropsNextMs
	prevInventory := cloneInt32Map(s.inventory)
	var changes []LandChange
	if nextDrops, nextMs, recovered := s.projectedWaterDropsLocked(now); recovered > 0 {
		s.inventory[7] = nextDrops
		s.waterDropsNextMs = nextMs
	}
	beforeSpend := s.currentWaterDropsLocked()
	for _, id := range landIDs {
		if l, ok := s.lands[id]; ok && l.State == 1 {
			before := l
			l.State = 2
			l.PlantTimeMs = now.UnixMilli()
			s.lands[id] = l
			changes = append(changes, LandChange{LandID: id, Before: before, After: l})
		}
	}
	if s.hasWaterDropsItem && len(landIDs) > 0 {
		spend := int32(len(landIDs))
		if spend >= s.inventory[7] {
			s.inventory[7] = 0
		} else {
			s.inventory[7] -= spend
		}
		if s.waterDropsTotal > 0 && s.inventory[7] < s.waterDropsTotal && (beforeSpend >= s.waterDropsTotal || s.waterDropsNextMs <= 0) {
			s.waterDropsNextMs = now.UnixMilli() + waterDropRestoreIntervalMs()
		}
	}
	resourceSnap := ResourceSnapshot{
		Gold: s.gold, WaterDrops: s.currentWaterDropsLocked(), WaterDropsTotal: s.waterDropsTotal, WaterDropsNextMs: s.waterDropsNextMs,
		Level: s.level, Experience: s.experience, Vip: s.vip, VipExp: s.vipExp, NobleEligible: s.nobleEligibleLocked(),
		DiamondsFree: s.diamondsFree, DiamondsPaid: s.diamondsPaid,
	}
	invChanges := inventoryChanges(prevInventory, s.inventory)
	var inventorySnap InventorySnapshot
	resourceCb := s.onResourceChange
	inventoryCb := s.onInventoryChange
	landCb := s.onChange
	waterChanged := resourceSnap.WaterDrops != prevWaterDrops || resourceSnap.WaterDropsNextMs != prevWaterNextMs
	if len(invChanges) > 0 {
		inventorySnap = InventorySnapshot{Inventory: cloneInt32Map(s.inventory), Changes: invChanges}
	}
	s.mu.Unlock()

	if landCb != nil && len(changes) > 0 {
		landCb(changes)
	}
	if waterChanged && resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil && len(inventorySnap.Changes) > 0 {
		inventoryCb(inventorySnap)
	}
}

// Inventory returns a copy of the inventory map.
func (s *State) Inventory() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]int32, len(s.inventory))
	for k, v := range s.inventory {
		out[k] = v
	}
	return out
}

// FlowerInventory returns only the flower-seed slice of the inventory.
func (s *State) FlowerInventory() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]int32)
	for k, v := range s.inventory {
		if int(k) >= FlowerSeedLow && int(k) < FlowerSeedHigh && v > 0 {
			out[k] = v
		}
	}
	return out
}

// LeastInventoryFlower returns the (id, count) of the lowest-stock flower
// among allowed ids (or all flower seeds if allowed is empty). Returns
// id=0 if the inventory has no flower with positive stock.
func (s *State) LeastInventoryFlower(allowed []int32, blocked []int32) (int32, int32) {
	return s.leastInventoryFlower(allowed, blocked, false)
}

// LeastPlantableFlower returns the lowest-stock flower that is both in
// inventory and has completed cultivation. The server rejects planting flowers
// that still have no successful cultivation record in namespace 101.
func (s *State) LeastPlantableFlower(allowed []int32, blocked []int32) (int32, int32) {
	return s.leastInventoryFlower(allowed, blocked, true)
}

// PlantableFlowers returns cultivated flowers that the server should accept
// for planting, filtered by allow/block lists. Planting does not consume 230xx
// flower inventory, so a cultivated flower with zero stock is still plantable.
func (s *State) PlantableFlowers(allowed []int32, blocked []int32) []PlantableFlower {
	s.mu.RLock()
	defer s.mu.RUnlock()
	allowedSet := setOf(allowed)
	blockedSet := setOf(blocked)
	out := make([]PlantableFlower, 0)
	for id, cv := range s.cultivations {
		if !isPlantableCultivation(cv) || !isFlowerItemID(id) {
			continue
		}
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[id]; !ok {
				continue
			}
		}
		if _, ok := blockedSet[id]; ok {
			continue
		}
		info := catalog.Flowers[id]
		out = append(out, PlantableFlower{
			FlowerID:   id,
			Stock:      s.inventory[id],
			Gold:       info.Gold,
			Experience: info.Experience,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FlowerID < out[j].FlowerID })
	return out
}

func (s *State) leastInventoryFlower(allowed []int32, blocked []int32, requireCultivated bool) (int32, int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	allowedSet := setOf(allowed)
	blockedSet := setOf(blocked)
	type entry struct {
		id    int32
		count int32
	}
	var candidates []entry
	for id, count := range s.inventory {
		if int(id) < FlowerSeedLow || int(id) >= FlowerSeedHigh {
			continue
		}
		if count <= 0 {
			continue
		}
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[id]; !ok {
				continue
			}
		}
		if _, ok := blockedSet[id]; ok {
			continue
		}
		if requireCultivated && !isPlantableCultivation(s.cultivations[id]) {
			continue
		}
		candidates = append(candidates, entry{id, count})
	}
	if len(candidates) == 0 {
		return 0, 0
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count < candidates[j].count
		}
		return candidates[i].id < candidates[j].id
	})
	return candidates[0].id, candidates[0].count
}

func isPlantableCultivation(cv *CultivateView) bool {
	return cv != nil && cv.Status == 2 && cv.Lvl > 0
}

// RoleID returns the cached role id (`100.0.0`).
func (s *State) RoleID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.roleID
}

func (s *State) currentWaterDropsLocked() int32 {
	if s.hasWaterDropsItem {
		return s.inventory[7]
	}
	return 0
}

func waterDropRestoreIntervalMs() int64 {
	if item, ok := catalog.Items[7]; ok && len(item.Restore) > 0 && item.Restore[0].Count > 0 {
		return int64(item.Restore[0].Count)
	}
	return 120001
}

func (s *State) projectedWaterDropsLocked(now time.Time) (drops int32, nextMs int64, recovered int32) {
	current := s.currentWaterDropsLocked()
	next := s.waterDropsNextMs
	if !s.hasWaterDropsItem || next <= 0 {
		return current, next, 0
	}
	if s.waterDropsTotal > 0 && current >= s.waterDropsTotal {
		return current, 0, 0
	}
	nowMs := now.UnixMilli()
	if nowMs < next {
		return current, next, 0
	}
	interval := waterDropRestoreIntervalMs()
	if interval <= 0 {
		return current, next, 0
	}

	recover := int32((nowMs-next)/interval) + 1
	if s.waterDropsTotal > 0 {
		remaining := s.waterDropsTotal - current
		if remaining <= 0 {
			return current, 0, 0
		}
		if recover > remaining {
			recover = remaining
		}
	}
	current += recover
	next += int64(recover) * interval
	if s.waterDropsTotal > 0 && current >= s.waterDropsTotal {
		next = 0
	}
	return current, next, recover
}

func (s *State) resourceSnapshotLocked() ResourceSnapshot {
	return ResourceSnapshot{
		Gold: s.gold, WaterDrops: s.currentWaterDropsLocked(), WaterDropsTotal: s.waterDropsTotal, WaterDropsNextMs: s.waterDropsNextMs,
		Level: s.level, Experience: s.experience, Vip: s.vip, VipExp: s.vipExp, NobleEligible: s.nobleEligibleLocked(),
		DiamondsFree: s.diamondsFree, DiamondsPaid: s.diamondsPaid,
	}
}

// WaterDrops returns current water drops, total capacity, and the next recovery
// timestamp. Current drops come from inventory item 7 in either the cold
// snapshot (7.0.32["7"]) or absolute inventory deltas (7.2.2["7"]). Some
// cold snapshots omit item 7; in that case 7.1.13 is used only as a bounded
// fallback. 7.0.33.7.1 is the total/capacity, not the current value.
func (s *State) WaterDrops() (int32, int32, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentWaterDropsLocked(), s.waterDropsTotal, s.waterDropsNextMs
}

// AvailableWaterDrops returns the drops that automation may safely attempt to
// spend. It is conservative: when the next recovery timestamp has elapsed but
// the server has not pushed namespace 7 yet, advance the local recovery clock
// with the c_item.restore interval and cap at the server-reported total.
func (s *State) AvailableWaterDrops(now time.Time) (int32, int32, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, nextMs, _ := s.projectedWaterDropsLocked(now)
	if s.waterDropsTotal > 0 && current > s.waterDropsTotal {
		current = s.waterDropsTotal
	}
	current -= s.waterDropsInFlight
	if current < 0 {
		current = 0
	}
	return current, s.waterDropsTotal, nextMs
}

// LockWaterDrops marks drops as committed to an in-flight water RPC. This
// keeps concurrent planners from spending them again before the server response
// updates namespace 7.
func (s *State) LockWaterDrops(n int32, now time.Time) bool {
	if n <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, _, _ := s.projectedWaterDropsLocked(now)
	if s.waterDropsTotal > 0 && current > s.waterDropsTotal {
		current = s.waterDropsTotal
	}
	if current-s.waterDropsInFlight < n {
		return false
	}
	s.waterDropsInFlight += n
	return true
}

// ReleaseWaterDropsLock releases a previous in-flight lock after the RPC
// fails or after the response has been reconciled into state.
func (s *State) ReleaseWaterDropsLock(n int32) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n >= s.waterDropsInFlight {
		s.waterDropsInFlight = 0
		return
	}
	s.waterDropsInFlight -= n
}

// RefreshWaterDrops materializes elapsed natural water-drop recovery into
// local state and emits normal resource/inventory callbacks. The next server
// namespace 7 update remains authoritative and can correct the local clock.
func (s *State) RefreshWaterDrops(now time.Time) bool {
	s.mu.Lock()
	next, nextMs, recovered := s.projectedWaterDropsLocked(now)
	if recovered <= 0 {
		s.mu.Unlock()
		return false
	}
	prevInventory := cloneInt32Map(s.inventory)
	s.inventory[7] = next
	s.waterDropsNextMs = nextMs
	resourceSnap := s.resourceSnapshotLocked()
	resourceCb := s.onResourceChange
	var inventorySnap InventorySnapshot
	var inventoryCb func(InventorySnapshot)
	changes := inventoryChanges(prevInventory, s.inventory)
	if len(changes) > 0 {
		inventorySnap = InventorySnapshot{
			Inventory: cloneInt32Map(s.inventory),
			Changes:   changes,
		}
		inventoryCb = s.onInventoryChange
	}
	s.mu.Unlock()

	if resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil {
		inventoryCb(inventorySnap)
	}
	return true
}

// Gold returns the current gold balance (itemId 44).
func (s *State) Gold() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gold
}

// Level returns the current player level (7.0.34).
func (s *State) Level() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.level
}

// Experience returns the current experience points (7.0.35).
func (s *State) Experience() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.experience
}

// Diamonds returns free and paid diamond balances (7.0.41, 7.0.42).
func (s *State) Diamonds() (free int32, paid int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.diamondsFree, s.diamondsPaid
}

// Resources returns a snapshot of all tracked resource fields.
func (s *State) Resources() ResourceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resourceSnapshotLocked()
}

func (s *State) nobleEligibleLocked() bool {
	return s.vip > 0
}

// Vip returns the observed VIP level and experience from namespace 7.
func (s *State) Vip() (level int32, exp int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vip, s.vipExp
}

// NobleEligible reports whether the account has the observed client-side
// privilege gate needed for noble-only actions such as usrLand.waterOneKey.
func (s *State) NobleEligible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nobleEligibleLocked()
}

// ObservedNamespaces returns every v namespace key observed by this state.
func (s *State) ObservedNamespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.namespaceCounts))
	for k := range s.namespaceCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// UnknownNamespaceCount returns how many namespace keys have been observed
// without a typed state model.
func (s *State) UnknownNamespaceCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int32(len(s.unknownNSCounts))
}

// NamespaceCounts returns a copy of namespace observation counts.
func (s *State) NamespaceCounts() map[string]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int32, len(s.namespaceCounts))
	for k, v := range s.namespaceCounts {
		out[k] = v
	}
	return out
}

// WaterwheelClaimedCount returns the total number of waterwheel claims made.
func (s *State) WaterwheelClaimedCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wwClaimedCount
}

// WaterwheelReady is a compatibility accessor used by older diagnostics. It
// returns 1 when the local cooldown view says a claim can be attempted, else 0.
func (s *State) WaterwheelReady() int32 {
	if s.WaterwheelCooldownReady() {
		return 1
	}
	return 0
}

func waterwheelBucketCreateInterval() time.Duration {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return time.Hour
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return time.Hour
	}
	seconds := readInt32Any(row["$bucketCreateCd"])
	if seconds <= 0 {
		return time.Hour
	}
	return time.Duration(seconds) * time.Second
}

func waterwheelBucketDailyMax() int32 {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return 0
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return readInt32Any(row["$bucketGetMax"])
}

// WaterwheelCooldownReady returns true if the local bucket-generation clock
// says a waterwheel claim can be attempted. The client config uses
// c_waterwheel.$bucketCreateCd seconds between generated buckets.
func (s *State) WaterwheelCooldownReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if max := waterwheelBucketDailyMax(); max > 0 && s.wwClaimedCount >= max {
		return false
	}
	if s.wwLastRecvTs == 0 {
		return true
	}
	return time.Duration(time.Now().UnixMilli()-s.wwLastRecvTs)*time.Millisecond >= waterwheelBucketCreateInterval()
}

// MaxLandID returns the highest land ID currently tracked.
func (s *State) MaxLandID() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var max int32
	for id := range s.lands {
		if id > max {
			max = id
		}
	}
	return max
}

// FlowerOrders returns the current resident order requirements.
func (s *State) FlowerOrders() map[int32]*FlowerOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]*FlowerOrder, len(s.flowerOrders))
	for k, v := range s.flowerOrders {
		cp := *v
		cp.Requires = append([]FlowerRequire(nil), v.Requires...)
		out[k] = &cp
	}
	return out
}

// ResidentSatinOrder returns the latest observed satin resident order state.
func (s *State) ResidentSatinOrder() ResidentSpecialOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.residentSatinOrder
}

// ResidentDecorateOrder returns the latest observed decorate resident order state.
func (s *State) ResidentDecorateOrder() ResidentSpecialOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.residentDecorateOrder
}

// PalaceOrder returns the current palace order state.
func (s *State) PalaceOrder() PalaceOrderView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.palaceOrder
}

// TeamOrder returns the current team order state.
func (s *State) TeamOrder() TeamOrderView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.teamOrder
}

// Statistics returns the latest observed daily statistics snapshot.
func (s *State) Statistics() StatisticsView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statistics
}

// FlowerRackSlots returns the current flower-art shelf slots.
func (s *State) FlowerRackSlots() map[int32]FlowerRackSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]FlowerRackSlot, len(s.flowerRack))
	for k, v := range s.flowerRack {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// FlowerRackClaimableSlotIDs returns listed rack slots whose configured sale
// window has elapsed. The client treats a rack as sold when:
// now - sellStartTime >= num * c_flowerRack.$sellTime.
func (s *State) FlowerRackClaimableSlotIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nowMs := now.UnixMilli()
	out := make([]int32, 0)
	for rackID, slot := range s.flowerRack {
		if slot == nil || slot.ItemID <= 0 || slot.Count <= 0 || slot.SellReadyAtMs <= 0 || nowMs < slot.SellReadyAtMs {
			continue
		}
		out = append(out, rackID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// EmptyFlowerRackSlotIDs returns observed rack slots with no listed art.
func (s *State) EmptyFlowerRackSlotIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for rackID, slot := range s.flowerRack {
		if slot == nil || slot.ItemID != 0 || slot.Count != 0 {
			continue
		}
		out = append(out, rackID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// MailObserved reports whether namespace 19 has been observed at least once.
func (s *State) MailObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mailObserved
}

// Mails returns the currently tracked ordinary mail list.
func (s *State) Mails() []MailView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MailView, 0, len(s.mails))
	for _, mail := range s.mails {
		if mail == nil {
			continue
		}
		cp := *mail
		cp.ItemsRaw = append(json.RawMessage(nil), mail.ItemsRaw...)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MsID != out[j].MsID {
			return out[i].MsID < out[j].MsID
		}
		return out[i].AllID < out[j].AllID
	})
	return out
}

// ReadyMailPickTargets returns unpicked mail entries that carry rewards.
func (s *State) ReadyMailPickTargets() []MailPickTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MailPickTarget, 0)
	for _, mail := range s.mails {
		if mail == nil || mail.IsDel != 0 || mail.IsPick != 0 || !mailHasItems(mail.ItemsRaw) {
			continue
		}
		if mail.MsID <= 0 && mail.AllID <= 0 {
			continue
		}
		out = append(out, MailPickTarget{MsID: mail.MsID, AllID: mail.AllID})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MsID != out[j].MsID {
			return out[i].MsID < out[j].MsID
		}
		return out[i].AllID < out[j].AllID
	})
	return out
}

// Vases returns the currently observed unlocked vase set.
func (s *State) Vases() map[int32]VaseView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]VaseView, len(s.vases))
	for k, v := range s.vases {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// VaseObserved reports whether namespace 102 has been observed at least once.
func (s *State) VaseObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vaseObserved
}

// HasVase reports whether the account has the requested vase unlocked.
func (s *State) HasVase(vaseID int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.vases[vaseID]
	return ok
}

// FlowerArt returns the tracked namespace 106 flower-art state.
func (s *State) FlowerArt() FlowerArtView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.flowerArt
	out.MakeList = cloneInt32s(out.MakeList)
	out.MakeListRaw = cloneRaw(out.MakeListRaw)
	out.SRecvList = cloneInt32s(out.SRecvList)
	out.SRecvListRaw = cloneRaw(out.SRecvListRaw)
	return out
}

// FmlBuild returns the tracked namespace 25 guild-build state.
func (s *State) FmlBuild() FmlBuildView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.fmlBuild
	out.BuildCounts = cloneInt32Map(out.BuildCounts)
	return out
}

// FmlBuildObserved reports whether namespace 25 has been observed.
func (s *State) FmlBuildObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlBuild.Observed
}

// FmlLandObserved reports whether namespace 25.102 has been observed.
func (s *State) FmlLandObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlLandObserved
}

// FmlLands returns a defensive copy of observed guild lands.
func (s *State) FmlLands() map[int32]FmlLandView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]FmlLandView, len(s.fmlLands))
	for id, land := range s.fmlLands {
		if land == nil {
			continue
		}
		out[id] = *land
	}
	return out
}

// ReadyFmlLandHarvestIDs returns guild lands with unclaimed mature flowers.
func (s *State) ReadyFmlLandHarvestIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlLands))
	for id, land := range s.fmlLands {
		if land == nil || land.MatureFlowerCnt <= land.HarvestedCnt {
			continue
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// FmlForestEnergy returns the tracked forest-energy state.
func (s *State) FmlForestEnergy() FmlForestEnergyView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.fmlForestEnergy
	out.EnergyByType = cloneInt32Map(out.EnergyByType)
	out.DailyEnergyByType = cloneInt32Map(out.DailyEnergyByType)
	out.PendingTempEnergyByType = cloneInt32Map(out.PendingTempEnergyByType)
	return out
}

// FmlForestEnergyObserved reports whether namespace 25.127 has been observed.
func (s *State) FmlForestEnergyObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlForestEnergy.Observed
}

// ReadyFmlForestEnergyTypes returns energy types with pending temporary energy.
func (s *State) ReadyFmlForestEnergyTypes() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlForestEnergy.PendingTempEnergyByType))
	for typ, count := range s.fmlForestEnergy.PendingTempEnergyByType {
		if count > 0 {
			out = append(out, typ)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// FmlFlowerShareObserved reports whether namespace 25.107 has been observed.
func (s *State) FmlFlowerShareObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlFlowerShare.Observed
}

// FmlFlowerShare returns a defensive copy of the account's own guild share.
func (s *State) FmlFlowerShare() FmlFlowerShareView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneFmlFlowerShareView(s.fmlFlowerShare)
}

// OtherFmlFlowerSharesObserved reports whether namespace 25.108 has been observed.
func (s *State) OtherFmlFlowerSharesObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlOtherShareObserved
}

// OtherFmlFlowerShares returns defensive copies of member guild shares.
func (s *State) OtherFmlFlowerShares() map[int64]FmlFlowerShareView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64]FmlFlowerShareView, len(s.fmlOtherFlowerShares))
	for uid, share := range s.fmlOtherFlowerShares {
		if share == nil {
			continue
		}
		out[uid] = cloneFmlFlowerShareView(*share)
	}
	return out
}

// ReadyFmlFlowerShareRewardSlotIDs returns own share slots with take rewards.
func (s *State) ReadyFmlFlowerShareRewardSlotIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlFlowerShare.Slots))
	for slotID, slot := range s.fmlFlowerShare.Slots {
		if slot.FlowerID > 0 && slot.TakeNum > 0 {
			out = append(out, slotID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// FmlFlowerTakeCandidates returns member share slots that still have flowers.
func (s *State) FmlFlowerTakeCandidates() []FmlFlowerTakeCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FmlFlowerTakeCandidate, 0)
	for uid, share := range s.fmlOtherFlowerShares {
		if share == nil {
			continue
		}
		actualUID := share.UID
		if actualUID == 0 {
			actualUID = uid
		}
		if actualUID == 0 {
			continue
		}
		for slotID, slot := range share.Slots {
			available := slot.ShareNum - slot.TakeNum
			if slot.FlowerID <= 0 || available <= 0 {
				continue
			}
			out = append(out, FmlFlowerTakeCandidate{
				UID:       actualUID,
				SlotID:    slotID,
				FlowerID:  slot.FlowerID,
				ShareNum:  slot.ShareNum,
				TakeNum:   slot.TakeNum,
				Available: available,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FlowerID != out[j].FlowerID {
			return out[i].FlowerID < out[j].FlowerID
		}
		if out[i].UID != out[j].UID {
			return out[i].UID < out[j].UID
		}
		return out[i].SlotID < out[j].SlotID
	})
	return out
}

// CollectRewards returns the currently observed namespace 103 reward state.
func (s *State) CollectRewards() map[int32]CollectRewardView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]CollectRewardView, len(s.collectRewards))
	for k, v := range s.collectRewards {
		if v == nil {
			continue
		}
		cp := *v
		cp.RecvIDs = cloneInt32s(v.RecvIDs)
		cp.ArtCreateRewardIDs = cloneInt32s(v.ArtCreateRewardIDs)
		out[k] = cp
	}
	return out
}

// CollectRewardObserved reports whether namespace 103 has been observed.
func (s *State) CollectRewardObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collectRewardObserved
}

// ReadyCollectRewardTypes returns collectRwd.recv types that have at least
// one unclaimed c_flowerCollect reward at or below the observed exp.
func (s *State) ReadyCollectRewardTypes(types ...int32) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.collectRewards) == 0 {
		return nil
	}
	filter := setOf(types)
	out := make([]int32, 0, len(s.collectRewards))
	for typeID, reward := range s.collectRewards {
		if reward == nil {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[typeID]; !ok {
				continue
			}
		}
		if collectRewardReady(*reward) {
			out = append(out, typeID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyArtCreateRewardVaseIDs returns vase ids whose flower-art creation
// reward can be claimed through collectRwd.recvArtCreateRwdByVase.
func (s *State) ReadyArtCreateRewardVaseIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reward := s.collectRewards[13]
	if reward == nil || len(s.flowerArt.MakeList) == 0 {
		return nil
	}
	received := setOf(reward.ArtCreateRewardIDs)
	ready := map[int32]struct{}{}
	for _, artID := range s.flowerArt.MakeList {
		if artID <= 0 {
			continue
		}
		if _, ok := received[artID]; ok {
			continue
		}
		vaseID := (artID - 1) / 100
		if vaseID <= 0 {
			continue
		}
		ready[vaseID] = struct{}{}
	}
	out := make([]int32, 0, len(ready))
	for vaseID := range ready {
		out = append(out, vaseID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ShopCultivateObserved reports whether namespace 113 has been observed.
func (s *State) ShopCultivateObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shopCultivateObserved
}

// ShopCultivateOffers returns current material-shop offers enriched with the
// static item/limit metadata from c_shop_cultivate.
func (s *State) ShopCultivateOffers() []ShopCultivateOfferView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ShopCultivateOfferView, 0, len(s.shopCultivateCosts))
	for shopID, cost := range s.shopCultivateCosts {
		view := ShopCultivateOfferView{
			ShopID:     shopID,
			CostItemID: cost.ItemID,
			CostCount:  cost.Count,
			Bought:     s.shopCultivateBought[shopID],
		}
		if itemID, itemCount, buyLimit, sortOrder, ok := shopCultivateStatic(shopID); ok {
			view.ItemID = itemID
			view.ItemCount = itemCount
			view.BuyLimit = buyLimit
			view.Sort = sortOrder
		}
		if view.BuyLimit > 0 {
			view.Remaining = view.BuyLimit - view.Bought
			if view.Remaining < 0 {
				view.Remaining = 0
			}
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return out[i].ShopID < out[j].ShopID
	})
	return out
}

// ShopGiftbagObserved reports whether namespace 112 has been observed.
func (s *State) ShopGiftbagObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shopGiftbagObserved
}

// ShopGiftbagOffers returns static gift-bag shop rows enriched with observed
// daily/weekly/monthly/total purchase records.
func (s *State) ShopGiftbagOffers() []ShopGiftbagOfferView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	table, ok := StaticTableByName("c_shop_giftbag")
	if !ok {
		return nil
	}
	out := make([]ShopGiftbagOfferView, 0, len(table.Rows))
	for idStr, rawRow := range table.Rows {
		shopID := atoi32(idStr)
		if shopID <= 0 {
			continue
		}
		view, ok := shopGiftbagStatic(shopID, rawRow)
		if !ok {
			continue
		}
		view.DailyBought = s.shopGiftbagDRecord[shopID]
		view.WeekBought = s.shopGiftbagWRecord[shopID]
		view.MonthBought = s.shopGiftbagMRecord[shopID]
		view.TotalBought = s.shopGiftbagTRecord[shopID]
		view.Remaining = giftbagRemaining(view)
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return out[i].ShopID < out[j].ShopID
	})
	return out
}

// PearlObserved reports whether namespace 115 has been observed.
func (s *State) PearlObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pearlObserved
}

// Pearl returns the currently observed pearl summary state.
func (s *State) Pearl() PearlView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pearl
}

// PearlPlaces returns a defensive copy of observed pearl production slots.
func (s *State) PearlPlaces() map[int32]PearlPlaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]PearlPlaceView, len(s.pearlPlaces))
	for id, place := range s.pearlPlaces {
		if place == nil {
			continue
		}
		out[id] = *place
	}
	return out
}

// ReadyPearlPlaceIDs returns pearl slots with observed surplus to receive.
func (s *State) ReadyPearlPlaceIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.pearlPlaces))
	for id, place := range s.pearlPlaces {
		if place != nil && place.SurplusRecvNum > 0 {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// PearlDrawCount returns the number of pearl draw entries currently observed.
func (s *State) PearlDrawCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pearlDrawCount
}

// PearlDailyFreeReady reports whether the daily free pearl has not been
// observed as received for the local day represented by now.
func (s *State) PearlDailyFreeReady(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.pearlObserved || !s.pearl.Observed {
		return false
	}
	return !sameLocalDay(s.pearl.RecvDailyDate, now)
}

// ReadyFlowerOrderAdBoxIDs returns resident-order boxes that currently present
// the client as a video/share reward before a concrete order is generated.
func (s *State) ReadyFlowerOrderAdBoxIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for id, order := range s.flowerOrders {
		if order != nil && order.Mode == 8 && len(order.Requires) == 0 {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyFlowerOrderRewardTargets returns resident-order milestone rewards that
// are claimable from observed daily progress.
func (s *State) ReadyFlowerOrderRewardTargets() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var finished int32
	for _, task := range s.dailyTasks {
		if task != nil && task.TaskID == 30060001 && task.Finished > finished {
			finished = task.Finished
		}
	}
	if finished <= 0 {
		return nil
	}
	thresholds := []int32{15, 30, 45, 60}
	out := make([]int32, 0, len(thresholds))
	for idx, threshold := range thresholds {
		target := int32(idx + 1)
		if finished >= threshold && !s.flowerOrderRewardsReceived[target] {
			out = append(out, target)
		}
	}
	return out
}

// FlowerOrderDeficits returns flower ids whose long-lived requirements are not
// yet covered by current inventory. Customer orders are intentionally excluded:
// they should be completed from current stock/craft capacity or refreshed.
func (s *State) FlowerOrderDeficits() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	needed := make(map[int32]int32)
	addRequires := func(reqs []FlowerRequire) {
		for _, req := range reqs {
			if req.FlowerID == 0 || req.Count <= 0 {
				continue
			}
			needed[req.FlowerID] += req.Count
		}
	}
	for _, order := range s.flowerOrders {
		if order != nil {
			addRequires(order.Requires)
		}
	}
	if s.mainTask != nil {
		if flowerID, missing, ok := MainTaskFlowerRequirement(s.mainTask.TaskID, s.mainTask.Finished); ok {
			needed[flowerID] += missing
		}
	}
	out := make(map[int32]int32)
	for flowerID, count := range needed {
		if have := s.inventory[flowerID]; have < count {
			out[flowerID] = count - have
		}
	}
	return out
}

// Cultivations returns a copy of the cultivation state map.
func (s *State) Cultivations() map[int32]CultivateView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]CultivateView, len(s.cultivations))
	for k, v := range s.cultivations {
		out[k] = *v
	}
	return out
}

// CustomerOrders returns the set of active customer order npcIds.
func (s *State) CustomerOrders() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.customerOrders))
	for id := range s.customerOrders {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CustomerOrderSummary returns namespace 109 metadata.
func (s *State) CustomerOrderSummary() CustomerOrderSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary := s.customerOrderSummary
	summary.ActiveCount = int32(len(s.customerOrders))
	return summary
}

// CustomerOrderGenerationReady reports whether ordinary customer orders can be
// requested now based on the observed client cooldown.
func (s *State) CustomerOrderGenerationReady(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.customerOrderSummary.Observed || len(s.customerOrders) > 0 {
		return false
	}
	next := s.customerOrderSummary.NextGenTimeMs
	return next <= 0 || now.UnixMilli() >= next+1000
}

// CustomerOrderDetails returns the active customer order requirements.
func (s *State) CustomerOrderDetails() map[int32]*CustomerOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]*CustomerOrder, len(s.customerOrders))
	for k, v := range s.customerOrders {
		if v == nil {
			continue
		}
		cp := *v
		cp.Requires = append([]FlowerRequire(nil), v.Requires...)
		cp.ItemRequires = append([]ItemRequire(nil), v.ItemRequires...)
		out[k] = &cp
	}
	return out
}

// MainTask returns the current main task progress when namespace 22.0 has
// been observed.
func (s *State) MainTask() (MainTaskView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainTask == nil {
		return MainTaskView{}, false
	}
	return *s.mainTask, true
}

// ReadyDailyTaskIDs returns daily task ids that look claimable from namespace
// 22. A status of 1 is treated as the client's explicit "ready" marker; when
// status is absent, completed target progress with no receipt is accepted.
func (s *State) ReadyDailyTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.dailyTasks))
	for id, task := range s.dailyTasks {
		if task != nil && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyWeeklyTaskIDs returns weekly task ids that look claimable from namespace
// 22.100 and the current c_task_week table.
func (s *State) ReadyWeeklyTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.weeklyTasks))
	for id, task := range s.weeklyTasks {
		if task != nil && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyRoadGrowTaskIDs returns growth-road rewards that can be claimed from
// the observed player state and client task table.
func (s *State) ReadyRoadGrowTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := RoadGrowLevelTasks()
	out := make([]int32, 0, len(tasks))
	for _, task := range tasks {
		if task.TaskID == 0 || s.roadGrowReceived[task.TaskID] {
			continue
		}
		if task.TargetLevel > 0 && s.level >= task.TargetLevel {
			out = append(out, task.TaskID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RoadGrowReceived returns a copy of the growth-road receipt map.
func (s *State) RoadGrowReceived() map[int32]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]bool, len(s.roadGrowReceived))
	for id, v := range s.roadGrowReceived {
		out[id] = v
	}
	return out
}

// ReadyRandomEventIDs returns map random events whose status is actionable.
func (s *State) ReadyRandomEventIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.randomEvents))
	for id, event := range s.randomEvents {
		if event != nil && (event.Status == 0 || event.Status == 1) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RandomEvents returns the current map-random-event state.
func (s *State) RandomEvents() map[int32]RandomEventView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]RandomEventView, len(s.randomEvents))
	for id, event := range s.randomEvents {
		if event != nil {
			out[id] = *event
		}
	}
	return out
}

// DailyTasks returns a copy of tracked daily task progress.
func (s *State) DailyTasks() map[int32]DailyTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]DailyTaskView, len(s.dailyTasks))
	for id, task := range s.dailyTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

// WeeklyTasks returns a copy of tracked weekly task progress.
func (s *State) WeeklyTasks() map[int32]WeeklyTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]WeeklyTaskView, len(s.weeklyTasks))
	for id, task := range s.weeklyTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

// NextFreeWaterIndex returns the next candidate idx for freeWater.recv.
// The static client schema exposes IFreeWater.recvIdx and the RPC argument
// is also named idx, so use the observed index directly and let the server
// response advance it.
func (s *State) NextFreeWaterIndex() (int32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.freeWaterObserved {
		return 0, false
	}
	return s.freeWaterRecvIdx, true
}

func setOf(ids []int32) map[int32]struct{} {
	if len(ids) == 0 {
		return nil
	}
	m := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func taskClaimable(status, target, finished, receipted int32) bool {
	if receipted != 0 {
		return false
	}
	return status == 1 || (status == 0 && target > 0 && finished >= target)
}

func collectRewardReady(view CollectRewardView) bool {
	if view.Type <= 0 || view.Exp <= 0 {
		return false
	}
	table, ok := StaticTableByName("c_flowerCollect")
	if !ok {
		return false
	}
	received := setOf(view.RecvIDs)
	for idStr, rawRow := range table.Rows {
		rowID := atoi32(idStr)
		if rowID <= 0 || rowID/10000 != view.Type {
			continue
		}
		if _, ok := received[rowID]; ok {
			continue
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(rawRow, &row); err != nil {
			continue
		}
		exp, ok := readInt32JSONField(row, "exp")
		if !ok || exp <= 0 {
			continue
		}
		if view.Exp >= exp {
			return true
		}
	}
	return false
}

func shopCultivateStatic(shopID int32) (itemID, itemCount, buyLimit, sortOrder int32, ok bool) {
	rawRow, ok := StaticRow("c_shop_cultivate", shopID)
	if !ok {
		return 0, 0, 0, 0, false
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return 0, 0, 0, 0, false
	}
	if rawItems, ok := row["items"]; ok {
		var stacks []json.RawMessage
		if err := json.Unmarshal(rawItems, &stacks); err == nil && len(stacks) > 0 {
			parts := readInt32OrderedListRaw(stacks[0])
			if len(parts) >= 2 {
				itemID = parts[0]
				itemCount = parts[1]
			}
		}
	}
	if rawLimit, ok := row["bLimit"]; ok {
		parts := readInt32OrderedListRaw(rawLimit)
		if len(parts) > 0 {
			buyLimit = parts[0]
		}
	}
	if n, ok := readInt32JSONField(row, "sort"); ok {
		sortOrder = n
	}
	return itemID, itemCount, buyLimit, sortOrder, itemID > 0
}

func shopGiftbagStatic(shopID int32, rawRow json.RawMessage) (ShopGiftbagOfferView, bool) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return ShopGiftbagOfferView{}, false
	}
	view := ShopGiftbagOfferView{ShopID: shopID}
	if n, ok := readInt32JSONField(row, "type"); ok {
		view.Type = n
	}
	if n, ok := readInt32JSONField(row, "shareId"); ok {
		view.ShareID = n
	}
	if n, ok := readInt32JSONField(row, "rchgId"); ok {
		view.RchgID = n
	}
	if n, ok := readInt32JSONField(row, "moneyId"); ok {
		view.MoneyID = n
	}
	if n, ok := readInt32JSONField(row, "price"); ok {
		view.Price = n
	}
	if n, ok := readInt32JSONField(row, "priceMax"); ok {
		view.PriceMax = n
	}
	if n, ok := readInt32JSONField(row, "sort"); ok {
		view.Sort = n
	}
	view.DailyLimit = firstInt32ListValue(row["dLimit"])
	view.WeeklyLimit = firstInt32ListValue(row["wLimit"])
	view.MonthLimit = firstInt32ListValue(row["mLimit"])
	view.TotalLimit = firstInt32ListValue(row["tLimit"])
	if rawItems, ok := row["items"]; ok {
		view.Rewards = readItemCountsRaw(rawItems)
	}
	return view, true
}

func firstInt32ListValue(raw json.RawMessage) int32 {
	parts := readInt32OrderedListRaw(raw)
	if len(parts) == 0 {
		return 0
	}
	return parts[0]
}

func giftbagRemaining(view ShopGiftbagOfferView) int32 {
	remaining := int32(0)
	applyLimit := func(limit, bought int32) {
		if limit <= 0 {
			return
		}
		left := limit - bought
		if left < 0 {
			left = 0
		}
		if remaining == 0 || left < remaining {
			remaining = left
		}
	}
	applyLimit(view.DailyLimit, view.DailyBought)
	applyLimit(view.WeeklyLimit, view.WeekBought)
	applyLimit(view.MonthLimit, view.MonthBought)
	applyLimit(view.TotalLimit, view.TotalBought)
	return remaining
}

func readItemCountsRaw(raw json.RawMessage) []ItemCount {
	var stacks []json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &stacks) != nil {
		return nil
	}
	out := make([]ItemCount, 0, len(stacks))
	for _, rawStack := range stacks {
		parts := readInt32OrderedListRaw(rawStack)
		if len(parts) < 2 || parts[0] <= 0 || parts[1] <= 0 {
			continue
		}
		out = append(out, ItemCount{ItemID: parts[0], Count: parts[1]})
	}
	return out
}

func readInt32OrderedListRaw(raw json.RawMessage) []int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]int32, 0, len(arr))
		for _, rawValue := range arr {
			if n, ok := readInt32Raw(rawValue); ok {
				out = append(out, n)
			}
		}
		return out
	}
	if n, ok := readInt32Raw(raw); ok {
		return []int32{n}
	}
	return nil
}

func rawCollectionCount(raw json.RawMessage) int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return int32(len(arr))
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		return int32(len(m))
	}
	if n, ok := readInt32Raw(raw); ok && n > 0 {
		return n
	}
	return 0
}

func readInt32ListRaw(raw json.RawMessage) []int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]int32, 0, len(arr))
		for _, rawValue := range arr {
			if n, ok := readInt32Raw(rawValue); ok {
				out = append(out, n)
			}
		}
		return uniqueSortedInt32s(out)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		out := make([]int32, 0, len(m))
		if denseIndexMap(m) {
			for i := 0; i < len(m); i++ {
				if n, ok := readInt32Raw(m[itoaState(i)]); ok {
					out = append(out, n)
				}
			}
		} else {
			for key, rawValue := range m {
				id := atoi32(key)
				if id == 0 || !truthyRaw(rawValue) {
					continue
				}
				out = append(out, id)
			}
		}
		return uniqueSortedInt32s(out)
	}
	if n, ok := readInt32Raw(raw); ok {
		return []int32{n}
	}
	return nil
}

func readInt32JSONField(fields map[string]json.RawMessage, keys ...string) (int32, bool) {
	for _, key := range keys {
		if raw, ok := fields[key]; ok {
			return readInt32Raw(raw)
		}
	}
	return 0, false
}

func readInt64JSONField(fields map[string]json.RawMessage, keys ...string) (int64, bool) {
	for _, key := range keys {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		var n int64
		if err := json.Unmarshal(raw, &n); err == nil {
			return n, true
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err == nil {
			return int64(f), true
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			n := atoi64(s)
			if n != 0 || s == "0" {
				return n, true
			}
		}
	}
	return 0, false
}

func readInt32Raw(raw json.RawMessage) (int32, bool) {
	var n int32
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int32(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n := atoi32(s)
		if n != 0 || s == "0" {
			return n, true
		}
	}
	return 0, false
}

func sameLocalDay(rawDate int64, now time.Time) bool {
	if rawDate <= 0 {
		return false
	}
	if rawDate >= 19000101 && rawDate <= 29991231 {
		return now.Format("20060102") == itoa64State(rawDate)
	}
	var t time.Time
	if rawDate > 1_000_000_000_000 {
		t = time.UnixMilli(rawDate)
	} else {
		t = time.Unix(rawDate, 0)
	}
	y1, m1, d1 := t.Local().Date()
	y2, m2, d2 := now.Local().Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func denseIndexMap(m map[string]json.RawMessage) bool {
	if len(m) == 0 {
		return false
	}
	for i := 0; i < len(m); i++ {
		if _, ok := m[itoaState(i)]; !ok {
			return false
		}
	}
	return true
}

func truthyRaw(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	if n, ok := readInt32Raw(raw); ok {
		return n != 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s != "" && s != "0" && s != "false"
	}
	return true
}

func uniqueSortedInt32s(in []int32) []int32 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int32]struct{}, len(in))
	out := make([]int32, 0, len(in))
	for _, id := range in {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func itoaState(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func itoa64State(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func readInt32RawMap(raw json.RawMessage) map[int32]int32 {
	out := map[int32]int32{}
	if len(raw) == 0 {
		return out
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return out
	}
	for key, rawValue := range values {
		id := atoi32(key)
		if id == 0 {
			continue
		}
		if n, ok := readInt32Raw(rawValue); ok {
			out[id] = n
		}
	}
	return out
}

func readNestedInt32RawMapTotals(raw json.RawMessage) (map[int32]int32, int32) {
	out := map[int32]int32{}
	if len(raw) == 0 || string(raw) == "null" {
		return out, 0
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return out, 0
	}
	var total int32
	for _, rawInner := range outer {
		inner := readInt32RawMap(rawInner)
		for typ, count := range inner {
			if count <= 0 {
				continue
			}
			out[typ] += count
			total += count
		}
	}
	return out, total
}

func readInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if i := readInt32Any(v); i != 0 {
				return int(i)
			}
		}
	}
	return 0
}

func readInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int:
				return int64(x)
			case int64:
				return x
			case json.Number:
				i, _ := x.Int64()
				return i
			}
		}
	}
	return 0
}

func readInt32Any(v any) int32 {
	switch x := v.(type) {
	case float64:
		return int32(x)
	case int:
		return int32(x)
	case int32:
		return x
	case int64:
		return int32(x)
	case json.Number:
		i, _ := x.Int64()
		return int32(i)
	}
	return 0
}

func atoi32(s string) int32 {
	var n int32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int32(c-'0')
	}
	return n
}

func atoi64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
