import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { AccountSchema } from "@/gen/mygardenworld/v1/account_pb";
import { Channel } from "@/gen/mygardenworld/v1/channel_pb";
import { AccountHealth, AccountStatusSchema } from "@/lib/api/workspace-models";
import AccountListPanel from "./account-list-panel";
import { AccountStatusNotice } from "./account-status-notice";
import { AccountHeaderPanel } from "./account-detail";

describe("account status layout", () => {
  const reason = "账号在其他设备登录，当前会话被替换";
  const expired = create(AccountStatusSchema, {
    health: AccountHealth.SESSION_EXPIRED, lastError: reason,
    diagnostics: { sessionInvalidatedReason: reason, blockedReasons: ["会话已失效", reason] },
  });

  it("shows the cause once without stacking generic session messages", () => {
    const html = renderToStaticMarkup(<AccountStatusNotice status={expired} />);
    expect(html.split(reason)).toHaveLength(2);
    expect(html).not.toContain("会话已失效");
    expect(html).toContain("被挤号后的自动重登由独立设置控制");
    expect(html).not.toContain("<details");
  });

  it("keeps distinct errors available in a bounded collapsed disclosure", () => {
    const status = create(AccountStatusSchema, {
      lastError: "主要错误", diagnostics: { lastOperationError: "订单返回未知错误", blockedReasons: ["原始诊断", "原始诊断"] },
    });
    const html = renderToStaticMarkup(<AccountStatusNotice status={status} />);
    expect(html).toContain("主要错误");
    expect(html).toContain("更多诊断（2）");
    expect(html).toContain("订单返回未知错误");
    expect(html).toContain("max-h-40");
    expect(html).not.toMatch(/<details[^>]*\bopen/);
    expect(html.split("原始诊断")).toHaveLength(2);
  });

  it("retains cooldown and recovery guidance even with a displaced session", () => {
    const status = create(AccountStatusSchema, {
      health: AccountHealth.SESSION_EXPIRED,
      diagnostics: { requestsPaused: true, requestRetryAtMs: BigInt(1791400000000), sessionInvalidatedReason: reason },
    });
    const html = renderToStaticMarkup(<AccountStatusNotice status={status} />);
    expect(html).toContain("后尝试核验，成功后才恢复操作");
    expect(html).toContain("被挤号后的自动重登");
  });

  it("distinguishes local waits, unknown blocks and healthy accounts", () => {
    expect(renderToStaticMarkup(<AccountStatusNotice status={create(AccountStatusSchema)} />)).toBe("");
    const waiting = create(AccountStatusSchema, { diagnostics: { requestsPaused: true, blockedReasons: ["异常恢复已尝试 3/3 次"] } });
    const html = renderToStaticMarkup(<AccountStatusNotice status={waiting} />);
    expect(html).toContain("恢复待处理");
    expect(html).toContain("3/3");
    expect(html).not.toContain("后尝试核验");
    expect(renderToStaticMarkup(<AccountStatusNotice status={create(AccountStatusSchema, { health: AccountHealth.BLOCKED })} />)).toContain("暂未收到具体原因");
  });

  it("separates account selection from run controls and keeps area text intact", () => {
    const account = create(AccountSchema, { id: BigInt(1), name: "小云朵", gsIdx: 2482, channel: Channel.IOS });
    const html = renderToStaticMarkup(<AccountListPanel accounts={[account]} statuses={new Map([["1", expired]])}
      selectedAccountId="1" loading={false} quota={null} busyAutomationAccountId="" busyBulkAutomation=""
      onRefresh={vi.fn()} onAdd={vi.fn()} onRedeem={vi.fn()} onSelect={vi.fn()}
      onAutomationToggle={vi.fn()} onAutomationStop={vi.fn()} onBulkStart={vi.fn()} onBulkPause={vi.fn()} />);
    const selection = html.match(/<button[^>]*aria-label="查看账号 小云朵"[^>]*>([\s\S]*?)<\/button>/)?.[1];
    expect(selection).toBeDefined();
    expect(selection).toContain('class="whitespace-nowrap">第2482区');
    expect(selection).toContain("会话失效");
    expect(selection).not.toContain("启动并上线");
    expect(selection).not.toContain("<button");
    expect(html).toContain('aria-label="小云朵的运行操作"');
    expect(html).toContain('aria-label="停止并离线"');
  });

  it("lets header controls wrap independently of a long account name", () => {
    const account = create(AccountSchema, { name: "很长的账号名称".repeat(12), gsIdx: 2482, channel: Channel.IOS });
    const html = renderToStaticMarkup(<AccountHeaderPanel account={account} status={expired} viewsLoading={false} busyAction=""
      onBack={vi.fn()} onRefresh={vi.fn()} onAction={async () => {}} onDelete={vi.fn()} onReauthenticate={vi.fn()} />);
    expect(html).toContain("@container/account-header");
    expect(html).toContain("@lg/account-header:flex-row");
    expect(html).toContain('aria-label="账号操作"');
    expect(html).toContain('aria-label="重新登录／更新凭据"');
    expect(html).toContain("flex-wrap");
  });
});
