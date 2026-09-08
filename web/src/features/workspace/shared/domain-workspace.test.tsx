import { create } from "@bufbuild/protobuf";
import { useState } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { PolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import { UnionViewSchema } from "@/lib/api/workspace-models";
import { EMPTY_ACCOUNT_VIEWS } from "@/features/workspace/model";
import DomainWorkspace, { type WorkspaceProps } from "./domain-workspace";

vi.mock("react", async (original) => ({
  ...await original<typeof import("react")>(),
  useState: vi.fn(),
}));

const { renderPolicy } = vi.hoisted(() => ({ renderPolicy: vi.fn() }));
vi.mock("./policy-panel", () => ({
  default: (props: unknown) => {
    renderPolicy(props);
    return <div>策略编辑表单</div>;
  },
}));

describe("settings are independent of live game membership", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useState).mockReturnValue(["status", vi.fn()]);
  });

  const views = [
    { name: "never connected", union: null },
    { name: "membership pending", union: create(UnionViewSchema) },
    { name: "not a member", union: create(UnionViewSchema, { membershipObserved: true }) },
    { name: "member", union: create(UnionViewSchema, { membershipObserved: true, inUnion: true }) },
  ];

  it.each(views)("keeps settings accessible: $name", ({ union }) => {
    const props: WorkspaceProps = {
      views: { ...EMPTY_ACCOUNT_VIEWS, union },
      policy: create(PolicySchema), capabilities: [], policyLoading: false,
      savingPolicy: false, policyMessage: "",
      onPolicyChange: vi.fn(), onPolicySave: vi.fn(),
    };
    const render = () => renderToStaticMarkup(<DomainWorkspace section="union" props={props} statusContent={<div>状态内容</div>} />);
    expect(render()).toContain("设置</button>");
    vi.mocked(useState).mockReturnValue(["settings", vi.fn()]);
    const html = render();
    expect(html).toContain("策略编辑表单");
    expect(html).not.toContain("状态内容");
    expect(html).toContain("保存不会主动开启自动化");
    expect(renderPolicy).toHaveBeenCalledWith(expect.objectContaining({
      policy: props.policy, unionView: union, onSave: props.onPolicySave,
    }));
    expect(props.onPolicySave).not.toHaveBeenCalled();
    expect(props.onPolicyChange).not.toHaveBeenCalled();
  });

  it("keeps workspaces without policy sections status-only", () => {
    vi.mocked(useState).mockReturnValue(["settings", vi.fn()]);
    const props: WorkspaceProps = {
      views: EMPTY_ACCOUNT_VIEWS, policy: null, capabilities: [], policyLoading: false,
      savingPolicy: false, policyMessage: "", onPolicyChange: vi.fn(), onPolicySave: vi.fn(),
    };
    const html = renderToStaticMarkup(<DomainWorkspace props={props} statusContent={<div>状态内容</div>} />);
    expect(html).toContain("状态内容");
    expect(html).not.toContain("设置</button>");
    expect(renderPolicy).not.toHaveBeenCalled();
  });
});
