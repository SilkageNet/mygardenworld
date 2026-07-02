package automation

import "strings"

const (
	PlanStatusExecutable     = "executable"
	PlanStatusManaged        = "managed"
	PlanStatusSyncOnly       = "sync_only"
	PlanStatusAdapterMissing = "adapter_missing"
)

// FeatureSpec is the shared feature catalog used by plan generation and UI
// status surfaces. It intentionally describes product capability separately
// from the current policy shape.
type FeatureSpec struct {
	ID             string
	Label          string
	Category       string
	Domain         string
	Action         string
	Status         string
	Executable     bool
	SyncOnly       bool
	BlockedReasons []string
}

var featureSpecs = []FeatureSpec{
	{ID: "plant.harvest", Label: "收获", Category: CategoryPlant, Domain: "farm.harvest", Action: "harvest", Status: PlanStatusExecutable, Executable: true},
	{ID: "plant.plant", Label: "种植", Category: CategoryPlant, Domain: "farm.plant", Action: "plant", Status: PlanStatusExecutable, Executable: true},
	{ID: "plant.water", Label: "浇水", Category: CategoryPlant, Domain: "farm.water", Action: "water", Status: PlanStatusExecutable, Executable: true},
	{ID: "plant.land_unlock", Label: "土地开垦", Category: CategoryPlant, Domain: "farm.land", Action: "unlock", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.speed_up", Label: "加速", Category: CategoryPlant, Domain: "farm.speed_up", Action: "speed_up", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.cultivate", Label: "培育", Category: CategoryPlant, Domain: "farm.cultivate", Action: "cultivate", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.upgrade", Label: "鲜花升级", Category: CategoryPlant, Domain: "farm.upgrade", Action: "upgrade", Status: PlanStatusManaged, Executable: true},

	{ID: "basic.waterwheel", Label: "水车水滴", Category: CategoryBasic, Domain: "basic.waterwheel", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.free_water", Label: "限时水滴", Category: CategoryBasic, Domain: "basic.free_water", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.benefit_box", Label: "福利宝箱", Category: CategoryBasic, Domain: "basic.benefit", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.mail", Label: "邮件", Category: CategoryBasic, Domain: "basic.mail", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.welfare", Label: "福利", Category: CategoryBasic, Domain: "basic.welfare", Action: "claim", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"缺少福利执行 adapter"}},
	{ID: "basic.task_main", Label: "主线任务", Category: CategoryBasic, Domain: "basic.task.main", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_daily", Label: "每日任务", Category: CategoryBasic, Domain: "basic.task.daily", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_weekly", Label: "每周任务", Category: CategoryBasic, Domain: "basic.task.weekly", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_achievement", Label: "成就任务", Category: CategoryBasic, Domain: "basic.task.achievement", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.story", Label: "剧情", Category: CategoryBasic, Domain: "basic.story", Action: "unlock", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.sign", Label: "签到/祈愿", Category: CategoryBasic, Domain: "basic.sign", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.road_grow", Label: "成长之路", Category: CategoryBasic, Domain: "basic.road_grow", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.random_event", Label: "地图事件", Category: CategoryBasic, Domain: "basic.random_event", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl", Label: "珍珠", Category: CategoryBasic, Domain: "basic.pearl", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "basic.shop", Label: "商城", Category: CategoryBasic, Domain: "basic.shop", Action: "buy", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "basic.zoo", Label: "喂猫撸猫", Category: CategoryBasic, Domain: "basic.zoo", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},

	{ID: "order.customer", Label: "顾客订单", Category: CategoryOrder, Domain: "order.customer", Action: "finish", Status: PlanStatusManaged, Executable: true},
	{ID: "order.resident", Label: "居民订单", Category: CategoryOrder, Domain: "order.resident", Action: "finish", Status: PlanStatusManaged, Executable: true},
	{ID: "order.flower_art", Label: "花艺/花架", Category: CategoryOrder, Domain: "order.flower_art", Action: "sell", Status: PlanStatusManaged, Executable: true},
	{ID: "order.palace", Label: "宫廷订单", Category: CategoryOrder, Domain: "order.palace", Action: "finish", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "order.team", Label: "组团订单", Category: CategoryOrder, Domain: "order.team", Action: "submit", Status: PlanStatusSyncOnly, SyncOnly: true},

	{ID: "union.build", Label: "公会建设", Category: CategoryUnion, Domain: "union.build", Action: "build", Status: PlanStatusManaged, Executable: true},
	{ID: "union.flower", Label: "公会鲜花共享", Category: CategoryUnion, Domain: "union.flower", Action: "share_take", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.race", Label: "公会竞赛", Category: CategoryUnion, Domain: "union.race", Action: "race", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.land", Label: "公会土地", Category: CategoryUnion, Domain: "union.land", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.red_packet", Label: "公会红包", Category: CategoryUnion, Domain: "union.red_packet", Action: "claim", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"缺少公会红包执行 adapter"}},
	{ID: "union.forest", Label: "能量森林", Category: CategoryUnion, Domain: "union.forest", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},

	activityFeature("actCyclicStory", "莳花纪闻/周期剧情", PlanStatusSyncOnly),
	activityFeature("actDessert", "香卉甜糕", PlanStatusSyncOnly),
	activityFeature("actDuanWu", "龙舟竞渡", PlanStatusAdapterMissing),
	activityFeature("actElim", "花漾物语", PlanStatusSyncOnly),
	activityFeature("actMerge2", "田园奇趣", PlanStatusSyncOnly),
	activityFeature("actSpool", "梳丝引线", PlanStatusSyncOnly),
	activityFeature("cyclicNote", "花笺集芳", PlanStatusSyncOnly),
	activityFeature("fishFun", "鱼乐无穷", PlanStatusAdapterMissing),
	activityFeature("fishMerge", "鱼类合成", PlanStatusAdapterMissing),
	activityFeature("lanternFestival", "元宵灯谜", PlanStatusAdapterMissing),
	activityFeature("magicBubble", "奇妙泡泡", PlanStatusAdapterMissing),
	activityFeature("moneyTree", "摇钱树", PlanStatusSyncOnly),
	activityFeature("recvLuck", "迎新接福", PlanStatusAdapterMissing),
	activityFeature("redPacket", "红包雨", PlanStatusSyncOnly),
	activityFeature("yzCall", "为紫打 call", PlanStatusAdapterMissing),
	activityFeature("zooGameElim", "动物消除", PlanStatusSyncOnly),
}

var featureByDomainAction = buildFeatureIndex()

func activityFeature(name, label, status string) FeatureSpec {
	spec := FeatureSpec{
		ID:       "activity." + name,
		Label:    label,
		Category: CategoryActivity,
		Domain:   "activity." + name,
		Action:   "run",
		Status:   status,
	}
	if status == PlanStatusSyncOnly {
		spec.SyncOnly = true
	}
	if status == PlanStatusAdapterMissing {
		spec.BlockedReasons = []string{"缺少活动执行 adapter"}
	}
	return spec
}

func buildFeatureIndex() map[string]FeatureSpec {
	out := make(map[string]FeatureSpec, len(featureSpecs))
	for _, spec := range featureSpecs {
		out[featureKey(spec.Domain, spec.Action)] = spec
	}
	return out
}

func enrichPlannedOp(op PlannedOp) PlannedOp {
	if spec, ok := featureByDomainAction[featureKey(op.Domain, op.Action)]; ok {
		if op.FeatureID == "" {
			op.FeatureID = spec.ID
		}
		if op.Label == "" {
			op.Label = spec.Label
		}
		if op.Status == "" {
			op.Status = spec.Status
		}
		if !op.Executable {
			op.Executable = spec.Executable
		}
		if !op.SyncOnly {
			op.SyncOnly = spec.SyncOnly
		}
		if len(op.BlockedReasons) == 0 && len(spec.BlockedReasons) > 0 {
			op.BlockedReasons = append([]string(nil), spec.BlockedReasons...)
		}
	}
	if op.FeatureID == "" {
		op.FeatureID = op.Domain
	}
	if op.Label == "" {
		op.Label = op.Domain
	}
	if op.Status == "" {
		if strings.HasPrefix(op.Kind, "usrLand.") {
			op.Status = PlanStatusExecutable
			op.Executable = true
		} else {
			op.Status = PlanStatusAdapterMissing
			op.BlockedReasons = append(op.BlockedReasons, "缺少功能注册信息")
		}
	}
	return op
}

func featureKey(domain, action string) string {
	return domain + "\x00" + action
}
