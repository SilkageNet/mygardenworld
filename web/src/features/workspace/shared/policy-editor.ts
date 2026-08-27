import { create } from "@bufbuild/protobuf";
import {
  ActivityPolicySchema,
  BasicPolicySchema,
  BasicTaskPolicySchema,
  BenefitPolicySchema,
  CultivatePolicySchema,
  CustomerOrderPolicySchema,
  CyclicNotePolicySchema,
  CyclicStoryPolicySchema,
  DessertPolicySchema,
  FlowerArtPolicySchema,
  FlowerElvesPolicySchema,
  FlowerMarketPolicySchema,
  FriendStealPolicySchema,
  OrderPolicySchema,
  PalaceOrderPolicySchema,
  PearlPolicySchema,
  PlantPolicySchema,
  PlantingPolicySchema,
  ReputationPolicySchema,
  ResidentOrderPolicySchema,
  ShopBuyPolicySchema,
  ShopPolicySchema,
  SignPolicySchema,
  TeamOrderPolicySchema,
  UnionBuildPolicySchema,
  UnionFlowerPolicySchema,
  UnionLandPolicySchema,
  UnionPolicySchema,
  UnionRacePolicySchema,
  VipShopPolicySchema,
  ZooPolicySchema,
  type ActivityPolicy,
  type BasicPolicy,
  type BasicTaskPolicy,
  type BenefitPolicy,
  type CultivatePolicy,
  type CustomerOrderPolicy,
  type CyclicNotePolicy,
  type CyclicStoryPolicy,
  type DessertPolicy,
  type FlowerArtPolicy,
  type FlowerElvesPolicy,
  type FlowerMarketPolicy,
  type FriendStealPolicy,
  type OrderPolicy,
  type PalaceOrderPolicy,
  type PearlPolicy,
  type PlantPolicy,
  type PlantingPolicy,
  type Policy,
  type ReputationPolicy,
  type ResidentOrderPolicy,
  type ShopBuyPolicy,
  type ShopPolicy,
  type SignPolicy,
  type TeamOrderPolicy,
  type UnionBuildPolicy,
  type UnionFlowerPolicy,
  type UnionLandPolicy,
  type UnionPolicy,
  type UnionRacePolicy,
  type VipShopPolicy,
  type ZooPolicy,
} from "@/gen/mygardenworld/v1/policy_pb";

