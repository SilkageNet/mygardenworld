package state

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestZooLogCaptureFixtureDerivesReadAndBlockedActions(t *testing.T) {
	s := New()
	raw, err := os.ReadFile("testdata/zoo_log_events.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	s.ApplyV(raw)

	if !s.ZooLogsObserved() {
		t.Fatal("ZooLogsObserved()=false after namespace 33.2 object")
	}
	logs := s.ZooLogs()
	if len(logs) != 2 {
		t.Fatalf("ZooLogs len=%d, want 2", len(logs))
	}
	completed := logs["7|41"]
	if completed.PetID != 7 || completed.Index != 41 || completed.GoOutEventID != 2096 || completed.ProType != 1 || completed.Gain[23004] != 30 {
		t.Fatalf("completed capture log=%+v", completed)
	}
	active := logs["7|42"]
	if active.PetID != 7 || active.Index != 42 || !active.PetIDObserved || !active.IndexObserved || active.GoOutEventID != 3010 || active.ProType != 0 {
		t.Fatalf("active capture log=%+v", active)
	}
	if !active.UIDObserved || !active.MoodChangeObserved || !active.SatietyChangeObserved || !active.GoOutEventIDObserved || !active.EventTypeObserved || !active.ProTypeObserved || !active.GainObserved || !active.SouvenirObserved || !active.UpdatedAtObserved || !active.CreatedAtObserved || !active.InsertedAtObserved {
		t.Fatalf("active capture log observed flags incomplete: %+v", active)
	}
	if !active.ExtObserved || !active.Ext.ConsumeObserved || !active.Ext.Consume2Observed || active.Ext.Consume[23004] != 3 || active.Ext.Consume2[11] != 540 {
		t.Fatalf("captured ext costs were not preserved: %+v", active.Ext)
	}
	if !active.ConsumeObserved || len(active.Consume) != 0 {
		t.Fatalf("explicit empty top-level consume was not observed: %+v", active)
	}

	actions := s.ZooEventActions()
	if len(actions) != 2 {
		t.Fatalf("ZooEventActions=%+v, want read + blocked cost event", actions)
	}
	if got := actions[0]; got.Blocked || got.Action != "read_log" || got.PetID != 7 || got.EventID != 2096 || got.TableID != 41 {
		t.Fatalf("read action=%+v", got)
	}
	if got := actions[1]; !got.Blocked || got.Action != "handle_event" || got.PetID != 7 || got.EventID != 3010 || got.TableID != 42 || !strings.Contains(got.BlockedReason, "扩展结果包含消耗") {
		t.Fatalf("cost-bearing action=%+v", got)
	}

	active.Ext.Consume[23004] = 99
	if got := s.ZooLogs()["7|42"].Ext.Consume[23004]; got != 3 {
		t.Fatalf("ZooLogs defensive copy leaked mutation: %d", got)
	}
}

func TestZooObservedType2LogIsBlockedAndUsesLogIndexAsTableID(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
		"7|42": safeZooLogFields(7, 42, 2096, 2000),
	}}})
	actions := s.ZooEventActions()
	if len(actions) != 1 {
		t.Fatalf("ZooEventActions=%+v, want one observed type-2 diagnostic", actions)
	}
	if got := actions[0]; !got.Blocked || got.Action != "handle_event" || got.PetID != 7 || got.EventID != 2096 || got.TableID != 42 || got.Agree || !strings.Contains(got.BlockedReason, "已观测客户端 handleEvent 分支") {
		t.Fatalf("blocked action=%+v, want eventId 2096 but tableId log idx 42", got)
	}
}

