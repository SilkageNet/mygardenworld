import type { FmlRaceTask } from "@/lib/api/workspace-models";

export type RaceTaskFilter = "all" | "ready";
export type RaceTaskSort = "pool" | "score";
export type RaceTaskTone = "ready" | "cooldown" | "claimed" | "blocked";

export type RaceTaskListItem = {
  task: FmlRaceTask;
  poolIndex: number;
};

export function raceTaskReady(task: FmlRaceTask, nowMs: number): boolean {
  const reason = (task.takeSkipReason ?? "").trim();
  if (reason === "") return true;
  return reason.startsWith("冷却中") && Number(task.appearTimeMs) <= nowMs;
}

// Presentation only: keep the existing takeability and server-side guards.
// Occupancy wins over cooldown; holding another task must not look actionable.
export function raceTaskTone(task: FmlRaceTask, nowMs: number, canTake: boolean): RaceTaskTone {
  const reason = (task.takeSkipReason ?? "").trim();
  if (reason === "已被接取") return "claimed";
  if (canTake && reason.startsWith("冷却中") && Number(task.appearTimeMs) > nowMs) return "cooldown";
  return raceTaskReady(task, nowMs) && canTake ? "ready" : "blocked";
}

export function selectRaceTaskList(
  tasks: FmlRaceTask[],
  filter: RaceTaskFilter,
  sort: RaceTaskSort,
  nowMs: number,
): RaceTaskListItem[] {
  const selected = tasks
    .map((task, poolIndex) => ({ task, poolIndex }))
    .filter(({ task }) => filter === "all" || raceTaskReady(task, nowMs));
  if (sort === "score") {
    selected.sort((a, b) => b.task.score - a.task.score || a.poolIndex - b.poolIndex);
  }
  return selected;
}

export function raceTaskAvailability(task: FmlRaceTask, nowMs: number, canTake = true): string {
  const reason = (task.takeSkipReason ?? "").trim();
  if (!canTake && (raceTaskReady(task, nowMs) || reason.startsWith("冷却中"))) return "需先完成当前任务";
  if (raceTaskReady(task, nowMs)) return "现在可抢";
  if (reason.startsWith("冷却中")) {
    return `${formatRaceTaskTime(task.appearTimeMs)} 后可抢`;
  }
  return reason ? `不可抢：${reason}` : "状态待刷新";
}

// A refresh deadline is not a promise that policy/state restrictions will clear.
export function raceTaskRefreshLabel(task: FmlRaceTask, nowMs: number, canTake: boolean): string | null {
  return raceTaskTone(task, nowMs, canTake) === "blocked" && Number(task.appearTimeMs) > nowMs
    ? `${formatRaceTaskTime(task.appearTimeMs)} 后刷新`
    : null;
}

export function formatRaceTaskTime(ms: bigint): string {
  if (ms <= BigInt(0)) return "";
  return new Date(Number(ms)).toLocaleString("zh-CN", {
    month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
}

// Wake once at a deadline to update availability or remove auxiliary refresh text.
// A blocked task stays blocked; no per-second countdown is needed.
export function nextRaceTaskReadyAt(tasks: FmlRaceTask[], nowMs: number): number | null {
  let next: number | null = null;
  for (const task of tasks) {
    const at = Number(task.appearTimeMs);
    if ((task.takeSkipReason ?? "").trim() !== "已被接取" && at > nowMs && (next === null || at < next)) next = at;
  }
  return next;
}

export function raceTaskProgressLabel(task: FmlRaceTask): string | null {
  if (task.targetCnt > 0) return `进度 ${task.finishCnt}/${task.targetCnt}`;
  if (task.finishCnt > 0) return `已有进度 ${task.finishCnt}`;
  return null;
}
