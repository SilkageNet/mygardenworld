package runner

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestPearlHireSuccessMessageIncludesTicketSpend(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 6}}},
		"115": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"2": int64(2001),
				"3": now.Add(2 * time.Hour).UnixMilli(),
				"4": 0,
				"6": 5,
			},
		}},
	})
	st.SetPearlHireTicketUsed(20260828, 4)

	op := &automation.PlannedOp{
		Kind:     clientproto.RPCPearlPlaceHire.String(),
		Category: automation.CategoryHire,
		Domain:   "basic.pearl.hire",
		Label:    "雇佣劳工",
		TargetID: 1,
		TargetUID: 2001,
	}
	got := pearlHireSuccessMessage(op, st, now)
	if !strings.Contains(got, "珍珠雇佣成功") ||
		!strings.Contains(got, "槽位=1") ||
		!strings.Contains(got, "劳工=2001") ||
		!strings.Contains(got, "产出=5/次") ||
		!strings.Contains(got, "预计获取珍珠=200") ||
		!strings.Contains(got, "雇佣书 -1（剩余 6，今日已用 4）") {
		t.Fatalf("pearlHireSuccessMessage=%q", got)
	}
}

func TestPearlHireFailureMessageNotesTicketSpend(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"1003": 5}}},
	})
	st.SetPearlHireTicketUsed(20260828, 3)
	op := &automation.PlannedOp{Kind: clientproto.RPCPearlPlaceHire.String(), Label: "雇佣劳工"}
	got := pearlHireFailureMessage(op, fmtError("pearlPlace.hire candidate was contested (hireFailCnt=1)"), st, now)
	if !strings.Contains(got, "雇佣劳工 失败") ||
		!strings.Contains(got, "hireFailCnt=1") ||
		!strings.Contains(got, "雇佣书已消耗（剩余 5，今日已用 3）") {
		t.Fatalf("pearlHireFailureMessage=%q", got)
	}
	if plain := pearlHireFailureMessage(op, fmtError("pearl_tips4"), st, now); strings.Contains(plain, "雇佣书已消耗") {
		t.Fatalf("tips4 failure should not claim ticket spend: %q", plain)
	}
}

func TestHandleOperationSuccessEmitsHireCategory(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 2}}},
		"115": map[string]any{"0": map[string]any{
			"1": map[string]any{"2": int64(2001), "3": now.Add(time.Hour).UnixMilli(), "4": 0, "6": 5},
		}},
	})
	st.SetPearlHireTicketUsed(20260828, 1)

	bus := NewBus()
	ch, cancel := bus.SubscribeLive(4)
	defer cancel()

	r := &Runner{
		state:   st,
		bus:     bus,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		account: &store.Account{ID: 1, Name: "test"},
	}
	r.handleOperationSuccess(context.Background(), operationResult{
		operationAttempt: operationAttempt{
			op: &automation.PlannedOp{
				Kind:      clientproto.RPCPearlPlaceHire.String(),
				Category:  automation.CategoryHire,
				Domain:    "basic.pearl.hire",
				Action:    "hire",
				Label:     "雇佣劳工",
				TargetID:  1,
				TargetUID: 2001,
			},
		},
		finishedAt: now,
	})

	select {
	case ev := <-ch:
		if ev.Kind != "pearl_hire" || ev.Category != automation.CategoryHire || ev.Label != "雇佣劳工" {
			t.Fatalf("event=%+v", ev)
		}
		if !strings.Contains(ev.Message, "珍珠雇佣成功") || !strings.Contains(ev.Message, "雇佣书 -1") {
			t.Fatalf("message=%q", ev.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pearl hire event")
	}
}

func TestOperationEventLabelPearlHire(t *testing.T) {
	if got := operationEventLabel(&automation.PlannedOp{Kind: clientproto.RPCPearlPlaceHire.String()}); got != "雇佣劳工" {
		t.Fatalf("operationEventLabel=%q", got)
	}
	if got := opKindDesc(clientproto.RPCPearlPlaceHire.String()); got != "雇佣劳工" {
		t.Fatalf("opKindDesc=%q", got)
	}
}

type fmtError string

func (e fmtError) Error() string { return string(e) }
