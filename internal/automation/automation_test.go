package automation

import (
	"strconv"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func applyMap(t *testing.T, s *state.State, top map[string]any) {
	t.Helper()
	s.ApplyVMap(top)
}

func emptyLands(n int) map[string]any {
	lands := make(map[string]any, n)
	for i := 0; i < n; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	return lands
}

func cultivate(flowers ...int32) map[string]any {
	out := make(map[string]any, len(flowers))
	for _, id := range flowers {
		out[itoa32(id)] = map[string]any{"1": id, "2": 1, "4": 2}
	}
	return out
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func itoa32(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func TestBuildPlan_CustomerArtDemandDrivesPlanting(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 0, "23007": 0, "23008": 0},
			"34": 12,
		}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(3)}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.CraftEnabled = true

	result := BuildPlan(s, p, time.Now())
	var planted bool
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" && op.FlowerID != 0 {
			planted = true
			break
		}
	}
	if !planted {
		t.Fatalf("expected customer art flower demand to produce plant op, ops=%+v demands=%+v", result.Operations, result.Demands)
	}
}

func TestBuildPlan_CustomerArtBlockedByMissingVase(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 2, "23007": 2, "23008": 2},
			"34": 12,
		}},
		"102": map[string]any{"0": map[string]any{"3001": map[string]any{"1": 3001}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.CraftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && len(op.BlockedReasons) == 0 {
			t.Fatalf("craft op should be blocked by missing vase: %+v", op)
		}
	}
}

func TestBuildPlan_FlowerRackRespectsCustomerLedgerAllocation(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 1}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerRackSell.String() {
			t.Fatalf("customer art allocation should not be sold on rack: %+v demands=%+v", op, result.Demands)
		}
	}
}