func TestZooVisitHistoryIsPreservedButExcludedFromAutomation(t *testing.T) {
	s := New()
	logs := make(map[string]any, 93)
	for index := int32(1); index <= 92; index++ {
		fields := safeZooLogFields(0, index, 2096, int64(1000+index))
		delete(fields, "1")
		fields["6"] = zooVisitEventType
		logs[fmt.Sprintf("0:%d", index)] = fields
	}
	// A non-visit entry missing pid must continue to fail closed; only the
	// client-defined visit-history class is excluded.
	normal := safeZooLogFields(0, 93, 2096, 2000)
	delete(normal, "1")
	logs["normal-missing-pid"] = normal
	applyMap(t, s, map[string]any{"33": map[string]any{"2": logs}})

	if got := len(s.ZooLogs()); got != 93 {
		t.Fatalf("ZooLogs len=%d, want all 93 raw observations preserved", got)
	}
	actions := s.ZooEventActions()
	if len(actions) != 1 || !actions[0].Blocked || actions[0].TableID != 93 || !strings.Contains(actions[0].BlockedReason, "宠物 ID") {
		t.Fatalf("ZooEventActions=%+v, want only normal missing-pid diagnostic", actions)
	}

	for _, actionName := range []string{"handle", "read"} {
		var action ZooEventAction
		var ok bool
		if actionName == "handle" {
			action, ok = s.ZooHandleEventAction(0, 42)
		} else {
			action, ok = s.ZooReadLogAction(0, 42)
		}
		if !ok || !action.Blocked || !strings.Contains(action.BlockedReason, "访客历史") {
			t.Fatalf("%s visit preflight action=%+v ok=%t", actionName, action, ok)
		}
	}
	if s.ZooLogHandled(0, 42) || s.ZooLogRead(0, 42, 1042) {
		t.Fatal("visit history satisfied an automation postcondition")
	}
}

func TestZooLogSparseMergeExtReplacementAndNullDeletion(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
		"7|42": safeZooLogFields(7, 42, 2096, 2000),
	}}})
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
		"7|42": map[string]any{"3": -5, "11": map[string]any{"3": map[string]any{"11": 3}}},
	}}})

	log := s.ZooLogs()["7|42"]
	if log.MoodChangeValue != -5 || log.GoOutEventID != 2096 || log.ProType != 0 || log.CreatedAtMs != 2000 {
		t.Fatalf("sparse log merge lost fields: %+v", log)
	}
	if log.Ext.Consume[11] != 3 {
		t.Fatalf("ext replacement cost=%+v", log.Ext)
	}
	actions := s.ZooEventActions()
	if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, "消耗") {
		t.Fatalf("cost-bearing sparse log action=%+v", actions)
	}

	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
		"7|42": map[string]any{"11": map[string]any{}},
	}}})
	log = s.ZooLogs()["7|42"]
	if !log.ExtObserved || len(log.Ext.Consume) != 0 || len(log.Ext.Consume2) != 0 {
		t.Fatalf("empty ext replacement did not clear old consume: %+v", log.Ext)
	}
	if action := s.ZooEventActions()[0]; !action.Blocked || strings.Contains(action.BlockedReason, "扩展结果包含消耗") || !strings.Contains(action.BlockedReason, "已观测客户端 handleEvent 分支") {
		t.Fatalf("empty ext replacement action=%+v, want only observed-client shape blocked", action)
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{"7|42": nil}}})
	if len(s.ZooLogs()) != 0 || len(s.ZooEventActions()) != 0 {
		t.Fatalf("null log entry did not delete: logs=%+v actions=%+v", s.ZooLogs(), s.ZooEventActions())
	}
}

