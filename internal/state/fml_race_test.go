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

func TestFmlRacePoolMissingParamCoversExecutionTargets(t *testing.T) {
	tests := []struct {
		name  string
		tasks []FmlRaceTaskView
		want  bool
	}{
		{name: "plant target missing", tasks: []FmlRaceTaskView{{TaskType: 3036}}, want: true},
		{name: "craft vase missing", tasks: []FmlRaceTaskView{{TaskType: 3034}}, want: true},
		{name: "all targets known", tasks: []FmlRaceTaskView{{TaskType: 3036, ParamID: 23001}, {TaskType: 3034, ParamID: 3002}}},
		{name: "targetless type", tasks: []FmlRaceTaskView{{TaskType: 3030}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FmlRacePoolMissingParam(tc.tasks); got != tc.want {
				t.Fatalf("FmlRacePoolMissingParam()=%v, want %v", got, tc.want)
			}
		})
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
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":178,"4":4012,"6":[23363],"7":600,"8":12,"10":25,"12":999,"13":1785368365572}],"110":{}}}`))
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
	if got.Taken.ExpireTime != 1785368365572 {
		t.Fatalf("ExpireTime=%d, want takeExpireTime from pool field 13", got.Taken.ExpireTime)
	}
}

func TestFmlRaceTakenExpireTimeFrom110(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577],"5":1785368365572}}}}}`))
	got := s.FmlRace().Taken
	if !got.HasTask || got.ExpireTime != 1785368365572 {
		t.Fatalf("taken ExpireTime from 110 = %+v", got)
	}
	if got.TakenAtMs <= 0 {
		t.Fatalf("TakenAtMs should be stamped, got %+v", got)
	}
}

func TestFmlRaceTakenExpireTimeFromTakeLimitMin(t *testing.T) {
	s := New()
	// No protocol expireTime; pool carries takeLimitMin=270. Deadline = TakenAtMs + 270m.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"9":270,"10":28,"12":999,"13":null}],"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	got := s.FmlRace().Taken
	if !got.HasTask || got.TaskMsId != 715 {
		t.Fatalf("expected taken task, got %+v", got)
	}
	if got.TakeLimitMin != 270 {
		t.Fatalf("TakeLimitMin=%d, want 270", got.TakeLimitMin)
	}
	if got.TakenAtMs <= 0 {
		t.Fatalf("TakenAtMs unset: %+v", got)
	}
	wantExpire := got.TakenAtMs + int64(270)*int64(time.Minute/time.Millisecond)
	if got.ExpireTime != wantExpire {
		t.Fatalf("ExpireTime=%d, want TakenAtMs+270m=%d", got.ExpireTime, wantExpire)
	}
	// Re-apply 110 without wipe: TakenAtMs / ExpireTime must stick.
	prevAt, prevExpire := got.TakenAtMs, got.ExpireTime
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":60,"4":[23577]}}}}}`))
	got = s.FmlRace().Taken
	if got.TakenAtMs != prevAt || got.ExpireTime != prevExpire {
		t.Fatalf("deadline must persist across 110 refresh: before at=%d exp=%d after %+v", prevAt, prevExpire, got)
	}
	if got.FinishCnt != 60 {
		t.Fatalf("FinishCnt=%d, want 60", got.FinishCnt)
	}
}

