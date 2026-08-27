import { CalendarDays, Flower2, ListChecks, Trophy } from "lucide-react";
import { PlanStatus } from "@/lib/api/workspace-models";
import type { CyclicStoryOrder, CyclicStoryView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { cyclicNotePhaseLabel, cyclicStoryPhaseDetail, formatCount } from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState, OverviewStat } from "@/features/workspace/shared/workspace-ui";
import { itemName } from "@/lib/game/catalog";
import ActivityItemChip from "./activity-item-chip";
import { ActivityMilestoneCard, ActivityStatusBadge } from "./status-panels";

export default function CyclicStoryMonitorPanel({ activity }: { activity?: CyclicStoryView }) {
  const phase = activity?.phase ?? 0;
  if (!activity?.found || (phase !== 1 && phase !== 2 && phase !== 3)) return null;

  const activeOrders = activity.orders.filter((order) => order.orderId > 0 && !order.onCooldown);
  const readyOrders = activity.valid ? activeOrders.filter((order) => order.status === PlanStatus.READY).length : 0;
  const readyMilestones = activity.valid ? activity.milestones.filter((milestone) => milestone.ready && !milestone.received).length : 0;

  return (
    <CollapsibleCard
      title={activity.name || "莳花纪闻"}
      contentClassName="space-y-3"
      actions={(
        <>
          <Badge variant={phase === 2 ? "secondary" : "outline"}>{cyclicNotePhaseLabel(phase)}</Badge>
          <Badge variant="outline">批次 {activity.batchId}</Badge>
          {!activity.valid && <Badge variant="destructive">配置异常</Badge>}
          {activity.valid && !activity.milestoneReceiptsObserved && <Badge variant="outline">里程碑待同步</Badge>}
          {readyOrders + readyMilestones > 0 && <Badge variant="secondary">可领取 {readyOrders + readyMilestones}</Badge>}
        </>
      )}
    >
      {!activity.observed ? (
        <EmptyState title="莳花纪闻状态尚未同步" detail="连接游戏后，监控会从活动状态中自动发现当前批次。" />
      ) : !activity.valid ? (
        <EmptyState title="莳花纪闻配置或状态异常" detail="已阻塞自动化；等待完整模板与时间状态同步后再显示订单详情。" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat icon={<CalendarDays />} label="活动阶段" value={cyclicNotePhaseLabel(phase)} detail={cyclicStoryPhaseDetail(activity)} />
            <OverviewStat icon={<Trophy />} label="累计积分" value={formatCount(activity.score)} detail={`完成订单 ${formatCount(activity.finishCount)} 次`} />
            <OverviewStat icon={<Flower2 />} label="花史残页" value={formatCount(activity.currencyBalance)} detail={activity.currencyItemId > 0 ? `${itemName(activity.currencyItemId)} #${activity.currencyItemId}` : "活动货币未识别"} />
            <OverviewStat icon={<ListChecks />} label="订单槽" value={`${activeOrders.length}/${activity.orders.length}`} detail={readyOrders > 0 ? `${readyOrders} 个订单可交` : activity.ordersObserved ? "已同步" : "等待进入活动同步"} />
          </div>

          {activity.description && <div className="rounded-md border border-border/58 bg-muted/20 px-3 py-2 text-sm text-muted-foreground">{activity.description}</div>}

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45"><span>订单详情</span><Badge variant="secondary">{activity.orders.length} 槽</Badge></div>
            {activity.orders.length === 0 ? (
              <div className="p-3"><EmptyState title={activity.ordersObserved ? "当前没有订单槽" : "订单列表尚未同步"} /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.orders.map((order) => <CyclicStoryOrderCard key={`${activity.batchId}:${order.orderIdx}:${order.orderId}`} order={order} />)}
              </div>
            )}
          </section>

          <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45"><span>积分里程碑</span><Badge variant="secondary">积分 {formatCount(activity.score)}</Badge></div>
            {activity.milestones.length === 0 ? (
              <div className="p-3"><EmptyState title="当前没有里程碑" /></div>
            ) : (
              <div className="grid gap-2 p-2 lg:grid-cols-3">
                {activity.milestones.map((milestone) => <ActivityMilestoneCard key={milestone.index} milestone={milestone} />)}
              </div>
            )}
          </section>

          {activity.items.length > 0 && <div className="flex flex-wrap gap-1.5">{activity.items.map((item) => <ActivityItemChip key={item.itemId} item={item} />)}</div>}
        </>
      )}
    </CollapsibleCard>
  );
}

function CyclicStoryOrderCard({ order }: { order: CyclicStoryOrder }) {
  if (order.onCooldown || order.orderId <= 0) {
    return (
      <div className="rounded-md border border-dashed border-border/70 bg-muted/15 p-3 text-sm text-muted-foreground">
        <div className="flex items-center justify-between gap-2"><span className="font-medium">订单槽 {order.orderIdx}</span><Badge variant="outline">{order.onCooldown ? "冷却中" : "空闲"}</Badge></div>
        <div className="mt-3 text-xs">仅监控，不会自动付费刷新或清 CD。</div>
      </div>
    );
  }
  return (
    <div className="rounded-md border border-border/58 bg-background/72 p-3 text-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs text-muted-foreground">订单槽 {order.orderIdx} · #{order.orderId}</div>
          <div className="mt-1 line-clamp-2 font-medium">{order.flowerId > 0 ? `${itemName(order.flowerId)} x${formatCount(order.cost)}` : `订单 #${order.orderId}`}</div>
        </div>
        <ActivityStatusBadge status={order.status} received={false} unknown={!order.catalogKnown} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>奖励</span>
        {order.reward.length > 0 ? order.reward.map((item, index) => <ActivityItemChip key={`${item.itemId}:${index}`} item={item} compact />) : <span>未配置</span>}
      </div>
    </div>
  );
}