func TestZooLogUnsafeGatesAreBlocked(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]any
		want   []string
	}{
		{name: "missing proType", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "7"), want: []string{"处理状态"}},
		{name: "missing event id", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5"), want: []string{"事件配置 ID"}},
		{name: "missing event type", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "6"), want: []string{"事件类型"}},
		{name: "event type mismatch", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "6", 3), want: []string{"事件类型与静态配置不一致"}},
		{name: "missing cTime", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "13"), want: []string{"创建时间"}},
		{name: "missing pet id", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "1"), want: []string{"宠物 ID"}},
		{name: "missing log index", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "2"), want: []string{"日志序号"}},
		{name: "missing gain", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "8"), want: []string{"收益字段未完整观测"}},
		{name: "missing top consume", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9"), want: []string{"消耗字段未观测"}},
		{name: "missing souvenir", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "10"), want: []string{"纪念品字段未完整观测"}},
		{name: "missing ext", fields: withoutZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11"), want: []string{"扩展消耗字段未完整观测"}},
		{name: "gain already present", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "8", map[string]any{"23004": 1}), want: []string{"已包含收益结果"}},
		{name: "zero gain entry is not empty", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "8", map[string]any{"23004": 0}), want: []string{"已包含收益结果"}},
		{name: "top consume", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", map[string]any{"11": 1}), want: []string{"存在物品或货币消耗"}},
		{name: "zero consume entry is not empty", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", map[string]any{"11": 0}), want: []string{"存在物品或货币消耗"}},
		{name: "souvenir already present", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "10", map[string]any{"5001": 1}), want: []string{"已包含纪念品结果"}},
		{name: "zero souvenir entry is not empty", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "10", map[string]any{"5001": 0}), want: []string{"已包含纪念品结果"}},
		{name: "null gain", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "8", nil), want: []string{"收益字段未完整观测"}},
		{name: "null top consume", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", nil), want: []string{"消耗字段未观测"}},
		{name: "null souvenir", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "10", nil), want: []string{"纪念品字段未完整观测"}},
		{name: "null ext", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11", nil), want: []string{"扩展消耗字段未完整观测"}},
		{name: "malformed top consume", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", "bad"), want: []string{"消耗字段未观测"}},
		{name: "array top consume", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", []int32{11, 1}), want: []string{"消耗字段未观测"}},
		{name: "fractional item count", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", map[string]any{"11": 0.5}), want: []string{"消耗字段未观测"}},
		{name: "overflow item id", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", map[string]any{"2147483648": 1}), want: []string{"消耗字段未观测"}},
		{name: "overflow item count", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "9", map[string]any{"11": int64(2147483648)}), want: []string{"消耗字段未观测"}},
		{name: "fractional proType", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "7", 0.5), want: []string{"处理状态"}},
		{name: "fractional event id", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5", 2096.5), want: []string{"事件配置 ID"}},
		{name: "overflow pet id", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "1", int64(2147483648)), want: []string{"宠物 ID"}},
		{name: "fractional created time", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "13", 2000.5), want: []string{"创建时间"}},
		{name: "ext consume", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11", map[string]any{"3": map[string]any{"11": 1}}), want: []string{"扩展结果包含消耗"}},
		{name: "zero ext consume entry is not empty", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11", map[string]any{"3": map[string]any{"11": 0}}), want: []string{"扩展结果包含消耗"}},
		{name: "ext consume2", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11", map[string]any{"4": map[string]any{"1": 1}}), want: []string{"扩展结果包含消耗"}},
		{name: "malformed ext", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11", "bad"), want: []string{"扩展消耗字段未完整观测"}},
		{name: "array ext consume", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "11", map[string]any{"3": []int32{11, 1}}), want: []string{"扩展消耗字段未完整观测"}},
		{name: "unknown config", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5", 999999), want: []string{"静态配置不存在"}},
		{name: "special code", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5", 1001), want: []string{"分享、视频或特殊客户端流程"}},
		{name: "share reward2 noHandle result and choice", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5", 3012), want: []string{"分享、视频或特殊客户端流程", "二选一、稍后处理或多结果分支"}},
		{name: "lost never find", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5", 4001), want: []string{"分享、视频或特殊客户端流程", "二选一、稍后处理或多结果分支"}},
		{name: "no unique result", fields: withZooLogField(safeZooLogFields(7, 42, 2096, 2000), "5", 8001), want: []string{"结果类别不唯一"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{"7|42": tc.fields}}})
			actions := s.ZooEventActions()
			if len(actions) != 1 || !actions[0].Blocked {
				t.Fatalf("ZooEventActions=%+v, want blocked", actions)
			}
			malformed := s.ZooLogs()["7|42"].Malformed
			for _, want := range tc.want {
				if !strings.Contains(actions[0].BlockedReason, want) && (!malformed || !strings.Contains(actions[0].BlockedReason, "拒绝沿用旧状态")) {
					t.Fatalf("ZooEventActions=%+v, want blocked reason containing %q", actions, want)
				}
			}
			if actions[0].Action == "find_pet" {
				t.Fatalf("unsafe log produced findPet: %+v", actions[0])
			}
		})
	}
}

func TestZooLogOpaqueKeysAndDecimalStrings(t *testing.T) {
	for _, key := range []string{"2147483648|1", "1|2147483648", "7.0|42", "7|42|1", "-7|42", "server-entry"} {
		t.Run("opaque key "+key, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
				key: safeZooLogFields(7, 42, 2096, 2000),
			}}})
			log, ok := s.ZooLogs()[key]
			if !ok || !s.ZooLogsObserved() || log.Key != key || log.PetID != 7 || log.Index != 42 {
				t.Fatalf("opaque key was not preserved independently from identity: observed=%t log=%+v", s.ZooLogsObserved(), log)
			}
			actions := s.ZooEventActions()
			if len(actions) != 1 || actions[0].PetID != 7 || actions[0].TableID != 42 {
				t.Fatalf("opaque key actions=%+v", actions)
			}
		})
	}

	t.Run("entry fields are authoritative identity", func(t *testing.T) {
		s := New()
		applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
			"7|42": safeZooLogFields(8, 99, 2096, 2000),
		}}})
		actions := s.ZooEventActions()
		if len(actions) != 1 || actions[0].PetID != 8 || actions[0].TableID != 99 {
			t.Fatalf("entry identity was replaced by transport key: %+v", actions)
		}
	})

	t.Run("exact decimal strings accepted", func(t *testing.T) {
		s := New()
		applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
			"7|42": map[string]any{
				"0": "12345", "1": "7", "2": "42", "3": "0", "4": "0",
				"5": "2096", "6": "2", "7": "0", "8": map[string]any{},
				"9": map[string]any{}, "10": map[string]any{}, "11": map[string]any{},
				"12": "2000", "13": "2000", "14": "2000",
			},
		}}})
		logs := s.ZooLogs()
		log := logs["7|42"]
		if len(logs) != 1 || !log.UIDObserved || log.UID != 12345 || !log.CreatedAtObserved || log.CreatedAtMs != 2000 {
			t.Fatalf("exact decimal strings not preserved: %+v", log)
		}
		actions := s.ZooEventActions()
		if len(actions) != 1 || !actions[0].Blocked || actions[0].TableID != 42 || !strings.Contains(actions[0].BlockedReason, "已观测客户端 handleEvent 分支") {
			t.Fatalf("exact decimal string log actions=%+v", actions)
		}
		items, ok := readZooLogItemMap([]byte(`{"11":"0"}`))
		if !ok || len(items) != 1 || items[11] != 0 {
			t.Fatalf("exact decimal string item map=%+v ok=%t", items, ok)
		}
	})
}