func TestFmlRaceProtocolExpireWinsOverComputed(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"9":270,"10":28,"12":999}],"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	computed := s.FmlRace().Taken.ExpireTime
	if computed <= 0 {
		t.Fatal("expected computed expire before protocol")
	}
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{"0":715,"1":4013,"2":300,"3":48,"4":[23577],"5":1785368365572},"4":1}}}}`))
	got := s.FmlRace().Taken
	if got.ExpireTime != 1785368365572 {
		t.Fatalf("protocol ExpireTime should win, got %d (computed was %d)", got.ExpireTime, computed)
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

func TestFmlRaceTakenPrefersPoolUIDOver110(t *testing.T) {
	s := New()
	// Sparse apply with conflicting 110 (花笼流芳 msId 99) vs pool UID row (白百合 msId 55).
	// Live holder is the pool UID==self row.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":55,"4":4001,"6":[23001],"7":100,"8":0,"10":9,"12":999}],"110":{"42":{"7":{"0":99,"1":4012,"2":50,"3":5,"4":[23363]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 55 || got.Taken.ParamID != 23001 {
		t.Fatalf("pool UID==self must win over 110, got %+v", got.Taken)
	}
}

func TestFmlRaceSameMsIDKeeps110ProgressOverPool(t *testing.T) {
	s := New()
	// Matching msId: pool omits progress; 110 has FinishCnt=2/TargetCnt=10 — keep 110.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":99,"4":4001,"6":[23001],"10":30,"12":999}],"110":{"42":{"7":{"0":99,"1":4001,"2":10,"3":2,"4":[23001]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 99 {
		t.Fatalf("taken = %+v", got.Taken)
	}
	if got.Taken.TargetCnt != 10 || got.Taken.FinishCnt != 2 {
		t.Fatalf("must keep 110 progress, got target=%d finish=%d", got.Taken.TargetCnt, got.Taken.FinishCnt)
	}
	if got.Taken.Score != 30 {
		t.Fatalf("Score = %d, want 30 from pool gap-fill", got.Taken.Score)
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

func TestFmlRaceFullPoolPoolUIDOverridesStale110(t *testing.T) {
	s := New()
	// Stale 110 still asserts 鹤望兰 (#23022) while the live holder in the
	// authoritative pool is 花笼流芳 (#23331). Full-pool reconcile must prefer
	// UID==self over the orphan 110 takeTaskData (UI score-0 ghost).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"7":{"0":99,"1":4001,"2":280,"3":0,"4":[23022]}}}}}`))
	if !s.FmlRace().Taken.HasTask || s.FmlRace().Taken.ParamID != 23022 {
		t.Fatalf("seed stale 110 taken = %+v", s.FmlRace().Taken)
	}
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"114":[{"0":2,"4":4001,"6":[23331],"7":280,"8":12,"10":31,"12":999},{"0":3,"4":4001,"6":[23001],"7":100,"8":0,"10":20,"12":0}],"110":{"42":{"7":{"0":99,"1":4001,"2":280,"3":0,"4":[23022]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 2 || got.Taken.ParamID != 23331 {
		t.Fatalf("full pool UID==self must override stale 110, got %+v", got.Taken)
	}
	if got.Taken.Score != 31 || got.Taken.FinishCnt != 12 || got.Taken.TargetLabel != "花笼流芳" {
		t.Fatalf("overridden taken detail = %+v", got.Taken)
	}
}

func TestFmlRaceFullPoolClearsStale110WhenNoPoolUID(t *testing.T) {
	s := New()
	// Authoritative pool has no UID==self, but 110 still asserts 鹤望兰.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":3,"4":4001,"6":[23001],"7":100,"8":0,"10":20,"12":0}],"110":{"42":{"7":{"0":99,"1":4001,"2":280,"3":0,"4":[23022]}}}}}`))
	if s.FmlRace().Taken.HasTask {
		t.Fatalf("full pool with stale 110 and no UID==self must clear Taken, got %+v", s.FmlRace().Taken)
	}
}

func TestFmlRaceTakenFinishCntAdvancesFromPool(t *testing.T) {
	s := New()
	// 110 starts at FinishCnt=48; later full pool row (UID=0) reaches 300.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}],"110":{"42":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	if s.FmlRace().Taken.FinishCnt != 48 {
		t.Fatalf("seed FinishCnt=%d, want 48", s.FmlRace().Taken.FinishCnt)
	}
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":300,"10":28,"12":0}],"110":{"42":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 715 {
		t.Fatalf("must keep live Taken (pool UID=0), got %+v", got.Taken)
	}
	if got.Taken.FinishCnt != 300 {
		t.Fatalf("FinishCnt=%d, want 300 advanced from pool", got.Taken.FinishCnt)
	}
}

