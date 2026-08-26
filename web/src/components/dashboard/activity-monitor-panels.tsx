"use client";

import { useEffect, useState } from "react";
import { CalendarDays, Coins, Flower2, ListChecks, Package, Play, Sparkles, Trophy } from "lucide-react";
import { PlanStatus } from "@/gen/mygardenworld/v1/query_service_pb";
import type { ActivityItem, CyclicNoteMilestone, CyclicNoteTaskSlot, CyclicNoteView, CyclicStoryOrder, CyclicStoryView, DessertCelebrityLikeView, DessertMilestoneView, DessertModeView, DessertRuntimeView, DessertTaskView, DessertView, FmlRaceTask, FmlRaceTaken, FmlRaceView } from "@/gen/mygardenworld/v1/query_service_pb";
import { Badge } from "@/components/ui/badge";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";
import { cyclicNotePhaseLabel, cyclicNotePhaseDetail, cyclicStoryPhaseDetail, dessertPhaseDetail, planStatusLabel, formatCount, truncateMiddle } from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState, OverviewStat } from "@/components/dashboard/monitor-panels";

export function CyclicNoteMonitorPanel({ activity }: { activity?: CyclicNoteView }) {
  const phase = activity?.phase ?? 0;
  if (!activity?.found || (phase !== 1 && phase !== 2 && phase !== 3)) {
    return null;
  }

  const activeTasks = activity.tasks.filter((task) => task.unlocked);
  const readyTasks = activity.valid ? activeTasks.filter((task) => task.status === PlanStatus.READY && !task.received).length : 0;
  const readyMilestones = activity.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;

  return (
    <CollapsibleCard
      title={activity.name || "花笺集芳"}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          <Badge variant="outline">批次 {activity.batchId}</Badge>
          {!activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyTasks + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyTasks + readyMilestones}</Badge>}
        </>
      }
    >
      {!activity.observed ? (
        <EmptyState title="花笺集芳状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
      ) : !activity.valid ? (
        <EmptyState title="花笺集芳配置或状态异常" detail="已阻塞自动化；等待完整模板与时间状态同步后再显示任务详情。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat
              icon={<CalendarDays />}
              label="活动阶段"
              value={cyclicNotePhaseLabel(phase)}
              detail={cyclicNotePhaseDetail(activity)}
            />
            <OverviewStat icon={<Trophy />} label="累计积分" value={formatCount(activity.score)} detail={`完成任务 ${formatCount(activity.finishCount)} 次`} />
            <OverviewStat
              icon={<Flower2 />}
              label="花笺余额"
              value={formatCount(activity.currencyBalance)}
              detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "活动货币未识别"}
            />
            <OverviewStat
              icon={<ListChecks />}
              label="任务槽"
              value={`${activeTasks.length}/${activity.tasks.length}`}
              detail={readyTasks > 0 ? `${readyTasks} 个奖励可领取` : activity.taskListObserved ? "已同步" : "等待进入活动同步"}
            />
          </div>

          {activity.description && <div className="rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>任务详情</span>
              <Badge variant="secondary">{activity.tasks.length} 槽</Badge>
            </div>
            {activity.tasks.length === 0 ? (
              <div className="p-3">
                <EmptyState title={activity.taskListObserved ? "当前没有任务槽" : "任务列表尚未同步"} />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.tasks.map((task) => (
                  <CyclicNoteTaskCard key={`${activity.batchId}:${task.slotId}:${task.taskId}`} task={task} />
                ))}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>积分里程碑</span>
              <Badge variant="secondary">积分 {formatCount(activity.score)}</Badge>
            </div>
            {activity.milestones.length === 0 ? (
              <div className="p-3">
                <EmptyState title="暂无里程碑配置" />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => (
                  <CyclicNoteMilestoneCard key={milestone.index} milestone={milestone} />
                ))}
              </div>
            )}
          </section>

          {activity.items.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {activity.items.map((item) => (
                <ActivityItemChip key={item.itemId} item={item} />
              ))}
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

export function FmlRaceMonitorPanel({
  race,
  showTakenTask,
  showPersonalScoreRank = false,
}: {
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
  const showScoreRank = showPersonalScoreRank;

  const formatMs = (ms: bigint) => {
    if (ms === BigInt(0)) return "";
    return new Date(Number(ms)).toLocaleString("zh-CN", {
      month: "numeric",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  return (
    <CollapsibleCard
      title="公会竞赛"
      contentClassName="space-y-3"
      actions={
        <>
          {!observed ? (
            <Badge variant="outline">等待同步</Badge>
          ) : !batchActive ? (
            <Badge variant="outline">非竞赛期间</Badge>
          ) : (
            <Badge variant="secondary">竞赛进行中</Badge>
          )}
          {taskQuotaObserved && totalTaskNum > 0 && (
            <Badge variant="outline">
              已做 {finishedTaskNum}/{totalTaskNum}
            </Badge>
          )}
          {taskQuotaObserved && totalTaskNum <= 0 && (
            <Badge variant="outline">已做 {finishedTaskNum}</Badge>
          )}
          {showScoreRank && scoreObserved && (
            <Badge variant="outline">得分 {score}</Badge>
          )}
          {showScoreRank && rankObserved && rank > 0 && (
            <Badge variant="outline">第 {rank} 名</Badge>
          )}
          {showTakenTask && taken?.hasTask && <Badge variant="secondary">已接任务</Badge>}
          {tasks.length > 0 && <Badge variant="outline">{tasks.length} 个可选</Badge>}
        </>
      }
    >
      {!observed ? (
        <EmptyState title="竞赛状态尚未同步" detail="连接游戏并进入公会界面后，竞赛任务列表会自动同步。" />
      ) : !batchActive ? (
        <EmptyState
          title="当前不在竞赛批次中"
          detail={
            batchStartMs > BigInt(0) && batchEndMs > BigInt(0)
              ? `竞赛按批次开放，非竞赛期间任务池不可用。当前批次：${formatMs(batchStartMs)} ~ ${formatMs(batchEndMs)}`
              : "竞赛按批次开放，非竞赛期间任务池不可用。"
          }
        />
      ) : (
        <>
          {(showScoreRank || taskQuotaObserved) && (
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-md border border-border/58 bg-white/34 px-3 py-2 text-sm dark:bg-white/5">
              {showScoreRank && (
                <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5">
                  <span className="text-muted-foreground">个人竞赛</span>
                  <span className="font-medium">
                    {scoreObserved || rankObserved ? (
                      <>
                        {scoreObserved ? `得分 ${score}` : "得分 —"}
                        {rankObserved && rank > 0 ? ` · 第 ${rank} 名` : ""}
                      </>
                    ) : (
                      <span className="font-normal text-muted-foreground">得分与排名同步中…</span>
                    )}
                  </span>
                </div>
              )}
              {taskQuotaObserved && (
                <div className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5">
                  <span className="text-muted-foreground">任务次数</span>
                  <span className="font-medium">
                    {totalTaskNum > 0 ? `已做 ${finishedTaskNum} / 总 ${totalTaskNum}` : `已做 ${finishedTaskNum}`}
                  </span>
                </div>
              )}
            </div>
          )}

          {showTakenTask &&
            (taken?.hasTask ? (
              <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
                <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
                  <span>当前已接任务</span>
                </div>
                <div className="p-3">
                  <FmlRaceTakenCard taken={taken} />
                </div>
              </section>
            ) : (
              <div className="rounded-md border border-dashed border-border/58 px-3 py-2 text-sm text-muted-foreground">当前未接取任务</div>
            ))}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span>任务池</span>
                {(race?.tasksSyncedAtMs ?? BigInt(0)) > BigInt(0) && (
                  <span className="text-xs font-normal text-muted-foreground">
                    更新于{" "}
                    {new Date(Number(race!.tasksSyncedAtMs)).toLocaleString("zh-CN", {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}{" "}
                    · 每 10 分钟重新获取
                  </span>
                )}
              </div>
              <Badge variant="secondary">{tasks.length} 个</Badge>
            </div>
            {tasks.length === 0 ? (
              <div className="p-3">
                <EmptyState title="任务池为空" detail="竞赛任务已接完或尚未刷新。" />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {tasks.map((task, index) => (
                  <FmlRaceTaskCard key={task.msId} index={index + 1} task={task} />
                ))}
              </div>
            )}
          </section>
        </>
      )}
    </CollapsibleCard>
  );
}

export function FmlRaceTakenCard({ taken }: { taken: FmlRaceTaken }) {
  const [nowMs, setNowMs] = useState<number | null>(null);

  useEffect(() => {
    const updateNow = () => setNowMs(Date.now());
    updateNow();
    const timer = window.setInterval(updateNow, 1000);
    return () => window.clearInterval(timer);
  }, []);

  const progress = taken.targetCnt > 0 ? Math.min(100, Math.round((taken.finishCnt / taken.targetCnt) * 100)) : 0;
  const title = taken.targetLabel
    ? `${taken.taskLabel || `任务 #${taken.taskId}`} · ${taken.targetLabel}`
    : taken.taskLabel || `任务 #${taken.taskId}`;
  const expireMs = Number(taken.expireTimeMs ?? BigInt(0));
  const remainMs = expireMs > 0 && nowMs !== null ? expireMs - nowMs : 0;
  const expireUrgent = expireMs > 0 && nowMs !== null && remainMs > 0 && remainMs <= 10 * 60 * 1000 && progress < 100;
  const expireLabel =
    expireMs > 0
      ? new Date(expireMs).toLocaleString("zh-CN", {
          month: "numeric",
          day: "numeric",
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
        })
      : "";
  const remainLabel = (() => {
    if (expireMs <= 0 || nowMs === null) return "";
    if (remainMs <= 0) return "已过期";
    const totalSec = Math.floor(remainMs / 1000);
    const h = Math.floor(totalSec / 3600);
    const m = Math.floor((totalSec % 3600) / 60);
    if (h > 0) return `剩余 ${h}小时${m}分`;
    if (m > 0) return `剩余 ${m}分钟`;
    return `剩余 ${totalSec}秒`;
  })();
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{title}</span>
        <Badge variant={progress >= 100 ? "secondary" : "outline"}>{progress}%</Badge>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${progress}%` }} />
      </div>
      <div className="text-xs text-muted-foreground">
        进度 {taken.finishCnt} / {taken.targetCnt} · 分数 {taken.score}
      </div>
      <div className={`text-xs ${expireUrgent ? "font-medium text-amber-700 dark:text-amber-400" : "text-muted-foreground"}`}>
        {expireLabel !== "" ? (
          <>
            {progress >= 100 ? "已完成，待提交" : expireUrgent ? "即将过期" : "过期时间"}：{expireLabel}
            {remainLabel !== "" && progress < 100 ? `（${remainLabel}）` : null}
          </>
        ) : (
          "过期时间：等待同步任务时长"
        )}
      </div>
    </div>
  );
}

export function FmlRaceTaskCard({ index, task }: { index: number; task: FmlRaceTask }) {
	const skipReason = (task.takeSkipReason ?? "").trim();
	// Empty = ready now. "冷却中…后可接" = passes filters, waiting on AppearTime.
	// Both are tasks automation would take; other skip reasons are hard rejects.
	const takeable = skipReason === "" || skipReason.startsWith("冷却中");
	// The server computes CD using the same lead window as task selection. Using
	// that snapshot keeps rendering pure and the label consistent with automation.
	const onCd = skipReason.startsWith("冷却中") || skipReason.endsWith("后刷新");
	const baseTitle = task.targetLabel
		? `${task.taskLabel || `任务 #${task.taskId}`} · ${task.targetLabel}`
		: task.taskLabel || `任务 #${task.taskId}`;
	const title = onCd ? `CD ${baseTitle}` : baseTitle;
  return (
    <div
      className={cn(
        "rounded-md border-2 bg-white/36 px-3 py-2 dark:bg-white/5",
        takeable ? "border-red-500 bg-red-500/5 dark:bg-red-500/10" : "border-border/55",
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="min-w-0 text-sm font-medium">
          <span className="mr-1.5 tabular-nums text-muted-foreground">{index}.</span>
          {title}
        </span>
        <Badge variant={task.isUpgrade ? "secondary" : "outline"}>{task.isUpgrade ? "已升级" : "普通"}</Badge>
      </div>
      <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
        <span>分数 {task.score}</span>
        {task.upgradeUid > 0 && <span>升级人 #{task.upgradeUid}</span>}
      </div>
      {skipReason === "" ? (
        <div className="mt-1 text-xs font-medium text-red-600 dark:text-red-400">可接取</div>
      ) : skipReason.startsWith("冷却中") ? (
        <div className="mt-1 text-xs font-medium text-red-600 dark:text-red-400">{skipReason}</div>
      ) : (
        <div className="mt-1 text-xs text-muted-foreground">不可接取：{skipReason}</div>
      )}
    </div>
  );
}

export function CyclicNoteTaskCard({ task }: { task: CyclicNoteTaskSlot }) {
  if (!task.unlocked) {
    return (
      <div className="rounded-md border border-dashed border-border/70 bg-muted/15 p-3 text-sm text-muted-foreground">
        <div className="flex items-center justify-between gap-2">
          <span className="font-medium">任务槽 {task.slotId}</span>
          <Badge variant="outline">未解锁</Badge>
        </div>
        <div className="mt-3 text-xs">仅监控，不会自动解锁付费槽位。</div>
      </div>
    );
  }
  const progress = Math.max(0, Math.min(task.progress, task.target > 0 ? task.target : task.progress));
  const percent = task.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / task.target) * 100))) : 0;
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">任务槽 {task.slotId} · #{task.taskId}</div>
          <div className="mt-1 line-clamp-2 font-medium">{task.title || `任务 #${task.taskId}`}</div>
        </div>
        <CyclicNoteStatusBadge status={task.status} received={task.received} unknown={!task.catalogKnown} />
      </div>
      {task.target > 0 && (
        <>
          <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>进度</span>
            <span className="tabular-nums">{formatCount(progress)}/{formatCount(task.target)}</span>
          </div>
          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
          </div>
        </>
      )}
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {task.reward.length > 0 ? task.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

export function CyclicNoteMilestoneCard({ milestone }: { milestone: CyclicNoteMilestone }) {
  const progress = Math.max(0, Math.min(milestone.progress, milestone.target > 0 ? milestone.target : milestone.progress));
  const percent = milestone.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / milestone.target) * 100))) : 0;
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="font-medium">积分 {formatCount(milestone.target)}</span>
        <CyclicNoteStatusBadge status={milestone.status} received={milestone.received} />
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>进度</span>
        <span className="tabular-nums">{formatCount(progress)}/{formatCount(milestone.target)}</span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {milestone.reward.length > 0 ? milestone.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

export function CyclicNoteStatusBadge({ status, received, unknown = false }: { status: PlanStatus; received: boolean; unknown?: boolean }) {
  if (unknown) return <Badge variant="destructive">未识别</Badge>;
  if (received) return <Badge variant="outline">已领取</Badge>;
  if (status === PlanStatus.READY) return <Badge variant="secondary">可领取</Badge>;
  if (status === PlanStatus.BLOCKED) return <Badge variant="destructive">阻塞</Badge>;
  if (status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">进行中</Badge>;
  return <Badge variant="outline">{planStatusLabel(status)}</Badge>;
}

export function CyclicStoryMonitorPanel({ activity }: { activity?: CyclicStoryView }) {
  const phase = activity?.phase ?? 0;
  if (!activity?.found || (phase !== 1 && phase !== 2 && phase !== 3)) {
    return null;
  }

  const activeOrders = activity.orders.filter((order) => order.orderId > 0 && !order.onCooldown);
  const readyOrders = activity.valid ? activeOrders.filter((order) => order.status === PlanStatus.READY).length : 0;
  const readyMilestones = activity.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;

  return (
    <CollapsibleCard
      title={activity.name || "莳花纪闻"}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          <Badge variant="outline">批次 {activity.batchId}</Badge>
          {!activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyOrders + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyOrders + readyMilestones}</Badge>}
        </>
      }
    >
      {!activity.observed ? (
        <EmptyState title="莳花纪闻状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
      ) : !activity.valid ? (
        <EmptyState title="莳花纪闻配置或状态异常" detail="已阻塞自动化；等待完整模板与时间状态同步后再显示订单详情。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat
              icon={<CalendarDays />}
              label="活动阶段"
              value={cyclicNotePhaseLabel(phase)}
              detail={cyclicStoryPhaseDetail(activity)}
            />
            <OverviewStat icon={<Trophy />} label="累计积分" value={formatCount(activity.score)} detail={`完成订单 ${formatCount(activity.finishCount)} 次`} />
            <OverviewStat
              icon={<Flower2 />}
              label="花史残页"
              value={formatCount(activity.currencyBalance)}
              detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "活动货币未识别"}
            />
            <OverviewStat
              icon={<ListChecks />}
              label="订单槽"
              value={`${activeOrders.length}/${activity.orders.length}`}
              detail={readyOrders > 0 ? `${readyOrders} 个订单可交` : activity.ordersObserved ? "已同步" : "等待进入活动同步"}
            />
          </div>

          {activity.description && <div className="rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>订单详情</span>
              <Badge variant="secondary">{activity.orders.length} 槽</Badge>
            </div>
            {activity.orders.length === 0 ? (
              <div className="p-3">
                <EmptyState title={activity.ordersObserved ? "当前没有订单槽" : "订单列表尚未同步"} />
              </div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.orders.map((order) => (
                  <CyclicStoryOrderCard key={`${activity.batchId}:${order.orderIdx}:${order.orderId}`} order={order} />
                ))}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>积分里程碑</span>
              <Badge variant="secondary">积分 {formatCount(activity.score)}</Badge>
            </div>
            {activity.milestones.length === 0 ? (
              <div className="p-3"><EmptyState title="当前没有里程碑" /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => (
                  <CyclicNoteMilestoneCard key={milestone.index} milestone={milestone} />
                ))}
              </div>
            )}
          </section>

          {activity.items.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {activity.items.map((item) => (
                <ActivityItemChip key={item.itemId} item={item} />
              ))}
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

export function CyclicStoryOrderCard({ order }: { order: CyclicStoryOrder }) {
  if (order.onCooldown || order.orderId <= 0) {
    return (
      <div className="rounded-md border border-dashed border-border/70 bg-muted/15 p-3 text-sm text-muted-foreground">
        <div className="flex items-center justify-between gap-2">
          <span className="font-medium">订单槽 {order.orderIdx}</span>
          <Badge variant="outline">{order.onCooldown ? "冷却中" : "空闲"}</Badge>
        </div>
        <div className="mt-3 text-xs">仅监控，不会自动付费刷新或清 CD。</div>
      </div>
    );
  }
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">订单槽 {order.orderIdx} · #{order.orderId}</div>
          <div className="mt-1 line-clamp-2 font-medium">
            {order.flowerId > 0 ? `${itemName(order.flowerId)} x${formatCount(order.cost)}` : `订单 #${order.orderId}`}
          </div>
        </div>
        <CyclicNoteStatusBadge status={order.status} received={false} unknown={!order.catalogKnown} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {order.reward.length > 0 ? order.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

export function DessertMonitorPanel({ activity }: { activity?: DessertView }) {
  const phase = activity?.phase ?? 0;
  const readyTasks = activity?.valid ? activity.tasks.filter((task) => task.status === PlanStatus.READY && !task.received).length : 0;
  const celebrityReady = activity?.valid && activity.celebrity?.status === PlanStatus.READY && !activity.celebrity.likedThisBatch;
  const actionable = readyTasks + (celebrityReady ? 1 : 0);

  return (
    <CollapsibleCard
      title={activity?.name || "香卉甜糕"}
      defaultOpen={false}
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          {activity?.found && <Badge variant="outline">批次 {activity.batchId}</Badge>}
          {activity?.found && !activity.valid && <Badge variant="destructive">状态异常</Badge>}
          {actionable > 0 && <Badge variant="secondary">可处理 {actionable}</Badge>}
        </>
      }
    >
      {!activity?.observed ? (
        <>
          <EmptyState title="香卉甜糕状态尚未同步" detail="连接游戏后，会按活动类型和服务端时间自动发现当前批次。" />
          {activity?.runtime && <DessertRuntimePanel runtime={activity.runtime} />}
        </>
      ) : !activity.found ? (
        <>
          <EmptyState title="当前未发现香卉甜糕活动" detail="不会固定使用历史批次，也不会探测已结束活动。" />
          <DessertRuntimePanel runtime={activity.runtime} />
        </>
      ) : !activity.valid ? (
        <>
          <EmptyState title="香卉甜糕配置或状态异常" detail="自动操作已阻塞；请等待活动背包、模板和模式状态完整同步。" />
          <DessertObservationStatus activity={activity} />
          <DessertRuntimePanel runtime={activity.runtime} />
        </>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 lg:grid-cols-3 xl:grid-cols-6">
            <OverviewStat icon={<CalendarDays />} label="活动阶段" value={cyclicNotePhaseLabel(phase)} detail={dessertPhaseDetail(activity)} />
            <OverviewStat
              icon={<Sparkles />}
              label="活动体力"
              value={activity.bagObserved ? formatCount(activity.energyBalance) : "-"}
              detail={activity.energyItemId > 0 ? `${itemName(activity.energyItemId)} #${activity.energyItemId}` : "等待识别"}
            />
            <OverviewStat
              icon={<Play />}
              label="累计投放"
              value={activity.dropCountObserved ? formatCount(activity.dropCount) : "-"}
              detail={activity.dropCountObserved ? "服务端累计次数" : "等待同步"}
            />
            <OverviewStat
              icon={<Trophy />}
              label="累计积分"
              value={activity.totalScoreObserved ? formatCount(activity.totalScore) : "-"}
              detail={activity.totalScoreObserved ? "合成累计积分" : "等待同步"}
            />
            <OverviewStat
              icon={<Coins />}
              label="花糕币"
              value={activity.bagObserved ? formatCount(activity.currencyBalance) : "-"}
              detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "等待识别"}
            />
            <OverviewStat
              icon={<Package />}
              label="未开箱"
              value={activity.bagObserved ? formatCount(activity.rewardBoxBalance) : "-"}
              detail="可在设置中开启单次安全开箱"
            />
          </div>

          {activity.description && (
            <div className="break-words rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>
          )}

          <DessertObservationStatus activity={activity} />
          <DessertRuntimePanel runtime={activity.runtime} />

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>合成模式</span>
              <Badge variant="outline">仅展示棋盘统计</Badge>
            </div>
            {activity.modes.length === 0 ? (
              <div className="p-3"><EmptyState title="模式状态尚未同步" /></div>
            ) : (
              <div className="grid grid-cols-1 gap-2 p-2 min-[480px]:grid-cols-2 xl:grid-cols-5">
                {activity.modes.map((mode) => <DessertModeCard key={mode.mode} mode={mode} />)}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>固定任务</span>
              <Badge variant={readyTasks > 0 ? "secondary" : "outline"}>{readyTasks > 0 ? `可领取 ${readyTasks}` : `${activity.tasks.length} 项`}</Badge>
            </div>
            {activity.tasks.length === 0 ? (
              <div className="p-3"><EmptyState title={activity.taskRecordObserved ? "当前没有固定任务" : "任务记录尚未同步"} /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.tasks.map((task) => <DessertTaskCard key={`${activity.batchId}:${task.taskIndex}:${task.taskId}`} task={task} />)}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>累计进度奖励</span>
              <Badge variant="outline">等待协议确认</Badge>
            </div>
            {activity.milestones.length === 0 ? (
              <div className="p-3"><EmptyState title="暂无累计进度奖励配置" /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => <DessertMilestoneCard key={milestone.index} milestone={milestone} />)}
              </div>
            )}
          </section>

          <DessertCelebrityCard celebrity={activity.celebrity} />

          <div className="grid gap-2 sm:grid-cols-2">
            <div className="rounded-md border border-border/58 bg-muted/20 p-3 text-sm">
              <div className="font-medium">奖励箱</div>
              <div className="mt-1 text-xs text-muted-foreground">当前余额 {formatCount(activity.rewardBoxBalance)}；自动开箱默认关闭，开启后每次只开 1 个。</div>
            </div>
            <div className="rounded-md border border-border/58 bg-muted/20 p-3 text-sm">
              <div className="font-medium">合成游戏</div>
              <div className="mt-1 text-xs text-muted-foreground">可展示影子运行时诊断；连续轨迹门禁未通过，不会发送任何游戏 RPC。</div>
            </div>
          </div>

          {activity.items.length > 0 && (
            <div className="flex min-w-0 flex-wrap gap-1.5">
              {activity.items.map((item) => <ActivityItemChip key={item.itemId} item={item} />)}
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

export function DessertRuntimePanel({ runtime }: { runtime?: DessertRuntimeView }) {
  const observed = runtime?.observed ?? false;
  const shortHash = runtime?.boardHash ? truncateMiddle(runtime.boardHash, 8, 6) : "-";
  const waitingValue = runtime?.waiting
    ? `${formatCount(runtime.waitingRemainingMs > BigInt(0) ? runtime.waitingRemainingMs : BigInt(0))} ms`
    : "未等待";
  const waitingDetail = runtime?.waiting && runtime.frozenWaitingLevel > 0
    ? `冻结等级 ${runtime.frozenWaitingLevel}`
    : "waiting ball 未冻结";

  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex min-h-9 flex-wrap items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
        <span>影子运行时</span>
        <span className="flex min-w-0 flex-wrap justify-end gap-1.5">
          <Badge variant={observed ? "outline" : "destructive"}>{observed ? "已观察" : "未观察"}</Badge>
          <Badge variant={runtime?.policyEnabled ? "secondary" : "outline"}>{runtime?.policyEnabled ? "策略已启用" : "策略未启用"}</Badge>
          <Badge variant="outline">{runtime?.shadowOnly ? "仅影子" : "实跑硬锁"}</Badge>
          <Badge variant={runtime?.liveEvidenceReady ? "outline" : "destructive"}>{runtime?.liveEvidenceReady ? "证据门禁已满足" : "证据门禁未满足"}</Badge>
          <Badge variant={runtime?.liveExecutionAllowed ? "destructive" : "outline"}>{runtime?.liveExecutionAllowed ? "执行门禁状态异常" : "执行硬锁"}</Badge>
          {runtime?.failureLocked && <Badge variant="destructive">失败锁定</Badge>}
        </span>
      </div>
      <div className="space-y-2 p-2">
        <div className="break-words rounded-md border border-amber-500/30 bg-amber-500/8 px-3 py-2 text-xs leading-5 text-muted-foreground dark:bg-amber-400/8">
          当前只展示影子诊断。证据门禁与执行门禁分别显示，gameStart / gameSync / gameOver 不会注册或发送。
        </div>
        {!runtime?.observed ? (
          <div className="space-y-1 rounded-md border border-border/55 bg-background/72 px-3 py-2 text-xs text-muted-foreground">
            <div>登录并同步活动后显示会话、权威棋盘版本和建议；未观察期间保持游戏 RPC 硬锁。</div>
            {runtime && <div>会话体力上限 {formatCount(runtime.maxSessionEnergy)}；最低保留 {formatCount(runtime.minEnergyReserve)}。</div>}
            {runtime?.blockedReason && <div className="break-words text-foreground">阻塞原因：{runtime.blockedReason}</div>}
          </div>
        ) : (
          <div className="grid min-w-0 grid-cols-1 gap-2 min-[420px]:grid-cols-2 lg:grid-cols-4">
            <DessertRuntimeMetric
              label="策略 / 模式"
              value={`${runtime.policyEnabled ? "已启用" : "未启用"} · 模式 ${runtime.mode || "-"}`}
              detail={runtime.shadowOnly ? "只计算建议，不执行" : "实跑仍被硬锁"}
            />
            <DessertRuntimeMetric
              label="会话 / 权威版本"
              value={`#${runtime.sessionEpoch.toString()} · r${runtime.authorityRevision.toString()}`}
              detail={runtime.batchId > 0 ? `批次 ${runtime.batchId}` : "批次未识别"}
            />
            <DessertRuntimeMetric
              label="棋盘"
              value={runtime.boardOwned ? "本会话拥有" : "未拥有"}
              detail={runtime.takeoverRequested ? "已请求评估接管" : "未请求接管"}
            />
            <DessertRuntimeMetric label="棋盘摘要" value={shortHash} detail="仅展示截断哈希" title={runtime.boardHash} mono />
            <DessertRuntimeMetric label="waiting ball" value={waitingValue} detail={waitingDetail} />
            <DessertRuntimeMetric
              label="会话体力预算"
              value={`已用 ${formatCount(runtime.sessionEnergyUsed)} / 上限 ${formatCount(runtime.maxSessionEnergy)}`}
              detail={`最低保留 ${formatCount(runtime.minEnergyReserve)}；当前构建不会实际扣除`}
            />
            <DessertRuntimeMetric
              label="证据 / 执行门禁"
              value={`${runtime.liveEvidenceReady ? "证据已满足" : "证据未满足"} · ${runtime.liveExecutionAllowed ? "状态异常" : "执行硬锁"}`}
              detail="执行硬锁独立于策略开关与证据状态"
            />
            <DessertRuntimeMetric
              label="影子建议"
              value={runtime.suggestion || "暂无建议"}
              detail="仅供诊断，不会转为 RPC"
              className="min-[420px]:col-span-2"
            />
            <DessertRuntimeMetric
              label="阻塞 / 锁定"
              value={runtime.blockedReason || (runtime.failureLocked ? "本会话已锁定" : "无额外原因")}
              detail={runtime.failureLocked ? "需重新登录或关闭后重新开启策略" : "连续轨迹门禁仍保持硬锁"}
              className="min-[420px]:col-span-2 lg:col-span-4"
            />
          </div>
        )}
      </div>
    </section>
  );
}

export function DessertRuntimeMetric({
  label,
  value,
  detail,
  title,
  mono = false,
  className,
}: {
  label: string;
  value: string;
  detail: string;
  title?: string;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={cn("min-w-0 rounded-md border border-border/55 bg-background/72 px-3 py-2 text-xs", className)}>
      <div className="text-muted-foreground">{label}</div>
      <div className={cn("mt-1 min-w-0 break-words font-medium text-foreground", mono && "font-mono")} title={title}>{value}</div>
      <div className="mt-1 break-words text-muted-foreground">{detail}</div>
    </div>
  );
}

export function DessertObservationStatus({ activity }: { activity: DessertView }) {
  const observations = [
    { label: "活动背包", ok: activity.bagObserved },
    { label: "扩展状态", ok: activity.extensionObserved && activity.extensionValid },
    { label: "模式地图", ok: activity.modeMapObserved && activity.modeMapValid },
    { label: "任务模板", ok: activity.taskGroupsObserved && activity.taskGroupsValid },
    { label: "任务记录", ok: activity.taskRecordObserved },
    { label: "进度回执", ok: activity.milestoneReceiptsObserved },
  ];
  return (
    <div className="flex min-w-0 flex-wrap gap-1.5">
      {observations.map((item) => (
        <Badge key={item.label} variant={item.ok ? "outline" : "destructive"}>{item.label} {item.ok ? "已同步" : "缺失"}</Badge>
      ))}
    </div>
  );
}

export function DessertModeCard({ mode }: { mode: DessertModeView }) {
  const modeLabel = mode.mode === 1 ? "普通模式" : `${formatCount(mode.multiplier)} 倍模式`;
  const levelSummary = mode.levelCounts
    .filter((level) => level.count > 0)
    .map((level) => `${level.level}级×${formatCount(level.count)}`);
  const status = !mode.unlocked ? "未解锁" : mode.isRunning ? "进行中" : mode.observed ? "待开始" : "待同步";
  return (
    <div className="min-w-0 rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate font-medium">{modeLabel}</div>
          <div className="mt-1 text-xs text-muted-foreground">倍率 ×{formatCount(mode.multiplier)}</div>
        </div>
        <Badge variant={mode.isRunning ? "secondary" : "outline"}>{status}</Badge>
      </div>
      {!mode.unlocked && <div className="mt-3 text-xs text-muted-foreground">解锁积分 {formatCount(mode.unlockScore)}</div>}
      {mode.observed && (
        <div className="mt-3 grid grid-cols-2 gap-1.5 text-xs text-muted-foreground">
          <span>投放 {formatCount(mode.step)}</span>
          <span>得分 {formatCount(mode.score)}</span>
          <span>当前 {mode.currentId > 0 ? `${mode.currentId}级` : "-"}</span>
          <span>对象 {formatCount(mode.objectCount)}</span>
        </div>
      )}
      <div className="mt-2 break-words text-xs text-muted-foreground">
        {levelSummary.length > 0 ? levelSummary.join("、") : mode.observed ? "棋盘暂无对象" : "等待模式状态"}
      </div>
      {mode.rawGameStatus !== mode.effectiveGameStatus && (
        <div className="mt-2 text-xs text-muted-foreground">状态恢复 {mode.rawGameStatus} → {mode.effectiveGameStatus}</div>
      )}
    </div>
  );
}

export function DessertTaskCard({ task }: { task: DessertTaskView }) {
  const progress = Math.max(0, task.progress);
  const percent = task.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / task.target) * 100))) : 0;
  return (
    <div className="min-w-0 rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">任务 {task.taskIndex}:{task.taskId}</div>
          <div className="mt-1 line-clamp-2 break-words font-medium">{task.title || `任务 #${task.taskId}`}</div>
        </div>
        <DessertTaskStatusBadge task={task} />
      </div>
      {task.target > 0 && (
        <>
          <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
            <span>进度</span>
            <span className="tabular-nums">{formatCount(progress)}/{formatCount(task.target)}</span>
          </div>
          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
          </div>
        </>
      )}
      <div className="mt-3 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {task.reward.length > 0 ? task.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

export function DessertTaskStatusBadge({ task }: { task: DessertTaskView }) {
  if (!task.catalogKnown) return <Badge variant="destructive">未识别</Badge>;
  if (task.received) return <Badge variant="outline">已领取</Badge>;
  if (task.status === PlanStatus.READY) return <Badge variant="secondary">可领取</Badge>;
  if (task.status === PlanStatus.BLOCKED) return <Badge variant="destructive">阻塞</Badge>;
  if (task.status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">进行中</Badge>;
  return <Badge variant="outline">{planStatusLabel(task.status)}</Badge>;
}

export function DessertMilestoneCard({ milestone }: { milestone: DessertMilestoneView }) {
  const progress = Math.max(0, milestone.progress);
  const percent = milestone.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / milestone.target) * 100))) : 0;
  return (
    <div className="min-w-0 rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="font-medium">积分 {formatCount(milestone.target)}</span>
        <Badge variant="outline">{milestone.received ? "已领取" : "等待协议确认"}</Badge>
      </div>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>进度</span>
        <span className="tabular-nums">{formatCount(progress)}/{formatCount(milestone.target)}</span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
      <div className="mt-3 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {milestone.reward.length > 0 ? milestone.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

export function DessertCelebrityCard({ celebrity }: { celebrity?: DessertCelebrityLikeView }) {
  const label = !celebrity?.observed
    ? "待同步"
    : celebrity.likedThisBatch
      ? "本期已点赞"
      : celebrity.status === PlanStatus.READY
        ? "可免费点赞"
        : celebrity.status === PlanStatus.BLOCKED
          ? "已阻塞"
          : "待同步";
  const variant = celebrity?.status === PlanStatus.BLOCKED ? "destructive" : celebrity?.status === PlanStatus.READY && !celebrity.likedThisBatch ? "secondary" : "outline";
  return (
    <section className="min-w-0 rounded-md border border-border/58 bg-white/34 p-3 dark:bg-white/5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="font-medium">名人榜免费点赞</div>
          <div className="mt-1 text-xs text-muted-foreground">
            {celebrity?.rankingObserved ? `榜单共 ${formatCount(celebrity.rankingCount)} 条，仅展示数量` : "榜单尚未完成受控同步"}
          </div>
        </div>
        <Badge variant={variant}>{label}</Badge>
      </div>
      <div className="mt-3 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {celebrity?.reward?.length ? celebrity.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>等待配置确认</span>}
      </div>
    </section>
  );
}

export function ActivityItemChip({ item, compact = false }: { item: ActivityItem; compact?: boolean }) {
  const label = item.itemName || itemName(item.itemId);
  return (
    <span className={cn("inline-flex max-w-full items-center gap-1 rounded border border-border/58 bg-white/52 dark:bg-white/5", compact ? "px-1.5 py-0.5" : "px-2 py-1 text-xs")}>
      <span className="min-w-0 truncate" title={item.itemId > 0 ? `${label} #${item.itemId}` : label}>{label}</span>
      <span className="shrink-0 font-semibold tabular-nums">×{formatCount(item.count)}</span>
    </span>
  );
}
