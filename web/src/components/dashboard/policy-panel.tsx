"use client";

import { create } from "@bufbuild/protobuf";
import { useEffect, useMemo, useState, type PointerEvent, type ReactNode } from "react";
import {
  BadgeCheck,
  Building2,
  CalendarDays,
  Check,
  Coins,
  Flower2,
  Gem,
  GripVertical,
  HandCoins,
  ListChecks,
  Loader2,
  Minus,
  Package,
  Play,
  Plus,
  Save,
  Search,
  ShieldCheck,
  ShoppingBag,
  Sparkles,
  Sprout,
  Trophy,
  Users,
} from "lucide-react";

import {
  ActivityModulePolicySchema,
  ActivityPolicySchema,
  BasicPolicySchema,
  BasicTaskPolicySchema,
  BenefitPolicySchema,
  CultivatePolicySchema,
  CustomerOrderPolicySchema,
  FlowerElvesPolicySchema,
  FlowerMarketPolicySchema,
  FlowerArtPolicySchema,
  FriendStealPolicySchema,
  MarketBuyMode,
  MarketPutMode,
  OrderPolicySchema,
  PalaceOrderPolicySchema,
  PearlPolicySchema,
  PlantPolicySchema,
  PlantingPolicySchema,
  ReputationPolicySchema,
  ResidentOrderPolicySchema,
  SelectionMode,
  SignPolicySchema,
  ShopBuyPolicySchema,
  ShopPolicySchema,
  TeamOrderPolicySchema,
  UnionBuildPolicySchema,
  UnionFlowerPolicySchema,
  UnionLandPolicySchema,
  UnionPolicySchema,
  UnionRacePolicySchema,
  VipShopPolicySchema,
  ZooPolicySchema,
} from "@/gen/mygardenworld/v1/policy_pb";
import type {
  ActivityModulePolicy,
  ActivityPolicy,
  BasicPolicy,
  BasicTaskPolicy,
  BenefitPolicy,
  CultivatePolicy,
  CustomerOrderPolicy,
  FlowerElvesPolicy,
  FlowerMarketPolicy,
  FlowerArtPolicy,
  FriendStealPolicy,
  OrderPolicy,
  PalaceOrderPolicy,
  PearlPolicy,
  PlantPolicy,
  PlantingPolicy,
  Policy,
  ReputationPolicy,
  ResidentOrderPolicy,
  ShopBuyPolicy,
  ShopPolicy,
  SignPolicy,
  TeamOrderPolicy,
  UnionBuildPolicy,
  UnionFlowerPolicy,
  UnionLandPolicy,
  UnionPolicy,
  UnionRacePolicy,
  VipShopPolicy,
  ZooPolicy,
} from "@/gen/mygardenworld/v1/policy_pb";
import type {
  FeatureCapability,
  GetSnapshotResponse,
  PlantableFlowerView,
} from "@/gen/mygardenworld/v1/query_service_pb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { settingStatusForCapability, type SettingStatus } from "@/lib/feature-capabilities";
import { allFlowers, flowerDisplay, itemName } from "@/lib/game/catalog";
import { flowerMatureCdSeconds } from "@/lib/game/flower-cd";
import { cn } from "@/lib/utils";

const NUMBER_FORMATTER = new Intl.NumberFormat("zh-CN");
const SHOW_UNSUPPORTED_SETTINGS = false;

const GOAL_OPTIONS = [
  { id: "order.customer", label: "顾客订单", defaultPriority: 90 },
  { id: "order.resident", label: "居民订单", defaultPriority: 80 },
  { id: "basic.task.main", label: "主线任务", defaultPriority: 70 },
  { id: "basic.task.daily", label: "日常任务", defaultPriority: 60 },
  { id: "basic.task.weekly", label: "周常任务", defaultPriority: 55 },
  { id: "order.flower_art", label: "花艺/花架", defaultPriority: 40 },
  { id: "fallback.auto_replant", label: "自主补种", defaultPriority: 10 },
];

type PolicyTabId = "basic" | "order" | "union" | "other" | "activity";

const POLICY_TABS: { id: PolicyTabId; label: string; icon: ReactNode }[] = [
  { id: "basic", label: "基础", icon: <Sprout /> },
  { id: "order", label: "订单", icon: <ListChecks /> },
  { id: "union", label: "公会", icon: <Users /> },
  { id: "other", label: "其他", icon: <ShoppingBag /> },
  { id: "activity", label: "活动", icon: <CalendarDays /> },
];

const QUALITY_OPTIONS = [1, 2, 3, 4, 5];
const QUALITY_LABELS: Record<number, string> = { 1: "凡", 2: "普", 3: "珍", 4: "华", 5: "仙" };

type FlowerPickerSortMode = "stock_desc" | "stock_asc" | "mature_asc" | "mature_desc";

const FLOWER_PICKER_SORT_OPTIONS: { value: FlowerPickerSortMode; label: string }[] = [
  { value: "stock_asc", label: "库存从低到高" },
  { value: "stock_desc", label: "库存从高到低" },
];

const PLANTABLE_FLOWER_PICKER_SORT_OPTIONS: { value: FlowerPickerSortMode; label: string }[] = [
  { value: "mature_asc", label: "成熟从短到长" },
  { value: "mature_desc", label: "成熟从长到短" },
  { value: "stock_asc", label: "库存从低到高" },
  { value: "stock_desc", label: "库存从高到低" },
];

const SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.QUALITY, label: "品质" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

const AUTO_REPLANT_SELECTION_MODE_OPTIONS = [
  { value: SelectionMode.ALL, label: "全部" },
  { value: SelectionMode.SPECIFIC, label: "指定" },
  { value: SelectionMode.EXCLUDE, label: "排除" },
];

const MARKET_PUT_MODE_OPTIONS = [
  { value: MarketPutMode.INVENTORY, label: "库存最多" },
  { value: MarketPutMode.SPECIFIC, label: "指定花朵" },
];

const MARKET_BUY_MODE_OPTIONS = [
  { value: MarketBuyMode.ALL, label: "全部" },
  { value: MarketBuyMode.SPECIFIC, label: "指定花朵" },
  { value: MarketBuyMode.QUALITY, label: "指定品质" },
];

type RaceTaskType = {
  id: number;
  label: string;
  defaultPriority: number;
  note?: string;
};

const RACE_TASK_TYPES: RaceTaskType[] = [
  { id: 2004, label: "VIP商店购买", defaultPriority: 0 },
  { id: 3006, label: "居民订单", defaultPriority: 0 },
  { id: 3016, label: "顾客订单", defaultPriority: 0 },
  { id: 3017, label: "材料商店购买", defaultPriority: 0 },
  { id: 3018, label: "宫廷订单", defaultPriority: 0 },
  { id: 3023, label: "珍珠采集雇佣", defaultPriority: 0 },
  { id: 3024, label: "好友偷花", defaultPriority: 0 },
  { id: 3030, label: "花艺售卖", defaultPriority: 0, note: "不要求「自动上架」；上架满5分钟会全部下架再挂；缺成品先按制作规则做最高价有种子花艺" },
  {
    id: 3034,
    label: "花艺制作",
    defaultPriority: 0,
    note: "不要求「自动制作」；只做配方花都有种子且售价最高的花艺",
  },
  { id: 3035, label: "鲜花升级", defaultPriority: 0 },
  { id: 3036, label: "种植收获", defaultPriority: 5 },
  {
    id: 3044,
    label: "花种培育",
    defaultPriority: 0,
    note: "只接正好 36 分且进度为 0；不要求开启鲜花培育。竞赛不主动培育，只接取并在进度达标后提交。已接的 36 分任务一律不放弃（含手动接取、优先级为 0）",
  },
  { id: 3052, label: "动物互动", defaultPriority: 0 },
];

type ActivityModuleMeta = {
  id: string;
  label: string;
  boolParams?: { key: string; label: string }[];
  intParams?: { key: string; label: string; defaultValue: number; min: number; max?: number }[];
};

const ACTIVITY_MODULES: ActivityModuleMeta[] = [
  {
    id: "cyclicNote",
    label: "花笺集芳",
    boolParams: [
      { key: "auto_claim_task_rewards", label: "自动领取任务奖励" },
      { key: "auto_claim_progress_boxes", label: "自动领取积分奖励" },
      { key: "satisfy_tasks", label: "驱动已启用模块完成任务" },
    ],
  },
  {
    id: "actCyclicStory",
    label: "莳花纪闻",
    boolParams: [
      { key: "auto_claim_order_rewards", label: "自动领取订单奖励" },
      { key: "auto_claim_progress_boxes", label: "自动领取积分奖励" },
    ],
    intParams: [
      { key: "max_score", label: "分数上限（0=不限制）", defaultValue: 0, min: 0 },
    ],
  },
  {
    id: "actDessert",
    label: "香卉甜糕",
    boolParams: [
      { key: "auto_claim_task_rewards", label: "自动领取任务奖励" },
      { key: "auto_like_celebrity", label: "自动免费点赞" },
      { key: "auto_open_reward_boxes", label: "自动开启奖励箱（每次1个）" },
      { key: "auto_play", label: "启用影子诊断（不执行）" },
      { key: "resume_existing_round", label: "请求接管评估（当前硬锁）" },
    ],
    intParams: [
      { key: "mode", label: "影子模式（仅 1 可用）", defaultValue: 1, min: 1, max: 1 },
      { key: "max_energy_per_session", label: "会话体力预算（0=禁用；当前仅诊断）", defaultValue: 0, min: 0, max: 100 },
      { key: "min_energy_reserve", label: "最低体力保留", defaultValue: 0, min: 0 },
    ],
  },
];

