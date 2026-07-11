package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestExecuteZooRecvSouvenirRewardAppliesOnceAndChecksWholeBatch(t *testing.T) {
	st := newZooSouvenirRunnerState(3, []int32{1}, nil)
	indices := []int32{2, 3}
	req := clientproto.ZooRecvSouvenirRwdRequest{IdxList: clientproto.RPCIDList(indices)}
	var order []string
	applyCount := 0
	exec := zooRecvSouvenirRewardExecution{
		preflight: func() error {
			order = append(order, "preflight")
			if !st.ZooSouvenirRewardsReady(indices) {
				return errors.New("preflight rejected")
			}
			return nil
		},
		recv: func(_ context.Context, got clientproto.ZooRecvSouvenirRwdRequest) (json.RawMessage, error) {
			order = append(order, "recv")
			if len(got.IdxList) != 2 || got.IdxList[0] != 2 || got.IdxList[1] != 3 {
				t.Fatalf("recv request=%+v", got)
			}
			return json.RawMessage(`{"33":{"0":{"13":[1,2,3]}}}`), nil
		},
		apply: func(raw json.RawMessage) {
			order = append(order, "apply")
			applyCount++
			st.ApplyV(raw)
		},
		claimed: func() bool {
			order = append(order, "postcondition")
			return st.ZooSouvenirRewardsClaimed(indices)
		},
	}
	raw, err := executeZooRecvSouvenirReward(context.Background(), req, exec)
	if err != nil {
		t.Fatalf("executeZooRecvSouvenirReward: %v", err)
	}
	if applyCount != 1 || !json.Valid(raw) {
		t.Fatalf("applyCount=%d raw=%s", applyCount, raw)
	}
	if got, want := strings.Join(order, ","), "preflight,recv,apply,postcondition"; got != want {
		t.Fatalf("execution order=%q, want %q", got, want)
	}
}

func TestExecuteZooRecvSouvenirRewardRejectsStaleAndPartialResponses(t *testing.T) {
	t.Run("stale preflight", func(t *testing.T) {
		st := newZooSouvenirRunnerState(2, []int32{1, 2}, nil)
		calls := 0
		exec := zooRecvSouvenirRewardExecution{
			preflight: func() error {
				if !st.ZooSouvenirRewardsReady([]int32{2}) {
					return errors.New("preflight rejected: milestone no longer ready")
				}
				return nil
			},
			recv: func(context.Context, clientproto.ZooRecvSouvenirRwdRequest) (json.RawMessage, error) {
				calls++
				return nil, nil
			},
			claimed: func() bool { return st.ZooSouvenirRewardsClaimed([]int32{2}) },
		}
		_, err := executeZooRecvSouvenirReward(context.Background(), clientproto.ZooRecvSouvenirRwdRequest{IdxList: clientproto.RPCIDList{2}}, exec)
		if err == nil || !strings.Contains(err.Error(), "preflight rejected") || calls != 0 {
			t.Fatalf("stale preflight err=%v calls=%d", err, calls)
		}
	})

	t.Run("partial response", func(t *testing.T) {
		st := newZooSouvenirRunnerState(3, []int32{1}, nil)
		indices := []int32{2, 3}
		exec := zooRecvSouvenirRewardExecution{
			preflight: func() error { return nil },
			recv: func(context.Context, clientproto.ZooRecvSouvenirRwdRequest) (json.RawMessage, error) {
				return json.RawMessage(`{"33":{"0":{"13":[1,2]}}}`), nil
			},
			apply:   st.ApplyV,
			claimed: func() bool { return st.ZooSouvenirRewardsClaimed(indices) },
		}
		_, err := executeZooRecvSouvenirReward(context.Background(), clientproto.ZooRecvSouvenirRwdRequest{IdxList: clientproto.RPCIDList(indices)}, exec)
		if err == nil || !strings.Contains(err.Error(), "postcondition failed") {
			t.Fatalf("partial response error=%v", err)
		}
		assertZooSouvenirErrorCooldown(t, clientproto.RPCZooRecvSouvenirRwd.String(), "claim", err)
	})
}

