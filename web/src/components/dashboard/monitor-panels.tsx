"use client";

import { Fragment, useEffect, useMemo, useState, type ReactNode } from "react";
import { AlertTriangle, CalendarDays, Check, ChevronDown, Coins, Flower2, Gem, HandCoins, ListChecks, Package, Search, ShieldCheck, ShoppingBag, Sparkles, Ticket, TrendingUp, Trophy, Waves } from "lucide-react";
import { ExecutionLane, PlanStatus } from "@/gen/mygardenworld/v1/query_service_pb";
import type { AccountStatus, BusinessStatisticsView, Event, FmlLandView, GetSnapshotResponse, InventoryLedgerItem, InventoryLedgerView, LandView, OrderStatisticsView, PendingTaskView, PlantableFlowerView, PlannedOperation, RequirementView, RuntimeActionTotal, RuntimeStatisticsView } from "@/gen/mygardenworld/v1/query_service_pb";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { experienceToNextLevel, itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";
import { OperationStatusBadge, comparePendingTasks, isOrderPendingTask, pendingTaskCategoryLabel, pendingTaskBlocked, pendingTaskHasShortage, pendingTaskCooling, taskMonitorDetail, requirementShortageSummary, pendingTaskProgressPercent, taskProgressLabel, orderStatisticItems, businessOrderItems, sumRuntimeActionTotals, runtimeWindowLabel, runtimeResourcePrimaryValue, runtimeResourceGainSummary, runtimeActionSummary, planStatusLabel, operationTitle, operationTargetLabel, operationCostLabel, operationNoteLabel, isOperationCooling, eventCategory, collapseRaceSyncLogEvents, eventTitle, eventMessage, categoryLabel, recommendationLabel, landStatusLabel, landDisplayNumber, landDisplayName, landTimingLabel, fmlLandTimingLabel, formatTimestamp, formatUnixTime, formatDayId, firstPositiveUnixTime, formatCount } from "@/components/dashboard/dashboard-utils";

type WarehouseCategory = "flower" | "art" | "item";

const WAREHOUSE_CATEGORIES: { id: WarehouseCategory; label: string; icon: ReactNode }[] = [
  { id: "flower", label: "鲜花", icon: <Flower2 /> },
  { id: "art", label: "花艺", icon: <Sparkles /> },
  { id: "item", label: "道具", icon: <Package /> },
];

const SPEED_UP_TICKET_ITEM_ID = 1001;
const FLORAL_COIN_ITEM_ID = 1002;

export function CollapsibleCard({
  title,
  actions,
  children,
  className,
  contentClassName,
  defaultOpen = true,
}: {
  title: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <Card className={cn("cloud-surface bg-card/88", !open && "gap-0", className)}>
      <CardHeader className="border-b border-border/42 px-3 pb-3 sm:px-4">
        <div className="flex flex-wrap items-center justify-between gap-2 sm:gap-3">
          <button
            type="button"
            className="flex min-w-0 items-center gap-2 text-left text-foreground transition-colors hover:text-primary active:scale-[0.99]"
            aria-expanded={open}
            onClick={() => setOpen((value) => !value)}
          >
            <ChevronDown className={cn("size-4 shrink-0 transition-transform", !open && "-rotate-90")} />
            <CardTitle className="truncate">{title}</CardTitle>
          </button>
          {actions && <div className="flex min-w-0 flex-wrap justify-end gap-1.5">{actions}</div>}
        </div>
      </CardHeader>
      {open && <CardContent className={cn("px-3 sm:px-4", contentClassName)}>{children}</CardContent>}
    </Card>
  );
}

export function StatusOverviewPanel({ snapshot, status }: { snapshot: GetSnapshotResponse | null; status?: AccountStatus }) {
  const floralCoins = snapshot?.inventory[FLORAL_COIN_ITEM_ID] ?? 0;
  const speedUpTickets = snapshot?.inventory[SPEED_UP_TICKET_ITEM_ID] ?? 0;
  const reputationObserved = snapshot?.reputationObserved ?? status?.reputationObserved ?? false;
  const reputationScore = snapshot?.reputationScore ?? status?.reputationScore ?? 0;
  const reputationTime = firstPositiveUnixTime(
    snapshot?.reputationLastViewTimeMs,
    snapshot?.reputationLastSyncTimeMs,
    status?.reputationLastViewTimeMs,
    status?.reputationLastSyncTimeMs,
  );
  const level = snapshot?.level ?? status?.level ?? 0;
  const experience = snapshot?.experience ?? status?.experience ?? 0;
  const apiNextLevelExperience = snapshot?.nextLevelExperience ?? status?.nextLevelExperience ?? 0;
  const apiLevelMaxed = snapshot?.levelMaxed ?? status?.levelMaxed ?? false;
  const apiHasNextLevel = apiLevelMaxed || apiNextLevelExperience > 0;
  const localNextLevel = experienceToNextLevel(level, experience);
  const levelMaxed = apiHasNextLevel ? apiLevelMaxed : localNextLevel.maxed;
  const nextLevelExperience = apiHasNextLevel ? apiNextLevelExperience : localNextLevel.required;
  const experienceToNext = apiHasNextLevel
    ? (snapshot?.experienceToNextLevel ?? status?.experienceToNextLevel ?? 0)
    : localNextLevel.remaining;
  const reputationDetail = reputationObserved ? (reputationTime ? `同步 ${formatUnixTime(reputationTime)}` : "已同步") : "未同步";
  const nextLevelValue = levelMaxed
    ? "已满级"
    : nextLevelExperience > 0
      ? `${formatCount(experienceToNext)} 经验`
      : "-";
  const nextLevelDetail = levelMaxed
    ? "已达最高等级"
    : nextLevelExperience > 0
      ? `当前 ${formatCount(experience)} / 需要 ${formatCount(nextLevelExperience)}`
      : undefined;
  return (
    <CollapsibleCard title="监控概览" actions={snapshot?.capturedAt && <Badge variant="outline">快照 {formatTimestamp(snapshot.capturedAt)}</Badge>}>
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat
          icon={<ShieldCheck />}
          label="礼仪分"
          value={reputationObserved ? formatCount(reputationScore) : "-"}
          detail={reputationDetail}
        />
        <OverviewStat icon={<Trophy />} label="等级" value={level > 0 ? `${level}级` : "-"} detail={`经验 ${formatCount(experience)}`} />
        <OverviewStat
          icon={<TrendingUp />}
          label="距下一等级"
          value={nextLevelValue}
          detail={nextLevelDetail}
          wrap
          compact
        />
        <OverviewStat icon={<Waves />} label="水滴" value={`${formatCount(snapshot?.waterDrops ?? 0)}/${formatCount(snapshot?.waterDropsTotal ?? 0)}`} />
        <OverviewStat icon={<Gem />} label="元宝" value={formatCount(snapshot?.diamondsFree ?? 0)} />
        <OverviewStat icon={<Coins />} label="金币" value={formatCount(snapshot?.gold ?? 0)} />
        <OverviewStat icon={<HandCoins />} label="花坊币" value={formatCount(floralCoins)} />
        <OverviewStat icon={<Ticket />} label="加速卡" value={formatCount(speedUpTickets)} />

      </div>
    </CollapsibleCard>
  );
}

export function RuntimeStatisticsPanel({ runtimeStatistics }: { runtimeStatistics?: RuntimeStatisticsView }) {
  const runtimeOrderCompletions = runtimeStatistics?.orderCompletions ?? [];
  const runtimeTaskCompletions = runtimeStatistics?.taskCompletions ?? [];
  const runtimeTotalOperations = runtimeStatistics?.totalOperations ?? BigInt(0);
  const runtimeOrderTotal = sumRuntimeActionTotals(runtimeOrderCompletions);
  const runtimeTaskTotal = sumRuntimeActionTotals(runtimeTaskCompletions);
  const runtimeResourceGains = runtimeStatistics?.resourceGains ?? [];
  const showCompletionGroups =
    runtimeOrderCompletions.length > 0 || runtimeTaskCompletions.length > 0 || runtimeTotalOperations > BigInt(0);

  return (
    <CollapsibleCard
      title="本次运行统计"
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant={runtimeStatistics?.running ? "secondary" : "outline"}>{runtimeStatistics ? (runtimeStatistics.running ? "运行中" : "已停止") : "暂无"}</Badge>
          {runtimeTotalOperations > BigInt(0) && <Badge variant="outline">操作 {formatCount(runtimeTotalOperations)}</Badge>}
        </>
      }
    >
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        <OverviewStat icon={<CalendarDays />} label="本次运行" value={runtimeStatistics ? (runtimeStatistics.running ? "运行中" : "已停止") : "-"} detail={runtimeWindowLabel(runtimeStatistics)} />
        <OverviewStat icon={<Sparkles />} label="本次获取" value={runtimeResourcePrimaryValue(runtimeResourceGains)} detail={runtimeResourceGainSummary(runtimeResourceGains)} />
        <OverviewStat icon={<ShoppingBag />} label="本次订单" value={runtimeStatistics ? formatCount(runtimeOrderTotal) : "-"} detail={runtimeActionSummary(runtimeOrderCompletions)} />
        <OverviewStat icon={<ListChecks />} label="本次任务" value={runtimeStatistics ? formatCount(runtimeTaskTotal) : "-"} detail={runtimeActionSummary(runtimeTaskCompletions)} />
      </div>
      {showCompletionGroups && (
        <div className="grid gap-2 xl:grid-cols-2">
          <RuntimeCompletionGroup title="订单完成" items={runtimeOrderCompletions} emptyText="本次暂无订单完成" />
          <RuntimeCompletionGroup title="任务完成" items={runtimeTaskCompletions} emptyText="本次暂无任务完成" />
        </div>
      )}
    </CollapsibleCard>
  );
}

