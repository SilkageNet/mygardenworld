"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  Activity,
  ArrowLeftRight,
  BarChart3,
  Building2,
  ChevronLeft,
  ChevronRight,
  LayoutDashboard,
  ListTodo,
  Package,
  ScrollText,
  Sprout,
} from "lucide-react";
import { cn } from "@/lib/utils";

export type DashboardTabId = "basic" | "garden" | "orders" | "union" | "activities" | "warehouse" | "statistics" | "logs";

const DASHBOARD_TABS: { id: DashboardTabId; label: string; icon: ReactNode }[] = [
  { id: "basic", label: "基础", icon: <LayoutDashboard /> },
  { id: "garden", label: "花园", icon: <Sprout /> },
  { id: "orders", label: "订单", icon: <ListTodo /> },
  { id: "union", label: "公会", icon: <Building2 /> },
  { id: "activities", label: "活动", icon: <Activity /> },
  { id: "warehouse", label: "仓库", icon: <Package /> },
  { id: "statistics", label: "统计", icon: <BarChart3 /> },
  { id: "logs", label: "日志", icon: <ScrollText /> },
];

export function DashboardTabBar({ activeTab, onChange }: { activeTab: DashboardTabId; onChange: (tab: DashboardTabId) => void }) {
  const scrollerRef = useRef<HTMLDivElement>(null);
  const [scrollEdges, setScrollEdges] = useState({ left: false, right: true });
  const activeIndex = Math.max(0, DASHBOARD_TABS.findIndex((tab) => tab.id === activeTab));

  const updateScrollEdges = useCallback(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    const maxScrollLeft = Math.max(0, scroller.scrollWidth - scroller.clientWidth);
    const next = {
      left: scroller.scrollLeft > 2,
      right: scroller.scrollLeft < maxScrollLeft - 2,
    };
    setScrollEdges((current) => {
      if (current.left === next.left && current.right === next.right) return current;
      return next;
    });
  }, []);

  useEffect(() => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    const observer = new ResizeObserver(updateScrollEdges);
    observer.observe(scroller);
    const frame = window.requestAnimationFrame(updateScrollEdges);
    return () => {
      window.cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [updateScrollEdges]);

  useEffect(() => {
    const scroller = scrollerRef.current;
    const activeButton = scroller?.querySelector<HTMLElement>(`[data-workspace-tab="${activeTab}"]`);
    if (!activeButton) return;
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    activeButton.scrollIntoView({
      behavior: reducedMotion ? "auto" : "smooth",
      block: "nearest",
      inline: "center",
    });
    const timeout = window.setTimeout(updateScrollEdges, reducedMotion ? 0 : 240);
    return () => window.clearTimeout(timeout);
  }, [activeTab, updateScrollEdges]);

  const scrollTabs = (direction: -1 | 1) => {
    const scroller = scrollerRef.current;
    if (!scroller) return;
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    scroller.scrollBy({
      left: direction * Math.max(168, scroller.clientWidth * 0.72),
      behavior: reducedMotion ? "auto" : "smooth",
    });
  };

  return (
    <nav
      aria-label="账号功能导航"
      className="sticky top-[3.25rem] z-20 shrink-0 rounded-md border border-white/58 bg-white/62 p-1 shadow-sm shadow-sky-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-card/72 sm:top-14 xl:static"
    >
      <div className="flex h-7 items-center justify-between gap-2 px-1.5 text-[11px] text-muted-foreground sm:hidden">
        <span className="font-semibold text-foreground/85">功能导航</span>
        <span className="flex items-center gap-1.5">
          <ArrowLeftRight className="size-3.5 text-ring" />
          左右滑动
          <span className="rounded-full bg-white/68 px-1.5 py-0.5 font-semibold tabular-nums text-foreground shadow-sm dark:bg-white/8">
            {activeIndex + 1}/{DASHBOARD_TABS.length}
          </span>
        </span>
      </div>
      <div className="relative min-w-0">
        <div
          ref={scrollerRef}
          role="tablist"
          aria-label="账号工作区"
          className="mobile-tab-scroller flex min-w-0 snap-x snap-proximity gap-1 overflow-x-auto overscroll-x-contain touch-pan-x"
          onScroll={updateScrollEdges}
        >
          {DASHBOARD_TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={activeTab === tab.id}
              data-workspace-tab={tab.id}
              className={cn(
                "flex h-9 min-w-[5.25rem] snap-start items-center justify-center gap-2 rounded px-2.5 text-sm font-semibold transition-all active:scale-[0.99] sm:min-w-20 sm:px-3",
                activeTab === tab.id
                  ? "bg-primary text-primary-foreground shadow-[0_8px_18px_rgba(255,111,97,0.24)]"
                  : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
              )}
              onClick={() => onChange(tab.id)}
            >
              <span className="[&_svg]:size-4">{tab.icon}</span>
              {tab.label}
            </button>
          ))}
        </div>
        {scrollEdges.left && (
          <div className="pointer-events-none absolute inset-y-0 left-0 flex w-9 items-center bg-gradient-to-r from-white/95 to-transparent pl-0.5 dark:from-card/95 sm:hidden">
            <button
              type="button"
              className="pointer-events-auto flex size-7 items-center justify-center rounded-full border border-border/65 bg-card/92 text-foreground shadow-md active:scale-95"
              aria-label="向左查看更多功能"
              onClick={() => scrollTabs(-1)}
            >
              <ChevronLeft className="size-4" />
            </button>
          </div>
        )}
        {scrollEdges.right && (
          <div className="pointer-events-none absolute inset-y-0 right-0 flex w-9 items-center justify-end bg-gradient-to-l from-white/95 to-transparent pr-0.5 dark:from-card/95 sm:hidden">
            <button
              type="button"
              className="pointer-events-auto flex size-7 items-center justify-center rounded-full border border-border/65 bg-card/92 text-foreground shadow-md active:scale-95"
              aria-label="向右查看更多功能"
              onClick={() => scrollTabs(1)}
            >
              <ChevronRight className="size-4" />
            </button>
          </div>
        )}
      </div>
    </nav>
  );
}
