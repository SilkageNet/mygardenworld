import { create, fromJson, toJson } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import { PendingTaskViewSchema, PlanStatus, TaskExecutionFeature } from "@/gen/mygardenworld/v1/workspace_common_pb";
import PolicyPanel from "@/features/workspace/shared/policy-panel";
import { createPolicyEditor } from "@/features/workspace/shared/policy-editor";
import { TaskOrderMonitorPanel } from "./status-panels";

describe("customer floral-coin filter", () => {
  it("clears a saved filter without changing other customer preferences", () => {
    const policy = create(PolicySchema, { order: { customer: { enabled: true, dailyLimit: 12, exactFloralCoin: BigInt(3) } } });
    const changed = vi.fn();
    createPolicyEditor(policy, changed).updateCustomer({ exactFloralCoin: undefined });
    const result = fromJson(PolicySchema, toJson(PolicySchema, changed.mock.calls[0][0]));
    expect(result.order?.customer?.exactFloralCoin).toBeUndefined();
    expect(result.order?.customer?.dailyLimit).toBe(12);
    expect(result.order?.customer?.enabled).toBe(true);
  });
  it.each([undefined, BigInt(0), BigInt(3)])("preserves optional amount %s in policy JSON", (amount) => {
    const policy = create(PolicySchema, { order: { customer: { exactFloralCoin: amount } } });
    const restored = fromJson(PolicySchema, toJson(PolicySchema, policy));
    expect(restored.order?.customer?.exactFloralCoin).toBe(amount);
  });

  it("keeps filter settings accessible without a live account", () => {
    const html = renderToStaticMarkup(<PolicyPanel
      policy={create(PolicySchema, { order: { customer: { exactFloralCoin: BigInt(3) } } })}
      section="orders" basicView={null} garden={null} orders={null} unionView={null} warehouse={null}
      capabilities={[]} loading={false} saving={false} message="" onPolicyChange={vi.fn()} onSave={vi.fn()} />);
    expect(html).toContain("指定花坊币奖励");
    expect(html).toContain("花坊币等于");
    expect(html).toContain("保留订单可能占满顾客名额");
  });

  it.each([BigInt(2), undefined])("shows reward and skip reason for %s without advertising it as executable", (reward) => {
    const html = renderToStaticMarkup(<TaskOrderMonitorPanel
      tasks={[create(PendingTaskViewSchema, { category: "顾客订单", id: "1", title: "测试订单", status: PlanStatus.SKIPPED,
        executionFeature: TaskExecutionFeature.CUSTOMER_ORDER, autoCompletionSupported: true, floralCoinReward: reward,
        automationSkipReason: "奖励不匹配，保留订单" })]}
      videoOrders={[]} policy={create(PolicySchema, { automationEnabled: true, order: { customer: { enabled: true } } })} />);
    expect(html).toContain("已跳过");
    expect(html).toContain("奖励不匹配，保留订单");
    expect(html).toContain(reward === undefined ? "待确认" : "花坊币奖励：");
    expect(html).not.toContain("可处理");
  });
});