func TestZooLogUnexpectedKeyDoesNotInvalidateExistingCollection(t *testing.T) {
	s := New()
	logs := make(map[string]any, 35)
	for i := int32(0); i < 35; i++ {
		index := 930 + i
		logs[fmt.Sprintf("1|%d", index)] = completedZooLogFields(1, index, 2001, int64(1000+i))
	}
	applyMap(t, s, map[string]any{"33": map[string]any{
		"1": map[string]any{"1": map[string]any{"1": 1, "19": int64(2000)}},
		"2": logs,
	}})
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
		"unexpected-server-key": safeZooLogFields(1, 1024, 2096, 3000),
	}}})

	if !s.ZooLogsObserved() || len(s.ZooLogs()) != 36 {
		t.Fatalf("unexpected key invalidated collection: observed=%t logs=%d", s.ZooLogsObserved(), len(s.ZooLogs()))
	}
	actions := s.ZooEventActions()
	if len(actions) != 1 || actions[0].PetID != 1 || actions[0].TableID != 1024 || strings.Contains(actions[0].BlockedReason, "键格式") {
		t.Fatalf("unexpected key fanned out blocked actions: %+v", actions)
	}
	if action, ok := s.ZooHandleEventAction(1, 1024); !ok || action.PetID != 1 || action.TableID != 1024 {
		t.Fatalf("opaque-key preflight lookup action=%+v ok=%t", action, ok)
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{"unexpected-server-key": nil}}})
	if !s.ZooLogHandled(1, 1024) {
		t.Fatal("opaque-key deletion did not satisfy handle postcondition")
	}
}

func TestZooLogDuplicateIdentityIsAggregatedAndFailsClosed(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
		"first":  safeZooLogFields(7, 42, 2096, 2000),
		"second": safeZooLogFields(7, 42, 2096, 2001),
	}}})
	actions := s.ZooEventActions()
	if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, "身份重复") {
		t.Fatalf("duplicate identity actions=%+v", actions)
	}
	if action, ok := s.ZooHandleEventAction(7, 42); !ok || !action.Blocked || !strings.Contains(action.BlockedReason, "身份重复") {
		t.Fatalf("duplicate identity preflight action=%+v ok=%t", action, ok)
	}
	if s.ZooLogHandled(7, 42) || s.ZooLogRead(7, 42, 2000) {
		t.Fatal("duplicate identity satisfied an operation postcondition")
	}
}

