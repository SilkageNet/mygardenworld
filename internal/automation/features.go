package automation

import "strings"

const (
	PlanStatusReady          = "ready"
	PlanStatusManaged        = "managed"
	PlanStatusSyncOnly       = "sync_only"
	PlanStatusAdapterMissing = "adapter_missing"
	PlanStatusBlocked        = "blocked"
	PlanStatusSkipped        = "skipped"
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
	{ID: "plant.harvest", Label: "收获", Category: CategoryPlant, Domain: "farm.harvest", Action: "harvest", Status: PlanStatusReady, Executable: true},
	{ID: "plant.plant", Label: "种植", Category: CategoryPlant, Domain: "farm.plant", Action: "plant", Status: PlanStatusReady, Executable: true},
	{ID: "plant.water", Label: "浇水", Category: CategoryPlant, Domain: "farm.water", Action: "water", Status: PlanStatusReady, Executable: true},
	{ID: "plant.land_unlock", Label: "土地开垦", Category: CategoryPlant, Domain: "farm.land", Action: "unlock", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.speed_up", Label: "加速", Category: CategoryPlant, Domain: "farm.speed_up", Action: "speed_up", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.cultivate", Label: "培育", Category: CategoryPlant, Domain: "farm.cultivate", Action: "cultivate", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.cultivate_recv", Label: "培育领取", Category: CategoryPlant, Domain: "farm.cultivate", Action: "recv", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.upgrade", Label: "鲜花升级", Category: CategoryPlant, Domain: "farm.upgrade", Action: "upgrade", Status: PlanStatusManaged, Executable: true},
	{ID: "plant.friend_steal", Label: "好友偷花", Category: CategoryPlant, Domain: "farm.friend_steal", Action: "steal", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "plant.elves", Label: "花灵", Category: CategoryPlant, Domain: "farm.elves", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "plant.elves_pass", Label: "花灵密令", Category: CategoryPlant, Domain: "farm.elves.pass", Action: "claim", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "plant.flower_pass", Label: "花之密令", Category: CategoryPlant, Domain: "farm.flower_pass", Action: "claim", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "plant.market", Label: "花贸市场", Category: CategoryPlant, Domain: "farm.market", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},

	{ID: "basic.waterwheel", Label: "水车水滴", Category: CategoryBasic, Domain: "basic.waterwheel", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.free_water", Label: "限时水滴", Category: CategoryBasic, Domain: "basic.free_water", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.benefit_box", Label: "福利宝箱", Category: CategoryBasic, Domain: "basic.benefit", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.reputation", Label: "礼仪分监控", Category: CategoryBasic, Domain: "basic.reputation", Action: "guard", Status: PlanStatusManaged},
	{ID: "basic.item_log", Label: "道具日志", Category: CategoryBasic, Domain: "basic.item_log", Action: "observe", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.mail_sync", Label: "邮件同步", Category: CategoryBasic, Domain: "basic.mail", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.mail", Label: "邮件", Category: CategoryBasic, Domain: "basic.mail", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.welfare", Label: "福利", Category: CategoryBasic, Domain: "basic.welfare", Action: "claim", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"缺少福利执行 adapter"}},
	{ID: "basic.double_coin", Label: "双倍金币", Category: CategoryBasic, Domain: "basic.benefit.double_coin", Action: "claim", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"看视频双倍金币需要客户端 SDK 广告 token，暂不自动执行"}},
	{ID: "basic.share_reward", Label: "分享奖励", Category: CategoryBasic, Domain: "basic.benefit.share", Action: "claim", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "basic.anti_scam_box", Label: "防骗宝箱", Category: CategoryBasic, Domain: "basic.benefit.anti_scam", Action: "answer", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.anti_scam_box", Label: "防骗宝箱", Category: CategoryBasic, Domain: "basic.benefit.anti_scam", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_main", Label: "主线任务", Category: CategoryBasic, Domain: "basic.task.main", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_daily", Label: "每日任务", Category: CategoryBasic, Domain: "basic.task.daily", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_weekly", Label: "每周任务", Category: CategoryBasic, Domain: "basic.task.weekly", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.task_achievement", Label: "成就任务", Category: CategoryBasic, Domain: "basic.task.achievement", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.story_sync", Label: "剧情同步", Category: CategoryBasic, Domain: "basic.story", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.story", Label: "剧情", Category: CategoryBasic, Domain: "basic.story", Action: "unlock", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.sign", Label: "签到/祈愿", Category: CategoryBasic, Domain: "basic.sign", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.sign_patch", Label: "自动补签", Category: CategoryBasic, Domain: "basic.sign.patch", Action: "claim", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"补签成本尚未确认"}},
	{ID: "basic.road_grow", Label: "成长之路", Category: CategoryBasic, Domain: "basic.road_grow", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.map_event_sync", Label: "地图事件同步", Category: CategoryBasic, Domain: "basic.map_event", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.map_event", Label: "地图随机事件", Category: CategoryBasic, Domain: "basic.map_event", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl_sync", Label: "珍珠同步", Category: CategoryBasic, Domain: "basic.pearl", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl_free", Label: "免费珍珠", Category: CategoryBasic, Domain: "basic.pearl.free", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl_place", Label: "珍珠收取", Category: CategoryBasic, Domain: "basic.pearl.place", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl_draw", Label: "开珍珠", Category: CategoryBasic, Domain: "basic.pearl.draw", Action: "draw", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl_protect", Label: "珍珠防身", Category: CategoryBasic, Domain: "basic.pearl.protect", Action: "enable", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.pearl_hire", Label: "雇佣劳工", Category: CategoryBasic, Domain: "basic.pearl.hire", Action: "hire", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"候选用户与雇佣券成本尚未确认"}},
	{ID: "basic.pearl_buy_hire_ticket", Label: "买雇佣书", Category: CategoryBasic, Domain: "basic.pearl.buy_hire_ticket", Action: "buy", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"元宝成本尚未放开自动执行"}},
	{ID: "basic.shop", Label: "商城", Category: CategoryBasic, Domain: "basic.shop", Action: "buy", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "basic.shop_giftbag_sync", Label: "礼包商店同步", Category: CategoryBasic, Domain: "basic.shop.giftbag", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.shop_video_gift", Label: "视频礼包", Category: CategoryBasic, Domain: "basic.shop.video_gift", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.shop_cultivate_sync", Label: "材料商店同步", Category: CategoryBasic, Domain: "basic.shop.cultivate", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.shop_cultivate", Label: "材料商店", Category: CategoryBasic, Domain: "basic.shop.cultivate", Action: "buy", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.shop_vip", Label: "VIP 商店", Category: CategoryBasic, Domain: "basic.shop.vip", Action: "buy", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"VIP 商店成本和状态尚未确认"}},
	{ID: "basic.zoo", Label: "宠物", Category: CategoryBasic, Domain: "basic.zoo", Action: "sync", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_refresh", Label: "宠物状态刷新", Category: CategoryBasic, Domain: "basic.zoo", Action: "refresh", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_feed", Label: "补充宠物食盆", Category: CategoryBasic, Domain: "basic.zoo.feed", Action: "stock", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_stroke", Label: "宠物互动", Category: CategoryBasic, Domain: "basic.zoo.stroke", Action: "stroke", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_buy_food", Label: "购买猫粮", Category: CategoryBasic, Domain: "basic.zoo.buy_food", Action: "buy", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"猫粮购买成本和商品选择尚未放开自动执行"}},
	{ID: "basic.zoo_find_pet", Label: "宠物寻回", Category: CategoryBasic, Domain: "basic.zoo.event", Action: "find_pet", Status: PlanStatusBlocked, BlockedReasons: []string{"实测寻回固定消耗 10 元宝，永久禁止自动执行"}},
	{ID: "basic.zoo_handle_event", Label: "宠物事件", Category: CategoryBasic, Domain: "basic.zoo.event", Action: "handle_event", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_read_log", Label: "宠物日志已读", Category: CategoryBasic, Domain: "basic.zoo.event", Action: "read_log", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_souvenir_reward", Label: "宠物纪念品奖励", Category: CategoryBasic, Domain: "basic.zoo.souvenir", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "basic.zoo_souvenir_read", Label: "宠物纪念品已读", Category: CategoryBasic, Domain: "basic.zoo.souvenir", Action: "read", Status: PlanStatusManaged, Executable: true},

	{ID: "order.customer", Label: "顾客订单", Category: CategoryOrder, Domain: "order.customer", Action: "finish", Status: PlanStatusManaged, Executable: true},
	{ID: "order.customer_craft", Label: "顾客订单花艺制作", Category: CategoryOrder, Domain: "order.customer", Action: "craft", Status: PlanStatusManaged, Executable: true},
	{ID: "order.customer_generate", Label: "顾客订单生成", Category: CategoryOrder, Domain: "order.customer", Action: "generate", Status: PlanStatusManaged, Executable: true},
	{ID: "order.customer_reject", Label: "顾客订单暂时无货", Category: CategoryOrder, Domain: "order.customer", Action: "reject", Status: PlanStatusManaged, Executable: true},
	{ID: "order.resident", Label: "居民订单", Category: CategoryOrder, Domain: "order.resident", Action: "finish", Status: PlanStatusManaged, Executable: true},
	{ID: "order.flower_art", Label: "花艺/花架", Category: CategoryOrder, Domain: "order.flower_art", Action: "sell", Status: PlanStatusManaged, Executable: true},
	{ID: "order.flower_art_craft", Label: "花艺制作", Category: CategoryOrder, Domain: "order.flower_art", Action: "craft", Status: PlanStatusManaged, Executable: true},
	{ID: "order.flower_art_claim", Label: "花架收益", Category: CategoryOrder, Domain: "order.flower_art", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "order.flower_art_stand", Label: "花架解锁", Category: CategoryOrder, Domain: "order.flower_art.stand", Action: "unlock", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"花架解锁成本尚未确认"}},
	{ID: "order.flower_art_create_reward", Label: "花艺经验奖励", Category: CategoryOrder, Domain: "order.flower_art.create_reward", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "order.flower_art_collect_reward", Label: "图鉴奖励", Category: CategoryOrder, Domain: "order.flower_art.collect_reward", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "order.palace_sync", Label: "宫廷订单同步", Category: CategoryOrder, Domain: "order.palace", Action: "sync", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "order.palace", Label: "宫廷订单", Category: CategoryOrder, Domain: "order.palace", Action: "finish", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "order.team_sync", Label: "组团订单同步", Category: CategoryOrder, Domain: "order.team", Action: "sync", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "order.team", Label: "组团订单", Category: CategoryOrder, Domain: "order.team", Action: "submit", Status: PlanStatusSyncOnly, SyncOnly: true},

	{ID: "union.build", Label: "公会建设", Category: CategoryUnion, Domain: "union.build", Action: "build", Status: PlanStatusManaged, Executable: true},
	{ID: "union.flower", Label: "公会鲜花共享", Category: CategoryUnion, Domain: "union.flower", Action: "share_take", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.flower_share", Label: "公会分享", Category: CategoryUnion, Domain: "union.flower.share", Action: "share", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.flower_reward", Label: "公会分享奖励", Category: CategoryUnion, Domain: "union.flower.reward", Action: "claim", Status: PlanStatusManaged, Executable: true},
	{ID: "union.flower_take", Label: "公会摸花", Category: CategoryUnion, Domain: "union.flower.take", Action: "take", Status: PlanStatusManaged, Executable: true},
	{ID: "union.race", Label: "公会竞赛", Category: CategoryUnion, Domain: "union.race", Action: "race", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.race_upgrade", Label: "公会竞赛升级任务", Category: CategoryUnion, Domain: "union.race.upgrade", Action: "upgrade", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"公会竞赛任务升级成本尚未确认"}},
	{ID: "union.land", Label: "公会土地", Category: CategoryUnion, Domain: "union.land", Action: "run", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.land_harvest", Label: "公会土地收获", Category: CategoryUnion, Domain: "union.land.harvest", Action: "harvest", Status: PlanStatusManaged, Executable: true},
	{ID: "union.land_plant", Label: "公会土地种植", Category: CategoryUnion, Domain: "union.land.plant", Action: "plant", Status: PlanStatusSyncOnly, SyncOnly: true},
	{ID: "union.red_packet", Label: "公会红包", Category: CategoryUnion, Domain: "union.red_packet", Action: "claim", Status: PlanStatusAdapterMissing, BlockedReasons: []string{"缺少公会红包执行 adapter"}},
	{ID: "union.forest", Label: "能量森林", Category: CategoryUnion, Domain: "union.forest", Action: "collect", Status: PlanStatusManaged, Executable: true},

	activityFeature("actCyclicStory", "莳花纪闻/周期剧情", PlanStatusSyncOnly),
	activityFeature("actDessert", "香卉甜糕", PlanStatusSyncOnly),
	activityFeature("actDuanWu", "龙舟竞渡", PlanStatusAdapterMissing),
	activityFeature("actElim", "花漾物语", PlanStatusSyncOnly),
	activityFeature("actMerge2", "田园奇趣", PlanStatusSyncOnly),
	activityFeature("actSpool", "梳丝引线", PlanStatusSyncOnly),
	activityFeature("cyclicNote", "花笺集芳", PlanStatusSyncOnly),
	activityFeature("fishFun", "鱼乐无穷", PlanStatusAdapterMissing),
	activityFeature("fishMerge", "丰仓鱼干", PlanStatusAdapterMissing),
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
	if op.Lane == "" {
		op.Lane = laneForDomain(op.Domain)
	}
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
			op.Status = PlanStatusReady
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
