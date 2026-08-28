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
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func validPearlHireOperation() *automation.PlannedOp {
	return &automation.PlannedOp{
		Kind: clientproto.RPCPearlPlaceHire.String(), TargetID: 2, TargetUID: 2001,
		Count: 1, ItemCost: map[int32]int32{1003: 1},
	}
}

func TestPearlHireRequestBuildersAreExact(t *testing.T) {
	hire, err := pearlHireRequest(validPearlHireOperation())
	if err != nil || hire.PlaceId != 2 || hire.DstUid != 2001 {
		t.Fatalf("hire request=%+v err=%v", hire, err)
	}
	for _, mutate := range []func(*automation.PlannedOp){
		func(op *automation.PlannedOp) { op.ItemCost = nil },
		func(op *automation.PlannedOp) { op.ItemCost = map[int32]int32{1003: 2} },
		func(op *automation.PlannedOp) { op.ItemCost[11] = 1 },
		func(op *automation.PlannedOp) { op.GoldCost = 1 },
		func(op *automation.PlannedOp) { op.TargetUIDs = []int64{2001} },
	} {
		op := validPearlHireOperation()
		mutate(op)
		if _, err := pearlHireRequest(op); err == nil {
			t.Fatalf("unsafe hire operation accepted: %+v", op)
		}
	}

	syncOp := &automation.PlannedOp{TargetUIDs: []int64{2001, 2002}}
	detail, err := pearlCandidateDetailRequest(syncOp)
	if err != nil || !reflect.DeepEqual(detail.UIDs, clientproto.RPCUIDList{2001, 2002}) || !reflect.DeepEqual(detail.ExtKeys, clientproto.RPCIDList{1}) {
		t.Fatalf("detail request=%+v err=%v", detail, err)
	}
	hireState, err := pearlCandidateHireStateRequest(syncOp)
	if err != nil || !reflect.DeepEqual(hireState.UIDs, clientproto.RPCUIDList{2001, 2002}) {
		t.Fatalf("hire-state request=%+v err=%v", hireState, err)
	}
	friend, err := pearlFriendSyncRequest(&automation.PlannedOp{})
	if err != nil || friend.NeedFriendList != 1 || friend.NeedApplyList != 0 || friend.NeedBlackList != 0 {
		t.Fatalf("friend request=%+v err=%v", friend, err)
	}
	if _, err := pearlCandidateDetailRequest(&automation.PlannedOp{TargetUIDs: []int64{2001, 2001}}); err == nil {
		t.Fatal("duplicate candidate UID accepted")
	}
}

func TestPearlHireGoldFallbackStrictField(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fallback bool
		wantErr  bool
	}{
		{name: "missing", raw: `{"115":{}}`},
		{name: "zero", raw: `{"3":{"0":0}}`},
		{name: "nonzero", raw: `{"3":{"0":1}}`, fallback: true},
		{name: "negative nonzero", raw: `{"3":{"0":-1}}`, fallback: true},
		{name: "null", raw: `{"3":{"0":null}}`, wantErr: true},
		{name: "string", raw: `{"3":{"0":"0"}}`, wantErr: true},
		{name: "decimal", raw: `{"3":{"0":0.5}}`, wantErr: true},
		{name: "object", raw: `{"3":{"0":{}}}`, wantErr: true},
		{name: "malformed namespace", raw: `{"3":[]}`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fallback, err := pearlHireGoldFallback(json.RawMessage(tc.raw))
			if fallback != tc.fallback || (err != nil) != tc.wantErr {
				t.Fatalf("fallback=%t err=%v", fallback, err)
			}
		})
	}
}

