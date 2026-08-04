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
  const displayName = normalizedCatalogName(item?.display_name);
  const name = normalizedCatalogName(item?.name);
  return displayName || name || (id ? `#${id}` : "");
}

function normalizedCatalogName(value?: string) {
  const text = value?.trim();
  if (!text || text === "0") return "";
  return text;
}

export function flowerDisplay(flowerId: number | string | bigint | undefined | null) {
  const flower = flowerInfo(flowerId);
  const item = itemInfo(flowerId);
  return {
    flower,
    item,
    name: itemName(flowerId),
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

// Mirrors backend c_lvl.$max / c_lvl[level].exp used by ExperienceToNextLevel.
const PLAYER_MAX_LEVEL = 65;
const PLAYER_LEVEL_EXP_REQUIRED: Record<number, number> = {
  1: 40,
  2: 120,
  3: 320,
  4: 650,
  5: 1000,
  6: 2100,
  7: 3250,
  8: 5000,
  9: 7500,
  10: 12000,
  11: 13500,
  12: 16300,
  13: 19200,
  14: 31000,
  15: 36500,
  16: 43900,
  17: 51800,
  18: 61600,
  19: 81400,
  20: 95100,
  21: 116600,
  22: 120400,
  23: 129900,
  24: 141000,
  25: 415000,
  26: 475000,
  27: 708000,
  28: 839000,
  29: 1035000,
  30: 1630000,
  31: 2204000,
  32: 2324000,
  33: 2871000,
  34: 3566000,
  35: 3741000,
  36: 4398000,
  37: 5257000,
  38: 6159000,
  39: 7230000,
  40: 8502000,
  41: 9919000,
  42: 11570000,
  43: 13455000,
  44: 15619000,
  45: 18358000,
  46: 22921000,
  47: 24961000,
  48: 27748000,
  49: 33258000,
  50: 40225000,
  51: 49094000,
  52: 60239000,
  53: 74873000,
  54: 93445000,
  55: 117757000,
  56: 139135000,
  57: 181626000,
  58: 241626000,
  59: 321626000,
  60: 471676000,
  61: 671876070,
  62: 922476120,
  63: 1230484920,
  64: 1730817920,
};

export function playerMaxLevel() {
  return PLAYER_MAX_LEVEL;
}

export function playerLevelExpRequired(level: number) {
  if (level <= 0) return undefined;
  const required = PLAYER_LEVEL_EXP_REQUIRED[level];
  return required && required > 0 ? required : undefined;
}

/** Remaining within-level XP to advance; mirrors backend ExperienceToNextLevel. */
export function experienceToNextLevel(level: number, experience: number) {
  if (level <= 0) {
    return { remaining: 0, required: 0, maxed: false };
  }
  if (level >= PLAYER_MAX_LEVEL) {
    return { remaining: 0, required: 0, maxed: true };
  }
  const required = playerLevelExpRequired(level);
  if (!required) {
    return { remaining: 0, required: 0, maxed: true };
  }
  return {
    remaining: Math.max(0, required - experience),
    required,
    maxed: false,
  };
}
