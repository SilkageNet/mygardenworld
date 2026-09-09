package babigame

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestActivityRefreshBatchIDs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []json.RawMessage
		want  []int32
	}{
		{"bst encoded content", []json.RawMessage{json.RawMessage(`{"name":"act_refreshBatch","content":"{\"refreshBatchIds\":[9002,9001,9002]}"}`)}, []int32{9001, 9002}},
		{"header body", []json.RawMessage{json.RawMessage(`{"t":"notify","i":"act_refreshBatch"}`), json.RawMessage(`{"refreshBatchIds":[9001]}`)}, []int32{9001}},
		{"unrelated", []json.RawMessage{json.RawMessage(`{"name":"usr_kick","content":{"refreshBatchIds":[9001]}}`)}, nil},
		{"no guessing", []json.RawMessage{json.RawMessage(`{"name":"act_refreshBatch","content":{}}`)}, nil},
		{"invalid", []json.RawMessage{json.RawMessage(`{"name":"act_refreshBatch","content":{"refreshBatchIds":[0,-1]}}`)}, nil},
		{"wrong type", []json.RawMessage{json.RawMessage(`{"name":"act_refreshBatch","content":{"refreshBatchIds":["9001"]}}`)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ActivityRefreshBatchIDs(tc.items); !slices.Equal(got, tc.want) {
				t.Fatalf("ids=%v want=%v", got, tc.want)
			}
		})
	}
}