func TestZooLogExistingEntryBecomingMalformedFailsClosed(t *testing.T) {
	for name, malformed := range map[string]any{
		"array entry":  []any{1, 2, 3},
		"string entry": "bad",
		"field parse":  map[string]any{"0": []any{12345}},
	} {
		t.Run(name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
				"7|42": safeZooLogFields(7, 42, 2096, 2000),
			}}})
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{"7|42": malformed}}})

			log := s.ZooLogs()["7|42"]
			if !log.Malformed || log.MalformedReason == "" {
				t.Fatalf("malformed entry retained trusted state: %+v", log)
			}
			actions := s.ZooEventActions()
			if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, "拒绝沿用旧状态") {
				t.Fatalf("malformed entry actions=%+v", actions)
			}
			if action, ok := s.ZooHandleEventAction(7, 42); !ok || !action.Blocked {
				t.Fatalf("malformed entry preflight action=%+v ok=%t", action, ok)
			}
			if s.ZooLogHandled(7, 42) {
				t.Fatal("malformed entry satisfied handle postcondition")
			}

			// A sparse delta after corruption starts from a blank entry; it must
			// not resurrect the previously observed event identity or timestamp.
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{"7|42": map[string]any{"7": 0}}}})
			log = s.ZooLogs()["7|42"]
			if log.Malformed || log.GoOutEventIDObserved || log.CreatedAtObserved {
				t.Fatalf("sparse recovery reused pre-corruption fields: %+v", log)
			}
			if actions = s.ZooEventActions(); len(actions) != 1 || !actions[0].Blocked {
				t.Fatalf("sparse recovery actions=%+v, want blocked", actions)
			}
		})
	}
}

func TestZooLogWholeMapMalformedProducesOneCollectionDiagnostic(t *testing.T) {
	for name, malformed := range map[string]any{
		"array":  []any{},
		"string": "bad",
		"null":   nil,
	} {
		t.Run(name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
				"7|42": safeZooLogFields(7, 42, 2096, 2000),
			}}})
			applyMap(t, s, map[string]any{"33": map[string]any{"2": malformed}})

			if s.ZooLogsObserved() {
				t.Fatal("malformed whole map remained observed")
			}
			log := s.ZooLogs()["7|42"]
			if log.Malformed {
				t.Fatalf("collection failure was copied onto an entry: %+v", log)
			}
			actions := s.ZooEventActions()
			if len(actions) != 1 || !actions[0].Blocked || actions[0].Action != "sync_logs" || !strings.Contains(actions[0].BlockedReason, "失去可信度") {
				t.Fatalf("whole-map malformed actions=%+v", actions)
			}
			if action, ok := s.ZooHandleEventAction(7, 42); !ok || !action.Blocked {
				t.Fatalf("whole-map preflight action=%+v ok=%t", action, ok)
			}
			if s.ZooLogHandled(7, 42) || s.ZooLogRead(7, 42, 2000) {
				t.Fatal("untrusted whole-map state satisfied an operation postcondition")
			}
		})
	}
}

func TestZooCompletedLogsReadOncePerPetAndRequireObservedReadTime(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"1": map[string]any{"7": map[string]any{"1": 7, "19": 1000}},
		"2": map[string]any{
			"7|41": completedZooLogFields(7, 41, 2001, 1500),
			"7|43": completedZooLogFields(7, 43, 2002, 2500),
		},
	}})
	actions := s.ZooEventActions()
	if len(actions) != 1 || actions[0].Blocked || actions[0].Action != "read_log" || actions[0].TableID != 43 {
		t.Fatalf("completed log actions=%+v, want newest single read", actions)
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"1": map[string]any{"7": map[string]any{"19": 3000}}}})
	if actions := s.ZooEventActions(); len(actions) != 0 {
		t.Fatalf("read logs repeated after readLogTime advanced: %+v", actions)
	}

	unknownRead := New()
	applyMap(t, unknownRead, map[string]any{"33": map[string]any{
		"1": map[string]any{"7": map[string]any{"1": 7}},
		"2": map[string]any{"7|41": completedZooLogFields(7, 41, 2001, 1500)},
	}})
	actions = unknownRead.ZooEventActions()
	if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, "已读日志时间未观测") {
		t.Fatalf("missing read time actions=%+v", actions)
	}
	for name, value := range map[string]any{"null": nil, "invalid": "bad"} {
		t.Run("read time "+name, func(t *testing.T) {
			st := New()
			applyMap(t, st, map[string]any{"33": map[string]any{
				"1": map[string]any{"7": map[string]any{"1": 7, "19": value}},
				"2": map[string]any{"7|41": completedZooLogFields(7, 41, 2001, 1500)},
			}})
			if st.ZooPets()[7].ReadLogTimeObserved {
				t.Fatalf("readLogTime %s was treated as observed", name)
			}
			actions := st.ZooEventActions()
			if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, "已读日志时间未观测") {
				t.Fatalf("readLogTime %s actions=%+v", name, actions)
			}
		})
	}
}

