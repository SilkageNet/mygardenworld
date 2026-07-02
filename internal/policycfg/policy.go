package policycfg

import (
	"strings"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

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
	if cp.Basic == nil {
		cp.Basic = proto.Clone(def.Basic).(*pb.BasicPolicy)
	}
	if cp.Basic.Pearl == nil {
		cp.Basic.Pearl = proto.Clone(def.Basic.Pearl).(*pb.PearlPolicy)
	}
	if cp.Basic.Shop == nil {
		cp.Basic.Shop = proto.Clone(def.Basic.Shop).(*pb.ShopPolicy)
	}
	if cp.Basic.Zoo == nil {
		cp.Basic.Zoo = proto.Clone(def.Basic.Zoo).(*pb.ZooPolicy)
	}
	if cp.Plant == nil {
		cp.Plant = proto.Clone(def.Plant).(*pb.PlantPolicy)
	}
	cp.Plant.PlantingMode = normalizeMode(cp.Plant.GetPlantingMode(), def.Plant.GetPlantingMode())
	if cp.Plant.PlantMaxBatch <= 0 {
		cp.Plant.PlantMaxBatch = def.Plant.PlantMaxBatch
	}
	if cp.Plant.WaterMaxBatch <= 0 {
		cp.Plant.WaterMaxBatch = def.Plant.WaterMaxBatch
	}
	if cp.Plant.MinWaterDrops <= 0 {
		cp.Plant.MinWaterDrops = def.Plant.MinWaterDrops
	}
	if cp.Plant.TaskPriority == nil {
		cp.Plant.TaskPriority = map[string]int32{}
	}
	for k, v := range def.Plant.TaskPriority {
		if _, ok := cp.Plant.TaskPriority[k]; !ok {
			cp.Plant.TaskPriority[k] = v
		}
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
	cp.Union.FlowerShareMode = normalizeMode(cp.Union.GetFlowerShareMode(), def.Union.GetFlowerShareMode())
	cp.Union.FlowerTakeMode = normalizeMode(cp.Union.GetFlowerTakeMode(), def.Union.GetFlowerTakeMode())
	cp.Union.LandPlantMode = normalizeMode(cp.Union.GetLandPlantMode(), def.Union.GetLandPlantMode())
	if cp.Union.RaceTaskTypePriority == nil {
		cp.Union.RaceTaskTypePriority = map[string]int32{}
	}
	for k, v := range def.Union.RaceTaskTypePriority {
		if _, ok := cp.Union.RaceTaskTypePriority[k]; !ok {
			cp.Union.RaceTaskTypePriority[k] = v
		}
	}
	if cp.Activity == nil {
		cp.Activity = proto.Clone(def.Activity).(*pb.ActivityPolicy)
	}
	if cp.Activity.Modules == nil {
		cp.Activity.Modules = map[string]*pb.ActivityModulePolicy{}
	}
	for k, v := range def.Activity.Modules {
		if _, ok := cp.Activity.Modules[k]; !ok {
			cp.Activity.Modules[k] = proto.Clone(v).(*pb.ActivityModulePolicy)
		}
	}
	if cp.Safety == nil {
		cp.Safety = proto.Clone(def.Safety).(*pb.SafetyPolicy)
	}
	if cp.Safety.MaxConsecutiveErrors <= 0 {
		cp.Safety.MaxConsecutiveErrors = def.Safety.MaxConsecutiveErrors
	}
	if cp.Safety.DomainBackoffSeconds <= 0 {
		cp.Safety.DomainBackoffSeconds = def.Safety.DomainBackoffSeconds
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
	if strings.TrimSpace(raw) == "" {
		return p, nil
	}
	if err := jsonUnmarshal.Unmarshal([]byte(raw), p); err != nil {
		return nil, err
	}
	return Normalize(p), nil
}

func normalizeMode(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}
