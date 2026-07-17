# Race Taken Sync Soft Recover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 登录同步后正确识别已接竞赛任务（`110` 为主、池 `UID==自己` 兜底），并对 `takeTask` 的 `fmlRace_tips1` 做软恢复，避免账号被标异常。

**Architecture:** 在 `applyFmlLocked` 完成 `110`/`114` apply 后补一次 Taken 合成；池行解析补上 `TargetCnt`/`FinishCnt` 供合成与 finish 判定；finish 要求 `TargetCnt > 0`，避免未知进度误 finish。Runner 将 tips1 归为 deferred，调用 `MarkFmlRaceTasksUnobserved` 强制下一拍 `getTaskList`。

**Tech Stack:** Go、`go test`（`./internal/state/`、`./internal/automation/`、`./internal/runner/`、`./internal/babigame/`）

**Spec:** `docs/superpowers/specs/2026-07-17-race-taken-sync-soft-recover-design.md`

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `internal/state/types.go` | `FmlRaceTaskView` 增加 `TargetCnt`/`FinishCnt` | Modify |
| `internal/state/fml.go` | 解析池进度；`110` 空后 UID 兜底合成 Taken | Modify |
| `internal/state/fml_race.go` 或 `fml.go` | `MarkFmlRaceTasksUnobserved` | Modify |
| `internal/state/fml_race_test.go` | Taken 兜底 RED/GREEN | Modify |
| `internal/automation/shop_union.go` | finish 门禁 `TargetCnt > 0` | Modify |
| `internal/automation/shop_union_race_test.go` | 池 UID 已接不 take / 可 giveUp | Modify |
| `internal/babigame/envelope.go` | `ErrorCodeOfLangJS()` | Modify |
| `internal/babigame/response_test.go` | codeOfLangJs 解析测试 | Modify |
| `internal/runner/operation_gates.go` | `isRaceTakeAlreadyTakenError` | Modify |
| `internal/runner/operation_events.go` | tips1 → deferred + invalidate | Modify |
| `internal/runner/race_tips1_test.go` | tips1 软恢复测试 | Create |

---

### Task 1: 状态层 — 池行进度字段 + UID 兜底合成 Taken

**Files:**
- Modify: `internal/state/types.go`
- Modify: `internal/state/fml.go`
- Modify: `internal/state/fml_race_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/state/fml_race_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/state/ -run 'TestFmlRaceTakenSynthesized|TestFmlRaceTakenPrefers110' -count=1`

Expected: FAIL（`HasTask` 仍为 false，或 TaskMsId 不对）

- [ ] **Step 3: Minimal implementation**

In `internal/state/types.go`，`FmlRaceTaskView` 增加：

```go
	TargetCnt   int32  // protocol targetCnt (field 7); 0 if absent
	FinishCnt   int32  // protocol finishCnt (field 8); 0 if absent
```

In `internal/state/fml.go` `applyFmlRaceTasksLocked` 填充：

```go
			TargetCnt:   t.TargetCnt,
			FinishCnt:   t.FinishCnt,
```

在 `applyFmlLocked` 的 Taken enrich 块之后（约现有 `if s.fmlRace.Taken.HasTask { ... }` 整块之后）追加：

```go
	if !s.fmlRace.Taken.HasTask {
		if taken, ok := synthesizeFmlRaceTakenFromPool(s.fmlRace.Tasks, s.roleID); ok {
			s.fmlRace.Taken = taken
		}
	}
```

新增 helper（同文件）：

```go
func synthesizeFmlRaceTakenFromPool(tasks []FmlRaceTaskView, roleID int64) (FmlRaceTakenView, bool) {
	if roleID <= 0 {
		return FmlRaceTakenView{}, false
	}
	for _, t := range tasks {
		if t.UID != roleID || t.MsId == 0 {
			continue
		}
		taskType := t.TaskType
		if taskType == 0 {
			taskType = t.TaskId
		}
		return FmlRaceTakenView{
			TaskMsId:    t.MsId,
			TaskId:      t.TaskId,
			TaskType:    taskType,
			Score:       t.Score,
			TargetCnt:   t.TargetCnt,
			FinishCnt:   t.FinishCnt,
			ParamID:     t.ParamID,
			TargetLabel: t.TargetLabel,
			HasTask:     true,
		}, true
	}
	return FmlRaceTakenView{}, false
}
```

