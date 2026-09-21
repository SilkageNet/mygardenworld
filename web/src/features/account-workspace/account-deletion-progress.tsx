import type { AccountDeletionProgress } from "@/lib/api/workspace-models";

const phases: Record<string, string> = {
  queued: "等待后台清理", wait_lifecycle: "等待账号操作结束", stop_runner: "停止账号连接",
  drain_game_work: "等待游戏请求结束", wait_writer: "等待数据库写连接", begin_transaction: "等待数据库写锁",
  event_log: "清理运行日志", operation_log: "清理操作日志", redeem_attempts: "清理兑换记录",
  notification_outbox: "清理通知记录", notification_incidents: "清理通知状态", commit: "提交本批清理", finalize: "移除账号记录",
};

const errors: Record<string, string> = {
  busy: "数据库正被其他操作占用", timeout: "本阶段处理超时", cancelled: "本次清理被中断",
  disk_full: "磁盘空间不足，请先释放空间", io: "磁盘读写失败，请检查存储状态",
  permission: "数据库写入权限不足，请检查文件和目录权限", corrupt: "数据库异常，请停止服务并检查备份",
  database: "数据库清理失败，请查看服务进程日志中的 account deletion deferred",
};

function timeLabel(ms: bigint) {
  return ms > BigInt(0) ? new Date(Number(ms)).toLocaleString("zh-CN", { hour12: false }) : "尚未记录";
}

// Use absolute timestamps, not a countdown or a misleading percentage. The
// total remaining history is intentionally not scanned during every update.
export function AccountDeletionProgressDetails({ progress }: { progress?: AccountDeletionProgress }) {
  if (!progress) return <p className="text-sm text-muted-foreground">正在读取清理进度…</p>;
  return <div className="space-y-3 text-sm">
    <dl className="grid gap-x-6 gap-y-2 rounded-lg border bg-background/40 p-3 sm:grid-cols-2">
      <div><dt className="text-muted-foreground">当前阶段</dt><dd>{phases[progress.phase] ?? "后台处理中"}</dd></div>
      <div><dt className="text-muted-foreground">已记录清理</dt><dd className="tabular-nums">{progress.removedRows.toLocaleString("zh-CN")} 条</dd></div>
      <div><dt className="text-muted-foreground">最后有进展</dt><dd>{timeLabel(progress.lastProgressMs)}</dd></div>
      <div><dt className="text-muted-foreground">最近一次尝试</dt><dd>{timeLabel(progress.attemptMs)}</dd></div>
    </dl>
    {progress.errorKind && <div className="rounded-lg border border-amber-400/40 bg-amber-50/50 p-3 text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
      <p>{errors[progress.errorKind] ?? errors.database} · 连续失败 {progress.failures} 次</p>
      {progress.retryAtMs > BigInt(0) && <p className="mt-1 text-xs">下次尝试不早于 {timeLabel(progress.retryAtMs)}</p>}
    </div>}
    {progress.stalled && <p className="text-amber-800 dark:text-amber-300">已超过 15 分钟没有新的清理进展。请检查磁盘空间、数据库占用及服务进程日志；无需反复点击删除。</p>}
    <details className="text-xs text-muted-foreground">
      <summary className="cursor-pointer">诊断信息</summary>
      <p className="mt-2 break-words">计划批次 {progress.batchSize} 条 · 最近等待 {progress.waitMs.toString()} ms · 执行与提交 {progress.workMs > BigInt(0) ? `${progress.workMs} ms` : "未采样"}</p>
      <p className="mt-1">进度记录始于 {timeLabel(progress.trackingStartedMs)}；升级前已清理的条数不计入。进度按已提交的批次累计，数据库文件缩小可能滞后。</p>
    </details>
  </div>;
}
