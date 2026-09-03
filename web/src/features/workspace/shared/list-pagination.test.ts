import { describe, expect, it } from "vitest";
import { resolveListPageWindow } from "./list-pagination";

describe("resolveListPageWindow", () => {
  it("keeps an unloaded requested page on the newest available page", () => {
    expect(resolveListPageWindow(1, 5, 5)).toEqual({
      pageIndex: 0,
      loadedPages: 1,
      start: 0,
      end: 5,
    });
  });

  it("moves to a requested page after its items are loaded", () => {
    expect(resolveListPageWindow(1, 10, 5)).toEqual({
      pageIndex: 1,
      loadedPages: 2,
      start: 5,
      end: 10,
    });
  });

  it("clamps stale pages after the list is replaced", () => {
    expect(resolveListPageWindow(4, 2, 5)).toEqual({
      pageIndex: 0,
      loadedPages: 1,
      start: 0,
      end: 2,
    });
  });

  it("rejects invalid page sizes", () => {
    expect(() => resolveListPageWindow(0, 1, 0)).toThrow("pageSize must be a positive integer");
  });
});
