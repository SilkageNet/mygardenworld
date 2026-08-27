"use client";

import { CalendarDays, Flower2, ListChecks, Trophy } from "lucide-react";
import { PlanStatus } from "@/lib/api/workspace-models";
import type { CyclicNoteMilestone, CyclicNoteTaskSlot, CyclicNoteView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { itemName } from "@/lib/game/catalog";
import { cyclicNotePhaseLabel, cyclicNotePhaseDetail, planStatusLabel, formatCount } from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState, OverviewStat } from "@/features/workspace/shared/workspace-ui";
import ActivityItemChip from "./activity-item-chip";

export function CyclicNoteMonitorPanel({ activity }: { activity?: CyclicNoteView; }) {
  const phase = activity?.phase ?? 0;
  if (!activity?.found || (phase !== 1 && phase !== 2 && phase !== 3)) return null;

  const activeTasks = activity.tasks.filter((task) => task.unlocked);
  const readyTasks = activity.valid ? activeTasks.filter((task) => task.status === PlanStatus.READY && !task.received).length : 0;
  const readyMilestones = activity.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;

  return (
    <CollapsibleCard
      title={activity.name || "花笺集芳"}
      contentClassName="space-y-3"
      actions={(
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          <Badge variant="outline">批次 {activity.batchId}</Badge>
          {!activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyTasks + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyTasks + readyMilestones}</Badge>}
        </>
      )}
    >
      {!activity.observed ? (
        <EmptyState title="花笺集芳状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
      ) : !activity.valid ? (
        <EmptyState title="花笺集芳配置或状态异常" detail="已阻塞自动化；等待完整模板与时间状态同步后再显示任务详情。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat icon={<CalendarDays />} label="活动阶段" value={cyclicNotePhaseLabel(phase)} detail={cyclicNotePhaseDetail(activity)} />
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
              <div className="p-3"><EmptyState title={activity.taskListObserved ? "当前没有任务槽" : "任务列表尚未同步"} /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.tasks.map((task) => <CyclicNoteTaskCard key={`${activity.batchId}:${task.slotId}:${task.taskId}`} task={task} />)}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
              <span>积分里程碑</span>
              <Badge variant="secondary">积分 {formatCount(activity.score)}</Badge>
            </div>
            {activity.milestones.length === 0 ? (
              <div className="p-3"><EmptyState title="暂无里程碑配置" /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => <ActivityMilestoneCard key={milestone.index} milestone={milestone} />)}
              </div>
            )}
          </section>

          {activity.items.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {activity.items.map((item) => <ActivityItemChip key={item.itemId} item={item} />)}
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

function CyclicNoteTaskCard({ task }: { task: CyclicNoteTaskSlot; }) {
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
        <ActivityStatusBadge status={task.status} received={task.received} unknown={!task.catalogKnown} />
      </div>
      {task.target > 0 && <Progress current={progress} target={task.target} percent={percent} />}
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {task.reward.length > 0 ? task.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

export function ActivityMilestoneCard({ milestone }: { milestone: CyclicNoteMilestone; }) {
  const progress = Math.max(0, Math.min(milestone.progress, milestone.target > 0 ? milestone.target : milestone.progress));
  const percent = milestone.target > 0 ? Math.max(0, Math.min(100, Math.round((progress / milestone.target) * 100))) : 0;
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span className="font-medium">积分 {formatCount(milestone.target)}</span>
        <ActivityStatusBadge status={milestone.status} received={milestone.received} />
      </div>
      <Progress current={progress} target={milestone.target} percent={percent} />
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {milestone.reward.length > 0 ? milestone.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}

function Progress({ current, target, percent }: { current: number; target: number; percent: number; }) {
  return (
    <>
      <div className="mt-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span>进度</span>
        <span className="tabular-nums">{formatCount(current)}/{formatCount(target)}</span>
      </div>
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
    </>
  );
}

export function ActivityStatusBadge({ status, received, unknown = false }: { status: PlanStatus; received: boolean; unknown?: boolean; }) {
  if (unknown) return <Badge variant="destructive">未识别</Badge>;
  if (received) return <Badge variant="outline">已领取</Badge>;
  if (status === PlanStatus.READY) return <Badge variant="secondary">可领取</Badge>;
  if (status === PlanStatus.BLOCKED) return <Badge variant="destructive">阻塞</Badge>;
  if (status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">进行中</Badge>;
  return <Badge variant="outline">{planStatusLabel(status)}</Badge>;
}
