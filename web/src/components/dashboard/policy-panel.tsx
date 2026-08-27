"use client";

import { create } from "@bufbuild/protobuf";
import { BadgeCheck, Building2, Coins, Flower2, Gem, HandCoins, ListChecks, Loader2, Package, Play, Save, ShieldCheck, ShoppingBag, Sparkles, Sprout, Trophy } from "lucide-react";
import { ActivityPolicySchema, BasicPolicySchema, BasicTaskPolicySchema, BenefitPolicySchema, CultivatePolicySchema, CustomerOrderPolicySchema, CyclicNotePolicySchema, CyclicStoryPolicySchema, DessertPolicySchema, FlowerElvesPolicySchema, FlowerMarketPolicySchema, FlowerArtPolicySchema, FriendStealPolicySchema, MarketBuyMode, MarketPutMode, OrderPolicySchema, PalaceOrderPolicySchema, PearlPolicySchema, PlantPolicySchema, PlantingPolicySchema, ReputationPolicySchema, ResidentOrderPolicySchema, SelectionMode, SignPolicySchema, ShopBuyPolicySchema, ShopPolicySchema, TeamOrderPolicySchema, UnionBuildPolicySchema, UnionFlowerPolicySchema, UnionLandPolicySchema, UnionPolicySchema, UnionRacePolicySchema, VipShopPolicySchema, ZooPolicySchema } from "@/gen/mygardenworld/v1/policy_pb";
import type { ActivityPolicy, BasicPolicy, BasicTaskPolicy, BenefitPolicy, CultivatePolicy, CustomerOrderPolicy, CyclicNotePolicy, CyclicStoryPolicy, DessertPolicy, FlowerElvesPolicy, FlowerMarketPolicy, FlowerArtPolicy, FriendStealPolicy, OrderPolicy, PalaceOrderPolicy, PearlPolicy, PlantPolicy, PlantingPolicy, Policy, ReputationPolicy, ResidentOrderPolicy, ShopBuyPolicy, ShopPolicy, SignPolicy, TeamOrderPolicy, UnionBuildPolicy, UnionFlowerPolicy, UnionLandPolicy, UnionPolicy, UnionRacePolicy, VipShopPolicy, ZooPolicy } from "@/gen/mygardenworld/v1/policy_pb";
import type { AssetsView, FeatureCapability, GardenView, OrdersView, OverviewView } from "@/lib/api/query-models";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { settingStatusForCapability } from "@/lib/feature-capabilities";
import { PolicyGroup, StatusRow, TextRow, BigIntNumberRow, IntListRow, QualityRow, SegmentedRow, DemandPriorityEditor, ToggleRow, NumberRow, SectionTitle, EmptyState, safeBigIntToNumber, safeNumberToBigInt, QUALITY_LABELS } from "@/components/dashboard/policy-controls";
import { FlowerArtMultiSelectRow, CatalogFlowerMultiSelectRow, FlowerMultiSelectRow } from "@/components/dashboard/flower-picker-controls";
import FriendStealPolicyGroup from "@/features/account-workspace/friend-steal-policy-group";

const SHOW_UNSUPPORTED_SETTINGS = false;

export type PolicySection = "garden" | "routine" | "orders" | "union" | "assets" | "activities";

const POLICY_SECTION_TITLES: Record<PolicySection, string> = {
  garden: "花园策略",
  routine: "账号与日常策略",
  orders: "订单经营策略",
  union: "公会策略",
  assets: "资产策略",
  activities: "活动策略",
};

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
  { id: 3030, label: "花艺售卖", defaultPriority: 0, note: "不要求「自动上架」；上架满5分钟会全部下架再挂；缺成品先按制作规则做最高价有种子花艺；上架时选库存数量最多的可售花艺" },
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

