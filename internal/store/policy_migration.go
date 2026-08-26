package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const migratedPolicySchemaVersion = 1

type policyV2Update struct {
	accountID int64
	json      string
}

// migratePoliciesV2 is the only bridge from schema-v1 policy documents to the
// strict policy schema. Runtime decoding deliberately contains no legacy
// aliases or missing-field backfills after this migration succeeds.
func migratePoliciesV2(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT account_id, policy_json FROM account_policies ORDER BY account_id`)
	if err != nil {
		return fmt.Errorf("read policies: %w", err)
	}
	var updates []policyV2Update
	for rows.Next() {
		var (
			accountID int64
			raw       string
		)
		if err := rows.Scan(&accountID, &raw); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan policy: %w", err)
		}
		migrated, err := migratePolicyDocumentV2(raw)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("account %d: %w", accountID, err)
		}
		updates = append(updates, policyV2Update{accountID: accountID, json: migrated})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate policies: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close policies: %w", err)
	}
	for _, update := range updates {
		if _, err := tx.ExecContext(ctx,
			`UPDATE account_policies SET policy_json = ?, updated_at = CURRENT_TIMESTAMP WHERE account_id = ?`,
			update.json, update.accountID,
		); err != nil {
			return fmt.Errorf("write account %d policy: %w", update.accountID, err)
		}
	}
	return nil
}

func migratePolicyDocumentV2(raw string) (string, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return "", fmt.Errorf("decode policy JSON: %w", err)
	}

	if basic := childObject(root, "basic"); basic != nil {
		if legacy := childObject(basic, "friend_touch"); legacy != nil {
			plant := ensureChildObject(root, "plant")
			friendSteal := ensureChildObject(plant, "friend_steal")
			copyMissingPolicyFields(friendSteal, legacy, map[string]string{
				"enabled":            "enabled",
				"steal_elves":        "steal_elves",
				"friend_counts":      "friend_counts",
				"auto_buy_times":     "auto_buy_times",
				"max_buy_per_friend": "max_buy_per_friend",
				"mode":               "friend_mode",
				"exclude_uids":       "exclude_uids",
			})
		}
		delete(basic, "friend_touch")
	}

	if plant := childObject(root, "plant"); plant != nil {
		if planting := childObject(plant, "planting"); planting != nil {
			if _, exists := planting["auto_harvest_enabled"]; !exists && boolValue(planting["auto_enabled"]) {
				planting["auto_harvest_enabled"] = true
			}
		}
		if friendSteal := childObject(plant, "friend_steal"); friendSteal != nil {
			delete(friendSteal, "buy_count")
			delete(friendSteal, "max_spend_diamond")
		}
	}

	if union := childObject(root, "union"); union != nil {
		if race := childObject(union, "race"); race != nil {
			if _, exists := race["auto_stop_on_quota_done"]; !exists {
				race["auto_stop_on_quota_done"] = true
			}
			if _, exists := race["min_task_score"]; !exists {
				if value, legacyExists := race["max_task_score"]; legacyExists {
					race["min_task_score"] = value
				}
			}
			delete(race, "max_task_score")
			delete(race, "urgent_speedup_enabled")
		}
	}

	if activity := childObject(root, "activity"); activity != nil {
		delete(activity, "enabled")
	}
	root["schema_version"] = migratedPolicySchemaVersion

	data, err := json.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("encode policy JSON: %w", err)
	}
	var policy pb.Policy
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(data, &policy); err != nil {
		return "", fmt.Errorf("validate migrated policy: %w", err)
	}
	if policy.GetSchemaVersion() != migratedPolicySchemaVersion {
		return "", fmt.Errorf("validate migrated policy: schema version %d", policy.GetSchemaVersion())
	}
	return string(data), nil
}

func childObject(parent map[string]any, key string) map[string]any {
	child, _ := parent[key].(map[string]any)
	return child
}

func ensureChildObject(parent map[string]any, key string) map[string]any {
	if child := childObject(parent, key); child != nil {
		return child
	}
	child := map[string]any{}
	parent[key] = child
	return child
}

func copyMissingPolicyFields(dst, src map[string]any, fields map[string]string) {
	for sourceKey, targetKey := range fields {
		if _, exists := dst[targetKey]; exists {
			continue
		}
		if value, exists := src[sourceKey]; exists {
			dst[targetKey] = value
		}
	}
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
