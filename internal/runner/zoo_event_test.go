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

func TestExecuteZooHandleEventAppliesEachPayloadOnceAndReadsImmediately(t *testing.T) {
	var order []string
	applyCount := 0
	handled := false
	req := clientproto.ZooHandleEventRequest{PetId: 7, TableId: 42, Agree: true}
	exec := zooHandleEventExecution{
		preflight: func() error {
			order = append(order, "preflight")
			return nil
		},
		handle: func(_ context.Context, got clientproto.ZooHandleEventRequest) (json.RawMessage, error) {
			order = append(order, "handle")
			if got.PetId != 7 || got.TableId != 42 || !got.Agree || got.IsShareVideo != 0 {
				t.Fatalf("handle request=%+v", got)
			}
			return json.RawMessage(`{"33":{"2":{"7|42":{"7":1}}}}`), nil
		},
		read: func(_ context.Context, got clientproto.ZooReadLogRequest) (json.RawMessage, error) {
			order = append(order, "read")
			if got.PetId != 7 {
				t.Fatalf("read request=%+v", got)
			}
			return json.RawMessage(`{"33":{"1":{"7":{"19":2100}}}}`), nil
		},
		apply: func(raw json.RawMessage) {
			order = append(order, "apply")
			applyCount++
			if strings.Contains(string(raw), `"7":1`) {
				handled = true
			}
		},
		handled: func() bool {
			order = append(order, "postcondition")
			return handled
		},
	}

	raw, err := executeZooHandleEvent(context.Background(), req, exec)
	if err != nil {
		t.Fatalf("executeZooHandleEvent: %v", err)
	}
	if applyCount != 2 {
		t.Fatalf("payload apply count=%d, want exactly 2", applyCount)
	}
	if got, want := strings.Join(order, ","), "preflight,handle,apply,read,apply,postcondition"; got != want {
		t.Fatalf("execution order=%q, want %q", got, want)
	}
	if !handled || !json.Valid(raw) {
		t.Fatalf("post state/raw invalid: handled=%t raw=%s", handled, raw)
	}
}

func TestExecuteZooHandleEventAcceptsExplicitLogRemoval(t *testing.T) {
	st := newObservedType2ZooRunnerState()
	req := clientproto.ZooHandleEventRequest{PetId: 7, TableId: 42, Agree: true}
	exec := zooHandleEventExecution{
		preflight: func() error { return nil },
		handle: func(context.Context, clientproto.ZooHandleEventRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"33":{"2":{"server-entry":null}}}`), nil
		},
		read: func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"33":{"1":{"7":{"19":2100}}}}`), nil
		},
		apply:   st.ApplyV,
		handled: func() bool { return st.ZooLogHandled(7, 42) },
	}
	if _, err := executeZooHandleEvent(context.Background(), req, exec); err != nil {
		t.Fatalf("executeZooHandleEvent removal: %v", err)
	}
	if _, exists := st.ZooLogs()["server-entry"]; exists {
		t.Fatal("removed log still present")
	}
}

