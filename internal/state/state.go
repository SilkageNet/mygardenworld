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
	"sync"
)

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
	diamondsFree       int32           // 7.0.41 游戏顶部显示/可用于门槛的元宝
	diamondsPaid       int32           // 7.0.42 observed secondary diamond balance; do not add to visible 元宝
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
	wwCTimeMs      int64 // 114.5 cTime; bucket generation anchor
	wwObserved     bool
	wwEntered      bool
	wwAdvList      []int32
	wwLocalGenMs   int64 // local zero-bucket generation anchor, mirroring BucketMgr's client timer
	wwBackoffUntil int64 // local guard after the server says the bucket state is invalid

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
	residentOrderLimitUntilMs  int64
	residentOrderLimitDayID    int32
	residentSatinOrder         ResidentSpecialOrder
	residentDecorateOrder      ResidentSpecialOrder
	palaceOrder                PalaceOrderView // 108.0 宫廷订单
	teamOrder                  TeamOrderView   // 107.0 组团订单

	mainTask         *MainTaskView                  // 22.0 当前主线任务
	dailyTasks       map[int32]*DailyTaskView       // 22.1.100.<taskId>
	weeklyTasks      map[int32]*WeeklyTaskView      // 22.100 + c_task_week
	achievementTasks map[int32]*AchievementTaskView // 22.2 + c_task_ach
	storyMain        StoryMainView                  // 7.101 当前主线剧情

	roadGrowReceived    map[int32]bool             // 119.3.<taskId> 成长之路已领取
	randomEvents        map[int32]*RandomEventView // 129.0.1.<eventId> 地图随机事件
	randomEventObserved bool                       // namespace 129 observed at least once

	freeWaterObserved bool    // namespace 117 has been observed at least once
	freeWaterRecvIdx  []int32 // 117.1 client recvIdx list: free-water slots already claimed today
	freeWaterResetMs  int64   // 117.2 reset timestamp

	benefitBoxDrawCnt    int32 // 116.0.1 drawCnt
	benefitBoxResetCntMs int64 // 116.0.2 resetCntTime
	benefitBoxUTimeMs    int64 // 116.0.3 uTime
	benefitBoxObserved   bool  // namespace 116 has been observed at least once
	usrExtra             UsrExtraView
	reputation           ReputationView
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
		achievementTasks:           make(map[int32]*AchievementTaskView),
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