func TestBuildPlan_FlowerArtRewardsProduceClaimOps(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"103": map[string]any{
			"0": map[string]any{
				"11": map[string]any{"1": 11, "2": 0, "3": 5, "4": []int32{}},
				"13": map[string]any{"1": 13, "2": 0, "3": 70, "4": []int32{}, "7": []int32{}},
			},
		},
		"106": map[string]any{"0": map[string]any{"2": []int32{300101}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.CreateRewardEnabled = true
	p.Order.FlowerArt.CollectRewardEnabled = true

	result := BuildPlan(s, p, time.Now())
	var createReward, collectReward bool
	for _, op := range result.Operations {
		if op.Domain == "order.flower_art.create_reward" {
			createReward = true
			if op.Kind != clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String() || op.TargetID != 3001 || !op.Executable || op.SyncOnly {
				t.Fatalf("create reward op mismatch: %+v", op)
			}
		}
		if op.Domain == "order.flower_art.collect_reward" {
			collectReward = true
			if op.Kind != clientproto.RPCCollectRwdRecv.String() || op.TargetID != 11 || !op.Executable || op.SyncOnly {
				t.Fatalf("collect reward op mismatch: %+v", op)
			}
		}
	}
	if !createReward || !collectReward {
		t.Fatalf("missing reward ops create=%t collect=%t ops=%+v", createReward, collectReward, result.Operations)
	}
}

func TestBuildPlan_ShopCultivateEnterBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateEnter.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("shop cultivate sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate sync op: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagEnterBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.giftbag" {
			if op.Kind != clientproto.RPCShopGiftbagEnter.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("shop giftbag sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop giftbag sync op: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagVideoGift(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"112": map[string]any{
			"1": map[string]any{"1": 3},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.video_gift" {
			if op.Kind != clientproto.RPCShopGiftbagBuy.String() || op.TargetID != 1 || op.Count != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("shop giftbag buy op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop giftbag buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopGiftbagPaidGiftIgnoredAndVipBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"112": map[string]any{
			"1": map[string]any{"1": 4},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true
	p.Basic.Shop.VipShop.AutoBuy = true

	result := BuildPlan(s, p, time.Now())
	var vipBlocked bool
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.video_gift" {
			t.Fatalf("paid or exhausted giftbag should not produce buy op: %+v", op)
		}
		if op.Domain == "basic.shop.vip" {
			vipBlocked = !op.Executable && len(op.BlockedReasons) > 0
		}
	}
	if !vipBlocked {
		t.Fatalf("missing blocked vip shop op: %+v", result.Operations)
	}
}

func TestBuildPlan_AntiScamBoxLifecycle(t *testing.T) {
	cases := []struct {
		name       string
		status     int32
		wantKind   string
		wantAction string
		wantOp     bool
	}{
		{name: "not answered", status: 0, wantKind: clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(), wantAction: "answer", wantOp: true},
		{name: "ready to claim", status: 1, wantKind: clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(), wantAction: "claim", wantOp: true},
		{name: "claimed", status: state.AntiFraudQAStatusClaimed, wantOp: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{
				"7": map[string]any{
					"13": map[string]any{
						"1": map[string]any{"104": tc.status},
					},
				},
			})
			p := DefaultPolicy()
			p.AutomationEnabled = true
			p.Basic.Benefit.AntiScamBoxEnabled = true

			result := BuildPlan(s, p, time.Now())
			for _, op := range result.Operations {
				if op.Domain != "basic.benefit.anti_scam" {
					continue
				}
				if !tc.wantOp {
					t.Fatalf("claimed anti-scam reward should not produce op: %+v", op)
				}
				if op.Kind != tc.wantKind || op.Action != tc.wantAction || op.FeatureID != "basic.anti_scam_box" || !op.Executable || op.SyncOnly {
					t.Fatalf("anti-scam op mismatch: %+v", op)
				}
				return
			}
			if tc.wantOp {
				t.Fatalf("missing anti-scam op: %+v", result.Operations)
			}
		})
	}
}

func TestBuildPlan_DoubleCoinBlockedUnlessActive(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Benefit.DoubleCoinEnabled = true

	s := state.New()
	result := BuildPlan(s, p, now)
	var blocked PlannedOp
	for _, op := range result.Operations {
		if op.Domain == "basic.benefit.double_coin" {
			blocked = op
			break
		}
	}
	if blocked.Domain == "" || blocked.Executable || blocked.Status != PlanStatusAdapterMissing || blocked.FeatureID != "basic.double_coin" || len(blocked.BlockedReasons) == 0 {
		t.Fatalf("double coin blocked op mismatch: %+v", blocked)
	}
	if got := Plan(s, p, now); got != nil && got.Domain == "basic.benefit.double_coin" {
		t.Fatalf("Plan returned blocked double coin op: %+v", got)
	}

	applyMap(t, s, map[string]any{
		"118": map[string]any{
			"1": 1,
			"2": now.Add(time.Hour).UnixMilli(),
		},
	})
	result = BuildPlan(s, p, now)
	for _, op := range result.Operations {
		if op.Domain == "basic.benefit.double_coin" {
			t.Fatalf("active double coin should not produce op: %+v", op)
		}
	}
}

func TestBuildPlan_ZooSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.FeedCat.Enabled = true
	p.Basic.FeedCat.AutoFeed = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.zoo" {
			if op.Kind != clientproto.RPCZooEnterZoo.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("zoo sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing zoo sync op: %+v", result.Operations)
}

