package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFmlRaceSparseNS25PreservesBatch(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":2000},"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}]}}`))
	before := s.FmlRace()
	if !before.Observed || !before.BatchActive || before.BatchID != 42 || len(before.Tasks) != 1 {
		t.Fatalf("seed race = %+v", before)
	}

	// Later guild update without race keys must not wipe race state.
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88},"133":{"1":88}}}`))
	got := s.FmlRace()
	if !got.Observed || !got.BatchActive || got.BatchID != 42 || len(got.Tasks) != 1 {
		t.Fatalf("sparse wipe destroyed race: %+v", got)
	}
}

func TestFmlRaceEmptyOrNullBatchDoesNotMarkObserved(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":null,"114":[],"110":{}}}`))
	got := s.FmlRace()
	if got.Observed {
		t.Fatalf("null/empty race stubs must not mark Observed: %+v", got)
	}

	s.ApplyV(json.RawMessage(`{"25":{"111":{}}}`))
	got = s.FmlRace()
	if got.Observed {
		t.Fatalf("empty batch object must not mark Observed: %+v", got)
	}
}

func TestFmlRaceParsesTimestampBatchID(t *testing.T) {
	s := New()
	// Observed production batchId is a millisecond timestamp, not a small int32.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000}}}`))
	got := s.FmlRace()
	if !got.Observed {
		t.Fatalf("timestamp batchId must mark Observed: %+v", got)
	}
	if got.BatchID != 1783872000000 {
		t.Fatalf("BatchID = %d, want 1783872000000", got.BatchID)
	}
	if got.BatchStatus != 1 || !got.BatchActive {
		t.Fatalf("expected active status=1 batch: %+v", got)
	}
}

func TestFmlRaceTasksObservedFromTaskList(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000}}}`))
	if s.FmlRace().TasksObserved {
		t.Fatal("tasks should not be observed before field 114")
	}
	// Observed getTaskList shape: large msId, array param/gain, catalog task id.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":178397176088900,"1":1,"2":1783872000000,"3":6100274,"4":4001,"5":1784081216561,"6":[23001],"7":280,"8":0,"9":270,"10":9,"11":[[1009,9]],"12":0,"13":null,"14":0,"15":0}]}}`))
	got := s.FmlRace()
	if !got.TasksObserved || len(got.Tasks) != 1 {
		t.Fatalf("tasks after getTaskList = %+v", got)
	}
	task := got.Tasks[0]
	if task.MsId != 178397176088900 || task.TaskId != 4001 || task.TaskType != 3036 || task.Score != 9 {
		t.Fatalf("parsed task = %+v", task)
	}
	if task.AppearTime != 1784081216561 {
		t.Fatalf("AppearTime = %d, want 1784081216561", task.AppearTime)
	}
	if task.ParamID != 23001 || task.TargetLabel != "白百合" {
		t.Fatalf("param detail = id=%d label=%q, want 23001/白百合", task.ParamID, task.TargetLabel)
	}
}

func TestFmlRaceTaskEmptyParamHasNoTargetLabel(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"6":[],"7":9,"10":9}]}}`))
	task := s.FmlRace().Tasks[0]
	if task.ParamID != 0 || task.TargetLabel != "" {
		t.Fatalf("empty param must stay blank: %+v", task)
	}
}

func TestFmlRaceUnresolvedFlowerUsesIDLabel(t *testing.T) {
	s := New()
	// 23562 exists in catalog with placeholder name "0" / short_name "待定".
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":915,"4":4028,"6":[23562],"10":25}]}}`))
	task := s.FmlRace().Tasks[0]
	if task.ParamID != 23562 {
		t.Fatalf("ParamID = %d, want 23562", task.ParamID)
	}
	if task.TargetLabel != "#23562" {
		t.Fatalf("TargetLabel = %q, want #23562 for unresolved flower name", task.TargetLabel)
	}
}

