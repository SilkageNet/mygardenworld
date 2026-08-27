import { MarketBuyMode, MarketPutMode, SelectionMode } from "@/gen/mygardenworld/v1/policy_pb";

export const SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.QUALITY, label: "品质" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

export const AUTO_REPLANT_SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

export const MARKET_PUT_MODE_OPTIONS = [
  { value: MarketPutMode.INVENTORY, label: "库存最多" },
  { value: MarketPutMode.SPECIFIC, label: "指定花朵" },
];

export const MARKET_BUY_MODE_OPTIONS = [
  { value: MarketBuyMode.ALL, label: "全部" },
  { value: MarketBuyMode.SPECIFIC, label: "指定花朵" },
  { value: MarketBuyMode.QUALITY, label: "指定品质" },
];

type RaceTaskType = { id: number; label: string; defaultPriority: number; note?: string };

export const RACE_TASK_TYPES: RaceTaskType[] = [
  { id: 2004, label: "VIP商店购买", defaultPriority: 0 },
  { id: 3006, label: "居民订单", defaultPriority: 0 },
  { id: 3016, label: "顾客订单", defaultPriority: 0 },
  { id: 3017, label: "材料商店购买", defaultPriority: 0 },
  { id: 3018, label: "宫廷订单", defaultPriority: 0 },
  { id: 3023, label: "珍珠采集雇佣", defaultPriority: 0 },
  { id: 3024, label: "好友偷花", defaultPriority: 0 },
  { id: 3030, label: "花艺售卖", defaultPriority: 0, note: "不要求「自动上架」；上架满5分钟会全部下架再挂；缺成品先按制作规则做最高价有种子花艺；上架时选库存数量最多的可售花艺" },
  { id: 3034, label: "花艺制作", defaultPriority: 0, note: "不要求「自动制作」；只做配方花都有种子且售价最高的花艺" },
  { id: 3035, label: "鲜花升级", defaultPriority: 0 },
  { id: 3036, label: "种植收获", defaultPriority: 5 },
  { id: 3044, label: "花种培育", defaultPriority: 0, note: "只接正好 36 分且进度为 0；不要求开启鲜花培育。竞赛不主动培育，只接取并在进度达标后提交。已接的 36 分任务一律不放弃（含手动接取、优先级为 0）" },
  { id: 3052, label: "动物互动", defaultPriority: 0 },
];
