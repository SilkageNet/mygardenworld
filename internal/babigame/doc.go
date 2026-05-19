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
//	27         | IM channel                       | im.getChannelId
//	31         | Share state                      | usr.share
//	100        | Land state (see below)           | plant/water/harvest/unlock
//	101        | Cultivation state (see below)    | cultivate.*
//	103        | Collection rewards               | cultivate.recv
//	105        | Flower orders                    | orderFlower.finishOrder
//	109        | Customer orders                  | orderCustomer.*
//	112        | Gift bag shop                    | shopGiftbag.enter
//	114        | Waterwheel state (see below)     | waterwheel.*
//	117        | Free-water reward state           | freeWater.recv
//	119        | High-freq task counters          | Most RPCs
//	124        | Daily summary / popup rewards    | harvest, orders
//	130        | Cultivation & art rewards        | cultivate.recv
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
//	7.0.41     int                Diamonds (free)
//	7.0.42     int                Diamonds (paid)
//	7.0.44     int                Gold coins
//
// # Land State (Namespace 100)
//
// Land data lives under namespace 100:
//
//	100.0.1.<landId>   Full roster (reLogin cold-start)
//	100.1.<landId>     Primary state delta (after plant/water/harvest)
//	100.2.<landId>     Harvest reward data
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
// Structure: {"0": aid, "1": claimedCount, "2": advList, "3": rTime, "4": uTime, "5": cTime}
//
// Each waterwheel.recv increments field "1" and grants water drops. The client
// generates clickable buckets locally using c_waterwheel.$bucketCreateCd.
// waterwheel.skip advances to the next slot without waiting.
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
// Misc:
//
//	freeWater.recv       {idx}                     → {7,117,119}
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