export function createPolicyEditor(policy: Policy | null, onPolicyChange: (policy: Policy | null) => void) {
  const updatePolicy = (patch: Partial<Policy>) => {
    if (policy) onPolicyChange({ ...policy, ...patch });
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
    const current = policy?.basic?.reputation ?? create(ReputationPolicySchema);
    updateBasic({ reputation: { ...current, ...patch } });
  };
  const updateBasicTask = (patch: Partial<BasicTaskPolicy>) => {
    const current = policy?.basic?.task ?? create(BasicTaskPolicySchema);
    updateBasic({ task: { ...current, ...patch } });
  };
  const updateBenefit = (patch: Partial<BenefitPolicy>) => {
    const current = policy?.basic?.benefit ?? create(BenefitPolicySchema);
    updateBasic({ benefit: { ...current, ...patch } });
  };
  const updateSign = (patch: Partial<SignPolicy>) => {
    const current = policy?.basic?.sign ?? create(SignPolicySchema);
    updateBasic({ sign: { ...current, ...patch } });
  };
  const updatePearl = (patch: Partial<PearlPolicy>) => {
    const current = policy?.basic?.pearl ?? create(PearlPolicySchema);
    updateBasic({ pearl: { ...current, ...patch } });
  };
  const updateShop = (patch: Partial<ShopPolicy>) => {
    const current = policy?.basic?.shop ?? create(ShopPolicySchema);
    updateBasic({ shop: { ...current, ...patch } });
  };
  const updateCultivateShop = (patch: Partial<ShopBuyPolicy>) => {
    const current = policy?.basic?.shop?.cultivateShop ?? create(ShopBuyPolicySchema);
    updateShop({ cultivateShop: { ...current, ...patch } });
  };
  const updateVipShop = (patch: Partial<VipShopPolicy>) => {
    const current = policy?.basic?.shop?.vipShop ?? create(VipShopPolicySchema);
    updateShop({ vipShop: { ...current, ...patch } });
  };
  const updateZoo = (patch: Partial<ZooPolicy>) => {
    const current = policy?.basic?.zoo ?? create(ZooPolicySchema);
    updateBasic({ zoo: { ...current, ...patch } });
  };
  const updatePlanting = (patch: Partial<PlantingPolicy>) => {
    const current = policy?.plant?.planting ?? create(PlantingPolicySchema);
    updatePlant({ planting: create(PlantingPolicySchema, { ...current, ...patch }) });
  };
  const updateCultivate = (patch: Partial<CultivatePolicy>) => {
    const current = policy?.plant?.cultivate ?? create(CultivatePolicySchema);
    updatePlant({ cultivate: { ...current, ...patch } });
  };
  const updateFriendSteal = (patch: Partial<FriendStealPolicy>) => {
    const current = policy?.plant?.friendSteal ?? create(FriendStealPolicySchema);
    updatePlant({ friendSteal: { ...current, ...patch } });
  };
  const updateFriendTouchCount = (uid: bigint, count: number) => {
    const key = uid.toString();
    const friendCounts = { ...(policy?.plant?.friendSteal?.friendCounts ?? {}) };
    if (count <= 0) delete friendCounts[key];
    else friendCounts[key] = count;
    updateFriendSteal({ friendCounts });
  };
  const updateFriendTouchExcluded = (uid: bigint, excluded: boolean) => {
    const current = policy?.plant?.friendSteal?.excludeUids ?? [];
    const excludeUids = excluded
      ? current.includes(uid) ? current : [...current, uid]
      : current.filter((value) => value !== uid);
    updateFriendSteal({ excludeUids });
  };
  const updateElves = (patch: Partial<FlowerElvesPolicy>) => {
    const current = policy?.plant?.elves ?? create(FlowerElvesPolicySchema);
    updatePlant({ elves: { ...current, ...patch } });
  };
  const updateMarket = (patch: Partial<FlowerMarketPolicy>) => {
    const current = policy?.plant?.market ?? create(FlowerMarketPolicySchema);
    updatePlant({ market: { ...current, ...patch } });
  };
  const updateOrder = (patch: Partial<OrderPolicy>) => {
    if (!policy) return;
    const current = policy.order ?? create(OrderPolicySchema);
    onPolicyChange({ ...policy, order: { ...current, ...patch } });
  };
  const updateCustomer = (patch: Partial<CustomerOrderPolicy>) => {
    const current = policy?.order?.customer ?? create(CustomerOrderPolicySchema);
    updateOrder({ customer: { ...current, ...patch } });
  };
  const updateResident = (patch: Partial<ResidentOrderPolicy>) => {
    const current = policy?.order?.resident ?? create(ResidentOrderPolicySchema);
    updateOrder({ resident: { ...current, ...patch } });
  };
  const updatePalace = (patch: Partial<PalaceOrderPolicy>) => {
    const current = policy?.order?.palace ?? create(PalaceOrderPolicySchema);
    updateOrder({ palace: { ...current, ...patch } });
  };
  const updateTeam = (patch: Partial<TeamOrderPolicy>) => {
    const current = policy?.order?.team ?? create(TeamOrderPolicySchema);
    updateOrder({ team: { ...current, ...patch } });
  };
  const updateFlowerArt = (patch: Partial<FlowerArtPolicy>) => {
    const current = policy?.order?.flowerArt ?? create(FlowerArtPolicySchema);
    updateOrder({ flowerArt: create(FlowerArtPolicySchema, { ...current, ...patch }) });
  };
  const updateUnion = (patch: Partial<UnionPolicy>) => {
    if (!policy) return;
    const current = policy.union ?? create(UnionPolicySchema);
    onPolicyChange({ ...policy, union: { ...current, ...patch } });
  };
  const updateUnionBuild = (patch: Partial<UnionBuildPolicy>) => {
    const current = policy?.union?.build ?? create(UnionBuildPolicySchema);
    updateUnion({ build: { ...current, ...patch } });
  };
  const updateUnionFlower = (patch: Partial<UnionFlowerPolicy>) => {
    const current = policy?.union?.flower ?? create(UnionFlowerPolicySchema);
    updateUnion({ flower: { ...current, ...patch } });
  };
  const updateUnionRace = (patch: Partial<UnionRacePolicy>) => {
    const current = policy?.union?.race ?? create(UnionRacePolicySchema);
    updateUnion({ race: { ...current, ...patch } });
  };
  const updateUnionLand = (patch: Partial<UnionLandPolicy>) => {
    const current = policy?.union?.land ?? create(UnionLandPolicySchema);
    updateUnion({ land: { ...current, ...patch } });
  };
  const updateActivity = (patch: Partial<ActivityPolicy>) => {
    if (!policy) return;
    const current = policy.activity ?? create(ActivityPolicySchema);
    onPolicyChange({ ...policy, activity: { ...current, ...patch } });
  };
  const updateCyclicNote = (patch: Partial<CyclicNotePolicy>) => {
    const current = policy?.activity?.cyclicNote ?? create(CyclicNotePolicySchema);
    updateActivity({ cyclicNote: create(CyclicNotePolicySchema, { ...current, ...patch }) });
  };
  const updateCyclicStory = (patch: Partial<CyclicStoryPolicy>) => {
    const current = policy?.activity?.cyclicStory ?? create(CyclicStoryPolicySchema);
    updateActivity({ cyclicStory: create(CyclicStoryPolicySchema, { ...current, ...patch }) });
  };
  const updateDessert = (patch: Partial<DessertPolicy>) => {
    const current = policy?.activity?.dessert ?? create(DessertPolicySchema);
    updateActivity({ dessert: create(DessertPolicySchema, { ...current, ...patch }) });
  };

  return {
    updatePolicy, updateBasic, updateReputation, updateBasicTask, updateBenefit, updateSign, updatePearl,
    updateCultivateShop, updateVipShop, updateZoo, updatePlanting, updateCultivate, updateFriendSteal,
    updateFriendTouchCount, updateFriendTouchExcluded, updateElves, updateMarket, updateCustomer,
    updateResident, updatePalace, updateTeam, updateFlowerArt, updateUnion, updateUnionBuild,
    updateUnionFlower, updateUnionRace, updateUnionLand, updateCyclicNote, updateCyclicStory, updateDessert,
  };
}
