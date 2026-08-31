import { Coins, Gem, HandCoins, ShieldCheck, Ticket, TrendingUp, Trophy, Waves } from "lucide-react";
import { ExecutionLane, PlanStatus } from "@/lib/api/workspace-models";
import type { AccountStatus, BasicView, PearlHireStatusView, PlannedOperation, WarehouseView } from "@/lib/api/workspace-models";
import { Badge } from "@/components/ui/badge";
import {
  firstPositiveUnixTime,
  formatCount,
  formatTimestamp,
  formatUnixTime,
  isOperationCooling,
  operationCostLabel,
  operationNoteLabel,
  OperationStatusBadge,
  operationTargetLabel,
  operationTitle,
} from "@/components/dashboard/dashboard-utils";
import { experienceToNextLevel } from "@/lib/game/catalog";
import { CollapsibleCard, EmptyState, OverviewStat } from "@/features/workspace/shared/workspace-ui";
import { VideoActionItems } from "@/features/workspace/shared/video-action-status";

const SPEED_UP_TICKET_ITEM_ID = 1001;
const FLORAL_COIN_ITEM_ID = 1002;

export function StatusOverviewPanel({ basic: snapshot, warehouse, status }: {
  basic: BasicView | null;
  warehouse: WarehouseView | null;
  status?: AccountStatus;
}) {
  const floralCoins = warehouse?.inventory[FLORAL_COIN_ITEM_ID] ?? 0;
  const speedUpTickets = warehouse?.inventory[SPEED_UP_TICKET_ITEM_ID] ?? 0;
  const reputationObserved = snapshot?.reputationObserved ?? status?.reputationObserved ?? false;
  const reputationScore = snapshot?.reputationScore ?? status?.reputationScore ?? 0;
  const reputationTime = firstPositiveUnixTime(
    snapshot?.reputationLastViewTimeMs,
    snapshot?.reputationLastSyncTimeMs,
    status?.reputationLastViewTimeMs,
    status?.reputationLastSyncTimeMs,
  );
  const level = snapshot?.level ?? status?.level ?? 0;
  const experience = snapshot?.experience ?? status?.experience ?? 0;
  const apiNextLevelExperience = snapshot?.nextLevelExperience ?? status?.nextLevelExperience ?? 0;
  const apiLevelMaxed = snapshot?.levelMaxed ?? status?.levelMaxed ?? false;
  const apiHasNextLevel = apiLevelMaxed || apiNextLevelExperience > 0;
  const localNextLevel = experienceToNextLevel(level, experience);
  const levelMaxed = apiHasNextLevel ? apiLevelMaxed : localNextLevel.maxed;
  const nextLevelExperience = apiHasNextLevel ? apiNextLevelExperience : localNextLevel.required;
  const experienceToNext = apiHasNextLevel
    ? (snapshot?.experienceToNextLevel ?? status?.experienceToNextLevel ?? 0)
    : localNextLevel.remaining;
  const reputationDetail = reputationObserved ? (reputationTime ? `同步 ${formatUnixTime(reputationTime)}` : "已同步") : "未同步";
  const nextLevelValue = levelMaxed
    ? "已满级"
    : nextLevelExperience > 0
      ? `${formatCount(experienceToNext)} 经验`
      : "-";
  const nextLevelDetail = levelMaxed
    ? "已达最高等级"
    : nextLevelExperience > 0
      ? `当前 ${formatCount(experience)} / 需要 ${formatCount(nextLevelExperience)}`
      : undefined;

  return (
    <CollapsibleCard title="监控概览" actions={snapshot?.capturedAt && <Badge variant="outline">快照 {formatTimestamp(snapshot.capturedAt)}</Badge>}>
      <div className="space-y-3">
        <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
          <OverviewStat icon={<ShieldCheck />} label="礼仪分" value={reputationObserved ? formatCount(reputationScore) : "-"} detail={reputationDetail} />
          <OverviewStat icon={<Trophy />} label="等级" value={level > 0 ? `${level}级` : "-"} detail={`经验 ${formatCount(experience)}`} />
          <OverviewStat icon={<TrendingUp />} label="距下一等级" value={nextLevelValue} detail={nextLevelDetail} wrap compact />
          <OverviewStat icon={<Waves />} label="水滴" value={`${formatCount(snapshot?.waterDrops ?? 0)}/${formatCount(snapshot?.waterDropsTotal ?? 0)}`} />
          <OverviewStat icon={<Gem />} label="元宝" value={formatCount(snapshot?.diamondsFree ?? 0)} />
          <OverviewStat icon={<Coins />} label="金币" value={formatCount(snapshot?.gold ?? 0)} />
          <OverviewStat icon={<HandCoins />} label="花坊币" value={formatCount(floralCoins)} />
          <OverviewStat icon={<Ticket />} label="加速卡" value={formatCount(speedUpTickets)} />
        </div>
        <section className="border-t border-border/58 pt-3">
          <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
            <div className="text-sm font-semibold">手动提醒</div>
            <div className="text-xs text-muted-foreground">仅展示服务端状态，请在官方游戏内操作</div>
          </div>
          <VideoActionItems actions={snapshot?.videoActions ?? []} className="sm:grid-cols-3" />
        </section>
      </div>
    </CollapsibleCard>
  );
}