export default function PolicyPanel({
  policy,
  snapshot,
  capabilities,
  loading,
  saving,
  message,
  onPolicyChange,
  onSave,
}: {
  policy: Policy | null;
  snapshot: GetSnapshotResponse | null;
  capabilities: FeatureCapability[];
  loading: boolean;
  saving: boolean;
  message: string;
  onPolicyChange: (policy: Policy | null) => void;
  onSave: () => void;
}) {
  const [activeTab, setActiveTab] = useState<PolicyTabId>("basic");
  useEffect(() => {
    if (!POLICY_TABS.some((tab) => tab.id === activeTab)) {
      setActiveTab("basic");
    }
  }, [activeTab]);
  const plant = policy?.plant;
  const planting = plant?.planting;
  const cultivate = plant?.cultivate;
  const friendSteal = plant?.friendSteal;
  const elves = plant?.elves;
  const market = plant?.market;
  const basic = policy?.basic;
  const reputation = basic?.reputation;
  const task = basic?.task;
  const benefit = basic?.benefit;
  const sign = basic?.sign;
  const pearl = basic?.pearl;
  const shop = basic?.shop;
  const cultivateShop = shop?.cultivateShop;
  const vipShop = shop?.vipShop;
  const zoo = basic?.zoo;
  const order = policy?.order;
  const customer = order?.customer;
  const resident = order?.resident;
  const palace = order?.palace;
  const team = order?.team;
  const flowerArt = order?.flowerArt;
  const union = policy?.union;
  const unionBuild = union?.build;
  const unionFlower = union?.flower;
  const unionRace = union?.race;
  const unionLand = union?.land;
  const activity = policy?.activity;
  const customerOrdersObserved = snapshot?.observedNamespaces.includes("109") ?? false;
  const customerOrderCount = snapshot?.pendingTasks.filter((taskItem) => taskItem.category === "顾客订单").length ?? 0;
  const customerOrderSyncLabel = snapshot
    ? customerOrdersObserved
      ? `已同步 109，当前 ${customerOrderCount} 单`
      : "未同步 109"
    : "状态未加载";

  const updatePolicy = (patch: Partial<Policy>) => {
    if (!policy) return;
    onPolicyChange({ ...policy, ...patch });
  };
  const updatePlant = (patch: Partial<PlantPolicy>) => {
    if (!policy) return;
    const current = policy.plant ?? create(PlantPolicySchema);
    onPolicyChange({ ...policy, plant: create(PlantPolicySchema, { ...current, ...patch }) });
  };
  const updateBasic = (patch: Partial<BasicPolicy>) => {
    if (!policy) return;
    const current = policy.basic ?? create(BasicPolicySchema);
    onPolicyChange({ ...policy, basic: { ...current, ...patch } });
  };
  const updateReputation = (patch: Partial<ReputationPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.reputation ?? create(ReputationPolicySchema);
    updateBasic({ reputation: { ...current, ...patch } });
  };
  const updateBasicTask = (patch: Partial<BasicTaskPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.task ?? create(BasicTaskPolicySchema);
    updateBasic({ task: { ...current, ...patch } });
  };
  const updateBenefit = (patch: Partial<BenefitPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.benefit ?? create(BenefitPolicySchema);
    updateBasic({ benefit: { ...current, ...patch } });
  };
  const updateSign = (patch: Partial<SignPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.sign ?? create(SignPolicySchema);
    updateBasic({ sign: { ...current, ...patch } });
  };
  const updatePearl = (patch: Partial<PearlPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.pearl ?? create(PearlPolicySchema);
    updateBasic({ pearl: { ...current, ...patch } });
  };
  const updateShop = (patch: Partial<ShopPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.shop ?? create(ShopPolicySchema);
    updateBasic({ shop: { ...current, ...patch } });
  };
  const updateCultivateShop = (patch: Partial<ShopBuyPolicy>) => {
    if (!policy) return;
    const currentShop = (policy.basic ?? create(BasicPolicySchema)).shop ?? create(ShopPolicySchema);
    const current = currentShop.cultivateShop ?? create(ShopBuyPolicySchema);
    updateShop({ cultivateShop: { ...current, ...patch } });
  };
  const updateVipShop = (patch: Partial<VipShopPolicy>) => {
    if (!policy) return;
    const currentShop = (policy.basic ?? create(BasicPolicySchema)).shop ?? create(ShopPolicySchema);
    const current = currentShop.vipShop ?? create(VipShopPolicySchema);
    updateShop({ vipShop: { ...current, ...patch } });
  };
  const updateZoo = (patch: Partial<ZooPolicy>) => {
    if (!policy) return;
    const currentBasic = policy.basic ?? create(BasicPolicySchema);
    const current = currentBasic.zoo ?? create(ZooPolicySchema);
    updateBasic({ zoo: { ...current, ...patch } });
  };
  const updatePlanting = (patch: Partial<PlantingPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.planting ?? create(PlantingPolicySchema);
    updatePlant({ planting: create(PlantingPolicySchema, { ...current, ...patch }) });
  };
  const updateCultivate = (patch: Partial<CultivatePolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.cultivate ?? create(CultivatePolicySchema);
    updatePlant({ cultivate: { ...current, ...patch } });
  };
  const updateFriendSteal = (patch: Partial<FriendStealPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.friendSteal ?? create(FriendStealPolicySchema);
    updatePlant({ friendSteal: { ...current, ...patch } });
  };
  const updateElves = (patch: Partial<FlowerElvesPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.elves ?? create(FlowerElvesPolicySchema);
    updatePlant({ elves: { ...current, ...patch } });
  };
  const updateMarket = (patch: Partial<FlowerMarketPolicy>) => {
    if (!policy) return;
    const currentPlant = policy.plant ?? create(PlantPolicySchema);
    const current = currentPlant.market ?? create(FlowerMarketPolicySchema);
    updatePlant({ market: { ...current, ...patch } });
  };
  const updateOrder = (patch: Partial<OrderPolicy>) => {
    if (!policy) return;
    const current = policy.order ?? create(OrderPolicySchema);
    onPolicyChange({ ...policy, order: { ...current, ...patch } });
  };
  const updateCustomer = (patch: Partial<CustomerOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.customer ?? create(CustomerOrderPolicySchema);
    updateOrder({ customer: { ...current, ...patch } });
  };
  const updateResident = (patch: Partial<ResidentOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.resident ?? create(ResidentOrderPolicySchema);
    updateOrder({ resident: { ...current, ...patch } });
  };
  const updatePalace = (patch: Partial<PalaceOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.palace ?? create(PalaceOrderPolicySchema);
    updateOrder({ palace: { ...current, ...patch } });
  };
  const updateTeam = (patch: Partial<TeamOrderPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.team ?? create(TeamOrderPolicySchema);
    updateOrder({ team: { ...current, ...patch } });
  };
  const updateFlowerArt = (patch: Partial<FlowerArtPolicy>) => {
    if (!policy) return;
    const currentOrder = policy.order ?? create(OrderPolicySchema);
    const current = currentOrder.flowerArt ?? create(FlowerArtPolicySchema);
    updateOrder({ flowerArt: create(FlowerArtPolicySchema, { ...current, ...patch }) });
  };
  const updateUnion = (patch: Partial<UnionPolicy>) => {
    if (!policy) return;
    const current = policy.union ?? create(UnionPolicySchema);
    onPolicyChange({ ...policy, union: { ...current, ...patch } });
  };
  const updateUnionBuild = (patch: Partial<UnionBuildPolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.build ?? create(UnionBuildPolicySchema);
    updateUnion({ build: { ...current, ...patch } });
  };
  const updateUnionFlower = (patch: Partial<UnionFlowerPolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.flower ?? create(UnionFlowerPolicySchema);
    updateUnion({ flower: { ...current, ...patch } });
  };
  const updateUnionRace = (patch: Partial<UnionRacePolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.race ?? create(UnionRacePolicySchema);
    updateUnion({ race: { ...current, ...patch } });
  };
  const updateUnionLand = (patch: Partial<UnionLandPolicy>) => {
    if (!policy) return;
    const currentUnion = policy.union ?? create(UnionPolicySchema);
    const current = currentUnion.land ?? create(UnionLandPolicySchema);
    updateUnion({ land: { ...current, ...patch } });
  };
  const updateActivity = (patch: Partial<ActivityPolicy>) => {
    if (!policy) return;
    const current = policy.activity ?? create(ActivityPolicySchema);
    onPolicyChange({ ...policy, activity: { ...current, ...patch } });
  };
  const updateActivityModule = (moduleID: string, patch: Partial<ActivityModulePolicy>) => {
    if (!policy) return;
    const currentActivity = policy.activity ?? create(ActivityPolicySchema);
    const current = currentActivity.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivity({
      modules: {
        ...currentActivity.modules,
        [moduleID]: { ...current, ...patch },
      },
    });
  };
  const updateActivityBoolParam = (moduleID: string, key: string, value: boolean) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, { boolParams: { ...current.boolParams, [key]: value } });
  };
  const updateActivityIntParam = (moduleID: string, key: string, value: number, fallback: number) => {
    const current = activity?.modules[moduleID] ?? create(ActivityModulePolicySchema);
    updateActivityModule(moduleID, { intParams: { ...current.intParams, [key]: safeNumberToBigInt(value, fallback) } });
  };
  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>策略</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState title="策略加载中" />
        </CardContent>
      </Card>
    );
  }

  if (!policy) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>策略</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState title="未加载策略" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-2">
          <CardTitle>策略</CardTitle>
          <Button type="button" size="sm" className="w-full sm:w-auto" onClick={onSave} disabled={saving}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
            {saving ? "保存中" : "保存"}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-5">
        {message && <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-sm">{message}</div>}

        <section className="space-y-3">
          <SectionTitle icon={<ShieldCheck />}>运行参数</SectionTitle>
          <div className="grid gap-2 sm:grid-cols-2">
            <NumberRow label="决策间隔" value={policy.decisionIntervalSeconds || 4} min={1} onChange={(value) => updatePolicy({ decisionIntervalSeconds: value })} />
          </div>
        </section>

        <div className="grid grid-cols-5 gap-1 rounded-md border border-border/70 bg-muted/20 p-1">
          {POLICY_TABS.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "flex min-h-12 min-w-0 flex-col items-center justify-center gap-1 rounded px-1 text-xs font-medium transition-colors sm:min-h-9 sm:flex-row sm:gap-2 sm:px-3 sm:text-sm [&_svg]:size-4",
                activeTab === tab.id ? "bg-background text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </div>

        {activeTab === "basic" && (
          <div className="space-y-4">
            <PolicyGroup title="土地与种植" icon={<Sprout />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动种植" checked={planting?.autoEnabled ?? false} onChange={(checked) => updatePlanting({ autoEnabled: checked })} />
                <ToggleRow label="自动收获" checked={planting?.autoHarvestEnabled ?? false} description="关闭后普通农田不自动收；公会竞赛种植任务仍会强制收获竞赛花" onChange={(checked) => updatePlanting({ autoHarvestEnabled: checked })} />
                <NumberRow
                  label="延时收获（秒）"
                  value={planting?.harvestDelaySeconds || 0}
                  min={0}
                  onChange={(value) => updatePlanting({ harvestDelaySeconds: value })}
                  description="植物成熟后等待多久再收获；0=立即收获。竞赛种植的花朵不受此间隔限制，默认直接收获"
                />
                <ToggleRow label="解锁土地" checked={planting?.autoUnlockLand ?? false} onChange={(checked) => updatePlanting({ autoUnlockLand: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="视频加速" checked={planting?.videoSpeedUpEnabled ?? false} onChange={(checked) => updatePlanting({ videoSpeedUpEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.video_speed_up")} />}
                <ToggleRow label="使用加速券" checked={planting?.useSpeedUpTicket ?? false} onChange={(checked) => updatePlanting({ useSpeedUpTicket: checked })} />
                <NumberRow label="加速券上限" value={planting?.speedUpTicketMax || 0} min={0} onChange={(value) => updatePlanting({ speedUpTicketMax: value })} />
                <NumberRow
                  label="保留水滴"
                  value={planting?.minWaterDrops || 0}
                  min={0}
                  onChange={(value) => updatePlanting({ minWaterDrops: value })}
                  description="可用水滴=当前−保留，仅限制下种数量"
                />
              </div>
            </PolicyGroup>

            <PolicyGroup title="自主补种" icon={<Package />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <SegmentedRow
                  label="补种范围"
                  value={planting?.autoReplantMode || SelectionMode.ALL}
                  options={AUTO_REPLANT_SELECTION_MODE_OPTIONS}
                  onChange={(value) => updatePlanting({ autoReplantMode: value })}
                />
                {(planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.ALL ? (
                  <QualityRow
                    label="补种品质"
                    value={planting?.autoReplantQualities ?? []}
                    onChange={(value) => updatePlanting({ autoReplantQualities: value })}
                    labels={QUALITY_LABELS}
                    emptyMeansAll
                  />
                ) : (
                  <FlowerMultiSelectRow
                    label={(planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.EXCLUDE ? "排除补种" : "指定补种"}
                    value={(planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.EXCLUDE ? planting?.autoReplantExcludeFlowerIds ?? [] : planting?.autoReplantFlowerIds ?? []}
                    plantableFlowers={snapshot?.plantableFlowers ?? []}
                    synced={Boolean(snapshot)}
                    onChange={(value) =>
                      (planting?.autoReplantMode || SelectionMode.ALL) === SelectionMode.EXCLUDE
                        ? updatePlanting({ autoReplantExcludeFlowerIds: value })
                        : updatePlanting({ autoReplantFlowerIds: value })
                    }
                  />
                )}
                <NumberRow
                  label="最低种植等级"
                  value={planting?.autoReplantMinLevel || 0}
                  min={0}
                  max={20}
                  onChange={(value) => updatePlanting({ autoReplantMinLevel: value })}
                  description="0=不限；设为11则只种培育等级11-20的鲜花"
                />
              </div>
              <ToggleRow
                label="生产需求优先级"
                checked={planting?.demandPriorityEnabled ?? false}
                onChange={(checked) => updatePlanting({ demandPriorityEnabled: checked })}
                description="开启后按下方排序优先为缺花订单/任务补种；关闭时空地只按库存自主补种"
              />
              {planting?.demandPriorityEnabled ? (
                <DemandPriorityEditor value={planting?.demandPriority ?? {}} onChange={(demandPriority) => updatePlanting({ demandPriority })} />
              ) : null}
            </PolicyGroup>

            <PolicyGroup title="培育配置" icon={<Flower2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动培育" checked={cultivate?.enabled ?? false} onChange={(checked) => updateCultivate({ enabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="视频加速培育" checked={cultivate?.videoSpeedUpEnabled ?? false} onChange={(checked) => updateCultivate({ videoSpeedUpEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.cultivate_video_speed_up")} />}
                <ToggleRow label="鲜花升级" checked={cultivate?.upgradeEnabled ?? false} onChange={(checked) => updateCultivate({ upgradeEnabled: checked })} />
                <NumberRow label="目标等级" value={cultivate?.targetLevel || 20} min={1} onChange={(value) => updateCultivate({ targetLevel: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="基础配置" icon={<ShieldCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="礼仪分监控" checked={reputation?.enabled ?? false} onChange={(checked) => updateReputation({ enabled: checked })} />
                <NumberRow label="礼仪分阈值" value={reputation?.threshold || 80} min={0} onChange={(value) => updateReputation({ threshold: value })} />
                <ToggleRow
                  label="被挤号后自动重登"
                  checked={basic?.displacedSessionReloginEnabled ?? false}
                  onChange={(checked) => updateBasic({ displacedSessionReloginEnabled: checked })}
                />
                <NumberRow
                  label="自动重登间隔（秒）"
                  value={basic?.reconnectIntervalSeconds || 300}
                  min={1}
                  max={86400}
                  disabled={!basic?.displacedSessionReloginEnabled}
                  onChange={(value) => updateBasic({ reconnectIntervalSeconds: value })}
                />
                <p className="px-1 text-xs leading-5 text-muted-foreground sm:col-span-2">
                  {basic?.displacedSessionReloginEnabled
                    ? "已启用：明确检测到异地登录或被挤下线后，将等待上述时间再自动登录。主动退出和普通业务失败不会触发。"
                    : "默认关闭。开启后仅在明确检测到异地登录或被挤下线时自动重登；关闭时不会自动登录。"}
                </p>
              </div>
            </PolicyGroup>

            <PolicyGroup title="任务与剧情" icon={<ListChecks />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="主线任务" checked={task?.mainEnabled ?? false} onChange={(checked) => updateBasicTask({ mainEnabled: checked })} />
                <ToggleRow label="每日任务" checked={task?.dailyEnabled ?? false} onChange={(checked) => updateBasicTask({ dailyEnabled: checked })} />
                <ToggleRow label="每周任务" checked={task?.weeklyEnabled ?? false} onChange={(checked) => updateBasicTask({ weeklyEnabled: checked })} />
                <ToggleRow label="主线剧情" checked={task?.storyEnabled ?? false} onChange={(checked) => updateBasicTask({ storyEnabled: checked })} />
                <ToggleRow label="成就任务" checked={task?.achievementEnabled ?? false} onChange={(checked) => updateBasicTask({ achievementEnabled: checked })} />
                <ToggleRow label="地图随机事件" checked={basic?.mapEventEnabled ?? false} onChange={(checked) => updateBasic({ mapEventEnabled: checked })} />
              </div>
            </PolicyGroup>

          </div>
        )}

        {activeTab === "other" && (
          <div className="space-y-4">
            <PolicyGroup title="邮件、福利、签到" icon={<BadgeCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="邮件" checked={basic?.mailEnabled ?? false} onChange={(checked) => updateBasic({ mailEnabled: checked })} />
                <ToggleRow label="水车水滴" checked={basic?.waterwheelEnabled ?? false} onChange={(checked) => updateBasic({ waterwheelEnabled: checked })} description="广告桶走部分领取(skip)，不看视频；每次约3–7滴，非看视频+30" />
                <ToggleRow label="限时水滴" checked={basic?.freeWaterEnabled ?? false} onChange={(checked) => updateBasic({ freeWaterEnabled: checked })} />
                <NumberRow label="水滴领取阈值" value={basic?.waterClaimThreshold || 0} min={0} onChange={(value) => updateBasic({ waterClaimThreshold: value })} description="当前水滴≥该值时暂停水车/限时领取；0=不限制。与自然恢复上限(如130)无关" />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="双倍金币" checked={benefit?.doubleCoinEnabled ?? false} onChange={(checked) => updateBenefit({ doubleCoinEnabled: checked })} status={settingStatusForCapability(capabilities, "basic.double_coin")} />}
                <ToggleRow label="福利宝箱" checked={benefit?.boxEnabled ?? false} onChange={(checked) => updateBenefit({ boxEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="分享奖励" checked={benefit?.shareRewardEnabled ?? false} onChange={(checked) => updateBenefit({ shareRewardEnabled: checked })} status={settingStatusForCapability(capabilities, "basic.share_reward")} />}
                <ToggleRow label="防骗宝箱" checked={benefit?.antiScamBoxEnabled ?? false} onChange={(checked) => updateBenefit({ antiScamBoxEnabled: checked })} />
                <ToggleRow label="防诈骗签到奖励" checked={sign?.dailyEnabled ?? false} onChange={(checked) => updateSign({ dailyEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="自动补签" checked={sign?.patchEnabled ?? false} onChange={(checked) => updateSign({ patchEnabled: checked })} status={settingStatusForCapability(capabilities, "basic.sign_patch")} />}
                <ToggleRow label="成长之路" checked={basic?.roadGrowRewardEnabled ?? false} onChange={(checked) => updateBasic({ roadGrowRewardEnabled: checked })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="珍珠" icon={<Gem />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="免费珍珠" checked={pearl?.freeEnabled ?? false} onChange={(checked) => updatePearl({ freeEnabled: checked })} />
                <ToggleRow label="安全雇佣劳工" checked={pearl?.autoHireEnabled ?? false} onChange={(checked) => updatePearl({ autoHireEnabled: checked })} />
                <NumberRow label="雇佣等级上限（0=不限）" value={pearl?.maxHireLevel || 0} min={0} onChange={(value) => updatePearl({ maxHireLevel: value })} />
                <NumberRow label="同时在岗上限（0=关闭）" value={pearl?.maxHireTicketUsage || 0} min={0} onChange={(value) => updatePearl({ maxHireTicketUsage: value })} />
                <ToggleRow label="自动开珍珠" checked={pearl?.drawEnabled ?? false} onChange={(checked) => updatePearl({ drawEnabled: checked })} />
                <ToggleRow label="开启防身" checked={pearl?.protectEnabled ?? false} onChange={(checked) => updatePearl({ protectEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="买雇佣书" checked={pearl?.autoBuyHireTicket ?? false} onChange={(checked) => updatePearl({ autoBuyHireTicket: checked })} status={settingStatusForCapability(capabilities, "basic.pearl_buy_hire_ticket")} />
                    <BigIntNumberRow label="元宝上限" value={pearl?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updatePearl({ maxSpendDiamond: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            <PolicyGroup title="商城" icon={<ShoppingBag />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="视频礼包" checked={shop?.videoFreeGiftEnabled ?? false} onChange={(checked) => updateShop({ videoFreeGiftEnabled: checked })} />
                <ToggleRow label="材料商店" checked={cultivateShop?.autoBuy ?? false} onChange={(checked) => updateCultivateShop({ autoBuy: checked })} />
                <BigIntNumberRow label="材料金币上限" value={cultivateShop?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateCultivateShop({ maxSpendGold: value })} />
                <IntListRow label="材料商品 ID" value={cultivateShop?.itemIds ?? []} onChange={(value) => updateCultivateShop({ itemIds: value })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="VIP 商店" checked={vipShop?.autoBuy ?? false} onChange={(checked) => updateVipShop({ autoBuy: checked })} status={settingStatusForCapability(capabilities, "basic.shop_vip")} />
                    <BigIntNumberRow label="VIP 元宝上限" value={vipShop?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateVipShop({ maxSpendDiamond: value })} />
                    <BigIntNumberRow label="VIP 花坊币上限" value={vipShop?.maxSpendFloralCoin ?? BigInt(0)} min={0} onChange={(value) => updateVipShop({ maxSpendFloralCoin: value })} />
                    <IntListRow label="VIP 商品 ID" value={vipShop?.itemIds ?? []} onChange={(value) => updateVipShop({ itemIds: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            <PolicyGroup title="宠物" icon={<Sparkles />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="宠物模块" checked={zoo?.enabled ?? false} onChange={(checked) => updateZoo({ enabled: checked })} />
                <ToggleRow label="宠物外出/事件处理" checked={zoo?.autoEventEnabled ?? false} onChange={(checked) => updateZoo({ autoEventEnabled: checked })} />
                <ToggleRow label="自动补充食盆" checked={zoo?.autoFeed ?? false} onChange={(checked) => updateZoo({ autoFeed: checked })} />
                <ToggleRow label="自动互动" checked={zoo?.autoStroke ?? false} onChange={(checked) => updateZoo({ autoStroke: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="购买饲料" checked={zoo?.autoBuyFood ?? false} onChange={(checked) => updateZoo({ autoBuyFood: checked })} status={settingStatusForCapability(capabilities, "basic.zoo_buy_food")} />
                    <BigIntNumberRow label="宠物金币上限" value={zoo?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateZoo({ maxSpendGold: value })} />
                    <BigIntNumberRow label="宠物元宝上限" value={zoo?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateZoo({ maxSpendDiamond: value })} />
                  </>
                )}
              </div>
            </PolicyGroup>

            {SHOW_UNSUPPORTED_SETTINGS && (
              <>
                <PolicyGroup title="好友偷花" icon={<HandCoins />}>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <ToggleRow label="自动偷花" checked={friendSteal?.enabled ?? false} onChange={(checked) => updateFriendSteal({ enabled: checked })} status={settingStatusForCapability(capabilities, "plant.friend_steal")} />
                    <ToggleRow label="偷取花灵" checked={friendSteal?.stealElves ?? false} onChange={(checked) => updateFriendSteal({ stealElves: checked })} />
                    <SegmentedRow label="偷花模式" value={friendSteal?.mode || SelectionMode.ALL} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateFriendSteal({ mode: value })} />
                    <QualityRow label="指定品质" value={friendSteal?.qualities ?? []} onChange={(value) => updateFriendSteal({ qualities: value })} />
                    <IntListRow label="指定花朵" value={friendSteal?.flowerIds ?? []} onChange={(value) => updateFriendSteal({ flowerIds: value })} />
                    <IntListRow label="排除花朵" value={friendSteal?.excludeFlowerIds ?? []} onChange={(value) => updateFriendSteal({ excludeFlowerIds: value })} />
                    <ToggleRow label="购买偷取次数" checked={friendSteal?.autoBuyTimes ?? false} onChange={(checked) => updateFriendSteal({ autoBuyTimes: checked })} status={settingStatusForCapability(capabilities, "plant.friend_steal_buy")} />
                    <NumberRow label="购买次数" value={friendSteal?.buyCount || 0} min={0} onChange={(value) => updateFriendSteal({ buyCount: value })} />
                    <BigIntNumberRow label="元宝上限" value={friendSteal?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateFriendSteal({ maxSpendDiamond: value })} />
                  </div>
                </PolicyGroup>

                <PolicyGroup title="花灵与密令" icon={<Sparkles />}>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <ToggleRow label="自动种花灵" checked={elves?.enabled ?? false} onChange={(checked) => updateElves({ enabled: checked })} status={settingStatusForCapability(capabilities, "plant.elves")} />
                    <IntListRow label="指定花灵" value={elves?.selectedIds ?? []} onChange={(value) => updateElves({ selectedIds: value })} />
                    <ToggleRow label="申请协助" checked={elves?.requestAid ?? false} onChange={(checked) => updateElves({ requestAid: checked })} />
                    <ToggleRow label="领取协助" checked={elves?.receiveAid ?? false} onChange={(checked) => updateElves({ receiveAid: checked })} />
                    <ToggleRow label="协助好友" checked={elves?.helpFriend ?? false} onChange={(checked) => updateElves({ helpFriend: checked })} />
                    <ToggleRow label="派遣花灵" checked={elves?.dispatch ?? false} onChange={(checked) => updateElves({ dispatch: checked })} />
                    <ToggleRow label="仅双倍花灵" checked={elves?.dispatchOnlyDoubleBuff ?? false} onChange={(checked) => updateElves({ dispatchOnlyDoubleBuff: checked })} />
                    <ToggleRow label="加速派遣" checked={elves?.speedUpDispatch ?? false} onChange={(checked) => updateElves({ speedUpDispatch: checked })} status={settingStatusForCapability(capabilities, "plant.elves_speed_up")} />
                    <ToggleRow label="派遣奖励" checked={elves?.receiveDispatchReward ?? false} onChange={(checked) => updateElves({ receiveDispatchReward: checked })} />
                    <ToggleRow label="花灵密令等级" checked={elves?.passRewardEnabled ?? false} onChange={(checked) => updateElves({ passRewardEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.elves_pass")} />
                    <ToggleRow label="花灵密令任务" checked={elves?.passTaskRewardEnabled ?? false} onChange={(checked) => updateElves({ passTaskRewardEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.elves_pass")} />
                    <ToggleRow label="花之密令等级" checked={elves?.flowerPassRewardEnabled ?? false} onChange={(checked) => updateElves({ flowerPassRewardEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.flower_pass")} />
                    <ToggleRow label="花之密令任务" checked={elves?.flowerPassTaskRewardEnabled ?? false} onChange={(checked) => updateElves({ flowerPassTaskRewardEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.flower_pass")} />
                    <BigIntNumberRow label="元宝上限" value={elves?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateElves({ maxSpendDiamond: value })} />
                  </div>
                </PolicyGroup>

                <PolicyGroup title="鲜花摊位" icon={<ShoppingBag />}>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <ToggleRow label="解锁货架" checked={market?.autoUnlockShelf ?? false} onChange={(checked) => updateMarket({ autoUnlockShelf: checked })} status={settingStatusForCapability(capabilities, "plant.market_unlock")} />
                    <ToggleRow label="自动上架鲜花" checked={market?.putEnabled ?? false} onChange={(checked) => updateMarket({ putEnabled: checked })} status={settingStatusForCapability(capabilities, "plant.market")} />
                    <SegmentedRow label="上架策略" value={market?.putMode || MarketPutMode.INVENTORY} options={MARKET_PUT_MODE_OPTIONS} onChange={(value) => updateMarket({ putMode: value })} />
                    <IntListRow label="上架花朵" value={market?.specificFlowerIds ?? []} onChange={(value) => updateMarket({ specificFlowerIds: value })} />
                    <NumberRow label="上架价格" value={market?.priceIndex ?? 2} min={0} onChange={(value) => updateMarket({ priceIndex: value })} />
                    <NumberRow label="上架数量" value={market?.maxSell || 25} min={1} onChange={(value) => updateMarket({ maxSell: value })} />
                    <TextRow label="上架密码" value={market?.putFlowerPassword ?? ""} onChange={(value) => updateMarket({ putFlowerPassword: value })} />
                    <ToggleRow label="好友摊位扫货" checked={market?.autoBuyFromFriend ?? false} onChange={(checked) => updateMarket({ autoBuyFromFriend: checked })} status={settingStatusForCapability(capabilities, "plant.market")} />
                    <SegmentedRow label="扫货策略" value={market?.buyMode || MarketBuyMode.ALL} options={MARKET_BUY_MODE_OPTIONS} onChange={(value) => updateMarket({ buyMode: value })} />
                    <IntListRow label="扫货花朵" value={market?.buySpecificFlowerIds ?? []} onChange={(value) => updateMarket({ buySpecificFlowerIds: value })} />
                    <QualityRow label="扫货品质" value={market?.buyQualities ?? []} onChange={(value) => updateMarket({ buyQualities: value })} />
                    <NumberRow label="最小上架秒" value={market?.minPutTimeSeconds || 0} min={0} onChange={(value) => updateMarket({ minPutTimeSeconds: value })} />
                    <BigIntNumberRow label="元宝上限" value={market?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateMarket({ maxSpendDiamond: value })} />
                    <BigIntNumberRow label="金币上限" value={market?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateMarket({ maxSpendGold: value })} />
                  </div>
                </PolicyGroup>
              </>
            )}
          </div>
        )}

        {activeTab === "order" && (
          <div className="space-y-4">
            <PolicyGroup title="居民订单" icon={<ListChecks />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="普通居民订单" checked={resident?.normalEnabled ?? false} onChange={(checked) => updateResident({ normalEnabled: checked })} />
                <NumberRow
                  label="普通订单上限"
                  value={resident?.normalDailyLimit ?? 1200}
                  min={1}
                  max={1200}
                  disabled={!(resident?.normalEnabled ?? false)}
                  description="需先开启普通居民订单；上限按今日已完成次数生效"
                  onChange={(value) => updateResident({ normalDailyLimit: value })}
                />
                <ToggleRow
                  label="绸缎订单"
                  checked={resident?.satinEnabled ?? false}
                  onChange={(checked) =>
                    updateResident({
                      satinEnabled: checked,
					  ...(checked && !((resident?.satinDailyLimit ?? 0) > 0) ? { satinDailyLimit: 120 } : {}),
                    })
                  }
                />
                <NumberRow
                  label="绸缎订单上限"
                  value={resident?.satinDailyLimit || 120}
                  min={1}
                  max={120}
                  disabled={!(resident?.satinEnabled ?? false)}
                  description="需先开启绸缎订单；上限按今日已完成次数生效"
                  onChange={(value) => updateResident({ satinDailyLimit: value })}
                />
                <ToggleRow
                  label="建材订单"
                  checked={resident?.decorateEnabled ?? false}
                  onChange={(checked) =>
                    updateResident({
                      decorateEnabled: checked,
					  ...(checked && !((resident?.decorateDailyLimit ?? 0) > 0) ? { decorateDailyLimit: 120 } : {}),
                    })
                  }
                />
                <NumberRow
                  label="建材订单上限"
                  value={resident?.decorateDailyLimit || 120}
                  min={1}
                  max={120}
                  disabled={!(resident?.decorateEnabled ?? false)}
                  description="需先开启建材订单；上限按今日已完成次数生效"
                  onChange={(value) => updateResident({ decorateDailyLimit: value })}
                />
                <ToggleRow label="居民领奖" checked={resident?.rewardEnabled ?? false} onChange={(checked) => updateResident({ rewardEnabled: checked })} />
                <QualityRow label="品质限定" value={resident?.qualities ?? []} onChange={(value) => updateResident({ qualities: value })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="顾客订单" icon={<Package />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="顾客订单" checked={customer?.enabled ?? false} onChange={(checked) => updateCustomer({ enabled: checked })} />
                <ToggleRow label="暂时无货" checked={customer?.rejectUnavailableEnabled ?? false} onChange={(checked) => updateCustomer({ rejectUnavailableEnabled: checked })} />
                <StatusRow label="订单状态" value={customerOrderSyncLabel} tone={customerOrdersObserved ? "ready" : "muted"} />
              </div>
            </PolicyGroup>

            {SHOW_UNSUPPORTED_SETTINGS && (
              <PolicyGroup title="宫廷、组团" icon={<Package />}>
                <div className="grid gap-2 sm:grid-cols-2">
                  <ToggleRow label="宫廷订单" checked={palace?.enabled ?? false} onChange={(checked) => updatePalace({ enabled: checked })} status={settingStatusForCapability(capabilities, "order.palace")} />
                  <QualityRow label="宫廷品质" value={palace?.qualities ?? []} onChange={(value) => updatePalace({ qualities: value })} />
                  <ToggleRow label="组团订单" checked={team?.enabled ?? false} onChange={(checked) => updateTeam({ enabled: checked })} status={settingStatusForCapability(capabilities, "order.team")} />
                  <ToggleRow label="再来一单" checked={team?.oneMoreEnabled ?? false} onChange={(checked) => updateTeam({ oneMoreEnabled: checked })} status={settingStatusForCapability(capabilities, "order.team_one_more")} />
                  <ToggleRow label="仅已培育" checked={team?.submitOnlyCultivated ?? false} onChange={(checked) => updateTeam({ submitOnlyCultivated: checked })} />
                  <QualityRow label="组团品质" value={team?.qualities ?? []} onChange={(value) => updateTeam({ qualities: value })} />
                  <BigIntNumberRow label="组团元宝上限" value={team?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateTeam({ maxSpendDiamond: value })} />
                </div>
              </PolicyGroup>
            )}

            <PolicyGroup title="花架售卖" icon={<Flower2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="解锁花架" checked={flowerArt?.autoUnlockStand ?? false} onChange={(checked) => updateFlowerArt({ autoUnlockStand: checked })} status={settingStatusForCapability(capabilities, "order.flower_art_stand")} />}
                <ToggleRow label="自动上架花艺" checked={flowerArt?.sellEnabled ?? false} onChange={(checked) => updateFlowerArt({ sellEnabled: checked })} />
                <ToggleRow label="自动制作" checked={flowerArt?.craftEnabled ?? false} onChange={(checked) => updateFlowerArt({ craftEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="提前下架" checked={flowerArt?.earlyCancelEnabled ?? false} onChange={(checked) => updateFlowerArt({ earlyCancelEnabled: checked })} status={settingStatusForCapability(capabilities, "order.flower_art_early_cancel")} />}
                <ToggleRow
                  label="0-8点关闭自动上架花艺"
                  checked={flowerArt?.sellNightPauseEnabled ?? false}
                  disabled={!flowerArt?.sellEnabled}
                  description="需同时开启自动上架花艺；仅在 0:00-8:00（北京时间）暂停上架，领取收益不受影响"
                  onChange={(checked) => updateFlowerArt({ sellNightPauseEnabled: checked })}
                />
                <ToggleRow label="花艺经验" checked={flowerArt?.createRewardEnabled ?? false} onChange={(checked) => updateFlowerArt({ createRewardEnabled: checked })} />
                <ToggleRow label="图鉴奖励" checked={flowerArt?.collectRewardEnabled ?? false} onChange={(checked) => updateFlowerArt({ collectRewardEnabled: checked })} />
              </div>
            </PolicyGroup>
          </div>
        )}

        {activeTab === "union" && (
          <div className="space-y-4">
            <PolicyGroup title="公会土地" icon={<Building2 />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="自动收获" checked={unionLand?.harvestEnabled ?? false} onChange={(checked) => updateUnionLand({ harvestEnabled: checked })} />
                <ToggleRow label="自动种植" checked={unionLand?.autoPlantEnabled ?? false} onChange={(checked) => updateUnionLand({ autoPlantEnabled: checked })} />
                <NumberRow
                  label="成熟时长(分钟)"
                  value={unionLand?.minMaturityMinutes || 20}
                  min={1}
                  onChange={(value) => updateUnionLand({ minMaturityMinutes: value })}
                  description="未满11级：强制换种低等级花练级（距成熟≤2分钟则等收获后再换）。全部达到11级后：才按成熟时长换种；指定花朵非空时只种这些 ID（莹白露薇=23117）。"
                />
                <FlowerMultiSelectRow
                  label="指定花朵"
                  value={unionLand?.flowerIds ?? []}
                  plantableFlowers={snapshot?.plantableFlowers ?? []}
                  synced={Boolean(snapshot)}
                  onChange={(value) => updateUnionLand({ flowerIds: value })}
                />
                <QualityRow label="指定品质" value={unionLand?.qualities ?? []} onChange={(value) => updateUnionLand({ qualities: value })} />
                <NumberRow
                  label="最高花朵等级"
                  value={unionLand?.maxFlowerLevel || 0}
                  min={0}
                  onChange={(value) => updateUnionLand({ maxFlowerLevel: value })}
                  description="0 表示不限制；设置后只种培育等级不超过该值的花"
                />
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会建设" icon={<Coins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="视频建设" checked={unionBuild?.freeEnabled ?? false} onChange={(checked) => updateUnionBuild({ freeEnabled: checked })} status={settingStatusForCapability(capabilities, "union.build_video")} />}
                <ToggleRow label="金币建设" checked={unionBuild?.goldEnabled ?? false} onChange={(checked) => updateUnionBuild({ goldEnabled: checked })} />
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="元宝建设" checked={unionBuild?.diamondEnabled ?? false} onChange={(checked) => updateUnionBuild({ diamondEnabled: checked })} status={settingStatusForCapability(capabilities, "union.build_diamond")} />}
                <BigIntNumberRow label="金币上限" value={unionBuild?.maxSpendGold ?? BigInt(0)} min={0} onChange={(value) => updateUnionBuild({ maxSpendGold: value })} />
                {SHOW_UNSUPPORTED_SETTINGS && <BigIntNumberRow label="元宝上限" value={unionBuild?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateUnionBuild({ maxSpendDiamond: value })} />}
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会分享与摸花" icon={<HandCoins />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && (
                  <>
                    <ToggleRow label="自动分享" checked={unionFlower?.shareEnabled ?? false} onChange={(checked) => updateUnionFlower({ shareEnabled: checked })} status={settingStatusForCapability(capabilities, "union.flower_share")} />
                    <SegmentedRow label="分享模式" value={unionFlower?.shareMode || SelectionMode.QUALITY} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateUnionFlower({ shareMode: value })} />
                    <QualityRow label="分享品质" value={unionFlower?.shareQualities ?? []} onChange={(value) => updateUnionFlower({ shareQualities: value })} />
                    <IntListRow label="分享花朵" value={unionFlower?.shareFlowerIds ?? []} onChange={(value) => updateUnionFlower({ shareFlowerIds: value })} />
                  </>
                )}
                <ToggleRow label="自动摸花" checked={unionFlower?.takeEnabled ?? false} onChange={(checked) => updateUnionFlower({ takeEnabled: checked })} />
                <SegmentedRow label="摸花模式" value={unionFlower?.takeMode || SelectionMode.QUALITY} options={SELECTION_MODE_OPTIONS} onChange={(value) => updateUnionFlower({ takeMode: value })} />
                <QualityRow label="摸花品质" value={unionFlower?.takeQualities ?? []} onChange={(value) => updateUnionFlower({ takeQualities: value })} />
                <CatalogFlowerMultiSelectRow
                  label="摸花花朵"
                  value={unionFlower?.takeFlowerIds ?? []}
                  inventory={snapshot?.inventory ?? {}}
                  synced={Boolean(snapshot)}
                  onChange={(value) => updateUnionFlower({ takeFlowerIds: value })}
                  className="sm:col-span-2"
                />
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会竞赛" icon={<Trophy />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="任务池同步" checked={unionRace?.enabled ?? true} description="竞赛期间同步任务池与当前已接任务（只读展示）；关闭后不再拉取竞赛数据" onChange={(checked) => updateUnionRace({ enabled: checked })} />
                <ToggleRow label="自动完成" checked={unionRace?.autoEnableModules ?? false} description="自动接取、推进种植/提交与放弃竞赛任务；默认关闭。未开启时仍会同步并显示已接任务，但不会自动执行" onChange={(checked) => updateUnionRace({ autoEnableModules: checked })} />
                <ToggleRow label="自动启停" checked={unionRace?.autoStopOnQuotaDone ?? true} description="任务次数做完后不再自动接取；已接任务仍会继续完成/放弃。关闭后仅在服务端提示次数用尽时停止接取" onChange={(checked) => updateUnionRace({ autoStopOnQuotaDone: checked })} />
                <ToggleRow label="种植任务使用加速卡" checked={unionRace?.useSpeedupTicketInTask ?? false} description="已接种植收获任务全程可用加速卡。关闭时仍强制保底：任务最后 10 分钟自动对竞赛花使用加速卡" onChange={(checked) => updateUnionRace({ useSpeedupTicketInTask: checked })} />
                <NumberRow label="最低任务分" value={unionRace?.minTaskScore ?? 0} min={0} description="分数不高于此值的任务将被跳过；已接且未完成的同样会自动放弃（需开启自动完成）。0 表示不限制" onChange={(value) => updateUnionRace({ minTaskScore: value })} />
                <ToggleRow label="只接已升级任务" checked={unionRace?.onlyUpgradeTask ?? false} description="只接取已被升级的任务（积分加成更高）" onChange={(checked) => updateUnionRace({ onlyUpgradeTask: checked })} />
                <ToggleRow label="排除他人升级任务" checked={unionRace?.excludeOthersUpgradeTask ?? true} onChange={(checked) => updateUnionRace({ excludeOthersUpgradeTask: checked })} />
                <ToggleRow label="自动升级任务" checked={unionRace?.upgradeTask ?? false} onChange={(checked) => updateUnionRace({ upgradeTask: checked })} status={settingStatusForCapability(capabilities, "union.race.upgrade")} />
                <ToggleRow label="删除低分任务" checked={unionRace?.deleteLowScoreTask ?? false} onChange={(checked) => updateUnionRace({ deleteLowScoreTask: checked })} />
                <NumberRow label="删除分数上限" value={unionRace?.deleteTaskMaxScore ?? 0} min={0} onChange={(value) => updateUnionRace({ deleteTaskMaxScore: value })} />
                <BigIntNumberRow label="元宝上限" value={unionRace?.maxSpendDiamond ?? BigInt(0)} min={0} onChange={(value) => updateUnionRace({ maxSpendDiamond: value })} />
              </div>
              <div className="mt-3 space-y-2">
                <p className="text-xs text-muted-foreground">类型优先级：数字越大越优先接取；0 表示不接取。当前支持自动推进：种植收获、顾客订单、珍珠雇佣、花艺制作/售卖；花种培育仅接取与提交。</p>
                <div className="grid gap-2 sm:grid-cols-2">
                  {RACE_TASK_TYPES.map((task) => (
                    <NumberRow
                      key={task.id}
                      label={task.label}
                      value={unionRace?.taskTypePriority?.[task.id] ?? task.defaultPriority}
                      min={0}
                      description={task.note}
                      onChange={(value) => updateUnionRace({ taskTypePriority: { ...(unionRace?.taskTypePriority ?? {}), [task.id]: value } })}
                    />
                  ))}
                </div>
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会其他" icon={<Sparkles />}>
              <div className="grid gap-2 sm:grid-cols-2">
                {SHOW_UNSUPPORTED_SETTINGS && <ToggleRow label="公会红包" checked={union?.redPacketEnabled ?? false} onChange={(checked) => updateUnion({ redPacketEnabled: checked })} status={settingStatusForCapability(capabilities, "union.red_packet")} />}
                <ToggleRow label="能量森林" checked={union?.forestEnabled ?? false} onChange={(checked) => updateUnion({ forestEnabled: checked })} />
              </div>
            </PolicyGroup>
          </div>
        )}

        {activeTab === "activity" && (
          <div className="space-y-4">
            <div className="grid gap-3">
              {ACTIVITY_MODULES.map((module) => {
                const modulePolicy = activity?.modules[module.id];
                return (
                  <PolicyGroup key={module.id} title={module.label} icon={<Play />}>
                    <div className="grid gap-2 sm:grid-cols-2">
                      <ToggleRow label="启用" checked={modulePolicy?.enabled ?? false} onChange={(checked) => updateActivityModule(module.id, { enabled: checked })} status={settingStatusForCapability(capabilities, `activity.${module.id}`)} />
                      {module.boolParams?.map((param) => (
                        <ToggleRow
                          key={param.key}
                          label={param.label}
                          checked={modulePolicy?.boolParams?.[param.key] ?? false}
                          onChange={(checked) => updateActivityBoolParam(module.id, param.key, checked)}
                        />
                      ))}
                      {module.intParams?.map((param) => (
                        <NumberRow
                          key={param.key}
                          label={param.label}
                          value={safeBigIntToNumber(modulePolicy?.intParams?.[param.key], param.defaultValue)}
                          min={param.min}
                          max={param.max}
                          onChange={(value) => updateActivityIntParam(module.id, param.key, value, param.defaultValue)}
                        />
                      ))}
                    </div>
                  </PolicyGroup>
                );
              })}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function PolicyGroup({ title, icon, children }: { title: string; icon: ReactNode; children: ReactNode }) {
  return (
    <section className="space-y-3 rounded-md border border-border/55 bg-white/34 p-3 dark:bg-white/5">
      <SectionTitle icon={icon}>{title}</SectionTitle>
      {children}
    </section>
  );
}

function StatusRow({ label, value, tone }: { label: string; value: string; tone: "ready" | "muted" }) {
  return (
    <div className="flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Badge variant={tone === "ready" ? "secondary" : "outline"}>{value}</Badge>
    </div>
  );
}

function TextRow({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <div className="flex min-h-9 flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
      <Label className="min-w-0 text-sm">{label}</Label>
      <Input className="h-8 w-full text-right text-sm sm:w-36" value={value} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

function BigIntNumberRow({
  label,
  value,
  min,
  onChange,
}: {
  label: string;
  value: bigint;
  min: number;
  onChange: (value: bigint) => void;
}) {
  const floor = BigInt(min);
  const normalizedValue = value < floor ? floor : value;

  return (
    <div className="flex min-h-12 items-center justify-between gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="min-w-0 leading-5">{label}</Label>
      <NumericStepper
        label={label}
        value={normalizedValue.toString()}
        min={min}
        decrementDisabled={normalizedValue <= floor}
        onDecrement={() => onChange(normalizedValue - BigInt(1))}
        onIncrement={() => onChange(normalizedValue + BigInt(1))}
        onValueChange={(nextValue) => onChange(parseBigInt(nextValue, min))}
        wide
      />
    </div>
  );
}

function NumericStepper({
  label,
  value,
  min,
  max,
  disabled = false,
  decrementDisabled = false,
  incrementDisabled = false,
  wide = false,
  onDecrement,
  onIncrement,
  onValueChange,
}: {
  label: string;
  value: string;
  min: number;
  max?: number;
  disabled?: boolean;
  decrementDisabled?: boolean;
  incrementDisabled?: boolean;
  wide?: boolean;
  onDecrement: () => void;
  onIncrement: () => void;
  onValueChange: (value: string) => void;
}) {
  const buttonClassName =
    "flex h-full items-center justify-center text-muted-foreground transition-colors hover:bg-secondary/80 hover:text-foreground disabled:pointer-events-none disabled:opacity-30";

  return (
    <div
      className={cn(
        "grid h-9 shrink-0 grid-cols-[2.25rem_minmax(0,1fr)_2.25rem] overflow-hidden rounded-lg border border-input/85 bg-white/66 shadow-[inset_0_1px_0_rgba(255,255,255,0.78)] transition-[border-color,box-shadow,background-color] focus-within:border-ring focus-within:bg-white/88 focus-within:ring-3 focus-within:ring-ring/24 dark:bg-input/42 dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.06)] dark:focus-within:bg-input/58",
        wide ? "w-40" : "w-36",
        disabled && "opacity-55",
      )}
    >
      <button
        type="button"
        aria-label={`减少${label}`}
        className={cn(buttonClassName, "border-r border-input/65")}
        disabled={disabled || decrementDisabled}
        onClick={onDecrement}
      >
        <Minus className="size-3.5" />
      </button>
      <input
        type="number"
        inputMode="numeric"
        aria-label={label}
        className="h-full min-w-0 bg-transparent px-1 text-center text-sm font-semibold tabular-nums text-foreground outline-none [appearance:textfield] disabled:cursor-not-allowed [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
        min={min}
        max={max}
        step={1}
        disabled={disabled}
        value={value}
        onChange={(event) => onValueChange(event.target.value)}
      />
      <button
        type="button"
        aria-label={`增加${label}`}
        className={cn(buttonClassName, "border-l border-input/65")}
        disabled={disabled || incrementDisabled}
        onClick={onIncrement}
      >
        <Plus className="size-3.5" />
      </button>
    </div>
  );
}

function IntListRow({
  label,
  value,
  onChange,
  description,
}: {
  label: string;
  value: number[];
  onChange: (value: number[]) => void;
  description?: string;
}) {
  return (
    <div className="space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="text-sm">{label}</Label>
      <Input
        className="h-8 text-sm"
        value={formatIntList(value)}
        onChange={(event) => onChange(parseIntList(event.target.value))}
        placeholder="用逗号分隔 ID"
      />
      {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
    </div>
  );
}

function CatalogFlowerMultiSelectRow({
  label,
  value,
  inventory,
  synced,
  onChange,
  className,
}: {
  label: string;
  value: number[];
  inventory: { [key: number]: number };
  synced: boolean;
  onChange: (value: number[]) => void;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [sortMode, setSortMode] = useState<FlowerPickerSortMode>("stock_asc");
  const selectedSet = useMemo(() => new Set(value), [value]);
  const catalogCount = useMemo(() => allFlowers().length, []);
  const flowers = useMemo(() => {
    const catalogFlowers = allFlowers().map((flower) => {
      const display = flowerDisplay(flower.id);
      return {
        id: flower.id,
        name: display.name,
        seedName: display.seedName,
        color: display.item?.color,
        stock: inventory[flower.id] ?? 0,
      };
    });
    const known = new Set(catalogFlowers.map((flower) => flower.id));
    for (const id of value) {
      if (known.has(id)) continue;
      const display = flowerDisplay(id);
      catalogFlowers.push({
        id,
        name: display.name,
        seedName: display.seedName,
        color: display.item?.color,
        stock: inventory[id] ?? 0,
      });
    }
    return catalogFlowers;
  }, [inventory, value]);
  const visibleFlowers = useMemo(() => {
    const text = query.trim().toLowerCase();
    return flowers
      .filter((flower) => {
        if (!text) return true;
        return String(flower.id).includes(text) || flower.name.toLowerCase().includes(text) || flower.seedName.toLowerCase().includes(text);
      })
      .sort((a, b) => {
        if (sortMode === "stock_desc" && a.stock !== b.stock) return b.stock - a.stock;
        if (a.stock !== b.stock) return a.stock - b.stock;
        return a.id - b.id;
      });
  }, [flowers, query, sortMode]);
  const selectedPreview = value.slice(0, 4).map((id) => itemName(id)).filter(Boolean).join("、");
  const extraCount = value.length > 4 ? value.length - 4 : 0;
  const toggleFlower = (flowerID: number) => onChange(toggleNumber(value, flowerID));

  return (
    <div className={cn("min-w-0 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5", className)}>
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 sm:gap-3">
        <Label className="text-sm">{label}</Label>
        <div className="flex gap-1">
          <Badge variant="outline">花库 {catalogCount}</Badge>
          <Badge variant={value.length > 0 ? "secondary" : "outline"}>{value.length > 0 ? `${value.length} 种` : "未选择"}</Badge>
        </div>
      </div>
      <div className="flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden">
        <div className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {value.length === 0 ? "未选择时不限制" : `${selectedPreview}${extraCount > 0 ? ` 等 ${extraCount} 种` : ""}`}
        </div>
        <Button type="button" variant="outline" size="sm" className="min-h-10 shrink-0 px-3 sm:min-h-7" onClick={() => setOpen(true)}>
          <Flower2 className="size-3.5" />
          选择
        </Button>
      </div>

      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) {
            setQuery("");
            setSortMode("stock_asc");
          }
        }}
      >
        <DialogContent className="flex h-[min(42rem,90dvh)] max-h-[90dvh] max-w-3xl flex-col overflow-hidden">
          <DialogHeader className="mb-3 shrink-0">
            <DialogTitle>{label}</DialogTitle>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <div className="relative min-w-0">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索花名、种子或 ID"
                  className="h-9 pl-9 max-sm:dark:bg-input max-sm:dark:shadow-none max-sm:dark:transition-none max-sm:dark:focus-visible:bg-input"
                />
              </div>
              <Badge variant="outline" className="max-sm:dark:bg-input max-sm:dark:transition-none">已选 {value.length}</Badge>
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
              <span className="shrink-0 text-xs text-muted-foreground">排序</span>
              <div className="flex flex-wrap gap-1">
                {FLOWER_PICKER_SORT_OPTIONS.map((option) => (
                  <FlowerPickerFilterChip
                    key={option.value}
                    selected={sortMode === option.value}
                    onClick={() => setSortMode(option.value)}
                  >
                    {option.label}
                  </FlowerPickerFilterChip>
                ))}
              </div>
              {!synced ? <span className="text-xs text-muted-foreground">登录后同步库存</span> : null}
            </div>
            <div className="dark-scrollbar min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain rounded-md border border-border/58 bg-white/42 p-2 dark:bg-muted">
              {visibleFlowers.length === 0 ? (
                <EmptyState title="没有匹配花朵" detail="换个名称或 ID 再试试" />
              ) : (
                <div className="grid grid-cols-1 gap-2 min-[540px]:grid-cols-2 lg:grid-cols-3">
                  {visibleFlowers.map((flower) => {
                    const selected = selectedSet.has(flower.id);
                    return (
                      <button
                        key={flower.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleFlower(flower.id)}
                        className={cn(
                          "flex min-h-[72px] w-full min-w-0 touch-manipulation items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors max-sm:dark:transition-none",
                          selected
                            ? "border-primary bg-primary/10 text-foreground max-sm:dark:bg-secondary"
                            : "border-border/58 bg-card/72 hover:bg-white/66 dark:hover:bg-white/8 max-sm:dark:bg-card max-sm:dark:hover:bg-card",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded border",
                            selected ? "border-primary bg-primary text-primary-foreground" : "border-border bg-white/54 text-transparent dark:bg-input",
                          )}
                        >
                          <Check className="size-3" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="truncate text-sm font-medium">{flower.name}</span>
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            <span>{flower.id}</span>
                            {flower.color ? <span>品质 {flower.color}</span> : null}
                            <span>库存 {formatCount(flower.stock)}</span>
                            {flower.seedName ? <span>{flower.seedName}</span> : null}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="mt-3 shrink-0 flex-row items-center justify-between border-t border-border/58 pt-3 [&>button]:min-h-10 [&>button]:min-w-24">
            <Button type="button" variant="ghost" className="max-sm:dark:bg-card max-sm:dark:transition-none max-sm:dark:hover:bg-muted" onClick={() => onChange([])} disabled={value.length === 0}>
              清空
            </Button>
            <Button
              type="button"
              className="max-sm:dark:transition-none"
              onClick={() => {
                setOpen(false);
                setQuery("");
                setSortMode("stock_asc");
              }}
            >
              完成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

type FlowerPickerOption = {
  id: number;
  name: string;
  seedName: string;
  stock: number;
  gold: number;
  experience: number;
  lvl: number;
  cdSeconds: number;
  quality: number;
  plantable: boolean;
};

function resetFlowerPickerFilters(
  setQuery: (value: string) => void,
  setQualityFilter: (value: number[]) => void,
  setLevelFilter: (value: number[]) => void,
  setSortMode: (value: FlowerPickerSortMode) => void,
  defaultSortMode: FlowerPickerSortMode = "stock_asc",
) {
  setQuery("");
  setQualityFilter([]);
  setLevelFilter([]);
  setSortMode(defaultSortMode);
}

function compareFlowerPickerOptions(a: FlowerPickerOption, b: FlowerPickerOption, sortMode: FlowerPickerSortMode) {
  if (a.plantable !== b.plantable) return a.plantable ? -1 : 1;
  if (sortMode === "mature_asc" || sortMode === "mature_desc") {
    const aCD = a.cdSeconds > 0 ? a.cdSeconds : Number.POSITIVE_INFINITY;
    const bCD = b.cdSeconds > 0 ? b.cdSeconds : Number.POSITIVE_INFINITY;
    if (aCD !== bCD) return sortMode === "mature_desc" ? bCD - aCD : aCD - bCD;
    if (a.stock !== b.stock) return a.stock - b.stock;
    return a.id - b.id;
  }
  if (sortMode === "stock_desc" && a.stock !== b.stock) return b.stock - a.stock;
  if (a.stock !== b.stock) return a.stock - b.stock;
  return a.id - b.id;
}

function formatFlowerMatureDuration(cdSeconds: number) {
  if (cdSeconds <= 0) return "";
  if (cdSeconds < 60) return `${cdSeconds}秒`;
  const minutes = Math.floor(cdSeconds / 60);
  const seconds = cdSeconds % 60;
  if (minutes < 60) {
    return seconds > 0 ? `${minutes}分${seconds}秒` : `${minutes}分钟`;
  }
  const hours = Math.floor(minutes / 60);
  const remMinutes = minutes % 60;
  return remMinutes > 0 ? `${hours}小时${remMinutes}分` : `${hours}小时`;
}

function FlowerPickerFilterChip({
  selected,
  onClick,
  children,
}: {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex h-7 min-w-7 items-center justify-center rounded border px-1.5 text-xs font-medium",
        selected
          ? "border-primary bg-primary text-primary-foreground"
          : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
      )}
    >
      {children}
    </button>
  );
}

function FlowerMultiSelectRow({
  label,
  value,
  plantableFlowers,
  synced,
  onChange,
}: {
  label: string;
  value: number[];
  plantableFlowers: PlantableFlowerView[];
  synced: boolean;
  onChange: (value: number[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [qualityFilter, setQualityFilter] = useState<number[]>([]);
  const [levelFilter, setLevelFilter] = useState<number[]>([]);
  const [sortMode, setSortMode] = useState<FlowerPickerSortMode>("mature_asc");
  const selectedSet = useMemo(() => new Set(value), [value]);
  const flowers = useMemo<FlowerPickerOption[]>(() => {
    const options = plantableFlowers.map((flower) => {
      const display = flowerDisplay(flower.flowerId);
      return {
        id: flower.flowerId,
        name: flower.flowerName || display.name,
        seedName: display.seedName,
        stock: flower.stock,
        gold: flower.gold,
        experience: flower.experience,
        lvl: flower.lvl,
        // Always recompute from current cultivation level (not catalog lvl1 / base row alone).
        cdSeconds: flowerMatureCdSeconds(flower.flowerId, flower.lvl) || flower.cdSeconds,
        quality: display.item?.color ?? 0,
        plantable: true,
      };
    });
    const known = new Set(options.map((option) => option.id));
    for (const id of value) {
      if (known.has(id)) continue;
      const display = flowerDisplay(id);
      options.push({
        id,
        name: display.name,
        seedName: display.seedName,
        stock: 0,
        gold: display.flower?.gold ?? 0,
        experience: display.flower?.experience ?? 0,
        lvl: 0,
        cdSeconds: 0,
        quality: display.item?.color ?? 0,
        plantable: false,
      });
    }
    return options;
  }, [plantableFlowers, value]);
  const availableLevels = useMemo(() => {
    const levels = new Set<number>();
    for (const flower of flowers) {
      if (flower.lvl > 0) levels.add(flower.lvl);
    }
    return [...levels].sort((a, b) => a - b);
  }, [flowers]);
  const qualityCounts = useMemo(() => {
    const counts: Record<number, number> = {};
    for (const quality of QUALITY_OPTIONS) counts[quality] = 0;
    for (const flower of flowers) {
      if (flower.quality > 0) counts[flower.quality] = (counts[flower.quality] ?? 0) + 1;
    }
    return counts;
  }, [flowers]);
  const visibleFlowers = useMemo(() => {
    const text = query.trim().toLowerCase();
    const qualitySet = qualityFilter.length > 0 ? new Set(qualityFilter) : null;
    const levelSet = levelFilter.length > 0 ? new Set(levelFilter) : null;
    return flowers
      .filter((flower) => {
        if (qualitySet && !qualitySet.has(flower.quality)) return false;
        if (levelSet && !levelSet.has(flower.lvl)) return false;
        if (!text) return true;
        const qualityLabel = QUALITY_LABELS[flower.quality] ?? "";
        return (
          String(flower.id).includes(text) ||
          flower.name.toLowerCase().includes(text) ||
          flower.seedName.toLowerCase().includes(text) ||
          qualityLabel.includes(text) ||
          (flower.lvl > 0 && (`lv${flower.lvl}` === text || `等级${flower.lvl}` === text || String(flower.lvl) === text))
        );
      })
      .sort((a, b) => compareFlowerPickerOptions(a, b, sortMode));
  }, [flowers, levelFilter, qualityFilter, query, sortMode]);
  const selectedPreview = value.slice(0, 4).map((id) => itemName(id)).filter(Boolean).join("、");
  const extraCount = value.length > 4 ? value.length - 4 : 0;
  const toggleFlower = (flowerID: number) => onChange(toggleNumber(value, flowerID));
  const filterActive = qualityFilter.length > 0 || levelFilter.length > 0;

  return (
    <div className="min-w-0 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 sm:gap-3">
        <Label className="text-sm">{label}</Label>
        <div className="flex gap-1">
          <Badge variant="outline">可种 {plantableFlowers.length}</Badge>
          <Badge variant={value.length > 0 ? "secondary" : "outline"}>{value.length > 0 ? `${value.length} 种` : "未选择"}</Badge>
        </div>
      </div>
      <div className="flex min-h-8 w-full min-w-0 items-center gap-2 overflow-hidden">
        <div className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {value.length === 0 ? "未选择时不限制" : `${selectedPreview}${extraCount > 0 ? ` 等 ${extraCount} 种` : ""}`}
        </div>
        <Button type="button" variant="outline" size="sm" className="min-h-10 shrink-0 px-3 sm:min-h-7" onClick={() => setOpen(true)}>
          <Flower2 className="size-3.5" />
          选择
        </Button>
      </div>

      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) resetFlowerPickerFilters(setQuery, setQualityFilter, setLevelFilter, setSortMode, "mature_asc");
        }}
      >
        <DialogContent className="flex h-[min(42rem,90dvh)] max-h-[90dvh] max-w-3xl flex-col overflow-hidden">
          <DialogHeader className="mb-3 shrink-0">
            <DialogTitle>{label}</DialogTitle>
          </DialogHeader>
          <div className="flex min-h-0 flex-1 flex-col gap-3">
            <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
              <div className="relative min-w-0">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="搜索花名、种子、品质或 ID"
                  className="h-9 pl-9 max-sm:dark:bg-input max-sm:dark:shadow-none max-sm:dark:transition-none max-sm:dark:focus-visible:bg-input"
                />
              </div>
              <Badge variant="outline" className="max-sm:dark:bg-input max-sm:dark:transition-none">已选 {value.length}</Badge>
            </div>
            <div className="flex shrink-0 flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="shrink-0 text-xs text-muted-foreground">品质</span>
                <div className="flex flex-wrap gap-1">
                  {QUALITY_OPTIONS.map((quality) => (
                    <FlowerPickerFilterChip
                      key={quality}
                      selected={qualityFilter.includes(quality)}
                      onClick={() => setQualityFilter((current) => toggleNumber(current, quality))}
                    >
                      {QUALITY_LABELS[quality]}({qualityCounts[quality] ?? 0})
                    </FlowerPickerFilterChip>
                  ))}
                </div>
              </div>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="shrink-0 text-xs text-muted-foreground">等级</span>
                <div className="flex flex-wrap gap-1">
                  {availableLevels.length === 0 ? (
                    <span className="text-xs text-muted-foreground">{synced ? "暂无等级数据" : "登录后同步等级"}</span>
                  ) : (
                    availableLevels.map((level) => (
                      <FlowerPickerFilterChip
                        key={level}
                        selected={levelFilter.includes(level)}
                        onClick={() => setLevelFilter((current) => toggleNumber(current, level))}
                      >
                        Lv.{level}
                      </FlowerPickerFilterChip>
                    ))
                  )}
                </div>
                {filterActive ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="ml-auto h-7 px-2 text-xs"
                    onClick={() => {
                      setQualityFilter([]);
                      setLevelFilter([]);
                    }}
                  >
                    清除筛选
                  </Button>
                ) : null}
              </div>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="shrink-0 text-xs text-muted-foreground">排序</span>
                <div className="flex flex-wrap gap-1">
                  {PLANTABLE_FLOWER_PICKER_SORT_OPTIONS.map((option) => (
                    <FlowerPickerFilterChip
                      key={option.value}
                      selected={sortMode === option.value}
                      onClick={() => setSortMode(option.value)}
                    >
                      {option.label}
                    </FlowerPickerFilterChip>
                  ))}
                </div>
              </div>
            </div>
            <div className="dark-scrollbar min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain rounded-md border border-border/58 bg-white/42 p-2 dark:bg-muted">
              {visibleFlowers.length === 0 ? (
                <EmptyState
                  title={synced ? "没有匹配花种" : "尚未同步可种花种"}
                  detail={synced ? (filterActive || query.trim() ? "试试调整品质/等级筛选或搜索词" : undefined) : "登录账号并同步培育状态后可选择"}
                />
              ) : (
                <div className="grid grid-cols-1 gap-2 min-[540px]:grid-cols-2 lg:grid-cols-3">
                  {visibleFlowers.map((flower) => {
                    const selected = selectedSet.has(flower.id);
                    const qualityLabel = flower.quality > 0 ? QUALITY_LABELS[flower.quality] : "";
                    const matureLabel = formatFlowerMatureDuration(flower.cdSeconds);
                    return (
                      <button
                        key={flower.id}
                        type="button"
                        aria-pressed={selected}
                        onClick={() => toggleFlower(flower.id)}
                        className={cn(
                          "flex min-h-[72px] w-full min-w-0 touch-manipulation items-start gap-2 rounded-md border px-3 py-2 text-left transition-colors max-sm:dark:transition-none",
                          selected
                            ? "border-primary bg-primary/10 text-foreground max-sm:dark:bg-secondary"
                            : "border-border/58 bg-card/72 hover:bg-white/66 dark:hover:bg-white/8 max-sm:dark:bg-card max-sm:dark:hover:bg-card",
                        )}
                      >
                        <span
                          className={cn(
                            "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded border",
                            selected ? "border-primary bg-primary text-primary-foreground" : "border-border bg-white/54 text-transparent dark:bg-input",
                          )}
                        >
                          <Check className="size-3" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex min-w-0 items-center gap-1.5">
                            <span className="shrink-0 text-xs text-muted-foreground">{flower.id}</span>
                            <span className="truncate text-sm font-medium">{flower.name}</span>
                            {flower.lvl > 0 ? <Badge variant="secondary">Lv.{flower.lvl}</Badge> : null}
                            {!flower.plantable && <Badge variant="outline">当前不可种</Badge>}
                          </span>
                          {matureLabel ? (
                            <span className="mt-1 block text-xs text-muted-foreground">
                              成熟 {matureLabel}
                              {flower.lvl > 0 ? `（按 Lv.${flower.lvl}）` : null}
                            </span>
                          ) : null}
                          <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            {qualityLabel ? <span>{qualityLabel}</span> : null}
                            {flower.stock > 0 ? <span>库存 {formatCount(flower.stock)}</span> : null}
                            {flower.gold ? <span>金币 {formatCount(flower.gold)}</span> : null}
                          </span>
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="mt-3 shrink-0 flex-row items-center justify-between border-t border-border/58 pt-3 [&>button]:min-h-10 [&>button]:min-w-24">
            <Button
              type="button"
              variant="ghost"
              className="max-sm:dark:bg-card max-sm:dark:transition-none max-sm:dark:hover:bg-muted"
              onClick={() => onChange([])}
              disabled={value.length === 0}
            >
              清空
            </Button>
            <Button
              type="button"
              className="max-sm:dark:transition-none"
              onClick={() => {
                setOpen(false);
                resetFlowerPickerFilters(setQuery, setQualityFilter, setLevelFilter, setSortMode, "mature_asc");
              }}
            >
              完成
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function QualityRow({
  label,
  value,
  onChange,
  labels,
  emptyMeansAll = false,
}: {
  label: string;
  value: number[];
  onChange: (value: number[]) => void;
  labels?: Record<number, string>;
  emptyMeansAll?: boolean;
}) {
  const selectedSet = useMemo(() => {
    if (emptyMeansAll && value.length === 0) return new Set(QUALITY_OPTIONS);
    return new Set(value);
  }, [emptyMeansAll, value]);

  const toggleQuality = (quality: number) => {
    const current = emptyMeansAll && value.length === 0 ? [...QUALITY_OPTIONS] : value;
    const next = toggleNumber(current, quality);
    if (emptyMeansAll && next.length === QUALITY_OPTIONS.length) {
      onChange([]);
      return;
    }
    onChange(next);
  };

  return (
    <div className="flex min-h-9 flex-col gap-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
      <Label className="min-w-0 text-sm">{label}</Label>
      <div className="flex gap-1">
        {QUALITY_OPTIONS.map((quality) => {
          const selected = selectedSet.has(quality);
          return (
            <button
              key={quality}
              type="button"
              onClick={() => toggleQuality(quality)}
              className={cn(
                "flex h-7 min-w-7 items-center justify-center rounded border px-1.5 text-xs font-medium",
                selected ? "border-primary bg-primary text-primary-foreground" : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
              )}
            >
              {labels?.[quality] ?? quality}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function SegmentedRow<T extends number>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { value: T; label: string }[];
  onChange: (value: T) => void;
}) {
  return (
    <div className="space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <Label className="text-sm">{label}</Label>
      <div className="flex flex-wrap gap-1">
        {options.map((option) => (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cn(
              "min-h-8 rounded border px-2 text-xs font-medium",
              option.value === value ? "border-primary bg-primary text-primary-foreground" : "border-border/58 bg-white/42 text-muted-foreground hover:bg-white/68 hover:text-foreground dark:bg-white/5",
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function DemandPriorityEditor({
  value,
  onChange,
}: {
  value: Record<string, number>;
  onChange: (value: Record<string, number>) => void;
}) {
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const orderedGoals = useMemo(() => {
    return [...GOAL_OPTIONS].sort((a, b) => {
      const priorityDelta = priorityForGoal(value, b) - priorityForGoal(value, a);
      if (priorityDelta !== 0) return priorityDelta;
      return GOAL_OPTIONS.findIndex((goal) => goal.id === a.id) - GOAL_OPTIONS.findIndex((goal) => goal.id === b.id);
    });
  }, [value]);

  const commitOrder = (goals: typeof GOAL_OPTIONS) => {
    const total = goals.length;
    onChange(Object.fromEntries(goals.map((goal, index) => [goal.id, (total - index) * 10])));
  };

  const moveGoal = (sourceId: string, targetId: string) => {
    if (sourceId === targetId) return;
    const from = orderedGoals.findIndex((goal) => goal.id === sourceId);
    const to = orderedGoals.findIndex((goal) => goal.id === targetId);
    if (from < 0 || to < 0) return;
    const next = [...orderedGoals];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    commitOrder(next);
  };

  const handlePointerUp = (event: PointerEvent<HTMLDivElement>, sourceId: string) => {
    const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-goal-id]");
    const targetId = target?.dataset.goalId;
    if (targetId) moveGoal(sourceId, targetId);
    setDraggingId(null);
  };

  return (
    <div className="mt-3 space-y-2 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs text-muted-foreground">拖拽调整缺花补种顺序</span>
      </div>
      <div
        className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3"
        onMouseUp={(event) => {
          if (!draggingId) return;
          const target = document.elementFromPoint(event.clientX, event.clientY)?.closest<HTMLElement>("[data-goal-id]");
          const targetId = target?.dataset.goalId;
          if (targetId) moveGoal(draggingId, targetId);
          setDraggingId(null);
        }}
        onMouseLeave={(event) => {
          if (event.buttons === 0) setDraggingId(null);
        }}
      >
        {orderedGoals.map((goal, index) => (
          <div
            key={goal.id}
            data-goal-id={goal.id}
            aria-grabbed={draggingId === goal.id}
            onMouseDown={(event) => {
              if (event.button !== 0) return;
              setDraggingId(goal.id);
            }}
            onPointerDown={(event) => {
              if (event.button !== 0) return;
              event.currentTarget.setPointerCapture(event.pointerId);
              setDraggingId(goal.id);
            }}
            onPointerUp={(event) => handlePointerUp(event, goal.id)}
            onPointerCancel={() => setDraggingId(null)}
            className={cn(
              "flex min-h-11 cursor-grab touch-none items-center gap-2 rounded-md border border-border/70 bg-card px-2.5 py-2 text-sm shadow-sm transition active:cursor-grabbing",
              draggingId === goal.id ? "opacity-60 ring-1 ring-primary" : "hover:border-primary/50 hover:bg-white/66 dark:hover:bg-white/8",
            )}
          >
            <GripVertical className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <span className="flex size-6 shrink-0 items-center justify-center rounded bg-secondary text-xs font-medium text-muted-foreground dark:bg-white/8">{index + 1}</span>
            <span className="min-w-0 flex-1 truncate font-medium">{goal.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function priorityForGoal(value: Record<string, number>, goal: (typeof GOAL_OPTIONS)[number]) {
  return value[goal.id] || goal.defaultPriority;
}
function ToggleRow({
  label,
  checked,
  onChange,
  status,
  description,
  disabled = false,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  status?: SettingStatus;
  description?: string;
  disabled?: boolean;
}) {
  return (
    <div
      className={cn(
        "flex min-h-9 items-center justify-between gap-3 rounded-md border border-border/55 bg-white/36 px-3 py-2 dark:bg-white/5",
        disabled && "opacity-55",
      )}
    >
      <span className="flex min-w-0 flex-wrap items-center gap-2 text-sm">
        <span className="flex flex-col">
          <span>{label}</span>
          {description && <span className="text-xs text-muted-foreground">{description}</span>}
        </span>
        {status && <SettingStatusBadge status={status} />}
      </span>
      <Switch checked={checked} disabled={disabled} onCheckedChange={onChange} />
    </div>
  );
}

function SettingStatusBadge({ status }: { status: SettingStatus }) {
  const variant = status.kind === "sync_only" ? "outline" : "destructive";
  return (
    <Badge variant={variant} title={status.detail} className="shrink-0">
      {status.label}
    </Badge>
  );
}

function NumberRow({
  label,
  value,
  min,
  max,
  disabled = false,
  onChange,
  description,
}: {
  label: string;
  value: number;
  min: number;
  max?: number;
  disabled?: boolean;
  onChange: (value: number) => void;
  description?: string;
}) {
  const normalizedValue = Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min, Number.isFinite(value) ? Math.trunc(value) : min));
  const updateValue = (nextValue: number) => onChange(Math.min(max ?? Number.POSITIVE_INFINITY, Math.max(min, nextValue)));

  return (
    <div
      className={cn(
        "flex min-h-12 items-center justify-between gap-3 rounded-lg border border-border/55 bg-white/36 px-3 py-2 transition-opacity dark:bg-white/5",
        disabled && "opacity-55",
      )}
    >
      <Label className="flex min-w-0 flex-col leading-5">
        <span>{label}</span>
        {description && <span className="text-xs font-normal text-muted-foreground">{description}</span>}
      </Label>
      <NumericStepper
        label={label}
        value={normalizedValue.toString()}
        min={min}
        max={max}
        disabled={disabled}
        decrementDisabled={normalizedValue <= min}
        incrementDisabled={max !== undefined && normalizedValue >= max}
        onDecrement={() => updateValue(normalizedValue - 1)}
        onIncrement={() => updateValue(normalizedValue + 1)}
        onValueChange={(nextValue) => updateValue(parseNumber(nextValue, min))}
      />
    </div>
  );
}

function SectionTitle({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className="flex items-center gap-2 text-sm font-semibold">
      <span className="flex size-7 items-center justify-center rounded-md bg-secondary text-sky-600 dark:bg-white/8 dark:text-sky-300 [&_svg]:size-4">{icon}</span>
      {children}
    </div>
  );
}

function EmptyState({ title, detail }: { title: string; detail?: string }) {
  return (
    <div className="rounded-md border border-dashed border-border/70 bg-white/32 px-3 py-4 text-center dark:bg-white/5">
      <Sparkles className="mx-auto mb-2 size-4 text-amber-400" />
      <div className="text-sm text-muted-foreground">{title}</div>
      {detail && <div className="mt-1 text-xs text-muted-foreground/80">{detail}</div>}
    </div>
  );
}

function formatCount(value: number | bigint) {
  const numeric = typeof value === "bigint" ? Number(value) : value;
  if (!Number.isFinite(numeric)) return "0";
  return NUMBER_FORMATTER.format(numeric);
}

function parseNumber(value: string, min: number) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return min;
  return Math.max(min, Math.trunc(parsed));
}

function parseBigInt(value: string, min: number) {
  const cleaned = value.trim();
  if (!cleaned) return BigInt(min);
  try {
    const parsed = BigInt(cleaned);
    const floor = BigInt(min);
    return parsed < floor ? floor : parsed;
  } catch {
    return BigInt(min);
  }
}

function safeBigIntToNumber(value: bigint | undefined, fallback: number) {
  if (value === undefined) return fallback;
  const upper = BigInt(Number.MAX_SAFE_INTEGER);
  const lower = BigInt(Number.MIN_SAFE_INTEGER);
  if (value > upper) return Number.MAX_SAFE_INTEGER;
  if (value < lower) return Number.MIN_SAFE_INTEGER;
  return Number(value);
}

function safeNumberToBigInt(value: number, fallback: number) {
  if (!Number.isFinite(value)) return BigInt(fallback);
  const integer = Math.trunc(value);
  if (integer > Number.MAX_SAFE_INTEGER) return BigInt(Number.MAX_SAFE_INTEGER);
  if (integer < Number.MIN_SAFE_INTEGER) return BigInt(Number.MIN_SAFE_INTEGER);
  return BigInt(integer);
}

function parseIntList(value: string) {
  const seen = new Set<number>();
  const out: number[] = [];
  for (const part of value.split(/[,\s，、]+/)) {
    const parsed = Number(part.trim());
    if (!Number.isInteger(parsed) || parsed <= 0 || seen.has(parsed)) continue;
    seen.add(parsed);
    out.push(parsed);
  }
  return out;
}

function formatIntList(value: number[]) {
  return value.join(", ");
}

function toggleNumber(values: number[], value: number) {
  if (values.includes(value)) return values.filter((item) => item !== value);
  return [...values, value].sort((a, b) => a - b);
}
