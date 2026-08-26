"use client";

import { useEffect, useRef, type ComponentProps, type ReactNode } from "react";
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  Building2,
  Cloud,
  LayoutDashboard,
  ListTodo,
  Loader2,
  LogOut,
  Package,
  Play,
  RefreshCw,
  ScrollText,
  Send,
  Sprout,
  Trash2,
} from "lucide-react";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import type { AccountStatus, Event, FeatureCapability } from "@/gen/mygardenworld/v1/query_service_pb";
import { accountConnected, accountIdentity, accountStatusIssues, HealthBadge } from "@/components/dashboard/dashboard-utils";
import { EventPanel } from "@/components/dashboard/monitor-panels";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { AccountViews } from "./model";
import {
  ActivitiesWorkspace,
  AssetsWorkspace,
  GardenWorkspace,
  OrdersWorkspace,
  OverviewWorkspace,
  UnionWorkspace,
} from "./workspaces";

export type DashboardTabId = "overview" | "garden" | "orders" | "union" | "activities" | "assets" | "logs";

const DASHBOARD_TABS: { id: DashboardTabId; label: string; icon: ReactNode }[] = [
  { id: "overview", label: "概览", icon: <LayoutDashboard /> },
  { id: "garden", label: "花园", icon: <Sprout /> },
  { id: "orders", label: "订单经营", icon: <ListTodo /> },
  { id: "union", label: "公会", icon: <Building2 /> },
  { id: "activities", label: "活动", icon: <Activity /> },
  { id: "assets", label: "资产", icon: <Package /> },
  { id: "logs", label: "全部日志", icon: <ScrollText /> },
];

export function SelectAccountPlaceholder() {
  return (
    <Card className="cloud-surface flex h-full min-h-[480px] items-center justify-center">
      <CardContent className="max-w-md text-center">
        <div className="mx-auto mb-3 flex size-14 items-center justify-center rounded-full bg-white/76 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300">
          <Send className="size-5" />
        </div>
        <div className="text-base font-semibold">选择账号</div>
        <div className="mt-1 text-sm text-muted-foreground">从左侧进入账号工作区。</div>
      </CardContent>
    </Card>
  );
}

export function AccountDetailView({
  account,
  status,
  featureCapabilities,
  views,
  viewsLoading,
  busyAction,
  activeTab,
  events,
  policy,
  policyLoading,
  savingPolicy,
  policyMessage,
  onBack,
  onTabChange,
  onRefresh,
  onAction,
  onDelete,
  onPolicyChange,
  onPolicySave,
}: {
  account: Account;
  status?: AccountStatus;
  featureCapabilities: FeatureCapability[];
  views: AccountViews;
  viewsLoading: boolean;
  busyAction: string;
  activeTab: DashboardTabId;
  events: Event[];
  policy: Policy | null;
  policyLoading: boolean;
  savingPolicy: boolean;
  policyMessage: string;
  onBack: () => void;
  onTabChange: (tab: DashboardTabId) => void;
  onRefresh: () => void;
  onAction: (action: "login" | "logout") => Promise<void>;
  onDelete: () => void;
  onPolicyChange: (policy: Policy | null) => void;
  onPolicySave: () => void;
}) {
  const contentRef = useRef<HTMLDivElement>(null);
  const workspaceProps = {
    views,
    status,
    events,
    policy,
    capabilities: featureCapabilities,
    policyLoading,
    savingPolicy,
    policyMessage,
    onPolicyChange,
    onPolicySave,
  };

  useEffect(() => {
    contentRef.current?.scrollTo({ top: 0 });
    window.scrollTo({ top: 0 });
  }, [account.id]);

  return (
    <div className="flex min-h-0 w-full min-w-0 max-w-full flex-col gap-3 sm:gap-4 xl:h-full xl:overflow-hidden">
      <div className="shrink-0">
        <HeaderPanel
          account={account}
          status={status}
          viewsLoading={viewsLoading}
          busyAction={busyAction}
          onBack={onBack}
          onRefresh={onRefresh}
          onAction={onAction}
          onDelete={onDelete}
        />
      </div>
      <DashboardTabBar activeTab={activeTab} onChange={onTabChange} />
      <div
        ref={contentRef}
        className={cn(
          "min-h-0",
          activeTab === "logs"
            ? "flex flex-1 xl:min-h-0 xl:overflow-hidden"
            : "dark-scrollbar xl:flex-1 xl:overflow-y-auto xl:pr-0.5",
        )}
      >
        {activeTab === "overview" && <OverviewWorkspace {...workspaceProps} />}
        {activeTab === "garden" && <GardenWorkspace {...workspaceProps} />}
        {activeTab === "orders" && <OrdersWorkspace {...workspaceProps} />}
        {activeTab === "union" && <UnionWorkspace {...workspaceProps} />}
        {activeTab === "activities" && <ActivitiesWorkspace {...workspaceProps} />}
        {activeTab === "assets" && <AssetsWorkspace {...workspaceProps} />}
        {activeTab === "logs" && <EventPanel events={events} />}
      </div>
    </div>
  );
}