func TestFmlRaceFullPoolKeepsTakenWhenMsIDStillInPoolUIDZero(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"7":{"0":715,"1":4013,"2":300,"3":128,"4":[23577]}}}}}`))
	// Authoritative pool has the same msId but UID=0 (no UID==self row).
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":214,"10":28,"12":0},{"0":3,"4":4001,"6":[23001],"7":100,"8":0,"10":20,"12":0}],"110":{"42":{"7":{"0":715,"1":4013,"2":300,"3":128,"4":[23577]}}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 715 {
		t.Fatalf("full pool must not clear Taken still present with UID=0, got %+v", got.Taken)
	}
	if got.Taken.FinishCnt != 214 {
		t.Fatalf("FinishCnt=%d, want 214 from pool", got.Taken.FinishCnt)
	}
}

func TestFmlRaceTakenProgressFromField134OnHarvest(t *testing.T) {
	s := New()
	// Seed held plant-harvest task at 48/300.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	if s.FmlRace().Taken.FinishCnt != 48 {
		t.Fatalf("seed FinishCnt=%d, want 48", s.FmlRace().Taken.FinishCnt)
	}
	// Harvest ACK shape: 25.134 batch map with takeTaskData at field 3.
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{"0":715,"1":4013,"2":300,"3":300,"4":[23577],"5":1785368365572},"4":1785358559363}}}}`))
	got := s.FmlRace()
	if !got.Taken.HasTask || got.Taken.TaskMsId != 715 {
		t.Fatalf("134 must keep Taken, got %+v", got.Taken)
	}
	if got.Taken.FinishCnt != 300 || got.Taken.TargetCnt != 300 {
		t.Fatalf("134 progress = %d/%d, want 300/300", got.Taken.FinishCnt, got.Taken.TargetCnt)
	}
	if got.Taken.ParamID != 23577 {
		t.Fatalf("ParamID=%d, want 23577", got.Taken.ParamID)
	}
	if got.Taken.ExpireTime != 1785368365572 {
		t.Fatalf("ExpireTime=%d, want 1785368365572 from takeTaskData field 5", got.Taken.ExpireTime)
	}
	if got.LocalFinishCnt != 300 || got.LocalFinishTaskMsId != 715 {
		t.Fatalf("LocalFinish=%d msId=%d, want 300/715", got.LocalFinishCnt, got.LocalFinishTaskMsId)
	}
}

func TestFmlRaceFullPoolClampsLocalFinishWhenServerStillShort(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}},"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}]}}`))
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{"0":715,"1":4013,"2":300,"3":300,"4":[23577],"5":1},"4":1}}}}`))
	if got := s.FmlRace(); got.LocalFinishCnt < 300 || got.Taken.FinishCnt != 300 {
		t.Fatalf("seed after 134 local=%d finish=%d", got.LocalFinishCnt, got.Taken.FinishCnt)
	}
	// getTaskList still reports FinishCnt=48 — authoritative; clamp inflated local.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":999}],"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	got := s.FmlRace()
	if got.Taken.FinishCnt != 48 {
		t.Fatalf("FinishCnt=%d, want 48 from full pool", got.Taken.FinishCnt)
	}
	if got.LocalFinishCnt != 48 {
		t.Fatalf("LocalFinishCnt=%d, want clamped to 48", got.LocalFinishCnt)
	}
}

func TestFmlRaceLocalFinishAdvancesFromLandHarvestBeforeField134(t *testing.T) {
	s := New()
	// Held plant-harvest 花椅轻摇-style task; FinishCnt still 0.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"110":{"1785081600000":{"7":{"0":99,"1":3028,"2":300,"3":0,"4":[23001]}}}},"101":{"0":{"23001":{"1":23001,"2":11,"4":2}}},"100":{"1":{"1001":{"0":23001,"1":2,"2":11,"3":0}}}}`))
	if s.FmlRace().Taken.FinishCnt != 0 {
		t.Fatalf("seed FinishCnt=%d, want 0", s.FmlRace().Taken.FinishCnt)
	}
	// Harvest bumps land HarvestCnt without field 134 yet (cropGets=3 at lvl11 cfg).
	s.ApplyV(json.RawMessage(`{"100":{"1":{"1001":{"0":23001,"1":2,"2":11,"3":1}}}}`))
	got := s.FmlRace()
	if got.Taken.FinishCnt != 0 {
		t.Fatalf("server FinishCnt must stay 0, got %d", got.Taken.FinishCnt)
	}
	if got.LocalFinishCnt != 3 {
		t.Fatalf("LocalFinishCnt=%d, want 3 from HarvestCnt delta", got.LocalFinishCnt)
	}
	// Later 134 catches up higher — local high-water follows.
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{"0":99,"1":3028,"2":300,"3":12,"4":[23001]},"4":1}}}}`))
	got = s.FmlRace()
	if got.Taken.FinishCnt != 12 {
		t.Fatalf("FinishCnt=%d, want 12", got.Taken.FinishCnt)
	}
	if got.LocalFinishCnt != 12 {
		t.Fatalf("LocalFinishCnt=%d, want 12 after 134", got.LocalFinishCnt)
	}
}

func TestFmlRaceLocalFinishCreditsFinalClearHarvest(t *testing.T) {
	s := New()
	// Last harvest round often clears the plot in the same delta ({}).
	// Before: HarvestCnt=3 of frequencys=4 → credit 1 remaining round × cropGets=3.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"110":{"1785081600000":{"7":{"0":99,"1":3028,"2":300,"3":0,"4":[23001]}}}},"101":{"0":{"23001":{"1":23001,"2":11,"4":2}}},"100":{"1":{"1001":{"0":23001,"1":2,"2":11,"3":3}}}}`))
	s.ApplyV(json.RawMessage(`{"100":{"1":{"1001":{}}}}`))
	got := s.FmlRace()
	if got.Taken.FinishCnt != 0 {
		t.Fatalf("server FinishCnt must stay 0, got %d", got.Taken.FinishCnt)
	}
	if got.LocalFinishCnt != 3 {
		t.Fatalf("LocalFinishCnt=%d, want 3 from final clear harvest", got.LocalFinishCnt)
	}
}

