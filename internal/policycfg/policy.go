package policycfg

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	KeyAutomationEnabled              = "automation_enabled"
	KeyDecisionIntervalSeconds        = "decision_interval_seconds"
	KeyHarvestEnabled                 = "harvest.enabled"
	KeyHarvestPreferOneKey            = "harvest.prefer_one_key"
	KeyPlantEnabled                   = "plant.enabled"
	KeyPlantMode                      = "plant.mode"
	KeyPlantTaskPriorityEnabled       = "plant.task_priority_enabled"
	KeyPlantMinStock                  = "plant.min_stock"
	KeyPlantMaxBatch                  = "plant.max_batch"
	KeyPlantAllowedFlowerIDs          = "plant.allowed_flower_ids"
	KeyPlantBlockedFlowerIDs          = "plant.blocked_flower_ids"
	KeyWaterEnabled                   = "water.enabled"
	KeyWaterMaxBatch                  = "water.max_batch"
	KeyWaterMinDrops                  = "water.min_drops"
	KeyWaterPreferOneKeyIfNoble       = "water.prefer_one_key_if_noble"
	KeyMiscLandUnlockEnabled          = "misc.land_unlock_enabled"
	KeyMiscStoryUnlockEnabled         = "misc.story_unlock_enabled"
	KeyMiscWaterwheelEnabled          = "misc.waterwheel_enabled"
	KeyMiscCultivateEnabled           = "misc.cultivate_enabled"
	KeyMiscFlowerUpgradeEnabled       = "misc.flower_upgrade_enabled"
	KeyMiscResidentOrderEnabled       = "misc.resident_order_enabled"
	KeyMiscCustomerOrderEnabled       = "misc.customer_order_enabled"
	KeyMiscCustomerOrderCraftEnabled  = "misc.customer_order_craft_enabled"
	KeyMiscCustomerOrderRejectEnabled = "misc.customer_order_reject_enabled"
	KeyMiscResidentOrderRewardEnabled = "misc.resident_order_reward_enabled"
	KeyMiscResidentOrderAdEnabled     = "misc.resident_order_ad_enabled"
	KeyMiscFlowerRackEnabled          = "misc.flower_rack_enabled"
	KeyMiscFlowerRackCraftEnabled     = "misc.flower_rack_craft_enabled"
	KeyMiscFreeWaterEnabled           = "misc.free_water_enabled"
	KeyMiscTaskMainRewardEnabled      = "misc.task_main_reward_enabled"
	KeyMiscTaskDailyRewardEnabled     = "misc.task_daily_reward_enabled"
	KeyMiscRoadGrowRewardEnabled      = "misc.road_grow_reward_enabled"
	KeyMiscRandomEventEnabled         = "misc.random_event_enabled"
	KeyMiscFlowerArtRewardEnabled     = "misc.flower_art_reward_enabled"
	KeyMiscBenefitBoxEnabled          = "misc.benefit_box_enabled"
	KeyMiscSpeedUpEnabled             = "misc.speed_up_enabled"
	KeyMiscTaskAchRewardEnabled       = "misc.task_ach_reward_enabled"
	KeyMiscOrderPalaceEnabled         = "misc.order_palace_enabled"
	KeyMiscSignEnabled                = "misc.sign_enabled"
	KeyMiscFlowerPassEnabled          = "misc.flower_pass_enabled"
	KeyMiscFlowerElvesPassEnabled     = "misc.flower_elves_pass_enabled"
	KeyMiscPlayerBackEnabled          = "misc.player_back_enabled"
	KeyMiscActivityRewardEnabled      = "misc.activity_reward_enabled"
	KeyMiscZooSyncEnabled             = "misc.zoo_sync_enabled"
	KeyMiscZooFeedEnabled             = "misc.zoo_feed_enabled"
)

func Normalize(p *pb.Policy) *pb.Policy {
	if p == nil {
		return automation.DefaultPolicy()
	}
	cp := proto.Clone(p).(*pb.Policy)
	def := automation.DefaultPolicy()
	if cp.Harvest == nil {
		cp.Harvest = proto.Clone(def.Harvest).(*pb.HarvestPolicy)
	}
	if cp.Plant == nil {
		cp.Plant = proto.Clone(def.Plant).(*pb.PlantPolicy)
	}
	cp.Plant.Mode = normalizePlantMode(cp.Plant.Mode)
	if cp.Plant.TaskPriorityEnabled == nil {
		cp.Plant.TaskPriorityEnabled = proto.Bool(def.Plant.GetTaskPriorityEnabled())
	}
	if cp.Plant.MaxBatch <= 0 {
		cp.Plant.MaxBatch = def.Plant.MaxBatch
	}
	if cp.Water == nil {
		cp.Water = proto.Clone(def.Water).(*pb.WaterPolicy)
	}
	if cp.Water.MaxBatch <= 0 {
		cp.Water.MaxBatch = def.Water.MaxBatch
	}
	if cp.Water.MinDrops <= 0 {
		cp.Water.MinDrops = def.Water.MinDrops
	}
	if cp.Misc == nil {
		cp.Misc = proto.Clone(def.Misc).(*pb.MiscPolicy)
	}
	if cp.DecisionIntervalSeconds <= 0 {
		cp.DecisionIntervalSeconds = def.DecisionIntervalSeconds
	}
	return cp
}