func TestBuildPlan_ZooFeedAndStroke(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"33": map[string]any{
			"0": map[string]any{"0": 77900091102482},
			"1": map[string]any{
				"1": map[string]any{
					"1":  1,
					"2":  50,
					"3":  20,
					"4":  []int32{1501},
					"5":  2,
					"12": now.Add(-time.Minute).UnixMilli(),
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.FeedCat.Enabled = true
	p.Basic.FeedCat.AutoFeed = true
	p.Basic.FeedCat.AutoStroke = true

	result := BuildPlan(s, p, now)
	want := map[string]string{
		"basic.zoo.feed":   clientproto.RPCZooFeedPets.String(),
		"basic.zoo.stroke": clientproto.RPCZooStrokePet.String(),
	}
	seen := map[string]bool{}
	for _, op := range result.Operations {
		if kind, ok := want[op.Domain]; ok {
			seen[op.Domain] = true
			if op.Kind != kind || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("zoo op mismatch for %s: %+v", op.Domain, op)
			}
		}
	}
	for domain := range want {
		if !seen[domain] {
			t.Fatalf("missing %s op: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_ZooCostAndRecallBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"0": 1}}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.FeedCat.Enabled = true
	p.Basic.FeedCat.AutoBuyFood = true
	p.Basic.FeedCat.AutoRecall = true

	result := BuildPlan(s, p, time.Now())
	want := map[string]bool{"basic.zoo.buy_food": false, "basic.zoo.recall": false}
	for _, op := range result.Operations {
		if _, ok := want[op.Domain]; ok {
			if op.Executable || op.Status != PlanStatusAdapterMissing || len(op.BlockedReasons) == 0 {
				t.Fatalf("zoo blocked op mismatch: %+v", op)
			}
			want[op.Domain] = true
		}
	}
	for domain, seen := range want {
		if !seen {
			t.Fatalf("missing blocked %s op: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_PearlRefreshBeforeObserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.FreeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.pearl" {
			if op.Kind != clientproto.RPCPearlRefresh.String() || op.Action != "sync" || !op.Executable || op.SyncOnly {
				t.Fatalf("pearl sync op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing pearl sync op: %+v", result.Operations)
}

func TestBuildPlan_PearlExecutableOps(t *testing.T) {
	s := state.New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"115": map[string]any{
			"0": map[string]any{"1": map[string]any{"1": 1, "8": 2}},
			"1": map[string]any{"1": 0, "2": 1, "6": now.Add(-24 * time.Hour).UnixMilli()},
			"2": []int32{101, 102},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.FreeEnabled = true
	p.Basic.Pearl.DrawEnabled = true
	p.Basic.Pearl.ProtectEnabled = true

	result := BuildPlan(s, p, now)
	want := map[string]string{
		"basic.pearl.free":    clientproto.RPCPearlRecvDailyFree.String(),
		"basic.pearl.place":   clientproto.RPCPearlPlaceRecv.String(),
		"basic.pearl.protect": clientproto.RPCPearlSetProtectState.String(),
		"basic.pearl.draw":    clientproto.RPCPearlDraw.String(),
	}
	seen := map[string]bool{}
	for _, op := range result.Operations {
		if kind, ok := want[op.Domain]; ok {
			seen[op.Domain] = true
			if op.Kind != kind || !op.Executable || op.SyncOnly {
				t.Fatalf("pearl op mismatch for %s: %+v", op.Domain, op)
			}
			if op.Domain == "basic.pearl.place" && op.TargetID != 1 {
				t.Fatalf("pearl place target=%d, want 1", op.TargetID)
			}
			if op.Domain == "basic.pearl.protect" && op.TargetID != 1 {
				t.Fatalf("pearl protect target=%d, want 1", op.TargetID)
			}
			if op.Domain == "basic.pearl.draw" && op.Count != 1 {
				t.Fatalf("pearl draw count=%d, want 1", op.Count)
			}
		}
	}
	for domain := range want {
		if !seen[domain] {
			t.Fatalf("missing pearl op %s: %+v", domain, result.Operations)
		}
	}
}

func TestBuildPlan_PearlHireAndBuyTicketBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"115": map[string]any{"1": map[string]any{}}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.AutoHireEnabled = true
	p.Basic.Pearl.AutoBuyHireTicket = true
	p.Basic.Pearl.MaxSpendDiamond = 25

	result := BuildPlan(s, p, time.Now())
	var hireBlocked, buyBlocked bool
	for _, op := range result.Operations {
		switch op.Domain {
		case "basic.pearl.hire":
			hireBlocked = !op.Executable && len(op.BlockedReasons) > 0
		case "basic.pearl.buy_hire_ticket":
			buyBlocked = !op.Executable && len(op.BlockedReasons) > 0
		}
	}
	if !hireBlocked || !buyBlocked {
		t.Fatalf("expected pearl blocked ops hire=%t buy=%t ops=%+v", hireBlocked, buyBlocked, result.Operations)
	}
}

func TestBuildPlan_ShopCultivateBuyWithGoldBudget(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 5000}},
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 3214}},
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendGold = 4000
	p.Basic.Shop.CultivateShop.ItemIds = []int32{1423}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Kind != clientproto.RPCShopCultivateBuy.String() || op.TargetID != 10001 || op.ItemID != 1423 || op.GoldCost != 3214 || !op.Executable || op.SyncOnly {
				t.Fatalf("shop cultivate buy op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing shop cultivate buy op: %+v", result.Operations)
}

func TestBuildPlan_ShopCultivateDiamondCostBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{1, 10}},
			"6": map[string]any{"10001": 0},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Shop.CultivateShop.AutoBuy = true
	p.Basic.Shop.CultivateShop.MaxSpendDiamond = 20
	p.Basic.Shop.CultivateShop.ItemIds = []int32{10001}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "basic.shop.cultivate" {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("diamond shop cultivate op should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked shop cultivate op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildFreeAndGold(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 20000}},
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"1": 0, "2": 0}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.FreeEnabled = true
	p.Union.Build.GoldEnabled = true
	p.Union.Build.MaxSpendGold = 12000

	result := BuildPlan(s, p, time.Now())
	var freeSeen bool
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			freeSeen = true
			if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 1 || !op.Executable || op.SyncOnly || op.GoldCost != 0 {
				t.Fatalf("free union build op mismatch: %+v", op)
			}
			break
		}
	}
	if !freeSeen {
		t.Fatalf("missing free union build op: %+v", result.Operations)
	}

	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"1": 1, "2": 0}},
		},
	})
	result = BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 2 || op.GoldCost != 10095 || !op.Executable || op.SyncOnly {
				t.Fatalf("gold union build op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildDiamondBlocked(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"41": 200}},
		"25": map[string]any{
			"133": map[string]any{"1": 88, "5": map[string]any{"3": 0}},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.DiamondEnabled = true
	p.Union.Build.MaxSpendDiamond = 200

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Kind != clientproto.RPCFmlBuild.String() || op.TargetID != 3 || op.DiamondCost != 106 || op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("diamond union build op should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionBuildRequiresObservedCounts(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{"0": map[string]any{"0": 88}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Build.FreeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.build" {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("union build without count map should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked union build op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandHarvest(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"1": 23005, "3": 6, "4": 2},
					"2": map[string]any{"1": 23007, "3": 4, "4": 4},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.harvest" {
			if op.Kind != clientproto.RPCFmlLandHarvest.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union land harvest op mismatch: %+v", op)
			}
			if len(op.LandIDs) != 1 || op.LandIDs[0] != 1 || op.Count != 1 {
				t.Fatalf("union land harvest ids/count mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union land harvest op: %+v", result.Operations)
}

func TestBuildPlan_UnionLandHarvestRequiresObservedState(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.HarvestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.land.harvest" {
			if op.Executable || len(op.BlockedReasons) == 0 {
				t.Fatalf("unobserved union land should be blocked: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing blocked union land harvest op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerShareReward(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{
					"1": map[string]any{"0": 23005, "1": 10, "2": 3},
					"2": map[string]any{"0": 23007, "1": 10, "2": 0},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.ShareEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.flower.reward" {
			if op.Kind != clientproto.RPCFmlFlowerShareRecvRwd.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union flower reward op mismatch: %+v", op)
			}
			if len(op.SlotIDs) != 1 || op.SlotIDs[0] != 1 || op.Count != 1 {
				t.Fatalf("union flower reward slot ids/count mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union flower reward op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerTakeSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			if op.Kind != clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union flower take sync op mismatch: %+v", op)
			}
			return
		}
		if op.Domain == "union.unknown" {
			t.Fatalf("take should not be folded into union.unknown: %+v", op)
		}
	}
	t.Fatalf("missing union flower take sync op: %+v", result.Operations)
}

func TestBuildPlan_UnionFlowerTakeSpecificFlower(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"108": []any{
				map[string]any{
					"0": 77900091102483,
					"1": map[string]any{
						"1": map[string]any{"0": 23009, "1": 8, "2": 7},
					},
				},
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 6, "2": 1},
					},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Flower.TakeEnabled = true
	p.Union.Flower.TakeMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	p.Union.Flower.TakeFlowerIds = []int32{23011}

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.flower.take" {
			if op.Kind != clientproto.RPCFmlFlowerShareTake.String() || !op.Executable || op.SyncOnly {
				t.Fatalf("union flower take op mismatch: %+v", op)
			}
			if op.TargetUID != 77900091102484 || op.TargetID != 2 || op.FlowerID != 23011 || op.Count != 1 {
				t.Fatalf("union flower take target mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing union flower take op: %+v", result.Operations)
}

func TestBuildPlan_UnionForestSyncWhenUnobserved(t *testing.T) {
	s := state.New()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.ForestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.forest" {
			if op.Kind != clientproto.RPCFmlForestRefresh.String() || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("union forest sync op mismatch: %+v", op)
			}
			return
		}
		if op.Domain == "union.unknown" {
			t.Fatalf("forest should not be folded into union.unknown: %+v", op)
		}
	}
	t.Fatalf("missing union forest sync op: %+v", result.Operations)
}

func TestBuildPlan_UnionForestCollect(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"127": map[string]any{
				"1": 88,
				"8": map[string]any{
					"88": map[string]any{"1": 5},
					"99": map[string]any{"1": 4, "3": 2},
				},
			},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.ForestEnabled = true

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Domain == "union.forest" {
			if op.Kind != clientproto.RPCFmlForestRefresh.String() || op.TargetID != 1 || !op.Executable || op.SyncOnly {
				t.Fatalf("union forest collect op mismatch: %+v", op)
			}
			if op.Count != 11 {
				t.Fatalf("union forest collect count=%d, want 11: %+v", op.Count, op)
			}
			return
		}
	}
	t.Fatalf("missing union forest collect op: %+v", result.Operations)
}

