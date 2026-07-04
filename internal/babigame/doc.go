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
//	3          | Recharge state                   | reLogin
//	6          | Config versions / split map      | reLogin
//	7          | Inventory (see below)            | Most RPCs
//	16         | VIP state                        | reLogin
//	20         | Shop purchase records            | shop.buy
//	22         | Daily/weekly task progress       | plant/water/harvest/cultivate
//	23         | Activity stats                   | harvest, act.getStat
//	24         | Friend list                      | frd.enter
//	25         | Guild / union state              | fml.*, Fml.build
//	27         | IM channel                       | im.getChannelId
//	31         | Share state                      | usr.share
//	33         | Zoo state                         | zoo.*
//	100        | Land state (see below)           | plant/water/harvest/unlock
//	101        | Cultivation state (see below)    | cultivate.*
//	103        | Collection rewards               | cultivate.recv, collectRwd.recv
//	104        | Flower-art rack/shelves          | flowerRack.sell
//	105        | Flower orders                    | orderFlower.finishOrder
//	106        | Flower art / share state          | flowerArt.makeFlowerArt, usr.share
//	109        | Customer orders                  | orderCustomer.*
//	112        | Gift bag shop                    | shopGiftbag.enter/buy
//	113        | Cultivation material shop        | shopCultivate.enter/buy
//	114        | Waterwheel state (see below)     | waterwheel.*
//	115        | Pearl state                       | pearl.*, pearlPlace.*
//	116        | Benefit-box state (see below)    | benefitBox.draw
//	117        | Free-water state (see below)     | freeWater.recv
//	118        | Double-coin video timer           | usr.share(shareId=11) after SDK video
//	119        | High-freq task counters          | Most RPCs
//	124        | Daily summary / popup rewards    | harvest, orders
//	131        | Observed high-frequency delta     | land, task, pass, random-event RPCs
//	132        | Observed order/pass delta          | orderFlower, flowerElvesPass
//	130        | Cultivation & art rewards        | cultivate.recv
//	148        | Observed broad activity delta      | Most reward/activity RPCs
//	165        | Celebrity state                   | celebrity.getAllTypesInfo
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
//	7.13.1.104 int                usrExtra.antiFraudQAStatus; 2 means the QA reward is claimed
//	7.13.1.105 int64              usrExtra.lastAntiFraudQATime timestamp (ms)
//	7.17.0.1   int                Own reputation/礼仪分 score; active refresh uses reputation.view
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
// Per-land fields (G.ILand schema, numeric-string keys):
//
//	"0" = flowerId (23000-23999; 0 = empty)
//	"1" = state (1=just planted/needs water, 2=growing, 3=harvestable)
//	"2" = level
//	"3" = harvestCount
//	"5" = nextTimeMs (regrow completion timestamp)
//	"7" = plantTimeMs (last state change)
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
// The client uses drawCnt as the currently available free draw count. When it
// is positive, benefitBox.draw can claim one reward; resetCntTime is the
// client-visible reset/cooldown timestamp.
//
// # Free Water (Namespace 117)
//
// Structure: {"0": uid, "1": recvIdx, "2": rTime, "3": uTime, "4": cTime}
//
// freeWater.recv sends {idx}; the static client schema names that argument and
// the namespace field recvIdx, so the observed recvIdx is the next candidate
// index unless a capture shows a channel-specific divergence.
//
// # Zoo / Cat State (Namespace 33)
//
// Structure:
//
//	33.0              G.IZoo, including comfort and petIdList
//	33.1.<petId>      G.IZooPet, including moodValue, satietyValue,
//	                  foodstuffArr, status, strokeCdTime, statusCdTime,
//	                  goOutCdTime
//
// The client red-dot gate uses c_zooState.isTouch plus c_zoo.$moodMax1 and
// strokeCdTime to decide whether strokePet is available. Feed execution is
// limited to pets with existing foodstuffArr and c_zooState.isEat; adding or
// buying food is a separate cost-bearing path.
//
// # Random Event (Namespace 129)
//
// Static client schema names IRandomEventInfo fields as eventId/posIdx/dialogId.
// The current state model still treats fields 1/2 as capture-derived
// status/affair markers; keep that behavior until fresh captures resolve the
// semantic mismatch.
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
//	orderFlower.finishOrder   {boxId}               → {7,22,105,119,124}
//	orderCustomer.finishOrder {npcId}               → {7,22,101,109,119,124}
//	orderCustomer.genOrder    {guestNpcIdList:[]}   → {109}
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
//	zoo.feedPets       {petIdList:[id]}            → {33}
//	zoo.strokePet      {petId:id}                  → {33}
//	pearl.refresh        {}                        → {115}
//	pearl.recvDailyFree  {}                        → {7,115}
//	pearl.draw           {count}                   → {7,115}
//	pearl.setProtectState {protectState}           → {115}
//	pearlPlace.recv      {placeId}                 → {7,115}
//	shopGiftbag.enter    {}                        → {112}
//	shopGiftbag.buy      {shopId,num}              → {7,112}
//	shopCultivate.enter  {}                        → {113}
//	shopCultivate.buy    {shopId}                  → {7,113}
//	Fml.build            {id}                      → {7,25}           guild build/donation
//	fmlLand.harvest      {landIds}                 → {7,25}
//	fmlFlowerShare.refresh {}                      → {25}
//	fmlFlowerShare.getFmlOtherShareList {}         → {25}
//	fmlFlowerShare.recvRwd {slotIds}               → {7,25}
//	fmlFlowerShare.take   {dstUid,slotId}          → {7,25}
//	fmlForest.refresh    {isAutoCollect}           → {25}             sync/collect forest energy
//	waterwheel.enter      {}                        → {114}
//	waterwheel.recv       {}                        → {7,114,119}
//	waterwheel.skip       {}                        → {114}
//	storyMain.unlock      {}                        → {7,22,119}
//	taskMain.recv         {}                        → {7,22,124}
//	taskDly.recv          {id}                      → {7,22,124}
//	usr.heartTick         {}                        → (fire-and-forget)
//
// # Capture Data Authority
//
// Wire captures are the source of truth. Static JS analysis is supplementary
// and may diverge. When they conflict, trust the capture.
package babigame
