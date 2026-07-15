# Guild race CD take-skip copy (可接 vs 刷新)

**Date:** 2026-07-15  
**Status:** approved (approach 1)

## Problem

Far-CD pool rows currently always get:

`冷却中，HH:mm 后可接`

“可接” only means the `AppearTime` gate will clear. Other take filters (score, upgrade, priority, plant-harvest cultivation) are not re-checked for the message, so operators may read it as “I will be able to take this after HH:mm” when automation would still skip the task.

## Goal

1. Keep far-CD as a **time-themed** primary skip reason (do not surface score/cultivate copy instead of refresh time while still on CD).
2. When far CD is the early gate, branch the Chinese copy:
   - After CD, existing take rules would allow take → `冷却中，HH:mm 后可接`
   - After CD, existing take rules would still block → `HH:mm 后刷新`
3. Leave take / preemptive-lead / selection behavior unchanged; this is display-side wording inside `RaceTakeSkipReason` only.

## Non-goals

- Do not predict whether a taken task can be finished (only whether take filters would pass once `AppearTime` is no longer blocking).
- Do not change reason priority for non-CD cases (`已被接取`, score, upgrade, priority, 目标花卉未培养, …).
- Do not add new API fields; still one `take_skip_reason` string.
- Do not change Web prefix `不可接取：`.

## Approach

In `RaceTakeSkipReason`:

1. If `UID != 0` → `已被接取` (unchanged).
2. If far CD (`AppearTime > 0` and `AppearTime > now+lead`):
   - Evaluate the **non-CD** take filters with the same predicates used today (max score, only-upgrade, exclude-others-upgrade, priority ≤ 0, plant-harvest cultivation).
   - If any non-CD filter would skip → return `HH:mm 后刷新` (`HH:mm` from `AppearTime` local clock, same format as today).
   - Else → return `冷却中，HH:mm 后可接`.
3. Continue with the existing non-CD filter returns and empty string for ready / within-lead.

Refactor detail: extract shared non-CD policy/state skip logic so the CD branch and the normal path do not drift.

## Testing

Extend `TestRaceTakeSkipReason` (or adjacent):

1. Far CD + otherwise takeable → `冷却中，HH:mm 后可接`.
2. Far CD + plant not cultivated (3036) → `HH:mm 后刷新` (not 可接, not 目标花卉未培养 as primary while on CD).
3. Far CD + score gate would fail → `HH:mm 后刷新`.
4. Existing non-CD cases unchanged; within-lead still `""`.

## Out of scope later

If operators want the underlying filter named while on CD (e.g. `14:46 后刷新（目标花卉未培养）`), that can be a follow-up; this spec keeps a single short time line.
