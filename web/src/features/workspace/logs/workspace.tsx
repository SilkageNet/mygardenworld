"use client";

import { useEffect, useMemo, useState } from "react";
import type { Event } from "@/lib/api/workspace-models";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  categoryLabel,
  collapseRaceSyncLogEvents,
  eventCategory,
  eventMessage,
  eventTitle,
  formatTimestamp,
} from "@/components/dashboard/dashboard-utils";
import { EmptyState } from "@/features/workspace/shared/workspace-ui";
import { cn } from "@/lib/utils";

export default function LogsWorkspace({ events }: { events: Event[] }) {
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
    return [...categoryCounts.keys()].sort((left, right) => {
      const leftIndex = order.indexOf(left);
      const rightIndex = order.indexOf(right);
      if (leftIndex >= 0 && rightIndex >= 0) return leftIndex - rightIndex;
      if (leftIndex >= 0) return -1;
      if (rightIndex >= 0) return 1;
      return left.localeCompare(right);
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
                className="grid gap-1 rounded-md border border-border/55 bg-card/72 px-3 py-2 last:border-b-0 sm:grid-cols-[108px_64px_minmax(0,1fr)] sm:gap-3 sm:rounded-none sm:border-x-0 sm:border-t-0 sm:bg-transparent"
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
