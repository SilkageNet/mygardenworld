import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AccountSchema } from "@/gen/mygardenworld/v1/account_pb";
import { AccountStatusSchema } from "@/lib/api/workspace-models";
import { accountStatusIssues, HealthBadge } from "./dashboard-utils";

describe("account protection status", () => {
  const account = create(AccountSchema);
  it("keeps overdue recovery protected until the server confirms it", () => {
    const status = create(AccountStatusSchema, { connected: true, diagnostics: { requestsPaused: true, requestRetryAtMs: BigInt(1) } });
    expect(renderToStaticMarkup(<HealthBadge account={account} status={status} />)).toContain("请求保护中");
    expect(accountStatusIssues(status).join()).toContain("核验成功才恢复");
  });
  it("distinguishes expired sessions from an online protection gate", () => {
    const status = create(AccountStatusSchema, { diagnostics: { sessionInvalidatedReason: "其他设备登录" } });
    expect(renderToStaticMarkup(<HealthBadge account={account} status={status} />)).toContain("会话失效");
    expect(accountStatusIssues(status).join()).toContain("手动登录");
  });
  it("shows local admission waits without promising another timed retry", () => {
    const reason = "缓存会话不可用，且未允许自动重新登录；请手动重新登录";
    const status = create(AccountStatusSchema, { diagnostics: { requestsPaused: true, blockedReasons: [reason] } });
    expect(renderToStaticMarkup(<HealthBadge account={account} status={status} />)).toContain("恢复待处理");
    const issues = accountStatusIssues(status).join();
    expect(issues).toContain(reason);
    expect(issues).not.toContain("后尝试核验");
    expect(issues).toContain("不会因本地等待反复延长冷却");
  });
});
