import { describe, expect, it } from "vitest";

import {
  formatIntList,
  parseBigInt,
  parseIntList,
  parseNumber,
  safeBigIntToNumber,
  safeNumberToBigInt,
  toggleNumber,
} from "@/components/dashboard/policy-controls";

describe("policy numeric normalization", () => {
  it("clamps ordinary and bigint values", () => {
    expect(parseNumber("8.9", 2)).toBe(8);
    expect(parseNumber("invalid", 2)).toBe(2);
    expect(parseBigInt("-1", 0)).toBe(BigInt(0));
    expect(parseBigInt("9007199254740993", 0)).toBe(BigInt("9007199254740993"));
  });

  it("converts protobuf bigint fields without unsafe overflow", () => {
    expect(safeBigIntToNumber(BigInt(12), 0)).toBe(12);
    expect(safeBigIntToNumber(BigInt("999999999999999999"), 0)).toBe(Number.MAX_SAFE_INTEGER);
    expect(safeNumberToBigInt(Number.POSITIVE_INFINITY, 7)).toBe(BigInt(7));
  });
});

describe("policy id-list normalization", () => {
  it("deduplicates and ignores invalid ids", () => {
    expect(parseIntList("3, 2，3、0 bad 7")).toEqual([3, 2, 7]);
    expect(formatIntList([3, 2, 7])).toBe("3, 2, 7");
  });

  it("toggles values while preserving a stable sort", () => {
    expect(toggleNumber([3, 1], 2)).toEqual([1, 2, 3]);
    expect(toggleNumber([1, 2, 3], 2)).toEqual([1, 3]);
  });
});