func TestZooSparseNullClearsSafetyObservations(t *testing.T) {
	tests := []struct {
		name          string
		field         string
		blockedReason string
		observed      func(ZooLogView) bool
	}{
		{name: "gain", field: "8", blockedReason: "收益字段未完整观测", observed: func(log ZooLogView) bool { return log.GainObserved }},
		{name: "consume", field: "9", blockedReason: "消耗字段未观测", observed: func(log ZooLogView) bool { return log.ConsumeObserved }},
		{name: "souvenir", field: "10", blockedReason: "纪念品字段未完整观测", observed: func(log ZooLogView) bool { return log.SouvenirObserved }},
		{name: "ext", field: "11", blockedReason: "扩展消耗字段未完整观测", observed: func(log ZooLogView) bool { return log.ExtObserved }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
				"7|42": safeZooLogFields(7, 42, 2096, 2000),
			}}})
			applyMap(t, s, map[string]any{"33": map[string]any{"2": map[string]any{
				"7|42": map[string]any{tc.field: nil},
			}}})

			log := s.ZooLogs()["7|42"]
			if tc.observed(log) {
				t.Fatalf("explicit null retained observed %s state: %+v", tc.name, log)
			}
			if log.Malformed {
				t.Fatalf("explicit null should be unobserved, not malformed: %+v", log)
			}
			actions := s.ZooEventActions()
			if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, tc.blockedReason) {
				t.Fatalf("explicit null actions=%+v, want blocked reason containing %q", actions, tc.blockedReason)
			}
		})
	}

	t.Run("pet read time and ext", func(t *testing.T) {
		s := New()
		applyMap(t, s, map[string]any{"33": map[string]any{
			"1": map[string]any{"7": map[string]any{"1": 7, "18": map[string]any{"kind": "observed"}, "19": 1000}},
			"2": map[string]any{"7|41": completedZooLogFields(7, 41, 2001, 1500)},
		}})
		applyMap(t, s, map[string]any{"33": map[string]any{
			"1": map[string]any{"7": map[string]any{"18": nil, "19": nil}},
		}})

		pet := s.ZooPets()[7]
		if pet.ReadLogTimeObserved || pet.ReadLogTimeMs != 0 || pet.ExtObserved || len(pet.Ext) != 0 {
			t.Fatalf("explicit null retained stale pet safety fields: %+v", pet)
		}
		actions := s.ZooEventActions()
		if len(actions) != 1 || !actions[0].Blocked || !strings.Contains(actions[0].BlockedReason, "已读日志时间未观测") {
			t.Fatalf("explicit null pet actions=%+v", actions)
		}
	})
}

func safeZooLogFields(petID, index, eventID int32, createdAt int64) map[string]any {
	return map[string]any{
		"0": int64(12345), "1": petID, "2": index, "3": 0, "4": 0,
		"5": eventID, "6": 2, "7": 0, "8": map[string]any{}, "9": map[string]any{},
		"10": map[string]any{}, "11": map[string]any{}, "12": createdAt, "13": createdAt, "14": createdAt,
	}
}

func completedZooLogFields(petID, index, eventID int32, createdAt int64) map[string]any {
	fields := safeZooLogFields(petID, index, eventID, createdAt)
	fields["7"] = 1
	return fields
}

func withoutZooLogField(fields map[string]any, key string) map[string]any {
	out := cloneZooLogTestFields(fields)
	delete(out, key)
	return out
}

func withZooLogField(fields map[string]any, key string, value any) map[string]any {
	out := cloneZooLogTestFields(fields)
	out[key] = value
	return out
}

func cloneZooLogTestFields(fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		out[key] = value
	}
	return out
}
