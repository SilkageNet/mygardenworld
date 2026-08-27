"use client";

import type { ReactNode } from "react";
import {
  Activity,
  BarChart3,
  Building2,
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
  return (
    <nav
      aria-label="账号功能导航"
      className="sticky top-[3.25rem] z-20 shrink-0 rounded-md border border-white/58 bg-white/62 p-1 shadow-sm shadow-sky-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-card/72 sm:top-14 xl:static"
    >
      <div role="tablist" aria-label="账号工作区" className="grid min-w-0 grid-cols-4 gap-1 md:grid-cols-8">
        {DASHBOARD_TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            className={cn(
              "flex h-9 min-w-0 items-center justify-center gap-1.5 rounded px-1.5 text-[13px] font-semibold transition-all active:scale-[0.99] sm:h-10 sm:gap-2 sm:px-2 sm:text-sm",
              activeTab === tab.id
                ? "bg-primary text-primary-foreground shadow-[0_8px_18px_rgba(255,111,97,0.24)]"
                : "text-muted-foreground hover:bg-white/62 hover:text-foreground dark:hover:bg-white/8",
            )}
            onClick={() => onChange(tab.id)}
          >
            <span className="[&_svg]:size-3.5 sm:[&_svg]:size-4">{tab.icon}</span>
            {tab.label}
          </button>
        ))}
      </div>
    </nav>
  );
}