func Clone(p *pb.Policy) *pb.Policy {
	return Normalize(p)
}

func FromEntries(entries map[string]string) *pb.Policy {
	p := automation.DefaultPolicy()
	ApplyEntries(p, entries)
	return Normalize(p)
}

func Flatten(p *pb.Policy) map[string]string {
	p = Normalize(p)
	return map[string]string{
		KeyAutomationEnabled:              fmt.Sprintf("%t", p.GetAutomationEnabled()),
		KeyDecisionIntervalSeconds:        fmt.Sprintf("%g", p.GetDecisionIntervalSeconds()),
		KeyHarvestEnabled:                 fmt.Sprintf("%t", p.GetHarvest().GetEnabled()),
		KeyHarvestPreferOneKey:            fmt.Sprintf("%t", p.GetHarvest().GetPreferOneKey()),
		KeyPlantEnabled:                   fmt.Sprintf("%t", p.GetPlant().GetEnabled()),
		KeyPlantMode:                      p.GetPlant().GetMode(),
		KeyPlantTaskPriorityEnabled:       fmt.Sprintf("%t", p.GetPlant().GetTaskPriorityEnabled()),
		KeyPlantMinStock:                  fmt.Sprintf("%d", p.GetPlant().GetMinStock()),
		KeyPlantMaxBatch:                  fmt.Sprintf("%d", p.GetPlant().GetMaxBatch()),
		KeyPlantAllowedFlowerIDs:          joinInts(p.GetPlant().GetAllowedFlowerIds()),
		KeyPlantBlockedFlowerIDs:          joinInts(p.GetPlant().GetBlockedFlowerIds()),
		KeyWaterEnabled:                   fmt.Sprintf("%t", p.GetWater().GetEnabled()),
		KeyWaterMaxBatch:                  fmt.Sprintf("%d", p.GetWater().GetMaxBatch()),
		KeyWaterMinDrops:                  fmt.Sprintf("%d", p.GetWater().GetMinDrops()),
		KeyWaterPreferOneKeyIfNoble:       fmt.Sprintf("%t", p.GetWater().GetPreferOneKeyIfNoble()),
		KeyMiscLandUnlockEnabled:          fmt.Sprintf("%t", p.GetMisc().GetLandUnlockEnabled()),
		KeyMiscStoryUnlockEnabled:         fmt.Sprintf("%t", p.GetMisc().GetStoryUnlockEnabled()),
		KeyMiscWaterwheelEnabled:          fmt.Sprintf("%t", p.GetMisc().GetWaterwheelEnabled()),
		KeyMiscCultivateEnabled:           fmt.Sprintf("%t", p.GetMisc().GetCultivateEnabled()),
		KeyMiscFlowerUpgradeEnabled:       fmt.Sprintf("%t", p.GetMisc().GetFlowerUpgradeEnabled()),
		KeyMiscResidentOrderEnabled:       fmt.Sprintf("%t", p.GetMisc().GetResidentOrderEnabled()),
		KeyMiscCustomerOrderEnabled:       fmt.Sprintf("%t", p.GetMisc().GetCustomerOrderEnabled()),
		KeyMiscCustomerOrderCraftEnabled:  fmt.Sprintf("%t", p.GetMisc().GetCustomerOrderCraftEnabled()),
		KeyMiscCustomerOrderRejectEnabled: fmt.Sprintf("%t", p.GetMisc().GetCustomerOrderRejectEnabled()),
		KeyMiscResidentOrderRewardEnabled: fmt.Sprintf("%t", p.GetMisc().GetResidentOrderRewardEnabled()),
		KeyMiscResidentOrderAdEnabled:     fmt.Sprintf("%t", p.GetMisc().GetResidentOrderAdEnabled()),
		KeyMiscFlowerRackEnabled:          fmt.Sprintf("%t", p.GetMisc().GetFlowerRackEnabled()),
		KeyMiscFlowerRackCraftEnabled:     fmt.Sprintf("%t", p.GetMisc().GetFlowerRackCraftEnabled()),
		KeyMiscFreeWaterEnabled:           fmt.Sprintf("%t", p.GetMisc().GetFreeWaterEnabled()),
		KeyMiscTaskMainRewardEnabled:      fmt.Sprintf("%t", p.GetMisc().GetTaskMainRewardEnabled()),
		KeyMiscTaskDailyRewardEnabled:     fmt.Sprintf("%t", p.GetMisc().GetTaskDailyRewardEnabled()),
		KeyMiscRoadGrowRewardEnabled:      fmt.Sprintf("%t", p.GetMisc().GetRoadGrowRewardEnabled()),
		KeyMiscRandomEventEnabled:         fmt.Sprintf("%t", p.GetMisc().GetRandomEventEnabled()),
		KeyMiscFlowerArtRewardEnabled:     fmt.Sprintf("%t", p.GetMisc().GetFlowerArtRewardEnabled()),
		KeyMiscBenefitBoxEnabled:          fmt.Sprintf("%t", p.GetMisc().GetBenefitBoxEnabled()),
		KeyMiscSpeedUpEnabled:             fmt.Sprintf("%t", p.GetMisc().GetSpeedUpEnabled()),
		KeyMiscTaskAchRewardEnabled:       fmt.Sprintf("%t", p.GetMisc().GetTaskAchRewardEnabled()),
		KeyMiscOrderPalaceEnabled:         fmt.Sprintf("%t", p.GetMisc().GetOrderPalaceEnabled()),
		KeyMiscSignEnabled:                fmt.Sprintf("%t", p.GetMisc().GetSignEnabled()),
		KeyMiscFlowerPassEnabled:          fmt.Sprintf("%t", p.GetMisc().GetFlowerPassEnabled()),
		KeyMiscFlowerElvesPassEnabled:     fmt.Sprintf("%t", p.GetMisc().GetFlowerElvesPassEnabled()),
		KeyMiscPlayerBackEnabled:          fmt.Sprintf("%t", p.GetMisc().GetPlayerBackEnabled()),
		KeyMiscActivityRewardEnabled:      fmt.Sprintf("%t", p.GetMisc().GetActivityRewardEnabled()),
		KeyMiscZooSyncEnabled:             fmt.Sprintf("%t", p.GetMisc().GetZooSyncEnabled()),
		KeyMiscZooFeedEnabled:             fmt.Sprintf("%t", p.GetMisc().GetZooFeedEnabled()),
	}
}

