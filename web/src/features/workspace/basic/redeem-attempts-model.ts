import {
  AccountRedeemAttemptFilter,
  type AccountRedeemAttempt,
  type AccountRedeemAttemptPage,
  type AccountRedeemAttemptSummary,
} from "@/gen/mygardenworld/v1/workspace_pb";

export const REDEEM_ATTEMPT_PAGE_SIZE = 20;

export type RedeemAttemptLoadingMode = "" | "replace" | "append";

export type RedeemAttemptFeed = {
  accountId: string;
  filter: AccountRedeemAttemptFilter;
  entries: AccountRedeemAttempt[];
  summary: AccountRedeemAttemptSummary | null;
  nextBeforeId: bigint;
  hasMore: boolean;
  loadingMode: RedeemAttemptLoadingMode;
};

export function emptyRedeemAttemptFeed(
  accountId = "",
  filter = AccountRedeemAttemptFilter.ALL,
): RedeemAttemptFeed {
  return {
    accountId,
    filter,
    entries: [],
    summary: null,
    nextBeforeId: BigInt(0),
    hasMore: false,
    loadingMode: "",
  };
}

export function loadingRedeemAttemptFeed(
  current: RedeemAttemptFeed,
  accountId: string,
  filter: AccountRedeemAttemptFilter,
  mode: Exclude<RedeemAttemptLoadingMode, "">,
): RedeemAttemptFeed {
  if (mode === "append") return { ...current, loadingMode: mode };
  return {
    ...emptyRedeemAttemptFeed(accountId, filter),
    entries: current.accountId === accountId && current.filter === filter ? current.entries : [],
    summary: current.accountId === accountId && current.filter === filter ? current.summary : null,
    loadingMode: mode,
  };
}

export function applyRedeemAttemptPage(
  current: RedeemAttemptFeed,
  page: AccountRedeemAttemptPage,
): RedeemAttemptFeed {
  const accountId = page.accountId.toString();
  if (current.accountId !== accountId || current.filter !== page.filter) return current;
  const merged = new Map<bigint, AccountRedeemAttempt>();
  if (!page.replace && current.loadingMode !== "replace") {
    for (const entry of current.entries) merged.set(entry.id, entry);
  }
  for (const entry of page.entries) merged.set(entry.id, entry);
  const entries = Array.from(merged.values()).sort((left, right) => (
    left.id === right.id ? 0 : left.id > right.id ? -1 : 1
  ));
  return {
    accountId,
    filter: page.filter,
    entries,
    summary: page.summary ?? current.summary,
    nextBeforeId: page.nextBeforeId,
    hasMore: page.hasMoreBefore,
    loadingMode: "",
  };
}
