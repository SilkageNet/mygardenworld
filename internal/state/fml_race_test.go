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
	// 23515 exists in catalog with placeholder name "0" / short_name "待定".
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":915,"4":4028,"6":[23515],"10":25}]}}`))
	task := s.FmlRace().Tasks[0]
	if task.ParamID != 23515 {
		t.Fatalf("ParamID = %d, want 23515", task.ParamID)
	}
	if task.TargetLabel != "#23515" {
		t.Fatalf("TargetLabel = %q, want #23515 for unresolved flower name", task.TargetLabel)
	}
}

func TestFmlRaceResolvedFlowerUsesCatalogName(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":915,"4":4028,"6":[23562],"10":25}]}}`))
	task := s.FmlRace().Tasks[0]
	if task.ParamID != 23562 {
		t.Fatalf("ParamID = %d, want 23562", task.ParamID)
	}
	if task.TargetLabel != "幽香绮囊" {
		t.Fatalf("TargetLabel = %q, want 幽香绮囊", task.TargetLabel)
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

func TestFmlRaceFullTaskPoolReplacesShorterSnapshot(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"6":[23001],"10":9},{"0":2,"4":4001,"6":[23002],"10":10}]}}`))

	// getTaskList is authoritative even when the refreshed pool shrank. Preserve
	// known detail on the retained row if that full snapshot happens to omit it.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":2,"4":4001,"10":10}]}}`))
	got := s.FmlRace().Tasks
	if len(got) != 1 || got[0].MsId != 2 {
		t.Fatalf("full shorter pool must replace stale rows: %+v", got)
	}
	if got[0].ParamID != 23002 {
		t.Fatalf("full refresh wiped retained task detail: %+v", got[0])
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

func TestFmlRaceFullPoolReplacesStaleTakenWithout110(t *testing.T) {
	s := New()
	// Stale Taken: 鹤望兰 (#23022), score unresolved — typical orphan after a
	// prior bad synthesize that sparse syncs never cleared.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":1,"4":4001,"6":[23022],"7":280,"8":0,"10":9,"12":999}],"110":{}}}`))
	if !s.FmlRace().Taken.HasTask || s.FmlRace().Taken.ParamID != 23022 {
		t.Fatalf("seed stale taken = %+v", s.FmlRace().Taken)
	}
	// Authoritative getTaskList: 鹤望兰 gone; 花笼流芳 (#23331) held by self; no 110.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":2,"4":4001,"6":[23331],"7":280,"8":12,"10":31,"12":999},{"0":3,"4":4001,"6":[23001],"7":100,"8":0,"10":20,"12":0}]}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 2 || got.Taken.ParamID != 23331 {
		t.Fatalf("full pool must replace stale Taken with pool UID row, got %+v", got.Taken)
	}
	if got.Taken.Score != 31 || got.Taken.FinishCnt != 12 || got.Taken.TargetLabel != "花笼流芳" {
		t.Fatalf("replaced taken detail = %+v", got.Taken)
	}
}

func TestFmlRaceFullPoolClearsOrphanTakenWhenNoPoolUID(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":1,"4":4001,"6":[23022],"7":280,"8":0,"10":9,"12":999}],"110":{}}}`))
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":3,"4":4001,"6":[23001],"7":100,"8":0,"10":20,"12":0}]}}`))
	if s.FmlRace().Taken.HasTask {
		t.Fatalf("full pool with no UID==self must clear orphan Taken, got %+v", s.FmlRace().Taken)
	}
}

