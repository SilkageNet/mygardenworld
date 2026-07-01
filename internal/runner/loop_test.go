package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestWaterResponseIncludesDrops(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{
			name: "water batch current total",
			raw:  json.RawMessage(`{"7":{"2":{"1":{"7":8},"2":{"7":5}}}}`),
			want: true,
		},
		{
			name: "cold snapshot inventory",
			raw:  json.RawMessage(`{"7":{"0":{"32":{"7":12}}}}`),
			want: true,
		},
		{
			name: "spend count only is not remaining drops",
			raw:  json.RawMessage(`{"7":{"2":{"1":{"7":8}}}}`),
			want: false,
		},
		{
			name: "no water namespace",
			raw:  json.RawMessage(`{"100":{"1":{"1001":{"1":3}}}}`),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := waterResponseIncludesDrops(tc.raw); got != tc.want {
				t.Fatalf("waterResponseIncludesDrops()=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestWaterOneKeyUsesWaterOperationPath(t *testing.T) {
	if !isWaterOp(clientproto.RPCUsrLandWaterOneKey.String()) {
		t.Fatal("waterOneKey should share water verification/reservation path")
	}
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCUsrLandWaterOneKey.String(), LandIDs: []int32{1001, 1002}})
	if err != nil {
		t.Fatalf("operationArgs(waterOneKey): %v", err)
	}
	if _, ok := args.(clientproto.UsrLandWaterOneKeyRequest); !ok {
		t.Fatalf("operationArgs(waterOneKey)=%T, want UsrLandWaterOneKeyRequest", args)
	}
}

func TestNextLandUnlockCandidateDoesNotInventNextFourLands(t *testing.T) {
	st := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[fmt.Sprint(id)] = map[string]any{}
	}
	st.ApplyVMap(map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 999999}},
	})

	if id, ok := nextLandUnlockCandidate(st); ok {
		t.Fatalf("nextLandUnlockCandidate()=(%d,true), want no guessed candidate", id)
	}
}

func TestNextLandUnlockCandidateUsesRuntimeLandConfig(t *testing.T) {
	st := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[fmt.Sprint(id)] = map[string]any{}
	}
	st.ApplyVMap(map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 1500}},
	})
	st.SetFarmLands([]state.FarmLandInfo{{ID: 1025, OpenLevel: 13, Cost: []int32{37, 1500}}})

	id, ok := nextLandUnlockCandidate(st)
	if !ok || id != 1025 {
		t.Fatalf("nextLandUnlockCandidate()=(%d,%t), want (1025,true)", id, ok)
	}
}

func TestApplyHarvestBlocksSkipsBlockedSingleLand(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != nil {
		t.Fatalf("applyHarvestBlocks()=%+v, want nil", got)
	}
}

func TestApplyHarvestBlocksDowngradesOneKeyWhenSomeLandsBlocked(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvestOneKey",
		LandIDs: []int32{1001, 1002, 1003},
	}

	got := r.applyHarvestBlocks(op, now)
	if got == nil {
		t.Fatal("applyHarvestBlocks()=nil, want single harvest op")
	}
	if got.Kind != "usrLand.harvest" {
		t.Fatalf("kind=%q, want usrLand.harvest", got.Kind)
	}
	if len(got.LandIDs) != 1 || got.LandIDs[0] != 1001 {
		t.Fatalf("landIDs=%v, want [1001]", got.LandIDs)
	}
	args, err := operationArgs(got)
	if err != nil {
		t.Fatalf("operationArgs() error: %v", err)
	}
	req, ok := args.(clientproto.UsrLandHarvestRequest)
	if !ok {
		t.Fatalf("operationArgs()=%T, want UsrLandHarvestRequest", args)
	}
	if req.LandId != 1001 {
		t.Fatalf("LandId=%v, want 1001", req.LandId)
	}
}

func TestApplyHarvestBlocksIgnoresExpiredBlock(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(-time.Second)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != op {
		t.Fatalf("applyHarvestBlocks()=%+v, want original op", got)
	}
}

func TestStateHandlersDoNotClearMaterialBlocksForWaterDropOnly(t *testing.T) {
	r := newStateHandlerTestRunner()
	r.installStateHandlers()
	r.flowerUpgradeBlocked[23001] = flowerUpgradeBlock{Until: time.Now().Add(time.Hour)}
	r.cultivateBlocked[23001] = time.Now().Add(time.Hour)

	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"32": map[string]any{"7": 1},
			},
		},
	})

	if len(r.flowerUpgradeBlocked) != 1 {
		t.Fatalf("flowerUpgradeBlocked was cleared by water-drop-only inventory change")
	}
	if len(r.cultivateBlocked) != 1 {
		t.Fatalf("cultivateBlocked was cleared by water-drop-only inventory change")
	}
}

