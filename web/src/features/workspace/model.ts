import type { Account } from "@/gen/mygardenworld/v1/account_pb";
import { WorkspaceDomain, type WorkspaceHistoryItem, type WorkspaceHistorySummary, type WorkspacePatch, type WorkspaceState } from "@/gen/mygardenworld/v1/workspace_pb";
import type {
  AccountStatus,
  ActivitiesView,
  BasicView,
  Event,
  GardenView,
  OrdersView,
  UnionView,
  WarehouseView,
} from "@/lib/api/workspace-models";

const EVENT_LIMIT = 500;

export type AccountViews = {
  basic: BasicView | null;
  garden: GardenView | null;
  orders: OrdersView | null;
  union: UnionView | null;
  activities: ActivitiesView | null;
  warehouse: WarehouseView | null;
  history: WorkspaceHistorySummary | null;
};

export const EMPTY_ACCOUNT_VIEWS: AccountViews = {
  basic: null,
  garden: null,
  orders: null,
  union: null,
  activities: null,
  warehouse: null,
  history: null,
};

export function workspaceStateToViews(state: WorkspaceState): AccountViews {
  return {
    basic: state.basic ?? null,
    garden: state.garden ?? null,
    orders: state.orders ?? null,
    union: state.union ?? null,
    activities: state.activities ?? null,
    warehouse: state.warehouse ?? null,
    history: state.history ?? null,
  };
}

export function applyWorkspacePatch(current: AccountViews, patch: WorkspacePatch): AccountViews {
  const cleared = new Set(patch.clearedDomains);
  return {
    basic: cleared.has(WorkspaceDomain.BASIC) ? null : (patch.basic ?? current.basic),
    garden: cleared.has(WorkspaceDomain.GARDEN) ? null : (patch.garden ?? current.garden),
    orders: cleared.has(WorkspaceDomain.ORDERS) ? null : (patch.orders ?? current.orders),
    union: cleared.has(WorkspaceDomain.UNION) ? null : (patch.union ?? current.union),
    activities: cleared.has(WorkspaceDomain.ACTIVITIES) ? null : (patch.activities ?? current.activities),
    warehouse: cleared.has(WorkspaceDomain.WAREHOUSE) ? null : (patch.warehouse ?? current.warehouse),
    history: cleared.has(WorkspaceDomain.HISTORY) ? null : (patch.history ?? current.history),
  };
}

export function withAccountStatus(current: Map<string, AccountStatus>, status: AccountStatus) {
  const next = new Map(current);
  next.set(status.accountId.toString(), status);
  return next;
}

export function mergeEvents(current: Event[], incoming: Event[]) {
  const seen = new Set<string>();
  const merged: Event[] = [];
  for (const event of [...incoming].reverse().concat(current)) {
    const key = event.id > BigInt(0)
      ? `id:${event.id}`
      : `volatile:${event.accountId}:${event.ts?.seconds ?? 0}:${event.kind}:${event.message}`;
    if (seen.has(key)) continue;
    seen.add(key);
    merged.push(event);
    if (merged.length >= EVENT_LIMIT) break;
  }
  return merged;
}

export function mergeHistoryItems(current: WorkspaceHistoryItem[], incoming: WorkspaceHistoryItem[]) {
  const seen = new Set<bigint>();
  const merged: WorkspaceHistoryItem[] = [];
  for (const item of [...incoming, ...current]) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    merged.push(item);
  }
  return merged.sort((left, right) => left.id === right.id ? 0 : left.id > right.id ? -1 : 1);
}

export function upsertAccount(current: Account[], incoming: Account) {
  const index = current.findIndex((account) => account.id === incoming.id);
  if (index < 0) return [...current, incoming];
  const next = current.slice();
  next[index] = incoming;
  return next;
}
