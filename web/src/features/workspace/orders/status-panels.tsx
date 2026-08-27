import { useMemo } from "react";
import { AlertTriangle, Check, ListChecks, Package } from "lucide-react";
import { PlanStatus } from "@/lib/api/workspace-models";
import type { OrderStatisticsView, PendingTaskView, RequirementView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import {
  comparePendingTasks,
  formatCount,
  formatUnixTime,
  isOrderPendingTask,
  orderStatisticItems,
  pendingTaskBlocked,
  pendingTaskCategoryLabel,
  pendingTaskCooling,
  pendingTaskHasShortage,
  pendingTaskProgressPercent,
  planStatusLabel,
  requirementShortageSummary,
  taskMonitorDetail,
  taskProgressLabel,
} from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState, OverviewStat } from "@/features/workspace/shared/workspace-ui";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

export function TaskOrderMonitorPanel({ tasks, statistics }: {
  tasks: PendingTaskView[];
  statistics?: OrderStatisticsView;
}) {
  const monitoredTasks = useMemo(() => [...tasks].sort(comparePendingTasks), [tasks]);
  const orderTasks = useMemo(() => monitoredTasks.filter(isOrderPendingTask), [monitoredTasks]);
  const taskItems = useMemo(() => monitoredTasks.filter((task) => !isOrderPendingTask(task)), [monitoredTasks]);
  const readyCount = monitoredTasks.filter((task) => task.status === PlanStatus.READY && !pendingTaskCooling(task)).length;
  const coolingCount = monitoredTasks.filter(pendingTaskCooling).length;
  const shortageCount = monitoredTasks.filter(pendingTaskHasShortage).length;
  const blockedCount = monitoredTasks.filter(pendingTaskBlocked).length;
  const missingItemCount = monitoredTasks.reduce((sum, task) => sum + task.requirements.filter((requirement) => requirement.missing > 0).length, 0);
  const missingSummary = useMemo(() => requirementShortageSummary(monitoredTasks), [monitoredTasks]);
  const orderStats = orderStatisticItems(statistics);

  return (
    <CollapsibleCard
      title="任务/订单监控"
      contentClassName="space-y-3"
      actions={(
        <>
          <Badge variant="secondary">总计 {monitoredTasks.length}</Badge>
          {readyCount > 0 && <Badge variant="secondary">可处理 {readyCount}</Badge>}
          {coolingCount > 0 && <Badge variant="outline">冷却 {coolingCount}</Badge>}
          {shortageCount > 0 && <Badge variant="outline">缺口 {shortageCount}</Badge>}
          {blockedCount > 0 && <Badge variant="destructive">阻塞 {blockedCount}</Badge>}
        </>
      )}
    >
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat icon={<ListChecks />} label="任务" value={taskItems.length} detail={taskMonitorDetail(taskItems)} />
        <OverviewStat icon={<Package />} label="订单" value={orderTasks.length} detail={taskMonitorDetail(orderTasks)} />
        <OverviewStat icon={<AlertTriangle />} label="缺项" value={missingItemCount} detail={missingSummary || "暂无资源缺口"} />
        <OverviewStat
          icon={<Check />}
          label="订单完成"
          value={statistics?.observed ? orderStats.reduce((sum, item) => sum + item.value, 0) : "-"}
          detail={statistics?.observed ? `更新 ${formatUnixTime(statistics.updatedAtMs)}` : "未同步"}
        />
      </div>

      {statistics?.observed && (
        <div className="dark-scrollbar flex gap-2 overflow-x-auto rounded-md border border-border/70 bg-muted/20 p-2">
          {orderStats.map((item) => (
            <div key={item.label} className="flex min-w-[5.5rem] shrink-0 items-center justify-between gap-3 rounded bg-background/70 px-3 py-2 text-sm sm:min-w-24">
              <span className="text-muted-foreground">{item.label}</span>
              <span className="font-semibold tabular-nums">{formatCount(item.value)}</span>
            </div>
          ))}
        </div>
      )}

      {monitoredTasks.length === 0 ? (
        <EmptyState title="暂无任务/订单快照" />
      ) : (
        <div className="grid gap-3 xl:grid-cols-2">
          <PendingTaskGroup title="任务" tasks={taskItems} emptyText="暂无任务待监控" />
          <PendingTaskGroup title="订单" tasks={orderTasks} emptyText="暂无订单待监控" />
        </div>
      )}
    </CollapsibleCard>
  );
}

