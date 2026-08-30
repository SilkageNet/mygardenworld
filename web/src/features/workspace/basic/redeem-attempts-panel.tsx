import { timestampDate } from "@bufbuild/protobuf/wkt";
import { AlertCircle, CheckCircle2, Clock3, Gift, LoaderCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import {
  AccountRedeemAttemptFilter,
  AccountRedeemAttemptStatus,
  type AccountRedeemAttempt,
} from "@/gen/mygardenworld/v1/workspace_pb";
import { CollapsibleCard, EmptyState } from "@/features/workspace/shared/workspace-ui";
import type { RedeemAttemptFeed } from "./redeem-attempts-model";

const FILTERS = [
  { id: AccountRedeemAttemptFilter.ALL, label: "全部" },
  { id: AccountRedeemAttemptFilter.REDEEMED, label: "已兑换" },
  { id: AccountRedeemAttemptFilter.UNAVAILABLE, label: "未兑换" },
  { id: AccountRedeemAttemptFilter.ATTENTION, label: "待确认" },
];

export function RedeemAttemptsPanel({ feed, onFilterChange, onLoadMore }: {
  feed: RedeemAttemptFeed;
  onFilterChange: (filter: AccountRedeemAttemptFilter) => void;
  onLoadMore: () => void;
}) {
  const summary = feed.summary;
  const redeemed = (summary?.success ?? BigInt(0)) + (summary?.alreadyRedeemed ?? BigInt(0));
  const unavailable = (summary?.expired ?? BigInt(0)) + (summary?.invalid ?? BigInt(0));
  const attention = (summary?.pending ?? BigInt(0)) + (summary?.running ?? BigInt(0)) +
    (summary?.retryable ?? BigInt(0)) + (summary?.unknown ?? BigInt(0));
  const filterCounts = new Map<AccountRedeemAttemptFilter, bigint>([
    [AccountRedeemAttemptFilter.ALL, summary?.total ?? BigInt(0)],
    [AccountRedeemAttemptFilter.REDEEMED, redeemed],
    [AccountRedeemAttemptFilter.UNAVAILABLE, unavailable],
    [AccountRedeemAttemptFilter.ATTENTION, attention],
  ]);
  const firstLoading = feed.loadingMode === "replace" && feed.entries.length === 0;

  return (
    <CollapsibleCard
      title="兑换记录"
      actions={(
        <div className="flex flex-wrap items-center justify-end gap-1.5">
          <Badge variant="secondary">已兑换 {redeemed.toString()}</Badge>
          <Badge variant="outline">共 {summary?.total.toString() ?? "-"}</Badge>
        </div>
      )}
    >
      <div className="space-y-2.5">
        <div className="dark-scrollbar flex gap-1 overflow-x-auto pb-0.5" role="group" aria-label="兑换记录筛选">
          {FILTERS.map((filter) => (
            <Button
              key={filter.id}
              type="button"
              size="sm"
              variant={feed.filter === filter.id ? "secondary" : "ghost"}
              className="h-8 shrink-0 gap-1.5 px-2.5"
              onClick={() => onFilterChange(filter.id)}
              disabled={feed.loadingMode !== "" && feed.filter === filter.id}
            >
              {filter.label}
              <span className="text-xs tabular-nums text-muted-foreground">{filterCounts.get(filter.id)?.toString() ?? "0"}</span>
            </Button>
          ))}
        </div>

        {firstLoading ? (
          <div className="flex min-h-28 items-center justify-center gap-2 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />正在加载兑换记录
          </div>
        ) : feed.entries.length === 0 ? (
          <EmptyState
            title={feed.filter === AccountRedeemAttemptFilter.ALL ? "暂无兑换记录" : "该分类暂无记录"}
            detail="新兑换码进入账号处理队列后会自动记录结果"
          />
        ) : (
          <div className="divide-y divide-border/65 overflow-hidden rounded-md border border-border/58 bg-white/34 dark:bg-white/5">
            {feed.entries.map((entry) => <RedeemAttemptRow key={entry.id.toString()} entry={entry} />)}
          </div>
        )}

        {feed.hasMore && (
          <Button
            type="button"
            variant="outline"
            className="w-full"
            onClick={onLoadMore}
            disabled={feed.loadingMode !== ""}
          >
            {feed.loadingMode === "append" && <LoaderCircle className="size-4 animate-spin" />}
            加载更早记录
          </Button>
        )}
      </div>
    </CollapsibleCard>
  );
}

function RedeemAttemptRow({ entry }: { entry: AccountRedeemAttempt }) {
  const timestamp = entry.attemptedAt ?? entry.updatedAt;
  return (
    <div className="flex min-w-0 flex-col gap-2 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          <span className="break-all font-medium">{entry.code}</span>
          <Badge variant="outline">{entry.channel === Channel.ALIPAY ? "Alipay" : "iOS"}</Badge>
          <AttemptStatusBadge status={entry.status} />
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
          <span>{timestamp ? formatAttemptTime(timestampDate(timestamp)) : "尚未尝试"}</span>
          {entry.attemptCount > 0 && <span>尝试 {entry.attemptCount} 次</span>}
          {entry.expiresAt && <span>有效至 {formatAttemptTime(timestampDate(entry.expiresAt))}</span>}
        </div>
        {entry.message && <div className="mt-1 break-words text-xs text-muted-foreground">{entry.message}</div>}
      </div>
    </div>
  );
}

function AttemptStatusBadge({ status }: { status: AccountRedeemAttemptStatus }) {
  switch (status) {
    case AccountRedeemAttemptStatus.SUCCESS:
      return <Badge><CheckCircle2 />兑换成功</Badge>;
    case AccountRedeemAttemptStatus.ALREADY_REDEEMED:
      return <Badge variant="secondary"><Gift />此前已兑换</Badge>;
    case AccountRedeemAttemptStatus.EXPIRED:
      return <Badge variant="outline"><Clock3 />已过期</Badge>;
    case AccountRedeemAttemptStatus.INVALID:
      return <Badge variant="destructive"><AlertCircle />无效码</Badge>;
    case AccountRedeemAttemptStatus.RUNNING:
      return <Badge variant="secondary"><LoaderCircle className="animate-spin" />兑换中</Badge>;
    case AccountRedeemAttemptStatus.RETRYABLE:
      return <Badge variant="outline"><Clock3 />等待重试</Badge>;
    case AccountRedeemAttemptStatus.UNKNOWN:
      return <Badge variant="outline"><AlertCircle />结果未知</Badge>;
    default:
      return <Badge variant="outline"><Clock3 />等待处理</Badge>;
  }
}

function formatAttemptTime(value: Date) {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(value);
}
