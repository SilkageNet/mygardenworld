# Guild race task pool periodic refresh + UI timestamps

**Date:** 2026-07-15  
**Status:** approved (approach 1)

## Problem

Operators cannot tell when the race task pool was last synced, and the daemon only calls `fmlRace.getTaskList` on first observation (or a one-shot missing-param refresh). Pool rows can go stale for a long time (new slots, score changes, param fills) while the UI still looks current.

## Goal

1. Periodically re-fetch the task pool every **10 minutes** while a race batch is active.
2. Show on the Web task-pool header: last sync time and the fixed refresh interval copy.
3. Never let the periodic refresh steal a tick from taking an eligible task (including preemptive CD take within the existing lead window).

## Non-goals

- Do not add a policy-configurable interval.
- Do not change CD / `selectRaceTasks` scoring or skip-reason rules.
- Do not change first-sync or missing-param refresh semantics beyond coexistence with TTL refresh.
- Do not refresh when the race module is disabled or the batch is inactive.

## Approach (hardcoded 10m + local `TasksSyncedAtMs`)

### State

- Add `FmlRaceView.TasksSyncedAtMs int64`: local wall time when field `114` (`FmlRaceTaskList`) was last applied successfully.
- Set on every successful `applyFmlRaceTasksLocked` path that marks `TasksObserved` (including empty/null pool).
- Do **not** advance the timestamp merely because a sparse take/finish delta merged into the pool without a full list replace — only when the apply path processes a task-list payload. Practical rule: set `TasksSyncedAtMs = s.lastApplyMs` (or `time.Now().UnixMilli()` at apply) inside `applyFmlRaceTasksLocked` whenever the function accepts the raw payload (including null → empty pool). Sparse merges still update the stamp because they arrive as field 114; that is acceptable freshness.

### Automation (`unionRaceOperations`)

Constant: `raceTaskPoolRefreshInterval = 10 * time.Minute`.

Keep existing **blocking** early returns for:

1. `!Observed` → `enter`
2. `BatchActive && (!TasksObserved || raceTaskPoolNeedsParamRefresh(view))` → `getTaskList`

After auto-module / batch gates, build giveUp / finish / take / upgrade / delete as today.

**Periodic sync** (separate from the early returns):

- Condition: `BatchActive && TasksObserved && now - TasksSyncedAtMs >= 10m` (treat `TasksSyncedAtMs == 0` as stale once observed, so a one-time sync is still emitted).
- Emit `getTaskList` only when this tick produced **no** higher-priority race mutating ops: giveUp, finish, or take.
- Upgrade / delete do not block periodic sync (they are optional hygiene). If both sync and upgrade would fire, emit sync this tick and leave upgrade for the next cycle after the pool updates — preferred: **if periodic sync is due and no giveUp/finish/take, emit only sync** (drop upgrade/delete that same tick) to avoid acting on a pool that is about to be replaced.

Pseudo-order:

```
enter / first&param sync (early return)
if !auto || !batch → stop
emit giveUp / finish / take as today
if any of those emitted → return (no periodic sync)
if TTL stale → emit getTaskList and return
else emit upgrade / delete as today
```

This guarantees: eligible ready or preemptive take always wins over 10-minute refresh.

### API / Web

- Proto `FmlRaceView`: add `int64 tasks_synced_at_ms = 8` (next free field after `batch_status = 7`).
- Optional constant for UI: hardcode Chinese copy `每 10 分钟重新获取` (interval is not policy-driven; no need to plumb interval via proto unless useful for tests — UI may hardcode).
- Query mapper copies `TasksSyncedAtMs`.
- Task pool header: when `tasks_synced_at_ms > 0`, show muted text like `更新于 HH:mm · 每 10 分钟重新获取` next to the pool title / count badge. When unset, omit the update time (or show only the interval line after first sync — prefer omit until known).

### Testing

1. After first `getTaskList` apply, `TasksSyncedAtMs > 0`.
2. Within 10 minutes, with no take pending, no periodic `getTaskList`.
3. After 10 minutes idle (no taken task, empty eligible take set), emit periodic `getTaskList`.
4. After 10 minutes but an eligible take (including upcoming within lead) exists → emit take, **not** sync.
5. After 10 minutes with finishable taken task → emit finish, not sync.
6. Proto/UI: snapshot exposes `tasks_synced_at_ms`; panel renders update + interval copy.

## Decisions locked

| Topic | Choice |
|-------|--------|
| Interval | Fixed 10 minutes |
| Configurability | Code constant only |
| Conflict with CD take | Take wins; periodic sync only when idle of giveUp/finish/take |
| UI | Pool header: last sync + “每 10 分钟重新获取” |
