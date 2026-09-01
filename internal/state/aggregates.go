package state

import "encoding/json"

// The account state remains one revisioned aggregate so a planner observes a
// coherent generation. Anonymous domain components make ownership explicit
// without exposing partial-lock snapshots or duplicating protocol data.

type protocolState struct {
	rawNamespaces   map[string]json.RawMessage
	namespaceCounts map[string]int32
	unknownNSCounts map[string]int32
	lastApplyMs     int64
}

type gardenState struct {
	lands              map[int32]LandView
	landRosterObserved bool
	farmLands          map[int32]FarmLandInfo
	farmLandObserved   bool
	cultivations       map[int32]*CultivateView
	hasWaterDropsItem  bool
	waterDropsTotal    int32
	waterDropsNextMs   int64
	waterDropsInFlight int32
	wwClaimedCount     int32
	wwLastRecvTs       int64
	wwCTimeMs          int64
	wwObserved         bool
	wwEntered          bool
	wwAdvList          []int32
	wwLocalGenMs       int64
	wwBackoffUntil     int64
	wwLastCountMs      int64
	freeWaterObserved  bool
	freeWaterRecvIdx   []int32
	freeWaterResetMs   int64
}

type resourceState struct {
	inventory       map[int32]int32
	gold            int32
	level           int32
	experience      int32
	vip             int32
	vipExp          int32
	diamondsFree    int32
	diamondsPaid    int32
	roleID          int64
	usrExtra        UsrExtraView
	reputation      ReputationView
	videoDouble     VideoDoubleView
	statistics      StatisticsView
	statisticsByDay map[int32]StatisticsView
}

type orderState struct {
	customerOrderSummary            CustomerOrderSummary
	customerOrders                  map[int32]*CustomerOrder
	flowerRack                      map[int32]*FlowerRackSlot
	vases                           map[int32]*VaseView
	vaseObserved                    bool
	collectRewards                  map[int32]*CollectRewardView
	collectRewardObserved           bool
	flowerArt                       FlowerArtView
	flowerOrders                    map[int32]*FlowerOrder
	flowerOrderRewardsReceived      map[int32]bool
	residentOrderLimitUntilMs       int64
	residentOrderLimitDayID         int32
	residentSatinLimitUntilMs       int64
	residentDecorateLimitUntilMs    int64
	residentOrderFinishBias         int32
	residentOrderFinishBiasDayID    int32
	residentSatinFinishBias         int32
	residentSatinFinishBiasDayID    int32
	residentDecorateFinishBias      int32
	residentDecorateFinishBiasDayID int32
	customerOrderFinishBias         int32
	customerOrderFinishBiasDayID    int32
	residentSatinOrder              ResidentSpecialOrder
	residentDecorateOrder           ResidentSpecialOrder
	palaceOrder                     PalaceOrderView
	teamOrder                       TeamOrderView
}

type unionState struct {
	fmlBuild                    FmlBuildView
	fmlLandObserved             bool
	fmlLands                    map[int32]*FmlLandView
	fmlForestEnergy             FmlForestEnergyView
	fmlForestRefreshAttemptAtMs int64
	fmlFlowerShare              FmlFlowerShareView
	fmlOtherFlowerShares        map[int64]*FmlFlowerShareView
	fmlOtherShareObserved       bool
	fmlOtherShareSyncedAtMs     int64
	fmlFlowerTakeLimitUntilMs   int64
	fmlRace                     FmlRaceView
}

type activityState struct {
	activityObserved    bool
	activityBatches     map[int32]*activityBatchState
	activityTemplates   map[int32]*activityTemplateState
	activityTaskRecords map[string]*activityTaskRecordState
}

type taskState struct {
	mainTask             *MainTaskView
	mainTaskReceipts     map[int32]int32
	mainTaskRecvObserved bool
	dailyTasks           map[int32]*DailyTaskView
	weeklyTasks          map[int32]*WeeklyTaskView
	achievementTasks     map[int32]*AchievementTaskView
	storyMain            StoryMainView
	randomEvents         map[int32]*RandomEventView
	randomEventObserved  bool
	randomEventMapValid  bool
	randomEventMapError  string
}

type socialState struct {
	frdStealObserved       bool
	frdStealRTimeMs        int64
	frdStealMap            map[int64]int32
	frdStealCntBuyObserved bool
	frdStealCntBuyRTimeMs  int64
	frdStealCntBuyMap      map[int64]int32
	frdOtherInfo           map[int64]FriendOtherInfoView
	frdOtherInfoObserved   bool
	frdVisitUID            int64
	frdVisitAtMs           int64
	frdVisitLands          map[int32]LandView
	frdStealSkipEnterUntil map[int64]int64
}

type assetState struct {
	mailObserved             bool
	mails                    map[string]*MailView
	shops                    map[int32]*shopState
	shopGiftbagDRecord       map[int32]int32
	shopGiftbagWRecord       map[int32]int32
	shopGiftbagMRecord       map[int32]int32
	shopGiftbagTRecord       map[int32]int32
	shopGiftbagBuyTimeRecord map[int32]int64
	shopGiftbagResetMs       int64
	shopGiftbagUpdatedAtMs   int64
	shopGiftbagCreatedAtMs   int64
	shopGiftbagObserved      bool
	shareUsages              map[int32]ShareUsageView
	shareTotObserved         bool
	shopCultivateCosts       map[int32]ItemCount
	shopCultivateBought      map[int32]int32
	shopCultivateResetMs     int64
	shopCultivateLarMs       int64
	shopCultivateMrCount     int32
	shopCultivateObserved    bool
	pearl                    PearlView
	pearlPlaces              map[int32]*PearlPlaceView
	pearlDrawCount           int32
	pearlDrawRaw             json.RawMessage
	pearlObserved            bool
	pearlFriendRelations     map[string]pearlFriendRelation
	pearlFriendOrder         []string
	pearlFriendsObserved     bool
	pearlProfiles            map[int64]*PearlCandidateProfile
	pearlHireStates          map[int64]*PearlCandidateHireState
	pearlRecommendUIDs       []int64
	pearlRecommendAtMs       int64
	pearlRecommendObserved   bool
	pearlEnemies             map[int64]int64
	pearlEnemiesObserved     bool
	pearlHireFailedUntil     map[int64]int64
	pearlHireSessionLocked   bool
	pearlHireLockReason      string
	pearlHireTicketUsedDayID int32
	pearlHireTicketUsedToday int32
	roadGrowReceived         map[int32]bool
	signTypes                map[int32]*SignTypeView
	signTypeObserved         bool
	signTypeMapValid         bool
	baseRewards              map[int32]*BaseRewardView
	baseRewardObserved       bool
	baseRewardMapValid       bool
	signTypeEnterAtMs        map[int32]int64
	benefitBoxDrawCnt        int32
	benefitBoxResetCntMs     int64
	benefitBoxUTimeMs        int64
	benefitBoxObserved       bool
	zoo                      ZooView
	zooPets                  map[int32]*ZooPetView
	zooLogs                  map[string]*ZooLogView
	zooSouvenirs             map[int32]*ZooSouvenirView
	zooDecorates             map[int32]*ZooDecorateView
	zooDecorateSuits         map[int32]*ZooDecorateSuitView
	zooLogsObserved          bool
	zooLogsInvalidReason     string
	zooSouvenirsObserved     bool
	zooDecoratesObserved     bool
	zooDecorateSuitsObserved bool
	zooObserved              bool
}

type hooksState struct {
	onChange          func(changed []LandChange)
	onResourceChange  func(ResourceSnapshot)
	onInventoryChange func(InventorySnapshot)
}
