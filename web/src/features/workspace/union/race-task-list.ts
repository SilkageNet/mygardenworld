import type { FmlRaceTask } from "@/lib/api/workspace-models";

export type RaceTaskFilter = "all" | "ready";
export type RaceTaskSort = "pool" | "score";

export type RaceTaskListItem = {
  task: FmlRaceTask;
  poolIndex: number;
};

export function raceTaskReady(task: FmlRaceTask, nowMs: number): boolean {
  const reason = (task.takeSkipReason ?? "").trim();
  if (reason === "") return true;
  return reason.startsWith("冷却中") && Number(task.appearTimeMs) <= nowMs;
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

export function raceTaskAvailability(task: FmlRaceTask, nowMs: number): string {
  const reason = (task.takeSkipReason ?? "").trim();
  if (raceTaskReady(task, nowMs)) return "现在可抢";
  if (reason.startsWith("冷却中")) {
    return `${formatRaceTaskTime(task.appearTimeMs)} 后可抢`;
  }
  return reason ? `不可抢：${reason}` : "状态待刷新";
}

export function formatRaceTaskTime(ms: bigint): string {
  if (ms <= BigInt(0)) return "";
  return new Date(Number(ms)).toLocaleString("zh-CN", {
    month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
}

// Only wake the task list when availability changes, not on every elapsed second.
export function nextRaceTaskReadyAt(tasks: FmlRaceTask[], nowMs: number): number | null {
  let next: number | null = null;
  for (const task of tasks) {
    const at = Number(task.appearTimeMs);
    if ((task.takeSkipReason ?? "").trim().startsWith("冷却中") && at > nowMs && (next === null || at < next)) next = at;
  }
  return next;
}

export function raceTaskProgressLabel(task: FmlRaceTask): string | null {
  if (task.targetCnt > 0) return `进度 ${task.finishCnt}/${task.targetCnt}`;
  if (task.finishCnt > 0) return `已有进度 ${task.finishCnt}`;
  return null;
}