func TestStateHandlersClearMaterialBlocksForMaterialInventoryChange(t *testing.T) {
	r := newStateHandlerTestRunner()
	r.installStateHandlers()
	r.flowerUpgradeBlocked[23001] = flowerUpgradeBlock{Until: time.Now().Add(time.Hour)}
	r.cultivateBlocked[23001] = time.Now().Add(time.Hour)

	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"32": map[string]any{"23001": 1},
			},
		},
	})

	if len(r.flowerUpgradeBlocked) != 0 {
		t.Fatalf("flowerUpgradeBlocked=%v, want cleared after material inventory change", r.flowerUpgradeBlocked)
	}
	if len(r.cultivateBlocked) != 0 {
		t.Fatalf("cultivateBlocked=%v, want cleared after material inventory change", r.cultivateBlocked)
	}
}

func TestStateHandlersClearOnlyFlowerUpgradeBlocksWhenGoldIncreases(t *testing.T) {
	r := newStateHandlerTestRunner()
	r.installStateHandlers()
	r.prevGold = 100
	r.flowerUpgradeBlocked[23001] = flowerUpgradeBlock{Until: time.Now().Add(time.Hour)}
	r.flowerUpgradeBlocked[23002] = flowerUpgradeBlock{Until: time.Now().Add(time.Hour), ItemID: 3001, Have: 0}
	r.cultivateBlocked[23001] = time.Now().Add(time.Hour)

	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"44": 120,
			},
		},
	})

	if len(r.flowerUpgradeBlocked) != 1 {
		t.Fatalf("flowerUpgradeBlocked=%v, want only item-specific block preserved after gold increase", r.flowerUpgradeBlocked)
	}
	if _, ok := r.flowerUpgradeBlocked[23002]; !ok {
		t.Fatalf("item-specific flower upgrade block was cleared after gold increase: %v", r.flowerUpgradeBlocked)
	}
	if len(r.cultivateBlocked) != 1 {
		t.Fatalf("cultivateBlocked=%v, want preserved after gold increase", r.cultivateBlocked)
	}
}

func newStateHandlerTestRunner() *Runner {
	return &Runner{
		account:              &store.Account{ID: 1, Name: "test"},
		log:                  slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:                state.New(),
		harvestBlockedUntil:  make(map[int32]time.Time),
		flowerUpgradeBlocked: make(map[int32]flowerUpgradeBlock),
		cultivateBlocked:     make(map[int32]time.Time),
	}
}

func TestResidentOrderDailyLimitHelpers(t *testing.T) {
	if !isResidentOrderDailyLimit("今日完成订单次数已达上限") {
		t.Fatal("daily limit message was not recognized")
	}
	if isResidentOrderDailyLimit("鲜花数量不足") {
		t.Fatal("unrelated order error was recognized as daily limit")
	}

	loc := time.FixedZone("CST", 8*60*60)
	got := nextLocalDay(time.Date(2026, 5, 21, 8, 19, 50, 0, loc))
	want := time.Date(2026, 5, 22, 0, 5, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("nextLocalDay()=%s, want %s", got, want)
	}
}

func TestNextFulfillableFlowerOrderBox(t *testing.T) {
	orders := map[int32]*state.FlowerOrder{
		1: {BoxID: 1, Requires: []state.FlowerRequire{{FlowerID: 23001, Count: 2}}},
		2: {BoxID: 2, Requires: []state.FlowerRequire{{FlowerID: 23002, Count: 5}}},
		3: {BoxID: 3, Requires: nil},
		4: {BoxID: 4, Requires: []state.FlowerRequire{{FlowerID: 23003, Count: 2}}},
	}
	inventory := map[int32]int32{
		23001: 2,
		23002: 8,
		23003: 1,
	}

	got, ok := nextFulfillableFlowerOrderBox(orders, inventory, nil)
	if !ok || got != 1 {
		t.Fatalf("nextFulfillableFlowerOrderBox()=(%d,%t), want (1,true)", got, ok)
	}

	got, ok = nextFulfillableFlowerOrderBox(orders, inventory, map[int32]bool{1: true})
	if !ok || got != 2 {
		t.Fatalf("nextFulfillableFlowerOrderBox(skip 1)=(%d,%t), want (2,true)", got, ok)
	}

	got, ok = nextFulfillableFlowerOrderBox(orders, inventory, map[int32]bool{1: true, 2: true})
	if ok {
		t.Fatalf("nextFulfillableFlowerOrderBox(skip ready boxes)=(%d,%t), want no match", got, ok)
	}
}
