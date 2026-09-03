import { describe, expect, it } from "vitest";

import { RedeemValidation } from "@/gen/mygardenworld/v1/redeem_pb";
import {
  DEFAULT_REDEEM_PAGE_SIZE,
  REDEEM_PAGE_SIZES,
  redeemPageCount,
  redeemValidationLabel,
  resolveRedeemExpiry,
} from "./redeem-utils";

describe("redeem list pagination", () => {
  it("defaults to a compact recent-code page", () => {
    expect(DEFAULT_REDEEM_PAGE_SIZE).toBe(5);
    expect(REDEEM_PAGE_SIZES).toEqual([5, 10]);
  });

  it("keeps the empty list on a single page", () => {
    expect(redeemPageCount(0, DEFAULT_REDEEM_PAGE_SIZE)).toBe(1);
    expect(redeemPageCount(21, DEFAULT_REDEEM_PAGE_SIZE)).toBe(5);
  });
});

describe("resolveRedeemExpiry", () => {
  const now = new Date("2026-08-29T00:00:00Z");

  it("supports minute presets", () => {
    expect(resolveRedeemExpiry("5m", 1, "days", now)?.toISOString()).toBe("2026-08-29T00:05:00.000Z");
  });

  it("supports custom minute-level expiry", () => {
    expect(resolveRedeemExpiry("custom", 3, "minutes", now)?.toISOString()).toBe("2026-08-29T00:03:00.000Z");
  });

  it("represents permanent codes without an expiry", () => {
    expect(resolveRedeemExpiry("permanent", 1, "minutes", now)).toBeNull();
  });

  it("rejects non-positive custom durations", () => {
    expect(() => resolveRedeemExpiry("custom", 0, "minutes", now)).toThrow("请输入有效的过期时间");
  });
});

describe("redeemValidationLabel", () => {
  it("distinguishes local and community verification", () => {
    expect(redeemValidationLabel(RedeemValidation.SUCCESS, false)).toBe("本节点已验证");
    expect(redeemValidationLabel(RedeemValidation.PENDING, false, true)).toBe("社区已验证");
  });

  it("prioritizes expiry over prior validation", () => {
    expect(redeemValidationLabel(RedeemValidation.SUCCESS, true)).toBe("已过期");
  });
});
