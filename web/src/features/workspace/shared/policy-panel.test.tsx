import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import { UnionViewSchema } from "@/lib/api/workspace-models";
import PolicyPanel from "./policy-panel";

it("exposes independent pearl collection and interval while offline", () => {
  const policy = create(PolicySchema, {basic:{pearl:{autoHireEnabled:true,collectEnabled:false,collectIntervalSeconds:600}}});
  const html = renderToStaticMarkup(<PolicyPanel policy={policy} section="basic"
    basicView={null} garden={null} orders={null} warehouse={null} unionView={null}
    capabilities={[]} loading={false} saving={false} message="" onPolicyChange={vi.fn()} onSave={vi.fn()} />);
  expect(html).toContain("自动收取珍珠产出");
  expect(html).toContain("产出收取间隔（秒）");
  expect(html).toContain('value="600"');
  expect(html).toContain("默认关闭，与雇佣、免费领取、开珍珠独立");
  expect(html).toContain("间隔内不影响种植和订单");
});

it("shows independent recovery opt-in and displacement warning while offline", () => {
  const html = renderToStaticMarkup(<PolicyPanel policy={create(PolicySchema)} section="basic"
    basicView={null} garden={null} orders={null} warehouse={null} unionView={null}
    capabilities={[]} loading={false} saving={false} message="" onPolicyChange={vi.fn()} onSave={vi.fn()} />);
  expect(html).toContain("异常后重新认证");
  expect(html).toContain("异常恢复最多尝试次数");
  expect(html).toContain("登录且业务核验通过才清零");
  expect(html).toContain("默认关闭");
  expect(html).toContain("可能挤下手机端");
  expect(html).toContain("与自动挤号设置独立");
  expect(html).toContain("不先重试旧会话");
  expect(html).toContain("暂停/启动不会重置冷却和额度");
  expect(html).toContain("关闭时仍可手动登录");
  expect(html).toContain("不受自动冷却、间隔和次数上限限制");
  expect(html).toContain("失败会直接提示");
  expect(html).toContain("不反复延长冷却");
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
