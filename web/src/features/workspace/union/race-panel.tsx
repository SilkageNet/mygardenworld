"use client";

import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { Hand, Loader2, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AutomationService, DeleteUnionRaceTaskRequestSchema, TakeUnionRaceTaskRequestSchema } from "@/gen/mygardenworld/v1/automation_service_pb";
import type { FmlRaceTask, FmlRaceTaken, FmlRaceView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { CollapsibleCard, EmptyState } from "@/features/workspace/shared/workspace-ui";
import { formatAPIError, transport } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import {
  raceTaskAvailability,
  raceTaskProgressLabel,
  raceTaskReady,
  selectRaceTaskList,
  type RaceTaskFilter,
  type RaceTaskSort,
} from "./race-task-list";

const automationClient = createClient(AutomationService, transport);
const EMPTY_RACE_TASKS: FmlRaceTask[] = [];

export default function FmlRaceMonitorPanel({ accountId, race, showTakenTask, showPersonalScoreRank = false, canDeleteTasks = false }: {
  accountId: bigint;
  race?: FmlRaceView;
  showTakenTask: boolean;
  showPersonalScoreRank?: boolean;
  canDeleteTasks?: boolean;
}) {
  const tasks = race?.tasks ?? EMPTY_RACE_TASKS;
  const taken = race?.taken;
  const observed = race?.observed ?? false;
  const batchActive = race?.batchActive ?? false;
  const batchStartMs = race?.batchStartMs ?? BigInt(0);
  const batchEndMs = race?.batchEndMs ?? BigInt(0);
  const taskQuotaObserved = race?.taskQuotaObserved ?? false;
  const finishedTaskNum = race?.finishedTaskNum ?? 0;
  const totalTaskNum = race?.totalTaskNum ?? 0;
  const scoreObserved = race?.scoreObserved ?? false;
  const score = race?.score ?? 0;
  const rankObserved = race?.rankObserved ?? false;
  const rank = race?.rank ?? 0;
  const [nowMs, setNowMs] = useState(() => Date.now());
  const [taskFilter, setTaskFilter] = useState<RaceTaskFilter>("all");
  const [taskSort, setTaskSort] = useState<RaceTaskSort>("score");
  const [busyAction, setBusyAction] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<FmlRaceTask>();
  const [deleteError, setDeleteError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const accountCanTake = !taken?.hasTask && accountId > BigInt(0);

  useEffect(() => {
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const readyCount = useMemo(
    () => accountCanTake ? tasks.filter((task) => raceTaskReady(task, nowMs)).length : 0,
    [accountCanTake, tasks, nowMs],
  );
  const visibleTasks = useMemo(
    () => taskFilter === "ready" && !accountCanTake ? [] : selectRaceTaskList(tasks, taskFilter, taskSort, nowMs),
    [accountCanTake, tasks, taskFilter, taskSort, nowMs],
  );

  const takeTask = async (task: FmlRaceTask) => {
    const taskKey = task.msId.toString();
    setBusyAction(`take:${taskKey}`);
    setActionMessage("");
    try {
      await automationClient.takeUnionRaceTask(create(TakeUnionRaceTaskRequestSchema, {
        accountId,
        taskMsId: task.msId,
      }));
      setActionMessage("接取请求已成功，正在等待任务状态同步。");
    } catch (err) {
      setActionMessage(formatAPIError(err, "接取竞赛任务失败"));
    } finally {
      setBusyAction("");
    }
  };

  const deleteTask = async () => {
    if (!deleteTarget) return;
    const taskKey = deleteTarget.msId.toString();
    setBusyAction(`delete:${taskKey}`);
    setActionMessage("");
    setDeleteError("");
    try {
      await automationClient.deleteUnionRaceTask(create(DeleteUnionRaceTaskRequestSchema, {
        accountId,
        taskMsId: deleteTarget.msId,
      }));
      setActionMessage("删除请求已成功，正在等待任务池刷新。");
      setDeleteTarget(undefined);
    } catch (err) {
      const message = formatAPIError(err, "删除竞赛任务失败");
      setActionMessage(message);
      setDeleteError(message);
    } finally {
      setBusyAction("");
    }
  };

  const formatMs = (ms: bigint) => ms === BigInt(0) ? "" : new Date(Number(ms)).toLocaleString("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <>
      <CollapsibleCard
      title="公会竞赛"
      contentClassName="space-y-3"
      actions={(
        <>
          {!observed ? <Badge variant="outline">等待同步</Badge> : !batchActive ? <Badge variant="outline">非竞赛期间</Badge> : <Badge variant="secondary">竞赛进行中</Badge>}
          {taskQuotaObserved && <Badge variant="outline">已做 {finishedTaskNum}{totalTaskNum > 0 ? `/${totalTaskNum}` : ""}</Badge>}
          {showPersonalScoreRank && scoreObserved && <Badge variant="outline">得分 {score}</Badge>}
          {showPersonalScoreRank && rankObserved && rank > 0 && <Badge variant="outline">第 {rank} 名</Badge>}
          {showTakenTask && taken?.hasTask && <Badge variant="secondary">已接任务</Badge>}
          {tasks.length > 0 && <Badge variant="outline">{tasks.length} 个任务</Badge>}
        </>
      )}
    >
      {!observed ? (
        <EmptyState title="竞赛状态尚未同步" detail="连接游戏并进入公会界面后，竞赛任务列表会自动同步。" />
      ) : !batchActive ? (
        <EmptyState
          title="当前不在竞赛批次中"
          detail={batchStartMs > BigInt(0) && batchEndMs > BigInt(0)
            ? `竞赛按批次开放，非竞赛期间任务池不可用。当前批次：${formatMs(batchStartMs)} ~ ${formatMs(batchEndMs)}`
            : "竞赛按批次开放，非竞赛期间任务池不可用。"}
        />
      ) : (
        <>
          {(showPersonalScoreRank || taskQuotaObserved) && (
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-md border border-border/58 bg-white/34 px-3 py-2 text-sm dark:bg-white/5">
              {showPersonalScoreRank && (
                <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5">
                  <span className="text-muted-foreground">个人竞赛</span>
                  <span className="font-medium">
                    {scoreObserved || rankObserved ? <>{scoreObserved ? `得分 ${score}` : "得分 —"}{rankObserved && rank > 0 ? ` · 第 ${rank} 名` : ""}</> : <span className="font-normal text-muted-foreground">得分与排名同步中…</span>}
                  </span>
                </div>
              )}
              {taskQuotaObserved && (
                <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5">
                  <span className="text-muted-foreground">任务次数</span>
                  <span className="font-medium">{totalTaskNum > 0 ? `已做 ${finishedTaskNum} / 总 ${totalTaskNum}` : `已做 ${finishedTaskNum}`}</span>
                </div>
              )}
            </div>
          )}

          {showTakenTask && (taken?.hasTask ? (
            <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
              <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45"><span>当前已接任务</span></div>
              <div className="p-3"><FmlRaceTakenCard taken={taken} /></div>
            </section>
          ) : <div className="rounded-md border border-dashed border-border/58 px-3 py-2 text-sm text-muted-foreground">当前未接取任务</div>)}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            {race?.autoDeleteStatus && <p className="border-b border-border/50 px-3 py-2 text-xs text-muted-foreground">自动删除：{race.autoDeleteStatus}</p>}
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-2 text-sm font-semibold dark:bg-muted/45">
              <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span>任务池</span>
                {(race?.tasksSyncedAtMs ?? BigInt(0)) > BigInt(0) && (
                  <span className="text-xs font-normal text-muted-foreground">
                    更新于 {new Date(Number(race!.tasksSyncedAtMs)).toLocaleString("zh-CN", { hour: "2-digit", minute: "2-digit" })} · 约每 30 秒校准
                  </span>
                )}
              </div>
              <div className="flex items-center gap-1.5">
                <Badge variant={readyCount > 0 ? "secondary" : "outline"}>可抢 {readyCount}</Badge>
                <Badge variant="outline">共 {tasks.length}</Badge>
              </div>
            </div>
            {tasks.length === 0 ? (
              <div className="p-3"><EmptyState title="任务池为空" detail="竞赛任务已接完或尚未刷新。" /></div>
            ) : (
              <div className="space-y-2 p-2">
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border/48 bg-background/42 p-1.5">
                  <div className="flex items-center gap-1" aria-label="任务过滤">
                    <Button type="button" size="sm" variant={taskFilter === "all" ? "secondary" : "ghost"} onClick={() => setTaskFilter("all")}>全部 {tasks.length}</Button>
                    <Button type="button" size="sm" variant={taskFilter === "ready" ? "secondary" : "ghost"} onClick={() => setTaskFilter("ready")}>可抢 {readyCount}</Button>
                  </div>
                  <div className="flex items-center gap-1" aria-label="任务排序">
                    <Button type="button" size="sm" variant={taskSort === "score" ? "secondary" : "ghost"} onClick={() => setTaskSort("score")}>分数 ↓</Button>
                    <Button type="button" size="sm" variant={taskSort === "pool" ? "secondary" : "ghost"} onClick={() => setTaskSort("pool")}>池顺序</Button>
                  </div>
                </div>
                {actionMessage && (
                  <div role="status" className="rounded-md border border-border/58 bg-background/54 px-3 py-2 text-xs text-muted-foreground">
                    {actionMessage}
                  </div>
                )}
                {visibleTasks.length === 0 ? (
                  <EmptyState title="当前没有可抢任务" detail="冷却结束或任务池刷新后，这里会自动出现可手动接取的任务。" />
                ) : (
                  <div className="dark-scrollbar max-h-[520px] overflow-y-auto pr-0.5 sm:max-h-[620px] sm:pr-1">
                    <div className="grid gap-2 lg:grid-cols-2 xl:grid-cols-3">
                      {visibleTasks.map(({ task, poolIndex }) => (
                        <FmlRaceTaskCard
                          key={task.msId}
                          index={poolIndex + 1}
                          task={task}
                          nowMs={nowMs}
                          canTake={accountCanTake}
                          canDelete={canDeleteTasks && task.deleteAllowed}
                          showDelete={canDeleteTasks}
                          deleteBlockedReason={task.deleteBlockedReason}
                          takeBusy={busyAction === `take:${task.msId.toString()}`}
                          deleteBusy={busyAction === `delete:${task.msId.toString()}`}
                          onTake={() => void takeTask(task)}
                          onDelete={() => { setDeleteError(""); setDeleteTarget(task); }}
                        />
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}
          </section>
        </>
      )}
      </CollapsibleCard>
      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => { if (!open && !busyAction.startsWith("delete:")) setDeleteTarget(undefined); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>删除竞赛任务</DialogTitle>
            <DialogDescription>该操作会立即从任务池移除这个任务，并让对应槽位进入刷新冷却。</DialogDescription>
          </DialogHeader>
          {deleteTarget && (
            <div className="rounded-md border border-border/60 bg-secondary/35 px-3 py-3 text-sm">
              <div className="font-medium">{raceTaskTitle(deleteTarget)}</div>
              <div className="mt-1 text-xs text-muted-foreground">{deleteTarget.score} 分 · 任务 #{deleteTarget.msId.toString()}</div>
            </div>
          )}
          {deleteError && <div className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{deleteError}</div>}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDeleteTarget(undefined)} disabled={busyAction.startsWith("delete:")}>取消</Button>
            <Button type="button" variant="destructive" onClick={() => void deleteTask()} disabled={!deleteTarget || busyAction.startsWith("delete:")}>
              {busyAction.startsWith("delete:") ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              {busyAction.startsWith("delete:") ? "删除中" : "确认删除"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

function FmlRaceTakenCard({ taken }: { taken: FmlRaceTaken }) {
  const [nowMs, setNowMs] = useState<number | null>(null);
  useEffect(() => {
    const updateNow = () => setNowMs(Date.now());
    updateNow();
    const timer = window.setInterval(updateNow, 1000);
    return () => window.clearInterval(timer);
  }, []);

  const progress = taken.targetCnt > 0 ? Math.min(100, Math.round((taken.finishCnt / taken.targetCnt) * 100)) : 0;
  const title = taken.targetLabel ? `${taken.taskLabel || `任务 #${taken.taskId}`} · ${taken.targetLabel}` : taken.taskLabel || `任务 #${taken.taskId}`;
  const expireMs = Number(taken.expireTimeMs ?? BigInt(0));
  const remainMs = expireMs > 0 && nowMs !== null ? expireMs - nowMs : 0;
  const expireUrgent = expireMs > 0 && nowMs !== null && remainMs > 0 && remainMs <= 10 * 60 * 1000 && progress < 100;
  const expireLabel = expireMs > 0 ? new Date(expireMs).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "";
  const remainLabel = (() => {
    if (expireMs <= 0 || nowMs === null) return "";
    if (remainMs <= 0) return "已过期";
    const totalSeconds = Math.floor(remainMs / 1000);
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    if (hours > 0) return `剩余 ${hours}小时${minutes}分`;
    if (minutes > 0) return `剩余 ${minutes}分钟`;
    return `剩余 ${totalSeconds}秒`;
  })();

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between"><span className="text-sm font-medium">{title}</span><Badge variant={progress >= 100 ? "secondary" : "outline"}>{progress}%</Badge></div>
      <div className="h-2 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-primary transition-all" style={{ width: `${progress}%` }} /></div>
      <div className="text-xs text-muted-foreground">进度 {taken.finishCnt} / {taken.targetCnt} · 分数 {taken.score}</div>
      <div className={`text-xs ${expireUrgent ? "font-medium text-amber-700 dark:text-amber-400" : "text-muted-foreground"}`}>
        {expireLabel ? <>{progress >= 100 ? "已完成，待提交" : expireUrgent ? "即将过期" : "过期时间"}：{expireLabel}{remainLabel && progress < 100 ? `（${remainLabel}）` : null}</> : "过期时间：等待同步任务时长"}
      </div>
    </div>
  );
}

function FmlRaceTaskCard({ index, task, nowMs, canTake, canDelete, showDelete, deleteBlockedReason, takeBusy, deleteBusy, onTake, onDelete }: {
  index: number;
  task: FmlRaceTask;
  nowMs: number;
  canTake: boolean;
  canDelete: boolean;
  showDelete: boolean;
  deleteBlockedReason: string;
  takeBusy: boolean;
  deleteBusy: boolean;
  onTake: () => void;
  onDelete: () => void;
}) {
  const skipReason = (task.takeSkipReason ?? "").trim();
  const takeable = raceTaskReady(task, nowMs);
  const onCooldown = !takeable && skipReason.startsWith("冷却中");
  const availability = takeable && !canTake ? "需先完成当前任务" : raceTaskAvailability(task, nowMs);
  const progressLabel = raceTaskProgressLabel(task);
  const baseTitle = raceTaskTitle(task);
  return (
    <div className={cn(
      "rounded-md border bg-white/36 px-3 py-2.5 dark:bg-white/5",
      takeable && canTake ? "border-primary/55 bg-primary/7 shadow-sm" : onCooldown ? "border-amber-300/65 bg-amber-50/48 dark:bg-amber-400/8" : "border-border/55",
    )}>
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 text-sm font-medium"><span className="mr-1.5 tabular-nums text-muted-foreground">{index}.</span>{baseTitle}</span>
        <Badge variant={task.isUpgrade ? "secondary" : "outline"}>{task.isUpgrade ? "已升级" : "普通"}</Badge>
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
        <span className="font-medium text-foreground">{task.score} 分</span>
        {progressLabel && <span className={cn("rounded-full px-1.5 py-0.5", task.finishCnt > 0 ? "bg-amber-100 text-amber-800 dark:bg-amber-400/12 dark:text-amber-300" : "bg-muted/70")}>{progressLabel}</span>}
        {task.upgradeUid > 0 && <span className="ml-auto">升级人 #{task.upgradeUid}</span>}
      </div>
      <div className="mt-2 flex min-h-7 items-center justify-between gap-2">
        <span className={cn("text-xs", takeable && canTake ? "font-medium text-primary" : onCooldown ? "font-medium text-amber-700 dark:text-amber-400" : "text-muted-foreground")}>{availability}</span>
        <span className="flex items-center gap-1.5">
          {showDelete && (
            <Button type="button" size="sm" variant="destructive" onClick={onDelete} disabled={!canDelete || deleteBusy || takeBusy} title={canDelete ? "删除此任务" : deleteBlockedReason || "当前不可删除"}>
              {deleteBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Trash2 className="size-3.5" />}
              {deleteBusy ? "删除中" : "删除"}
            </Button>
          )}
          {takeable && canTake && (
            <Button type="button" size="sm" onClick={onTake} disabled={takeBusy || deleteBusy} title="立即接取此任务">
              {takeBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Hand className="size-3.5" />}
              {takeBusy ? "接取中" : "手动抢"}
            </Button>
          )}
        </span>
      </div>
    </div>
  );
}

function raceTaskTitle(task: FmlRaceTask): string {
  return task.targetLabel ? `${task.taskLabel || `任务 #${task.taskId}`} · ${task.targetLabel}` : task.taskLabel || `任务 #${task.taskId}`;
}