func TestFmlRaceTakenClearedByEmptyField134(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":300,"4":[23577]}}}}}`))
	if !s.FmlRace().Taken.HasTask {
		t.Fatal("expected seeded Taken")
	}
	// finishTask ACK clears takeTaskData via empty object in 134.3.
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{},"4":1785364748931}}}}`))
	if s.FmlRace().Taken.HasTask {
		t.Fatalf("empty 134.3 must clear Taken, got %+v", s.FmlRace().Taken)
	}
	if s.FmlRace().LocalFinishCnt != 0 || s.FmlRace().LocalFinishTaskMsId != 0 {
		t.Fatalf("clear must reset LocalFinish, got %+v", s.FmlRace())
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

func TestMarkFmlRaceTaskPoolStale(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":4001,"10":9}]}}`))
	if !s.FmlRace().TasksObserved {
		t.Fatal("expected observed")
	}
	s.MarkFmlRaceTaskPoolStale()
	got := s.FmlRace()
	if !got.TasksObserved || !got.TaskPoolStale {
		t.Fatalf("expected observed snapshot marked stale, got %+v", got)
	}
	// Pool rows preserved so UI/planner still see last snapshot until re-sync.
	if len(got.Tasks) != 1 {
		t.Fatalf("tasks wiped: %+v", got.Tasks)
	}
}

func TestMarkFmlRacePoolTaskClaimed(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":55,"4":4001,"10":9},{"0":66,"4":4001,"10":9}]}}`))
	s.MarkFmlRacePoolTaskClaimed(55)
	got := s.FmlRace()
	var claimed, other FmlRaceTaskView
	for _, task := range got.Tasks {
		switch task.MsId {
		case 55:
			claimed = task
		case 66:
			other = task
		}
	}
	if claimed.UID == 0 {
		t.Fatal("msId 55 must be marked claimed (UID!=0)")
	}
	if other.UID != 0 {
		t.Fatalf("msId 66 UID=%d, want 0", other.UID)
	}
	s.MarkFmlRacePoolTaskClaimed(0)
	s.MarkFmlRacePoolTaskClaimed(999)
}

func TestMarkFmlRaceTakeQuotaExhausted(t *testing.T) {
	s := New()
	s.MarkFmlRaceTakeQuotaExhausted()
	if !s.FmlRace().TakeQuotaExhausted {
		t.Fatal("expected TakeQuotaExhausted")
	}
	// New batch identity clears the sticky flag.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":100,"1":1,"2":1000,"3":9000000000}}}`))
	s.MarkFmlRaceTakeQuotaExhausted()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":200,"1":1,"2":1000,"3":9000000000}}}`))
	if s.FmlRace().TakeQuotaExhausted {
		t.Fatal("batch change must clear TakeQuotaExhausted")
	}

	// A batch can disappear between rounds before the next identity arrives.
	// That transition must not leave the previous quota flag stuck forever.
	s.MarkFmlRaceTakeQuotaExhausted()
	s.ApplyV(json.RawMessage(`{"25":{"111":null}}`))
	if s.FmlRace().TakeQuotaExhausted {
		t.Fatal("cleared batch must clear TakeQuotaExhausted")
	}
	s.MarkFmlRaceTakeQuotaExhausted()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":300,"1":1,"2":1000,"3":9000000000}}}`))
	if s.FmlRace().TakeQuotaExhausted {
		t.Fatal("new batch after an empty identity must clear TakeQuotaExhausted")
	}
}

