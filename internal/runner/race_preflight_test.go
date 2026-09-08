package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestRacePreflightFailureYieldsToOtherTask(t *testing.T) {
	for _, kind := range []string{clientproto.RPCFmlRaceDelTask.String(), clientproto.RPCFmlRaceTakeTask.String()} {
		t.Run(kind, func(t *testing.T) {
			r := newOperationEventTestRunner()
			op := automation.PlannedOp{
				Kind: kind, OperationID: kind, CooldownKey: "race-task:1",
				Lane: automation.LaneSide, Category: automation.CategoryRace,
				Executable: true, Status: automation.PlanStatusManaged, TaskMsID: 1,
			}
			// A failed refresh must leave the same retry safeguards as an RPC
			// failure. No game client means this exercises the real early return.
			if err := r.executeOperation(context.Background(), nil, nil, &op, time.Now()); err == nil {
				t.Fatal("expected failed task-pool refresh")
			}
			now := time.Now()
			if _, cooling := r.operationCoolingDown(&op, now.Add(4*time.Second)); !cooling {
				t.Fatal("preflight failure must yield at least the next ordinary tick")
			}
			other := op
			other.OperationID = kind + ":2"
			other.CooldownKey = "race-task:2"
			other.TaskMsID = 2
			selected := r.selectRunnableOperation([]automation.PlannedOp{op, other}, now)
			if kind == clientproto.RPCFmlRaceDelTask.String() {
				if selected != nil {
					t.Fatalf("delete failure must retain account-wide interval: %+v", selected)
				}
			} else if selected == nil || selected.TaskMsID != 2 {
				t.Fatalf("failed preflight blocked another task: %+v", selected)
			}
			if _, cooling := r.operationCoolingDown(&op, now.Add(6*time.Second)); cooling {
				t.Fatal("preflight backoff must allow a later retry")
			}
		})
	}
}

func TestRaceTakeRechecksAfterRequestPacing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		initial  string
		update   string
		policyOn bool
		blocked  bool
	}{
		{name: "unchanged"},
		{name: "upgraded by other", update: `"14":1,"15":100`, blocked: true},
		{name: "upgrade ownership missing", update: `"14":1`, blocked: true},
		{name: "upgrade owner changed without score change", initial: `"14":1,"15":999`, update: `"14":1,"15":100`, blocked: true},
		{name: "taken by other", update: `"12":100`, blocked: true},
		{name: "exclusion enabled while queued", initial: `"14":1,"15":100`, policyOn: true, blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newOperationEventTestRunner()
				r.policy = automation.DefaultPolicy()
				r.policy.Union.Race.MinTaskScore = 0
				r.policy.Union.Race.ExcludeOthersUpgradeTask = !tc.policyOn
				r.policy.Union.Race.TaskTypePriority = map[int32]int32{3036: 5}
				r.state.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"101":{"0":{"23001":{"1":23001,"2":1,"4":2}}},"25":{"1":{"0":999,"1":42},"111":{"0":42,"1":1},"117":{"5":4},"110":{"999":{"0":999,"1":42,"3":0,"4":0}}}}`))
				applyTask := func(fields string) {
					if fields != "" {
						fields = "," + fields
					}
					r.state.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"6":[23001],"10":28` + fields + `}]}}`))
				}
				applyTask(tc.initial)
				op, err := automation.ManualRaceTakeOperation(r.state, r.Policy(), 1, time.Now())
				if err != nil {
					t.Fatal(err)
				}
				r.pacer = newRequestPacer(RequestPacing{})
				r.pacer.lastRequest = time.Now()
				// Exercise the real executor -> typed RPC -> Client.BeforeRPC
				// context path, but stop before any network/build operation.
				reachedTransport := errors.New("test transport boundary")
				client := babigame.NewClient(&babigame.Session{})
				client.BeforeRPC = func(ctx context.Context, name string) error {
					if err := r.beforeGameRPC(ctx, name); err != nil {
						return err
					}
					return reachedTransport
				}
				result := make(chan error, 1)
				go func() {
					_, err := r.executePlannedOp(t.Context(), client, nil, &op)
					result <- err
				}()
				synctest.Wait() // The mutation is now durably queued on its pacing timer.
				select {
				case err := <-result:
					t.Fatalf("operation did not wait for pacing: %v", err)
				default:
				}
				if tc.update != "" {
					applyTask(tc.update)
				}
				if tc.policyOn {
					p := r.Policy()
					p.Union.Race.ExcludeOthersUpgradeTask = true
					r.SetPolicy(p)
				}
				err = <-result
				if tc.blocked {
					if err == nil || !strings.Contains(err.Error(), "发送前校验未通过") {
						t.Fatalf("queued change reached transport: %v", err)
					}
				} else if !errors.Is(err, reachedTransport) {
					t.Fatalf("unchanged task rejected: %v", err)
				}
			})
		})
	}
}

func TestRaceSendGuardDoesNotLeakToOtherRPCs(t *testing.T) {
	r := newOperationEventTestRunner()
	// Invalid race operation would be rejected if accidentally checked.
	op := &automation.PlannedOp{Kind: clientproto.RPCFmlRaceTakeTask.String()}
	ctx := context.WithValue(t.Context(), raceMutationContextKey{}, op)
	for _, name := range []string{clientproto.RPCFmlRaceGetTaskList.String(), clientproto.RPCUsrHeartTick.String()} {
		if err := r.beforeGameRPC(ctx, name); err != nil {
			t.Fatalf("%s inherited mutation guard: %v", name, err)
		}
	}
	if err := r.beforeGameRPC(ctx, op.Kind); err == nil {
		t.Fatal("invalid mutation bypassed send guard")
	}
}
