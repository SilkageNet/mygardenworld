# Guild race task take skip reason (UI)

**Date:** 2026-07-15  
**Status:** approved (approach 3, single primary reason)

## Problem

The Web task pool already labels CD rows with a `CD ` title prefix and shows upgrade metadata, but operators cannot see **why automation will not take** a given pool task (score gate, upgrade filters, plant-harvest cultivation gate, far CD, already taken). Reasons are buried in `selectRaceTasks` filter branches with no shared explanation surface.

## Goal

1. Align displayed “不可接取” reasons with the account’s current `UnionRacePolicy` + observed state (same rules as automation take selection).
2. Show **one primary reason** per task (fixed priority).
3. Expose the reason to Web task cards and `query_race` CLI from a single backend field.

## Non-goals

- Do not change take / give-up / upgrade / delete selection outcomes beyond refactoring filters into a shared helper.
- Do not list every matching reason; no tooltip stack of secondary reasons.
- Do not add a reason like “竞赛自动化已关” on each card (module switch ≠ per-task take gate).
- Do not invent UI-only filters that diverge from Go automation.

## Approach

Extract `raceTakeSkipReason(s, task, policy, uid, now) string` in `internal/automation` (same package as `selectRaceTasks`):

- `""` → automation would consider the task takeable (including preemptive take inside `raceTakeLeadWindow`)
- non-empty → Chinese one-liner primary skip reason

Refactor `selectRaceTasks` to skip when `raceTakeSkipReason(...) != ""`, then keep today’s ready / upcoming partition and sort. Filtering semantics stay unchanged.

### Primary reason priority

Evaluate in order; return the first hit:

| # | Condition | Copy |
|---|-----------|------|
| 1 | `task.UID != 0` | `已被接取` |
| 2 | `AppearTime > 0` and beyond lead window (`AppearTime > now+lead`) | `冷却中，HH:mm 后可接` (local clock from `AppearTime`) |
| 3 | `max_task_score > 0` and `Score <= max` | `分数不足（≤N）` where N is `max_task_score` |
| 4 | `only_upgrade_task` and not upgraded | `仅接已升级任务` |
| 5 | `exclude_others_upgrade_task` and `UpgradeUid != 0` and `UpgradeUid != uid` | `他人已升级` |
| 6 | plant-harvest (`3036`) with `ParamID <= 0` or `!flowerCultivated` | `目标花卉未培养` |

Within lead window (preemptive-eligible CD): return `""` so UI does not mark the row un-takeable.

### API

```protobuf
message FmlRaceTask {
  // ... existing fields ...
  // Empty = automation would consider takeable; otherwise primary skip reason.
  string take_skip_reason = 10;
}
```

`fmlRaceProto` (or its caller) must pass `*state.State`, `*pb.UnionRacePolicy` (from account policy; nil-safe defaults), account `uid`, and `now` so each pool task gets `TakeSkipReason` filled.

After `proto-gen`, Web/TS types pick up `takeSkipReason`.

### UI / CLI

**Web (`FmlRaceTaskCard`)**

- If `takeSkipReason` non-empty: muted one-line reason under the score row.
- If empty: no reason line.
- Keep existing `CD ` title prefix, 普通/已升级 badge, and 升级人 when present.

**CLI (`query_race`)**

- Append ` skip=…` (or equivalent) when `take_skip_reason` is set, using the same field.

### Testing

1. Unit tests for `raceTakeSkipReason`: each priority row + within-lead returns `""`.
2. Existing `selectRaceTasks` / shop_union race tests remain green after refactor.
3. Optional smoke: query mapping sets `take_skip_reason` when state/policy warrant it.

## Implementation sketch

1. RED: `raceTakeSkipReason` table tests.
2. GREEN: implement helper; wire `selectRaceTasks` through it.
3. Proto field + `fmlRaceProto` wiring + `make proto-gen`.
4. Web card + CLI display.
5. Verify race tests + targeted helper tests.
