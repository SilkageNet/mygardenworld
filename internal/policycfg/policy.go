package policycfg

import (
	"errors"
	"fmt"
	"math"
	"sort"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxReconnectIntervalSeconds = 24 * 60 * 60

const CurrentSchemaVersion uint32 = 3

var ErrSchemaVersion = errors.New("unsupported policy schema version")

func ValidateVersion(p *pb.Policy) error {
	if p == nil {
		return fmt.Errorf("%w: missing policy", ErrSchemaVersion)
	}
	if p.GetSchemaVersion() != CurrentSchemaVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrSchemaVersion, p.GetSchemaVersion(), CurrentSchemaVersion)
	}
	return nil
}

func normalizeFriendSteal(policy, def *pb.FriendStealPolicy) {
	if policy == nil {
		return
	}
	switch policy.GetMode() {
	case pb.SelectionMode_SELECTION_MODE_ALL,
		pb.SelectionMode_SELECTION_MODE_QUALITY,
		pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		pb.SelectionMode_SELECTION_MODE_EXCLUDE:
	default:
		policy.Mode = def.GetMode()
	}
	switch policy.GetFriendMode() {
	case pb.SelectionMode_SELECTION_MODE_ALL, pb.SelectionMode_SELECTION_MODE_SPECIFIC:
	default:
		if len(policy.GetFriendCounts()) > 0 {
			policy.FriendMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
		} else {
			policy.FriendMode = def.GetFriendMode()
		}
	}
	if policy.FriendCounts == nil {
		policy.FriendCounts = map[int64]int32{}
	}
	maxTarget := int32(20)
	maxBuy := int32(10)
	if cfg, ok := state.FriendTouchConfigFromCatalog(); ok {
		maxTarget = cfg.StealMax + cfg.PickMax
		maxBuy = cfg.PickMax
	}
	for uid, count := range policy.FriendCounts {
		if uid <= 0 || count <= 0 {
			delete(policy.FriendCounts, uid)
			continue
		}
		if count > maxTarget {
			policy.FriendCounts[uid] = maxTarget
		}
	}
	if policy.MaxBuyPerFriend < 0 {
		policy.MaxBuyPerFriend = 0
	} else if policy.MaxBuyPerFriend > maxBuy {
		policy.MaxBuyPerFriend = maxBuy
	}
	seen := make(map[int64]struct{}, len(policy.ExcludeUids))
	clean := policy.ExcludeUids[:0]
	for _, uid := range policy.ExcludeUids {
		if uid <= 0 {
			continue
		}
		if _, exists := seen[uid]; exists {
			continue
		}
		seen[uid] = struct{}{}
		clean = append(clean, uid)
	}
	sort.Slice(clean, func(i, j int) bool { return clean[i] < clean[j] })
	policy.ExcludeUids = clean
}

var jsonMarshal = protojson.MarshalOptions{
	EmitUnpopulated: true,
	UseProtoNames:   true,
	Indent:          "  ",
}

var jsonUnmarshal = protojson.UnmarshalOptions{
	DiscardUnknown: false,
}