func TestExecuteZooHandleEventRejectsStaleSafetyAndUnchangedResponses(t *testing.T) {
	t.Run("observed type-2 shape is not executable", func(t *testing.T) {
		st := newObservedType2ZooRunnerState()
		calls := 0
		exec := zooHandleEventExecution{
			preflight: func() error {
				return zooHandleEventPreflight(st, clientproto.ZooHandleEventRequest{PetId: 7, TableId: 42, Agree: true})
			},
			handle: func(context.Context, clientproto.ZooHandleEventRequest) (json.RawMessage, error) {
				calls++
				return nil, nil
			},
			read: func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error) {
				calls++
				return nil, nil
			},
			handled: func() bool { return st.ZooLogHandled(7, 42) },
		}
		if _, err := executeZooHandleEvent(context.Background(), clientproto.ZooHandleEventRequest{PetId: 7, TableId: 42, Agree: true}, exec); err == nil || !strings.Contains(err.Error(), "已观测客户端 handleEvent 分支") {
			t.Fatalf("preflight error=%v", err)
		}
		if calls != 0 {
			t.Fatalf("RPC calls=%d after failed preflight", calls)
		}
	})

	t.Run("empty unchanged responses", func(t *testing.T) {
		exec := zooHandleEventExecution{
			preflight: func() error { return nil },
			handle: func(context.Context, clientproto.ZooHandleEventRequest) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			},
			read: func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			},
			apply:   func(json.RawMessage) {},
			handled: func() bool { return false },
		}
		_, err := executeZooHandleEvent(context.Background(), clientproto.ZooHandleEventRequest{PetId: 7, TableId: 42, Agree: true}, exec)
		if err == nil || !strings.Contains(err.Error(), "postcondition failed") {
			t.Fatalf("unchanged response error=%v", err)
		}

		runner := newOperationEventTestRunner()
		op := &automation.PlannedOp{
			OperationID: "zoo-handle-7-42",
			Kind:        clientproto.RPCZooHandleEvent.String(),
			Lane:        automation.LaneSide,
			Category:    automation.CategoryBasic,
			Domain:      "basic.zoo.event",
			Action:      "handle_event",
		}
		now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
		got := runner.handleOperationError(context.Background(), operationResult{
			operationAttempt: operationAttempt{op: op},
			err:              err,
			finishedAt:       now,
		})
		if !errors.Is(got, err) {
			t.Fatalf("handleOperationError=%v, want postcondition error", got)
		}
		if _, cooling := runner.operationCoolingDown(op, now.Add(time.Second)); !cooling {
			t.Fatal("postcondition failure did not enter side-operation cooldown")
		}
	})
}

