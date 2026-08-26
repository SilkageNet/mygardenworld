import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { AlipayLoginStatus } from "@/gen/mygardenworld/v1/account_service_pb";
import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { PlanStatus } from "@/gen/mygardenworld/v1/query_service_pb";
import type { AccountStatus, DailyBusinessStatisticsView, CyclicNoteView, CyclicStoryView, DessertView, Event, FmlLandView, LandView, OrderStatisticsView, PendingTaskView, PlannedOperation, RuntimeActionTotal, RuntimeResourceTotal, RuntimeStatisticsView } from "@/gen/mygardenworld/v1/query_service_pb";
import { Badge } from "@/components/ui/badge";
import { itemName } from "@/lib/game/catalog";

const NUMBER_FORMATTER = new Intl.NumberFormat("zh-CN");

export function accountIdentity(account: Account, status?: AccountStatus) {
  return {
    nickname: accountNickname(account),
    area: accountAreaLabel(account, status),
    channel: channelLabel(account.channel),
  };
}

export function accountNickname(account: Account) {
  const withoutArea = account.name
    .replace(/\s*·\s*第\s*\d+\s*区(?:\s*#\d+)?\s*$/, "")
    .replace(/\s+第\s*\d+\s*区(?:\s*#\d+)?\s*$/, "")
    .trim();
  const withoutServerPrefix = withoutArea.replace(/^s\d{2,}[.．·\s_-]+/i, "").trim();
  return withoutServerPrefix || withoutArea || account.name || "账号";
}

export function accountAreaLabel(account: Account, status?: AccountStatus) {
  const gsIdx = status?.gsIdx || account.gsIdx;
  if (gsIdx > 0) return `第${gsIdx}区`;
  const match = account.name.match(/第\s*(\d+)\s*区/);
  if (match) return `第${match[1]}区`;
  const serverMatch = account.name.match(/^s(\d{2,})[.．·\s_-]+/i);
  if (serverMatch) return `第${serverMatch[1]}区`;
  return "未知区";
}

export function channelLabel(channel: Channel) {
  switch (channel) {
    case Channel.IOS:
      return "iOS";
    case Channel.ALIPAY:
      return "支付宝";
    default:
      return "未知渠道";
  }
}

export function alipayLoginStatusLabel(status: AlipayLoginStatus) {
  switch (status) {
    case AlipayLoginStatus.WAITING_FOR_SCAN:
      return "等待支付宝扫码确认";
    case AlipayLoginStatus.PROCESSING:
      return "正在验证游戏登录…";
    case AlipayLoginStatus.COMPLETE:
      return "绑定完成";
    case AlipayLoginStatus.EXPIRED:
      return "二维码已过期";
    case AlipayLoginStatus.FAILED:
      return "登录验证失败";
    default:
      return "准备扫码";
  }
}

export function accountConnected(account: Account, status?: AccountStatus) {
  return status?.connected ?? account.connected;
}

export function isRunnerNotStartedError(err: unknown) {
  const message = err instanceof Error ? err.message : String(err ?? "");
  return message.includes("runner not started") || message.includes("failed_precondition");
}

export function isTransientConnectionMessage(message: string) {
  return /network\s*error|networkerror|failed to fetch|load failed|无法连接到后端服务|事件流中断|后端服务暂时不可用|请求超时/i.test(message);
}

export function waitForAbortableDelay(delayMs: number, signal: AbortSignal): Promise<boolean> {
  if (signal.aborted) return Promise.resolve(false);
  return new Promise((resolve) => {
    const onTimeout = () => {
      signal.removeEventListener("abort", onAbort);
      resolve(true);
    };
    const timeout = window.setTimeout(onTimeout, delayMs);
    const onAbort = () => {
      window.clearTimeout(timeout);
      signal.removeEventListener("abort", onAbort);
      resolve(false);
    };
    signal.addEventListener("abort", onAbort, { once: true });
    if (signal.aborted) onAbort();
  });
}

export function accountIsAbnormal(status?: AccountStatus) {
  if (accountStatusIssues(status).length > 0) return true;
  return status?.health === "blocked" || status?.health === "session_expired" || Boolean(status?.lastError);
}

export function HealthBadge({ account, status }: { account: Account; status?: AccountStatus }) {
  const connected = accountConnected(account, status);
  if (accountIsAbnormal(status)) return <Badge variant="destructive">异常</Badge>;
  if (!connected) return <Badge variant="outline">离线</Badge>;
  return <Badge variant="secondary">在线</Badge>;
}

export function accountStatusIssues(status?: AccountStatus) {
  const diagnostics = status?.diagnostics;
  const issues = [
    status?.lastError,
    diagnostics?.lastOperationError,
    diagnostics?.sessionInvalidatedReason,
    ...(diagnostics?.blockedReasons ?? []),
  ]
    .map((issue) => issue?.trim())
    .filter((issue): issue is string => Boolean(issue));

  if (status?.health === "blocked" && issues.length === 0) {
    issues.push("账号处于异常状态，但后端未返回具体原因。");
  }

  return [...new Set(issues)];
}

export function OperationStatusBadge({ operation }: { operation: PlannedOperation }) {
  if (isOperationCooling(operation)) return <Badge variant="secondary">冷却</Badge>;
  if (operation.status === PlanStatus.BLOCKED || operation.blockedReasons.length > 0) return <Badge variant="destructive">阻塞</Badge>;
  if (operation.syncOnly) return <Badge variant="outline">同步</Badge>;
  if (!operation.executable) return <Badge variant="outline">{planStatusLabel(operation.status)}</Badge>;
  if (operation.status === PlanStatus.MANAGED) return <Badge variant="secondary">调度</Badge>;
  return <Badge>可执行</Badge>;
}

export function comparePendingTasks(a: PendingTaskView, b: PendingTaskView) {
  const statusDelta = pendingTaskStatusRank(a) - pendingTaskStatusRank(b);
  if (statusDelta !== 0) return statusDelta;
  const categoryDelta = pendingTaskCategoryRank(a.category) - pendingTaskCategoryRank(b.category);
  if (categoryDelta !== 0) return categoryDelta;
  const aID = Number(a.id);
  const bID = Number(b.id);
  if (Number.isFinite(aID) && Number.isFinite(bID) && aID !== bID) return aID - bID;
  return (a.title || a.id).localeCompare(b.title || b.id, "zh-CN");
}

export function pendingTaskStatusRank(task: PendingTaskView) {
  if (pendingTaskBlocked(task)) return 0;
  if (pendingTaskHasShortage(task)) return 1;
  if (pendingTaskCooling(task)) return 3;
  switch (task.status) {
    case PlanStatus.READY:
      return 2;
    case PlanStatus.MANAGED:
      return 4;
    case PlanStatus.SYNC_ONLY:
      return 5;
    case PlanStatus.SKIPPED:
      return 6;
    default:
      return 7;
  }
}

export function pendingTaskCategoryRank(category: string) {
  const order = ["顾客订单", "居民订单", "主线任务", "主线剧情", "日常任务", "周常任务", "成就任务", "activity", "地图随机事件", "宠物事件"];
  const index = order.indexOf(category);
  return index >= 0 ? index : order.length;
}

export function isOrderPendingTask(task: PendingTaskView) {
  if (task.category === "activity") return false;
  return task.category.includes("订单") || task.title.includes("订单");
}

export function pendingTaskCategoryLabel(category: string) {
  return category === "activity" ? "活动" : categoryLabel(category);
}

export function pendingTaskBlocked(task: PendingTaskView) {
  return (
    task.status === PlanStatus.BLOCKED ||
    task.requirements.some((req) => req.blockedReasons.length > 0)
  );
}

export function pendingTaskHasShortage(task: PendingTaskView) {
  return task.requirements.some((req) => req.missing > 0);
}

export function pendingTaskCooling(task: PendingTaskView) {
  return Number(task.cooldownUntilMs) > Date.now();
}

export function taskMonitorDetail(tasks: PendingTaskView[]) {
  if (tasks.length === 0) return "暂无";
  const ready = tasks.filter((task) => task.status === PlanStatus.READY && !pendingTaskCooling(task)).length;
  const cooling = tasks.filter(pendingTaskCooling).length;
  const shortage = tasks.filter(pendingTaskHasShortage).length;
  const blocked = tasks.filter(pendingTaskBlocked).length;
  return [`可处理 ${ready}`, cooling > 0 ? `冷却 ${cooling}` : "", shortage > 0 ? `缺口 ${shortage}` : "", blocked > 0 ? `阻塞 ${blocked}` : ""].filter(Boolean).join(" / ");
}

export function requirementShortageSummary(tasks: PendingTaskView[]) {
  const totals = new Map<number, { name: string; missing: number }>();
  for (const task of tasks) {
    for (const req of task.requirements) {
      if (req.missing <= 0) continue;
      const current = totals.get(req.itemId) ?? { name: req.itemName || itemName(req.itemId), missing: 0 };
      current.missing += req.missing;
      totals.set(req.itemId, current);
    }
  }
  return [...totals.values()]
    .sort((a, b) => b.missing - a.missing || a.name.localeCompare(b.name, "zh-CN"))
    .slice(0, 3)
    .map((item) => `${item.name} ${formatCount(item.missing)}`)
    .join("、");
}

export function pendingTaskProgressPercent(task: PendingTaskView) {
  if (task.target <= 0) return 0;
  return Math.max(0, Math.min(100, Math.round((task.finished / task.target) * 100)));
}

export function taskProgressLabel(task: PendingTaskView) {
  if (pendingTaskCooling(task)) {
    const reason = task.cooldownReason || "冷却中";
    return `${reason}，约 ${pendingTaskCooldownRemaining(task)}后可交付`;
  }
  if (task.target > 0) return `${formatCount(task.finished)}/${formatCount(task.target)}`;
  if (task.requirements.length === 0) return "";
  const missing = task.requirements.reduce((sum, req) => sum + Math.max(0, req.missing), 0);
  return missing > 0 ? `缺 ${formatCount(missing)}` : "资源满足";
}

export function pendingTaskCooldownRemaining(task: PendingTaskView) {
  const seconds = Math.max(1, Math.ceil((Number(task.cooldownUntilMs) - Date.now()) / 1000));
  if (seconds < 60) return `${seconds} 秒`;
  const minutes = Math.ceil(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟`;
  return `${Math.ceil(minutes / 60)} 小时`;
}

export function orderStatisticItems(statistics?: OrderStatisticsView) {
  return [
    { label: "居民", value: statistics?.residentNormalFinished ?? 0 },
    { label: "顾客", value: statistics?.customerFinished ?? 0 },
    { label: "宫廷", value: statistics?.palaceFinished ?? 0 },
    { label: "绸缎", value: statistics?.residentSatinFinished ?? 0 },
    { label: "建材", value: statistics?.residentDecorateFinished ?? 0 },
    { label: "花艺", value: statistics?.flowerArtSold ?? 0 },
  ];
}

export function businessOrderItems(day?: DailyBusinessStatisticsView) {
  return [
    { label: "居民", value: day?.residentNormalFinished ?? 0 },
    { label: "顾客", value: day?.customerFinished ?? 0 },
    { label: "宫廷", value: day?.palaceFinished ?? 0 },
    { label: "绸缎", value: day?.residentSatinFinished ?? 0 },
    { label: "建材", value: day?.residentDecorateFinished ?? 0 },
    { label: "花艺", value: day?.flowerArtSold ?? 0 },
    { label: "绸缎库存", value: day?.satin ?? 0 },
    { label: "木材", value: day?.wood ?? 0 },
  ];
}

export function sumRuntimeActionTotals(items: RuntimeActionTotal[]) {
  return items.reduce((sum, item) => sum + item.count, BigInt(0));
}

export function runtimeWindowLabel(statistics?: RuntimeStatisticsView) {
  if (!statistics) return "暂无运行统计";
  if (statistics.running) {
    const started = formatTimestamp(statistics.startedAt);
    return started === "-" ? "运行中" : `启动 ${started}`;
  }
  const stopped = formatTimestamp(statistics.stoppedAt);
  return stopped === "-" ? "最近已停止" : `停止 ${stopped}`;
}

export function runtimeResourcePrimaryValue(items: RuntimeResourceTotal[]) {
  const first = items.find((item) => item.gained > BigInt(0));
  if (!first) return "-";
  return `+${formatCount(first.gained)}`;
}

export function runtimeResourceGainSummary(items: RuntimeResourceTotal[]) {
  const visible = items.filter((item) => item.gained > BigInt(0)).slice(0, 3);
  if (visible.length === 0) return "暂无资源进账";
  return visible.map((item) => `${item.label || item.key} +${formatCount(item.gained)}`).join("、");
}

export function runtimeActionSummary(items: RuntimeActionTotal[]) {
  const visible = items.filter((item) => item.count > BigInt(0)).slice(0, 3);
  if (visible.length === 0) return "暂无完成";
  return visible.map((item) => `${item.label || item.key} ${formatCount(item.count)}`).join("、");
}

export function cyclicNotePhaseLabel(phase: number) {
  switch (phase) {
    case 1:
      return "预告期";
    case 2:
      return "进行中";
    case 3:
      return "领奖期";
    case 4:
      return "已结束";
    default:
      return "未开始";
  }
}

export function cyclicNotePhaseDetail(activity: CyclicNoteView) {
  if (activity.phase === 4) return activity.endMs > BigInt(0) ? `结束于 ${formatUnixTime(activity.endMs)}` : "活动已结束";
  const endMs = Number(activity.phaseEndMs);
  if (!Number.isFinite(endMs) || endMs <= 0) return "阶段时间尚未同步";
  const remaining = endMs - Date.now();
  if (remaining <= 0) return "等待服务端阶段更新";
  const prefix = activity.phase === 1 ? "距开始" : activity.phase === 3 ? "领奖剩余" : "剩余";
  return `${prefix} ${formatRemainingMilliseconds(remaining)}`;
}

export function cyclicStoryPhaseDetail(activity: CyclicStoryView) {
  if (activity.phase === 4) return activity.endMs > BigInt(0) ? `结束于 ${formatUnixTime(activity.endMs)}` : "活动已结束";
  const endMs = Number(activity.phaseEndMs);
  if (!Number.isFinite(endMs) || endMs <= 0) return "阶段时间尚未同步";
  const remaining = endMs - Date.now();
  if (remaining <= 0) return "等待服务端阶段更新";
  const prefix = activity.phase === 1 ? "距开始" : activity.phase === 3 ? "领奖剩余" : "剩余";
  return `${prefix} ${formatRemainingMilliseconds(remaining)}`;
}

export function dessertPhaseDetail(activity: DessertView) {
  if (activity.phase === 4) return activity.endMs > BigInt(0) ? `结束于 ${formatUnixTime(activity.endMs)}` : "活动已结束";
  const endMs = Number(activity.phaseEndMs);
  if (!Number.isFinite(endMs) || endMs <= 0) return "阶段时间尚未同步";
  const remaining = endMs - Date.now();
  if (remaining <= 0) return "等待服务端阶段更新";
  const prefix = activity.phase === 1 ? "距开始" : activity.phase === 3 ? "领奖剩余" : "剩余";
  return `${prefix} ${formatRemainingMilliseconds(remaining)}`;
}

export function formatRemainingMilliseconds(milliseconds: number) {
  const totalMinutes = Math.max(1, Math.ceil(milliseconds / 60_000));
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;
  if (days > 0) return `${days}天${hours > 0 ? `${hours}小时` : ""}`;
  if (hours > 0) return `${hours}小时${minutes > 0 ? `${minutes}分` : ""}`;
  return `${minutes}分钟`;
}

export function planStatusLabel(status: PlanStatus) {
  switch (status) {
    case PlanStatus.READY:
      return "可执行";
    case PlanStatus.MANAGED:
      return "调度";
    case PlanStatus.SYNC_ONLY:
      return "同步";
    case PlanStatus.ADAPTER_MISSING:
      return "缺适配";
    case PlanStatus.BLOCKED:
      return "阻塞";
    case PlanStatus.SKIPPED:
      return "跳过";
    default:
      return "等待";
  }
}

export function operationTitle(operation: PlannedOperation) {
  return operation.label || operationActionLabel(operation.action) || operation.domain || operation.rpc || "操作";
}

export function operationTargetLabel(operation: PlannedOperation) {
  const landIds = operationLandIds(operation);
  if (landIds.length > 0) {
    return landIds.map(landDisplayName).join("、");
  }
  if (operation.rpc === "flowerArt.makeFlowerArt") {
    const art = operation.itemId ? itemName(operation.itemId) : "花艺";
    const count = operation.count ? `x${operation.count}` : "";
    const prefix = operation.domain === "order.customer" && operation.targetId ? `NPC ${operation.targetId}` : "";
    return [prefix, art, count].filter(Boolean).join(" ");
  }
  if (operation.rpc === "orderCustomer.finishOrder" || operation.rpc === "orderCustomer.rejectOrder") {
    return operation.targetId ? `NPC ${operation.targetId}` : "";
  }
  if (operation.targetUid !== BigInt(0)) {
    return `UID ${operation.targetUid.toString()}${operation.targetId ? ` · 槽位 ${operation.targetId}` : ""}`;
  }
  if (operation.targetUids.length > 0) {
    return `${operation.targetUids.length} 个候选 UID`;
  }
  const parts = [
    operation.targetId ? operationTargetIdLabel(operation) : "",
    operation.itemId ? itemName(operation.itemId) : "",
    operation.flowerId ? itemName(operation.flowerId) : "",
    operation.count ? `x${operation.count}` : "",
  ].filter(Boolean);
  return parts.join(" ");
}

export function operationCostLabel(operation: PlannedOperation) {
  if (operation.costGates.length > 0) {
    const gateCosts = operation.costGates
      .filter((gate) => Number(gate.required) > 0)
      .map((gate) => {
        const label = gate.label || (gate.itemId ? itemName(gate.itemId) : "成本");
        const available = Number(gate.available);
        const required = Number(gate.required);
        const availability = available > 0 || gate.status === PlanStatus.BLOCKED ? `/${available}` : "";
        return `${label} ${required}${availability}`;
      });
    if (gateCosts.length > 0) {
      return `成本 ${gateCosts.join("、")}`;
    }
  }
  const itemCosts = Object.entries(operation.itemCost)
    .filter(([, count]) => count > 0)
    .map(([id, count]) => `${itemName(Number(id))}x${count}`);
  const costs = [
    operation.goldCost ? `金币 ${operation.goldCost}` : "",
    operation.diamondCost ? `元宝 ${operation.diamondCost}` : "",
    ...itemCosts,
  ].filter(Boolean);
  return costs.length > 0 ? `成本 ${costs.join("、")}` : "";
}

export function operationNoteLabel(operation: PlannedOperation) {
  if (isOperationCooling(operation)) {
    const reason = operation.cooldownReason || "操作冷却中";
    return `${reason}，${operationCooldownRemaining(operation)}后重试`;
  }
  const raw = operation.blockedReasons.length > 0 ? operation.blockedReasons.join("、") : operation.reason;
  return operationReasonLabel(raw);
}

export function isOperationCooling(operation: PlannedOperation) {
  return Number(operation.cooldownUntilMs) > Date.now();
}

export function operationCooldownRemaining(operation: PlannedOperation) {
  const seconds = Math.max(1, Math.ceil((Number(operation.cooldownUntilMs) - Date.now()) / 1000));
  if (seconds < 60) return `${seconds}秒`;
  const minutes = Math.ceil(seconds / 60);
  if (minutes < 60) return `${minutes}分钟`;
  return `${Math.ceil(minutes / 60)}小时`;
}

export function operationLandIds(operation: PlannedOperation) {
  if (operation.landIds.length > 0) return operation.landIds;
  if ((operation.domain.startsWith("farm.") || operation.rpc.startsWith("usrLand.")) && operation.targetId > 0) {
    return [operation.targetId];
  }
  return [];
}

export function operationTargetIdLabel(operation: PlannedOperation) {
  if (operation.domain === "order.customer") return `NPC ${operation.targetId}`;
  if (operation.domain.startsWith("order.flower_art")) return `花架 ${operation.targetId}`;
  if (operation.domain.startsWith("union.")) return `目标 ${operation.targetId}`;
  return `#${operation.targetId}`;
}

export function operationActionLabel(action: string) {
  switch (action) {
    case "harvest":
      return "收获";
    case "plant":
      return "种植";
    case "water":
      return "浇水";
    case "finish":
    case "submit":
      return "提交";
    case "reject":
      return "暂时无货";
    case "claim":
      return "领取";
    case "craft":
      return "制作";
    case "sell":
      return "上架";
    case "sync":
      return "同步";
    case "buy":
      return "购买";
    case "unlock":
      return "解锁";
    case "feed":
      return "喂食";
    case "stroke":
      return "互动";
    case "find_pet":
      return "寻回";
    case "handle_event":
      return "处理";
    default:
      return action;
  }
}

export function operationReasonLabel(reason: string) {
  if (!reason) return "";
  if (reason === "ready land" || reason.includes("initial bloom ready") || reason.includes("elapsed")) return "可收获";
  if (reason === "land is empty") return "空地";
  if (reason.includes("awaiting first water")) return "待浇水";
  if (reason.includes("regrowing")) return "成长中";
  if (reason.includes("not actionable")) return "等待";
  if (reason.includes("no observed")) return "未同步";
  return reason;
}

export function eventCategory(event: Event) {
  if (event.category === "flower_art") return "order";
  if (event.category === "redeem") return "system";
  if (event.category) return event.category;
  if (event.domain) {
    const category = event.domain.split(".")[0];
    if (category === "redeem") return "system";
    return category || "system";
  }
  return "system";
}

/** Race getTaskList completions are frequent; keep only the newest one (events are newest-first). */
export function collapseRaceSyncLogEvents(events: Event[]): Event[] {
  let keptLatestSyncComplete = false;
  return events.filter((event) => {
    if (isRaceSyncPlannedLogEvent(event)) return false;
    if (!isRaceSyncCompleteLogEvent(event)) return true;
    if (keptLatestSyncComplete) return false;
    keptLatestSyncComplete = true;
    return true;
  });
}

export function isRaceSyncLogEvent(event: Event) {
  if (event.domain === "union.race.sync" || event.kind === "race_task_sync") return true;
  const title = eventTitle(event);
  const message = eventMessage(event);
  return title.includes("同步竞赛任务") || message.includes("同步竞赛任务");
}

export function isRaceSyncCompleteLogEvent(event: Event) {
  if (!isRaceSyncLogEvent(event)) return false;
  if (event.kind === "operation_planned") return false;
  if (event.kind === "race_task_sync" || event.kind === "operation_ack") return true;
  const title = eventTitle(event);
  const message = eventMessage(event);
  return title === "同步竞赛任务" || message.includes("同步竞赛任务 完成") || message === "完成";
}

export function isRaceSyncPlannedLogEvent(event: Event) {
  return event.kind === "operation_planned" && isRaceSyncLogEvent(event);
}

export function eventTitle(event: Event) {
  if (event.label) return event.label;
  if (event.kind === "order_satin_finish") return "绸缎订单";
  if (event.kind === "order_decorate_finish") return "建材订单";
  if (event.kind === "waterwheel") return "水车水滴";
  if (event.kind === "free_water") return "限时水滴";
  if (event.domain?.includes("resident.satin")) return "绸缎订单";
  if (event.domain?.includes("resident.decorate")) return "建材订单";
  return [event.domain, event.action].filter(Boolean).join(".") || event.kind || "-";
}

export function eventMessage(event: Event) {
  return event.message || event.payloadJson || "";
}

export function categoryLabel(category: string) {
  switch (category) {
    case "basic":
      return "基础";
    case "water":
      return "水滴";
    case "plant":
      return "种植";
    case "order":
      return "订单";
    case "union":
      return "公会";
    case "race":
      return "竞赛";
    case "activity":
      return "活动";
    case "account":
      return "账号";
    case "system":
      return "系统";
    default:
      return category || "-";
  }
}

export function recommendationLabel(value: string) {
  switch (value) {
    case "harvest":
      return "可采收";
    case "plant":
      return "可种植";
    case "water":
      return "可浇水";
    case "wait":
      return "等待";
    case "unlock":
      return "待开";
    case "locked":
      return "锁定";
    case "unknown":
      return "未知";
    default:
      return value || "未知";
  }
}

export function landStatusLabel(status: string) {
  switch (status) {
    case "opened":
      return "已开";
    case "unopened":
      return "未开";
    case "locked":
      return "锁定";
    default:
      return status || "未知";
  }
}

export function landDisplayNumber(landId: number) {
  if (landId >= 1001 && landId < 2000) return landId - 1000;
  return landId;
}

export function landDisplayName(landId: number) {
  return `#${landDisplayNumber(landId)}`;
}

export function landTimingLabel(land: LandView, status: string) {
  switch (land.recommendation) {
    case "harvest":
      return "可收获";
    case "water":
      return "待浇水";
    case "plant":
      return "待种植";
  }
  if (status !== "opened") {
    return landStatusLabel(status);
  }
  const nextTime = formatUnixTime(land.nextTimeMs);
  if (nextTime !== "-") return `成熟 ${nextTime}`;
  return land.flowerId > 0 ? "成长中" : "待同步";
}

export function fmlLandTimingLabel(land: FmlLandView) {
  switch (land.recommendation) {
    case "harvest":
      return land.pendingHarvest > 0 ? `可收获 ${land.pendingHarvest} 朵` : "可收获";
    case "plant":
      return "待种植";
  }
  const nextTime = formatUnixTime(land.nextMatureMs);
  if (nextTime !== "-") return `下朵 ${nextTime}`;
  if (land.flowerId > 0 && land.stockCap > 0 && land.pendingHarvest >= land.stockCap) {
    return "库存已满";
  }
  return land.flowerId > 0 ? "成长中" : "空地";
}

export function formatTimestamp(ts?: Timestamp) {
  if (!ts) return "-";
  const milliseconds = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(milliseconds));
}

export function formatUnixTime(value?: bigint) {
  const milliseconds = Number(value ?? BigInt(0));
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(milliseconds));
}

export function formatDayId(dayId?: number) {
  if (!dayId || dayId < 20000101 || dayId > 21001231) return dayId ? String(dayId) : "-";
  const year = Math.floor(dayId / 10000);
  const month = Math.floor((dayId % 10000) / 100);
  const day = dayId % 100;
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

export function firstPositiveUnixTime(...values: (bigint | undefined)[]) {
  return values.find((value) => Number(value ?? BigInt(0)) > 0);
}

export function formatCount(value: number | bigint) {
  const numeric = typeof value === "bigint" ? Number(value) : value;
  if (!Number.isFinite(numeric)) return "0";
  return NUMBER_FORMATTER.format(numeric);
}
export function truncateMiddle(value: string, head: number, tail: number) {
  if (value.length <= head + tail + 1) return value;
  return `${value.slice(0, head)}…${value.slice(-tail)}`;
}
