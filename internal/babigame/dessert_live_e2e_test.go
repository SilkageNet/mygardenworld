package babigame_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/dessertphysics"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

const (
	dessertE2EMode         int32 = 1
	dessertE2EDropBudget         = 3
	dessertE2EAdvanceLimit       = 1800
)

// TestDessertModeOneLifecycleE2E is an explicitly authorized, mutating live
// test. It consumes exactly three activity-energy points in normal mode and
// exercises the causal client sequence:
//
//	enter -> gameStart -> 3 drops -> 2 merges -> checkpoint -> gameOver
//
// Ordinary go test runs always skip it. E2E_DESSERT_LIVE=1 is deliberately
// separate from the general E2E credential gate so login-only tests can never
// start a real activity round by accident.
func TestDessertModeOneLifecycleE2E(t *testing.T) {
	if os.Getenv("E2E_DESSERT_LIVE") != "1" {
		t.Skip("E2E_DESSERT_LIVE=1 not set; skipping mutating dessert E2E")
	}

	username, password, cfg := dessertLiveCredentials(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	httpClient := babigame.NewHTTPClient(cfg, "", "", "")
	session, err := babigame.PerformLoginWithPassword(ctx, httpClient, username, password, 1)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	client := babigame.NewClient(session)
	defer func() { _ = client.Close() }()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("websocket connect: %v", err)
	}

	tracker := state.New()
	loginV, err := client.Login(ctx, 1)
	if err != nil {
		t.Fatalf("index.login: %v", err)
	}
	tracker.ApplyV(loginV)
	lazyV, err := client.LazySync(ctx)
	if err != nil {
		t.Fatalf("usr.lazySync: %v", err)
	}
	tracker.ApplyV(lazyV)

	rpc := clientrpc.NewClient(babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(15*time.Second),
		babigame.WithApplyV(tracker.ApplyV),
	))

	view := dessertLiveView(t, tracker)
	dessertLiveDiscoveryPreflight(t, view)
	batchID := view.BatchID

	if _, err := rpc.ActDessert().Enter(ctx, clientproto.ActDessertEnterRequest{BatchId: batchID}); err != nil {
		t.Fatalf("actDessert.enter: %v", err)
	}
	view = dessertLiveView(t, tracker)
	if view.BatchID != batchID {
		t.Fatalf("dessert batch changed after enter: %d -> %d", batchID, view.BatchID)
	}
	dessertLivePreflight(t, view)
	initialRunning := make(map[int32]bool, len(view.Modes))
	for _, mode := range view.Modes {
		initialRunning[mode.Mode] = mode.IsRunning
	}

	startAttempted := false
	cleanedModes := make(map[int32]bool)
	defer func() {
		if !startAttempted {
			return
		}
		observeCtx, observeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, observeErr := rpc.ActDessert().Enter(observeCtx, clientproto.ActDessertEnterRequest{BatchId: batchID})
		observeCancel()
		if observeErr != nil {
			t.Logf("cleanup observation failed; refusing blind gameOver: %v", observeErr)
			return
		}
		cleanupView, found := tracker.DessertView(time.Now())
		if !found || cleanupView.BatchID != batchID {
			t.Logf("cleanup observation lost dessert batch %d; refusing blind gameOver", batchID)
			return
		}
		for modeID, wasRunning := range initialRunning {
			if wasRunning || cleanedModes[modeID] {
				continue
			}
			cleanupMode, observed := dessertModeIfObserved(cleanupView, modeID)
			if !observed || !cleanupMode.IsRunning {
				continue
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, cleanupErr := rpc.ActDessert().GameOver(cleanupCtx, clientproto.ActDessertGameOverRequest{
				BatchId: batchID, GameType: modeID,
			})
			cleanupCancel()
			if cleanupErr != nil {
				t.Logf("cleanup gameOver mode %d failed: %v", modeID, cleanupErr)
			}
		}
	}()

	beforeStart := dessertEconomyOf(view)
	startAttempted = true
	startResponse, err := rpc.ActDessert().GameStart(ctx, clientproto.ActDessertGameStartRequest{BatchId: batchID})
	if err != nil {
		t.Fatalf("actDessert.gameStart: %v", err)
	}
	view = dessertLiveView(t, tracker)
	mode := dessertLiveMode(t, view, dessertE2EMode)
	if !mode.IsRunning || mode.Step != 0 || mode.CurID != 1 || mode.ObjectCount != 0 || mode.Score != 0 {
		// A completed response plus unchanged idle mode is an authoritative
		// no-op, not an ambiguous timeout. Do not call gameOver on a round the
		// test did not actually create.
		startAttempted = false
		t.Fatalf("gameStart did not initialize mode1: phase=%d now=%d begin=%d end=%d grace=%d envelope=%s payload=%s mode=%+v",
			view.Phase, time.Now().UnixMilli(), view.BeginMs, view.EndMs, view.GraceEndMs,
			string(startResponse.Envelope.M), string(startResponse.Payload), mode)
	}
	if got := dessertEconomyOf(view); got != beforeStart {
		t.Fatalf("gameStart changed economy: before=%+v after=%+v", beforeStart, got)
	}

	world, err := dessertphysics.NewWorld(dessertphysics.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("new empty dessert world: %v", err)
	}
	catalog, ok := state.DessertCatalogConfig()
	if !ok {
		t.Fatal("dessert catalog invariant failed")
	}

	mergeResults := []int{0, 2, 3}
	for dropIndex := 0; dropIndex < dessertE2EDropBudget; dropIndex++ {
		view = dessertLiveView(t, tracker)
		mode = dessertLiveMode(t, view, dessertE2EMode)
		level := int(mode.CurID)
		wantLevel := int(catalog.FirstDrops[dropIndex])
		if level != wantLevel {
			t.Fatalf("drop %d curId=%d, want fixed first-drop level %d", dropIndex+1, level, wantLevel)
		}
		if !world.Snapshot().Stable {
			t.Fatalf("drop %d started from a moving local board: %+v", dropIndex+1, world.Snapshot())
		}

		// The official client exposes a waiting ball only after 800 ms. It is
		// absent from saveData until released, then appears at y=360 with zero
		// velocity. Physics starts only after the RPC succeeds.
		time.Sleep(850 * time.Millisecond)
		beforeDropView := view
		beforeDropMode := mode
		wireMap := dessertWireBodies(world.Snapshot().Bodies)
		wireMap = append(wireMap, dessertWaitingWireBody(level, 0))
		dessertLiveSync(t, ctx, rpc, batchID, mode, wireMap, 1, 0)

		view = dessertLiveView(t, tracker)
		mode = dessertLiveMode(t, view, dessertE2EMode)
		assertDessertDropResult(t, catalog, dropIndex, beforeDropView, beforeDropMode, view, mode)
		if mode.ObjectCount != int32(len(wireMap)) {
			t.Fatalf("drop %d server object count=%d, submitted=%d", dropIndex+1, mode.ObjectCount, len(wireMap))
		}

		if _, err := world.AddDrop(0, level, dessertphysics.Vec2{X: 0, Y: dessertphysics.DefaultConfig().WaitingYPX}); err != nil {
			t.Fatalf("drop %d local release: %v", dropIndex+1, err)
		}
		advanced, err := world.AdvanceUntil(dessertE2EAdvanceLimit)
		if err != nil {
			t.Fatalf("drop %d local advance: %v", dropIndex+1, err)
		}

		wantMergeLevel := mergeResults[dropIndex]
		if wantMergeLevel == 0 {
			if advanced.Reason != dessertphysics.AdvanceStable {
				t.Fatalf("drop %d advance reason=%v, want stable", dropIndex+1, advanced.Reason)
			}
			continue
		}
		if advanced.Reason != dessertphysics.AdvanceMerge || len(advanced.Last.MergeCandidates) != 1 {
			t.Fatalf("drop %d advance=%+v, want one deterministic merge", dropIndex+1, advanced)
		}
		candidate := advanced.Last.MergeCandidates[0]
		if candidate.Level+1 != wantMergeLevel {
			t.Fatalf("drop %d merge result level=%d, want %d", dropIndex+1, candidate.Level+1, wantMergeLevel)
		}
		if _, err := world.ApplyMerge(candidate); err != nil {
			t.Fatalf("drop %d apply local merge: %v", dropIndex+1, err)
		}

		beforeMergeView := view
		beforeMergeMode := mode
		mergedWire := dessertWireBodies(world.Snapshot().Bodies)
		dessertLiveSync(t, ctx, rpc, batchID, mode, mergedWire, 0, int32(wantMergeLevel))
		view = dessertLiveView(t, tracker)
		mode = dessertLiveMode(t, view, dessertE2EMode)
		assertDessertMergeResult(t, catalog, int32(wantMergeLevel), beforeMergeView, beforeMergeMode, view, mode, world.Snapshot())

		settled, err := world.AdvanceUntil(dessertE2EAdvanceLimit)
		if err != nil {
			t.Fatalf("merge level %d settle: %v", wantMergeLevel, err)
		}
		if settled.Reason != dessertphysics.AdvanceStable {
			t.Fatalf("merge level %d settle reason=%v", wantMergeLevel, settled.Reason)
		}
	}

	// This is the same zero-operation save used by the client when switching
	// modes or persisting a terminal board. It proves a stable board can be
	// checkpointed without changing activity economy.
	view = dessertLiveView(t, tracker)
	mode = dessertLiveMode(t, view, dessertE2EMode)
	checkpointEconomy := dessertEconomyOf(view)
	checkpointStep, checkpointScore := mode.Step, mode.Score
	checkpointLevels := dessertSnapshotLevels(world.Snapshot())
	dessertLiveSync(t, ctx, rpc, batchID, mode, dessertWireBodies(world.Snapshot().Bodies), 0, 0)
	view = dessertLiveView(t, tracker)
	mode = dessertLiveMode(t, view, dessertE2EMode)
	if got := dessertEconomyOf(view); got != checkpointEconomy || mode.Step != checkpointStep || mode.Score != checkpointScore ||
		!reflect.DeepEqual(mode.LevelCounts, checkpointLevels) {
		t.Fatalf("checkpoint changed state: economy=%+v/%+v step=%d/%d score=%d/%d levels=%v/%v",
			checkpointEconomy, got, checkpointStep, mode.Step, checkpointScore, mode.Score, checkpointLevels, mode.LevelCounts)
	}

	beforeOver := view
	beforeOverMode := mode
	if _, err := rpc.ActDessert().GameOver(ctx, clientproto.ActDessertGameOverRequest{
		BatchId: batchID, GameType: dessertE2EMode,
	}); err != nil {
		t.Fatalf("actDessert.gameOver: %v", err)
	}
	cleanedModes[dessertE2EMode] = true
	view = dessertLiveView(t, tracker)
	mode = dessertLiveMode(t, view, dessertE2EMode)
	assertDessertGameOverResult(t, beforeOver, beforeOverMode, view, mode)

	// gameStart initializes all five empty modes. Restore every mode that was
	// idle before this test so the live account is left in its original shape.
	modeIDs := make([]int, 0, len(initialRunning))
	for modeID, wasRunning := range initialRunning {
		if modeID != dessertE2EMode && !wasRunning {
			modeIDs = append(modeIDs, int(modeID))
		}
	}
	sort.Ints(modeIDs)
	for _, rawModeID := range modeIDs {
		modeID := int32(rawModeID)
		if _, err := rpc.ActDessert().GameOver(ctx, clientproto.ActDessertGameOverRequest{
			BatchId: batchID, GameType: modeID,
		}); err != nil {
			t.Fatalf("restore mode %d: %v", modeID, err)
		}
		cleanedModes[modeID] = true
	}

	t.Logf("live dessert E2E passed: batch=%d drops=3 merges=2 energy=%d->%d currency=%d->%d score=%d",
		batchID, beforeStart.Energy, view.EnergyBalance, beforeStart.Currency, view.CurrencyBalance, beforeOverMode.Score)
}

