package babigame

import (
	"encoding/json"
	"slices"
)

// ActivityRefreshBatchIDs decodes the official client's bst act_refreshBatch
// notice. Only server-supplied positive IDs are returned; missing/empty lists
// are not treated as permission to probe arbitrary or all activity batches.
func ActivityRefreshBatchIDs(items []json.RawMessage) []int32 {
	var ids []int32
	for i, item := range items {
		var event struct {
			Name    string          `json:"name"`
			Type    string          `json:"t"`
			ID      string          `json:"i"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(item, &event) != nil {
			continue
		}
		content := event.Content
		if event.Name != "act_refreshBatch" {
			if (event.ID != "act_refreshBatch" && event.Type != "act_refreshBatch") || i+1 >= len(items) {
				continue
			}
			content = items[i+1]
		}
		if encoded := rawJSONString(content); encoded != "" {
			content = json.RawMessage(encoded)
		}
		var body struct {
			IDs []int32 `json:"refreshBatchIds"`
		}
		if json.Unmarshal(content, &body) != nil {
			continue
		}
		for _, id := range body.IDs {
			if id > 0 && len(ids) < 128 {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
