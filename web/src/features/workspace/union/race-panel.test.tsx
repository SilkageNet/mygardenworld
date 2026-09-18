import { create } from "@bufbuild/protobuf";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { FmlRaceTaskSchema } from "@/gen/mygardenworld/v1/workspace_union_pb";
import { FmlRaceTaskCard } from "./race-panel";

describe("race task status cards", () => {
  it.each([
    ["", true, "ready", "emerald", "现在可抢", true],
    ["冷却中", true, "cooldown", "amber", "后可抢", false],
    ["冷却中", false, "blocked", "muted", "需先完成当前任务", false],
    ["已被接取", true, "claimed", "sky", "已被接取", false],
    ["优先级为0", true, "blocked", "muted", "优先级为0", false],
    ["", false, "blocked", "muted", "需先完成当前任务", false],
  ])("keeps readable status and existing actions for %s", (reason, canTake, tone, color, label, hasTake) => {
    const onTake = vi.fn();
    const onDelete = vi.fn();
    const html = renderToStaticMarkup(<FmlRaceTaskCard index={1}
      task={create(FmlRaceTaskSchema, { takeSkipReason: reason, appearTimeMs: BigInt(reason === "冷却中" ? 10_000 : 0), score: 40 })}
      nowMs={9_000} canTake={canTake} canDelete={false} showDelete={false}
      deleteBlockedReason="" takeBusy={false} deleteBusy={false} onTake={onTake} onDelete={onDelete} />);
    expect(html).toContain(`data-task-state="${tone}"`);
    expect(html).toContain(`bg-${color}`);
    expect(html).toContain(`dark:bg-${color}`);
    expect(html).toContain(label);
    expect(html.includes("手动抢")).toBe(hasTake);
    expect(html).not.toContain("animate-");
    expect(onTake).not.toHaveBeenCalled();
    expect(onDelete).not.toHaveBeenCalled();
  });

  it.each(["他人已升级", "当前账号身份尚未同步", "目标花卉未培养"])("shows %s separately from refresh time without a take action", (reason) => {
    for (const nowMs of [9_000, 10_000]) {
      const html = renderToStaticMarkup(<FmlRaceTaskCard index={18}
        task={create(FmlRaceTaskSchema, { takeSkipReason: reason, appearTimeMs: BigInt(10_000), score: 50, isUpgrade: true })}
        nowMs={nowMs} canTake={true} canDelete={false} showDelete={false}
        deleteBlockedReason="" takeBusy={false} deleteBusy={false} onTake={vi.fn()} onDelete={vi.fn()} />);
      expect(html).toContain('data-task-state="blocked"');
      expect(html).toContain("bg-muted");
      expect(html).not.toContain("bg-amber");
      expect(html).toContain(`不可抢：${reason}`);
      expect(html.includes("后刷新")).toBe(nowMs < 10_000);
      expect(html).not.toContain("后可抢");
      expect(html).not.toContain("手动抢");
    }
  });

  it("allows an unclaimed upgraded holly task with no upgrade member once cooldown ends", () => {
    const task = create(FmlRaceTaskSchema, {
      taskType: 3036, taskLabel: "种植收获", targetLabel: "轮生冬青", score: 42, targetCnt: 560,
      isUpgrade: true, upgradeUid: BigInt(0), appearTimeMs: BigInt(10_000), takeSkipReason: "冷却中，00:00:10 后可接",
    });
    for (const nowMs of [9_000, 10_000]) {
      const ready = nowMs >= 10_000;
      const html = renderToStaticMarkup(<FmlRaceTaskCard index={39} task={task}
        nowMs={nowMs} canTake={true} canDelete={false} showDelete={false}
        deleteBlockedReason="" takeBusy={false} deleteBusy={false} onTake={vi.fn()} onDelete={vi.fn()} />);
      expect(html).toContain("轮生冬青");
      expect(html).toContain("已升级");
      expect(html).toContain(`data-task-state="${ready ? "ready" : "cooldown"}"`);
      expect(html.includes("手动抢")).toBe(ready);
      expect(html).not.toContain("升级归属不明");
    }
  });
});
