import type {
  ActivitiesView,
  AssetsView,
  GardenView,
  OrdersView,
  OverviewView,
  UnionView,
} from "@/lib/api/query-models";

export type AccountViews = {
  overview: OverviewView | null;
  garden: GardenView | null;
  orders: OrdersView | null;
  union: UnionView | null;
  activities: ActivitiesView | null;
  assets: AssetsView | null;
};

export const EMPTY_ACCOUNT_VIEWS: AccountViews = {
  overview: null,
  garden: null,
  orders: null,
  union: null,
  activities: null,
  assets: null,
};
