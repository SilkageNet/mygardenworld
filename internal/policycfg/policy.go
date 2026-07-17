package policycfg

import (
	"encoding/json"
	"math"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxReconnectIntervalSeconds = 24 * 60 * 60

var jsonMarshal = protojson.MarshalOptions{
	EmitUnpopulated: true,
	UseProtoNames:   true,
	Indent:          "  ",
}

var jsonUnmarshal = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

func Normalize(p *pb.Policy) *pb.Policy {
	if p == nil {
		return automation.DefaultPolicy()
	}
	cp := proto.Clone(p).(*pb.Policy)
	def := automation.DefaultPolicy()
	if cp.Basic == nil {
		cp.Basic = proto.Clone(def.Basic).(*pb.BasicPolicy)
	}
	if cp.Basic.Reputation == nil {
		cp.Basic.Reputation = proto.Clone(def.Basic.Reputation).(*pb.ReputationPolicy)
	}
	if cp.Basic.Reputation.Threshold <= 0 {
		cp.Basic.Reputation.Threshold = def.Basic.Reputation.Threshold
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
	if cp.Activity.Modules == nil {
		cp.Activity.Modules = map[string]*pb.ActivityModulePolicy{}
	}
	if cp.DecisionIntervalSeconds <= 0 {
		cp.DecisionIntervalSeconds = def.DecisionIntervalSeconds
	}
	return cp
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
	p := automation.DefaultPolicy()
	if raw == "" {
		return p, nil
	}
	compatAutoHarvest := shouldBackfillAutoHarvest(raw)
	raw = rewriteLegacyRaceScoreField(raw)
	if err := jsonUnmarshal.Unmarshal([]byte(raw), p); err != nil {
		return nil, err
	}
	if compatAutoHarvest {
		if p.Plant == nil {
			p.Plant = &pb.PlantPolicy{}
		}
		if p.Plant.Planting == nil {
			p.Plant.Planting = &pb.PlantingPolicy{}
		}
		p.Plant.Planting.AutoHarvestEnabled = true
	}
	return Normalize(p), nil
}

// rewriteLegacyRaceScoreField accepts the short-lived max_task_score spelling
// used by the KK branch. The field is a lower-bound threshold and historically
// shipped as min_task_score, so keeping that stable name also preserves stored
// policies from main.
func rewriteLegacyRaceScoreField(raw string) string {
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return raw
	}
	union, ok := objectField(doc, "union")
	if !ok {
		return raw
	}
	race, ok := objectField(union, "race")
	if !ok || hasAnyField(race, "min_task_score", "minTaskScore") {
		return raw
	}
	var value any
	for _, key := range []string{"max_task_score", "maxTaskScore"} {
		if candidate, exists := race[key]; exists {
			value = candidate
			break
		}
	}
	if value == nil {
		return raw
	}
	race["min_task_score"] = value
	data, err := json.Marshal(doc)
	if err != nil {
		return raw
	}
	return string(data)
}

func shouldBackfillAutoHarvest(raw string) bool {
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return false
	}
	plant, ok := objectField(doc, "plant")
	if !ok {
		return false
	}
	planting, ok := objectField(plant, "planting")
	if !ok {
		return false
	}
	if hasAnyField(planting, "auto_harvest_enabled", "autoHarvestEnabled") {
		return false
	}
	return boolField(planting, "auto_enabled", "autoEnabled")
}

func objectField(obj map[string]any, key string) (map[string]any, bool) {
	v, ok := obj[key]
	if !ok {
		return nil, false
	}
	child, ok := v.(map[string]any)
	return child, ok
}

func hasAnyField(obj map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func boolField(obj map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			b, ok := v.(bool)
			return ok && b
		}
	}
	return false
}
