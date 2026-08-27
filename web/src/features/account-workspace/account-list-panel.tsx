import { Cloud, Loader2, Pause, Play, Plus, RefreshCw, Square, Ticket } from "lucide-react";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import type { AccountStatus } from "@/lib/api/query-models";
import {
  accountConnected,
  accountIdentity,
  accountIsAbnormal,
  HealthBadge,
} from "@/components/dashboard/dashboard-utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export type AccountQuota = {
  current: number;
  max: number;
  reached: boolean;
};

export default function AccountListPanel({
  accounts,
  statuses,
  selectedAccountId,
  loading,
  quota,
  busyAutomationAccountId,
  busyBulkAutomation,
  onRefresh,
  onAdd,
  onRedeem,
  onSelect,
  onAutomationToggle,
  onAutomationStop,
  onBulkStart,
  onBulkPause,
}: {
  accounts: Account[];
  statuses: Map<string, AccountStatus>;
  selectedAccountId: string;
  loading: boolean;
  quota: AccountQuota | null;
  busyAutomationAccountId: string;
  busyBulkAutomation: "" | "start" | "pause";
  onRefresh: () => void;
  onAdd: () => void;
  onRedeem: () => void;
  onSelect: (accountId: string) => void;
  onAutomationToggle: (accountId: string) => void;
  onAutomationStop: (accountId: string) => void;
  onBulkStart: () => void;
  onBulkPause: () => void;
}) {
  const hasAccounts = accounts.length > 0;
  const quotaReached = quota?.reached ?? false;
  const bulkBusy = busyBulkAutomation !== "";
  const automationLocked = bulkBusy || busyAutomationAccountId !== "";
  return (
    <Card className={cn("cloud-surface min-h-[340px]", hasAccounts ? "xl:h-full xl:min-h-[480px]" : "xl:min-h-[360px]")}>
      <CardHeader className="border-b border-border/45 pb-2.5 sm:pb-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="flex size-8 items-center justify-center rounded-md bg-white/72 text-sky-500 shadow-sm dark:bg-white/8 dark:text-sky-300">
              <Cloud className="size-4" />
            </span>
            <CardTitle>账号</CardTitle>
            {quota ? <Badge variant={quotaReached ? "destructive" : "secondary"}>{quota.current}/{quota.max}</Badge> : hasAccounts && <Badge variant="secondary">{accounts.length}</Badge>}
          </div>
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon-sm" onClick={onRefresh} aria-label="刷新" disabled={loading || bulkBusy}>
              <RefreshCw className={cn("size-4", loading && "animate-spin")} />
            </Button>
            {hasAccounts && <Button type="button" variant="ghost" size="icon-sm" onClick={onRedeem} aria-label="兑换码" disabled={bulkBusy}><Ticket className="size-4" /></Button>}
            {hasAccounts && (
              <Button type="button" variant="outline" size="icon-sm" onClick={onAdd} aria-label="新增账号" disabled={quotaReached || bulkBusy}>
                <Plus className="size-4" />
              </Button>
            )}
          </div>
        </div>
        {hasAccounts && (
          <div className="mt-2.5 flex items-center gap-2">
            <Button type="button" size="sm" className="h-7 flex-1 px-2" aria-label="一键启动全部账号" disabled={automationLocked} onClick={onBulkStart}>
              {busyBulkAutomation === "start" ? <Loader2 className="size-3.5 animate-spin" /> : <Play className="size-3.5" />}
              一键启动
            </Button>
            <Button type="button" variant="secondary" size="sm" className="h-7 flex-1 px-2" aria-label="一键暂停全部账号" disabled={automationLocked} onClick={onBulkPause}>
              {busyBulkAutomation === "pause" ? <Loader2 className="size-3.5 animate-spin" /> : <Pause className="size-3.5" />}
              一键暂停
            </Button>
          </div>
        )}
      </CardHeader>
      <CardContent className="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden">
        {!hasAccounts ? (
          <div className="flex min-h-[220px] flex-1 flex-col items-center justify-center px-4 py-8 text-center">
            <div className="mb-4 flex size-14 items-center justify-center rounded-full bg-white/78 text-sky-500 shadow-[0_12px_28px_rgba(46,137,199,0.16)] dark:bg-white/8 dark:text-sky-300"><Cloud className="size-6" /></div>
            <div className="text-base font-semibold">还没有账号</div>
            <div className="mt-1 text-sm text-muted-foreground">添加后开始监控。</div>
            <Button type="button" className="mt-5 w-full max-w-xs" onClick={onAdd} disabled={quotaReached}><Plus className="size-4" />添加账号</Button>
          </div>
        ) : (
          <div className="dark-scrollbar flex-1 space-y-2 overflow-y-auto pr-0.5 sm:pr-1">
            {accounts.map((account) => {
              const accountId = account.id.toString();
              const status = statuses.get(accountId);
              const selected = accountId === selectedAccountId;
              const identity = accountIdentity(account, status);
              const online = accountConnected(account, status);
              const abnormal = accountIsAbnormal(status);
              const automationBusy = bulkBusy || busyAutomationAccountId === accountId;
              const automationSpinning = busyAutomationAccountId === accountId;
              return (
                <div
                  key={accountId}
                  role="button"
                  tabIndex={0}
                  className={cn(
                    "w-full cursor-pointer rounded-md border p-3 text-left shadow-sm transition-all active:scale-[0.99]",
                    selected
                      ? "border-primary/45 bg-white/78 shadow-[0_10px_20px_rgba(255,111,97,0.12)] dark:bg-primary/12 dark:shadow-black/20"
                      : "border-border/58 bg-white/42 hover:border-ring/45 hover:bg-white/66 dark:bg-white/5 dark:hover:bg-white/8",
                  )}
                  onClick={() => onSelect(accountId)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onSelect(accountId);
                    }
                  }}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{identity.nickname}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground"><span>{identity.area}</span><span>{identity.channel}</span></div>
                    </div>
                    <div className="flex shrink-0 items-center gap-1.5">
                      <Button
                        type="button"
                        variant={online ? "secondary" : "default"}
                        size="sm"
                        className="h-7 px-2"
                        aria-label={online ? "暂停并离线" : "启动并上线"}
                        disabled={automationBusy}
                        onClick={(event) => { event.stopPropagation(); onAutomationToggle(accountId); }}
                      >
                        {automationSpinning ? <Loader2 className="size-3.5 animate-spin" /> : online ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
                        {online ? "暂停" : "启动"}
                      </Button>
                      {abnormal && (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="h-7 px-2"
                          aria-label="停止并离线"
                          disabled={automationBusy}
                          onClick={(event) => { event.stopPropagation(); onAutomationStop(accountId); }}
                        >
                          {automationSpinning ? <Loader2 className="size-3.5 animate-spin" /> : <Square className="size-3.5" />}
                          停止
                        </Button>
                      )}
                      <HealthBadge status={status} account={account} />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
