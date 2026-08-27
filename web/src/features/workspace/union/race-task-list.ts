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
    const remainSeconds = Math.max(1, Math.ceil((Number(task.appearTimeMs) - nowMs) / 1000));
    if (remainSeconds < 60) return `${remainSeconds} 秒后可抢`;
    const minutes = Math.floor(remainSeconds / 60);
    const seconds = remainSeconds % 60;
    return `${minutes} 分${seconds > 0 ? ` ${seconds} 秒` : ""}后可抢`;
  }
  return reason ? `不可抢：${reason}` : "状态待刷新";
}
