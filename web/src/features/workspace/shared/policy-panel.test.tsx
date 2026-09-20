import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import { UnionViewSchema } from "@/lib/api/workspace-models";
import PolicyPanel from "./policy-panel";

it("shows independent recovery opt-in and displacement warning while offline", () => {
  const html = renderToStaticMarkup(<PolicyPanel policy={create(PolicySchema)} section="basic"
    basicView={null} garden={null} orders={null} warehouse={null} unionView={null}
    capabilities={[]} loading={false} saving={false} message="" onPolicyChange={vi.fn()} onSave={vi.fn()} />);
  expect(html).toContain("5000 异常后允许重新登录");
  expect(html).toContain("默认关闭");
  expect(html).toContain("可能挤下手机端");
  expect(html).toContain("与自动挤号设置独立");
  expect(html).toContain("不先重试旧会话");
  expect(html).toContain("暂停/启动不会重置冷却和额度");
});

describe("race upgrade configuration explains spending permission", () => {
  it.each([0, 100])("keeps budget next to upgrade and never invents permission: %i", (budget) => {
    const policy = create(PolicySchema, { union: { race: { upgradeTask: true, maxSpendDiamond: BigInt(budget) } } });
    const onChange = vi.fn();
    const onSave = vi.fn();
    const html = renderToStaticMarkup(<PolicyPanel policy={policy} section="union"
      basicView={null} garden={null} orders={null} warehouse={null}
      unionView={create(UnionViewSchema, { race: { autoUpgradeStatus: "当前任务已提交升级，结果尚未确认" } })}
      capabilities={[]} loading={false} saving={false} message="" onPolicyChange={onChange} onSave={onSave} />);
    expect(html.indexOf("自动升级任务")).toBeLessThan(html.indexOf("单次升级元宝上限"));
    expect(html.indexOf("单次升级元宝上限")).toBeLessThan(html.indexOf("删除低分任务"));
    expect(html.includes("已打开升级开关，但预算为 0，不会执行升级")).toBe(budget === 0);
    expect(html).toContain("当前执行状态（已保存配置）");
    expect(html).toContain("当前任务已提交升级，结果尚未确认");
    expect(onChange).not.toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });
});

it("explains that upgrade member exclusion does not imply task occupancy", () => {
  const html = renderToStaticMarkup(<PolicyPanel policy={create(PolicySchema)} section="union"
    basicView={null} garden={null} orders={null} warehouse={null} unionView={null}
    capabilities={[]} loading={false} saving={false} message="" onPolicyChange={vi.fn()} onSave={vi.fn()} />);
  expect(html).toContain("仅排除明确由其他成员升级的任务");
  expect(html).toContain("未记录升级人的任务仍按其余条件筛选");
  expect(html).toContain("已被接取的任务始终跳过");
  expect(html).not.toContain("已升级但归属不明的任务");
  expect(html).toContain("基础次数加游戏内已购买次数");
  expect(html).toContain("同步到新增可用次数后继续，不会自动购买次数");
});
