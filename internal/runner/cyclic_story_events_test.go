package runner

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestCyclicStoryOrderClaimSuccessMessage(t *testing.T) {
	raw, err := os.ReadFile("../state/testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	s := state.New()
	s.ApplyV(json.RawMessage(raw))
	now := time.UnixMilli(1783696000000)

	op := &automation.PlannedOp{
		Kind:     clientproto.RPCActCyclicStoryRecvOrderRwd.String(),
		BatchID:  9101,
		SlotID:   0,
		TaskID:   1,
		FlowerID: 23001,
		ItemCost: map[int32]int32{23001: 80},
		Domain:   "activity.actCyclicStory",
		Action:   "claim_order",
		Category: automation.CategoryActivity,
		Label:    "莳花纪闻",
	}

	// Fixture score is 45; simulate post-claim score 53 (+8 from catalog reward).
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"11":53}}}}`))
	got := cyclicStoryOrderClaimSuccessMessage(op, 45, true, s, now)
	if !strings.Contains(got, "提交了80朵") || !strings.Contains(got, "获得了8分") || !strings.Contains(got, "累计53分") {
		t.Fatalf("message=%q", got)
	}
	if !strings.Contains(got, flowerName(23001)) && !strings.Contains(got, "#23001") {
		t.Fatalf("message missing flower name: %q", got)
	}
}

func TestCyclicStoryOrderClaimSuccessMessageCatalogFallback(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCActCyclicStoryRecvOrderRwd.String(),
		TaskID:   1,
		FlowerID: 23001,
		ItemCost: map[int32]int32{23001: 80},
	}
	got := cyclicStoryOrderClaimSuccessMessage(op, 0, false, nil, time.Time{})
	if !strings.Contains(got, "提交了80朵") || !strings.Contains(got, "获得了8分") {
		t.Fatalf("catalog fallback message=%q", got)
	}
}

func TestOperationEventLabelCyclicStory(t *testing.T) {
	for _, kind := range []string{
		clientproto.RPCActCyclicStoryEnter.String(),
		clientproto.RPCActCyclicStoryRecvOrderRwd.String(),
		clientproto.RPCActCyclicStoryRecv.String(),
	} {
		if got := operationEventLabel(&automation.PlannedOp{Kind: kind}); got != "莳花纪闻" {
			t.Fatalf("operationEventLabel(%s)=%q", kind, got)
		}
	}
}

func TestEventCategoryCyclicStoryKinds(t *testing.T) {
	for _, kind := range []string{
		"activity_cyclic_story_order",
		"activity_cyclic_story_enter",
		"activity_cyclic_story_progress",
	} {
		if got := eventCategory(kind); got != "activity" {
			t.Fatalf("eventCategory(%s)=%q", kind, got)
		}
	}
}
