import { useMemo } from "react";
import { AlertTriangle, Check, ListChecks, Package } from "lucide-react";
import { PlanStatus, TaskExecutionFeature } from "@/lib/api/workspace-models";
import type { OrderStatisticsView, PendingTaskView, RequirementView, VideoActionStatusView } from "@/lib/api/workspace-models";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
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
import { VideoActionItems } from "@/features/workspace/shared/video-action-status";

export function TaskOrderMonitorPanel({ tasks, statistics, videoOrders, policy }: {
  tasks: PendingTaskView[];
  statistics?: OrderStatisticsView;
  videoOrders: VideoActionStatusView[];
  policy: Policy | null;
}) {
  const monitoredTasks = useMemo(() => [...tasks].sort(comparePendingTasks), [tasks]);
  const orderTasks = useMemo(() => monitoredTasks.filter(isOrderPendingTask), [monitoredTasks]);
  const taskItems = useMemo(() => monitoredTasks.filter((task) => !isOrderPendingTask(task)), [monitoredTasks]);
  const enabledTasks = monitoredTasks.filter((task) => pendingTaskAutomationState(task, policy).kind === "enabled");
  const disabledCount = monitoredTasks.filter((task) => pendingTaskAutomationState(task, policy).kind === "disabled").length;
  const waitingCount = monitoredTasks.filter((task) => pendingTaskAutomationState(task, policy).kind === "waiting").length;
  const readyCount = enabledTasks.filter((task) => task.status === PlanStatus.READY && !pendingTaskCooling(task)).length;
  const coolingCount = monitoredTasks.filter(pendingTaskCooling).length;
  const shortageCount = enabledTasks.filter(pendingTaskHasShortage).length;
  const blockedCount = monitoredTasks.filter(pendingTaskBlocked).length;
  const missingItemCount = enabledTasks.reduce((sum, task) => sum + task.requirements.filter((requirement) => requirement.missing > 0).length, 0);
  const missingSummary = requirementShortageSummary(enabledTasks);
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
          {disabledCount > 0 && <Badge variant="outline">未启用 {disabledCount}</Badge>}
          {waitingCount > 0 && <Badge variant="outline">待处理 {waitingCount}</Badge>}
        </>
      )}
    >
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat icon={<ListChecks />} label="任务" value={taskItems.length} detail={taskMonitorDetailWithPolicy(taskItems, policy)} />
        <OverviewStat icon={<Package />} label="订单" value={orderTasks.length} detail={taskMonitorDetailWithPolicy(orderTasks, policy)} />
        <OverviewStat icon={<AlertTriangle />} label="缺项" value={missingItemCount} detail={missingSummary || "暂无资源缺口"} />
        <OverviewStat
          icon={<Check />}
          label="订单完成"
          value={statistics?.observed ? orderStats.reduce((sum, item) => sum + item.value, 0) : "-"}
          detail={statistics?.observed ? `更新 ${formatUnixTime(statistics.updatedAtMs)}` : "未同步"}
        />
      </div>

      {(disabledCount > 0 || waitingCount > 0) && (
        <div className="rounded-md border border-border/60 bg-secondary/35 px-3 py-2 text-xs leading-5 text-muted-foreground">
          未启用项仅展示状态；任务开关开启后，系统会调用已开启的业务模块尝试推进。条件不足或协议暂不支持时会继续等待，不会反复试探。
        </div>
      )}

      {videoOrders.length > 0 && (
        <section className="rounded-md border border-border/58 bg-white/30 p-2.5 dark:bg-white/4">
          <div className="mb-2 flex flex-wrap items-center justify-between gap-2 px-0.5">
            <div className="text-sm font-semibold">广告订单</div>
            <div className="text-xs text-muted-foreground">当前订单需在官方游戏内观看，自动化不会代领</div>
          </div>
          <VideoActionItems actions={videoOrders} className="sm:grid-cols-2 xl:grid-cols-3" />
        </section>
      )}

      {statistics?.observed && (
        <div className="grid grid-cols-3 gap-2 rounded-md border border-border/70 bg-muted/20 p-2 sm:grid-cols-6">
          {orderStats.map((item) => (
            <div key={item.label} className="flex min-w-0 items-center justify-between gap-2 rounded bg-background/70 px-2 py-2 text-sm sm:px-3">
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
          <PendingTaskGroup title="任务" tasks={taskItems} policy={policy} emptyText="暂无任务待监控" />
          <PendingTaskGroup title="订单" tasks={orderTasks} policy={policy} emptyText="暂无订单待监控" />
        </div>
      )}
    </CollapsibleCard>
  );
}

function PendingTaskGroup({ title, tasks, policy, emptyText }: { title: string; tasks: PendingTaskView[]; policy: Policy | null; emptyText: string }) {
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
          {tasks.map((task, index) => <PendingTaskRow key={`${task.category}-${task.id}-${index}`} task={task} policy={policy} />)}
        </div>
      )}
    </section>
  );
}