func TestExecuteZooReadSouvenirAppliesOnceAndAcceptsExplicitDeletion(t *testing.T) {
	t.Run("sparse read flags", func(t *testing.T) {
		ids := []int32{30201, 32901}
		st := newZooSouvenirRunnerState(2, []int32{1, 2}, ids)
		applyCount := 0
		exec := zooReadSouvenirExecution{
			preflight: func() error {
				if !st.ZooSouvenirsReadyToAcknowledge(ids) {
					return errors.New("preflight rejected")
				}
				return nil
			},
			read: func(_ context.Context, got clientproto.ZooReadSouvenirRequest) (json.RawMessage, error) {
				if len(got.SouvenirIds) != 2 || got.SouvenirIds[0] != 30201 || got.SouvenirIds[1] != 32901 {
					t.Fatalf("read request=%+v", got)
				}
				return json.RawMessage(`{"33":{"4":{"30201":{"2":1,"3":5000},"32901":{"2":1,"3":5000}}}}`), nil
			},
			apply: func(raw json.RawMessage) {
				applyCount++
				st.ApplyV(raw)
			},
			acknowledged: func() bool { return st.ZooSouvenirsAcknowledged(ids) },
		}
		raw, err := executeZooReadSouvenir(context.Background(), clientproto.ZooReadSouvenirRequest{SouvenirIds: clientproto.RPCIDList(ids)}, exec)
		if err != nil || applyCount != 1 || !json.Valid(raw) {
			t.Fatalf("read execution err=%v applyCount=%d raw=%s", err, applyCount, raw)
		}
	})

	t.Run("explicit map deletion", func(t *testing.T) {
		ids := []int32{32901}
		st := newZooSouvenirRunnerState(1, []int32{1}, ids)
		exec := zooReadSouvenirExecution{
			preflight: func() error { return nil },
			read: func(context.Context, clientproto.ZooReadSouvenirRequest) (json.RawMessage, error) {
				return json.RawMessage(`{"33":{"4":{"32901":null}}}`), nil
			},
			apply:        st.ApplyV,
			acknowledged: func() bool { return st.ZooSouvenirsAcknowledged(ids) },
		}
		if _, err := executeZooReadSouvenir(context.Background(), clientproto.ZooReadSouvenirRequest{SouvenirIds: clientproto.RPCIDList(ids)}, exec); err != nil {
			t.Fatalf("explicit deletion: %v", err)
		}
	})
}

func TestExecuteZooReadSouvenirRejectsStaleAndUnchangedResponses(t *testing.T) {
	t.Run("stale preflight", func(t *testing.T) {
		ids := []int32{32901}
		st := newZooSouvenirRunnerState(1, []int32{1}, nil)
		calls := 0
		exec := zooReadSouvenirExecution{
			preflight: func() error {
				if !st.ZooSouvenirsReadyToAcknowledge(ids) {
					return errors.New("preflight rejected: souvenir no longer unread")
				}
				return nil
			},
			read: func(context.Context, clientproto.ZooReadSouvenirRequest) (json.RawMessage, error) {
				calls++
				return nil, nil
			},
			acknowledged: func() bool { return st.ZooSouvenirsAcknowledged(ids) },
		}
		_, err := executeZooReadSouvenir(context.Background(), clientproto.ZooReadSouvenirRequest{SouvenirIds: clientproto.RPCIDList(ids)}, exec)
		if err == nil || !strings.Contains(err.Error(), "preflight rejected") || calls != 0 {
			t.Fatalf("stale preflight err=%v calls=%d", err, calls)
		}
	})

	t.Run("unchanged response", func(t *testing.T) {
		ids := []int32{32901}
		st := newZooSouvenirRunnerState(1, []int32{1}, ids)
		exec := zooReadSouvenirExecution{
			preflight: func() error { return nil },
			read: func(context.Context, clientproto.ZooReadSouvenirRequest) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			},
			apply:        st.ApplyV,
			acknowledged: func() bool { return st.ZooSouvenirsAcknowledged(ids) },
		}
		_, err := executeZooReadSouvenir(context.Background(), clientproto.ZooReadSouvenirRequest{SouvenirIds: clientproto.RPCIDList(ids)}, exec)
		if err == nil || !strings.Contains(err.Error(), "postcondition failed") {
			t.Fatalf("unchanged response error=%v", err)
		}
		assertZooSouvenirErrorCooldown(t, clientproto.RPCZooReadSouvenir.String(), "read", err)
	})
}

func newZooSouvenirRunnerState(count int, claimed []int32, unread []int32) *state.State {
	st := state.New()
	entries := make(map[string]any, count)
	unreadSet := make(map[int32]bool, len(unread))
	ids := make([]int32, 0, count)
	seen := make(map[int32]bool, count)
	for _, id := range unread {
		unreadSet[id] = true
		if id > 0 && !seen[id] && len(ids) < count {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, id := range []int32{30201, 32901, 32902, 32903, 32904} {
		if len(ids) >= count {
			break
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for i := 0; i < count; i++ {
		id := ids[i]
		isRead := int32(1)
		if unreadSet[id] {
			isRead = 0
		}
		entries[stateIDString(id)] = map[string]any{"1": id, "2": isRead}
	}
	st.ApplyVMap(map[string]any{"33": map[string]any{
		"0": map[string]any{"13": claimed},
		"4": entries,
	}})
	return st
}

func stateIDString(id int32) string {
	if id == 0 {
		return "0"
	}
	var buf [11]byte
	i := len(buf)
	n := id
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func assertZooSouvenirErrorCooldown(t *testing.T, kind, action string, err error) {
	t.Helper()
	runner := newOperationEventTestRunner()
	op := &automation.PlannedOp{
		OperationID: kind,
		Kind:        kind,
		Lane:        automation.LaneSide,
		Category:    automation.CategoryBasic,
		Domain:      "basic.zoo.souvenir",
		Action:      action,
	}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	got := runner.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              err,
		finishedAt:       now,
	})
	if !errors.Is(got, err) {
		t.Fatalf("handleOperationError=%v, want %v", got, err)
	}
	if _, cooling := runner.operationCoolingDown(op, now.Add(time.Second)); !cooling {
		t.Fatal("postcondition failure did not enter side-operation cooldown")
	}
}
