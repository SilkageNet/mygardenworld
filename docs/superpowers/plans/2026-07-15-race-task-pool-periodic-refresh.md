# Race Task Pool Periodic Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 竞赛任务池每 10 分钟空闲重拉，Web 显示更新时间与间隔；近 CD（剩余不足 10 分钟）的可接任务优先于定时刷新。

**Architecture:** 在 field 114 apply 时记录 `TasksSyncedAtMs`；`unionRaceOperations` 在 giveUp/finish/take 之后、按 TTL 与「近可接 CD」门禁决定是否发 `getTaskList`；proto/query/Web 展示时间戳与固定文案。

**Tech Stack:** Go、buf protobuf（`make proto-gen` / `make proto-gen-web`）、Next.js（`web/src/app/page.tsx`）、`go test`。

**Spec:** `docs/superpowers/specs/2026-07-15-race-task-pool-periodic-refresh-design.md`

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `internal/state/types.go` | `FmlRaceView.TasksSyncedAtMs` | Modify |
| `internal/state/fml.go` | apply 114 时写入时间戳 | Modify |
| `internal/state/fml_race_test.go` | 时间戳 RED/GREEN | Modify |
| `internal/automation/shop_union.go` | 常量、近 CD 检测、定时 sync 顺序 | Modify |
| `internal/automation/shop_union_race_test.go` | 定时刷新回归 | Modify |
| `proto/mygardenworld/v1/query_service.proto` | `tasks_synced_at_ms` | Modify |
| `internal/apiserver/query_service.go` | 映射字段 | Modify |
| `gen/` + `web/src/gen/` | 生成代码 | Via make |
| `web/src/app/page.tsx` | 任务池标题文案 | Modify |

---

### Task 1: State — `TasksSyncedAtMs`

**Files:**
- Modify: `internal/state/types.go`
- Modify: `internal/state/fml.go`
- Modify: `internal/state/fml_race_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/state/fml_race_test.go`:

```go
func TestFmlRaceTasksSyncedAtMsSetOnTaskList(t *testing.T) {
	s := state.New()
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
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"114":null}}`))
	got := s.FmlRace()
	if !got.TasksObserved || got.TasksSyncedAtMs <= 0 {
		t.Fatalf("empty/null pool must set TasksObserved + TasksSyncedAtMs, got %+v", got)
	}
}
```

Add imports (`time`) if missing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/state/ -run 'TestFmlRaceTasksSyncedAtMs' -count=1`

Expected: FAIL（无字段或始终为 0）

- [ ] **Step 3: Minimal implementation**

In `internal/state/types.go` inside `FmlRaceView`，加：

```go
	// TasksSyncedAtMs is local wall time (ms) when field 114 was last applied.
	TasksSyncedAtMs int64
```

In `internal/state/fml.go`，改调用为传入 `s.lastApplyMs`：

```go
	if rawTasks, ok := ns25["114"]; ok {
		applyFmlRaceTasksLocked(&s.fmlRace, rawTasks, s.lastApplyMs)
	}
```

改签名与写入（在成功路径标记 `TasksObserved` 时）：

```go
func applyFmlRaceTasksLocked(view *FmlRaceView, raw json.RawMessage, nowMs int64) {
	if isJSONNull(raw) {
		view.TasksObserved = true
		view.Tasks = nil
		view.MissingParamRefreshFP = ""
		view.TasksSyncedAtMs = nowMs
		return
	}
	var tasks []clientproto.IFmlRaceTask
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return
	}
	// ... existing merge logic unchanged ...
	view.TasksObserved = true
	view.TasksSyncedAtMs = nowMs
	updateFmlRaceMissingParamRefreshFP(view, wasObserved)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/state/ -run 'TestFmlRaceTasksSyncedAtMs|TestFmlRace' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/types.go internal/state/fml.go internal/state/fml_race_test.go
git commit -m "$(cat <<'EOF'
feat(state): record race task pool sync timestamp

EOF
)"
```

---

### Task 2: Automation — periodic `getTaskList` + near-CD defer

**Files:**
- Modify: `internal/automation/shop_union.go`
- Modify: `internal/automation/shop_union_race_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/automation/shop_union_race_test.go`:

```go
func TestUnionRacePeriodicGetTaskListAfterTTL(t *testing.T) {
	s := state.New()
	// Empty pool: TasksObserved, nothing to take.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	if synced <= 0 {
		t.Fatal("need TasksSyncedAtMs from apply")
	}
	policy := testRacePolicy()
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	ops := unionRaceOperations(s, policy, 0, now)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected periodic getTaskList, got %+v", ops)
	}
}

func TestUnionRaceNoPeriodicGetTaskListWithinTTL(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval - time.Second)
	ops := unionRaceOperations(s, policy, 0, now)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("unexpected getTaskList within TTL: %+v", ops)
		}
	}
}

func TestUnionRaceTakeWinsOverPeriodicSync(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 20, 0, 0}})
	synced := s.FmlRace().TasksSyncedAtMs
	plannerNow := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	ops := unionRaceOperations(s, testRacePolicy(), 0, plannerNow)
	if len(ops) < 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take first, got %+v", ops)
	}
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("take tick must not also sync, got %+v", ops)
		}
	}
}

// raceStateAtTTL returns state whose pool has AppearTime = plannerNow+appearRem,
// and plannerNow is exactly TasksSyncedAtMs + refreshInterval + 1s (TTL due).
func raceStateAtTTL(t *testing.T, appearRem time.Duration, plantParam int32) (*state.State, time.Time) {
	t.Helper()
	s := state.New()
	if plantParam > 0 {
		s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(plantParam)}})
	}
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"6":[23001],"10":20}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	plannerNow := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	appear := plannerNow.Add(appearRem).UnixMilli()
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"114":[{"0":1,"4":3036,"5":%d,"6":[%d],"10":20,"12":0,"14":0,"15":0}]}}`,
		appear, plantParam,
	)))
	delta := s.FmlRace().TasksSyncedAtMs - synced
	plannerNow = plannerNow.Add(time.Duration(delta) * time.Millisecond)
	return s, plannerNow
}

func TestUnionRacePeriodicDeferredForNearTakeableCD(t *testing.T) {
	s, now := raceStateAtTTL(t, 5*time.Minute, 23001)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("near takeable CD must defer periodic sync, got %+v", ops)
		}
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("outside lead must not take yet, got %+v", ops)
		}
	}
}

func TestUnionRacePeriodicRunsDespiteFarTakeableCD(t *testing.T) {
	s, now := raceStateAtTTL(t, 15*time.Minute, 23001)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("far takeable CD must not block periodic sync, got %+v", ops)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/automation/ -run 'TestUnionRacePeriodic|TestUnionRaceTakeWinsOverPeriodic' -count=1`

Expected: FAIL（尚无常量/逻辑）

- [ ] **Step 3: Implement helpers + reorder `unionRaceOperations`**

In `internal/automation/shop_union.go`（常量旁 `raceTakeLeadWindow`）：

```go
const raceTaskPoolRefreshInterval = 10 * time.Minute
```

Add:

```go
// raceTaskPoolTTLStale reports whether a periodic getTaskList is due.
func raceTaskPoolTTLStale(view state.FmlRaceView, now time.Time) bool {
	if !view.BatchActive || !view.TasksObserved {
		return false
	}
	if view.TasksSyncedAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceTaskPoolRefreshInterval))
}

// raceHasNearTakeableCD reports whether any pool task is still on CD with
// remaining time in (0, raceTaskPoolRefreshInterval) and would pass non-CD take filters.
func raceHasNearTakeableCD(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time) bool {
	nowMs := now.UnixMilli()
	for _, t := range tasks {
		if t.AppearTime <= 0 || t.AppearTime <= nowMs {
			continue
		}
		rem := time.Duration(t.AppearTime-nowMs) * time.Millisecond
		if rem >= raceTaskPoolRefreshInterval {
			continue
		}
		if raceTakeNonCDSkipReason(s, t, policy, uid) == "" {
			return true
		}
	}
	return false
}