func TestFmlRaceTakenKeyedByBatchID(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000}}}`))
	// Observed map key is batchId, not uid.
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1783872000000":{"7":{"0":178397176088961,"1":4012,"2":600,"3":12,"4":[23363],"5":1784111927357}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 178397176088961 || got.Taken.TaskId != 4012 ||
		got.Taken.TargetCnt != 600 || got.Taken.FinishCnt != 12 || got.Taken.TaskType != 3036 {
		t.Fatalf("taken = %+v", got.Taken)
	}
	if got.Taken.ParamID != 23363 || got.Taken.TargetLabel == "" {
		t.Fatalf("taken param detail = id=%d label=%q", got.Taken.ParamID, got.Taken.TargetLabel)
	}
}

func TestFmlRaceTaskListMergesSparseDelta(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"6":[23001],"10":9,"12":0,"14":0,"15":0},{"0":2,"4":4001,"10":10,"12":0,"14":0,"15":0}]}}`))
	if len(s.FmlRace().Tasks) != 2 {
		t.Fatalf("seed pool = %+v", s.FmlRace().Tasks)
	}
	if s.FmlRace().Tasks[0].TargetLabel != "白百合" {
		t.Fatalf("seed target label = %q", s.FmlRace().Tasks[0].TargetLabel)
	}
	// takeTask-style sparse 114 containing only the changed row (no param).
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"10":9,"12":99,"14":0,"15":0}]}}`))
	got := s.FmlRace().Tasks
	if len(got) != 2 {
		t.Fatalf("sparse delta replaced pool: %+v", got)
	}
	if got[0].MsId != 1 || got[0].UID != 99 {
		t.Fatalf("task 1 not updated: %+v", got[0])
	}
	if got[0].ParamID != 23001 || got[0].TargetLabel != "白百合" {
		t.Fatalf("sparse delta wiped param detail: %+v", got[0])
	}
	if got[1].MsId != 2 || got[1].UID != 0 {
		t.Fatalf("task 2 not preserved: %+v", got[1])
	}
}

func TestFmlRaceTasksSyncedAtMsSetOnTaskList(t *testing.T) {
	s := New()
	before := time.Now().UnixMilli()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"6":[23001],"10":9}]}}`))
	after := time.Now().UnixMilli()
	got := s.FmlRace()
	if !got.TasksObserved {
		t.Fatal("expected TasksObserved")
	}
	if got.TasksSyncedAtMs < before || got.TasksSyncedAtMs > after {
		t.Fatalf("TasksSyncedAtMs=%d, want in [%d,%d]", got.TasksSyncedAtMs, before, after)
	}
}

func TestFmlRaceTasksSyncedAtMsSetOnEmptyPool(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":null}}`))
	got := s.FmlRace()
	if !got.TasksObserved || got.TasksSyncedAtMs <= 0 {
		t.Fatalf("empty/null pool must set TasksObserved + TasksSyncedAtMs, got %+v", got)
	}
}

func TestFmlRaceTakenSynthesizedFromPoolUID(t *testing.T) {
	s := New()
	// roleID=999; 110 empty map (no takeTaskData); pool row uid=999.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":178,"4":4012,"6":[23363],"7":600,"8":12,"10":25,"12":999}],"110":{}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask {
		t.Fatalf("expected Taken from pool UID, got %+v", got.Taken)
	}
	if got.Taken.TaskMsId != 178 || got.Taken.TaskId != 4012 || got.Taken.Score != 25 ||
		got.Taken.ParamID != 23363 || got.Taken.TargetCnt != 600 || got.Taken.FinishCnt != 12 {
		t.Fatalf("synthesized taken = %+v", got.Taken)
	}
	if got.Taken.TaskType == 0 {
		t.Fatalf("expected TaskType resolved, got %+v", got.Taken)
	}
}

func TestFmlRaceTakenSynthesizedAfterNull110WithPoolUID(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"7":{"0":1,"1":4012,"2":10,"3":1}}}}}`))
	if !s.FmlRace().Taken.HasTask {
		t.Fatal("seed taken missing")
	}
	// Same apply: null 110 clears, but 114 has UID==self → re-synthesize.
	s.ApplyV(json.RawMessage(`{"25":{"110":null,"114":[{"0":55,"4":4001,"6":[23001],"7":100,"8":0,"10":9,"12":999}]}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 55 {
		t.Fatalf("null 110 + pool UID must synthesize, got %+v", got.Taken)
	}
}

func TestFmlRaceTakenPrefers110OverPoolUID(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":55,"4":4001,"6":[23001],"7":100,"8":0,"10":9,"12":999}],"110":{"42":{"7":{"0":99,"1":4012,"2":50,"3":5,"4":[23363]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 99 {
		t.Fatalf("110 must win over pool UID, got %+v", got.Taken)
	}
}
