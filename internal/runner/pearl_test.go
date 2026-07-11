package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestExecutePearlRecvOneKeyAppliesPayloadOnce(t *testing.T) {
	now := time.UnixMilli(8_000_000)
	st := newReadyPearlRunnerState()
	snapshot, ok := st.PearlClaimSnapshot(now)
	if !ok {
		t.Fatal("claim snapshot unavailable")
	}
	var order []string
	applyCount := 0
	exec := pearlRecvOneKeyExecution{
		preflight: func() (state.PearlClaimSnapshot, error) {
			order = append(order, "preflight")
			return snapshot, nil
		},
		recv: func(_ context.Context, got clientproto.PearlPlaceRecvOneKeyRequest) (json.RawMessage, error) {
			order = append(order, "recv")
			if !reflect.DeepEqual(got, clientproto.PearlPlaceRecvOneKeyRequest{}) {
				t.Fatalf("request=%+v, want empty", got)
			}
			return json.RawMessage(`{"115":{"0":{"1":{"2":0,"3":null,"6":0,"7":0,"8":0},"2":{"2":0,"3":null,"6":0,"7":0,"8":0}}}}`), nil
		},
		apply: func(raw json.RawMessage) {
			order = append(order, "apply")
			applyCount++
			st.ApplyV(raw)
		},
		claimed: func(got state.PearlClaimSnapshot) bool {
			order = append(order, "postcondition")
			return st.PearlClaimApplied(got)
		},
	}
	raw, err := executePearlRecvOneKey(context.Background(), clientproto.PearlPlaceRecvOneKeyRequest{}, exec)
	if err != nil {
		t.Fatalf("executePearlRecvOneKey: %v", err)
	}
	if applyCount != 1 {
		t.Fatalf("payload apply count=%d, want exactly 1", applyCount)
	}
	if got, want := strings.Join(order, ","), "preflight,recv,apply,postcondition"; got != want {
		t.Fatalf("execution order=%q, want %q", got, want)
	}
	if !json.Valid(raw) || !st.PearlClaimApplied(snapshot) {
		t.Fatalf("post state/raw invalid: raw=%s", raw)
	}
}

func TestExecutePearlRecvOneKeyAcceptsRecvCountAdvanceAtFixedInstant(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0},
	}}})
	claimAt := time.UnixMilli(180_000)
	snapshot, ok := st.PearlClaimSnapshot(claimAt)
	if !ok {
		t.Fatal("claim snapshot unavailable")
	}
	exec := pearlRecvOneKeyExecution{
		preflight: func() (state.PearlClaimSnapshot, error) { return snapshot, nil },
		recv: func(context.Context, clientproto.PearlPlaceRecvOneKeyRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"115":{"0":{"1":{"7":1,"8":0}}}}`), nil
		},
		apply:   st.ApplyV,
		claimed: st.PearlClaimApplied,
	}
	if _, err := executePearlRecvOneKey(context.Background(), clientproto.PearlPlaceRecvOneKeyRequest{}, exec); err != nil {
		t.Fatalf("executePearlRecvOneKey: %v", err)
	}
	if st.PearlClaimApplied(state.PearlClaimSnapshot{At: time.UnixMilli(360_000), PlaceIDs: []int32{1}}) {
		t.Fatal("postcondition used moving time; second cycle should be ready at 360 seconds")
	}
}

func TestExecutePearlRecvOneKeyRejectsStaleEmptyAndPartial(t *testing.T) {
	t.Run("stale preflight prevents RPC", func(t *testing.T) {
		st := newReadyPearlRunnerState()
		st.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{"1": nil, "2": nil}}})
		calls := 0
		exec := pearlRecvOneKeyExecution{
			preflight: func() (state.PearlClaimSnapshot, error) {
				if snapshot, ok := st.PearlClaimSnapshot(time.UnixMilli(8_000_000)); ok {
					return snapshot, nil
				}
				return state.PearlClaimSnapshot{}, errors.New("preflight rejected: no time-matured production")
			},
			recv: func(context.Context, clientproto.PearlPlaceRecvOneKeyRequest) (json.RawMessage, error) {
				calls++
				return nil, nil
			},
			apply:   func(json.RawMessage) {},
			claimed: st.PearlClaimApplied,
		}
		_, err := executePearlRecvOneKey(context.Background(), clientproto.PearlPlaceRecvOneKeyRequest{}, exec)
		if err == nil || !strings.Contains(err.Error(), "preflight rejected") {
			t.Fatalf("stale preflight error=%v", err)
		}
		if calls != 0 {
			t.Fatalf("RPC calls=%d after stale preflight", calls)
		}
		assertPearlErrorEntersCooldown(t, err)
	})

	for _, tc := range []struct {
		name     string
		response json.RawMessage
	}{
		{name: "empty", response: json.RawMessage(`{}`)},
		{name: "partial", response: json.RawMessage(`{"115":{"0":{"1":{"3":null,"6":0,"7":0,"8":0}}}}`)},
	} {
		t.Run(tc.name+" response enters cooldown", func(t *testing.T) {
			st := newReadyPearlRunnerState()
			snapshot, ok := st.PearlClaimSnapshot(time.UnixMilli(8_000_000))
			if !ok {
				t.Fatal("claim snapshot unavailable")
			}
			exec := pearlRecvOneKeyExecution{
				preflight: func() (state.PearlClaimSnapshot, error) { return snapshot, nil },
				recv: func(context.Context, clientproto.PearlPlaceRecvOneKeyRequest) (json.RawMessage, error) {
					return tc.response, nil
				},
				apply:   st.ApplyV,
				claimed: st.PearlClaimApplied,
			}
			_, err := executePearlRecvOneKey(context.Background(), clientproto.PearlPlaceRecvOneKeyRequest{}, exec)
			if err == nil || !strings.Contains(err.Error(), "postcondition failed") {
				t.Fatalf("unchanged response error=%v", err)
			}

			assertPearlErrorEntersCooldown(t, err)
		})
	}
}

func assertPearlErrorEntersCooldown(t *testing.T, err error) {
	t.Helper()
	runner := newOperationEventTestRunner()
	op := &automation.PlannedOp{
		OperationID: "pearl-recv-one-key",
		Kind:        clientproto.RPCPearlPlaceRecvOneKey.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryBasic,
		Domain:      "basic.pearl.place",
		Action:      "claim",
	}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	got := runner.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              err,
		finishedAt:       now,
	})
	if !errors.Is(got, err) {
		t.Fatalf("handleOperationError=%v, want original error", got)
	}
	if _, cooling := runner.operationCoolingDown(op, now.Add(time.Second)); !cooling {
		t.Fatal("pearl recvOneKey failure did not enter side-operation cooldown")
	}
}

func TestPearlRecvOneKeyRequestRejectsNonEmptyOrCostlyOperation(t *testing.T) {
	if _, err := pearlRecvOneKeyRequest(&automation.PlannedOp{}); err != nil {
		t.Fatalf("empty operation rejected: %v", err)
	}
	for _, op := range []*automation.PlannedOp{
		{TargetID: 1},
		{Count: 1},
		{GoldCost: 1},
		{ItemCost: map[int32]int32{1003: 1}},
	} {
		if _, err := pearlRecvOneKeyRequest(op); err == nil {
			t.Fatalf("non-empty/costly operation accepted: %+v", op)
		}
	}
}

func newReadyPearlRunnerState() *state.State {
	st := state.New()
	st.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0},
		"2": map[string]any{"1": 2, "3": int64(7_200_000), "6": 4, "7": 0, "8": 0},
	}}})
	return st
}
