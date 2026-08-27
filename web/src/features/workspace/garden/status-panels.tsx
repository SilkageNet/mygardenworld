import { Fragment, useMemo } from "react";
import type { LandView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import {
  formatCount,
  landDisplayName,
  landDisplayNumber,
  landStatusLabel,
  landTimingLabel,
  recommendationLabel,
} from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState } from "@/features/workspace/shared/workspace-ui";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

export function LandMonitorPanel({ lands, waterDrops, waterDropsTotal, minWaterDrops }: {
  lands: LandView[];
  waterDrops: number;
  waterDropsTotal: number;
  minWaterDrops: number;
}) {
  const landsByDisplay = useMemo(() => {
    const map = new Map<number, LandView>();
    for (const land of lands) map.set(landDisplayNumber(land.landId), land);
    return map;
  }, [lands]);
  const mapSlots = useMemo(() => {
    // 8×8 map order: left 1-32 by rows of 4, right 33-64 by rows of 4.
    const slots: number[] = [];
    for (let row = 0; row < 8; row++) {
      for (let index = 0; index < 4; index++) slots.push(row * 4 + 1 + index);
      for (let index = 0; index < 4; index++) slots.push(33 + row * 4 + index);
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
      actions={(
        <>
          <Badge variant="secondary">已开 {openedCount}</Badge>
          {unopenedCount > 0 && <Badge variant="outline">未开 {unopenedCount}</Badge>}
          {lockedCount > 0 && <Badge variant="outline">锁定 {lockedCount}</Badge>}
        </>
      )}
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
                  {count > 0 && <Badge variant="outline">{recommendationLabel(key)} {count}</Badge>}
                  {key === "plant" && <Badge variant="outline">水滴总数 {formatCount(waterDrops)}/{formatCount(waterDropsTotal)}</Badge>}
                </Fragment>
              );
            })}
            {minWaterDrops > 0 && <Badge variant="outline">可用水滴数 {formatCount(availableWaterDrops)}</Badge>}
          </div>
          <div className="dark-scrollbar max-h-[440px] overflow-y-auto pr-0.5 sm:h-[560px] sm:max-h-none sm:pr-1">
            <div className="grid gap-2" style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr)) 0.75rem repeat(4, minmax(0, 1fr))" }}>
              {mapSlots.flatMap((display, index) => {
                const land = landsByDisplay.get(display);
                const tile = land ? (
                  <LandTile key={land.landId} land={land} />
                ) : (
                  <div key={`slot-${display}`} className="flex min-h-[78px] items-center justify-center rounded-md border border-dashed border-border/45 text-xs text-muted-foreground">#{display}</div>
                );
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

function LandTile({ land }: { land: LandView }) {
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
        <div className="min-w-0"><div className="font-mono text-xs font-medium sm:text-sm">{landDisplayName(land.landId)}</div></div>
        <Badge variant={opened && land.recommendation === "harvest" ? "secondary" : "outline"} className="h-5 shrink-0 px-1 text-[10px] sm:px-1.5 sm:text-[11px]">
          {opened ? recommendation : landStatusLabel(status)}
        </Badge>
      </div>
      <div className="mt-1 truncate text-xs sm:text-sm">{opened ? (planted ? itemName(land.flowerId) : "空地") : landStatusLabel(status)}</div>
      <div className="mt-1 text-[10px] text-muted-foreground sm:text-xs">
        <div className="truncate">
          {opened ? <>{land.lvl ? `${land.lvl}级` : "-"}{planted ? ` · 收${land.harvestCnt || 0}` : ""}</> : land.openLevel > 0 ? `${land.openLevel}级解锁` : "-"}
        </div>
        <div className="text-left">{timing}</div>
      </div>
    </div>
  );
}
