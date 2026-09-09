package runner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// TestRunnerAutomationE2E is explicitly opt-in: it consumes normal game
// resources through harvesting/planting/watering, existing orders and activity
// rewards. Never run it concurrently with another process using this account.
// Premium spending, shopping, cultivation/upgrades, guild-task mutations,
// guild replanting and displaced-session relogin remain off. Credentials and
// recoverable sessions use the normal encrypted store in an auto-cleaned temp
// directory; logs printed by this test contain no raw RPC/session payloads.
func TestRunnerAutomationE2E(t *testing.T) {
	username, password := os.Getenv("E2E_USERNAME"), os.Getenv("E2E_PASSWORD")
	if username == "" || password == "" || os.Getenv("E2E_AUTOMATION") != "1" {
		t.Skip("requires explicit E2E_USERNAME, E2E_PASSWORD and E2E_AUTOMATION=1")
	}
	duration := 5 * time.Minute
	if value := os.Getenv("E2E_AUTOMATION_DURATION"); value != "" {
		var err error
		duration, err = time.ParseDuration(value)
		if err != nil || duration < time.Minute || duration > 15*time.Minute {
			t.Fatal("E2E_AUTOMATION_DURATION must be 1m..15m")
		}
	}
	ctx := t.Context()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	owner, err := db.CreateUser(ctx, "e2e-owner", "e2e@example.test", "not-a-web-login")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, owner.ID, "iOS E2E", "ios", username, password)
	if err != nil {
		t.Fatal("could not create encrypted test account")
	}
	cfg, err := babigame.ConfigForChannel(babigame.ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	bus := NewBus()
	events, unsubscribe := bus.SubscribeLive(1000)
	defer unsubscribe()
	r := New(cfg, db, account, bus, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer r.Stop()
	startCtx, cancelStart := context.WithTimeout(ctx, 2*time.Minute)
	defer cancelStart()
	if err := r.Start(startCtx); err != nil {
		t.Fatalf("runner startup failed (%T); no credentials or server payload printed", err)
	}
	if !r.Connected() {
		t.Fatal("startup did not establish a live game session")
	}
	if len(r.State().Lands()) == 0 {
		t.Fatal("startup did not observe any land")
	}
	t.Logf("startup: connected=%t namespaces=%v lands=%d inventoryKinds=%d", r.Connected(), r.State().ObservedNamespaces(), len(r.State().Lands()), len(r.State().Inventory()))

	policy := policycfg.Normalize(automation.DefaultPolicy())
	policy.AutomationEnabled = true
	policy.Order.Customer.Enabled = true
	policy.Order.Resident.NormalEnabled = true
	policy.Order.Resident.NormalDailyLimit = 10
	policy.Union.Land.HarvestEnabled = true
	policy.Activity.CyclicNote.Enabled = true
	policy.Activity.CyclicNote.AutoClaimTaskRewards = true
	policy.Activity.CyclicNote.AutoClaimProgressBoxes = true
	policy.Activity.CyclicNote.SatisfyTasks = true
	policy.Activity.CyclicStory.Enabled = true
	policy.Activity.CyclicStory.AutoClaimOrderRewards = true
	policy.Activity.CyclicStory.AutoClaimProgressBoxes = true
	r.SetPolicy(policy)
	policyJSON, err := policycfg.ToJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, account.ID, policyJSON); err != nil {
		t.Fatal(err)
	}
	logE2EPlanner(t, r)
	logE2EActivityState(t, r)
	counts := map[string]int{}
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	report := time.NewTicker(30 * time.Second)
	defer report.Stop()
	running := true
	for running {
		select {
		case <-ctx.Done():
			t.Fatal("test canceled")
		case <-r.Done():
			t.Fatal("runner stopped unexpectedly")
		case <-deadline.C:
			running = false
		case <-report.C:
			t.Logf("progress: connected=%t eventCounts=%v", r.Connected(), counts)
			logE2EPlanner(t, r)
		case event := <-events:
			counts[event.Category+"/"+event.Action]++
			if event.Kind == "session_invalidated" || event.Kind == "account_request_paused" {
				t.Fatalf("stopping live test on account protection: %s", event.Kind)
			}
			if event.Level == "warn" || event.Level == "error" {
				t.Logf("runtime diagnostic: kind=%s domain=%s action=%s", event.Kind, event.Domain, event.Action)
			}
		}
	}
	paused := r.Policy()
	paused.AutomationEnabled = false
	r.SetPolicy(paused)
	// Drain the already-selected operation before verifying pause. The same
	// connection remains active; pause must not clear/recreate the game session.
	pausedOperations := drainE2EOperation(r)
	if !r.Connected() || r.Policy().GetAutomationEnabled() {
		t.Fatal("pause lost session or left automation enabled")
	}
	if !sleepOrDone(ctx, 20*time.Second) {
		t.Fatal("pause verification canceled")
	}
	if r.RuntimeStats().TotalOperations != pausedOperations {
		t.Fatal("automation executed after pause drained")
	}
	session := r.readTickSnapshot().session
	r.SetPolicy(policy)
	if r.readTickSnapshot().session != session {
		t.Fatal("policy resume replaced the game session")
	}
	if !sleepOrDone(ctx, 30*time.Second) {
		t.Fatal("resume verification canceled")
	}
	r.SetPolicy(paused)
	r.Stop()
	drainE2EOperation(r)
	if r.Connected() {
		t.Fatal("stop left a live connection")
	}
	logs, err := db.ListEventLogs(ctx, store.ListEventLogsOptions{AccountIDs: []int64{account.ID}, Limit: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Fatal("runtime events not persisted")
	}
	completed := map[string]int64{}
	stats := r.RuntimeStats()
	for _, operation := range stats.OperationCompletions {
		completed[operation.Key] = operation.Count
	}
	if stats.TotalOperations == 0 {
		t.Error("no successful automation operation; live execution was not verified")
	}
	for _, op := range automation.PlanOperations(r.state, policy, time.Now()) {
		if !runnablePlannedOp(op) {
			t.Logf("remaining gate: domain=%s reasons=%v", op.Domain, op.BlockedReasons)
		}
	}
	logE2EActivityState(t, r)
	fml := r.state.FmlBuild()
	t.Logf("guild: membershipObserved=%t joined=%t observedLand=%t lands=%d readyHarvest=%d", fml.MembershipObserved, fml.MemberFmlID > 0, r.state.FmlLandObserved(), len(r.state.FmlLands()), len(r.state.ReadyFmlLandHarvestIDs(time.Now())))
	cache, err := db.LoadSession(ctx, account.ID)
	if err != nil || len(cache) == 0 {
		t.Fatal("successful login did not persist recoverable encrypted session")
	}
	t.Logf("finished: duration=%v persistedEvents=%d completedDomains=%v pause/resume/stop=OK sessionCache=OK", duration, len(logs), completed)
}

func logE2EPlanner(t *testing.T, r *Runner) {
	t.Helper()
	ready, blocked := map[string]int{}, map[string]int{}
	for _, op := range automation.PlanOperations(r.state, r.Policy(), time.Now()) {
		if runnablePlannedOp(op) {
			ready[op.Domain]++
		} else {
			blocked[op.Domain]++
		}
	}
	note, _ := r.state.CyclicNoteView(time.Now())
	t.Logf("planner: ready=%v blocked=%v cyclicNote(found=%t valid=%t tasks=%d)", ready, blocked, note.Found, note.Valid, len(note.Tasks))
}

func drainE2EOperation(r *Runner) int64 {
	r.operationMu.Lock()
	defer r.operationMu.Unlock()
	// Acquiring the serialization lock waits for any in-flight operation.
	return r.RuntimeStats().TotalOperations
}

func logE2EActivityState(t *testing.T, r *Runner) {
	t.Helper()
	note, _ := r.state.CyclicNoteView(time.Now())
	t.Logf("activity baseline: phase=%d recordsObserved=%t listObserved=%t", note.Phase, note.TaskRecordObserved, note.TaskListObserved)
	for _, task := range note.Tasks {
		t.Logf("activity task: slot=%d type=%d progress=%d/%d progressObserved=%t receiptObserved=%t unlocked=%t received=%t catalogKnown=%t", task.SlotID, task.TaskType, task.Progress, task.Target, task.ProgressObserved, task.ReceiptObserved, task.Unlocked, task.Received, task.CatalogKnown)
	}
}