func TestExecuteZooReadLogAppliesOnceAndRequiresStateChange(t *testing.T) {
	t.Run("read time advances", func(t *testing.T) {
		st := newUnreadZooRunnerState()
		var order []string
		applyCount := 0
		exec := zooReadLogExecution{
			preflight: zooRunnerReadPreflight(st, 7, 41, &order),
			read: func(_ context.Context, got clientproto.ZooReadLogRequest) (json.RawMessage, error) {
				order = append(order, "read")
				if got.PetId != 7 {
					t.Fatalf("read request=%+v", got)
				}
				return json.RawMessage(`{"33":{"1":{"7":{"19":1600}}}}`), nil
			},
			apply: func(raw json.RawMessage) {
				order = append(order, "apply")
				applyCount++
				st.ApplyV(raw)
			},
			readDone: func(createdAtMs int64) bool {
				order = append(order, "postcondition")
				return st.ZooLogRead(7, 41, createdAtMs)
			},
		}
		raw, err := executeZooReadLog(context.Background(), clientproto.ZooReadLogRequest{PetId: 7}, exec)
		if err != nil {
			t.Fatalf("executeZooReadLog: %v", err)
		}
		if applyCount != 1 {
			t.Fatalf("payload apply count=%d, want exactly 1", applyCount)
		}
		if got, want := strings.Join(order, ","), "preflight,read,apply,postcondition"; got != want {
			t.Fatalf("execution order=%q, want %q", got, want)
		}
		if !json.Valid(raw) || !st.ZooLogRead(7, 41, 1500) {
			t.Fatalf("post state/raw invalid: done=%t raw=%s", st.ZooLogRead(7, 41, 1500), raw)
		}
	})

	t.Run("explicit removal succeeds", func(t *testing.T) {
		st := newUnreadZooRunnerState()
		exec := zooReadLogExecution{
			preflight: zooRunnerReadPreflight(st, 7, 41, nil),
			read: func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error) {
				return json.RawMessage(`{"33":{"2":{"7|41":null}}}`), nil
			},
			apply:    st.ApplyV,
			readDone: func(createdAtMs int64) bool { return st.ZooLogRead(7, 41, createdAtMs) },
		}
		if _, err := executeZooReadLog(context.Background(), clientproto.ZooReadLogRequest{PetId: 7}, exec); err != nil {
			t.Fatalf("executeZooReadLog removal: %v", err)
		}
	})

	t.Run("unchanged response fails", func(t *testing.T) {
		st := newUnreadZooRunnerState()
		exec := zooReadLogExecution{
			preflight: zooRunnerReadPreflight(st, 7, 41, nil),
			read: func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error) {
				return json.RawMessage(`{}`), nil
			},
			apply:    st.ApplyV,
			readDone: func(createdAtMs int64) bool { return st.ZooLogRead(7, 41, createdAtMs) },
		}
		_, err := executeZooReadLog(context.Background(), clientproto.ZooReadLogRequest{PetId: 7}, exec)
		if err == nil || !strings.Contains(err.Error(), "postcondition failed") {
			t.Fatalf("unchanged read response error=%v", err)
		}

		runner := newOperationEventTestRunner()
		op := &automation.PlannedOp{
			OperationID: "zoo-read-7-41",
			Kind:        clientproto.RPCZooReadLog.String(),
			Lane:        automation.LaneSide,
			Category:    automation.CategoryBasic,
			Domain:      "basic.zoo.event",
			Action:      "read_log",
		}
		now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
		got := runner.handleOperationError(context.Background(), operationResult{
			operationAttempt: operationAttempt{op: op},
			err:              err,
			finishedAt:       now,
		})
		if !errors.Is(got, err) {
			t.Fatalf("handleOperationError=%v, want postcondition error", got)
		}
		if _, cooling := runner.operationCoolingDown(op, now.Add(time.Second)); !cooling {
			t.Fatal("readLog postcondition failure did not enter side-operation cooldown")
		}
	})

	t.Run("stale preflight prevents RPC", func(t *testing.T) {
		st := newUnreadZooRunnerState()
		st.ApplyVMap(map[string]any{"33": map[string]any{"1": map[string]any{"7": map[string]any{"19": 1600}}}})
		calls := 0
		exec := zooReadLogExecution{
			preflight: zooRunnerReadPreflight(st, 7, 41, nil),
			read: func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error) {
				calls++
				return nil, nil
			},
			readDone: func(createdAtMs int64) bool { return st.ZooLogRead(7, 41, createdAtMs) },
		}
		if _, err := executeZooReadLog(context.Background(), clientproto.ZooReadLogRequest{PetId: 7}, exec); err == nil || !strings.Contains(err.Error(), "preflight rejected") {
			t.Fatalf("stale preflight error=%v", err)
		}
		if calls != 0 {
			t.Fatalf("RPC calls=%d after stale read preflight", calls)
		}
	})
}

func newObservedType2ZooRunnerState() *state.State {
	st := state.New()
	st.ApplyVMap(map[string]any{"33": map[string]any{"2": map[string]any{
		"server-entry": map[string]any{
			"1": 7, "2": 42, "5": 2096, "6": 2, "7": 0,
			"8": map[string]any{}, "9": map[string]any{}, "10": map[string]any{}, "11": map[string]any{}, "13": int64(2000),
		},
	}}})
	return st
}

func newUnreadZooRunnerState() *state.State {
	st := state.New()
	st.ApplyVMap(map[string]any{"33": map[string]any{
		"1": map[string]any{"7": map[string]any{"1": 7, "19": int64(1000)}},
		"2": map[string]any{"7|41": map[string]any{
			"1": 7, "2": 41, "5": 2096, "7": 1, "13": int64(1500),
		}},
	}})
	return st
}

func zooRunnerReadPreflight(st *state.State, petID, index int32, order *[]string) func() (int64, error) {
	return func() (int64, error) {
		if order != nil {
			*order = append(*order, "preflight")
		}
		action, ok := st.ZooReadLogAction(petID, index)
		if !ok {
			return 0, errors.New("log missing")
		}
		if action.Blocked {
			return 0, errors.New("preflight rejected: " + action.BlockedReason)
		}
		return action.CreatedAtMs, nil
	}
}
