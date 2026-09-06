"use client";

import { useEffect, useRef, type ComponentProps } from "react";
import {
  AlertTriangle,
  ArrowLeft,
  Cloud,
  Loader2,
  LogOut,
  KeyRound,
  Play,
  RefreshCw,
  Send,
  Trash2,
} from "lucide-react";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import type { Policy } from "@/gen/mygardenworld/v1/policy_pb";
import type { AccountRedeemAttemptFilter } from "@/gen/mygardenworld/v1/workspace_pb";
import type { AccountStatus, Event, FeatureCapability } from "@/lib/api/workspace-models";
import { accountConnected, accountIdentity, accountStatusIssues, HealthBadge } from "@/components/dashboard/dashboard-utils";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ContentReveal } from "@/components/effects/content-reveal";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { AccountViews } from "@/features/workspace/model";
import type { RedeemAttemptFeed } from "@/features/workspace/basic/redeem-attempts-model";
import { DashboardTabBar, type DashboardTabId } from "@/features/account-workspace/dashboard-tab-bar";
import {
  ActivitiesWorkspace,
  BasicWorkspace,
  GardenWorkspace,
  StatisticsWorkspace,
  LogsWorkspace,
  OrdersWorkspace,
  UnionWorkspace,
  WarehouseWorkspace,
} from "@/features/workspace";

export type { DashboardTabId } from "@/features/account-workspace/dashboard-tab-bar";

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
  redeemFeed,
  events,
  logsHasMore,
  logsLoading,
  policy,
  policyLoading,
  savingPolicy,
  policyMessage,
  onBack,
  onTabChange,
  onRefresh,
  onAction,
  onDelete,
  onReauthenticate,
  onPolicyChange,
  onPolicySave,
  onLoadMoreLogs,
  onRedeemFilterChange,
  onLoadMoreRedeemAttempts,
}: {
  account: Account;
  status?: AccountStatus;
  featureCapabilities: FeatureCapability[];
  views: AccountViews;
  viewsLoading: boolean;
  busyAction: string;
  activeTab: DashboardTabId;
  redeemFeed: RedeemAttemptFeed;
  events: Event[];
  logsHasMore: boolean;
  logsLoading: boolean;
  policy: Policy | null;
  policyLoading: boolean;
  savingPolicy: boolean;
  policyMessage: string;
  onBack: () => void;
  onTabChange: (tab: DashboardTabId) => void;
  onRefresh: () => void;
  onAction: (action: "login" | "logout") => Promise<void>;
  onDelete: () => void;
  onReauthenticate: () => void;
  onPolicyChange: (policy: Policy | null) => void;
  onPolicySave: () => void;
  onLoadMoreLogs: () => void;
  onRedeemFilterChange: (filter: AccountRedeemAttemptFilter) => void;
  onLoadMoreRedeemAttempts: () => void;
}) {
  const contentRef = useRef<HTMLDivElement>(null);
  const workspaceProps = {
    views,
    status,
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
          onReauthenticate={onReauthenticate}
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
        {activeTab === "logs" ? (
          <LogsWorkspace events={events} hasMore={logsHasMore} loading={logsLoading} onLoadMore={onLoadMoreLogs} />
        ) : (
          <ContentReveal key={`${account.id.toString()}-${activeTab}`}>
            {activeTab === "basic" && (
              <BasicWorkspace
                {...workspaceProps}
                redeemFeed={redeemFeed}
                onRedeemFilterChange={onRedeemFilterChange}
                onLoadMoreRedeemAttempts={onLoadMoreRedeemAttempts}
              />
            )}
            {activeTab === "garden" && <GardenWorkspace {...workspaceProps} />}
            {activeTab === "orders" && <OrdersWorkspace {...workspaceProps} />}
            {activeTab === "union" && <UnionWorkspace {...workspaceProps} />}
            {activeTab === "activities" && <ActivitiesWorkspace {...workspaceProps} />}
            {activeTab === "warehouse" && <WarehouseWorkspace {...workspaceProps} />}
            {activeTab === "statistics" && <StatisticsWorkspace views={views} status={status} />}
          </ContentReveal>
        )}
      </div>
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
  onReauthenticate,
}: {
  account: Account;
  status?: AccountStatus;
  viewsLoading: boolean;
  busyAction: string;
  onBack: () => void;
  onRefresh: () => void;
  onAction: (action: "login" | "logout") => Promise<void>;
  onDelete: () => void;
  onReauthenticate: () => void;
}) {
  const connected = accountConnected(account, status);
  const sessionAction = connected ? "logout" : "login";
  const identity = accountIdentity(account, status);
  const statusIssues = accountStatusIssues(status);
  return (
    <Card className="cloud-surface bg-card/88 py-3 sm:py-4">
      <CardContent className="space-y-2 px-3 sm:space-y-3 sm:px-4">
        <div className="flex min-w-0 items-center justify-between gap-2 sm:gap-3">
          <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
            <Button type="button" variant="ghost" size="icon" className="shrink-0 xl:hidden" onClick={onBack} aria-label="返回账号列表">
              <ArrowLeft className="size-4" />
            </Button>
            <div className="hidden size-12 shrink-0 items-center justify-center rounded-full bg-white/72 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300 sm:flex">
              <Cloud className="size-6" />
            </div>
            <div className="min-w-0">
              <div className="flex min-w-0 items-center gap-1.5 sm:gap-2">
                <h1 className="min-w-0 truncate text-lg font-semibold leading-tight sm:text-xl">{identity.nickname}</h1>
                <HealthBadge account={account} status={status} />
              </div>
              <div className="mt-0.5 flex min-w-0 items-center gap-1 text-xs text-muted-foreground sm:text-sm">
                <span className="truncate">{identity.area}</span><span>·</span><span className="truncate">{identity.channel}</span>
              </div>
            </div>
          </div>
          <div className="flex shrink-0 items-center justify-end gap-1 sm:gap-1.5">
            <IconButtonWithTooltip label="刷新" type="button" variant="outline" size="icon-lg" className="size-8 sm:size-9" onClick={onRefresh} disabled={viewsLoading || !connected}>
              <RefreshCw className={cn("size-4", viewsLoading && "animate-spin")} />
            </IconButtonWithTooltip>
            <IconButtonWithTooltip
              label={connected ? "退出登录" : "登录"}
              type="button"
              variant="outline"
              size="icon-lg"
              className="size-8 sm:size-9"
              onClick={() => void onAction(sessionAction)}
              disabled={busyAction === sessionAction}
            >
              {busyAction === sessionAction ? <Loader2 className="size-4 animate-spin" /> : connected ? <LogOut className="size-4" /> : <Play className="size-4" />}
            </IconButtonWithTooltip>
            <IconButtonWithTooltip label="重新登录／更新凭据" type="button" variant="outline" size="icon-lg" className="size-8 sm:size-9" onClick={onReauthenticate} disabled={!!busyAction}>
              <KeyRound className="size-4" />
            </IconButtonWithTooltip>
            <IconButtonWithTooltip label="删除账号" type="button" variant="destructive" size="icon-lg" className="size-8 sm:size-9" onClick={onDelete} disabled={busyAction === "delete"}>
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
