// Package babigame contains the protocol adapter used by the local automation
// runner. It speaks the HTTP login chain, the encoded gateway envelope, and a
// long-lived WebSocket session.
//
// Protocol notes live in this package doc. Wire captures and static
// extraction artifacts should be kept outside tracked source files, and should
// be collected only from accounts and clients you are authorized to inspect.
//
// # Protocol Overview
//
// The adapter uses three transport layers:
//
//   - HTTPS REST login chain
//   - HTTPS gateway calls
//   - WebSocket event/RPC stream
//
// # Namespace Reference
//
// Server responses carry a "v" field containing namespace-keyed data:
//
//	Namespace  | Content                          | Updated by
//	-----------|----------------------------------|----------------------------------
//	2          | Server config / timezone         | reLogin
//	3          | Generic response extension        | selected RPC responses
//	6          | Config versions / split map      | reLogin
//	7          | Inventory (see below)            | Most RPCs
//	16         | VIP state                        | reLogin
//	20         | Shop purchase records            | shop.buy
//	22         | Daily/weekly task progress       | plant/water/harvest/cultivate
//	23         | Activity state / cyclic tasks    | normal actions, actCyclicNote.*
//	24         | Friend list                      | frd.enter
//	25         | Guild / union state              | fml.*, fml.bld
//	27         | IM channel                       | im.getChannelId
//	28         | Opponent/player summaries         | oppt.getDetailOppts
//	31         | Share state                      | usr.share
//	33         | Zoo state                         | zoo.*
//	100        | Land state (see below)           | plant/water/harvest/unlock
//	101        | Cultivation state (see below)    | cultivate.*
//	103        | Collection rewards               | cultivate.recv, collectRwd.recv
//	104        | Flower-art rack/shelves          | flowerRack.sell
//	105        | Flower orders                    | orderFlower.finishOrder / finishSatinOrder / finishDecorateOrder
//	106        | Flower art / share state          | flowerArt.makeFlowerArt, usr.share
//	109        | Customer orders                  | orderCustomer.*
//	110        | Friend availability/status       | frdExt.getFrdOtherInfoByUids
//	111        | Friend flower-pick state/land    | frdSteal.*
//	112        | Gift bag shop                    | shopGiftbag.enter/buy
//	113        | Cultivation material shop        | shopCultivate.enter/refresh/buy
//	114        | Waterwheel state (see below)     | waterwheel.*
//	115        | Pearl state                       | pearl.*, pearlPlace.*
//	116        | Benefit-box state (see below)    | benefitBox.draw
//	117        | Free-water state (see below)     | freeWater.recv
//	118        | Double-coin video timer           | usr.share(shareId=11) after SDK video
//	119        | High-freq task counters          | Most RPCs
//	124        | Daily summary / popup rewards    | harvest, orders
//	130        | Cultivation & art rewards        | cultivate.recv
//	131        | Observed high-frequency delta     | land, task, pass, random-event RPCs
//	132        | Observed order/pass delta          | orderFlower, flowerElvesPass
//	140        | Stateful anti-fraud daily reward   | signType.enter/sign/recv
//	148        | Observed broad activity delta      | Most reward/activity RPCs
//	165        | Celebrity state (legacy)          | older celebrity responses
//	166        | Celebrity state (authoritative)   | celebrity.getAllTypesInfo
//
// When a delta contains both celebrity namespaces, apply legacy namespace 165
// first and authoritative namespace 166 second.
//
// # Inventory (Namespace 7)
//
// Namespace 7 is structured as 7.0.<field>:
//
//	7.0.32     map[itemId]count   Flower seeds (23000-23999) + materials; item 7 is current water drops when present
//	7.1.13     int                Cold-login current water-drop fallback when 7.0.32 omits item 7
//	7.2.0      map[itemId]delta   Inventory item deltas after reward/spend RPCs
//	7.2.2      map[itemId]count   Absolute post-RPC inventory counts; item 7 is current water drops
//	7.0.33.7.1 int                Water drops total/capacity
//	7.0.33.7.5 int64              Water drops next recovery timestamp (ms)
//	7.0.34     int                Player level
//	7.0.35     int                Experience points
//	7.0.41     int                元宝 shown by the game top bar / spendable gate balance
//	7.0.42     int                Secondary diamond balance; tracked separately, not added to 7.0.41
//	7.0.44     int                Gold coins
//	7.7.2      IRwd               Anti-fraud one-time base reward gate (type=2)
//	7.13.1.104 int                usrExtra.antiFraudQAStatus; 2 means the QA reward is claimed
//	7.13.1.105 int64              usrExtra.lastAntiFraudQATime timestamp (ms)
//	7.17.0.1   int                Own reputation/礼仪分 score; active refresh uses reputation.view
//	7.101.1    int                Main-story chapter containing the next section to unlock
//	7.101.2    int                Zero-based next-section index; decoded terminal is 165:0
//	7.4.111.2  int                Daily story stars obtained (display statistic, never an unlock gate)
//
// # Anti-fraud daily reward (Namespace 140)
//
// signType type=1 is c_open.sign_1 ("防诈骗签到"), not the ordinary monthly
// sign.sign flow. Namespace 140.0.1 follows the observed server state machine
// status 0 -- sign --> status 1 -- recv --> status 2. The server returns code
// 3500 ("条件已达成，无需重复操作") when sign is repeated after the condition
// has already advanced. Automation must therefore merge sparse namespace 140
// deltas and select exactly one stage from authoritative status.
//
// # Land State (Namespace 100)
//
// Land data lives under namespace 100:
//
//	100.0.1.<landId>   Full opened/owned land roster (reLogin cold-start)
//	100.1.<landId>     Primary state delta (after plant/water/harvest)
//	100.2.<landId>     Harvest reward data
//
// `100.0.1` is authoritative for lands the player already owns. The client
// decides whether an absent land is unlockable from the current package's
// runtime `c_farmLand.openLvl` plus the player's level, then uses that same
// `c_farmLand.cost` when calling unlockLand. Do not infer candidates from
// land-id ordering or stale embedded tables. `usrLand.unlockLand` responses add
// the newly opened land through namespace 100.
//
// # Pearl Hire Candidate State
//
// Automatic labor hiring uses only observed ticket-gated state:
//
//	24.1[]       friend relations; a full 24.0+24.1 replaces, relation-only deltas merge
//	28.5[]       opponent summaries keyed by exact int64 UID, including level
//	115.1.5      enemy UID -> event timestamp (incremental map; null entry deletes)
//	115.5        candidate UID -> last/current labor end timestamp (incremental subset)
//	115.6[]      recommendation UID list (whole-list replacement)
//
// Candidate summaries and hire states are trusted for 30 seconds. A contested
// UID is cooled for 60 seconds. `pearlPlace.hire` must carry the exact observed
// item 1003 x1 cost gate. Only this RPC inspects namespace `3.0` as the
// client-side `$ext.iv`: exact zero is safe; nonzero or malformed-present data
// locks automatic hiring for the rest of the session.
//
// # Friend Flower Pick State (Namespaces 24, 110, 111)
//
// Automatic friend flower picking is driven only by observed state:
//
//	24.0.9        relation reset timestamp used for the daily purchase map
//	24.0.104      friend UID -> extra attempts already bought today
//	24.1[]        authoritative friend relationships
//	110.1.<uid>.0 isSteal; nonzero means the friend currently exposes a pick opportunity
//	111.0.1       friend UID -> attempts already used today
//	111.0.3       daily reset timestamp for the used-attempt map
//	111.1.0       currently visited friend UID
//	111.1.1       visited friend's land map
//	111.2         sparse visited-land changes
//
// c_frd currently declares $stealMax=10, $pickMax=10, and $pickAddCost=1;
// item 1305 is the friendship coin used by frdExt.buyStealCnt. The runner
// validates friendship membership, current-day used/bought maps, recent
// isSteal state, inventory cost, and the exact land again immediately before
// mutation. Unknown or stale state is a hard stop. The observed request field
// stealElves exists, but flower-elf availability and success deltas are not yet
// live-verified, so automation always sends stealElves=0.
//
// # Cyclic Note Activity (Namespace 23)
//
// 花笺集芳 is discovered from live batch/template state rather than a fixed
// batch id. The current activity has tmpType 4002 and status 1; client phase
// selection prefers active (2), then reward grace (3), then preview (1):
//
//	23.0.<batchId>             IActInfo: tmpId/type/status/times/score/bag/boxes
//	23.0.<batchId>.14.105.0    taskList, authoritative replacement by slot position
//	23.0.<batchId>.14.105.1    finishCnt
//	23.0.<batchId>.14.105.2    lrTime
//	23.1.<tmpId>               IActTmp, including field 9 score milestones
//	23.3."<batchId>|0".3       authoritative task progress map
//	23.3."<batchId>|0".5       authoritative task reward receipt map
//
// Top-level batch/template/task-record maps are entry-sparse; null deletes one
// entry. Object fields merge sparsely, while taskList, progress, receipt, bag,
// claimed-box, and template-box fields replace their prior complete value when
// present. Task completion uses raw server progress; UI progress is clamped.
// Slot unlock, paid reroll/direct-complete, gifts, and the activity shop are
// deliberately outside the safe automatic surface.
//
// # Dessert Activity (Namespace 23 ext121)
//
// 香卉甜糕 uses tmpType 5601. The dynamically selected batch stores activity
// energy/currency/reward boxes in its bag (items 1342/1343/1347), total drops
// in field 11, claimed milestones in field 13, and game state in
// 23.0.<batchId>.14.121. Field 121.1 is an authoritative five-mode map; every
// mode contains numeric schema fields 0..9, while actDessert.gameSync requests
// must send the corresponding named saveData keys (step, itemUse, map,
// gameStatus, firstMerge, isRunning, totalGain, curId, score, lvMap).
//
// The official client permits game input only in phase 2. A live phase-3
// observation on 2026-07-12 confirmed that actDessert.gameStart completes
// without starting mode 1, while actDessert.gameOver on an idle mode performs
// end-of-event settlement: all remaining item 1342 energy was converted to
// item 1343 currency at 1:2. Therefore gameOver is not a generic cleanup RPC.
// Cleanup may call it only after an enter refresh proves that the exact mode
// was running and was created/owned by the current test or automation session.
//
// The mutating live test TestDessertModeOneLifecycleE2E is additionally gated
// by E2E_DESSERT_LIVE=1. During phase 2 it uses a three-energy deterministic
// prefix (first-drop levels 1,1,2) to cover start, drop, merge, checkpoint,
// and active settlement without replaying account-specific captured boards.
//
// # Guild Land State (Namespace 25.102)
//
// Guild/union planting lives under namespace 25 field 102 (IFmlLand):
//
//	25.102.0           uid
//	25.102.1.<landId>  IOneFmlLand map (authoritative landMap replacement)
//	25.102.2           uTime
//	25.102.3           cTime
//
// Per-land fields (IOneFmlLand, numeric-string keys):
//
//	"0" = level (c_fmlLandLvl growth tier)
//	"1" = flowerId (0 = empty)
//	"2" = startTime (ms; plant start)
//	"3" = matureFlwCnt (often stale until the client UI recalculates)
//	"4" = harvestedFlwCnt
//	"5" = lastCalcTime (ms)
//
// Pending harvest prefers max(protocol mature-harvested, startTime+c_fmlLandLvl
// time/stock). Sync via fml.enter; mutate with fmlLand.harvest / fmlLand.plant.
//
// # Personal Land Fields (G.ILand, Namespace 100)
//
// Per-land fields use numeric-string keys:
//
//	"0" = flowerId (23000-23999; 0 = empty)
//	"1" = state (1=just planted/needs water, 2=growing, 3=harvestable)
//	"2" = level
//	"3" = harvestCount
//	"4" = stealUids (players who already picked this plot)
//	"5" = nextTimeMs (regrow completion timestamp)
//	"6" = elvesId (observed schema; automatic elf picking is unsupported)
//	"7" = plantTimeMs (last state change)
//	"8" = elvesStealUids (observed schema; automatic elf picking is unsupported)
//
// # Cultivation State (Namespace 101)
//
// Structure: 101.0.<flowerId>
//
//	"2" = level (0-5, max 5)
//	"3" = culTimeMs (cultivation completion timestamp)
//	"4" = status (0=idle, 1=cultivating, 2=received)
//	"5" = updateTimeMs
//	"6" = create/start timestamp
//	"8" = client marker observed on some flowers; not sufficient as upgradeability
//
// Lifecycle: cultivate → status=1 → wait culTime → recv → status=2 → upgrade → lvl+1
//
// # Waterwheel (Namespace 114)
//
// Structure: {"0": uid, "1": count, "2": advList, "3": rTime, "4": uTime, "5": cTime}
//
// Each waterwheel.recv increments field "1" and grants water drops. The mini
// client generates clickable buckets locally after waterwheel.enter using
// c_waterwheel.$bucketCreateCd, persists positions in BucketPosUsed_<uid>, and
// only calls waterwheel.recv after a local bucket click. If the next bucket
// index is present in advList (field "2"), the client calls waterwheel.skip
// before recv when no ad is played. waterwheel.skip advances to the next slot
// without waiting.
//
// # Benefit Box (Namespace 116)
//
// Structure: {"0": {"0": uid, "1": drawCnt, "2": resetCntTime, "3": uTime, "4": cTime}}
//
// The client uses drawCnt as the observed free-draw baseline. When below
// c_benefitBox.$boxMax (8), boxes refill every $boxCd seconds (3600) relative
// to resetCntTime without a namespace push — see BenefitBoxCtrl.getBenefitBoxInfo.
// Automation claims only during 04:30–05:00 Asia/Shanghai: it reads that
// accrued unopened count, then calls benefitBox.draw once per box. Daytime
// opens are left for the player.
//
// # Shop Cultivate / Material Shop (Namespace 113)
//
// Structure: {"0": uid, "1": infoMap, "2": lResetTime, "3": larTime, "4": mrCount,
// "5": lmrTime, "6": bRecord, "7": uTime, "8": cTime}
//
// infoMap holds dynamic [costItemId, costCount] per shopId. bRecord tracks buy
// counts against c_shop_cultivate.bLimit. Free auto-refresh uses
// c_shop_cultivate.$autoRefreshCd (9000s ≈ 2h30m) relative to larTime and is
// capped by $frTimes (vs mrCount). When the CD elapses and free times remain,
// automation calls shopCultivate.refresh before buy; after free times are used
// it must not refresh (paid refresh costs yuanbao via $nrResults).
//
// # Free Water (Namespace 117)
//
// Structure: {"0": uid, "1": recvIdx, "2": rTime, "3": uTime, "4": cTime}
//
// The client treats recvIdx as the list of free-water slot indexes already
// claimed for the current reset day. freeWater.recv sends {idx} for the active
// c_gameCfg.$freeWaterTime slot only; automation claims any time that window is
// open (typically 11:00–14:00 and 17:00–21:00 Asia/Shanghai) while the slot is
// still unclaimed. idx 0 is valid and must not be omitted from JSON.
//
// # Zoo / Cat State (Namespace 33)
//
// Structure:
//
//	33.0              G.IZoo, including comfort, petIdList, and field 13
//	                  souvenirRwdList (claimed collection milestone indexes)
//	33.1.<petId>      G.IZooPet, including moodValue, satietyValue,
//	                  foodstuffArr, status, strokeCdTime, statusCdTime,
//	                  goOutCdTime
//	33.2.<petId>|<idx> G.IZooLog; idx is the handleEvent tableId and proType
//	                  distinguishes pending (0) from completed logs
//	33.4.<tempId>      G.IZooSouvenir: uid, tempId, isRead, uTime, cTime
//
// The client red-dot gate uses c_zooState.isTouch plus c_zoo.$moodMax1 and
// strokeCdTime to decide whether strokePet is available. Normal bowl stocking
// uses zoo.addFoodstuff with inventory food IDs; zoo.feedPets is only an
// acknowledgement path for another player's feeding notification. Automated
// event handling is sourced from 33.2 logs, never inferred from pet fields.
// Souvenir collection progress is the number of distinct 33.4 map entries,
// independent of isRead. Reward readiness requires both that map and 33.0.13
// to be observed, then compares the count with c_zooSouvenirCollect.value.
// Sparse 33.4 entries merge by field; a null map entry deletes that souvenir.
//
// # Random Event (Namespace 129)
//
// Static client schema names IRandomEventInfo fields as eventId/posIdx/dialogId.
// Captures confirm those exact semantics: posIdx is a zero-based index into
// c_randomEvent.place and dialogId must belong to that event's dialog list.
// Namespace 129.0.1 is a whole-table replacement whenever present; missing
// sparse fields retain the previous table, while null or an empty object means
// a valid empty table. Within a replacement object, a null entry deletes that
// event id (doAffair commonly clears the claimed affair this way).
//
// # Key RPCs
//
// Land operations:
//
//	usrLand.plant         {landId, flowerId}        → {7,22,100,119}
//	usrLand.plantBatch    {landIds, flowerId}       → {7,22,100,119}
//	usrLand.water         {landId}                  → {7,22,100,119}    consumes 1 water drop
//	usrLand.waterBatch    {landIds}                 → {7,22,100,119}    consumes N water drops
//	usrLand.harvest       {landId}                  → {7,23,100,119,124}
//	usrLand.harvestOneKey {}                        → {7,23,100,119,124}
//	usrLand.unlockLand    {landId}                  → {7,100,119}       costs ~800 gold
//
// Cultivation:
//
//	cultivate.cultivate   {flowerId}                → {7,101}           starts cultivation
//	cultivate.recv        {flowerId}                → {101,103,119,130,22} claims result
//	cultivate.upgrade     {flowerId}                → {7,101,119}       upgrades a flower level; costs gold + bouquet item materials
//
// Orders:
//
//	orderFlower.finishOrder         {boxId}            → {7,22,105,119,124}
//	orderFlower.finishSatinOrder    {}                 → {7,105,119,124}   satin resident order (ns 105.0.6)
//	orderFlower.finishDecorateOrder {}                 → {7,105,119,124}   decorate resident order (ns 105.0.7)
//	orderCustomer.finishOrder       {npcId}            → {7,22,101,109,119,124}
//	orderCustomer.genOrder          {guestNpcIdList:[]} → {109}
//
// Flower art:
//
//	flowerArt.makeFlowerArt              {vaseId, flowersIds, num} → {7,119}
//	flowerRack.sell                     {rackId, iid, num}         → {7,22,104,119}
//	collectRwd.recv                     {type}                     → {7,103,...}
//	collectRwd.recvArtCreateRwdByVase   {flowerArtId:<vaseId>}     → {7,103,...}
//
// Misc:
//
//	reputation.view     {}                        → {7}             refreshes own 礼仪分/健康分
//	freeWater.recv       {idx}                     → {7,117,119}
//	usrExtra.updateAntiFraudQAStatus {}            → {7}
//	usrExtra.recvAntiFraudQARwd {}                 → {7}
//	zoo.enterZoo       {}                          → {33}
//	zoo.refreshPetStatus {petIdList:[id]}          → {33}
//	zoo.addFoodstuff   {petId,foodstuffIds}         → {7,33,...}
//	zoo.strokePet      {petId:id}                  → {33}
//	zoo.handleEvent    {petId,tableId:<log idx>,agree:true,isShareVideo:0} → {33,...}
//	zoo.readLog        {petId:id}                  → {33}
//	zoo.recvSouvenirRwd {idxList:[milestone idx]}  → {7,33,...}
//	zoo.readSouvenir  {souvenirIds:[tempId]}       → {33}
//	pearl.refresh        {}                        → {115}
//	pearl.recvDailyFree  {}                        → {7,115}
//	pearl.draw           {count}                   → {7,115}
//	pearl.setProtectState {protectState}           → {115}
//	frd.enter            {needFriendList:1,needApplyList:0,needBlackList:0} → {24,28,...}
//	oppt.getDetailOppts  {uids:[uid],extKeys:[1]}  → {28}
//	frdExt.getFrdOtherInfoByUids {uids:[uid],steal:1} → {110}
//	frdExt.buyStealCnt   {frdUid,buyCnt:1}        → {7,24,...}       costs c_frd.$pickAddCost item 1305
//	frdSteal.enterFrdSteal {point:[22,deviceFingerprint]} → {111,...}
//	frdHome.getFrdHomeInfo {frdUid}              → {133,...}
//	frdSteal.steal      {frdUid,landId,stealElves:0} → {7,111,...}
//	pearl.getHireStateByUids {uids:[uid]}          → {115.5}
//	pearl.getRecommendList {}                      → {115.5,115.6}
//	pearlPlace.hire      {placeId,dstUid}           → {7,115,...}
//	pearlPlace.recvOneKey {}                       → {7,115,148}
//	pearlPlace.recv      {placeId}                 → {7,115}
//	shopGiftbag.enter    {}                        → {112}
//	shopGiftbag.buy      {shopId,num}              → {7,112}
//	shopCultivate.enter  {}                        → {113}
//	shopCultivate.refresh {}                       → {113}
//	shopCultivate.buy    {shopId}                  → {7,113}
//	fml.bld              {id}                      → {7,25}           guild build/donation
//	fmlLand.harvest      {landIds}                 → {7,25}
//	fmlLand.plant        {landIds,flwId}           → {7,25}
//	fmlFlowerShare.refresh {}                      → {25}
//	fmlFlowerShare.getFmlOtherShareList {}         → {25}
//	fmlFlowerShare.recvRwd {slotIds}               → {7,25}
//	fmlFlowerShare.take   {dstUid,slotId}          → {7,25}
//	fmlForest.refresh    {isAutoCollect}           → {25}             sync/collect forest energy
//	waterwheel.enter      {}                        → {114}
//	waterwheel.recv       {}                        → {7,114,119}
//	waterwheel.skip       {}                        → {114}
//	storyMain.enter       {}                        → {7}
//	storyMain.unlock      {}                        → {7,22,119,148}
//	taskMain.recv         {}                        → {7,22,124}
//	taskDly.recv          {id}                      → {7,22,124}
//	usr.heartTick         {}                        → (fire-and-forget)
//
// # Capture Data Authority
//
// Wire captures are the source of truth. Static JS analysis is supplementary
// and may diverge. When they conflict, trust the capture.
package babigame
