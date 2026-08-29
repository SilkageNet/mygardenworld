"use client";

import { useEffect, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { formatAPIError } from "@/lib/api/client";
import { cn } from "@/lib/utils";

export type RedeemCodeItem = {
  code: string;
  fetchedAt: Date | undefined;
  sourceTime: Date | undefined;
};

export type RedeemHistoryItem = {
  accountId: bigint;
  accountName: string;
  code: string;
  status: string;
  message: string;
  createdAt: Date | undefined;
};

function formatCompactTime(d: Date | undefined): string {
  if (!d) return "-";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function statusLabel(status: string): { text: string; className: string } {
  switch (status) {
    case "redeemed":
      return { text: "兑换成功", className: "text-emerald-600 dark:text-emerald-400" };
    case "already_claimed":
      return { text: "已领取过", className: "text-amber-600 dark:text-amber-400" };
    case "expired":
      return { text: "兑换过期", className: "text-muted-foreground" };
    case "invalid":
      return { text: "兑换无效", className: "text-muted-foreground" };
    case "failed":
      return { text: "兑换失败", className: "text-destructive" };
    default:
      return { text: status, className: "text-muted-foreground" };
  }
}

// Parse account display name like "花艺师 · 第4041区" into name + zone.
function parseAccountDisplay(fullName: string): { name: string; zone: string | null } {
  const sep = fullName.indexOf("·");
  if (sep === -1) {
    // Try CJK middle dot variants
    const sep2 = fullName.indexOf("・");
    if (sep2 === -1) return { name: fullName.trim(), zone: null };
    return { name: fullName.slice(0, sep2).trim(), zone: fullName.slice(sep2 + 1).trim() || null };
  }
  return { name: fullName.slice(0, sep).trim(), zone: fullName.slice(sep + 1).trim() || null };
}

export default function RedeemCodeHistoryDialog({
  onOpenChange,
  onLoadSyncStatus,
  onLoadCodes,
  onLoadHistory,
  onForceSync,
}: {
  onOpenChange: (open: boolean) => void;
  onLoadSyncStatus: () => Promise<Date | undefined>;
  onLoadCodes: () => Promise<RedeemCodeItem[]>;
  onLoadHistory: () => Promise<RedeemHistoryItem[]>;
  onForceSync: () => Promise<Date | undefined>;
}) {
  const [codes, setCodes] = useState<RedeemCodeItem[]>([]);
  const [history, setHistory] = useState<RedeemHistoryItem[]>([]);
  const [lastSyncAt, setLastSyncAt] = useState<Date | undefined>(undefined);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<"codes" | "history">("codes");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError("");
      try {
        const [codesData, historyData, syncTime] = await Promise.all([onLoadCodes(), onLoadHistory(), onLoadSyncStatus()]);
        if (cancelled) return;
        setCodes(codesData);
        setHistory(historyData);
        setLastSyncAt(syncTime);
      } catch (err) {
        if (cancelled) return;
        setError(formatAPIError(err, "加载失败"));
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [onLoadCodes, onLoadHistory, onLoadSyncStatus]);

  async function handleRefresh() {
    if (syncing) return;
    setSyncing(true);
    try {
      const syncTime = await onForceSync();
      setLastSyncAt(syncTime ?? new Date());
      const [codesData, historyData] = await Promise.all([onLoadCodes(), onLoadHistory()]);
      setCodes(codesData);
      setHistory(historyData);
    } catch {
      // ignore — button re-enables
    } finally {
      setSyncing(false);
    }
  }

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>
            <span className="inline-flex items-center gap-2">
              自动兑换记录
              {(() => {
                if (!lastSyncAt) return null;
                const pad = (n: number) => String(n).padStart(2, "0");
                return (
                  <>
                    <Badge variant="secondary" className="text-[10px] font-normal">
                      最近拉取{pad(lastSyncAt.getMonth() + 1)}/{pad(lastSyncAt.getDate())} {pad(lastSyncAt.getHours())}:{pad(lastSyncAt.getMinutes())}
                    </Badge>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-5 shrink-0"
                      disabled={syncing}
                      aria-label="刷新"
                      onClick={() => void handleRefresh()}
                    >
                      <RefreshCw className={cn("size-3", syncing && "animate-spin")} />
                    </Button>
                  </>
                );
              })()}
            </span>
          </DialogTitle>
        </DialogHeader>
        <div className="grid grid-cols-2 border-b border-border/60">
          <button
            type="button"
            className={cn(
              "px-3 py-1.5 text-sm font-medium transition-colors border-b-2 -mb-px",
              tab === "codes" ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
            )}
            onClick={() => setTab("codes")}
          >
            拉取信息（{codes.length}）
          </button>
          <button
            type="button"
            className={cn(
              "px-3 py-1.5 text-sm font-medium transition-colors border-b-2 -mb-px",
              tab === "history" ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
            )}
            onClick={() => setTab("history")}
          >
            兑换历史（{history.length}）
          </button>
        </div>
        {loading ? (
          <div className="flex items-center justify-center py-8 text-muted-foreground">
            <Loader2 className="size-4 animate-spin mr-2" />
            加载中
          </div>
        ) : error ? (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div>
        ) : tab === "codes" ? (
          <div className="dark-scrollbar max-h-80 overflow-y-auto mt-1.5">
            {codes.length === 0 ? (
              <div className="py-8 text-center text-sm text-muted-foreground">暂无兑换码</div>
            ) : (
              <table className="w-full table-fixed text-sm text-center">
                <thead>
                  <tr className="border-b border-border/50 text-muted-foreground">
                    <th className="pb-1.5 font-medium w-1/2">兑换码</th>
                    <th className="pb-1.5 font-medium w-1/2">创建时间</th>
                  </tr>
                </thead>
                <tbody>
                  {codes.map((c) => (
                    <tr key={c.code} className="border-b border-border/30 last:border-0">
                      <td className="py-1.5 font-mono text-xs">{c.code}</td>
                      <td className="py-1.5 text-muted-foreground text-xs">{formatCompactTime(c.sourceTime ?? c.fetchedAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        ) : (
          <div className="dark-scrollbar max-h-80 overflow-y-auto mt-1.5">
            {history.length === 0 ? (
              <div className="py-8 text-center text-sm text-muted-foreground">暂无兑换历史</div>
            ) : (
              <table className="w-full text-sm text-center">
                <thead>
                  <tr className="border-b border-border/50 text-muted-foreground">
                    <th className="pb-1.5 pr-2 font-medium">账号</th>
                    <th className="pb-1.5 pr-2 font-medium">兑换码</th>
                    <th className="pb-1.5 pr-2 font-medium">状态</th>
                    <th className="pb-1.5 font-medium">时间</th>
                  </tr>
                </thead>
                <tbody>
                  {history.map((h, i) => {
                    const s = statusLabel(h.status);
                    const { name, zone } = parseAccountDisplay(h.accountName || h.accountId.toString());
                    return (
                      <tr key={`${h.accountId}-${h.code}-${i}`} className="border-b border-border/30 last:border-0">
                        <td className="py-1.5 pr-2">
                          <div className="font-medium truncate max-w-[100px]">{name}</div>
                          {zone && <div className="text-[10px] text-muted-foreground">{zone}</div>}
                        </td>
                        <td className="py-1.5 pr-2 font-mono text-xs truncate max-w-[120px]">{h.code}</td>
                        <td className={cn("py-1.5 pr-2", s.className)}>{s.text}</td>
                        <td className="py-1.5 text-muted-foreground text-xs">{formatCompactTime(h.createdAt)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        )}
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