func isRacePrimaryMutatingOp(op PlannedOp) bool {
	switch op.Kind {
	case clientproto.RPCFmlRaceGiveUpTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceTakeTask.String():
		return true
	default:
		return false
	}
}
```

Refactor the body after auto/batch gates so that after building giveUp/finish/take into `ops`:

```go
	hasPrimary := false
	for _, op := range ops {
		if isRacePrimaryMutatingOp(op) {
			hasPrimary = true
			break
		}
	}
	if hasPrimary {
		// Keep existing upgrade/delete append behavior after primary ops.
		// (move upgrade/delete blocks here unchanged)
		return opsWithUpgradeDelete(ops, ...) // or inline existing upgrade/delete
	}

	if raceTaskPoolTTLStale(view, now) && !raceHasNearTakeableCD(s, view.Tasks, policy, uid, now) {
		return []PlannedOp{domainOp(clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync", "公会竞赛定时刷新任务池", 4398, 0, 0, 0)}
	}

	// upgrade / delete as today
	return ops
```

Keep early returns for enter / first&param sync **unchanged** at the top.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/automation/ -run 'TestUnionRace' -count=1`

Expected: PASS（含既有 race 测试）

- [ ] **Step 5: Commit**

```bash
git add internal/automation/shop_union.go internal/automation/shop_union_race_test.go
git commit -m "$(cat <<'EOF'
feat(automation): refresh race task pool every 10m with near-CD defer

EOF
)"
```

---

### Task 3: Proto + query API

**Files:**
- Modify: `proto/mygardenworld/v1/query_service.proto`
- Modify: `internal/apiserver/query_service.go`
- Generated: via `make proto-gen` / `make proto-gen-web`

- [ ] **Step 1: Add proto field**

In `message FmlRaceView`:

```protobuf
message FmlRaceView {
  bool observed = 1;
  bool batch_active = 2;
  FmlRaceTaken taken = 3;
  repeated FmlRaceTask tasks = 4;
  int64 batch_start_ms = 5;
  int64 batch_end_ms = 6;
  int32 batch_status = 7;
  // Local ms when task pool (NS25 field 114) was last applied.
  int64 tasks_synced_at_ms = 8;
}
```

- [ ] **Step 2: Generate**

Run:

```bash
make proto-gen
make proto-gen-web
```

Expected: success, `TasksSyncedAtMs` on Go/TS stubs.

- [ ] **Step 3: Map in `fmlRaceProto`**

```go
	out := &pb.FmlRaceView{
		Observed:         view.Observed,
		BatchActive:      view.BatchActive,
		BatchStartMs:     view.BatchStartMs,
		BatchEndMs:       view.BatchEndMs,
		BatchStatus:      view.BatchStatus,
		TasksSyncedAtMs:  view.TasksSyncedAtMs,
	}
```

- [ ] **Step 4: Smoke compile**

Run: `go test ./internal/apiserver/ -count=1`

Expected: PASS（或至少 compile）

- [ ] **Step 5: Commit**

```bash
git add proto/mygardenworld/v1/query_service.proto gen/ internal/apiserver/query_service.go web/src/gen/
git commit -m "$(cat <<'EOF'
feat(api): expose race tasks_synced_at_ms on snapshot

EOF
)"
```

---

### Task 4: Web — 任务池标题展示

**Files:**
- Modify: `web/src/app/page.tsx`

- [ ] **Step 1: Update `FmlRaceMonitorPanel` header**

In the 任务池 section header (`web/src/app/page.tsx`，约 1467–1471 行），改为：

```tsx
          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span>任务池</span>
                {(race?.tasksSyncedAtMs ?? BigInt(0)) > BigInt(0) && (
                  <span className="text-xs font-normal text-muted-foreground">
                    更新于{" "}
                    {new Date(Number(race!.tasksSyncedAtMs)).toLocaleString("zh-CN", {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}{" "}
                    · 每 10 分钟重新获取
                  </span>
                )}
              </div>
              <Badge variant="secondary">{tasks.length} 个</Badge>
            </div>
```

- [ ] **Step 2: Typecheck / lint if available**

Run: `cd web && npx tsc --noEmit`（若项目常用）或目视确认 `tasksSyncedAtMs` 来自 gen 类型。

- [ ] **Step 3: Commit**

```bash
git add web/src/app/page.tsx
git commit -m "$(cat <<'EOF'
feat(web): show race task pool sync time and 10m refresh hint

EOF
)"
```

---

### Task 5: Final verification

- [ ] **Step 1: Run focused + broader tests**

```bash
go test ./internal/state/ ./internal/automation/ ./internal/apiserver/ -count=1
```

Expected: PASS

- [ ] **Step 2: Spec checklist**

Confirm each spec test case 1–8 is covered by Task 1–4 tests or manual UI check (case 8 = Web).

---

## Self-review (plan vs spec)

| Spec requirement | Task |
|---|---|
| `TasksSyncedAtMs` on apply 114 | Task 1 |
| 10m constant TTL sync | Task 2 |
| Near CD rem under 10m defer | Task 2 |
| Far CD rem at least 10m sync OK | Task 2 |
| take/finish/giveUp 让路 | Task 2 |
| enter / missing-param 早退不变 | Task 2（保留） |
| proto `tasks_synced_at_ms` | Task 3 |
| Web 文案 | Task 4 |
