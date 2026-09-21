import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { AccountSchema } from "@/gen/mygardenworld/v1/account_pb";
import { AccountStatusSchema, AccountDeletionProgressSchema } from "@/lib/api/workspace-models";
import { AccountDeletionProgressDetails } from "./account-deletion-progress";
import { accountConnected, HealthBadge } from "@/components/dashboard/dashboard-utils";
import AccountListPanel from "./account-list-panel";
import { accountDeleting, reconcileAccountDeletions } from "./account-deletion";

describe("durable account deletion", () => {
  it("shows committed progress, stable times, retry cause and stalled warning", () => {
    const progress = create(AccountDeletionProgressSchema, {
      removedRows: BigInt(12345), phase: "event_log", errorKind: "disk_full", failures: 3,
      trackingStartedMs: BigInt(1_790_000_000_000), lastProgressMs: BigInt(1_790_000_000_100),
      attemptMs: BigInt(1_790_000_900_000), retryAtMs: BigInt(1_790_000_960_000), stalled: true, batchSize: 125,
    });
    const html = renderToStaticMarkup(<AccountDeletionProgressDetails progress={progress} />);
    for (const text of ["12,345", "清理运行日志", "磁盘空间不足", "连续失败 3 次", "下次尝试不早于", "超过 15 分钟", "升级前已清理的条数不计入"]) expect(html).toContain(text);
    expect(html).not.toContain("%");
    expect(renderToStaticMarkup(<AccountDeletionProgressDetails />)).toContain("正在读取清理进度");
  });

  it("does not render unknown diagnostic strings as raw errors", () => {
    const progress = create(AccountDeletionProgressSchema, { errorKind: "/private/secret", phase: "secret SQL" });
    const html = renderToStaticMarkup(<AccountDeletionProgressDetails progress={progress} />);
    expect(html).not.toContain("secret");
    expect(html).toContain("服务进程日志");
    expect(html).not.toContain("没有新的清理进展");
  });
  it("keeps pending state across stale status replies and waits for collection confirmation", () => {
    const account = create(AccountSchema, { id: BigInt(1), deletionPending: true });
    const stale = create(AccountStatusSchema, { accountId: BigInt(1), connected: true });
    expect(accountDeleting(account, stale)).toBe(true);
    expect(accountConnected(account, stale)).toBe(false);
    expect(reconcileAccountDeletions([account], new Map([["1", stale]]))[0].deletionPending).toBe(true);
    expect(reconcileAccountDeletions([account], new Map())).toEqual([account]);
  });

  it("accepts pushed deletion and retry status without marking the account deleted", () => {
    const account = create(AccountSchema, { id: BigInt(1) });
    const status = create(AccountStatusSchema, { accountId: BigInt(1), deletionPending: true, deletionFailed: true });
    const accounts = reconcileAccountDeletions([account], new Map([["1", status]]));
    expect(accounts).toHaveLength(1);
    expect(accounts[0].deletionFailed).toBe(true);
    expect(renderToStaticMarkup(<HealthBadge account={accounts[0]} status={status} />)).toContain("清理待重试");
    status.deletionFailed = false;
    expect(reconcileAccountDeletions(accounts, new Map([["1", status]]))[0].deletionFailed).toBe(false);
  });

  it("keeps pending accounts visible with disabled start controls", () => {
    const account = create(AccountSchema, { id: BigInt(1), name: "清理中的账号", deletionPending: true });
    const html = renderToStaticMarkup(<AccountListPanel accounts={[account]} statuses={new Map()}
      selectedAccountId="1" loading={false} quota={null} busyAutomationAccountId="" busyBulkAutomation=""
      onRefresh={vi.fn()} onAdd={vi.fn()} onRedeem={vi.fn()} onSelect={vi.fn()}
      onAutomationToggle={vi.fn()} onAutomationStop={vi.fn()} onBulkStart={vi.fn()} onBulkPause={vi.fn()} />);
    expect(html).toContain("清理中的账号");
    expect(html).toContain("删除中");
    expect(html).toMatch(/<button[^>]*(?:aria-label="启动并上线"[^>]*disabled|disabled[^>]*aria-label="启动并上线")/);
  });
});