export function TaskOrderMonitorPanel({
  tasks,
  statistics,
}: {
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
  const missingItemCount = monitoredTasks.reduce((sum, task) => sum + task.requirements.filter((req) => req.missing > 0).length, 0);
  const missingSummary = useMemo(() => requirementShortageSummary(monitoredTasks), [monitoredTasks]);
  const orderStats = orderStatisticItems(statistics);

  return (
    <CollapsibleCard
      title="任务/订单监控"
      contentClassName="space-y-3"
      actions={
        <>
          <Badge variant="secondary">总计 {monitoredTasks.length}</Badge>
          {readyCount > 0 && <Badge variant="secondary">可处理 {readyCount}</Badge>}
          {coolingCount > 0 && <Badge variant="outline">冷却 {coolingCount}</Badge>}
          {shortageCount > 0 && <Badge variant="outline">缺口 {shortageCount}</Badge>}
          {blockedCount > 0 && <Badge variant="destructive">阻塞 {blockedCount}</Badge>}
        </>
      }
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

export function RuntimeCompletionGroup({ title, items, emptyText }: { title: string; items: RuntimeActionTotal[]; emptyText: string }) {
  const total = sumRuntimeActionTotals(items);
  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex min-h-9 items-center justify-between gap-2 bg-secondary/55 px-3 py-1.5 text-sm font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{formatCount(total)}</Badge>
      </div>
      <div className="p-2">
        {items.length === 0 ? (
          <EmptyState title={emptyText} />
        ) : (
          <div className="flex flex-wrap gap-2">
            {items.map((item) => (
              <span key={item.key} className="inline-flex min-h-8 items-center gap-2 rounded border border-border/58 bg-background/72 px-3 py-1 text-sm">
                <span className="text-muted-foreground">{item.label || item.key}</span>
                <span className="font-semibold tabular-nums">{formatCount(item.count)}</span>
              </span>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

export function PendingTaskGroup({ title, tasks, emptyText }: { title: string; tasks: PendingTaskView[]; emptyText: string }) {
  return (
    <section className="min-w-0 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
      <div className="flex h-9 items-center justify-between gap-2 bg-secondary/55 px-3 text-sm font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{tasks.length}</Badge>
      </div>
      {tasks.length === 0 ? (
        <div className="p-3">
          <EmptyState title={emptyText} />
        </div>
      ) : (
        <div className="dark-scrollbar max-h-[300px] divide-y divide-border/70 overflow-y-auto sm:max-h-[360px]">
          {tasks.map((task, index) => (
            <PendingTaskRow key={`${task.category}-${task.id}-${index}`} task={task} />
          ))}
        </div>
      )}
    </section>
  );
}

export function PendingTaskRow({ task }: { task: PendingTaskView }) {
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

export function PendingTaskStatusBadge({ task }: { task: PendingTaskView }) {
  if (pendingTaskCooling(task)) return <Badge variant="outline">冷却</Badge>;
  if (pendingTaskBlocked(task)) return <Badge variant="destructive">阻塞</Badge>;
  if (pendingTaskHasShortage(task)) return <Badge variant="destructive">缺口</Badge>;
  if (task.status === PlanStatus.READY) return <Badge variant="secondary">可处理</Badge>;
  if (task.status === PlanStatus.SYNC_ONLY) return <Badge variant="outline">同步</Badge>;
  return <Badge variant="outline">{planStatusLabel(task.status)}</Badge>;
}

export function RequirementChips({ requirements }: { requirements: RequirementView[] }) {
  const visible = requirements.slice(0, 4);
  return (
    <div className="flex flex-wrap gap-1.5">
      {visible.map((req, index) => (
        <span
          key={`${req.itemId}-${req.required}-${req.owned}-${index}`}
          className={cn(
            "inline-flex min-h-6 max-w-full items-center gap-1 rounded border px-2 py-0.5 text-xs",
            req.missing > 0 ? "border-destructive/35 bg-destructive/10 text-destructive" : "border-border/58 bg-white/42 text-muted-foreground dark:bg-white/5",
          )}
          title={req.blockedReasons.join("、")}
        >
          <span className="truncate">{req.itemName || itemName(req.itemId)}</span>
          <span className="shrink-0 tabular-nums">
            {formatCount(req.owned)}/{formatCount(req.required)}
          </span>
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

export function LandMonitorPanel({
  lands,
  waterDrops,
  waterDropsTotal,
  minWaterDrops,
}: {
  lands: LandView[];
  waterDrops: number;
  waterDropsTotal: number;
  minWaterDrops: number;
}) {
  const landsByDisplay = useMemo(() => {
    const map = new Map<number, LandView>();
    for (const land of lands) {
      map.set(landDisplayNumber(land.landId), land);
    }
    return map;
  }, [lands]);
  const mapSlots = useMemo(() => {
    // 8×8 map order: left 1-32 by rows of 4, right 33-64 by rows of 4.
    // Row 0: 1-4, 33-36 … Row 7: 29-32, 61-64
    const slots: number[] = [];
    for (let row = 0; row < 8; row++) {
      for (let i = 0; i < 4; i++) slots.push(row * 4 + 1 + i);
      for (let i = 0; i < 4; i++) slots.push(33 + row * 4 + i);
    }
    return slots;
  }, []);
  const recommendationCounts = useMemo(() => {
    const stats = new Map<string, number>();
    for (const land of lands) {
      if (land.landStatus !== "opened") continue;
      stats.set(land.recommendation || "unknown", (stats.get(land.recommendation || "unknown") ?? 0) + 1);
    }
    return stats;
  }, [lands]);
  const availableWaterDrops = Math.max(0, waterDrops - minWaterDrops);
  const openedCount = lands.filter((land) => land.landStatus === "opened").length;
  const unopenedCount = lands.filter((land) => land.landStatus === "unopened").length;
  const lockedCount = lands.filter((land) => land.landStatus === "locked").length;
  const statusOrder = ["harvest", "plant", "water", "wait"] as const;

  return (
    <CollapsibleCard
      title="土地"
      actions={
        <>
          <Badge variant="secondary">已开 {openedCount}</Badge>
          {unopenedCount > 0 && <Badge variant="outline">未开 {unopenedCount}</Badge>}
          {lockedCount > 0 && <Badge variant="outline">锁定 {lockedCount}</Badge>}
        </>
      }
    >
      {lands.length === 0 ? (
        <EmptyState title="暂无土地快照" />
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {statusOrder.map((key) => {
              const count = recommendationCounts.get(key) ?? 0;
              return (
                <Fragment key={key}>
                  {count > 0 && (
                    <Badge variant="outline">
                      {recommendationLabel(key)} {count}
                    </Badge>
                  )}
                  {key === "plant" && (
                    <Badge variant="outline">
                      水滴总数 {formatCount(waterDrops)}/{formatCount(waterDropsTotal)}
                    </Badge>
                  )}
                </Fragment>
              );
            })}
            {minWaterDrops > 0 && (
              <Badge variant="outline">可用水滴数 {formatCount(availableWaterDrops)}</Badge>
            )}
          </div>
          <div className="dark-scrollbar max-h-[440px] overflow-y-auto pr-0.5 sm:h-[560px] sm:max-h-none sm:pr-1">
            <div
              className="grid gap-2"
              style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr)) 0.75rem repeat(4, minmax(0, 1fr))" }}
            >
              {mapSlots.flatMap((display, index) => {
                const tile = (() => {
                  const land = landsByDisplay.get(display);
                  if (!land) {
                    return (
                      <div
                        key={`slot-${display}`}
                        className="flex min-h-[78px] items-center justify-center rounded-md border border-dashed border-border/45 text-xs text-muted-foreground"
                      >
                        #{display}
                      </div>
                    );
                  }
                  return <LandTile key={land.landId} land={land} />;
                })();
                if (index % 8 !== 4) return [tile];
                return [<div key={`aisle-${index}`} className="min-h-[78px]" aria-hidden />, tile];
              })}
            </div>
          </div>
        </div>
      )}
    </CollapsibleCard>
  );
}

export function LandTile({ land }: { land: LandView }) {
  const planted = land.flowerId > 0;
  const status = land.landStatus || (land.observed ? "opened" : "unknown");
  const opened = status === "opened";
  const recommendation = recommendationLabel(land.recommendation);
  const timing = landTimingLabel(land, status);
  return (
    <div
      className={cn(
        "min-h-[78px] rounded-md border border-border/58 bg-white/58 p-1.5 shadow-sm transition-colors dark:bg-white/6 sm:p-2",
        opened && land.recommendation === "harvest" && "border-primary/50 bg-primary/8",
        opened && land.recommendation === "water" && "border-sky-300/70 bg-sky-50/72 dark:bg-sky-500/10",
        opened && land.recommendation === "plant" && "border-amber-300/70 bg-amber-50/76 dark:bg-amber-400/10",
        !opened && "opacity-70",
        !land.observed && opened && "opacity-70",
      )}
    >
      <div className="flex items-start justify-between gap-1">
        <div className="min-w-0">
          <div className="font-mono text-xs font-medium sm:text-sm">{landDisplayName(land.landId)}</div>
        </div>
        <Badge variant={opened && land.recommendation === "harvest" ? "secondary" : "outline"} className="h-5 shrink-0 px-1 text-[10px] sm:px-1.5 sm:text-[11px]">
          {opened ? recommendation : landStatusLabel(status)}
        </Badge>
      </div>
      <div className="mt-1 truncate text-xs sm:text-sm">{opened ? (planted ? itemName(land.flowerId) : "空地") : landStatusLabel(status)}</div>
      <div className="mt-1 text-[10px] text-muted-foreground sm:text-xs">
        <div className="truncate">
          {opened ? (
            <>
              {land.lvl ? `${land.lvl}级` : "-"}
              {planted ? ` · 收${land.harvestCnt || 0}` : ""}
            </>
          ) : land.openLevel > 0 ? (
            `${land.openLevel}级解锁`
          ) : (
            "-"
          )}
        </div>
        <div className="text-left">{timing}</div>
      </div>
    </div>
  );
}

export function FmlLandMonitorPanel({
  lands,
  plantableFlowers,
  observed,
  automationEnabled,
}: {
  lands: FmlLandView[];
  plantableFlowers: PlantableFlowerView[];
  observed: boolean;
  automationEnabled: boolean;
}) {
  const flowerLvlById = useMemo(() => {
    const levels = new Map<number, number>();
    for (const flower of plantableFlowers) {
      if (flower.flowerId > 0 && flower.lvl > 0) {
        levels.set(flower.flowerId, flower.lvl);
      }
    }
    return levels;
  }, [plantableFlowers]);
  const recommendationCounts = useMemo(() => {
    const stats = new Map<string, number>();
    for (const land of lands) {
      stats.set(land.recommendation || "unknown", (stats.get(land.recommendation || "unknown") ?? 0) + 1);
    }
    return stats;
  }, [lands]);
  const pendingTotal = useMemo(
    () => lands.reduce((sum, land) => sum + (land.pendingHarvest || 0), 0),
    [lands],
  );
  const plantedCount = lands.filter((land) => land.flowerId > 0).length;
  const emptyCount = lands.filter((land) => land.flowerId <= 0).length;
  const statusOrder = ["harvest", "plant", "wait"] as const;

  return (
    <CollapsibleCard
      title="公会土地"
      actions={
        <>
          {!observed ? (
            <Badge variant="outline">等待同步</Badge>
          ) : (
            <Badge variant="secondary">已观测 {lands.length}</Badge>
          )}
          {observed && plantedCount > 0 && <Badge variant="outline">种植中 {plantedCount}</Badge>}
          {observed && emptyCount > 0 && <Badge variant="outline">空地 {emptyCount}</Badge>}
          {pendingTotal > 0 && <Badge variant="secondary">可收 {pendingTotal}</Badge>}
        </>
      }
    >
      {!observed ? (
        <EmptyState
          title="公会土地尚未同步"
          detail={
            automationEnabled
              ? "账号运行中时会自动执行 fml.enter 拉取公会土地；稍等数秒后刷新即可。"
              : "请先启动账号自动化，守护进程会自动进入公会并同步土地种植信息。"
          }
        />
      ) : lands.length === 0 ? (
        <EmptyState title="暂无公会土地" detail="当前账号还没有可观测的公会土地槽位。" />
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {statusOrder.map((key) => {
              const count = recommendationCounts.get(key) ?? 0;
              if (count <= 0) return null;
              return (
                <Badge key={key} variant="outline">
                  {recommendationLabel(key)} {count}
                </Badge>
              );
            })}
          </div>
          <div className="grid grid-cols-3 gap-2">
            {lands.map((land) => (
              <FmlLandTile
                key={land.landId}
                land={land}
                flowerLvl={land.flowerLvl > 0 ? land.flowerLvl : flowerLvlById.get(land.flowerId) ?? 0}
              />
            ))}
          </div>
        </div>
      )}
    </CollapsibleCard>
  );
}

export function FmlLandTile({ land, flowerLvl }: { land: FmlLandView; flowerLvl: number }) {
  const planted = land.flowerId > 0;
  const recommendation = recommendationLabel(land.recommendation);
  const timing = fmlLandTimingLabel(land);
  const stockLabel =
    planted && land.stockCap > 0
      ? `${land.pendingHarvest}/${land.stockCap}`
      : planted
        ? `收${land.harvestedCount || 0}`
        : "";
  const flowerLabel = planted
    ? flowerLvl > 0
      ? `${itemName(land.flowerId)} lv${flowerLvl}`
      : itemName(land.flowerId)
    : "空地";
  return (
    <div
      className={cn(
        "min-h-[78px] rounded-md border border-border/58 bg-white/58 p-1.5 shadow-sm transition-colors dark:bg-white/6 sm:p-2",
        land.recommendation === "harvest" && "border-primary/50 bg-primary/8",
        land.recommendation === "plant" && "border-amber-300/70 bg-amber-50/76 dark:bg-amber-400/10",
      )}
    >
      <div className="flex items-start justify-between gap-1">
        <div className="font-mono text-xs font-medium sm:text-sm">#{land.landId}</div>
        <Badge
          variant={land.recommendation === "harvest" ? "secondary" : "outline"}
          className="h-5 shrink-0 px-1 text-[10px] sm:px-1.5 sm:text-[11px]"
        >
          {recommendation}
        </Badge>
      </div>
      <div className="mt-1 truncate text-xs sm:text-sm">{flowerLabel}</div>
      <div className="mt-1 text-[10px] text-muted-foreground sm:text-xs">
        <div className="truncate">
          {`地${land.level}级`}
          {stockLabel ? ` · ${stockLabel}` : ""}
          {land.pendingHarvest > 0 ? ` · 待收${land.pendingHarvest}` : ""}
        </div>
        <div className="text-left">{timing}</div>
      </div>
    </div>
  );
}

export function warehouseCategoryForItem(item: InventoryLedgerItem): WarehouseCategory {
  const id = item.itemId;
  if (id >= 23000 && id < 24000) return "flower";
  if (id >= 300000 && id < 400000) return "art";
  return "item";
}

export function warehouseCategoryLabel(category: WarehouseCategory) {
  return WAREHOUSE_CATEGORIES.find((entry) => entry.id === category)?.label ?? "仓库";
}

export function warehouseSearchPlaceholder(category: WarehouseCategory) {
  switch (category) {
    case "flower":
      return "搜索花朵或 ID";
    case "art":
      return "搜索花艺或 ID";
    case "item":
      return "搜索道具或 ID";
  }
}

export function WarehouseMonitorPanel({ ledger }: { ledger?: InventoryLedgerView }) {
  const [inventoryQuery, setInventoryQuery] = useState("");
  const [warehouseCategory, setWarehouseCategory] = useState<WarehouseCategory>("flower");
  const inventoryItems = useMemo(() => {
    return [...(ledger?.items ?? [])]
      .filter((item) => item.owned > 0 || item.allocated > 0)
      .sort((a, b) => b.owned - a.owned || b.allocated - a.allocated || a.itemId - b.itemId);
  }, [ledger]);
  const categoryCounts = useMemo(() => {
    const counts = new Map<WarehouseCategory, number>();
    for (const category of WAREHOUSE_CATEGORIES) counts.set(category.id, 0);
    for (const item of inventoryItems) {
      const category = warehouseCategoryForItem(item);
      counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    return counts;
  }, [inventoryItems]);
  const categoryItems = useMemo(() => {
    return inventoryItems.filter((item) => warehouseCategoryForItem(item) === warehouseCategory);
  }, [inventoryItems, warehouseCategory]);
  const visibleItems = useMemo(() => {
    const query = inventoryQuery.trim().toLowerCase();
    if (!query) return categoryItems;
    return categoryItems.filter((item) => {
      const name = item.itemName || itemName(item.itemId);
      return name.toLowerCase().includes(query) || String(item.itemId).includes(query);
    });
  }, [categoryItems, inventoryQuery]);
  const categoryLabel = warehouseCategoryLabel(warehouseCategory);

  return (
    <CollapsibleCard
      title="仓库"
      actions={
        inventoryItems.length > 0 ? (
          <>
            <Badge variant="secondary">种类 {inventoryItems.length}</Badge>
            {inventoryQuery.trim() && <Badge variant="outline">匹配 {visibleItems.length}</Badge>}
          </>
        ) : undefined
      }
    >
      {inventoryItems.length > 0 && (
        <div className="mb-3 grid gap-2 lg:grid-cols-[minmax(296px,1fr)_minmax(150px,0.65fr)] lg:items-center">
          <div className="grid min-w-0 grid-cols-3 rounded-md border border-border/58 bg-white/42 p-1 dark:bg-white/5">
            {WAREHOUSE_CATEGORIES.map((category) => (
              <button
                key={category.id}
                type="button"
                aria-pressed={warehouseCategory === category.id}
                onClick={() => {
                  setWarehouseCategory(category.id);
                  setInventoryQuery("");
                }}
                className={cn(
                  "flex h-8 min-w-0 items-center justify-center gap-1.5 rounded px-2 text-xs font-medium transition-colors",
                  warehouseCategory === category.id
                    ? "bg-white text-foreground shadow-sm dark:bg-muted"
                    : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
                )}
              >
                <span className="shrink-0 [&_svg]:size-3.5">{category.icon}</span>
                <span className="shrink-0 whitespace-nowrap">{category.label}</span>
                <span className="shrink-0 tabular-nums text-muted-foreground">{categoryCounts.get(category.id) ?? 0}</span>
              </button>
            ))}
          </div>
          <div className="relative min-w-0">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={inventoryQuery}
              onChange={(event) => setInventoryQuery(event.target.value)}
              placeholder={warehouseSearchPlaceholder(warehouseCategory)}
              className="h-10 rounded-md pl-9"
            />
          </div>
        </div>
      )}
      {inventoryItems.length === 0 ? (
        <EmptyState title="暂无仓库数据" />
      ) : categoryItems.length === 0 ? (
        <EmptyState title={`暂无${categoryLabel}`} />
      ) : visibleItems.length === 0 ? (
        <EmptyState title={`没有匹配${categoryLabel}`} detail="换个名称或 ID 再试试" />
      ) : (
        <div className="dark-scrollbar max-h-[440px] overflow-y-auto rounded-md border border-border/58 bg-white/42 sm:h-[560px] sm:max-h-none dark:bg-white/5">
          <Table>
            <TableHeader className="sticky top-0 z-10 bg-card/92 shadow-[0_1px_0_0_var(--border)] backdrop-blur-xl">
              <TableRow className="hover:bg-transparent">
                <TableHead className="h-9 text-xs">名称</TableHead>
                <TableHead className="h-9 text-xs">数量</TableHead>
                <TableHead className="h-9 text-xs">预留</TableHead>
                <TableHead className="h-9 text-xs">可用</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleItems.map((item) => (
                <TableRow key={item.itemId} className="h-10 hover:bg-muted/35">
                  <TableCell className="min-w-0">
                    <div className="flex min-w-0 items-baseline gap-2">
                      <span className="truncate font-medium">{item.itemName || itemName(item.itemId)}</span>
                      <span className="shrink-0 text-xs text-muted-foreground">{item.itemId}</span>
                    </div>
                  </TableCell>
                  <TableCell>{item.owned}</TableCell>
                  <TableCell className={cn(item.allocated > 0 && "text-primary")}>{item.allocated}</TableCell>
                  <TableCell className={cn(item.available < 0 && "text-destructive")}>{item.available}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </CollapsibleCard>
  );
}

export function BusinessStatisticsPanel({ statistics }: { statistics?: BusinessStatisticsView }) {
  const observed = statistics?.observed ?? false;
  const today = statistics?.today;
  const days = statistics?.days ?? [];
  const orderTotal = today
    ? today.residentNormalFinished + today.customerFinished + today.palaceFinished + today.residentSatinFinished + today.residentDecorateFinished
    : 0;

  return (
    <CollapsibleCard
      title="营业统计"
      contentClassName="space-y-3"
      actions={
        observed ? (
          <>
            {today?.dayId ? <Badge variant="secondary">{formatDayId(today.dayId)}</Badge> : null}
            {today?.updatedAtMs ? <Badge variant="outline">更新 {formatUnixTime(today.updatedAtMs)}</Badge> : null}
          </>
        ) : (
          <Badge variant="outline">未同步</Badge>
        )
      }
    >
      {!observed || !today ? (
        <EmptyState title="暂无营业统计" detail="登录后同步今日收益、收获和订单完成数" />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
            <OverviewStat icon={<Coins />} label="金币" value={formatCount(today.gold)} detail="今日获得" />
            <OverviewStat icon={<TrendingUp />} label="经验" value={formatCount(today.experience)} detail="今日获得" />
            <OverviewStat icon={<Gem />} label="元宝" value={formatCount(today.diamonds)} detail="今日获得" />
            <OverviewStat icon={<Flower2 />} label="收获鲜花" value={formatCount(today.flowerHarvestNum)} detail="今日收获" />
            <OverviewStat icon={<Sparkles />} label="花艺售出" value={formatCount(today.flowerArtSold)} />
            <OverviewStat icon={<ListChecks />} label="完成订单" value={formatCount(orderTotal)} detail="居民/顾客/宫廷/绸缎/建材" />
            <OverviewStat icon={<Ticket />} label="加速券" value={formatCount(today.speedUpCard)} />
            <OverviewStat icon={<HandCoins />} label="花币" value={formatCount(today.flowerShopCoin)} />
          </div>

          <div className="dark-scrollbar flex gap-2 overflow-x-auto rounded-md border border-border/70 bg-muted/20 p-2">
            {businessOrderItems(today).map((item) => (
              <div key={item.label} className="flex min-w-[5.5rem] shrink-0 items-center justify-between gap-3 rounded bg-background/70 px-3 py-2 text-sm sm:min-w-24">
                <span className="text-muted-foreground">{item.label}</span>
                <span className="font-semibold tabular-nums">{formatCount(item.value)}</span>
              </div>
            ))}
          </div>

          {days.length > 0 && (
            <div className="dark-scrollbar max-h-[440px] overflow-auto rounded-md border border-border/58 bg-white/42 sm:h-[560px] sm:max-h-none dark:bg-white/5">
              <Table>
                <TableHeader className="sticky top-0 z-10 bg-card/92 shadow-[0_1px_0_0_var(--border)] backdrop-blur-xl">
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="h-9 text-xs">日期</TableHead>
                    <TableHead className="h-9 text-xs">金币</TableHead>
                    <TableHead className="h-9 text-xs">经验</TableHead>
                    <TableHead className="h-9 text-xs">元宝</TableHead>
                    <TableHead className="h-9 text-xs">收获</TableHead>
                    <TableHead className="h-9 text-xs">花艺</TableHead>
                    <TableHead className="h-9 text-xs">居民</TableHead>
                    <TableHead className="h-9 text-xs">顾客</TableHead>
                    <TableHead className="h-9 text-xs">宫廷</TableHead>
                    <TableHead className="h-9 text-xs">绸缎</TableHead>
                    <TableHead className="h-9 text-xs">建材</TableHead>
                    <TableHead className="h-9 text-xs">加速券</TableHead>
                    <TableHead className="h-9 text-xs">花币</TableHead>
                    <TableHead className="h-9 text-xs">绸缎库存</TableHead>
                    <TableHead className="h-9 text-xs">木材</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {days.map((day) => (
                    <TableRow key={day.dayId} className="h-10 hover:bg-muted/35">
                      <TableCell className="font-medium tabular-nums">{formatDayId(day.dayId)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.gold)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.experience)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.diamonds)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.flowerHarvestNum)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.flowerArtSold)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.residentNormalFinished)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.customerFinished)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.palaceFinished)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.residentSatinFinished)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.residentDecorateFinished)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.speedUpCard)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.flowerShopCoin)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.satin)}</TableCell>
                      <TableCell className="tabular-nums">{formatCount(day.wood)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </>
      )}
    </CollapsibleCard>
  );
}

export function OperationPanel({ operations }: { operations: PlannedOperation[] }) {
  const queueOperations = operations.filter(isQueueOperation);
  const farmOperations = queueOperations.filter((operation) => operation.lane === ExecutionLane.FARM);
  const sideOperations = queueOperations.filter((operation) => operation.lane !== ExecutionLane.FARM);
  return (
    <CollapsibleCard title="执行队列" actions={<Badge variant="secondary">{queueOperations.length}</Badge>}>
      <div className="max-h-[360px] overflow-hidden rounded-md border border-border/58 bg-white/34 md:h-[220px] md:max-h-none dark:bg-white/5">
        {queueOperations.length === 0 ? (
          <div className="flex min-h-28 items-center justify-center px-3 text-sm text-muted-foreground md:h-full md:min-h-0">当前无可执行操作</div>
        ) : (
          <div className="grid min-h-0 md:h-full md:grid-cols-2">
            <OperationLaneSection title="种植通道" operations={farmOperations} emptyText="暂无收获、播种或浇水" />
            <OperationLaneSection title="其他通道" operations={sideOperations} emptyText="暂无任务、订单或活动操作" />
          </div>
        )}
      </div>
    </CollapsibleCard>
  );
}

export function OperationLaneSection({ title, operations, emptyText }: { title: string; operations: PlannedOperation[]; emptyText: string }) {
  return (
    <section className="flex min-h-0 min-w-0 flex-col border-b border-border/58 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
      <div className="flex h-8 items-center justify-between bg-secondary/55 px-3 text-xs font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{operations.length}</Badge>
      </div>
      {operations.length === 0 ? (
        <div className="flex min-h-14 flex-1 items-center px-3 py-3 text-sm text-muted-foreground md:min-h-0">{emptyText}</div>
      ) : (
        <div className="dark-scrollbar min-h-0 flex-1 divide-y divide-border/70 overflow-auto">
          {operations.map((operation, index) => (
            <OperationRow key={operation.operationId || `${operation.rpc}-${index}`} operation={operation} />
          ))}
        </div>
      )}
    </section>
  );
}

export function OperationRow({ operation }: { operation: PlannedOperation }) {
  const target = operationTargetLabel(operation);
  const cost = operationCostLabel(operation);
  const note = operationNoteLabel(operation);
  return (
    <div className="flex min-h-12 items-center gap-3 px-3 py-2" title={[operation.rpc, operation.domain, operation.reason].filter(Boolean).join(" · ")}>
      <div className="shrink-0">
        <OperationStatusBadge operation={operation} />
      </div>
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1 text-sm">
        <span className="font-medium">{operationTitle(operation)}</span>
        {target && <span className="text-muted-foreground">{target}</span>}
        {cost && <span className="text-muted-foreground">{cost}</span>}
        {note && <span className="text-muted-foreground">{note}</span>}
      </div>
    </div>
  );
}

export function isQueueOperation(operation: PlannedOperation) {
  return isRunnableOperation(operation) || isOperationCooling(operation);
}

export function isRunnableOperation(operation: PlannedOperation) {
  return (
    operation.executable &&
    !operation.syncOnly &&
    operation.status !== PlanStatus.ADAPTER_MISSING &&
    operation.status !== PlanStatus.BLOCKED &&
    operation.blockedReasons.length === 0
  );
}




export function EventPanel({ events }: { events: Event[] }) {
  const [activeCategory, setActiveCategory] = useState("all");
  const displayEvents = useMemo(() => collapseRaceSyncLogEvents(events), [events]);
  const categoryCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const event of displayEvents) {
      const category = eventCategory(event);
      counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    return counts;
  }, [displayEvents]);
  const categories = useMemo(() => {
    const order = ["basic", "water", "plant", "order", "union", "race", "activity", "account", "system"];
    const keys = new Set(categoryCounts.keys());
    return [...keys].sort((a, b) => {
      const ai = order.indexOf(a);
      const bi = order.indexOf(b);
      if (ai >= 0 && bi >= 0) return ai - bi;
      if (ai >= 0) return -1;
      if (bi >= 0) return 1;
      return a.localeCompare(b);
    });
  }, [categoryCounts]);
  const visibleEvents = useMemo(() => {
    if (activeCategory === "all") return displayEvents;
    return displayEvents.filter((event) => eventCategory(event) === activeCategory);
  }, [activeCategory, displayEvents]);

  useEffect(() => {
    if (activeCategory !== "all" && !categories.includes(activeCategory)) {
      setActiveCategory("all");
    }
  }, [activeCategory, categories]);

  return (
    <Card className="cloud-surface min-h-0 flex-1">
      <CardHeader className="shrink-0">
        <CardTitle>日志</CardTitle>
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col gap-3">
        <div className="dark-scrollbar flex shrink-0 gap-1 overflow-x-auto rounded-md border border-border/58 bg-white/42 p-1 dark:bg-white/5">
          <button
            type="button"
            className={cn(
              "flex h-8 shrink-0 items-center gap-2 rounded px-3 text-xs font-medium transition-colors",
              activeCategory === "all" ? "bg-white text-foreground shadow-sm dark:bg-muted" : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
            )}
            onClick={() => setActiveCategory("all")}
          >
            全部
          </button>
          {categories.map((category) => (
            <button
              key={category}
              type="button"
              className={cn(
                "flex h-8 shrink-0 items-center gap-2 rounded px-3 text-xs font-medium transition-colors",
                activeCategory === category ? "bg-white text-foreground shadow-sm dark:bg-muted" : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
              )}
              onClick={() => setActiveCategory(category)}
            >
              {categoryLabel(category)}
            </button>
          ))}
        </div>

        {visibleEvents.length === 0 ? (
          <div className="flex min-h-0 flex-1 items-center justify-center">
            <EmptyState title="暂无日志" />
          </div>
        ) : (
          <div className="dark-scrollbar min-h-0 flex-1 space-y-2 overflow-y-auto rounded-md border border-border/58 bg-white/34 p-2 font-mono text-xs sm:space-y-0 sm:p-0 dark:bg-white/5">
            {visibleEvents.map((event, index) => (
              <div
                key={event.id || `${event.kind}-${index}-${event.message}`}
                className="grid gap-1 rounded-md border border-border/55 bg-card/72 px-3 py-2 last:border-b-0 sm:rounded-none sm:border-x-0 sm:border-t-0 sm:bg-transparent sm:grid-cols-[108px_64px_minmax(0,1fr)] sm:gap-3"
              >
                <span className="text-muted-foreground">{formatTimestamp(event.ts)}</span>
                <span
                  className={cn(
                    "font-sans text-xs font-medium",
                    event.level === "error" ? "text-destructive" : event.level === "warn" ? "text-amber-600 dark:text-amber-300" : "text-primary",
                  )}
                >
                  {categoryLabel(eventCategory(event))}
                </span>
                <div className="min-w-0 whitespace-pre-wrap break-words text-foreground">
                  <span className="font-semibold">{eventTitle(event)}</span>
                  {eventMessage(event) && <span className="text-muted-foreground"> - {eventMessage(event)}</span>}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function OverviewStat({
  icon,
  label,
  value,
  detail,
  wrap = false,
  compact = false,
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  detail?: ReactNode;
  wrap?: boolean;
  compact?: boolean;
}) {
  return (
    <div className="flex min-h-[72px] min-w-0 items-center gap-2 rounded-md border border-border/55 bg-white/52 px-2.5 py-2 shadow-sm transition-colors hover:bg-white/68 dark:bg-white/6 dark:hover:bg-white/9 sm:min-h-[76px] sm:gap-3 sm:px-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-sky-600 shadow-sm dark:bg-white/8 dark:text-sky-300 sm:size-9 [&_svg]:size-4">{icon}</div>
      <div className="min-w-0 flex-1">
        <div className="text-xs text-muted-foreground">{label}</div>
        <div
          className={cn(
            "font-semibold tabular-nums",
            compact ? "text-sm sm:text-base" : "text-base sm:text-lg",
            wrap ? "whitespace-normal break-all" : "truncate",
          )}
        >
          {value}
        </div>
        {detail && (
          <div className={cn("text-xs text-muted-foreground", wrap ? "whitespace-normal break-all" : "truncate")}>{detail}</div>
        )}
      </div>
    </div>
  );
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <Label>{label}</Label>
      {children}
    </div>
  );
}


export function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="rounded-md border border-dashed border-border/70 bg-white/32 px-3 py-4 text-center dark:bg-white/5">
      <Sparkles className="mx-auto mb-2 size-4 text-amber-400" />
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}
