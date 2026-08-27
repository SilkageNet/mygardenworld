"use client";

import { useMemo, useState } from "react";
import { Loader2 } from "lucide-react";
import { WorkspaceLogCategory } from "@/gen/mygardenworld/v1/workspace_common_pb";
import type { Event } from "@/lib/api/workspace-models";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { collapseRaceSyncLogEvents, eventMessage, eventTitle, formatTimestamp } from "@/components/dashboard/dashboard-utils";
import { EmptyState } from "@/features/workspace/shared/workspace-ui";
import { cn } from "@/lib/utils";

type LogView = "key" | "warning" | "state" | "all";

const CATEGORY_TABS: Array<{ value: WorkspaceLogCategory; label: string }> = [
  { value: WorkspaceLogCategory.ACCOUNT, label: "账号" },
  { value: WorkspaceLogCategory.BASIC, label: "基础" },
  { value: WorkspaceLogCategory.GARDEN, label: "花园" },
  { value: WorkspaceLogCategory.ORDERS, label: "订单" },
  { value: WorkspaceLogCategory.UNION, label: "公会" },
  { value: WorkspaceLogCategory.ACTIVITIES, label: "活动" },
  { value: WorkspaceLogCategory.WAREHOUSE, label: "仓库" },
  { value: WorkspaceLogCategory.SYSTEM, label: "系统" },
];

const VIEW_TABS: Array<{ value: LogView; label: string }> = [
  { value: "key", label: "关键操作" },
  { value: "warning", label: "警告与错误" },
  { value: "state", label: "状态变化" },
  { value: "all", label: "全部明细" },
];

const STATE_EVENT_KINDS = new Set(["land_changed", "resource_changed", "inventory_changed", "race_task_sync"]);

export default function LogsWorkspace({ events, hasMore, loading, onLoadMore }: {
  events: Event[];
  hasMore: boolean;
  loading: boolean;
  onLoadMore: () => void;
}) {
  const [activeCategory, setActiveCategory] = useState<WorkspaceLogCategory | "all">("all");
  const [activeView, setActiveView] = useState<LogView>("key");
  const displayEvents = useMemo(() => collapseRaceSyncLogEvents(events), [events]);
  const categoryCounts = useMemo(() => {
    const counts = new Map<WorkspaceLogCategory, number>();
    for (const event of displayEvents) counts.set(event.category, (counts.get(event.category) ?? 0) + 1);
    return counts;
  }, [displayEvents]);
  const visibleEvents = useMemo(() => displayEvents.filter((event) => {
    if (activeCategory !== "all" && event.category !== activeCategory) return false;
    return eventMatchesView(event, activeView);
  }), [activeCategory, activeView, displayEvents]);

  return (
    <Card className="cloud-surface min-h-0 flex-1">
      <CardHeader className="shrink-0 gap-3">
        <div className="flex items-center justify-between gap-3">
          <CardTitle>日志</CardTitle>
          <span className="text-xs text-muted-foreground">内存窗口 {events.length} 条</span>
        </div>
        <div className="dark-scrollbar grid grid-cols-3 gap-1 rounded-md border border-border/58 bg-white/42 p-1 dark:bg-white/5 sm:flex sm:overflow-x-auto">
          <CategoryButton active={activeCategory === "all"} label="全部" count={displayEvents.length} onClick={() => setActiveCategory("all")} />
          {CATEGORY_TABS.map((category) => (
            <CategoryButton key={category.value} active={activeCategory === category.value} label={category.label} count={categoryCounts.get(category.value) ?? 0} onClick={() => setActiveCategory(category.value)} />
          ))}
        </div>
        <div className="dark-scrollbar grid grid-cols-2 gap-1 sm:flex sm:overflow-x-auto">
          {VIEW_TABS.map((view) => (
            <button
              key={view.value}
              type="button"
              className={cn(
                "h-8 min-w-0 rounded-md border px-2 text-xs font-medium transition-colors sm:shrink-0 sm:px-3",
                activeView === view.value
                  ? "border-primary/45 bg-primary/10 text-primary"
                  : "border-border/55 bg-white/30 text-muted-foreground hover:text-foreground dark:bg-white/5",
              )}
              onClick={() => setActiveView(view.value)}
            >
              {view.label}
            </button>
          ))}
        </div>
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col gap-3">
        {visibleEvents.length === 0 ? (
          <div className="flex min-h-0 flex-1 items-center justify-center">
            <EmptyState title="当前筛选下暂无日志" detail="分类始终保留；有事件后会自动出现在这里。" />
          </div>
        ) : (
          <div className="dark-scrollbar min-h-0 flex-1 space-y-2 overflow-y-auto rounded-md border border-border/58 bg-white/34 p-2 font-mono text-xs sm:space-y-0 sm:p-0 dark:bg-white/5">
            {visibleEvents.map((event, index) => (
              <div key={event.id || `${event.kind}-${index}-${event.message}`} className="grid gap-1 rounded-md border border-border/55 bg-card/72 px-3 py-2 last:border-b-0 sm:grid-cols-[108px_72px_minmax(0,1fr)] sm:gap-3 sm:rounded-none sm:border-x-0 sm:border-t-0 sm:bg-transparent">
                <span className="text-muted-foreground">{formatTimestamp(event.ts)}</span>
                <span className={cn("font-sans text-xs font-medium", event.level === "error" ? "text-destructive" : event.level === "warn" ? "text-amber-600 dark:text-amber-300" : "text-primary")}>
                  {logCategoryLabel(event.category)}
                </span>
                <div className="min-w-0 whitespace-pre-wrap break-words text-foreground">
                  <span className="font-semibold">{eventTitle(event)}</span>
                  {eventMessage(event) && <span className="text-muted-foreground"> - {eventMessage(event)}</span>}
                </div>
              </div>
            ))}
          </div>
        )}
        {hasMore && (
          <div className="flex shrink-0 justify-center">
            <Button type="button" variant="outline" onClick={onLoadMore} disabled={loading}>
              {loading && <Loader2 className="size-4 animate-spin" />}
              {loading ? "加载中" : "加载更早日志"}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function CategoryButton({ active, label, count, onClick }: { active: boolean; label: string; count: number; onClick: () => void }) {
  return (
    <button
      type="button"
      className={cn(
        "flex h-8 min-w-0 items-center justify-center gap-1.5 rounded px-1.5 text-xs font-medium transition-colors sm:shrink-0 sm:px-3",
        active ? "bg-white text-foreground shadow-sm dark:bg-muted" : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
        count === 0 && !active && "opacity-45",
      )}
      onClick={onClick}
    >
      {label}<span className="tabular-nums text-[10px] opacity-70">{count}</span>
    </button>
  );
}

function eventMatchesView(event: Event, view: LogView) {
  if (view === "all") return true;
  if (view === "warning") return event.level === "warn" || event.level === "error";
  const stateEvent = STATE_EVENT_KINDS.has(event.kind);
  if (view === "state") return stateEvent;
  return !stateEvent && event.kind !== "operation_planned";
}

function logCategoryLabel(category: WorkspaceLogCategory) {
  return CATEGORY_TABS.find((entry) => entry.value === category)?.label ?? "系统";
}
