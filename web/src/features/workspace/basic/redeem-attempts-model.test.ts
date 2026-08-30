import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  AccountRedeemAttemptFilter,
  AccountRedeemAttemptPageSchema,
  AccountRedeemAttemptSchema,
  AccountRedeemAttemptSummarySchema,
} from "@/gen/mygardenworld/v1/workspace_pb";
import {
  applyRedeemAttemptPage,
  emptyRedeemAttemptFeed,
  loadingRedeemAttemptFeed,
} from "./redeem-attempts-model";

describe("redeem attempt feed", () => {
  it("replaces the current window and ignores stale filters", () => {
    const loading = loadingRedeemAttemptFeed(
      emptyRedeemAttemptFeed("7"),
      "7",
      AccountRedeemAttemptFilter.REDEEMED,
      "replace",
    );
    const stale = create(AccountRedeemAttemptPageSchema, {
      accountId: BigInt(7),
      filter: AccountRedeemAttemptFilter.ALL,
    });
    expect(applyRedeemAttemptPage(loading, stale)).toBe(loading);

    const page = create(AccountRedeemAttemptPageSchema, {
      accountId: BigInt(7),
      filter: AccountRedeemAttemptFilter.REDEEMED,
      entries: [create(AccountRedeemAttemptSchema, { id: BigInt(2), code: "NEW" })],
      summary: create(AccountRedeemAttemptSummarySchema, { total: BigInt(3), success: BigInt(1) }),
      nextBeforeId: BigInt(2),
      hasMoreBefore: true,
      replace: true,
    });
    const next = applyRedeemAttemptPage(loading, page);
    expect(next.entries.map((entry) => entry.code)).toEqual(["NEW"]);
    expect(next.summary?.success).toBe(BigInt(1));
    expect(next.hasMore).toBe(true);
  });

  it("replaces a filtered window when a live result arrives", () => {
    const current = {
      ...emptyRedeemAttemptFeed("7", AccountRedeemAttemptFilter.ATTENTION),
      entries: [create(AccountRedeemAttemptSchema, { id: BigInt(4), code: "DONE" })],
    };
    const next = applyRedeemAttemptPage(
      current,
      create(AccountRedeemAttemptPageSchema, {
        accountId: BigInt(7),
        filter: AccountRedeemAttemptFilter.ATTENTION,
        replace: true,
      }),
    );
    expect(next.entries).toEqual([]);
  });

  it("appends older entries without duplicating updated records", () => {
    const initial = applyRedeemAttemptPage(
      loadingRedeemAttemptFeed(emptyRedeemAttemptFeed("7"), "7", AccountRedeemAttemptFilter.ALL, "replace"),
      create(AccountRedeemAttemptPageSchema, {
        accountId: BigInt(7),
        filter: AccountRedeemAttemptFilter.ALL,
        entries: [create(AccountRedeemAttemptSchema, { id: BigInt(3), code: "THREE" })],
      }),
    );
    const appended = applyRedeemAttemptPage(
      loadingRedeemAttemptFeed(initial, "7", AccountRedeemAttemptFilter.ALL, "append"),
      create(AccountRedeemAttemptPageSchema, {
        accountId: BigInt(7),
        filter: AccountRedeemAttemptFilter.ALL,
        entries: [
          create(AccountRedeemAttemptSchema, { id: BigInt(3), code: "THREE-UPDATED" }),
          create(AccountRedeemAttemptSchema, { id: BigInt(2), code: "TWO" }),
        ],
      }),
    );
    expect(appended.entries.map((entry) => entry.code)).toEqual(["THREE-UPDATED", "TWO"]);
  });
});
