import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { PearlHireStatusViewSchema } from "@/gen/mygardenworld/v1/workspace_basic_pb";
import { PearlHirePanel } from "./status-panels";

describe("pearl hire protection", () => {
  it("shows a scoped lock even without slot observations", () => {
    const pearlHire = create(PearlHireStatusViewSchema, { sessionLocked: true, sessionLockReason: "扣券结果未知" });
    const html = renderToStaticMarkup(<PearlHirePanel pearlHire={pearlHire} />);
    for (const text of ["雇佣已保护暂停", "扣券结果未知", "不影响其他任务", "不要反复重登录"]) expect(html).toContain(text);
  });
  it("does not report a lock for normal candidate waits", () => {
    expect(renderToStaticMarkup(<PearlHirePanel pearlHire={create(PearlHireStatusViewSchema)} />)).not.toContain("雇佣已保护暂停");
  });
});