function PendingTaskGroup({ title, tasks, emptyText }: { title: string; tasks: PendingTaskView[]; emptyText: string }) {
  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex h-9 items-center justify-between gap-2 bg-secondary/55 px-3 text-sm font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{tasks.length}</Badge>
      </div>
      {tasks.length === 0 ? (
        <div className="p-3"><EmptyState title={emptyText} /></div>
      ) : (
        <div className="dark-scrollbar max-h-[300px] divide-y divide-border/70 overflow-y-auto sm:max-h-[360px]">
          {tasks.map((task, index) => <PendingTaskRow key={`${task.category}-${task.id}-${index}`} task={task} />)}
        </div>
      )}
    </section>
  );
}

function PendingTaskRow({ task }: { task: PendingTaskView }) {
  return (
    <div className="min-h-[4.5rem] px-3 py-2.5">
      <div className="flex items-start gap-3">
        <PendingTaskStatusBadge task={task} />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1 text-sm">
            <span className="shrink-0 text-xs text-muted-foreground">{pendingTaskCategoryLabel(task.category)}</span>
            <span className="min-w-0 truncate font-medium">{task.title || `#${task.id}`}</span>
            {task.id && <span className="shrink-0 font-mono text-xs text-muted-foreground">#{task.id}</span>}
            {taskProgressLabel(task) && <span className="shrink-0 text-xs text-muted-foreground">{taskProgressLabel(task)}</span>}
          </div>
          {task.target > 0 && (
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary" style={{ width: `${pendingTaskProgressPercent(task)}%` }} />
            </div>
          )}
          {task.requirements.length > 0 && <RequirementChips requirements={task.requirements} />}
        </div>
      </div>
    </div>
  );
}

function PendingTaskStatusBadge({ task }: { task: PendingTaskView }) {
  if (pendingTaskCooling(task)) return <Badge variant="outline">冷却</Badge>;
  if (pendingTaskBlocked(task)) return <Badge variant="destructive">阻塞</Badge>;
  if (pendingTaskHasShortage(task)) return <Badge variant="destructive">缺口</Badge>;
  if (task.status === PlanStatus.READY) return <Badge variant="secondary">可处理</Badge>;
  if (task.status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">同步</Badge>;
  return <Badge variant="outline">{planStatusLabel(task.status)}</Badge>;
}

function RequirementChips({ requirements }: { requirements: RequirementView[] }) {
  const visible = requirements.slice(0, 4);
  return (
    <div className="flex flex-wrap gap-1.5">
      {visible.map((requirement, index) => (
        <span
          key={`${requirement.itemId}-${requirement.required}-${requirement.owned}-${index}`}
          className={cn(
            "inline-flex min-h-6 max-w-full items-center gap-1 rounded border px-2 py-0.5 text-xs",
            requirement.missing > 0 ? "border-destructive/35 bg-destructive/10 text-destructive" : "border-border/58 bg-white/42 text-muted-foreground dark:bg-white/5",
          )}
          title={requirement.blockedReasons.join("、")}
        >
          <span className="truncate">{requirement.itemName || itemName(requirement.itemId)}</span>
          <span className="shrink-0 tabular-nums">{formatCount(requirement.owned)}/{formatCount(requirement.required)}</span>
        </span>
      ))}
      {requirements.length > visible.length && (
        <span className="inline-flex min-h-6 items-center rounded border border-border/58 bg-white/42 px-2 py-0.5 text-xs text-muted-foreground dark:bg-white/5">
          +{requirements.length - visible.length}
        </span>
      )}
    </div>
  );
}
