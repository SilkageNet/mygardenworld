package runner

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestOperationPayloadBoundsAndSyncSummary(t *testing.T) {
	for _, tt := range []struct {
		name, action, raw, omitted string
		failure                    bool
	}{
		{"sync snapshot", "sync", `{"state":"large snapshot"}`, "successful_sync", false},
		{"mutation evidence", "buy", `{"cost":100}`, "", false},
		{"failed sync evidence", "sync", `{"code":5000}`, "", true},
		{"oversized evidence", "buy", `{"data":"` + strings.Repeat("x", 40<<10) + `"}`, "size_limit", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.failure {
				err = errors.New("server rejected request")
			}
			payload := operationPayload(&automation.PlannedOp{Kind: "test", Action: tt.action}, map[string]int{"count": 1}, json.RawMessage(tt.raw), err)
			var got map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatal("invalid JSON")
			}
			if tt.omitted != "" {
				if got["rawOmitted"] != tt.omitted || got["raw"] != nil {
					t.Fatal(got)
				}
			} else if got["raw"] == nil {
				t.Fatal("lost diagnostic evidence")
			}
			if len(payload) > 34<<10 {
				t.Fatalf("unbounded payload: %d", len(payload))
			}
			if tt.failure && got["error"] == nil {
				t.Fatal("lost error")
			}
		})
	}
}

func TestSuccessfulOperationHasOneDurableRecord(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	u, err := db.CreateUser(ctx, "owner", "o@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "main", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}
	r := newOperationEventTestRunner()
	r.account = a
	r.db = db
	r.handleOperationSuccess(ctx, operationResult{operationAttempt: operationAttempt{op: &automation.PlannedOp{Kind: "test.success", Action: "buy"}}, raw: json.RawMessage(`{"cost":100}`), finishedAt: time.Now()})
	var events, operations int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM event_log`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM operation_log`).Scan(&operations); err != nil {
		t.Fatal(err)
	}
	if events != 1 || operations != 0 {
		t.Fatalf("events=%d operations=%d", events, operations)
	}
}

func TestInventoryLogSizeDoesNotGrowWithWarehouse(t *testing.T) {
	snapshot := state.InventorySnapshot{Inventory: make(map[int32]int32), Changes: []state.InventoryItemDelta{{ItemID: 1501, Before: 3, After: 2}}}
	for id := int32(1); id <= 10000; id++ {
		snapshot.Inventory[id] = 999
	}
	payload := inventoryChangePayload(snapshot)
	if len(payload) > 200 || strings.Contains(payload, "inventory") {
		t.Fatalf("full inventory leaked into log: %d bytes", len(payload))
	}
	var actual struct {
		Changes []state.InventoryItemDelta `json:"changes"`
	}
	if err := json.Unmarshal([]byte(payload), &actual); err != nil || len(actual.Changes) != 1 || actual.Changes[0] != snapshot.Changes[0] {
		t.Fatal("lost observed delta")
	}
	if len(snapshot.Inventory) != 10000 {
		t.Fatal("mutated authoritative state")
	}
}
