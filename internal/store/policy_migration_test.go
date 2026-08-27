package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/policycfg"
)

func TestMigratePolicyDocumentV2ProducesStrictCurrentPolicy(t *testing.T) {
	raw := `{
		"automation_enabled":true,
		"basic":{"friend_touch":{"enabled":true,"mode":"SELECTION_MODE_SPECIFIC","friend_counts":{"2001":3}}},
		"plant":{"planting":{"auto_enabled":true},"friend_steal":{"buy_count":5,"max_spend_diamond":"100"}},
		"union":{"race":{"max_task_score":19,"urgent_speedup_enabled":true}},
		"activity":{"enabled":true,"modules":{
			"cyclicNote":{"enabled":true,"bool_params":{"satisfy_tasks":true}},
			"actCyclicStory":{"enabled":true,"int_params":{"max_score":"88"}}
		}}
	}`
	migrated, err := migratePolicyDocumentV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(migrated), &document); err != nil {
		t.Fatal(err)
	}
	if got := uint32(document["schema_version"].(float64)); got != migratedPolicySchemaVersionV2 {
		t.Fatalf("schema_version=%d want %d", got, migratedPolicySchemaVersionV2)
	}
	current, err := migratePolicyDocumentV3(migrated)
	if err != nil {
		t.Fatalf("v3 migration rejected v2 document: %v", err)
	}
	policy, err := policycfg.FromJSON(current)
	if err != nil {
		t.Fatalf("strict decoder rejected migrated document: %v\n%s", err, migrated)
	}
	friend := policy.GetPlant().GetFriendSteal()
	if !friend.GetEnabled() || friend.GetFriendMode().String() != "SELECTION_MODE_SPECIFIC" || friend.GetFriendCounts()[2001] != 3 {
		t.Fatalf("friend policy not migrated: %+v", friend)
	}
	if !policy.GetPlant().GetPlanting().GetAutoHarvestEnabled() {
		t.Fatal("auto harvest backfill missing")
	}
	if race := policy.GetUnion().GetRace(); race.GetMinTaskScore() != 19 || !race.GetAutoStopOnQuotaDone() {
		t.Fatalf("race policy not migrated: %+v", race)
	}
	if !policy.GetActivity().GetCyclicNote().GetSatisfyTasks() ||
		policy.GetActivity().GetCyclicStory().GetMaxScore() != 88 {
		t.Fatalf("typed activity policy not migrated: %+v", policy.GetActivity())
	}
}

func TestMigratePolicyDocumentV3RemovesOnlyRetiredDessertPolicy(t *testing.T) {
	raw := `{
		"schema_version":2,
		"automation_enabled":true,
		"activity":{
			"cyclic_note":{"enabled":true,"satisfy_tasks":true},
			"dessert":{"enabled":true,"max_session_energy":20}
		}
	}`
	migrated, err := migratePolicyDocumentV3(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(migrated), "dessert") {
		t.Fatalf("retired policy survived migration: %s", migrated)
	}
	policy, err := policycfg.FromJSON(migrated)
	if err != nil {
		t.Fatalf("strict decoder rejected migrated document: %v\n%s", err, migrated)
	}
	if !policy.GetAutomationEnabled() || !policy.GetActivity().GetCyclicNote().GetEnabled() || !policy.GetActivity().GetCyclicNote().GetSatisfyTasks() {
		t.Fatalf("unrelated policy values changed: %+v", policy)
	}
}

func TestMigratePolicyDocumentV3RejectsOtherUnknownFields(t *testing.T) {
	_, err := migratePolicyDocumentV3(`{"schema_version":2,"removed_product_switch":true,"activity":{"dessert":{}}}`)
	if err == nil || !strings.Contains(err.Error(), "removed_product_switch") {
		t.Fatalf("migration error=%v, want strict unknown-field rejection", err)
	}
}

func TestMigratePolicyDocumentV2RejectsUnmappedUnknownFields(t *testing.T) {
	_, err := migratePolicyDocumentV2(`{"automation_enabled":true,"removed_product_switch":true}`)
	if err == nil || !strings.Contains(err.Error(), "removed_product_switch") {
		t.Fatalf("migration error=%v, want strict unknown-field rejection", err)
	}
}
