# 顾客订单按库存制作、无库存才拒单 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将顾客订单判定流程从「交付 → 未培育/未解锁拒单 → craft」改为「交付 → craft → 无库存拒单」，移除培育/解锁/状态不完整作为拒单触发条件。

**Architecture:** 仅改动 `internal/automation/automation.go` 的顾客订单分支（1200-1234 行），重排判定顺序：先交付、再 craft、最后无库存拒单。`customerOrderUnavailableReasons` 函数保留为 dead code 但不再调用。

**Tech Stack:** Go 1.x, gRPC/Connect, protobuf, `testing` 包

**Spec:** `docs/superpowers/specs/2026-07-06-customer-order-inventory-reject-design.md`

---

## File Structure

| 文件 | 责任 | 改动类型 |
| --- | --- | --- |
| `internal/automation/automation.go` | 顾客订单分支判定流程 | 修改 1200-1234 行 |
| `internal/automation/automation_test.go` | 顾客订单测试用例 | 修改 3 个现有测试 + 新增 1 个测试函数 |

---

### Task 1: 修改现有测试以反映新行为（TDD - 先让测试失败）

**Files:**
- Modify: `internal/automation/automation_test.go:307-334` (`TestBuildPlan_CustomerArtBlockedByMissingVase`)
- Modify: `internal/automation/automation_test.go:336-364` (`TestBuildPlan_CustomerRejectUnavailableWhenUnlockedRequirementsMissing`)
- Modify: `internal/automation/automation_test.go:366-395` (`TestBuildPlan_CustomerMissingRecipeBlocksWithoutReject`)

- [ ] **Step 1: 修改 `TestBuildPlan_CustomerArtBlockedByMissingVase`**

该测试原来期望「花瓶未解锁 → blocked with 花瓶 reason」。新逻辑下 `RejectUnavailableEnabled` 未设置（false），应产生 blocked，理由为「库存不足且无法制作，reject_unavailable_enabled 未开启」。

将 `internal/automation/automation_test.go:325-333` 的断言部分替换为：

```go
	result := BuildPlan(s, p, time.Now())
	var blocked bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() && !op.Executable {
			if !hasReasonContaining(op.BlockedReasons, "reject_unavailable_enabled 未开启") {
				t.Fatalf("expected reject_unavailable_enabled block, ops=%+v", op)
			}
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("expected missing-vase blocked op, ops=%+v", result.Operations)
	}
}
```

- [ ] **Step 2: 修改 `TestBuildPlan_CustomerRejectUnavailableWhenUnlockedRequirementsMissing`**

该测试原来期望「花瓶未解锁 → reject with 花瓶 reason」。新逻辑下 `RejectUnavailableEnabled = true`，craft 不可制作（花瓶未解锁）→ 落入拒单分支，理由为「库存不足且无法制作」。

将 `internal/automation/automation_test.go:354-364` 的断言部分替换为：

```go
	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if !op.Executable || op.TargetID != 7 || !strings.Contains(op.Reason, "库存不足且无法制作") {
				t.Fatalf("reject op mismatch: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer reject op: %+v", result.Operations)
}
```

- [ ] **Step 3: 修改 `TestBuildPlan_CustomerMissingRecipeBlocksWithoutReject`**

该测试原来期望「缺花艺配方 → blocked with 配方 reason，不拒单」。新逻辑下 `RejectUnavailableEnabled = true`，craft 无法匹配配方 → 落入拒单分支。测试名和断言都需更新。

将函数名改为 `TestBuildPlan_CustomerMissingRecipeRejectsWhenEnabled`，并将 `internal/automation/automation_test.go:382-395` 的断言部分替换为：

```go
	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if !op.Executable || !strings.Contains(op.Reason, "库存不足且无法制作") {
				t.Fatalf("missing recipe should reject when enabled: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer reject op: %+v", result.Operations)
}
```

- [ ] **Step 4: 运行测试验证它们失败**

Run: `cd /Users/kk/work/garden/mygardenworld && go test ./internal/automation/ -run TestBuildPlan_Customer -v`

Expected: 3 个修改的测试 FAIL（因为实现尚未改动，仍走旧流程）

- [ ] **Step 5: Commit**