export default function PolicyPanel({
  policy,
  section,
  overview,
  garden,
  orders,
  assets,
  capabilities,
  loading,
  saving,
  message,
  onPolicyChange,
  onSave,
}: {
  policy: Policy | null;
  section: PolicySection;
  overview: OverviewView | null;
  garden: GardenView | null;
  orders: OrdersView | null;
  assets: AssetsView | null;
  capabilities: FeatureCapability[];
  loading: boolean;
  saving: boolean;
  message: string;
  onPolicyChange: (policy: Policy | null) => void;
  onSave: () => void;
}) {
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
  const customerOrdersObserved = overview?.observedNamespaces.includes("109") ?? false;
  const customerFinishedToday = orders?.orderStatistics?.customerFinished ?? 0;
  const customerStatsObserved = orders?.orderStatistics?.observed ?? false;
  const customerDailyLimit = customer?.dailyLimit ?? 0;
  const customerLimitReached = customerDailyLimit > 0 && customerFinishedToday >= customerDailyLimit;
  const customerPendingCount = orders?.pendingTasks.filter((taskItem) => taskItem.category === "顾客订单").length ?? 0;
  const customerOrderStatusLabel = !orders
    ? "状态未加载"
    : !customerStatsObserved
      ? customerOrdersObserved
        ? `今日进度未同步（当前挂单 ${customerPendingCount}）`
        : "未同步订单统计"
      : customerDailyLimit > 0
        ? `今日已完成 ${customerFinishedToday}/${customerDailyLimit}`
        : `今日已完成 ${customerFinishedToday}`;
  const customerOrderStatusTone = !orders || !customerStatsObserved ? "muted" : customerLimitReached ? "warn" : "ready";

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
    onPolicyChange({ ...policy, basic: create(BasicPolicySchema, { ...current, ...patch }) });
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
  const updateFriendTouchCount = (uid: bigint, count: number) => {
    const key = uid.toString();
    const next = { ...(friendSteal?.friendCounts ?? {}) };
    if (count <= 0) {
      delete next[key];
    } else {
      next[key] = count;
    }
    updateFriendSteal({ friendCounts: next });
  };
  const updateFriendTouchExcluded = (uid: bigint, excluded: boolean) => {
    const current = friendSteal?.excludeUids ?? [];
    const next = excluded
      ? current.includes(uid)
        ? current
        : [...current, uid]
      : current.filter((value) => value !== uid);
    updateFriendSteal({ excludeUids: next });
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
  const updateCyclicNote = (patch: Partial<CyclicNotePolicy>) => {
    const current = activity?.cyclicNote ?? create(CyclicNotePolicySchema);
    updateActivity({ cyclicNote: create(CyclicNotePolicySchema, { ...current, ...patch }) });
  };
  const updateCyclicStory = (patch: Partial<CyclicStoryPolicy>) => {
    const current = activity?.cyclicStory ?? create(CyclicStoryPolicySchema);
    updateActivity({ cyclicStory: create(CyclicStoryPolicySchema, { ...current, ...patch }) });
  };
  const updateDessert = (patch: Partial<DessertPolicy>) => {
    const current = activity?.dessert ?? create(DessertPolicySchema);
    updateActivity({ dessert: create(DessertPolicySchema, { ...current, ...patch }) });
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
    <Card className="overflow-visible">
      <CardHeader>
        <CardTitle>{POLICY_SECTION_TITLES[section]}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {message && <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-sm">{message}</div>}

        {section === "routine" && <section className="space-y-3">
          <SectionTitle icon={<ShieldCheck />}>运行参数</SectionTitle>
          <div className="grid gap-2 sm:grid-cols-2">
            <NumberRow label="决策间隔" value={policy.decisionIntervalSeconds || 4} min={1} onChange={(value) => updatePolicy({ decisionIntervalSeconds: value })} />
          </div>
        </section>}

        <div className="flex justify-end xl:sticky xl:top-0 xl:z-10 xl:-mx-4 xl:border-b xl:border-border/55 xl:bg-card/92 xl:px-4 xl:py-3 xl:backdrop-blur-xl">
          <Button type="button" size="sm" className="w-full shrink-0 sm:w-auto" onClick={onSave} disabled={saving}>
            {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
            {saving ? "保存中" : "保存"}
          </Button>
        </div>

        {section === "garden" && (
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
                    plantableFlowers={garden?.plantableFlowers ?? []}
                    synced={Boolean(garden)}
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
                <ToggleRow label="鲜花升级" checked={cultivate?.upgradeEnabled ?? false} onChange={(checked) => updateCultivate({ upgradeEnabled: checked })} />
                <NumberRow label="目标等级" value={cultivate?.targetLevel || 20} min={1} onChange={(value) => updateCultivate({ targetLevel: value })} />
              </div>
            </PolicyGroup>

            <FriendStealPolicyGroup
              policy={friendSteal}
              garden={garden}
              assets={assets}
              capabilities={capabilities}
              onChange={updateFriendSteal}
              onCountChange={updateFriendTouchCount}
              onExcludedChange={updateFriendTouchExcluded}
            />
          </div>
        )}

        {section === "routine" && (
          <div className="space-y-4">
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

        {section === "assets" && (
          <div className="space-y-4">
            <PolicyGroup title="邮件、福利、签到" icon={<BadgeCheck />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="邮件" checked={basic?.mailEnabled ?? false} onChange={(checked) => updateBasic({ mailEnabled: checked })} />
                <ToggleRow label="水车水滴" checked={basic?.waterwheelEnabled ?? false} onChange={(checked) => updateBasic({ waterwheelEnabled: checked })} description="广告桶仅使用服务端明确支持的 skip→recv 路径，不触发或伪造广告 SDK 回调；每次约3–7滴，普通桶约30滴" />
                <ToggleRow label="限时水滴" checked={basic?.freeWaterEnabled ?? false} onChange={(checked) => updateBasic({ freeWaterEnabled: checked })} />
                <NumberRow label="水滴领取阈值" value={basic?.waterClaimThreshold || 0} min={0} onChange={(value) => updateBasic({ waterClaimThreshold: value })} description="当前水滴≥该值时暂停水车/限时领取；0=不限制。与自然恢复上限(如130)无关" />
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
              <p className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-xs leading-5 text-muted-foreground">
                激励视频和广告礼包不提供自动化：系统不会伪造广告 SDK 回调或 token。仅支持协议明确允许直接领取或跳过广告的流程。
              </p>
              <div className="grid gap-2 sm:grid-cols-2">
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

        {section === "orders" && (
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
                <NumberRow
                  label="每日上限"
                  value={customer?.dailyLimit ?? 0}
                  min={0}
                  max={9999}
                  disabled={!(customer?.enabled ?? false)}
                  description="0 表示不限制；按今日已完成次数生效"
                  onChange={(value) => updateCustomer({ dailyLimit: value })}
                />
                <NumberRow
                  label="最少花艺"
                  value={customer?.minFlowerArtCount ?? 0}
                  min={0}
                  max={3}
                  disabled={!(customer?.enabled ?? false)}
                  description="0 不限；设 2 只做需 2/3 件花艺的单，设 3 只做需 3 件的单；已接竞赛顾客任务时不受此限"
                  onChange={(value) => updateCustomer({ minFlowerArtCount: value })}
                />
                <ToggleRow label="暂时无货" checked={customer?.rejectUnavailableEnabled ?? false} onChange={(checked) => updateCustomer({ rejectUnavailableEnabled: checked })} />
                <StatusRow label="今日进度" value={customerOrderStatusLabel} tone={customerOrderStatusTone} />
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
                <FlowerArtMultiSelectRow
                  label="上架花艺"
                  value={flowerArt?.sellArtIds ?? []}
                  sellableArts={orders?.sellableFlowerArts ?? []}
                  synced={Boolean(orders)}
                  onChange={(value) => updateFlowerArt({ sellArtIds: value })}
                />
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

        {section === "union" && (
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
                  description="未满11级：强制换种低等级花练级（距成熟≤2分钟则等收获后再换）。全部达到11级后：才按成熟时长选种；指定花朵非空时只种这些 ID（莹白露薇=23117）。"
                />
                <NumberRow
                  label="改种冷却(分钟)"
                  value={unionLand?.minReplantMinutes || 60}
                  min={1}
                  onChange={(value) => updateUnionLand({ minReplantMinutes: value })}
                  description="全部≥11级后：空地随时补种；已种地块需满此时间才允许改种其他花，多种花可并存。未满11级练级换种不受此限制。"
                />
                <FlowerMultiSelectRow
                  label="指定花朵"
                  value={unionLand?.flowerIds ?? []}
                  plantableFlowers={garden?.plantableFlowers ?? []}
                  synced={Boolean(garden)}
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
                  inventory={assets?.inventory ?? {}}
                  synced={Boolean(assets)}
                  onChange={(value) => updateUnionFlower({ takeFlowerIds: value })}
                  className="sm:col-span-2"
                />
              </div>
            </PolicyGroup>

            <PolicyGroup title="公会竞赛" icon={<Trophy />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="任务池同步" checked={unionRace?.enabled ?? true} description="竞赛期间同步任务池与当前已接任务（只读展示）；关闭后不再拉取竞赛数据" onChange={(checked) => updateUnionRace({ enabled: checked })} />
                <ToggleRow label="显示个人得分排名" checked={unionRace?.showPersonalScoreRank ?? false} description="开启后在竞赛页展示当期个人累计得分与公会内排名；默认关闭" onChange={(checked) => updateUnionRace({ showPersonalScoreRank: checked })} />
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

        {section === "activities" && (
          <div className="grid gap-3">
            <PolicyGroup title="花笺集芳" icon={<Play />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="启用" checked={activity?.cyclicNote?.enabled ?? false} onChange={(enabled) => updateCyclicNote({ enabled })} status={settingStatusForCapability(capabilities, "activity.cyclicNote")} />
                <ToggleRow label="自动领取任务奖励" checked={activity?.cyclicNote?.autoClaimTaskRewards ?? false} onChange={(autoClaimTaskRewards) => updateCyclicNote({ autoClaimTaskRewards })} />
                <ToggleRow label="自动领取积分奖励" checked={activity?.cyclicNote?.autoClaimProgressBoxes ?? false} onChange={(autoClaimProgressBoxes) => updateCyclicNote({ autoClaimProgressBoxes })} />
                <ToggleRow label="驱动已启用模块完成任务" checked={activity?.cyclicNote?.satisfyTasks ?? false} onChange={(satisfyTasks) => updateCyclicNote({ satisfyTasks })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="莳花纪闻" icon={<Play />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="启用" checked={activity?.cyclicStory?.enabled ?? false} onChange={(enabled) => updateCyclicStory({ enabled })} status={settingStatusForCapability(capabilities, "activity.actCyclicStory")} />
                <ToggleRow label="自动领取订单奖励" checked={activity?.cyclicStory?.autoClaimOrderRewards ?? false} onChange={(autoClaimOrderRewards) => updateCyclicStory({ autoClaimOrderRewards })} />
                <ToggleRow label="自动领取积分奖励" checked={activity?.cyclicStory?.autoClaimProgressBoxes ?? false} onChange={(autoClaimProgressBoxes) => updateCyclicStory({ autoClaimProgressBoxes })} />
                <NumberRow label="分数上限（0=不限制）" value={safeBigIntToNumber(activity?.cyclicStory?.maxScore, 0)} min={0} onChange={(value) => updateCyclicStory({ maxScore: safeNumberToBigInt(value, 0) })} />
              </div>
            </PolicyGroup>

            <PolicyGroup title="香卉甜糕" icon={<Play />}>
              <div className="grid gap-2 sm:grid-cols-2">
                <ToggleRow label="启用" checked={activity?.dessert?.enabled ?? false} onChange={(enabled) => updateDessert({ enabled })} status={settingStatusForCapability(capabilities, "activity.actDessert")} />
                <ToggleRow label="自动领取任务奖励" checked={activity?.dessert?.autoClaimTaskRewards ?? false} onChange={(autoClaimTaskRewards) => updateDessert({ autoClaimTaskRewards })} />
                <ToggleRow label="自动免费点赞" checked={activity?.dessert?.autoLikeCelebrity ?? false} onChange={(autoLikeCelebrity) => updateDessert({ autoLikeCelebrity })} />
                <ToggleRow label="自动开启奖励箱（每次1个）" checked={activity?.dessert?.autoOpenRewardBoxes ?? false} onChange={(autoOpenRewardBoxes) => updateDessert({ autoOpenRewardBoxes })} />
                <ToggleRow label="启用影子诊断（不执行）" checked={activity?.dessert?.autoPlay ?? false} onChange={(autoPlay) => updateDessert({ autoPlay })} />
                <ToggleRow label="请求接管评估（当前硬锁）" checked={activity?.dessert?.resumeExistingRound ?? false} onChange={(resumeExistingRound) => updateDessert({ resumeExistingRound })} />
                <NumberRow label="影子模式（仅 1 可用）" value={activity?.dessert?.mode || 1} min={1} max={1} onChange={(mode) => updateDessert({ mode })} />
                <NumberRow label="会话体力预算（0=禁用；当前仅诊断）" value={activity?.dessert?.maxEnergyPerSession ?? 0} min={0} max={100} onChange={(maxEnergyPerSession) => updateDessert({ maxEnergyPerSession })} />
                <NumberRow label="最低体力保留" value={activity?.dessert?.minEnergyReserve ?? 0} min={0} onChange={(minEnergyReserve) => updateDessert({ minEnergyReserve })} />
              </div>
            </PolicyGroup>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
