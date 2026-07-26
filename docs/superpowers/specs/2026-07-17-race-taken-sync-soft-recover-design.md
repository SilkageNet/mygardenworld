# 竞赛登录同步已接任务 + takeTask tips1 软恢复

**日期：** 2026-07-17
**状态：** 已实现
**实现计划：** `docs/superpowers/plans/2026-07-17-race-taken-sync-soft-recover.md`

## 实现进度

| 阶段 | 状态 | 说明 |
|------|------|------|
| 设计批准 | ✅ 完成 | 方案 1：状态层补全已接 + runner tips1 软恢复 |
| 代码摸底 | ✅ 完成 | 已定位 `applyFml`/`parseFmlRaceTaken`、`unionRaceOperations`、`handleOperationError`；tips1 对应 `codeOfLangJs`，非数值 `code` |
| 实现计划 | ✅ 完成 | `docs/superpowers/plans/2026-07-17-race-taken-sync-soft-recover.md`（Task 1–6） |
| Task1：状态层 UID 兜底合成 Taken | ✅ 完成 | 池行补 `TargetCnt`/`FinishCnt`；`synthesizeFmlRaceTakenFromPool` |
| Task2：finish/giveUp 门禁 | ✅ 完成 | 未知进度视为未完成，不误 finish |
| Task3：`MarkFmlRaceTasksUnobserved` | ✅ 完成 | 强制下一拍重新 `getTaskList` |
| Task4：`ErrorCodeOfLangJS` | ✅ 完成 | 暴露 `codeOfLangJs` 供软恢复匹配 |
| Task5：runner tips1 软恢复 | ✅ 完成 | `isRaceTakeAlreadyTakenError` + deferred + 强制再 sync，不写账号异常 |
| Task6：回归 + 文档收尾 | ✅ 完成 | 全量相关回归通过，设计文档标记已实现 |

## 问题

账号登录并开启竞赛自动化后，流程应是：先同步当前是否已接任务 → 有则按规则继续或放弃 → 无则从任务池选接。
实际出现：`getTaskList`（同步竞赛任务）显示完成，下一拍仍规划 `takeTask`，服务端返回 `fmlRace_tips1` /「已接取其他任务」，账号被标为异常。根因是本地 `Taken.HasTask` 与服务端不一致：同步成功但未正确识别已接任务。

## 目标

1. 竞赛开启时，登录后先同步再决策：有已接则校验规则（符合继续 / 不符合放弃）；无已接才选池接取。
2. 已接判定：主信字段 `110`（`takeTaskData`）；若缺失，用任务池 `UID == 自己` 的行兜底合成 `Taken`。
3. `takeTask` 命中 `fmlRace_tips1` 时软恢复：不打账号异常，强制再同步，下一拍走已接分支。

## 非目标

- 不改 give-up 冷却时长、分数/优先级/种植可种等既有规则语义。
- 不扩大其他竞赛 RPC 的软恢复范围。
- 不新增 policy 开关。
- 不做 UI 大改（异常消失即可；靠 deferred 日志可观测）。
- 不改定时 10 分钟任务池刷新、CD lead take 行为。

## 方案（状态层补全已接 + tips1 软恢复）

### 1. 登录后生命周期

竞赛 `enabled` 时：

1. 未观测批次 → `fmlRace.enter`
2. 批次激活且任务池未观测（或缺参需补拉）→ `fmlRace.getTaskList`
3. 完成上述同步之前，不规划 `takeTask`
4. 同步应用后按已接判定分支：
   - **有已接且未完成：** 不符合既有放弃规则 → `giveUpTask`；符合 → 不 take，由进度驱动继续
   - **有已接且已完成：** `finishTask`
   - **无已接：** `selectRaceTasks` 选最优 → `takeTask`
5. 同一拍竞赛主操作至多一个，优先级：`giveUp` → `finish` → `take`

（与现有 `unionRaceOperations` 结构对齐；本设计补齐状态识别与 tips1 防护。）

### 2. 已接判定（状态层）

在 namespace-25 apply（字段 `110` / `114`）完成后：

| 优先级 | 条件 | 结果 |
|--------|------|------|
| 1 | `110` 解析出有效 `takeTaskData`（`TaskMsId != 0`） | `Taken.HasTask = true`（现有逻辑） |
| 2 | 否则，任务池存在 `UID == RoleID` 的行 | 用该行合成 `Taken`（`TaskMsId` / `TaskId` / `TaskType` / `Score` / `ParamID` 等；`FinishCnt`/`TargetCnt` 未知时保持 0，后续 sync 可补全） |
| 3 | 两处皆无 | `HasTask = false`，允许 take |

合成后的 `Taken` 须参与现有 giveUp / finish / take 门禁，使 UI 快照与 planner 一致。

`110` 为 JSON `null` 仍清空 `Taken`（现有语义），但同一次 apply 若 `114` 含 `UID == 自己`，应在清空后再走兜底合成，避免「null 110 + 池内已接」被误判为空。

### 3. tips1 软恢复（runner）

仅针对 `fmlRace.takeTask` 返回的服务端错误 `code == "fmlRace_tips1"`（已接取其他任务）：

1. 归类为可恢复错误：发 `operation_deferred`（warn），**不**发 `operation_failed`，从而不写入导致账号「异常」的 `LastError` 路径。
2. 将 `TasksObserved` 置为 `false`（沿用现有「批次激活且未观测任务池 → getTaskList」早退），下一拍只同步、不 take。
3. 不另加 take 冷却：同步完成并重新判定已接后，自然恢复正常决策。
4. 再同步后走第 1 节已接分支。若仍识别不出已接且再次 tips1：继续软恢复 + 再同步，不升级为账号异常。

其他 `takeTask` / 竞赛 RPC 错误保持现有失败行为。

### 4. 与定时刷新的关系

- 本设计的「强制再 sync」优先于 10 分钟 TTL 空闲刷新。
- 本 tick 已发 giveUp / finish / take 时仍不发定时 sync（既有规则）；tips1 软恢复触发的强制 sync 在**失败处理当拍之后的后续 tick**发出，不与同拍 take 并存。

## 测试

1. `110` 有 `takeTaskData` → `Taken.HasTask`。
2. `110` 空/缺，`114` 有 `UID == 自己` → 兜底合成 `Taken`；planner 不发 take。
3. 同一次 apply：`110` 为 null 且池内有自己的 UID → 仍合成 `Taken`。
4. Planner：符合规则的已接 → 不 take；不符合 → giveUp；已完成 → finish；无已接 → 可 take。
5. Runner：`takeTask` + tips1 → deferred（非 failed）、强制再 sync、不写账号异常。
6. 回归：10 分钟池刷新、CD lead、种植收获可种门禁行为不变。

## 已锁定决策

| 议题 | 选择 |
|------|------|
| 范围 | 修状态识别 + 钉死登录后生命周期 + tips1 软恢复 |
| tips1 处理 | 软恢复（deferred，不打异常，强制再 sync） |
| 已接判定 | `110` 为主；池 `UID == 自己` 兜底 |
| 实现路径 | 方案 1：状态层补全 + runner 软恢复（非仅 planner 拦 take） |
