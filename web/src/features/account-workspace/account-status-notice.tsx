import { AlertTriangle } from "lucide-react";
import { AccountHealth, type AccountStatus } from "@/lib/api/workspace-models";
import { formatUnixTime } from "@/components/dashboard/dashboard-utils";

// Presentation only: retain distinct server diagnostics, without repeating
// generic session labels around a more specific cause. No recovery decisions.
export function AccountStatusNotice({ status }: { status?: AccountStatus }) {
  const diagnostics = status?.diagnostics;
  const expired = Boolean(diagnostics?.sessionInvalidatedReason) || status?.health === AccountHealth.SESSION_EXPIRED;
  const paused = diagnostics?.requestsPaused;
  const timedRetry = paused && diagnostics.requestRetryAtMs > BigInt(0);
  const reasons = [...new Set([
    diagnostics?.sessionInvalidatedReason,
    ...(paused ? diagnostics?.blockedReasons ?? [] : []),
    status?.lastError,
    diagnostics?.lastOperationError,
    ...(diagnostics?.blockedReasons ?? []),
  ].map((reason) => reason?.trim()).filter((reason): reason is string => Boolean(reason)))];
  if (!expired && !paused && status?.health !== AccountHealth.BLOCKED && reasons.length === 0) return null;

  const specificReasons = expired
    ? reasons.filter((reason) => !["会话已失效", "会话失效"].includes(reason))
    : reasons;
  const title = expired ? "会话失效" : paused ? (timedRetry ? "请求保护中" : "恢复待处理") : "运行异常";
  const primary = specificReasons[0] || (expired ? "游戏会话已失效，任务已停止。" : paused ? "账号游戏请求已暂停。" : "账号处于异常状态，暂未收到具体原因。");
  const guidance = [
    expired ? "可手动重新登录；被挤号后的自动重登由独立设置控制。" : null,
    paused ? timedRetry
        ? `${formatUnixTime(diagnostics.requestRetryAtMs)} 后尝试核验，成功后才恢复操作。`
        : "请检查恢复设置或手动处理；本地等待不会延长服务端冷却。"
      : null,
  ].filter((message): message is string => Boolean(message));
  const details = specificReasons.slice(1);

  return <section aria-label="账号异常提示" className="min-w-0 rounded-md border border-destructive/20 bg-destructive/5 px-3 py-2.5">
    <div className="flex items-start gap-2.5">
      <AlertTriangle aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-destructive" />
      <div className="min-w-0 flex-1 space-y-1">
        <h2 className="text-xs font-semibold text-destructive">{title}</h2>
        <p className="dark-scrollbar max-h-24 overflow-y-auto text-sm leading-relaxed [overflow-wrap:anywhere]">{primary}</p>
        {guidance.map((message) => <p key={message} className="text-xs leading-relaxed text-muted-foreground">{message}</p>)}
        {details.length > 0 && <details className="group pt-1">
          <summary className="w-fit cursor-pointer rounded-sm text-xs text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring">更多诊断（{details.length}）</summary>
          <ul className="dark-scrollbar mt-2 max-h-40 space-y-1.5 overflow-y-auto border-l border-border pl-3 text-xs leading-relaxed text-muted-foreground [overflow-wrap:anywhere]">
            {details.map((reason) => <li key={reason}>{reason}</li>)}
          </ul>
        </details>}
      </div>
    </div>
  </section>;
}