func TestFmlRaceBatchChangeClearsPreviousTaskPool(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":100,"1":1,"2":1000,"3":9000000000},"114":[{"0":55,"4":3034,"6":[3002],"10":24}],"110":{"100":{"7":{"0":55,"1":3034,"2":4,"3":1,"4":[3002]}}}}}`))
	before := s.FmlRace()
	if !before.TasksObserved || len(before.Tasks) != 1 || !before.Taken.HasTask {
		t.Fatalf("seed race state incomplete: %+v", before)
	}
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":200,"1":1,"2":1000,"3":9000000000}}}`))
	after := s.FmlRace()
	if after.TasksObserved || after.TaskPoolStale || len(after.Tasks) != 0 || after.Taken.HasTask {
		t.Fatalf("new batch retained previous task state: %+v", after)
	}
}

func TestNoteFmlRaceTaskPoolSyncDoesNotInventObservation(t *testing.T) {
	s := New()
	at := time.UnixMilli(1_700_000_000_000)
	s.NoteFmlRaceTaskPoolSync(at)
	got := s.FmlRace()
	if got.TasksObserved || got.TasksSyncedAtMs != 0 {
		t.Fatalf("empty sync must not invent field 114 observation: %+v", got)
	}
	if got.TaskPoolSyncAttemptAtMs != at.UnixMilli() || got.TaskPoolStale {
		t.Fatalf("sync attempt not recorded correctly: %+v", got)
	}
}

func TestNoteFmlRaceTaskPoolSyncConfirmsObservedIncompletePool(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":99,"4":3034,"6":[],"10":24}]}}`))
	s.MarkFmlRaceTaskPoolStale()
	at := time.UnixMilli(1_700_000_000_000)
	s.NoteFmlRaceTaskPoolSync(at)
	got := s.FmlRace()
	if !got.TasksObserved || got.TaskPoolStale {
		t.Fatalf("observed pool must be confirmed and fresh: %+v", got)
	}
	if got.TasksSyncedAtMs != at.UnixMilli() || got.TaskPoolSyncAttemptAtMs != at.UnixMilli() {
		t.Fatalf("sync timestamps not confirmed at %d: %+v", at.UnixMilli(), got)
	}
	wantFP := FmlRaceMissingParamFingerprint(got.Tasks)
	if wantFP == "" || got.MissingParamRefreshFP != wantFP {
		t.Fatalf("missing-param attempt fingerprint=%q, want %q", got.MissingParamRefreshFP, wantFP)
	}
}

func TestFmlRaceBareTaskListWithoutNamespace25DoesNotObserve(t *testing.T) {
	s := New()
	// Top-level 114 is waterwheel NS, not race task pool — mimics getTaskList
	// responses that omit the "25" wrapper (must be normalized before ApplyV).
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"114":[{"0":1,"4":4001,"6":[23001],"10":9}]}`))
	if s.FmlRace().TasksObserved {
		t.Fatal("bare top-level 114 must not set race TasksObserved")
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

func TestFmlRaceQuotaResetsOnBatchChange(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"3":5,"6":1}}}}`))
	got := s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 5 {
		t.Fatalf("batch A quota: %+v", got)
	}
	// New batch: sparse 110 rows omit "3"/"6" while the counters are zero.
	// The old batch's counts must not leak into the new batch.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":43,"1":1,"2":1000,"3":9000},"110":{"43":{"9":1500}}}}`))
	got = s.FmlRace()
	if got.TaskQuotaObserved || got.FinishedTaskNum != 0 || got.BuyTaskNum != 0 {
		t.Fatalf("batch B must reset quota: %+v", got)
	}
	// A full row for the new batch re-observes the quota.
	s.ApplyV(json.RawMessage(`{"25":{"110":{"43":{"3":1,"6":0}}}}`))
	got = s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 1 {
		t.Fatalf("batch B quota re-observe: %+v", got)
	}
}

func TestFmlRaceUsrRcdGiveUpSparseDoesNotWipeFinished(t *testing.T) {
	s := New()
	// Live shape: finishTask sets fTaskNum=4; giveUpTask later sends only
	// giveUpTime/uTime under the same batch key (叶小楠 2026-08-14).
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1786291200000,"1":1,"2":1000,"3":9000},"110":{"1786291200000":{"3":4,"4":126,"5":1000,"9":1000}}}}`))
	got := s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 4 {
		t.Fatalf("after finish: %+v", got)
	}
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1786291200000":{"8":2000,"9":2000}}}}`))
	got = s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 4 {
		t.Fatalf("giveUp sparse must keep finished=4, got %+v", got)
	}
	if got.Taken.HasTask {
		t.Fatalf("giveUp sparse should clear taken, got %+v", got.Taken)
	}
}

