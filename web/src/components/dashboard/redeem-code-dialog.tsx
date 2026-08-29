"use client";

import { useMemo, useState, type FormEvent } from "react";
import { History, Loader2, Ticket } from "lucide-react";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { accountNickname, channelLabel } from "@/components/dashboard/dashboard-utils";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { formatAPIError } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import { Field } from "@/features/workspace/shared/workspace-ui";
import RedeemCodeHistoryDialog, { type RedeemCodeItem, type RedeemHistoryItem } from "./redeem-code-history-dialog";

const REDEEM_CHANNELS = [Channel.IOS, Channel.ALIPAY] as const;

export type RedeemCodeOutcome = {
  results: Array<{
    accountId: bigint;
    accountName: string;
    ok: boolean;
    message: string;
  }>;
  successCount: number;
  failureCount: number;
};

export function availableRedeemChannels(accounts: Account[]): Channel[] {
  const present = new Set(accounts.map((account) => account.channel));
  return REDEEM_CHANNELS.filter((channel) => present.has(channel));
}

export function redeemTargets(accounts: Account[], channel: Channel): Account[] {
  return accounts.filter((account) => account.channel === channel);
}

export function initialRedeemChannel(accounts: Account[], preferredChannel?: Channel): Channel {
  const channels = availableRedeemChannels(accounts);
  if (preferredChannel !== undefined && channels.includes(preferredChannel)) return preferredChannel;
  return channels[0] ?? Channel.UNSPECIFIED;
}

export default function RedeemCodeDialog({
  accounts,
  preferredChannel,
  onOpenChange,
  onRedeem,
  autoRedeemEnabled,
  onAutoRedeemChange,
  onLoadSyncStatus,
  onLoadCodes,
  onLoadHistory,
  onForceSync,
}: {
  accounts: Account[];
  preferredChannel?: Channel;
  onOpenChange: (open: boolean) => void;
  onRedeem: (code: string, accountIds: bigint[]) => Promise<RedeemCodeOutcome>;
  autoRedeemEnabled: boolean;
  onAutoRedeemChange: (enabled: boolean) => void;
  onLoadSyncStatus: () => Promise<Date | undefined>;
  onLoadCodes: () => Promise<RedeemCodeItem[]>;
  onLoadHistory: () => Promise<RedeemHistoryItem[]>;
  onForceSync: () => Promise<Date | undefined>;
}) {
  const channels = useMemo(() => availableRedeemChannels(accounts), [accounts]);
  const [channel, setChannel] = useState(() => initialRedeemChannel(accounts, preferredChannel));
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [summary, setSummary] = useState("");
  const [results, setResults] = useState<RedeemCodeOutcome["results"]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const targets = useMemo(() => redeemTargets(accounts, channel), [accounts, channel]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const normalizedCode = code.trim();
    if (!normalizedCode || targets.length === 0 || busy) return;
    setBusy(true);
    setSummary("");
    setResults([]);
    try {
      const outcome = await onRedeem(normalizedCode, targets.map((account) => account.id));
      setResults(outcome.results);
      setSummary(`成功 ${outcome.successCount} / 失败 ${outcome.failureCount}（共 ${outcome.results.length} 个账号）`);
    } catch (err) {
      setSummary(formatAPIError(err, "兑换失败"));
    } finally {
      setBusy(false);
    }
  }

  const targetLabel = channel === Channel.UNSPECIFIED ? "所选渠道" : channelLabel(channel);

  return (
    <>
      <Dialog
        open
        onOpenChange={(open) => {
          if (!busy) onOpenChange(open);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>兑换码</DialogTitle>
          </DialogHeader>
          <form className="space-y-4" onSubmit={(event) => void submit(event)}>
            <Field label="兑换渠道">
              <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="兑换渠道">
                {channels.map((option) => {
                  const selected = option === channel;
                  const count = redeemTargets(accounts, option).length;
                  return (
                    <button
                      key={option}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      className={cn(
                        "h-10 rounded-md border px-3 text-sm font-medium transition-colors",
                        selected
                          ? "border-primary bg-primary text-primary-foreground"
                          : "border-border/70 text-muted-foreground hover:text-foreground",
                      )}
                      onClick={() => {
                        setChannel(option);
                        setSummary("");
                        setResults([]);
                      }}
                      disabled={busy}
                    >
                      {channelLabel(option)}（{count}）
                    </button>
                  );
                })}
              </div>
            </Field>
            <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-sm">
              <div>
                本次仅兑换到 {targetLabel} 的 {targets.length} 个账号，离线账号会先尝试登录。
              </div>
              {targets.length > 0 && (
                <div className="mt-1 text-xs text-muted-foreground">
                  {targets.map((account) => accountNickname(account)).join("、")}
                </div>
              )}
            </div>
            <Field label="兑换码">
              <Input
                value={code}
                onChange={(event) => setCode(event.target.value)}
                placeholder="粘贴兑换码"
                autoComplete="off"
                disabled={busy}
              />
            </Field>
            {summary && <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-sm">{summary}</div>}
            {results.length > 0 && (
              <div className="dark-scrollbar max-h-48 space-y-1.5 overflow-y-auto rounded-md border border-border/50 p-2 text-sm">
                {results.map((item) => (
                  <div key={item.accountId.toString()} className="flex items-start justify-between gap-2">
                    <span className="min-w-0 truncate font-medium">{item.accountName || item.accountId.toString()}</span>
                    <span className={cn("min-w-0 text-right", item.ok ? "text-emerald-600 dark:text-emerald-400" : "text-destructive")}>
                      {item.ok ? (item.message && item.message !== "ok" ? item.message : "成功") : item.message || "失败"}
                    </span>
                  </div>
                ))}
              </div>
            )}
            <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 space-y-1">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-sm">
                  <Switch checked={autoRedeemEnabled} onCheckedChange={onAutoRedeemChange} />
                  <span>自动兑换</span>
                </div>
                <Button type="button" variant="ghost" size="sm" onClick={() => setShowHistory(true)}>
                  <History className="size-4" />
                  查看兑换码
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">19:00 - 22:30 每 10 分钟自动拉取兑换码并兑换，其余时段每小时拉取一次。</p>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={busy}>
                关闭
              </Button>
              <Button type="submit" disabled={busy || !code.trim() || targets.length === 0}>
                {busy ? <Loader2 className="size-4 animate-spin" /> : <Ticket className="size-4" />}
                {busy ? "兑换中" : `兑换到${targetLabel}（${targets.length}）`}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      {showHistory && (
        <RedeemCodeHistoryDialog
          onOpenChange={setShowHistory}
          onLoadSyncStatus={onLoadSyncStatus}
          onLoadCodes={onLoadCodes}
          onLoadHistory={onLoadHistory}
          onForceSync={onForceSync}
        />
      )}
    </>
  );
}
