"use client";

import { useEffect, useState } from "react";
import { Clock3, Eye, Gift, Sparkles } from "lucide-react";
import { VideoActionState, type VideoActionStatusView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import { formatCount } from "@/components/dashboard/dashboard-utils";
import { cn } from "@/lib/utils";

export function VideoActionItems({ actions, className }: {
  actions: VideoActionStatusView[];
  className?: string;
}) {
  // Keep the server render and first client render identical. The wall clock is
  // started after hydration so countdown text cannot cause a hydration mismatch.
  const [nowMs, setNowMs] = useState(0);
  const hasLiveDeadline = actions.some(
    (action) => Number(action.availableAtMs) > nowMs || Number(action.expiresAtMs) > nowMs,
  );

  useEffect(() => {
    if (!hasLiveDeadline) return;
    const frame = window.requestAnimationFrame(() => setNowMs(Date.now()));
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => {
      window.cancelAnimationFrame(frame);
      window.clearInterval(timer);
    };
  }, [hasLiveDeadline]);

  return (
    <div className={cn("grid gap-2", className)}>
      {actions.map((action) => <VideoActionItem key={action.id} action={action} nowMs={nowMs} />)}
    </div>
  );
}

function VideoActionItem({ action, nowMs }: { action: VideoActionStatusView; nowMs: number }) {
  const effectiveState = effectiveVideoActionState(action, nowMs);
  const status = videoActionStatusLabel(action, nowMs);
  const reward = videoActionRewardLabel(action);
  const used = action.state === VideoActionState.EXHAUSTED && effectiveState === VideoActionState.READY ? 0 : action.used;
  const usage = action.observed && action.limit > 0 ? `${formatCount(used)}/${formatCount(action.limit)}` : "";
  return (
    <div
      className={cn(
        "min-w-0 rounded-md border border-border/58 bg-white/42 px-3 py-2 dark:bg-white/5",
        effectiveState === VideoActionState.READY && "border-primary/35 bg-primary/6",
        effectiveState === VideoActionState.ACTIVE && "border-emerald-400/40 bg-emerald-400/8",
        effectiveState === VideoActionState.COOLDOWN && "border-amber-300/55 bg-amber-50/55 dark:bg-amber-400/8",
      )}
    >
      <div className="flex min-w-0 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2 text-sm font-medium">
          <VideoActionIcon action={action} />
          <span className="truncate">{action.label}</span>
        </div>
        <Badge variant={effectiveState === VideoActionState.READY || effectiveState === VideoActionState.ACTIVE ? "secondary" : "outline"} className="shrink-0">
          {status}
        </Badge>
      </div>
      <div className="mt-1 flex min-w-0 items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="min-w-0 truncate" title={action.detail}>{reward || action.detail || "仅展示服务端状态"}</span>
        {usage && <span className="shrink-0 tabular-nums">今日 {usage}</span>}
      </div>
    </div>
  );
}

function VideoActionIcon({ action }: { action: VideoActionStatusView }) {
  const className = "size-4 shrink-0 text-primary";
  if (action.id.includes("double_gold")) return <Sparkles className={className} />;
  if (action.id.includes("giftbag")) return <Gift className={className} />;
  if (action.state === VideoActionState.COOLDOWN) return <Clock3 className={className} />;
  return <Eye className={className} />;
}

export function videoActionStatusLabel(action: VideoActionStatusView, nowMs: number) {
  switch (effectiveVideoActionState(action, nowMs)) {
    case VideoActionState.ACTIVE:
      if (nowMs <= 0) return "生效中";
      return deadlineLabel("剩余", Number(action.expiresAtMs), nowMs) || "生效中";
    case VideoActionState.READY:
      return "可尝试";
    case VideoActionState.COOLDOWN:
      if (nowMs <= 0) return "冷却中";
      return deadlineLabel("还有", Number(action.availableAtMs), nowMs) || "待刷新";
    case VideoActionState.EXHAUSTED:
      return "今日已用完";
    case VideoActionState.UNKNOWN:
    case VideoActionState.UNSPECIFIED:
    default:
      return "待同步";
  }
}

export function effectiveVideoActionState(action: VideoActionStatusView, nowMs: number) {
  if (nowMs <= 0) return action.state;
  const expiresAtMs = Number(action.expiresAtMs);
  const availableAtMs = Number(action.availableAtMs);
  if (action.state === VideoActionState.ACTIVE && expiresAtMs > 0 && expiresAtMs <= nowMs) {
    return VideoActionState.READY;
  }
  if (
    (action.state === VideoActionState.COOLDOWN || action.state === VideoActionState.EXHAUSTED)
    && availableAtMs > 0
    && availableAtMs <= nowMs
  ) {
    return VideoActionState.READY;
  }
  return action.state;
}

export function videoActionRewardLabel(action: VideoActionStatusView) {
  return action.rewards
    .slice(0, 2)
    .map((reward) => `${reward.itemName || `#${reward.itemId}`} ×${formatCount(reward.count)}`)
    .join(" · ");
}

function deadlineLabel(prefix: string, deadlineMs: number, nowMs: number) {
  const remaining = deadlineMs - nowMs;
  if (!Number.isFinite(remaining) || remaining <= 0) return "";
  const totalSeconds = Math.ceil(remaining / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${prefix} ${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  return `${prefix} ${minutes}:${String(seconds).padStart(2, "0")}`;
}