func Normalize(p *pb.Policy) *pb.Policy {
	if p == nil {
		return automation.DefaultPolicy()
	}
	cp := proto.Clone(p).(*pb.Policy)
	def := automation.DefaultPolicy()
	cp.SchemaVersion = CurrentSchemaVersion
	if cp.Basic == nil {
		cp.Basic = proto.Clone(def.Basic).(*pb.BasicPolicy)
	}
	if cp.Basic.Reputation == nil {
		cp.Basic.Reputation = proto.Clone(def.Basic.Reputation).(*pb.ReputationPolicy)
	}
	if cp.Basic.Reputation.Threshold <= 0 {
		cp.Basic.Reputation.Threshold = def.Basic.Reputation.Threshold
	}
	switch cp.Basic.GetRedeemConnectMode() {
	case pb.RedeemConnectMode_REDEEM_CONNECT_MODE_AUTO,
		pb.RedeemConnectMode_REDEEM_CONNECT_MODE_ONLINE_ONLY:
	default:
		cp.Basic.RedeemConnectMode = def.Basic.GetRedeemConnectMode()
	}
	switch {
	case math.IsNaN(cp.Basic.ReconnectIntervalSeconds), math.IsInf(cp.Basic.ReconnectIntervalSeconds, 0), cp.Basic.ReconnectIntervalSeconds <= 0:
		cp.Basic.ReconnectIntervalSeconds = def.Basic.ReconnectIntervalSeconds
	case cp.Basic.ReconnectIntervalSeconds < 1:
		cp.Basic.ReconnectIntervalSeconds = 1
	case cp.Basic.ReconnectIntervalSeconds > maxReconnectIntervalSeconds:
		cp.Basic.ReconnectIntervalSeconds = maxReconnectIntervalSeconds
	}
	if cp.Basic.Task == nil {
		cp.Basic.Task = proto.Clone(def.Basic.Task).(*pb.BasicTaskPolicy)
	}
	if cp.Basic.Benefit == nil {
		cp.Basic.Benefit = proto.Clone(def.Basic.Benefit).(*pb.BenefitPolicy)
	}
	if cp.Basic.Sign == nil {
		cp.Basic.Sign = proto.Clone(def.Basic.Sign).(*pb.SignPolicy)
	}
	if cp.Basic.Pearl == nil {
		cp.Basic.Pearl = proto.Clone(def.Basic.Pearl).(*pb.PearlPolicy)
	}
	if cp.Basic.Shop == nil {
		cp.Basic.Shop = proto.Clone(def.Basic.Shop).(*pb.ShopPolicy)
	}
	if cp.Basic.Shop.CultivateShop == nil {
		cp.Basic.Shop.CultivateShop = proto.Clone(def.Basic.Shop.CultivateShop).(*pb.ShopBuyPolicy)
	}
	if cp.Basic.Shop.VipShop == nil {
		cp.Basic.Shop.VipShop = proto.Clone(def.Basic.Shop.VipShop).(*pb.VipShopPolicy)
	}
	if cp.Basic.Zoo == nil {
		cp.Basic.Zoo = proto.Clone(def.Basic.Zoo).(*pb.ZooPolicy)
	}
	if cp.Plant == nil {
		cp.Plant = proto.Clone(def.Plant).(*pb.PlantPolicy)
	}
	if cp.Plant.Cultivate == nil {
		cp.Plant.Cultivate = proto.Clone(def.Plant.Cultivate).(*pb.CultivatePolicy)
	}
	if cp.Plant.Cultivate.TargetLevel <= 0 {
		cp.Plant.Cultivate.TargetLevel = def.Plant.Cultivate.TargetLevel
	}
	if cp.Plant.Planting == nil {
		cp.Plant.Planting = proto.Clone(def.Plant.Planting).(*pb.PlantingPolicy)
	}
	if cp.Plant.Planting.AutoReplantMode == pb.SelectionMode_SELECTION_MODE_UNSPECIFIED || cp.Plant.Planting.AutoReplantMode == pb.SelectionMode_SELECTION_MODE_QUALITY {
		cp.Plant.Planting.AutoReplantMode = def.Plant.Planting.AutoReplantMode
	}
	if cp.Plant.Planting.MinWaterDrops <= 0 {
		cp.Plant.Planting.MinWaterDrops = def.Plant.Planting.MinWaterDrops
	}
	if cp.Plant.Planting.AutoReplantMinLevel < 0 {
		cp.Plant.Planting.AutoReplantMinLevel = 0
	}
	if cp.Plant.Planting.HarvestDelaySeconds < 0 {
		cp.Plant.Planting.HarvestDelaySeconds = 0
	}
	if maxLevel := state.FlowerMaxLevel(); maxLevel > 0 && cp.Plant.Planting.AutoReplantMinLevel > maxLevel {
		cp.Plant.Planting.AutoReplantMinLevel = maxLevel
	}
	if cp.Plant.Planting.DemandPriority == nil {
		cp.Plant.Planting.DemandPriority = map[string]int32{}
	}
	for k, v := range def.Plant.Planting.DemandPriority {
		if _, ok := cp.Plant.Planting.DemandPriority[k]; !ok {
			cp.Plant.Planting.DemandPriority[k] = v
		}
	}
	if cp.Plant.FriendSteal == nil {
		cp.Plant.FriendSteal = proto.Clone(def.Plant.FriendSteal).(*pb.FriendStealPolicy)
	}
	normalizeFriendSteal(cp.Plant.FriendSteal, def.Plant.FriendSteal)
	if cp.Plant.Elves == nil {
		cp.Plant.Elves = proto.Clone(def.Plant.Elves).(*pb.FlowerElvesPolicy)
	}
	if cp.Plant.Market == nil {
		cp.Plant.Market = proto.Clone(def.Plant.Market).(*pb.FlowerMarketPolicy)
	}
	if cp.Plant.Market.PutMode == pb.MarketPutMode_MARKET_PUT_MODE_UNSPECIFIED {
		cp.Plant.Market.PutMode = def.Plant.Market.PutMode
	}
	if cp.Plant.Market.BuyMode == pb.MarketBuyMode_MARKET_BUY_MODE_UNSPECIFIED {
		cp.Plant.Market.BuyMode = def.Plant.Market.BuyMode
	}
	if cp.Plant.Market.PriceIndex == 0 {
		cp.Plant.Market.PriceIndex = def.Plant.Market.PriceIndex
	}
	if cp.Plant.Market.MaxSell <= 0 {
		cp.Plant.Market.MaxSell = def.Plant.Market.MaxSell
	}
	if cp.Order == nil {
		cp.Order = proto.Clone(def.Order).(*pb.OrderPolicy)
	}
	if cp.Order.Customer == nil {
		cp.Order.Customer = proto.Clone(def.Order.Customer).(*pb.CustomerOrderPolicy)
	}
	if cp.Order.Resident == nil {
		cp.Order.Resident = proto.Clone(def.Order.Resident).(*pb.ResidentOrderPolicy)
	}
	if cp.Order.Resident.NormalDailyLimit <= 0 {
		cp.Order.Resident.NormalDailyLimit = def.Order.Resident.NormalDailyLimit
	}
	if cp.Order.Resident.DecorateDailyLimit <= 0 {
		cp.Order.Resident.DecorateDailyLimit = def.Order.Resident.DecorateDailyLimit
	}
	if cp.Order.Resident.SatinDailyLimit <= 0 {
		cp.Order.Resident.SatinDailyLimit = def.Order.Resident.SatinDailyLimit
	}
	if cp.Order.Palace == nil {
		cp.Order.Palace = proto.Clone(def.Order.Palace).(*pb.PalaceOrderPolicy)
	}
	if cp.Order.Team == nil {
		cp.Order.Team = proto.Clone(def.Order.Team).(*pb.TeamOrderPolicy)
	}
	if cp.Order.FlowerArt == nil {
		cp.Order.FlowerArt = proto.Clone(def.Order.FlowerArt).(*pb.FlowerArtPolicy)
	}
	if cp.Union == nil {
		cp.Union = proto.Clone(def.Union).(*pb.UnionPolicy)
	}
	if cp.Union.Build == nil {
		cp.Union.Build = proto.Clone(def.Union.Build).(*pb.UnionBuildPolicy)
	}
	if cp.Union.Flower == nil {
		cp.Union.Flower = proto.Clone(def.Union.Flower).(*pb.UnionFlowerPolicy)
	}
	if cp.Union.Race == nil {
		cp.Union.Race = proto.Clone(def.Union.Race).(*pb.UnionRacePolicy)
	}
	if cp.Union.Race.TaskTypePriority == nil {
		cp.Union.Race.TaskTypePriority = map[int32]int32{}
	}
	if cp.Union.Race.AvoidProgressedTasks == nil {
		cp.Union.Race.AvoidProgressedTasks = proto.Bool(def.Union.Race.GetAvoidProgressedTasks())
	}
	for k, v := range def.Union.Race.TaskTypePriority {
		if _, ok := cp.Union.Race.TaskTypePriority[k]; !ok {
			cp.Union.Race.TaskTypePriority[k] = v
		}
	}
	if cp.Union.Land == nil {
		cp.Union.Land = proto.Clone(def.Union.Land).(*pb.UnionLandPolicy)
	}
	if cp.Activity == nil {
		cp.Activity = proto.Clone(def.Activity).(*pb.ActivityPolicy)
	}
	if cp.Activity.CyclicNote == nil {
		cp.Activity.CyclicNote = proto.Clone(def.Activity.CyclicNote).(*pb.CyclicNotePolicy)
	}
	if cp.Activity.CyclicStory == nil {
		cp.Activity.CyclicStory = proto.Clone(def.Activity.CyclicStory).(*pb.CyclicStoryPolicy)
	}
	if cp.DecisionIntervalSeconds <= 0 {
		cp.DecisionIntervalSeconds = def.DecisionIntervalSeconds
	}
	clearUnsupportedSDKAdAutomation(cp)
	return cp
}