注意：现有 apply 顺序已是 `114` 再 `110`；`110==null` 清空后仍可走兜底。不要改成先 110 后 114 再丢兜底。

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/state/ -run 'TestFmlRaceTaken|TestFmlRaceTasks' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/types.go internal/state/fml.go internal/state/fml_race_test.go
git commit -m "$(cat <<'EOF'
fix(state): synthesize race Taken from pool UID when 110 empty

EOF
)"
```

---

### Task 2: Planner — 未知进度不 finish；池 UID 已接不 take

**Files:**
- Modify: `internal/automation/shop_union.go`
- Modify: `internal/automation/shop_union_race_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/automation/shop_union_race_test.go`:

```go
func TestUnionRaceNoTakeWhenTakenSynthesizedFromPoolUID(t *testing.T) {
	s := state.New()
	// No 110 takeTaskData; pool marks uid=999 on msId=55. Another free task exists.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":55,"4":4012,"6":[23363],"7":100,"8":10,"10":25,"12":999},{"0":56,"4":4001,"6":[23001],"7":50,"8":0,"10":30,"12":0}],"110":{}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take while holding synthesized task, ops=%+v", ops)
		}
	}
}

func TestUnionRaceNoFinishWhenTakenProgressUnknown(t *testing.T) {
	s := state.New()
	// Synthesized taken with TargetCnt=0/FinishCnt=0 must not finish.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":55,"4":4012,"6":[23363],"10":25,"12":999}],"110":{}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceFinishTask.String() {
			t.Fatalf("must not finish when TargetCnt unknown, ops=%+v", ops)
		}
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take while HasTask, ops=%+v", ops)
		}
	}
}

func TestUnionRaceGiveUpSynthesizedTakenPriorityZero(t *testing.T) {
	s := state.New()
	// task type 3017 priority 0 in testRacePolicy; unfinished progress in pool.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":55,"4":3017,"7":10,"8":1,"10":25,"12":999}],"110":{}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp for priority-0 synthesized taken, got %+v", ops)
	}
}
```

确认 `testRacePolicy()` 里 `3017` 优先级为 0（与现有 `TestUnionRaceGiveUpTakenPriorityZero` 一致）；若不一致，改用该测试同款 policy 构造。

- [ ] **Step 2: Run tests to verify fail/partial**

Run: `go test ./internal/automation/ -run 'TestUnionRaceNoTakeWhenTakenSynthesized|TestUnionRaceNoFinishWhenTaken|TestUnionRaceGiveUpSynthesized' -count=1`

Expected: `NoTake` 可能已因 Task1 通过；`NoFinish` 在未改门禁时 FAIL（`0>=0` 误 finish）；`GiveUp` 在未改「未完成」判定时可能 FAIL。

- [ ] **Step 3: Minimal implementation**

In `unionRaceOperations`（`shop_union.go`）：

1. giveUp 条件改为「有已接且未完成（含进度未知）」：

```go
	if view.Taken.HasTask && (view.Taken.TargetCnt <= 0 || view.Taken.FinishCnt < view.Taken.TargetCnt) {
```

2. finish 条件改为「有已接且目标已知且已完成」：

```go
	if view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
```

take 分支保持 `if !view.Taken.HasTask`。

- [ ] **Step 4: Run tests**

Run: `go test ./internal/automation/ -run 'TestUnionRace' -count=1`

Expected: PASS（含既有 finish/giveUp/周期刷新回归）

- [ ] **Step 5: Commit**

```bash
git add internal/automation/shop_union.go internal/automation/shop_union_race_test.go
git commit -m "$(cat <<'EOF'
fix(automation): treat unknown race progress as unfinished, not finished

EOF
)"
```

---

### Task 3: State — `MarkFmlRaceTasksUnobserved`

**Files:**
- Modify: `internal/state/fml.go`
- Modify: `internal/state/fml_race_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/state/ -run TestMarkFmlRaceTasksUnobserved -count=1`

Expected: FAIL（method undefined）

- [ ] **Step 3: Implement**

```go
// MarkFmlRaceTasksUnobserved forces the next race tick to re-fetch getTaskList
// without wiping the last observed pool snapshot.
func (s *State) MarkFmlRaceTasksUnobserved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.TasksObserved = false
}
```

- [ ] **Step 4: Pass**

Run: `go test ./internal/state/ -run TestMarkFmlRaceTasksUnobserved -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/state/fml.go internal/state/fml_race_test.go
git commit -m "$(cat <<'EOF'
feat(state): allow forcing race task pool re-sync

