package runner

import (
	"encoding/json"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestNormalizeFmlRaceEnterVWrapsBareTaskList(t *testing.T) {
	bare := json.RawMessage(`{"114":[{"0":1,"4":4001,"6":[23001],"10":9}],"110":{}}`)
	got := normalizeFmlRaceEnterV(bare)
	var top map[string]json.RawMessage
	if err := json.Unmarshal(got, &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["25"]; !ok {
		t.Fatalf("expected wrap under 25, got %s", got)
	}
	already := json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"10":9}]}}`)
	if string(normalizeFmlRaceEnterV(already)) != string(already) {
		t.Fatal("namespaced payload must pass through")
	}
}

func TestNormalizeBareGetTaskListSetsTasksObserved(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000}}}`))
	if s.FmlRace().TasksObserved {
		t.Fatal("batch alone must not mark tasks observed")
	}

	bare := json.RawMessage(`{"114":[{"0":1,"4":4001,"6":[23001],"10":9}]}`)
	s.ApplyVFullFmlRaceTaskPool(bare)
	if s.FmlRace().TasksObserved {
		t.Fatal("unnormalized bare 114 must not observe race tasks")
	}

	s.ApplyVFullFmlRaceTaskPool(normalizeFmlRaceEnterV(bare))
	got := s.FmlRace()
	if !got.TasksObserved || len(got.Tasks) != 1 || got.Tasks[0].MsId != 1 {
		t.Fatalf("normalized bare getTaskList must observe pool: %+v", got)
	}
}