func TestExecutePearlHireOutcomesAndLocks(t *testing.T) {
	base := time.UnixMilli(1_700_000_000_000)
	snapshot := state.PearlHireAttemptSnapshot{At: base, PlaceID: 1, TargetUID: 2001, TicketCount: 3}
	tests := []struct {
		name       string
		raw        string
		hireErr    error
		success    bool
		failCount  int32
		known      bool
		spent      bool
		wantErr    string
		wantFailed bool
		wantLocked bool
		wantNoted  bool
	}{
		{name: "success explicit zero", raw: `{"3":{"0":0},"115":{}}`, success: true, known: true, spent: true, wantNoted: true},
		{name: "contested", raw: `{"115":{}}`, failCount: 2, known: true, spent: true, wantErr: "contested", wantFailed: true, wantNoted: true},
		{name: "tips4 contested", hireErr: &babigame.RPCServerError{
			Name:     clientproto.RPCPearlPlaceHire,
			Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":"pearl_tips4","msg":"对方已被其他人雇佣","param":[4]}`)},
		}, wantErr: "contested", wantFailed: true},
		{name: "gold fallback", raw: `{"3":{"0":1}}`, wantErr: "金币回退", wantFailed: true, wantLocked: true},
		{name: "malformed fallback", raw: `{"3":{"0":null}}`, wantErr: "格式异常", wantFailed: true, wantLocked: true},
		{name: "postcondition unknown", raw: `{"115":{}}`, wantErr: "postcondition", wantFailed: true, wantLocked: true},
		{name: "transport unknown", hireErr: errors.New("timeout"), wantErr: "timeout", wantFailed: true, wantLocked: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			failed, locked, applied, noted := false, false, false, false
			exec := pearlHireExecution{
				preflight: func(time.Time) (state.PearlHireAttemptSnapshot, error) { return snapshot, nil },
				hire: func(context.Context, clientproto.PearlPlaceHireRequest) (json.RawMessage, error) {
					return json.RawMessage(tc.raw), tc.hireErr
				},
				apply: func(json.RawMessage) { applied = true },
				outcome: func(state.PearlHireAttemptSnapshot) (bool, int32, bool) {
					return tc.success, tc.failCount, tc.known
				},
				ticketSpent: func(state.PearlHireAttemptSnapshot) bool { return tc.spent },
				markFailed:  func(uid int64, _ time.Time) { failed = uid == 2001 },
				noteUsed:    func(time.Time) { noted = true },
				lockSession: func(string) { locked = true },
				now:         func() time.Time { return base },
			}
			_, err := executePearlHire(context.Background(), clientproto.PearlPlaceHireRequest{PlaceId: 1, DstUid: 2001}, exec)
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err=%v, want substring %q", err, tc.wantErr)
			}
			if failed != tc.wantFailed || locked != tc.wantLocked || noted != tc.wantNoted {
				t.Fatalf("failed=%t locked=%t noted=%t", failed, locked, noted)
			}
			if tc.raw != "" && tc.hireErr == nil && !applied {
				t.Fatal("authoritative payload was not applied")
			}
		})
	}
}

func TestPearlHireOperationRegistryAndFreshSessionReset(t *testing.T) {
	for _, kind := range []string{
		clientproto.RPCFrdEnter.String(),
		clientproto.RPCOpptGetDetailOppts.String(),
		clientproto.RPCPearlGetHireStateByUids.String(),
		clientproto.RPCPearlGetRecommendList.String(),
		clientproto.RPCPearlPlaceHire.String(),
	} {
		if spec, ok := operationSpecFor(kind); !ok || spec.args == nil || spec.run == nil {
			t.Fatalf("operation %s not fully registered", kind)
		}
	}

	s := state.New()
	s.LockPearlHireSession("fallback")
	s.MarkPearlHireFailed(2001, time.Now())
	r := &Runner{
		state: s,
		operationCooldowns: map[string]operationCooldown{
			clientproto.RPCPearlPlaceHire.String() + ":1:2001": {},
			"unrelated": {},
		},
	}
	r.resetPearlHireSession()
	view := s.PearlHire()
	if view.SessionLocked || len(view.FailedUntilMs) != 0 {
		t.Fatalf("state reset incomplete: %+v", view)
	}
	if _, exists := r.operationCooldowns[clientproto.RPCPearlPlaceHire.String()+":1:2001"]; exists {
		t.Fatal("hire side cooldown survived fresh session")
	}
	if _, exists := r.operationCooldowns["unrelated"]; !exists {
		t.Fatal("fresh session reset removed unrelated cooldown")
	}
}