type dessertEconomy struct {
	Energy     int32
	Currency   int32
	Boxes      int32
	DropCount  int32
	TotalScore int32
}

func dessertEconomyOf(view state.DessertView) dessertEconomy {
	return dessertEconomy{
		Energy: view.EnergyBalance, Currency: view.CurrencyBalance, Boxes: view.RewardBoxBalance,
		DropCount: view.DropCount, TotalScore: view.TotalScore,
	}
}

func dessertLiveCredentials(t *testing.T) (string, string, babigame.Config) {
	t.Helper()
	username, password := os.Getenv("E2E_USERNAME"), os.Getenv("E2E_PASSWORD")
	channelName := os.Getenv("E2E_CHANNEL")
	if channelName == "" {
		channelName = string(babigame.ChannelIOS)
	}
	if username != "" || password != "" {
		if username == "" || password == "" {
			t.Fatal("E2E_USERNAME and E2E_PASSWORD must be set together")
		}
		channel, err := babigame.ParseChannel(channelName)
		if err != nil {
			t.Fatalf("E2E_CHANNEL: %v", err)
		}
		cfg, err := babigame.ConfigForChannel(channel)
		if err != nil {
			t.Fatalf("channel config: %v", err)
		}
		return username, password, cfg
	}

	dataDir := os.Getenv("E2E_DATA_DIR")
	if dataDir == "" {
		t.Skip("set E2E_USERNAME/E2E_PASSWORD or E2E_DATA_DIR for live dessert E2E")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Open(ctx, filepath.Join(dataDir, "garden.db"))
	if err != nil {
		t.Fatalf("open E2E data store: %v", err)
	}
	defer func() { _ = db.Close() }()

	var accountID int64
	if rawID := os.Getenv("E2E_ACCOUNT_ID"); rawID != "" {
		accountID, err = strconv.ParseInt(rawID, 10, 64)
		if err != nil || accountID <= 0 {
			t.Fatalf("E2E_ACCOUNT_ID=%q is invalid", rawID)
		}
	} else {
		accounts, listErr := db.ListAccounts(ctx, 0)
		if listErr != nil {
			t.Fatalf("list E2E accounts: %v", listErr)
		}
		if len(accounts) != 1 {
			t.Fatalf("E2E_ACCOUNT_ID required when data store has %d accounts", len(accounts))
		}
		accountID = accounts[0].ID
	}
	account, err := db.GetAccountByID(ctx, accountID)
	if err != nil {
		t.Fatalf("load E2E account %d: %v", accountID, err)
	}
	username, password, err = db.GetCredentials(ctx, accountID)
	if err != nil {
		t.Fatalf("decrypt E2E account %d credentials: %v", accountID, err)
	}
	channel, err := babigame.ParseChannel(account.Channel)
	if err != nil {
		t.Fatalf("account %d channel: %v", accountID, err)
	}
	cfg, err := babigame.ConfigForChannel(channel)
	if err != nil {
		t.Fatalf("account %d config: %v", accountID, err)
	}
	return username, password, cfg
}

func dessertLiveView(t *testing.T, tracker *state.State) state.DessertView {
	t.Helper()
	view, found := tracker.DessertView(time.Now())
	if !found || !view.Found {
		t.Fatal("active dessert batch not found in namespace 23")
	}
	return view
}

func dessertLiveMode(t *testing.T, view state.DessertView, modeID int32) state.DessertModeView {
	t.Helper()
	for _, mode := range view.Modes {
		if mode.Mode == modeID {
			if !mode.Observed || !mode.Valid {
				t.Fatalf("dessert mode %d is incomplete: %+v", modeID, mode)
			}
			return mode
		}
	}
	t.Fatalf("dessert mode %d not found", modeID)
	return state.DessertModeView{}
}

func dessertLivePreflight(t *testing.T, view state.DessertView) {
	t.Helper()
	dessertLiveDiscoveryPreflight(t, view)
	if view.Phase != 2 {
		t.Skipf("dessert game phase is closed: phase=%d now=%s begin=%s end=%s grace=%s; live gameplay E2E waits for the next active phase",
			view.Phase, time.Now().Format(time.RFC3339), time.UnixMilli(view.BeginMs).Format(time.RFC3339),
			time.UnixMilli(view.EndMs).Format(time.RFC3339), time.UnixMilli(view.GraceEndMs).Format(time.RFC3339))
	}
	if !view.BagObserved || !view.ExtensionObserved || !view.ExtensionValid || !view.ModeMapObserved || !view.ModeMapValid || len(view.Modes) != 5 {
		t.Fatalf("dessert state incomplete: bag=%t ext=%t/%t modes=%t/%t count=%d",
			view.BagObserved, view.ExtensionObserved, view.ExtensionValid, view.ModeMapObserved, view.ModeMapValid, len(view.Modes))
	}
	if view.EnergyBalance < dessertE2EDropBudget {
		t.Fatalf("dessert energy=%d, need %d after enter (bag=%v tasks=%+v milestones=%+v celebrity=%+v)",
			view.EnergyBalance, dessertE2EDropBudget, view.Bag, view.Tasks, view.Milestones, view.Celebrity)
	}
	for _, mode := range view.Modes {
		if !mode.Observed || !mode.Valid || mode.ObjectCount != 0 || mode.Step != 0 || mode.Score != 0 {
			t.Fatalf("refusing to overwrite existing mode %d round: running=%t step=%d score=%d objects=%d",
				mode.Mode, mode.IsRunning, mode.Step, mode.Score, mode.ObjectCount)
		}
	}
	target := dessertLiveMode(t, view, dessertE2EMode)
	if target.IsRunning {
		t.Fatal("mode 1 is already running; refusing to take over a round not created by this test")
	}
}

func dessertLiveDiscoveryPreflight(t *testing.T, view state.DessertView) {
	t.Helper()
	if view.TmpType != 5601 || view.BatchID <= 0 || view.Status != 1 || (view.Phase != 2 && view.Phase != 3) {
		t.Fatalf("dessert identity/phase is not live-testable: batch=%d type=%d status=%d phase=%d", view.BatchID, view.TmpType, view.Status, view.Phase)
	}
}

func dessertModeIfObserved(view state.DessertView, modeID int32) (state.DessertModeView, bool) {
	for _, mode := range view.Modes {
		if mode.Mode == modeID && mode.Observed && mode.Valid {
			return mode, true
		}
	}
	return state.DessertModeView{}, false
}

func dessertLiveSync(
	t *testing.T,
	ctx context.Context,
	rpc *clientrpc.Client,
	batchID int32,
	mode state.DessertModeView,
	bodies []any,
	operationType int32,
	mergeLevel int32,
) {
	t.Helper()
	args := clientproto.RPCObject{
		"_useItem2SelIdx": int32(0),
		"operationType":   operationType,
		"mergeLvl":        mergeLevel,
		"saveData": clientproto.RPCObject{
			"step":       mode.Step,
			"itemUse":    dessertIntMap(mode.ItemUse),
			"map":        bodies,
			"gameStatus": mode.GameStatus,
			"firstMerge": dessertIntMap(mode.FirstMerge),
			"isRunning":  true,
			"totalGain":  dessertIntMap(mode.TotalGain),
			"curId":      mode.CurID,
			"score":      mode.Score,
			"lvMap":      dessertIntMap(mode.LevelMap),
		},
	}
	_, err := rpc.ActDessert().GameSync(ctx, clientproto.ActDessertGameSyncRequest{
		BatchId: batchID, GameType: dessertE2EMode, Args: args,
	})
	if err != nil {
		t.Fatalf("actDessert.gameSync operation=%d merge=%d: %v", operationType, mergeLevel, err)
	}
}

func dessertWireBodies(bodies []dessertphysics.BodyState) []any {
	out := make([]any, 0, len(bodies))
	for _, body := range bodies {
		wire := clientproto.RPCObject{
			"lv":    body.Level,
			"isSyn": body.IsSyn,
			"pos": clientproto.RPCObject{
				"x": body.PositionPX.X,
				"y": body.PositionPX.Y,
			},
			"linearVelocity": clientproto.RPCObject{
				"x": body.LinearVelocityMPS.X,
				"y": body.LinearVelocityMPS.Y,
			},
			"angularVelocity": body.AngularVelocityRadPerS,
			"scale": clientproto.RPCObject{
				"x": body.ScaleX,
				"y": body.ScaleY,
				"z": 0.5,
			},
			"nodeAngle": body.AngleRad * 180 / math.Pi,
			"isAwake":   body.Awake,
			"_lineTime": body.DangerLineTimeMS,
		}
		if body.IsFallBall {
			wire["isFallBall"] = true
		}
		out = append(out, wire)
	}
	return out
}

func dessertWaitingWireBody(level int, x float64) any {
	return clientproto.RPCObject{
		"lv":    level,
		"isSyn": false,
		"pos": clientproto.RPCObject{
			"x": x,
			"y": dessertphysics.DefaultConfig().WaitingYPX,
		},
		"linearVelocity":  clientproto.RPCObject{"x": float64(0), "y": float64(0)},
		"angularVelocity": float64(0),
		"scale":           clientproto.RPCObject{"x": float64(1), "y": float64(1), "z": float64(0.5)},
		"nodeAngle":       float64(0),
		"isAwake":         true,
		"_lineTime":       float64(0),
		"isFallBall":      true,
	}
}

func dessertIntMap(in map[int32]int32) map[int32]int32 {
	out := make(map[int32]int32, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func assertDessertDropResult(
	t *testing.T,
	catalog state.DessertCatalog,
	dropIndex int,
	beforeView state.DessertView,
	beforeMode state.DessertModeView,
	afterView state.DessertView,
	afterMode state.DessertModeView,
) {
	t.Helper()
	before, after := dessertEconomyOf(beforeView), dessertEconomyOf(afterView)
	if after.Energy != before.Energy-1 || after.DropCount != before.DropCount+1 || after.Currency < before.Currency+1 ||
		after.Boxes != before.Boxes || after.TotalScore != before.TotalScore {
		t.Fatalf("drop %d economy mismatch: before=%+v after=%+v", dropIndex+1, before, after)
	}
	if afterMode.Step != beforeMode.Step+1 || afterMode.Score != beforeMode.Score || !afterMode.IsRunning {
		t.Fatalf("drop %d mode mismatch: before=%+v after=%+v", dropIndex+1, beforeMode, afterMode)
	}
	wantNext := catalog.FirstDrops[dropIndex+1]
	if afterMode.CurID != wantNext {
		t.Fatalf("drop %d next curId=%d, want %d", dropIndex+1, afterMode.CurID, wantNext)
	}
	if afterMode.TotalGain[catalog.CurrencyItemID] < beforeMode.TotalGain[catalog.CurrencyItemID]+1 {
		t.Fatalf("drop %d totalGain currency did not increase: %v -> %v", dropIndex+1, beforeMode.TotalGain, afterMode.TotalGain)
	}
}

func assertDessertMergeResult(
	t *testing.T,
	catalog state.DessertCatalog,
	resultLevel int32,
	beforeView state.DessertView,
	beforeMode state.DessertModeView,
	afterView state.DessertView,
	afterMode state.DessertModeView,
	snapshot dessertphysics.Snapshot,
) {
	t.Helper()
	if resultLevel <= 0 || int(resultLevel) > len(catalog.Levels) {
		t.Fatalf("invalid expected merge level %d", resultLevel)
	}
	level := catalog.Levels[resultLevel-1]
	before, after := dessertEconomyOf(beforeView), dessertEconomyOf(afterView)
	if after.Energy != before.Energy || after.DropCount != before.DropCount || after.Currency != before.Currency ||
		after.Boxes != before.Boxes || after.TotalScore != before.TotalScore+level.Score {
		t.Fatalf("merge level %d economy mismatch: before=%+v after=%+v catalog=%+v", resultLevel, before, after, level)
	}
	if afterMode.Step != beforeMode.Step || afterMode.Score != beforeMode.Score+level.Score || afterMode.CurID != beforeMode.CurID ||
		!afterMode.IsRunning {
		t.Fatalf("merge level %d mode mismatch: before=%+v after=%+v", resultLevel, beforeMode, afterMode)
	}
	if afterMode.FirstMerge[resultLevel] < beforeMode.FirstMerge[resultLevel] {
		t.Fatalf("merge level %d firstMerge regressed: %v -> %v", resultLevel, beforeMode.FirstMerge, afterMode.FirstMerge)
	}
	wantLevels := dessertSnapshotLevels(snapshot)
	if !reflect.DeepEqual(afterMode.LevelCounts, wantLevels) {
		t.Fatalf("merge level %d server levels=%v, local=%v", resultLevel, afterMode.LevelCounts, wantLevels)
	}
}

func assertDessertGameOverResult(
	t *testing.T,
	beforeView state.DessertView,
	beforeMode state.DessertModeView,
	afterView state.DessertView,
	afterMode state.DessertModeView,
) {
	t.Helper()
	if afterMode.IsRunning || afterMode.Step != 0 || afterMode.CurID != 0 || afterMode.Score != 0 || afterMode.ObjectCount != 0 {
		t.Fatalf("gameOver did not clear mode1: %+v", afterMode)
	}
	if got, want := dessertEconomyOf(afterView), dessertEconomyOf(beforeView); got != want {
		t.Fatalf("gameOver changed settled economy: before=%+v after=%+v", want, got)
	}
	if !reflect.DeepEqual(afterMode.TotalGain, beforeMode.TotalGain) {
		t.Fatalf("gameOver changed totalGain: %v -> %v", beforeMode.TotalGain, afterMode.TotalGain)
	}
}

func dessertSnapshotLevels(snapshot dessertphysics.Snapshot) map[int32]int32 {
	out := make(map[int32]int32)
	for _, body := range snapshot.Bodies {
		out[int32(body.Level)]++
	}
	return out
}

func (e dessertEconomy) String() string {
	return fmt.Sprintf("energy=%d currency=%d boxes=%d drops=%d totalScore=%d", e.Energy, e.Currency, e.Boxes, e.DropCount, e.TotalScore)
}