```bash
cd /Users/kk/work/garden/mygardenworld
git add internal/automation/automation_test.go
git commit -m "test: 更新顾客订单测试以反映按库存拒单的新行为

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: 实现新的顾客订单判定流程

**Files:**
- Modify: `internal/automation/automation.go:1200-1234`

- [ ] **Step 1: 替换顾客订单分支的判定逻辑**

将 `internal/automation/automation.go:1200-1234` 的整个 `if goal, ok := goalByID(goals, GoalCustomerOrder); ok { ... }` 块替换为：

```go
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		for npcID, customerOrder := range s.CustomerOrderDetails() {
			if canFulfillCustomerOrder(customerOrder, npcID, goal, ledger) {
				ops = append(ops, op(clientproto.RPCOrderCustomerFinishOrder.String(), goal, "finish", "顾客订单可交付", customerOperationPriority(goal, 200), npcID, 0, 0))
				continue
			}
			if craft, ok := craftOperationForCustomerOrder(s, customerOrder, npcID, goal, demands, ledger); ok && craft.Executable {
				ops = append(ops, craft)
				continue
			}
			if order.GetCustomer().GetRejectUnavailableEnabled() {
				reject := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", "顾客订单库存不足且无法制作，执行暂时无货", customerOperationPriority(goal, 180), npcID, 0, 0)
				ops = append(ops, reject)
				continue
			}
			blocked := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", "顾客订单库存不足且无法制作，等待策略允许暂时无货", goal.Priority*100+130, npcID, 0, 0)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{"order.customer.reject_unavailable_enabled 未开启", "库存不足且无法制作"}
			ops = append(ops, blocked)
		}
		if s.CustomerOrderGenerationReady(now) {
			ops = append(ops, op(clientproto.RPCOrderCustomerGenOrder.String(), goal, "generate", "顾客订单为空且刷新时间已到，生成顾客订单", customerOperationPriority(goal, 190), 0, 0, 0))
		}
	}
```

- [ ] **Step 2: 运行测试验证通过**

Run: `cd /Users/kk/work/garden/mygardenworld && go test ./internal/automation/ -run TestBuildPlan_Customer -v`

Expected: 所有 `TestBuildPlan_Customer*` 测试 PASS

- [ ] **Step 3: 运行完整 automation 包测试**

Run: `cd /Users/kk/work/garden/mygardenworld && go test ./internal/automation/ -v`

Expected: 全部 PASS，无 regression

- [ ] **Step 4: Commit**

```bash
cd /Users/kk/work/garden/mygardenworld
git add internal/automation/automation.go
git commit -m "feat: 顾客订单按库存制作、无库存才拒单

移除培育/解锁/状态不完整作为拒单触发条件，统一为「无法交付且无法制作」才拒单。
判定顺序改为：交付 → craft → 无库存拒单。

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: 新增边界用例测试

**Files:**
- Modify: `internal/automation/automation_test.go` (在 `TestBuildPlan_CustomerEmptyOrdersRespectGenerationCooldown` 之后插入)

- [ ] **Step 1: 新增「纯花朵订单库存不足开关关 → blocked」测试**

在 `internal/automation/automation_test.go` 的 `TestBuildPlan_CustomerEmptyOrdersRespectGenerationCooldown` 函数之后（约 485 行后），插入：

```go
func TestBuildPlan_CustomerFlowerOrderBlockedWhenNoInventoryAndSwitchOff(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{},
			"34": 12,
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 23005, "2": 1, "3": 1},
		}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Order.Customer.RejectUnavailableEnabled = false

	result := BuildPlan(s, p, time.Now())
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() {
			if op.Executable {
				t.Fatalf("should not execute reject when switch off: %+v", op)
			}
			if !hasReasonContaining(op.BlockedReasons, "reject_unavailable_enabled 未开启") {
				t.Fatalf("expected reject_unavailable_enabled block: %+v", op)
			}
			return
		}
	}
	t.Fatalf("missing customer blocked op: %+v", result.Operations)
}
```

- [ ] **Step 2: 运行新测试验证通过**

Run: `cd /Users/kk/work/garden/mygardenworld && go test ./internal/automation/ -run TestBuildPlan_CustomerFlowerOrderBlockedWhenNoInventoryAndSwitchOff -v`

Expected: PASS

- [ ] **Step 3: Commit**

```bash
cd /Users/kk/work/garden/mygardenworld
git add internal/automation/automation_test.go
git commit -m "test: 新增纯花朵订单库存不足开关关 blocked 用例

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: 全量验证

**Files:**
- 无文件改动，仅运行验证命令

- [ ] **Step 1: 运行全量测试**

Run: `cd /Users/kk/work/garden/mygardenworld && go test ./...`

Expected: 全部 PASS，无 regression

- [ ] **Step 2: 运行 lint**

Run: `cd /Users/kk/work/garden/mygardenworld && go vet ./internal/automation/`

Expected: 无 warning

- [ ] **Step 3: 检查 `customerOrderUnavailableReasons` 是否还有调用方**

Run: `cd /Users/kk/work/garden/mygardenworld && grep -rn "customerOrderUnavailableReasons" internal/`

Expected: 仅在 `automation.go:2189` 的函数定义处出现，无调用点。函数已为 dead code，保留以减小改动面（spec 中明确允许）。

- [ ] **Step 4: 最终 commit（如有 lint 修复）**

如果前述步骤有修复，commit；否则跳过。

```bash
cd /Users/kk/work/garden/mygardenworld
git add -A
git commit -m "chore: lint 修复

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```