function PendingTaskRow({ task, policy }: { task: PendingTaskView; policy: Policy | null }) {
  return (
    <div className="min-h-[4.5rem] px-3 py-2.5">
      <div className="flex items-start gap-3">
        <PendingTaskStatusBadge task={task} policy={policy} />
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
          {task.cooldownReason && <div className="text-xs text-muted-foreground">{task.cooldownReason}</div>}
          {task.requirements.length > 0 && <RequirementChips requirements={task.requirements} />}
        </div>
      </div>
    </div>
  );
}

function PendingTaskStatusBadge({ task, policy }: { task: PendingTaskView; policy: Policy | null }) {
  const disabledLabel = pendingTaskAutomationDisabledLabel(task, policy);
  if (disabledLabel) return <Badge variant="outline">{disabledLabel}</Badge>;
  if (pendingTaskCooling(task)) return <Badge variant="outline">冷却</Badge>;
  if (pendingTaskBlocked(task)) return <Badge variant="destructive">阻塞</Badge>;
  if (pendingTaskHasShortage(task)) return <Badge variant="destructive">缺口</Badge>;
  if (task.status === PlanStatus.READY) return <Badge variant="secondary">可处理</Badge>;
  if (task.status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">同步</Badge>;
  return <Badge variant="outline">{planStatusLabel(task.status)}</Badge>;
}

type PendingTaskAutomationState = { kind: "enabled" | "disabled" | "waiting" | "unknown"; label: string };

function pendingTaskAutomationState(task: PendingTaskView, policy: Policy | null): PendingTaskAutomationState {
  if (!policy) return { kind: "unknown", label: "" };
  if (!policy.automationEnabled) return { kind: "disabled", label: "自动化关闭" };
  const categoryEnabled = pendingTaskCategoryEnabled(task, policy);
  if (categoryEnabled === false) return { kind: "disabled", label: "未启用" };
  if (categoryEnabled === undefined) return { kind: "enabled", label: "" };
  if (task.status === PlanStatus.READY) {
    return { kind: "enabled", label: "" };
  }
  if (!pendingTaskUsesExecutionFeature(task.category)) {
    return { kind: "enabled", label: "" };
  }
  if (task.executionFeature === TaskExecutionFeature.CLAIM_ONLY) {
    return { kind: "waiting", label: "等待自然进度" };
  }
  if (!task.autoCompletionSupported) return { kind: "waiting", label: "暂不支持" };
  if (!pendingTaskExecutionFeatureEnabled(task.executionFeature, policy)) {
    return { kind: "waiting", label: `需开启${taskExecutionFeatureLabel(task.executionFeature)}` };
  }
  return { kind: "enabled", label: "" };
}

function pendingTaskUsesExecutionFeature(category: string) {
  return category === "居民订单"
    || category === "顾客订单"
    || category === "主线任务"
    || category === "主线剧情"
    || category === "日常任务"
    || category === "周常任务"
    || category === "成就任务";
}

function pendingTaskCategoryEnabled(task: PendingTaskView, policy: Policy): boolean | undefined {
  const taskPolicy = policy.basic?.task;
  switch (task.category) {
    case "居民订单":
      return policy.order?.resident?.normalEnabled;
    case "顾客订单":
      return policy.order?.customer?.enabled;
    case "主线任务":
      return taskPolicy?.mainEnabled;
    case "主线剧情":
      return taskPolicy?.storyEnabled;
    case "日常任务":
      return taskPolicy?.dailyEnabled;
    case "周常任务":
      return taskPolicy?.weeklyEnabled;
    case "成就任务":
      return taskPolicy?.achievementEnabled;
    case "地图随机事件":
      return policy.basic?.mapEventEnabled;
    case "宠物事件":
    case "宠物纪念品":
      return policy.basic?.zoo?.enabled && policy.basic.zoo.autoEventEnabled;
    case "activity":
      return task.id.startsWith("story:") || task.id.startsWith("story-box:")
        ? policy.activity?.cyclicStory?.enabled
        : policy.activity?.cyclicNote?.enabled;
    default:
      return undefined;
  }
}

function pendingTaskAutomationDisabledLabel(task: PendingTaskView, policy: Policy | null) {
  return pendingTaskAutomationState(task, policy).label;
}

function pendingTaskExecutionFeatureEnabled(feature: TaskExecutionFeature, policy: Policy) {
  switch (feature) {
    case TaskExecutionFeature.CLAIM_ONLY:
      return true;
    case TaskExecutionFeature.STORY:
      return policy.basic?.task?.storyEnabled === true;
    case TaskExecutionFeature.PLANTING:
      return policy.plant?.planting?.autoEnabled === true;
    case TaskExecutionFeature.RESIDENT_ORDER:
      return policy.order?.resident?.normalEnabled === true;
    case TaskExecutionFeature.FLOWER_RACK:
      return policy.order?.flowerArt?.sellEnabled === true;
    case TaskExecutionFeature.CUSTOMER_ORDER:
      return policy.order?.customer?.enabled === true;
    case TaskExecutionFeature.CULTIVATE_SHOP:
      return policy.basic?.shop?.cultivateShop?.autoBuy === true;
    case TaskExecutionFeature.PALACE_ORDER:
      return policy.order?.palace?.enabled === true;
    case TaskExecutionFeature.PEARL_HIRE:
      return policy.basic?.pearl?.autoHireEnabled === true;
    case TaskExecutionFeature.FRIEND_TOUCH:
      return policy.plant?.friendSteal?.enabled === true;
    case TaskExecutionFeature.ZOO_STROKE:
      return policy.basic?.zoo?.enabled === true && policy.basic.zoo.autoStroke === true;
    case TaskExecutionFeature.CULTIVATION:
      return policy.plant?.cultivate?.enabled === true;
    default:
      return false;
  }
}

function taskExecutionFeatureLabel(feature: TaskExecutionFeature) {
  switch (feature) {
    case TaskExecutionFeature.STORY: return "剧情自动解锁";
    case TaskExecutionFeature.PLANTING: return "自动种植";
    case TaskExecutionFeature.RESIDENT_ORDER: return "居民订单";
    case TaskExecutionFeature.FLOWER_RACK: return "花架出售";
    case TaskExecutionFeature.CUSTOMER_ORDER: return "顾客订单";
    case TaskExecutionFeature.CULTIVATE_SHOP: return "材料商店购买";
    case TaskExecutionFeature.PALACE_ORDER: return "宫廷订单";
    case TaskExecutionFeature.PEARL_HIRE: return "珍珠雇佣";
    case TaskExecutionFeature.FRIEND_TOUCH: return "好友摸花";
    case TaskExecutionFeature.VIDEO: return "视频能力";
    case TaskExecutionFeature.ZOO_STROKE: return "宠物互动";
    case TaskExecutionFeature.CULTIVATION: return "自动培育";
    default: return "相关模块";
  }
}

function taskMonitorDetailWithPolicy(tasks: PendingTaskView[], policy: Policy | null) {
  const enabled = tasks.filter((task) => pendingTaskAutomationState(task, policy).kind === "enabled");
  const disabled = tasks.filter((task) => pendingTaskAutomationState(task, policy).kind === "disabled").length;
  const waiting = tasks.filter((task) => pendingTaskAutomationState(task, policy).kind === "waiting").length;
  const detail = enabled.length > 0 ? taskMonitorDetail(enabled) : "";
  return [detail, disabled > 0 ? `未启用 ${disabled}` : "", waiting > 0 ? `待处理 ${waiting}` : ""].filter(Boolean).join(" / ") || "暂无";
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