func TestBuildPlan_LowStockFallbackBalancesMultipleFlowers(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 0,
			"23002": 1,
			"23003": 5,
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(6)}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Flower.PlantMaxBatch = 6
	p.Plant.Flower.MaxPerFlowerPerCycle = 4

	result := BuildPlan(s, p, time.Now())
	countByFlower := map[int32]int32{}
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" {
			countByFlower[op.FlowerID] += int32(len(op.LandIDs))
		}
	}
	if countByFlower[23001] == 0 || countByFlower[23002] == 0 {
		t.Fatalf("fallback should split across low-stock flowers, got %v ops=%+v", countByFlower, result.Operations)
	}
	if countByFlower[23001] > 4 {
		t.Fatalf("fallback exceeded max_per_flower_per_cycle: %v", countByFlower)
	}
}

func TestBuildPlan_LowStockFallbackHonorsStockFloor(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 0,
			"23002": 2,
			"23003": 5,
		}}},
		"100": map[string]any{"0": map[string]any{"1": emptyLands(6)}},
		"101": map[string]any{"0": cultivate(23001, 23002, 23003)},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Flower.PlantMaxBatch = 6
	p.Plant.Flower.MaxPerFlowerPerCycle = 6
	p.Plant.Flower.FallbackStockFloor = 2

	result := BuildPlan(s, p, time.Now())
	countByFlower := map[int32]int32{}
	for _, op := range result.Operations {
		if op.Domain == "farm.plant" {
			countByFlower[op.FlowerID] += int32(len(op.LandIDs))
		}
	}
	if countByFlower[23001] != 2 {
		t.Fatalf("fallback should fill 23001 to floor 2, got %v ops=%+v", countByFlower, result.Operations)
	}
	if countByFlower[23002] != 0 || countByFlower[23003] != 0 {
		t.Fatalf("fallback should not plant flowers already at/above floor, got %v", countByFlower)
	}
}

func TestNextLandUnlockCandidateDoesNotInventNextFourLands(t *testing.T) {
	s := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[itoa32(id)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 999999}},
	})

	if id, _, ok := nextLandUnlockCandidate(s); ok {
		t.Fatalf("nextLandUnlockCandidate()=(%d,true), want no guessed candidate", id)
	}
}

func TestNextLandUnlockCandidateUsesRuntimeLandConfig(t *testing.T) {
	s := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[itoa32(id)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 1500}},
	})
	s.SetFarmLands([]state.FarmLandInfo{{ID: 1025, OpenLevel: 13, Cost: []int32{37, 1500}}})

	id, cost, ok := nextLandUnlockCandidate(s)
	if !ok || id != 1025 {
		t.Fatalf("nextLandUnlockCandidate()=(%d,%t), want (1025,true)", id, ok)
	}
	if cost != 1474 {
		t.Fatalf("nextLandUnlockCandidate cost=%d, want 1474", cost)
	}
}
