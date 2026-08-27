import { describe, expect, it } from "vitest";
import { activityIsVisible } from "./activity-overview";

describe("activityIsVisible", () => {
  it.each([1, 2, 3])("shows a discovered activity in phase %s", (phase) => {
    expect(activityIsVisible({ found: true, observed: true, phase })).toBe(true);
  });

  it.each([
    undefined,
    { found: false, observed: true, phase: 0 },
    { found: true, observed: true, phase: 0 },
    { found: true, observed: true, phase: 4 },
  ])("keeps activity details inactive for %j", (activity) => {
    expect(activityIsVisible(activity)).toBe(false);
  });
});