function DashboardTabBar({ activeTab, onChange }: { activeTab: DashboardTabId; onChange: (tab: DashboardTabId) => void }) {
  return (
    <div className="dark-scrollbar sticky top-[3.25rem] z-20 flex shrink-0 gap-1 overflow-x-auto rounded-md border border-white/58 bg-white/62 p-1 shadow-sm shadow-sky-900/5 backdrop-blur-xl dark:border-white/10 dark:bg-card/72 sm:top-14 xl:static">
      {DASHBOARD_TABS.map((tab) => (
        <button
          key={tab.id}
          type="button"
          className={cn(
            "flex h-9 min-w-[6.25rem] shrink-0 items-center justify-center gap-2 rounded px-3 text-sm font-semibold transition-all active:scale-[0.99] sm:min-w-20",
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
  );
}

function HeaderPanel({
  account,
  status,
  viewsLoading,
  busyAction,
  onBack,
  onRefresh,
  onAction,
  onDelete,
}: {
  account: Account;
  status?: AccountStatus;
  viewsLoading: boolean;
  busyAction: string;
  onBack: () => void;
  onRefresh: () => void;
  onAction: (action: "login" | "logout") => Promise<void>;
  onDelete: () => void;
}) {
  const connected = accountConnected(account, status);
  const sessionAction = connected ? "logout" : "login";
  const identity = accountIdentity(account, status);
  const statusIssues = accountStatusIssues(status);
  return (
    <Card className="cloud-surface bg-card/88">
      <CardContent className="space-y-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-start gap-3 sm:items-center">
            <Button type="button" variant="ghost" size="icon-sm" className="mt-0.5 shrink-0 xl:hidden" onClick={onBack} aria-label="返回账号列表">
              <ArrowLeft className="size-4" />
            </Button>
            <div className="hidden size-12 shrink-0 items-center justify-center rounded-full bg-white/72 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300 sm:flex">
              <Cloud className="size-6" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                <h1 className="min-w-0 max-w-full truncate text-xl font-semibold leading-tight sm:text-xl">{identity.nickname}</h1>
                <div className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-sm text-muted-foreground">
                  <span>{identity.area}</span><span>·</span><span>{identity.channel}</span>
                </div>
                <HealthBadge account={account} status={status} />
              </div>
            </div>
          </div>
          <div className="flex shrink-0 items-center justify-end gap-1">
            <IconButtonWithTooltip label="刷新" type="button" variant="outline" size="icon-sm" onClick={onRefresh} disabled={viewsLoading || !connected}>
              <RefreshCw className={cn("size-4", viewsLoading && "animate-spin")} />
            </IconButtonWithTooltip>
            <IconButtonWithTooltip
              label={connected ? "退出登录" : "登录"}
              type="button"
              variant="outline"
              size="icon-sm"
              onClick={() => void onAction(sessionAction)}
              disabled={busyAction === sessionAction}
            >
              {busyAction === sessionAction ? <Loader2 className="size-4 animate-spin" /> : connected ? <LogOut className="size-4" /> : <Play className="size-4" />}
            </IconButtonWithTooltip>
            <IconButtonWithTooltip label="删除账号" type="button" variant="destructive" size="icon-sm" onClick={onDelete} disabled={busyAction === "delete"}>
              <Trash2 className="size-4" />
            </IconButtonWithTooltip>
          </div>
        </div>
        {statusIssues.length > 0 && (
          <div className="rounded-md border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive shadow-sm">
            <div className="flex items-start gap-2">
              <AlertTriangle className="mt-0.5 size-4 shrink-0" />
              <div className="min-w-0 space-y-1">
                <div className="font-medium">异常信息</div>
                {statusIssues.map((issue) => <div key={issue} className="break-words text-destructive/90">{issue}</div>)}
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function IconButtonWithTooltip({ label, children, ...props }: ComponentProps<typeof Button> & { label: string }) {
  return (
    <Tooltip disabled={props.disabled}>
      <TooltipTrigger render={<Button {...props} aria-label={props["aria-label"] ?? label}>{children}</Button>} />
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}
