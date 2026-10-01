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
});
