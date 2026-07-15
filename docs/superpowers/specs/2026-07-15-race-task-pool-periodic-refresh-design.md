# 公会竞赛任务池定时刷新 + UI 更新时间

**日期：** 2026-07-15  
**状态：** 已批准（方案 1）

## 问题

运维端无法判断竞赛任务池上次何时同步；daemon 目前只在首次观测到任务池时调用 `fmlRace.getTaskList`（或缺参时补拉一次）。池内条目可能长时间过期（新坑位、分数变化、param 补全），而 UI 看起来仍像是最新的。

## 目标

1. 竞赛批次进行中时，每 **10 分钟** 重新拉取一次任务池。
2. 在 Web 任务池标题区展示：上次同步时间，以及固定的刷新间隔文案。
3. 定时刷新与「即将可接的 CD 任务」的优先级按剩余 CD 时长切分（见下方）。

## 非目标

- 不增加可配置（policy）的刷新间隔。
- 不改动 CD / `selectRaceTasks` 的排序与 skip-reason 规则（含既有 `raceTakeLeadWindow` 抢先 take）。
- 除与 TTL 刷新并存外，不改动首次同步、缺参重拉的语义。
- 竞赛模块关闭或批次未激活时不刷新。
- 不把抢先 take 窗口扩大到 10 分钟；「剩余 CD 不足 10 分钟」只用于**推迟定时 sync**，真正发 take 仍遵循现有 lead。

## 方案（硬编码 10 分钟 + 本地 `TasksSyncedAtMs`）

### 状态

- 新增 `FmlRaceView.TasksSyncedAtMs int64`：本地墙上时钟，表示字段 `114`（`FmlRaceTaskList`）上次成功 apply 的时间。
- 在 `applyFmlRaceTasksLocked` 所有会标记 `TasksObserved` 的成功路径上写入（含空/null 空池）。
- **不要**仅因 take/finish 的稀疏 delta 合并进池、且未走完整列表替换就额外推进时间戳——只应在 apply 路径处理到任务池 payload 时更新。实务规则：在 `applyFmlRaceTasksLocked` 内，只要函数接受了该 raw payload（含 null → 空池），就设 `TasksSyncedAtMs = s.lastApplyMs`（或 apply 时的 `time.Now().UnixMilli()`）。稀疏合并若也走 field 114，同样会更新时间戳——作为新鲜度信号可接受。

### 自动化（`unionRaceOperations`）

常量：`raceTaskPoolRefreshInterval = 10 * time.Minute`。

保留现有**阻塞式**早退：

1. `!Observed` → `enter`
2. `BatchActive && (!TasksObserved || raceTaskPoolNeedsParamRefresh(view))` → `getTaskList`

通过 auto-module / batch 门控后，按现有逻辑组装 giveUp / finish / take / upgrade / delete。

**定时同步**（与早退分开）：

- TTL 条件：`BatchActive && TasksObserved && now - TasksSyncedAtMs >= 10m`（一旦已观测，将 `TasksSyncedAtMs == 0` 视为过期，仍会发出一次同步）。
- 本 tick 已发出 giveUp / finish / take → **不发**定时 sync（接完/交完之后的后续 tick，若 TTL 仍到期，再拉）。
- upgrade / delete：若本 tick 决定发定时 sync，则只发 sync、丢掉同 tick 的 upgrade/delete。

**与 CD 可接任务的关系（按剩余 CD）**

定义「可接 CD 任务」：仍在 CD（`AppearTime > now`），且撤掉 CD 门禁后现有 take 过滤器会通过（分数 / 优先级 / 升级 / 种植可种等与 `RaceTakeSkipReason` 非 CD 分支一致）。剩余 CD：`rem = AppearTime - now`。

| 情况 | 定时 sync 到期时 |
|------|------------------|
| 存在可接 CD 且 `rem >= 10m` | **正常刷新**（不为这种远 CD 让路） |
| 存在可接 CD 且 `0 < rem < 10m` | **推迟刷新**：本 tick 不发 sync；等之后某 tick 真正 `take`（含 lead 内抢先）完成之后，再在空闲 tick 补拉 |
| 存在已 ready、本可 take 的任务 | 先 take，再刷新（同「推迟」） |
| 无可接 CD / 无可 take | 正常发 `getTaskList` |

若同时存在「远 CD（剩余 ≥10m）」与「近 CD（剩余不足 10m）」的可接任务：以近 CD 为准，**推迟** sync。

伪顺序：

```
enter / 首次&缺参同步（早退）
if !auto || !batch → 停止
按现有逻辑发出 giveUp / finish / take
若上述任一已发出 → return（不发定时 sync）
若 TTL 过期：
  若存在可接 CD 且 rem < 10m → 跳过 sync，继续 upgrade/delete（或直接 return 更干净：推荐跳过 sync 后仍可 upgrade/delete）
  否则 → 发出 getTaskList 并 return
否则按现有逻辑发出 upgrade / delete
```

说明：剩余 CD 在 (lead, 10m) 期间不会发 take，只是守着池子不换；进入 lead 后按现有逻辑抢先 take，take 发出的那一 tick 仍不 sync；take 成功后的后续空闲 tick 再 sync。

### API / Web

- Proto `FmlRaceView`：新增 `int64 tasks_synced_at_ms = 8`（`batch_status = 7` 之后的下一个空闲字段）。
- UI 间隔文案硬编码为 `每 10 分钟重新获取`（间隔不由 policy 驱动；不必经 proto 下发，除非测试需要——UI 可硬编码）。
- Query 映射拷贝 `TasksSyncedAtMs`。
- 任务池标题：当 `tasks_synced_at_ms > 0` 时，在标题 / 数量徽章旁显示弱化文案，例如 `更新于 HH:mm · 每 10 分钟重新获取`。未设置时省略更新时间（优先：未知则不显示）。

### 测试

1. 首次 `getTaskList` apply 后，`TasksSyncedAtMs > 0`。
2. 10 分钟内、无待接任务时，不发出定时 `getTaskList`。
3. 空闲满 10 分钟（无已接、无可 take、无可接近 CD）→ 发出定时 `getTaskList`。
4. 满 10 分钟且存在可 take（ready 或 lead 内）→ 发 take，**不**发 sync。
5. 满 10 分钟且已接可交 → 发 finish，不发 sync。
6. 满 10 分钟、唯一可接 CD 的 `rem >= 10m` → **发**定时 sync（不为远 CD 让路）。
7. 满 10 分钟、存在可接 CD 且 `rem < 10m`（哪怕尚未进入 lead、本 tick 不能 take）→ **不**发 sync；待之后 take 完成且空闲后再发。
8. Proto/UI：快照暴露 `tasks_synced_at_ms`；面板渲染更新时间 + 间隔文案。

## 已锁定决策

| 议题 | 选择 |
|------|------|
| 间隔 | 固定 10 分钟 |
| 可配置性 | 仅代码常量 |
| 与可接 CD 的冲突 | 剩余 CD ≥10m 正常刷新；不足 10m 先接到再刷新；本 tick 有 giveUp/finish/take 也不 sync |
| UI | 任务池标题：上次同步 +「每 10 分钟重新获取」 |