export function PearlHirePanel({ pearlHire }: { pearlHire?: PearlHireStatusView }) {
  const slots = pearlHire?.slots ?? [];
  const ticketCount = pearlHire?.ticketCount ?? 0;
  const usedToday = pearlHire?.ticketUsedToday ?? 0;
  const dailyLimit = pearlHire?.dailyTicketLimit ?? 0;
  const usageLabel = dailyLimit > 0 ? `${formatCount(usedToday)}/${formatCount(dailyLimit)}` : `${formatCount(usedToday)}（不限）`;
  return (
    <CollapsibleCard
      title="珍珠雇佣"
      actions={(
        <>
          <Badge variant="outline">雇佣券 {formatCount(ticketCount)}</Badge>
          <Badge variant="secondary">今日 {usageLabel}</Badge>
        </>
      )}
    >
      {slots.length === 0 ? (
        <EmptyState title="暂无劳工槽位快照" detail="登录并完成珍珠状态同步后显示" />
      ) : (
        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
          {slots.map((slot) => {
            const occupied = slot.laborUid > BigInt(0);
            const name = slot.laborName?.trim() || (occupied ? String(slot.laborUid) : "空闲");
            const endMs = Number(slot.laborEndTimeMs);
            const endLabel = endMs > 0
              ? `${slot.active ? "结束" : "已结束"} ${new Intl.DateTimeFormat("zh-CN", {
                  month: "2-digit",
                  day: "2-digit",
                  hour: "2-digit",
                  minute: "2-digit",
                }).format(new Date(endMs))}`
              : "待雇佣";
            return (
              <div key={slot.placeId} className="rounded-md border border-border/58 bg-white/34 px-3 py-2 dark:bg-white/5">
                <div className="flex items-center justify-between gap-2 text-sm">
                  <span className="font-medium">槽位 {slot.placeId}</span>
                  <Badge variant={slot.active ? "default" : "outline"}>{slot.active ? "在岗" : occupied ? "到期" : "空闲"}</Badge>
                </div>
                <div className="mt-1 truncate text-sm text-muted-foreground">{name}</div>
                <div className="mt-0.5 text-xs text-muted-foreground">{endLabel}</div>
              </div>
            );
          })}
        </div>
      )}
    </CollapsibleCard>
  );
}

export function OperationPanel({ operations }: { operations: PlannedOperation[] }) {
  const queueOperations = operations.filter(isQueueOperation);
  const farmOperations = queueOperations.filter((operation) => operation.lane === ExecutionLane.FARM);
  const sideOperations = queueOperations.filter((operation) => operation.lane !== ExecutionLane.FARM);
  return (
    <CollapsibleCard title="执行队列" actions={<Badge variant="secondary">{queueOperations.length}</Badge>}>
      <div className="max-h-[360px] overflow-hidden rounded-md border border-border/58 bg-white/34 md:h-[220px] md:max-h-none dark:bg-white/5">
        {queueOperations.length === 0 ? (
          <div className="flex min-h-28 items-center justify-center px-3 text-sm text-muted-foreground md:h-full md:min-h-0">当前无可执行操作</div>
        ) : (
          <div className="grid min-h-0 md:h-full md:grid-cols-2">
            <OperationLaneSection title="种植通道" operations={farmOperations} emptyText="暂无收获、播种或浇水" />
            <OperationLaneSection title="其他通道" operations={sideOperations} emptyText="暂无任务、订单或活动操作" />
          </div>
        )}
      </div>
    </CollapsibleCard>
  );
}

function OperationLaneSection({ title, operations, emptyText }: { title: string; operations: PlannedOperation[]; emptyText: string }) {
  return (
    <section className="flex min-h-0 min-w-0 flex-col border-b border-border/58 last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
      <div className="flex h-8 items-center justify-between bg-secondary/55 px-3 text-xs font-semibold dark:bg-muted/45">
        <span>{title}</span>
        <Badge variant="secondary">{operations.length}</Badge>
      </div>
      {operations.length === 0 ? (
        <div className="flex min-h-14 flex-1 items-center px-3 py-3 text-sm text-muted-foreground md:min-h-0">{emptyText}</div>
      ) : (
        <div className="dark-scrollbar min-h-0 flex-1 divide-y divide-border/70 overflow-auto">
          {operations.map((operation, index) => (
            <OperationRow key={operation.operationId || `${operation.rpc}-${index}`} operation={operation} />
          ))}
        </div>
      )}
    </section>
  );
}

function OperationRow({ operation }: { operation: PlannedOperation }) {
  const target = operationTargetLabel(operation);
  const cost = operationCostLabel(operation);
  const note = operationNoteLabel(operation);
  return (
    <div className="flex min-h-12 items-center gap-3 px-3 py-2" title={[operation.rpc, operation.domain, operation.reason].filter(Boolean).join(" · ")}>
      <div className="shrink-0"><OperationStatusBadge operation={operation} /></div>
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1 text-sm">
        <span className="font-medium">{operationTitle(operation)}</span>
        {target && <span className="text-muted-foreground">{target}</span>}
        {cost && <span className="text-muted-foreground">{cost}</span>}
        {note && <span className="text-muted-foreground">{note}</span>}
      </div>
    </div>
  );
}

function isQueueOperation(operation: PlannedOperation) {
  return isRunnableOperation(operation) || isOperationCooling(operation);
}

function isRunnableOperation(operation: PlannedOperation) {
  return operation.executable &&
    !operation.syncOnly &&
    operation.status !== PlanStatus.ADAPTER_MISSING &&
    operation.status !== PlanStatus.BLOCKED &&
    operation.blockedReasons.length === 0;
}
