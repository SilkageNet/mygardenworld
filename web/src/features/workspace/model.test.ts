import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { EventSchema } from "@/gen/mygardenworld/v1/workspace_common_pb";
import { GardenViewSchema } from "@/gen/mygardenworld/v1/workspace_garden_pb";
import { BasicViewSchema } from "@/gen/mygardenworld/v1/workspace_basic_pb";
import {
  WorkspaceDomain,
  WorkspaceHistoryItemSchema,
  WorkspacePatchSchema,
  WorkspaceStateSchema,
} from "@/gen/mygardenworld/v1/workspace_pb";
import {
  applyWorkspacePatch,
  EMPTY_ACCOUNT_VIEWS,
  mergeHistoryItems,
  mergeEvents,
  workspaceStateToViews,
} from "./model";

describe("workspace model", () => {
  it("maps the protocol state to product views", () => {
    const basic = create(BasicViewSchema, { accountId: BigInt(7), gold: 18 });
    const garden = create(GardenViewSchema, { accountId: BigInt(7) });
    const views = workspaceStateToViews(create(WorkspaceStateSchema, {
      accountId: BigInt(7),
      basic,
      garden,
    }));
    expect(views.basic?.gold).toBe(18);
    expect(views.garden?.accountId).toBe(BigInt(7));
    expect(views.orders).toBeNull();
  });

  it("applies changed fields and explicit domain clears", () => {
    const current = {
      ...EMPTY_ACCOUNT_VIEWS,
      basic: create(BasicViewSchema, { gold: 8 }),
      garden: create(GardenViewSchema, { accountId: BigInt(7) }),
    };
    const next = applyWorkspacePatch(current, create(WorkspacePatchSchema, {
      accountId: BigInt(7),
      basic: create(BasicViewSchema, { gold: 9 }),
      clearedDomains: [WorkspaceDomain.GARDEN],
    }));
    expect(next.basic?.gold).toBe(9);
    expect(next.garden).toBeNull();
  });

  it("deduplicates replayed logs and keeps newest first", () => {
    const old = create(EventSchema, { id: BigInt(1), accountId: BigInt(7), kind: "old" });
    const fresh = create(EventSchema, { id: BigInt(2), accountId: BigInt(7), kind: "fresh" });
    const merged = mergeEvents([old], [old, fresh]);
    expect(merged.map((event) => event.id)).toEqual([BigInt(2), BigInt(1)]);
  });

  it("merges paged history without duplicates and keeps newest first", () => {
    const first = create(WorkspaceHistoryItemSchema, { id: BigInt(3), label: "new" });
    const second = create(WorkspaceHistoryItemSchema, { id: BigInt(2), label: "old" });
    const merged = mergeHistoryItems([first], [first, second]);
    expect(merged.map((item) => item.id)).toEqual([BigInt(3), BigInt(2)]);
  });
});