// UnsupportedSDKAdFeatures reports policy switches that require a client
// advertising SDK callback or token. Callers should inspect the raw
// request before Normalize clears these fields for persisted-policy safety.
func UnsupportedSDKAdFeatures(p *pb.Policy) []string {
	if p == nil {
		return nil
	}
	var features []string
	if p.GetPlant().GetPlanting().GetVideoSpeedUpEnabled() {
		features = append(features, "视频加速")
	}
	if p.GetPlant().GetCultivate().GetVideoSpeedUpEnabled() {
		features = append(features, "视频加速培育")
	}
	if p.GetBasic().GetBenefit().GetDoubleCoinEnabled() {
		features = append(features, "双倍金币")
	}
	if p.GetBasic().GetShop().GetVideoFreeGiftEnabled() {
		features = append(features, "视频礼包")
	}
	if p.GetUnion().GetBuild().GetFreeEnabled() {
		features = append(features, "公会视频建设")
	}
	return features
}

func clearUnsupportedSDKAdAutomation(p *pb.Policy) {
	if p == nil {
		return
	}
	if planting := p.GetPlant().GetPlanting(); planting != nil {
		planting.VideoSpeedUpEnabled = false
	}
	if cultivate := p.GetPlant().GetCultivate(); cultivate != nil {
		cultivate.VideoSpeedUpEnabled = false
	}
	if benefit := p.GetBasic().GetBenefit(); benefit != nil {
		benefit.DoubleCoinEnabled = false
	}
	if shop := p.GetBasic().GetShop(); shop != nil {
		shop.VideoFreeGiftEnabled = false
	}
	if build := p.GetUnion().GetBuild(); build != nil {
		build.FreeEnabled = false
	}
}

func Clone(p *pb.Policy) *pb.Policy {
	return Normalize(p)
}

func ToJSON(p *pb.Policy) (string, error) {
	data, err := jsonMarshal.Marshal(Normalize(p))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FromJSON(raw string) (*pb.Policy, error) {
	if raw == "" {
		return automation.DefaultPolicy(), nil
	}
	p := &pb.Policy{}
	if err := jsonUnmarshal.Unmarshal([]byte(raw), p); err != nil {
		return nil, err
	}
	if err := ValidateVersion(p); err != nil {
		return nil, err
	}
	return Normalize(p), nil
}