EOF
)"
```

---

### Task 4: babigame — 暴露 `codeOfLangJs`

**Files:**
- Modify: `internal/babigame/envelope.go`
- Modify: `internal/babigame/response_test.go`（或现有 envelope 测试文件）

- [ ] **Step 1: Write the failing test**

```go
func TestErrorCodeOfLangJS(t *testing.T) {
	d := babigame.WSResponseD{M: json.RawMessage(`{"codeOfLangJs":"fmlRace_tips1","msg":"已接取其他任务"}`)}
	if got := d.ErrorCodeOfLangJS(); got != "fmlRace_tips1" {
		t.Fatalf("ErrorCodeOfLangJS=%q", got)
	}
	if d.ErrorMsg() != "已接取其他任务" {
		t.Fatalf("ErrorMsg=%q", d.ErrorMsg())
	}
}
```

（按测试文件所在 package 调整 `babigame.` 前缀。）

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/babigame/ -run TestErrorCodeOfLangJS -count=1`

Expected: FAIL

- [ ] **Step 3: Implement**

In `envelope.go`：

```go
// ErrorCodeOfLangJS returns codeOfLangJs from M when present.
func (d WSResponseD) ErrorCodeOfLangJS() string {
	m, ok := d.parseErrorMessage()
	if !ok {
		return ""
	}
	return strings.TrimSpace(m.CodeOfLangJS)
}
```

- [ ] **Step 4: Pass**

Run: `go test ./internal/babigame/ -run TestErrorCodeOfLangJS -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/babigame/envelope.go internal/babigame/response_test.go
git commit -m "$(cat <<'EOF'
feat(babigame): expose ErrorCodeOfLangJS for soft-recover matching

EOF
)"
```

---

### Task 5: Runner — tips1 软恢复

**Files:**
- Modify: `internal/runner/operation_gates.go`
- Modify: `internal/runner/operation_events.go`
- Create: `internal/runner/race_tips1_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/runner/race_tips1_test.go`:

```go
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestHandleOperationErrorRaceTakeTips1SoftRecover(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"114":[{"0":1,"4":4001,"10":9}]}}`))
	if !r.state.FmlRace().TasksObserved {
		t.Fatal("seed TasksObserved")
	}

	var events []Event
	r.emitFn = func(e Event) { events = append(events, e) }

	tipsErr := &babigame.RPCServerError{
		Name: clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{
			M: json.RawMessage(`{"codeOfLangJs":"fmlRace_tips1","msg":"已接取其他任务"}`),
		},
	}
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFmlRaceTakeTask.String(),
		Lane:     automation.LaneSide,
		Category: "union",
		Domain:   "union.race",
		Action:   "take",
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              tipsErr,
		finishedAt:       time.Now(),
	})
	if got != nil {
		t.Fatalf("tips1 must soft-recover (nil), got %v", got)
	}
	if r.state.FmlRace().TasksObserved {
		t.Fatal("tips1 must MarkFmlRaceTasksUnobserved")
	}
	var deferred, failed bool
	for _, e := range events {
		if e.Kind == "operation_deferred" {
			deferred = true
		}
		if e.Kind == "operation_failed" {
			failed = true
		}
	}
	if !deferred || failed {
		t.Fatalf("want deferred only, events=%+v", events)
	}
}

