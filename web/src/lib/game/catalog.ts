import catalog from "./catalog.json";

export type ItemStack = {
  item_id: number;
  count?: number;
  extra?: number[];
};

export type ItemInfo = {
  id: number;
  name?: string;
  short_name?: string;
  display_name?: string;
  icon_path?: string;
  color?: number;
  type?: number;
  use_type?: number;
  items?: ItemStack[];
  restore?: ItemStack[];
};

export type FlowerInfo = {
  id: number;
  seed_id?: number;
  elite_id?: number;
  sort?: number;
  experience?: number;
  gold?: number;
  cultivate_cost?: ItemStack[];
};

export type FarmLandInfo = {
  id: number;
  open_level?: number;
  cost?: number[];
  wasteland?: number[];
};

type Catalog = {
  items: Record<string, ItemInfo>;
  flowers: Record<string, FlowerInfo>;
  farm_lands: Record<string, FarmLandInfo>;
};

const gameCatalog = catalog as Catalog;

export function itemInfo(id: number | string | bigint | undefined | null): ItemInfo | undefined {
  if (id === undefined || id === null) return undefined;
  return gameCatalog.items[String(id)];
}

export function flowerInfo(id: number | string | bigint | undefined | null): FlowerInfo | undefined {
  if (id === undefined || id === null) return undefined;
  return gameCatalog.flowers[String(id)];
}

export function farmLandInfo(id: number | string | bigint | undefined | null): FarmLandInfo | undefined {
  if (id === undefined || id === null) return undefined;
  return gameCatalog.farm_lands[String(id)];
}

export function allFarmLands(): FarmLandInfo[] {
  return Object.values(gameCatalog.farm_lands)
    .filter((land) => land.id > 0)
    .sort((a, b) => a.id - b.id);
}

export function allFlowers(): FlowerInfo[] {
  return Object.values(gameCatalog.flowers)
    .filter((flower) => flower.id >= 23000 && flower.id < 24000)
    .sort((a, b) => (a.sort || a.id) - (b.sort || b.id));
}

export function itemName(id: number | string | bigint | undefined | null): string {
  const item = itemInfo(id);
  return item?.display_name || item?.name || (id ? `#${id}` : "");
}

export function itemIconPath(id: number | string | bigint | undefined | null): string | undefined {
  return itemInfo(id)?.icon_path;
}

export function flowerDisplay(flowerId: number | string | bigint | undefined | null) {
  const flower = flowerInfo(flowerId);
  const item = itemInfo(flowerId);
  return {
    flower,
    item,
    name: itemName(flowerId),
    iconPath: item?.icon_path,
    seedName: flower?.seed_id ? itemName(flower.seed_id) : "",
    essenceName: flower?.elite_id ? itemName(flower.elite_id) : "",
  };
}

export function itemCategory(item?: ItemInfo): string {
  if (!item) return "未知资源";
  if (item.id === 1 || item.id === 7 || item.id === 11 || item.id === 17) return "核心资源";
  if (item.id >= 23000 && item.id < 24000) return "鲜花";
  if (item.id >= 22000 && item.id < 23000) return "花朵精华";
  if (item.id >= 1400 && item.id < 1600) return "培育材料";
  if (item.use_type || item.items?.length) return "可用道具";
  if (item.type === 0) return "货币资源";
  return "其他库存";
}

export function itemColorClass(item?: ItemInfo): string {
  switch (item?.color) {
    case 1:
      return "border-slate-400/30 bg-slate-400/10 text-slate-100";
    case 2:
      return "border-sky-400/30 bg-sky-400/10 text-sky-100";
    case 3:
      return "border-amber-400/30 bg-amber-400/10 text-amber-100";
    case 4:
      return "border-violet-400/30 bg-violet-400/10 text-violet-100";
    default:
      return "border-border bg-muted/35 text-foreground";
  }
}

export function allCatalogItems(): ItemInfo[] {
  return Object.values(gameCatalog.items);
}
