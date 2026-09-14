import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import { UnionViewSchema } from "@/lib/api/workspace-models";
import PolicyPanel from "./policy-panel";

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