func TestIsRaceTakeAlreadyTakenError(t *testing.T) {
	tips := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"codeOfLangJs":"fmlRace_tips1","msg":"已接取其他任务"}`)},
	}
	if !isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), tips) {
		t.Fatal("expected match")
	}
	other := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlRaceTakeTask,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"codeOfLangJs":"fmlRace_other","msg":"其他"}`)},
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), other) {
		t.Fatal("must not match other codes")
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceFinishTask.String(), tips) {
		t.Fatal("must not match other RPCs")
	}
	if isRaceTakeAlreadyTakenError(clientproto.RPCFmlRaceTakeTask.String(), errors.New("已接取其他任务")) {
		t.Fatal("plain error without codeOfLangJs must not match")
	}
}
```

若 `Runner` 无 `emitFn`，改为收集 `r.events` 或现有测试同款断言方式（先搜 `newOperationEventTestRunner` / emit hook；没有则临时挂 `r.onEvent` 或直接断言返回值 + `TasksObserved`，事件 kind 用现有 loop 测试模式）。

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/runner/ -run 'TestHandleOperationErrorRaceTakeTips1|TestIsRaceTakeAlreadyTakenError' -count=1`

Expected: FAIL

- [ ] **Step 3: Implement**

`operation_gates.go`：

```go
func isRaceTakeAlreadyTakenError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceTakeTask.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if !errors.As(err, &rpcErr) || rpcErr == nil {
		return false
	}
	return rpcErr.Envelope.ErrorCodeOfLangJS() == "fmlRace_tips1"
}
```

`operation_events.go`：新增 kind + classify 分支 + handle：

```go
	operationErrorRaceTakeAlreadyTaken operationErrorKind = "race_take_already_taken"
```

在 `classifyOperationError`：

```go
	case isRaceTakeAlreadyTakenError(kind, err):
		return operationErrorRaceTakeAlreadyTaken
```

在 `handleOperationError` switch：

```go
	case operationErrorRaceTakeAlreadyTaken:
		r.state.MarkFmlRaceTasksUnobserved()
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示已接取其他任务，将重新同步任务池后继续", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "race_taken_resync"})
		return nil
```

要点：`return nil` → `opErr` 为空 → 不写 `LastOperationError`（账号异常路径）。不加 take 冷却。

- [ ] **Step 4: Pass + 对齐 emit 收集方式**

若测试因 emit hook 编译失败：读 `Runner` 结构，用与 `loop_test.go` / 其他 soft-recover 测试相同方式收集事件；保持断言语义不变。

Run: `go test ./internal/runner/ -run 'TestHandleOperationError|TestIsRaceTakeAlreadyTaken' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/runner/operation_gates.go internal/runner/operation_events.go internal/runner/race_tips1_test.go
git commit -m "$(cat <<'EOF'
fix(runner): soft-recover fmlRace_tips1 by resyncing race task pool

EOF
)"
```

---

### Task 6: 全量相关回归 + 更新设计进度

**Files:**
- Modify: `docs/superpowers/specs/2026-07-17-race-taken-sync-soft-recover-design.md`（进度表勾完）

- [ ] **Step 1: Run focused suites**

```bash
go test ./internal/state/ ./internal/automation/ ./internal/runner/ ./internal/babigame/ -count=1
```

Expected: PASS

- [ ] **Step 2: Update design progress table**

将设计文档「实现进度」全部标为 ✅，状态改为「已实现」。

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-07-17-race-taken-sync-soft-recover-design.md
git commit -m "$(cat <<'EOF'
docs: mark race taken sync soft-recover as implemented

EOF
)"
```

---

## Self-Review

1. **Spec coverage:** §2 Taken 兜底 → Task1；§1 finish/giveUp/take → Task2；§3 tips1 → Task3–5；§4 强制 sync 优先 TTL → `TasksObserved=false` 走早退 getTaskList，不改周期刷新代码。
2. **Placeholder scan:** 无 TBD；Task5 emit hook 若仓库无现成字段，步骤内要求对齐现有测试模式而非占位。
3. **Type consistency:** `MarkFmlRaceTasksUnobserved`、`ErrorCodeOfLangJS`、`isRaceTakeAlreadyTakenError`、`operationErrorRaceTakeAlreadyTaken` 命名前后一致。
4. **0/0 陷阱：** 设计允许合成时进度为 0；Task2 明确 finish 需 `TargetCnt > 0`，giveUp 将未知进度视为未完成。