func TestFmlRaceFullPoolKeeps110TakenOverPoolUID(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"7":{"0":99,"1":4001,"2":50,"3":5,"4":[23022]}}}}}`))
	// Same apply-style full pool with conflicting UID row + fresh 110 for 鹤望兰.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"114":[{"0":2,"4":4001,"6":[23331],"7":280,"8":0,"10":31,"12":999}],"110":{"42":{"7":{"0":99,"1":4001,"2":50,"3":5,"4":[23022]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 99 || got.Taken.ParamID != 23022 {
		t.Fatalf("110 in full-pool apply must still win, got %+v", got.Taken)
	}
}

func TestFmlRaceTakenEnrichedTargetCntFromPool(t *testing.T) {
	s := New()
	// 110 has takeTaskData with TargetCnt=0/FinishCnt=0 (server omitted fields 2/3);
	// pool row has UID=self with TargetCnt=600, FinishCnt=12. Enrichment must
	// backfill TargetCnt/FinishCnt/TaskType from pool.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":178,"4":4012,"6":[23363],"7":600,"8":12,"10":25,"12":999}],"110":{"42":{"7":{"0":178,"1":4012,"4":[23363]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask {
		t.Fatalf("expected HasTask, got %+v", got.Taken)
	}
	if got.Taken.TargetCnt != 600 {
		t.Fatalf("TargetCnt = %d, want 600 (enriched from pool)", got.Taken.TargetCnt)
	}
	if got.Taken.FinishCnt != 12 {
		t.Fatalf("FinishCnt = %d, want 12 (enriched from pool)", got.Taken.FinishCnt)
	}
	if got.Taken.ParamID != 23363 {
		t.Fatalf("ParamID = %d, want 23363", got.Taken.ParamID)
	}
	if got.Taken.Score != 25 {
		t.Fatalf("Score = %d, want 25 (enriched from pool)", got.Taken.Score)
	}
}

func TestMarkFmlRaceTasksUnobserved(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"10":9}]}}`))
	if !s.FmlRace().TasksObserved {
		t.Fatal("expected observed")
	}
	s.MarkFmlRaceTasksUnobserved()
	got := s.FmlRace()
	if got.TasksObserved {
		t.Fatal("expected TasksObserved=false")
	}
	// Pool rows preserved so UI/planner still see last snapshot until re-sync.
	if len(got.Tasks) != 1 {
		t.Fatalf("tasks wiped: %+v", got.Tasks)
	}
}

func TestFmlRaceUsrRcdTaskQuota(t *testing.T) {
	s := New()
	// batchId key; fTaskNum=3, buyTaskNum=2, no taken task.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"0":{"103":4},"110":{"42":{"0":99,"1":42,"3":3,"6":2}}}}`))
	got := s.FmlRace()
	if !got.TaskQuotaObserved {
		t.Fatalf("TaskQuotaObserved=false: %+v", got)
	}
	if got.FinishedTaskNum != 3 || got.BuyTaskNum != 2 {
		t.Fatalf("quota finished=%d buy=%d, want 3/2", got.FinishedTaskNum, got.BuyTaskNum)
	}
	if got.Taken.HasTask {
		t.Fatalf("unexpected taken: %+v", got.Taken)
	}
	if s.FmlBuild().RaceLvl != 4 {
		t.Fatalf("RaceLvl=%d, want 4", s.FmlBuild().RaceLvl)
	}
	if total := FmlRaceTotalTaskNum(s.FmlBuild().RaceLvl, got.BuyTaskNum); total != 18 {
		// c_fmlRace(4).taskNum=18 (buyTaskNum not included in displayed total)
		t.Fatalf("total=%d, want 18", total)
	}
}

func TestFmlRaceCurRcdRaceLvl(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"117":{"0":42,"1":7,"5":4},"110":{"42":{"3":6,"6":0}}}}`))
	got := s.FmlRace()
	if got.RaceLvl != 4 || !got.RaceLvlObserved {
		t.Fatalf("RaceLvl=%d observed=%v, want 4/true from CurFmlRaceRcd", got.RaceLvl, got.RaceLvlObserved)
	}
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 6 {
		t.Fatalf("quota=%+v", got)
	}
	if total := FmlRaceTotalTaskNum(got.RaceLvl, got.BuyTaskNum); total != 18 {
		t.Fatalf("total=%d, want 18", total)
	}
}

func TestFmlRaceGroupRcdRaceLvl(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":7},"111":{"0":42,"1":1,"2":1000,"3":9000},"112":[{"0":42,"1":7,"5":4},{"0":42,"1":8,"5":4}]}}`))
	got := s.FmlRace()
	if got.RaceLvl != 4 {
		t.Fatalf("RaceLvl=%d, want 4 from group list fid match", got.RaceLvl)
	}
}

func TestFmlRaceUsrRcdTaskQuotaPreservedWithoutTaken(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"3":5,"6":1,"7":{"0":9,"1":4001,"2":3,"3":1}}}}}`))
	got := s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 5 || !got.Taken.HasTask {
		t.Fatalf("with taken: %+v", got)
	}
	// Later 110 without takeTaskData still updates quota and clears taken.
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":6,"6":1}}}}`))
	got = s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 6 || got.Taken.HasTask {
		t.Fatalf("after finish clear: %+v", got)
	}
}
