import { CalendarDays, Coins, Flower2, Gem, HandCoins, ListChecks, ShoppingBag, Sparkles, Ticket, TrendingUp } from "lucide-react";
import type { BusinessStatisticsView, RuntimeActionTotal, RuntimeStatisticsView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  businessOrderItems,
  formatCount,
  formatDayId,
  formatUnixTime,
  runtimeActionSummary,
  runtimeResourceGainSummary,
  runtimeResourcePrimaryValue,
  runtimeWindowLabel,
  sumRuntimeActionTotals,
} from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState, OverviewStat } from "@/features/workspace/shared/workspace-ui";

export function RuntimeStatisticsPanel({ runtimeStatistics }: { runtimeStatistics?: RuntimeStatisticsView }) {
  const runtimeOrderCompletions = runtimeStatistics?.orderCompletions ?? [];
  const runtimeTaskCompletions = runtimeStatistics?.taskCompletions ?? [];
  const runtimeTotalOperations = runtimeStatistics?.totalOperations ?? BigInt(0);
  const runtimeOrderTotal = sumRuntimeActionTotals(runtimeOrderCompletions);
  const runtimeTaskTotal = sumRuntimeActionTotals(runtimeTaskCompletions);
  const runtimeResourceGains = runtimeStatistics?.resourceGains ?? [];
  const showCompletionGroups = runtimeOrderCompletions.length > 0 || runtimeTaskCompletions.length > 0 || runtimeTotalOperations > BigInt(0);

  return (
    <CollapsibleCard
      title="本次运行统计"
      contentClassName="space-y-3"
      actions={(
        <>
          <Badge variant={runtimeStatistics?.running ? "secondary" : "outline"}>{runtimeStatistics ? (runtimeStatistics.running ? "运行中" : "已停止") : "暂无"}</Badge>
          {runtimeTotalOperations > BigInt(0) && <Badge variant="outline">操作 {formatCount(runtimeTotalOperations)}</Badge>}
        </>
      )}
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

function RuntimeCompletionGroup({ title, items, emptyText }: { title: string; items: RuntimeActionTotal[]; emptyText: string }) {
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
      actions={observed ? (
        <>
          {today?.dayId ? <Badge variant="secondary">{formatDayId(today.dayId)}</Badge> : null}
          {today?.updatedAtMs ? <Badge variant="outline">更新 {formatUnixTime(today.updatedAtMs)}</Badge> : null}
        </>
      ) : <Badge variant="outline">未同步</Badge>}
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
                    {['日期', '金币', '经验', '元宝', '收获', '花艺', '居民', '顾客', '宫廷', '绸缎', '建材', '加速券', '花币', '绸缎库存', '木材'].map((label) => (
                      <TableHead key={label} className="h-9 text-xs">{label}</TableHead>
                    ))}
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
