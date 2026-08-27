import { useMemo } from "react";
import type { FmlLandView, PlantableFlowerView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { fmlLandTimingLabel, recommendationLabel } from "@/components/dashboard/dashboard-utils";
import { CollapsibleCard, EmptyState } from "@/features/workspace/shared/workspace-ui";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

export function FmlLandMonitorPanel({ lands, plantableFlowers, observed, automationEnabled }: {
  lands: FmlLandView[];
  plantableFlowers: PlantableFlowerView[];
  observed: boolean;
  automationEnabled: boolean;
}) {
  const flowerLvlById = useMemo(() => {
    const levels = new Map<number, number>();
    for (const flower of plantableFlowers) {
      if (flower.flowerId > 0 && flower.lvl > 0) levels.set(flower.flowerId, flower.lvl);
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
  const pendingTotal = useMemo(() => lands.reduce((sum, land) => sum + (land.pendingHarvest || 0), 0), [lands]);
  const plantedCount = lands.filter((land) => land.flowerId > 0).length;
  const emptyCount = lands.filter((land) => land.flowerId <= 0).length;
  const statusOrder = ["harvest", "plant", "wait"] as const;

  return (
    <CollapsibleCard
      title="公会土地"
      actions={(
        <>
          {!observed ? <Badge variant="outline">等待同步</Badge> : <Badge variant="secondary">已观测 {lands.length}</Badge>}
          {observed && plantedCount > 0 && <Badge variant="outline">种植中 {plantedCount}</Badge>}
          {observed && emptyCount > 0 && <Badge variant="outline">空地 {emptyCount}</Badge>}
          {pendingTotal > 0 && <Badge variant="secondary">可收 {pendingTotal}</Badge>}
        </>
      )}
    >
      {!observed ? (
        <EmptyState
          title="公会土地尚未同步"
          detail={automationEnabled
            ? "账号运行中时会自动执行 fml.enter 拉取公会土地；稍等数秒后刷新即可。"
            : "请先启动账号自动化，守护进程会自动进入公会并同步土地种植信息。"}
        />
      ) : lands.length === 0 ? (
        <EmptyState title="暂无公会土地" detail="当前账号还没有可观测的公会土地槽位。" />
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap gap-2">
            {statusOrder.map((key) => {
              const count = recommendationCounts.get(key) ?? 0;
              return count > 0 ? <Badge key={key} variant="outline">{recommendationLabel(key)} {count}</Badge> : null;
            })}
          </div>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
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

function FmlLandTile({ land, flowerLvl }: { land: FmlLandView; flowerLvl: number }) {
  const planted = land.flowerId > 0;
  const stockLabel = planted && land.stockCap > 0
    ? `${land.pendingHarvest}/${land.stockCap}`
    : planted ? `收${land.harvestedCount || 0}` : "";
  const flowerLabel = planted
    ? flowerLvl > 0 ? `${itemName(land.flowerId)} lv${flowerLvl}` : itemName(land.flowerId)
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
        <Badge variant={land.recommendation === "harvest" ? "secondary" : "outline"} className="h-5 shrink-0 px-1 text-[10px] sm:px-1.5 sm:text-[11px]">
          {recommendationLabel(land.recommendation)}
        </Badge>
      </div>
      <div className="mt-1 truncate text-xs sm:text-sm">{flowerLabel}</div>
      <div className="mt-1 text-[10px] text-muted-foreground sm:text-xs">
        <div className="truncate">{`地${land.level}级`}{stockLabel ? ` · ${stockLabel}` : ""}{land.pendingHarvest > 0 ? ` · 待收${land.pendingHarvest}` : ""}</div>
        <div className="text-left">{fmlLandTimingLabel(land)}</div>
      </div>
    </div>
  );
}
