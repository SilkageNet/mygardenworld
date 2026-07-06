# 顾客订单按库存制作、无库存才拒单

## 背景

mygardenworld 的顾客订单自动化当前判定流程为「交付 → 未培育/未解锁拒单 → craft」。当订单需求的花朵未培育或花艺花瓶未解锁时，会触发拒单（受 `reject_unavailable_enabled` 开关控制），即使库存里其实有足够的花朵/花艺成品。

用户希望改为「按库存制作、无库存才拒单」：移除培育/解锁/状态不完整作为拒单触发条件，统一为「无法交付且无法制作」才拒单。

## 目标

- 顾客订单优先按库存交付，其次按库存材料制作花艺
- 仅当无法交付且无法制作时才拒单
- 移除培育/解锁状态作为拒单触发条件
- 保留 `reject_unavailable_enabled` 开关语义（开→拒单，关→blocked）
- 状态不完整（订单为空、FlowerID/Count<=0、缺花艺配方等）也走拒单分支，不再单独 blocked

## 非目标

- 不改策略 schema（`CustomerOrderPolicy` 字段不变）
- 不改 `features.go` 功能目录
- 不改 Web UI
- 不清理 `customerOrderUnavailableReasons` dead code（后续可单独清理）
- 不改其他订单类型（居民、宫廷、组团）

## 改动范围

唯一改动文件：`internal/automation/automation.go` 的顾客订单分支（当前 1200-1234 行）。

## 设计

### 现有流程（automation.go:1200-1234）

```
for npcID, customerOrder := range s.CustomerOrderDetails() {
    if canFulfillCustomerOrder(...) { → finish; continue }
    rejectable, blockedReasons := customerOrderUnavailableReasons(s, customerOrder)
    if len(rejectable) > 0 {
        if reject_unavailable_enabled { → reject; continue }
        → blocked (reject_unavailable_enabled 未开启); continue
    }
    if len(blockedReasons) > 0 { → blocked (状态不完整); continue }
    if craft, ok := craftOperationForCustomerOrder(...); ok { ops = append(ops, craft) }
}
if s.CustomerOrderGenerationReady(now) { → generate }
```

### 新流程

```
for npcID, customerOrder := range s.CustomerOrderDetails() {
    if canFulfillCustomerOrder(...) { → finish; continue }
    if craft, ok := craftOperationForCustomerOrder(...); ok && craft.Executable {
        ops = append(ops, craft); continue
    }
    // 无法交付且无法制作
    if order.GetCustomer().GetRejectUnavailableEnabled() {
        → reject (理由: 库存不足且无法制作); continue
    }
    → blocked (理由: 库存不足且无法制作，reject_unavailable_enabled 未开启)
}
if s.CustomerOrderGenerationReady(now) { → generate }
```

### 关键变化

1. **移除 `customerOrderUnavailableReasons` 调用**：函数保留为 dead code，但不再被调用
2. **craft 分支上移**：从流程末尾移到拒单判定之前
3. **craft 不可制作时落入拒单**：`craftOperationForCustomerOrder` 返回 `(blocked, true)` 时，新逻辑检查 `craft.Executable`，若为 false 则不 append，落入拒单分支
4. **拒单理由统一**为 `"库存不足且无法制作"`
5. **blocked 理由统一**为 `"库存不足且无法制作，reject_unavailable_enabled 未开启"`

### 优先级

保持现有优先级不变：
- finish: `customerOperationPriority(goal, 200)`
- generate: `customerOperationPriority(goal, 190)`
- reject: `customerOperationPriority(goal, 180)`
- blocked: `goal.Priority*100 + 130`

### 边界情况

- **纯花朵订单、库存不足**：`canFulfillCustomerOrder` 返回 false，`craftOperationForCustomerOrder` 只处理 `ItemRequires` 会返回 false → 拒单分支
- **花艺订单、材料不够**：`craftOperationForCustomerOrder` 返回 `(blocked, true)`，`Executable=false` → 拒单分支
- **订单无任何可识别需求**：`canFulfillCustomerOrder` 返回 false（`hasRequirements=false`），craft 返回 false → 拒单分支
- **`customerOrder == nil`**：`CustomerOrderDetails()` 已过滤 nil，循环内不会遇到

## 测试

### 需修改的现有测试（internal/automation/automation_test.go）

- 第 363 行 `missing customer reject op`：验证拒单 op 存在，新逻辑下仍成立（库存不足→拒单），但拒单理由需更新为「库存不足且无法制作」
- 第 429 行 `customer order should not be rejected by flower art cfg lvl`：需确认在新逻辑下是否仍不被拒——如果库存足够交付，则 finish；如果库存不足，则拒单。需根据测试数据确认
- 第 482 行 `customer gen should wait for cooldown`：与生成冷却相关，不受影响

### 需新增的测试用例

1. 纯花朵订单、库存不足、开关开 → reject，理由「库存不足且无法制作」
2. 纯花朵订单、库存不足、开关关 → blocked，理由「库存不足且无法制作，reject_unavailable_enabled 未开启」
3. 花艺订单、材料不够、开关开 → reject（不再有 blocked craft op）
4. 花艺订单、材料够 → craft（不变）
5. 订单无需求、开关开 → reject（原为 blocked）
6. 库存足够 → finish（不变，回归用例）

## 风险

- **`customerOrderUnavailableReasons` 变为 dead code**：保留以减小改动面，后续可单独清理
- **`craftOperationForCustomerOrder` 返回 blocked op 被丢弃**：新逻辑下 blocked craft 不再 append，其携带的 `VaseID/FlowerIDs/BlockedReasons` 信息会丢失。但用户已确认「直接拒单」可接受
- **拒单理由信息变粗**：原 `customerOrderUnavailableReasons` 会返回具体哪朵花/哪个花瓶未解锁，新逻辑统一为「库存不足且无法制作」。如需更细粒度可后续增强
