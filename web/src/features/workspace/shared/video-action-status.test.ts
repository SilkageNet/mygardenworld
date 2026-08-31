import { describe, expect, it } from "vitest";
import { VideoActionState, type VideoActionStatusView } from "@/lib/api/workspace-models";
import { effectiveVideoActionState, videoActionRewardLabel, videoActionStatusLabel } from "./video-action-status";

function action(overrides: Partial<VideoActionStatusView> = {}): VideoActionStatusView {
  return {
    $typeName: "mygardenworld.v1.VideoActionStatusView",
    id: "basic.test",
    label: "测试提醒",
    observed: true,
    state: VideoActionState.READY,
    used: 0,
    limit: 0,
    availableAtMs: BigInt(0),
    expiresAtMs: BigInt(0),
    rewards: [],
    detail: "",
    ...overrides,
  };
}

describe("video action status labels", () => {
  it("distinguishes ready, cooldown, active and unknown states", () => {
    expect(videoActionStatusLabel(action(), 1000)).toBe("可尝试");
    expect(videoActionStatusLabel(action({ state: VideoActionState.COOLDOWN, availableAtMs: BigInt(62_000) }), 1000)).toBe("还有 1:01");
    expect(videoActionStatusLabel(action({ state: VideoActionState.ACTIVE, expiresAtMs: BigInt(3_661_000) }), 1000)).toBe("剩余 1:01:00");
    expect(videoActionStatusLabel(action({ state: VideoActionState.UNKNOWN, observed: false }), 1000)).toBe("待同步");
  });

  it("uses deterministic hydration labels and rolls expired states forward locally", () => {
    expect(videoActionStatusLabel(action({ state: VideoActionState.ACTIVE, expiresAtMs: BigInt(10_000) }), 0)).toBe("生效中");
    expect(videoActionStatusLabel(action({ state: VideoActionState.COOLDOWN, availableAtMs: BigInt(10_000) }), 0)).toBe("冷却中");
    expect(effectiveVideoActionState(action({ state: VideoActionState.COOLDOWN, availableAtMs: BigInt(10_000) }), 10_001)).toBe(VideoActionState.READY);
    expect(videoActionStatusLabel(action({ state: VideoActionState.ACTIVE, expiresAtMs: BigInt(10_000) }), 10_001)).toBe("可尝试");
    expect(videoActionStatusLabel(action({ state: VideoActionState.EXHAUSTED, availableAtMs: BigInt(10_000) }), 10_001)).toBe("可尝试");
  });

  it("formats compact catalog-backed rewards", () => {
    expect(videoActionRewardLabel(action({
      rewards: [
        { $typeName: "mygardenworld.v1.VideoActionRewardView", itemId: 1006, itemName: "珍珠", count: 500 },
        { $typeName: "mygardenworld.v1.VideoActionRewardView", itemId: 1, itemName: "元宝", count: 6 },
      ],
    }))).toBe("珍珠 ×500 · 元宝 ×6");
  });
});