func ApplyEntries(p *pb.Policy, entries map[string]string) {
	for k, v := range entries {
		_ = SetKey(p, k, v)
	}
}

func SetKey(p *pb.Policy, key, value string) error {
	if p == nil {
		return errors.New("nil policy")
	}
	switch key {
	case KeyAutomationEnabled:
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.AutomationEnabled = b
	case KeyDecisionIntervalSeconds:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		p.DecisionIntervalSeconds = f
	case KeyHarvestEnabled:
		ensureHarvest(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Harvest.Enabled = b
	case KeyHarvestPreferOneKey:
		ensureHarvest(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Harvest.PreferOneKey = b
	case KeyPlantEnabled:
		ensurePlant(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Plant.Enabled = b
	case KeyPlantMode:
		ensurePlant(p)
		mode, err := parsePlantMode(value)
		if err != nil {
			return err
		}
		p.Plant.Mode = mode
	case KeyPlantTaskPriorityEnabled:
		ensurePlant(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Plant.TaskPriorityEnabled = proto.Bool(b)
	case KeyPlantMinStock:
		ensurePlant(p)
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		p.Plant.MinStock = int32(n)
	case KeyPlantMaxBatch:
		ensurePlant(p)
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		p.Plant.MaxBatch = int32(n)
	case KeyPlantAllowedFlowerIDs:
		ensurePlant(p)
		p.Plant.AllowedFlowerIds = splitInts(value)
	case KeyPlantBlockedFlowerIDs:
		ensurePlant(p)
		p.Plant.BlockedFlowerIds = splitInts(value)
	case KeyWaterEnabled:
		ensureWater(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Water.Enabled = b
	case KeyWaterMaxBatch:
		ensureWater(p)
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		p.Water.MaxBatch = int32(n)
	case KeyWaterMinDrops:
		ensureWater(p)
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		p.Water.MinDrops = int32(n)
	case KeyWaterPreferOneKeyIfNoble:
		ensureWater(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Water.PreferOneKeyIfNoble = b
	case KeyMiscLandUnlockEnabled:
		ensureMisc(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Misc.LandUnlockEnabled = b
	case KeyMiscStoryUnlockEnabled:
		ensureMisc(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Misc.StoryUnlockEnabled = b
	case KeyMiscWaterwheelEnabled:
		ensureMisc(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Misc.WaterwheelEnabled = b
	case KeyMiscCultivateEnabled:
		ensureMisc(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Misc.CultivateEnabled = b
	case KeyMiscFlowerUpgradeEnabled:
		ensureMisc(p)
		b, err := parseBool(value)
		if err != nil {
			return err
		}
		p.Misc.FlowerUpgradeEnabled = b
	case KeyMiscResidentOrderEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.ResidentOrderEnabled = b })
	case KeyMiscCustomerOrderEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.CustomerOrderEnabled = b })
	case KeyMiscCustomerOrderCraftEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.CustomerOrderCraftEnabled = b })
	case KeyMiscCustomerOrderRejectEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.CustomerOrderRejectEnabled = b })
	case KeyMiscResidentOrderRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.ResidentOrderRewardEnabled = b })
	case KeyMiscResidentOrderAdEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.ResidentOrderAdEnabled = b })
	case KeyMiscFlowerRackEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.FlowerRackEnabled = b })
	case KeyMiscFlowerRackCraftEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.FlowerRackCraftEnabled = b })
	case KeyMiscFreeWaterEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.FreeWaterEnabled = b })
	case KeyMiscTaskMainRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.TaskMainRewardEnabled = b })
	case KeyMiscTaskDailyRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.TaskDailyRewardEnabled = b })
	case KeyMiscRoadGrowRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.RoadGrowRewardEnabled = b })
	case KeyMiscRandomEventEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.RandomEventEnabled = b })
	case KeyMiscFlowerArtRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.FlowerArtRewardEnabled = b })
	case KeyMiscBenefitBoxEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.BenefitBoxEnabled = b })
	case KeyMiscSpeedUpEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.SpeedUpEnabled = b })
	case KeyMiscTaskAchRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.TaskAchRewardEnabled = b })
	case KeyMiscOrderPalaceEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.OrderPalaceEnabled = b })
	case KeyMiscSignEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.SignEnabled = b })
	case KeyMiscFlowerPassEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.FlowerPassEnabled = b })
	case KeyMiscFlowerElvesPassEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.FlowerElvesPassEnabled = b })
	case KeyMiscPlayerBackEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.PlayerBackEnabled = b })
	case KeyMiscActivityRewardEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.ActivityRewardEnabled = b })
	case KeyMiscZooSyncEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.ZooSyncEnabled = b })
	case KeyMiscZooFeedEnabled:
		return setMiscBool(p, value, func(m *pb.MiscPolicy, b bool) { m.ZooFeedEnabled = b })
	default:
		if p.Extras == nil {
			p.Extras = &structpb.Struct{Fields: map[string]*structpb.Value{}}
		}
		p.Extras.Fields[key] = structpb.NewStringValue(value)
	}
	return nil
}

