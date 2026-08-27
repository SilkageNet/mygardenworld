import type { ActivityItem } from "@/lib/api/workspace-models";
import { formatCount } from "@/components/dashboard/dashboard-utils";
import { itemName } from "@/lib/game/catalog";
import { cn } from "@/lib/utils";

export default function ActivityItemChip({ item, compact = false }: { item: ActivityItem; compact?: boolean }) {
  const label = item.itemName || itemName(item.itemId);
  return (
    <span className={cn("inline-flex max-w-full items-center gap-1 rounded border border-border/58 bg-white/52 dark:bg-white/5", compact ? "px-1.5 py-0.5" : "px-2 py-1 text-xs")}>
      <span className="min-w-0 truncate" title={item.itemId > 0 ? `${label} #${item.itemId}` : label}>{label}</span>
      <span className="shrink-0 font-semibold tabular-nums">×{formatCount(item.count)}</span>
    </span>
  );
}
