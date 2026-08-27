"use client";

import { useEffect, useState } from "react";
import type { FmlRaceTask, FmlRaceTaken, FmlRaceView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { CollapsibleCard, EmptyState } from "@/features/workspace/shared/workspace-ui";
import { cn } from "@/lib/utils";

export default function FmlRaceMonitorPanel({ race, showTakenTask, showPersonalScoreRank = false }: {
  race?: FmlRaceView;
  showTakenTask: boolean;
  showPersonalScoreRank?: boolean;
}) {
  const tasks = race?.tasks ?? [];
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

  const formatMs = (ms: bigint) => ms === BigInt(0) ? "" : new Date(Number(ms)).toLocaleString("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
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
          {tasks.length > 0 && <Badge variant="outline">{tasks.length} 个可选</Badge>}
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
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span>任务池</span>
                {(race?.tasksSyncedAtMs ?? BigInt(0)) > BigInt(0) && (
                  <span className="text-xs font-normal text-muted-foreground">
                    更新于 {new Date(Number(race!.tasksSyncedAtMs)).toLocaleString("zh-CN", { hour: "2-digit", minute: "2-digit" })} · 每 10 分钟重新获取
                  </span>
                )}
              </div>
              <Badge variant="secondary">{tasks.length} 个</Badge>
            </div>
            {tasks.length === 0 ? (
              <div className="p-3"><EmptyState title="任务池为空" detail="竞赛任务已接完或尚未刷新。" /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {tasks.map((task, index) => <FmlRaceTaskCard key={task.msId} index={index + 1} task={task} />)}
              </div>
            )}
          </section>
        </>
      )}
    </CollapsibleCard>
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

function FmlRaceTaskCard({ index, task }: { index: number; task: FmlRaceTask }) {
  const skipReason = (task.takeSkipReason ?? "").trim();
  const takeable = skipReason === "" || skipReason.startsWith("冷却中");
  const onCooldown = skipReason.startsWith("冷却中") || skipReason.endsWith("后刷新");
  const baseTitle = task.targetLabel ? `${task.taskLabel || `任务 #${task.taskId}`} · ${task.targetLabel}` : task.taskLabel || `任务 #${task.taskId}`;
  return (
    <div className={cn("rounded-md border-2 bg-white/36 px-3 py-2 dark:bg-white/5", takeable ? "border-red-500 bg-red-500/5 dark:bg-red-500/10" : "border-border/55")}>
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 text-sm font-medium"><span className="mr-1.5 tabular-nums text-muted-foreground">{index}.</span>{onCooldown ? `CD ${baseTitle}` : baseTitle}</span>
        <Badge variant={task.isUpgrade ? "secondary" : "outline"}>{task.isUpgrade ? "已升级" : "普通"}</Badge>
      </div>
      <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground"><span>分数 {task.score}</span>{task.upgradeUid > 0 && <span>升级人 #{task.upgradeUid}</span>}</div>
      {skipReason === "" ? <div className="mt-1 text-xs font-medium text-red-600 dark:text-red-400">可接取</div> : skipReason.startsWith("冷却中") ? <div className="mt-1 text-xs font-medium text-red-600 dark:text-red-400">{skipReason}</div> : <div className="mt-1 text-xs text-muted-foreground">不可接取：{skipReason}</div>}
    </div>
  );
}