func normalizePlantMode(mode string) string {
	mode, err := parsePlantMode(mode)
	if err != nil {
		return automation.PlantModeHighValue
	}
	return mode
}

func parsePlantMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", automation.PlantModeHighValue:
		return automation.PlantModeHighValue, nil
	case automation.PlantModeLowStock:
		return automation.PlantModeLowStock, nil
	case automation.PlantModeSelected:
		return automation.PlantModeSelected, nil
	default:
		return "", fmt.Errorf("invalid plant mode %q (want high_value, low_stock, or selected)", value)
	}
}

func ensureHarvest(p *pb.Policy) {
	if p.Harvest == nil {
		p.Harvest = &pb.HarvestPolicy{}
	}
}

func ensurePlant(p *pb.Policy) {
	if p.Plant == nil {
		p.Plant = &pb.PlantPolicy{}
	}
}

func ensureWater(p *pb.Policy) {
	if p.Water == nil {
		p.Water = &pb.WaterPolicy{}
	}
}

func ensureMisc(p *pb.Policy) {
	if p.Misc == nil {
		p.Misc = &pb.MiscPolicy{}
	}
}

func setMiscBool(p *pb.Policy, value string, setter func(*pb.MiscPolicy, bool)) error {
	ensureMisc(p)
	b, err := parseBool(value)
	if err != nil {
		return err
	}
	setter(p.Misc, b)
	return nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "yes", "y", "on":
		return true, nil
	case "0", "f", "false", "no", "n", "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool %q", s)
}

func splitInts(s string) []int32 {
	parts := strings.Split(s, ",")
	out := make([]int32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			continue
		}
		out = append(out, int32(n))
	}
	return out
}

func joinInts(xs []int32) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(int(x))
	}
	return strings.Join(parts, ",")
}