func TestFmlRaceUsrRcdGiveUpSparseAloneDoesNotObserveQuota(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1786291200000,"1":1,"2":1000,"3":9000},"110":{"1786291200000":{"8":2000,"9":2000}}}}`))
	got := s.FmlRace()
	if got.TaskQuotaObserved || got.FinishedTaskNum != 0 {
		t.Fatalf("sparse giveUp without fTaskNum must not mark quota observed: %+v", got)
	}
}

func TestFmlRaceUsrRankListRecoversFinished(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":99}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"117":{"5":4}}}`))
	if s.FmlRace().TaskQuotaObserved {
		t.Fatal("quota must start unobserved")
	}
	// Rank list row for self with fTaskNum=4 / buyTaskNum=1.
	s.ApplyV(json.RawMessage(`{"25":{"116":[{"0":99,"1":42,"3":4,"6":1},{"0":100,"1":42,"3":9,"6":0}]}}`))
	got := s.FmlRace()
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 4 || got.BuyTaskNum != 1 {
		t.Fatalf("rank list must recover quota for uid=99: %+v", got)
	}
}

func TestFmlRaceUsrRankListPersonalScoreAndRank(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":99}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000}}}`))
	// Self score 80 (earlier time) vs peer 100 vs peer 80 (later) → rank 2.
	s.ApplyV(json.RawMessage(`{"25":{"116":[
		{"0":100,"1":42,"3":5,"4":100,"5":2000},
		{"0":99,"1":42,"3":4,"4":80,"5":1000,"6":1},
		{"0":101,"1":42,"3":3,"4":80,"5":3000}
	]}}`))
	got := s.FmlRace()
	if !got.ScoreObserved || got.Score != 80 {
		t.Fatalf("score = observed=%v score=%d, want 80", got.ScoreObserved, got.Score)
	}
	if !got.RankObserved || got.Rank != 2 {
		t.Fatalf("rank = observed=%v rank=%d, want 2", got.RankObserved, got.Rank)
	}
	if !got.TaskQuotaObserved || got.FinishedTaskNum != 4 {
		t.Fatalf("quota still required: %+v", got)
	}
}

func TestFmlRaceUsrRcdScorePresenceMerge(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":99}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000},"110":{"42":{"0":99,"1":42,"3":2,"4":55,"5":1500}}}}`))
	got := s.FmlRace()
	if !got.ScoreObserved || got.Score != 55 || got.ScoreTimeMs != 1500 {
		t.Fatalf("110 score not applied: %+v", got)
	}
	// Sparse giveUp-style 110 must not wipe score.
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"8":9999,"9":10000}}}}`))
	got = s.FmlRace()
	if !got.ScoreObserved || got.Score != 55 || got.ScoreTimeMs != 1500 {
		t.Fatalf("sparse 110 wiped score: %+v", got)
	}
}

func TestFmlRaceCalendarSessionWindow(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"monday", time.Date(2026, 8, 17, 12, 0, 0, 0, loc), false},
		{"tuesday_before", time.Date(2026, 8, 18, 8, 59, 59, 0, loc), false},
		{"tuesday_open", time.Date(2026, 8, 18, 9, 0, 0, 0, loc), true},
		{"friday", time.Date(2026, 8, 21, 15, 0, 0, 0, loc), true},
		{"sunday_before_end", time.Date(2026, 8, 23, 20, 59, 59, 0, loc), true},
		{"sunday_end", time.Date(2026, 8, 23, 21, 0, 0, 0, loc), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FmlRaceCalendarInSession(tc.now); got != tc.want {
				t.Fatalf("FmlRaceCalendarInSession(%s)=%v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func TestFmlRaceActiveAtOpensPublishedWindow(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(2026, 8, 18, 9, 0, 0, 0, loc)
	end := time.Date(2026, 8, 23, 21, 0, 0, 0, loc)
	view := FmlRaceView{BatchStatus: 0, BatchStartMs: start.UnixMilli(), BatchEndMs: end.UnixMilli()}
	if view.ActiveAt(start.Add(-time.Second)) {
		t.Fatal("status=0 window must stay inactive before start")
	}
	if !view.ActiveAt(start) {
		t.Fatal("status=0 window must become active at start")
	}
	ended := view
	ended.BatchStatus = 2
	if ended.ActiveAt(start) {
		t.Fatal("status=2 must stay inactive after weekly open")
	}
}
