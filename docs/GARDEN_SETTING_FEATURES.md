# Garden 设置页产品功能文档

更新时间：2026-07-02

本文根据已观测设置页功能和本项目现有策略模型整理。目标是把“设置项清单”转成 mygardenworld 后续可实现、可解释、可维护的产品规格，而不是复制第三方前端资源或实现细节。

## 产品定位

设置页应服务于本地守护进程的自动化策略编辑。每个设置项都必须回答四个问题：

- 是否启用某个目标或执行器。
- 该目标会产生哪些资源、库存、花朵、花艺、任务需求。
- 执行时有哪些成本、阈值、优先级和安全门槛。
- UI 如何解释“为什么执行/跳过/阻塞”。

本项目已有统一分类：`basic`、`plant`、`order`、`union`、`activity`，并额外使用 `account`、`system` 做运行和系统日志。本文沿用这五个业务域。

## 当前落地状态

2026-07-02 已将 `proto/mygardenworld/v1/policy.proto` 重塑为对标设置页的树形策略模型：

- 账号体系继续使用 mygardenworld 自有账号、会话、权限和本地存储，不纳入对标范围。
- 自动化策略按 `basic`、`plant`、`order`、`union`、`activity` 五类承载完整设置项。
- 原扁平策略字段已被 nested policy 取代，允许破坏性变更，不做旧策略 JSON 兼容迁移。
- 当前已有执行器已适配新 schema：土地收获/种植/浇水、培育/升级、基础领奖、订单和花艺链路继续可运行。
- 查询 schema 已引入 `PlanStatus` enum、`CostGate`、订单统计、库存账本和阻塞汇总；操作、需求、领域状态、待办任务共用同一状态枚举，Web UI 已展示这些结构化视图。
- runner 已用 operation registry 承接计划操作的参数构造和 RPC 执行，新增执行器应优先登记到同一张表。
- 未完成协议确认的功能先进入 schema 和 feature catalog，以 `sync_only` 或 `adapter_missing` 状态展示，后续补状态与执行器。

## 总体原则

### 1. 目标驱动优先

设置项不应只是“打开某个动作”。更高价值的抽象是目标产生需求，需求进入统一账本，账本再驱动种植、浇水、收获、制作、提交、领取。

例如：

- 顾客订单需要花艺。
- 花艺需要花朵和花瓶能力。
- 花朵不足时转成种植需求。
- 种植需求按优先级排队，并保留高优先级订单库存。

### 2. 成本操作必须有资源门槛

所有会消耗金币、元宝、道具、次数的动作都需要显式门槛。元宝成本默认不启用，除非对应功能明确实现并有上限。

### 3. UI 展示策略，不隐藏风险

有成本的开关需要在 UI 上明确标识，例如：

- 花费金币。
- 花费元宝。
- 消耗道具。
- 看视频。
- 可能改变库存分配。

### 4. 高级功能可以先登记为能力

不是每个竞品功能都要立刻实现。可以先进入 Feature Catalog，状态标记为：

- `ready`：已满足条件，可直接执行。
- `managed`：可由 runner 编排执行。
- `sync_only`：只展示、只同步、暂不执行。
- `adapter_missing`：需要补协议/状态/执行器。

## 功能总览

| 分类 | 产品目标 | 当前实现关系 |
| --- | --- | --- |
| 基础 | 任务领奖、邮件、福利、水滴、签到、剧情、珍珠、商城、动物/猫 | 部分已在 `BasicPolicy` 和 feature catalog 中存在 |
| 种植 | 土地、收获、浇水、种植、加速、培育、升级、任务优先、花灵、花艺上架、花贸市场 | 核心土地/培育/升级已有雏形，任务优先需要继续强化 |
| 订单 | 居民、顾客、宫廷、组团、花艺制作/提交 | 普通居民、顾客、宫廷、组团、花艺/花架已进入需求、账本和执行链路；绸缎/建材仍阻塞解释 |
| 公会 | 公会建设、土地、分享/摸花、竞赛、红包、森林 | 已接入能力保留；未完成公会扩展本阶段暂停 |
| 活动 | 花笺集芳、莳花纪闻、小游戏、节日活动、红包雨等 | 应使用活动模块注册表逐步接入 |

## 基础 Basic

### 基础配置

| 功能 | 字段建议 | 说明 | 实现建议 |
| --- | --- | --- | --- |
| 礼仪分监控 | `basic.reputation.enabled` | 周期检查礼仪分，低于阈值时停止任务 | 需要观测礼仪分来源；先作为 adapter_missing |
| 礼仪分阈值 | `basic.reputation.threshold` | 默认 80，范围 0-100 | 与全局自动化熔断绑定 |
| 道具日志 | `basic.debug.item_log_enabled` | 展示背包道具增减详情 | 可接入现有 op log payload |

### 任务与剧情

| 功能 | 字段建议 | 说明 | 当前映射 |
| --- | --- | --- | --- |
| 主线任务自动领奖 | `basic.main_task_enabled` | 领取主线任务奖励 | 已有 |
| 每日任务自动领奖 | `basic.daily_task_enabled` | 领取每日任务奖励 | 已有 |
| 每周任务自动领奖 | `basic.weekly_task_enabled` | 领取每周任务奖励 | 已有 |
| 主线剧情 | `basic.story_enabled` | 自动解锁主线剧情 | feature catalog 已有 `basic.story` |
| 动物/地图事件 | `basic.random_event_enabled` | 自动完成随机事件互动 | 已有字段 |
| 花坊悬赏领奖 | `basic.achievement_enabled` | 领取悬赏/成就类奖励 | 可映射到 `basic.task_achievement` |

### 邮件、福利、签到

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 邮件自动领取 | `basic.mail_enabled` | 领取邮件奖励 |
| 双倍金币 | `basic.benefit.double_coin_enabled` | 已接入 `118 videoDouble` 状态；执行依赖客户端 SDK 广告 token，当前标记 `adapter_missing` |
| 福利宝箱 | `basic.benefit_box_enabled` | 已可作为基础奖励执行器 |
| 分享奖励 | `basic.benefit.share_reward_enabled` | 制作新花艺、培育新花朵、升级后触发分享领奖 |
| 防骗宝箱 | `basic.benefit.anti_scam_box_enabled` | 已接入 `7.13.1 usrExtra`，可自动推进问答状态并领取奖励 |
| 每日祈愿 | `basic.sign_enabled` | 可和签到合并展示 |
| 自动补签 | `basic.sign_patch_enabled` | 有成本风险，需明确补签消耗 |

实现状态：双倍金币读取 namespace `118` 的 `videoCnt/eTime`，可判断当前金币加成是否生效；客户端入口是 `UT.share(11,{opType:1})`，激励视频成功后携带 SDK token 调用 `usr.share`。本地 runner 不伪造视频完成或 SDK token，因此仅生成阻塞解释项，不自动执行。

实现状态：防骗宝箱读取 `7.13.1.104 antiFraudQAStatus`，客户端语义为 `2` 表示已领取；非 `2` 且开关启用时，先执行 `usrExtra.updateAntiFraudQAStatus {}` 推进问答状态，随后在状态为 `1` 时执行 `usrExtra.recvAntiFraudQARwd {}` 领取奖励。

### 珍珠

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 免费珍珠 | `basic.pearl.free_enabled` | 基于 `115.pearl.recvDailyDate` 判断每日领取 |
| 雇佣劳工 | `basic.pearl.auto_hire_enabled` | 已登记策略；候选 UID、等级过滤和雇佣券成本仍需捕获确认 |
| 雇佣等级上限 | `basic.pearl.max_hire_level` | 0 表示不限制 |
| 雇佣券上限 | `basic.pearl.max_hire_ticket_usage` | 0 表示不限制 |
| 自动开珍珠 | `basic.pearl.draw_enabled` | 基于 `115.drawList` 自动 `pearl.draw` |
| 开启防身 | `basic.pearl.protect_enabled` | `protectNum` 足够时自动开启防身 |
| 自动买雇佣书 | `basic.pearl.auto_buy_hire_ticket` | 已登记策略；元宝成本默认阻塞 |
| 元宝上限 | `basic.pearl.max_spend_diamond` | 默认 0 或显式确认后启用 |

### 商城

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 视频礼包 | `basic.shop.video_free_gift_enabled` | 基于 `112` + `c_shop_giftbag` 自动领取免费/分享礼包 |
| 材料商店 | `basic.shop.cultivate_shop.auto_buy` | 基于 `113.infoMap` 的动态价格购买白名单材料 |
| 材料商店金币上限 | `basic.shop.cultivate_shop.max_spend_gold` | 0 表示不执行金币购买；单次成本需低于上限 |
| 材料商品 ID | `basic.shop.cultivate_shop.item_ids` | 支持填 shopId 或最终获得的材料 itemId |
| VIP 商店 | `basic.shop.vip_shop.auto_buy` | 已登记策略；商品状态与成本仍默认阻塞 |
| VIP 元宝上限 | `basic.shop.vip_shop.max_spend_diamond` | 必填成本门槛 |
| VIP 花坊币上限 | `basic.shop.vip_shop.max_spend_floral_coin` | 必填成本门槛 |

### 喂猫撸猫

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 总开关 | `basic.feed_cat.enabled` | 猫/动物互动模块总开关 |
| 自动召回 | `basic.feed_cat.auto_recall` | 已登记；召回事件链路与成本未确认，暂不执行 |
| 自动购买猫粮 | `basic.feed_cat.auto_buy_food` | 已登记；成本操作，暂不执行 |
| 自动喂猫 | `basic.feed_cat.auto_feed` | 已接入 `33.1.<petId>.foodstuffArr`，仅在猫碗已有食物且状态可进食时执行 `zoo.feedPets {petIdList}` |
| 自动撸猫 | `basic.feed_cat.auto_stroke` | 已接入 `33.1.<petId>`，按 `c_zooState.isTouch`、`c_zoo.$moodMax1`、`strokeCdTime` 执行 `zoo.strokePet {petId}` |

实现状态：喂猫撸猫模块会先 `zoo.enterZoo {}` 同步 namespace `33`；自动喂猫只消费猫碗中已存在的食物，不自动购买或新增猫粮；自动撸猫按客户端红点条件判断，避免冷却中或心情已满时重复请求。自动召回、购买猫粮仍作为 `adapter_missing` 阻塞项展示。

## 种植 Plant

### 培育配置

| 功能 | 字段建议 | 说明 | 当前映射 |
| --- | --- | --- | --- |
| 自动培育 | `plant.cultivate_enabled` | 自动培育可培育花种 | 已有 |
| 视频加速培育 | `plant.cultivate_video_speedup_enabled` | 培育时间减半 | 需视频能力 |
| 鲜花升级 | `plant.flower_upgrade_enabled` | 花费金币升级鲜花 | 已有字段 |
| 目标等级 | `plant.flower_upgrade_target_level` | 默认 20，范围 1-20 | 需要补字段 |

### 水滴与浇水

竞品设置把水车和限时水滴放在种植页；mygardenworld 建议拆成两层：

- 资源领取：`basic.waterwheel_enabled`、`basic.free_water_enabled`。
- 种植消耗：`plant.water_enabled`、`plant.min_water_drops`。

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 水车水滴 | `basic.waterwheel_enabled` | 领取水车水滴 |
| 限时水滴 | `basic.free_water_enabled` | 领取限时水滴 |
| 水滴阈值 | `basic.water_claim_threshold` | 水滴少于阈值才领取，0 不限制 |
| 保留水滴 | `plant.min_water_drops` | 保留多少水滴不用于浇花 |

### 土地与种植

| 功能 | 字段建议 | 说明 | 当前映射 |
| --- | --- | --- | --- |
| 解锁土地 | `plant.land_unlock_enabled` | 花费金币解锁可解锁土地 | 已有 |
| 自动收获 | `plant.harvest_enabled` | 自动收获成熟土地 | 已有 |
| 自动种植 | `plant.plant_enabled` | 自动完成空地种植 | 已有 |
| 自动浇水 | `plant.water_enabled` | 自动浇水 | 已有 |
| 视频加速 | `plant.speed_up_video_enabled` | 所有地块可加速时使用，避免浪费 | 需补执行器 |
| 使用加速券 | `plant.speed_up_enabled` | 消耗加速券 | 已有概念 |
| 加速券上限 | `plant.speed_up_ticket_max` | 0 表示不限制 | 需资源门槛 |

### 种植策略

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 任务优先 | `plant.task_priority_enabled` | 启用需求驱动种植 |
| 任务日志 | `plant.task_log_enabled` | 展示种植需求队列 |
| 任务优先级 | `plant.goal_priority` | 数字越小越优先 |
| 种植模式 | `plant.planting_mode` | `quality` 指定品质；`count` 指定种类数；`specific` 指定花朵 |
| 选择品质 | `plant.allowed_qualities` | 品质 1-5 |
| 选择数量 | `plant.plant_flower_kind_count` | 默认 4，范围 1/2/4/8/16 |
| 选择花朵 | `plant.allowed_flower_ids` | 指定可种花朵 |
| 排除花朵 | `plant.blocked_flower_ids` | 本项目已有字段 |
| 最低花朵等级 | `plant.min_flower_level` | 0 不限制 |

默认目标优先级建议：

| 目标 | goal id | 默认优先级 |
| --- | --- | --- |
| 顾客订单 | `order.customer` | 1 |
| 居民订单 | `order.resident` | 1 |
| 花艺售卖 | `order.flower_art` | 1 |
| 公会竞赛 | `union.race` | 1 |
| 莳花纪闻 | `activity.actCyclicStory` | 2 |
| 宫廷订单 | `order.palace` | 3 |

### 好友偷花

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动偷花 | `plant.friend_steal.enabled` | 好友花田采集 |
| 偷取花灵 | `plant.friend_steal.steal_elves` | 是否偷有花灵的地块 |
| 偷花模式 | `plant.friend_steal.mode` | `quality`、`specific`、`exclude` |
| 指定品质 | `plant.friend_steal.qualities` | 只偷指定品质 |
| 指定花朵 | `plant.friend_steal.flower_ids` | 只偷指定花朵 |
| 排除花朵 | `plant.friend_steal.exclude_flower_ids` | 不偷指定花朵 |
| 购买偷取次数 | `plant.friend_steal.auto_buy_times` | 成本操作 |
| 购买次数 | `plant.friend_steal.buy_count` | 每个好友购买次数，范围 0-10 |

### 花灵与密令

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动种花灵 | `plant.elves.enabled` | 优先指定花灵，否则选当期双倍加成花灵；任务结束恢复原种植模式 |
| 指定花灵 | `plant.elves.selected_ids` | 花灵 ID 列表 |
| 自动申请协助 | `plant.elves.request_aid` | 请求协助 |
| 自动领取协助加成 | `plant.elves.receive_aid` | 协助人数达标后领取 |
| 自动协助好友 | `plant.elves.help_friend` | 协助好友 |
| 自动派遣花灵 | `plant.elves.dispatch` | 派遣背包花灵 |
| 仅派遣双倍花灵 | `plant.elves.dispatch_only_double_buff` | 活动双倍约束 |
| 自动加速派遣 | `plant.elves.speed_up_dispatch` | 元宝成本 |
| 自动领取派遣奖励 | `plant.elves.receive_dispatch_reward` | 星辰币等奖励 |
| 花灵密令等级奖励 | `plant.elves.pass_reward_enabled` | 领取等级奖励 |
| 花灵密令任务奖励 | `plant.elves.pass_task_reward_enabled` | 领取任务奖励 |
| 花之密令等级奖励 | `plant.elves.flower_pass_reward_enabled` | 领取等级奖励 |
| 花之密令任务奖励 | `plant.elves.flower_pass_task_reward_enabled` | 领取任务奖励 |

### 花艺上架

花艺上架建议放在 `order.flower_art` 下，UI 可以仍在种植页附近展示，因为它依赖种植生产。

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动解锁花架 | `order.flower_art.auto_unlock_stand` | 花架解锁，成本需确认 |
| 自动上架 | `order.flower_art.sell_enabled` | 上架花艺并领取金币收益 |
| 提前下架 | `order.flower_art.early_cancel_enabled` | 下架售卖超过一定时间的花艺 |
| 花艺白名单 | `order.flower_art.allowed_art_ids` | 非空时只允许这些花艺；为空时全花艺参与最高价值选择 |
| 上架数量 | `order.flower_art.per_rack_count` | 每个花架数量，0-12 |
| 花艺经验 | `order.flower_art.create_reward_enabled` | 基于 `106.makeList` + `103.artCreateRwdList` 自动领取制作经验 |
| 图鉴奖励 | `order.flower_art.collect_reward_enabled` | 基于 `103` + `c_flowerCollect` 自动领取鲜花/花瓶/花艺图鉴奖励 |

实现进度：`allowed_art_ids` 非空时只在白名单内选择，为空时全花艺按 `SaleValue` 参与最高价值选择。`per_rack_count > 0` 时按未预留库存上架 `min(per_rack_count, available)`，`<= 0` 会阻塞且不制作/不上架。花架会在 `sellStartTime + num * c_flowerRack.$sellTime` 到达后用普通 `flowerRack.recvSellMoney {rackId}` 领取已售收益，再上架未预留成品；无成品时，仅在已解锁配方/花瓶/鲜花且当前未预留鲜花库存足够的范围内选择最高价值可制作花艺，制作数量为 `min(per_rack_count, 当前可制作数量)`；若没有可制作项则跳过本轮，不为花架生成种植需求。花架解锁和提前下架继续 `adapter_missing`。

### 花贸市场

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 解锁货架 | `plant.market.auto_unlock_shelf` | 元宝成本，默认关闭 |
| 自动上架 | `plant.market.put_enabled` | 领取收益并上架花朵，会消耗元宝 |
| 上架策略 | `plant.market.put_mode` | `inventory` 库存最多；`specific` 指定花朵 |
| 指定花朵 | `plant.market.specific_flower_ids` | 上架白名单 |
| 上架价格 | `plant.market.price_index` | 0 最低；1 中等；2 最高 |
| 上架数量 | `plant.market.max_sell` | 1-25 |
| 上架密码 | `plant.market.password` | 4 位数字 |
| 好友摊位扫货 | `plant.market.auto_buy_from_friend` | 自动购买好友货架 |
| 扫货策略 | `plant.market.buy_mode` | `all`、`specific`、`quality` |
| 扫货指定花朵 | `plant.market.buy_flower_ids` | 买入白名单 |
| 扫货指定品质 | `plant.market.buy_qualities` | 品质 1-5 |
| 最小上架时长 | `plant.market.min_put_time_seconds` | 0 不限制 |

## 订单 Order

### 居民订单

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 普通居民订单 | `order.resident.normal_enabled` | 自动提交普通居民订单 |
| 普通订单上限 | `order.resident.normal_daily_limit` | 默认可参考 1200 |
| 绸缎订单 | `order.resident.satin_enabled` | 自动提交绸缎订单 |
| 绸缎订单上限 | `order.resident.satin_daily_limit` | 默认可参考 120 |
| 建材订单 | `order.resident.decorate_enabled` | 自动提交建材订单 |
| 建材订单上限 | `order.resident.decorate_daily_limit` | 默认可参考 120 |
| 品质限定 | `order.resident.qualities` | 仅提交指定品质花朵 |

实现进度：普通居民订单现在受 `normal_enabled`、`normal_daily_limit` 和 `qualities` 共同约束；`124.orderFlowerFinishNum` 已用于今日完成数判断，缺失 `124` 时不会误判达上限，并会留下诊断说明。`normal_daily_limit <= 0` 视为安全阻塞。绸缎/建材已接入 `105.0.6/7` 观察和日上限解释，但协议当前只暴露聚合 `flowers` 字段，缺少可安全提交的花朵需求列表，继续以专门 `adapter_missing` 阻塞项展示。

### 顾客订单

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动完成 | `order.customer.enabled` | 自动完成顾客订单 |
| 暂时无货 | `order.customer.reject_unavailable_enabled` | 花瓶未解锁、配方鲜花未培育时拒单；缺配方/未知状态只阻塞解释 |

实现进度：顾客订单启用后，花艺制作是内建步骤，不再由 `craft_enabled` 独立控制。链路按“成品花艺 -> 配方 -> 花朵 -> 花瓶类型”解释实际数量缺口；成品足够直接 `orderCustomer.finishOrder {npcId}`，成品不足且材料满足时 `flowerArt.makeFlowerArt`，其中 `vaseId` 只是花瓶类型/解锁校验，`num` 才是制作份数。材料不足时生成种植需求。`reject_unavailable_enabled` 开启后，只对确定花瓶类型未解锁或配方鲜花未培育的订单执行 `orderCustomer.rejectOrder {npcId}`；缺配方或状态未观察到时只阻塞，不拒单。`c_flowerArt.lvl` 只作为花艺配置等级展示，不再当作玩家等级门槛。若 namespace `109` 已同步且当前订单为空，并且 `nextGenTime` 冷却已到，自动调用 `orderCustomer.genOrder {guestNpcIdList: []}` 生成普通顾客订单。

### 宫廷订单

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动完成 | `order.palace.enabled` | 自动完成宫廷订单 |
| 品质限定 | `order.palace.qualities` | 仅接受指定品质；不符合时可免费刷新一次 |

实现进度：已接入 namespace `108` 解析，但本轮仍保持 `sync_only`。未观察、可交付或不符合策略时只生成同步/阻塞解释，不执行 `orderPalace.finishOrder`。

### 组团订单

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动完成 | `order.team.enabled` | 自动提交组团订单 |
| 再来一单 | `order.team.one_more_enabled` | 元宝成本 |
| 仅已培育 | `order.team.submit_only_cultivated` | 只提交已培育花朵 |
| 品质限定 | `order.team.qualities` | 仅提交指定品质 |

实现进度：已接入 namespace `107` 解析，但本轮仍保持 `sync_only`。未观察、可提交或不符合策略时只生成同步/阻塞解释，不执行 `orderTeam.submitOrder`。

## 公会 Union

### 公会土地

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动收获 | `union.land_harvest` | 基于 `25.102.fmlLand.landMap` 自动 `fmlLand.harvest` |
| 自动种植 | `union.land_auto_plant` | 空地种植；替换不符合条件的已种土地 |
| 种植策略 | `union.land_plant_mode` | `quality` 品质；`specific` 指定花 |
| 指定品质 | `union.land_qualities` | 留空不限制 |
| 指定花朵 | `union.land_flower_ids` | 留空不限制 |
| 最高等级限制 | `union.land_max_flower_level` | 高于该等级不种，0 不限制 |

实现状态：公会土地收获已接入 `25.102`，当 `matureFlwCnt > harvestedFlwCnt` 时生成 `fmlLand.harvest {landIds}`。公会土地种植、解锁、升级仍需确认花朵消耗、土地等级成本与替换策略。

### 公会建设

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 视频建设 | `union.build_free_enabled` | 看视频建设 |
| 金币建设 | `union.build_gold_enabled` | 花费金币建设 |
| 元宝建设 | `union.build_diamond_enabled` | 花费元宝，默认不启用 |

实现状态：已接入 `25.fmlTot.fmlBld.bldCountMap` + `c_fmlBld`，planner 会按每日次数和成本门槛生成 `Fml.build {id}`。视频建设无成本；金币建设使用金币预算和余额校验；元宝建设会展示成本与阻塞原因，执行层仍按项目约定默认拦截元宝消耗。

### 公会分享与摸花

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动分享 | `union.flower_share_enabled` | 分享花到公会 |
| 分享模式 | `union.flower_share_mode` | `quality`、`specific` |
| 分享品质 | `union.flower_share_qualities` | 品质筛选 |
| 分享花朵 | `union.flower_share_ids` | 指定花朵 |
| 自动摸花 | `union.flower_take_enabled` | 摸取别人分享 |
| 摸花模式 | `union.flower_take_mode` | `quality`、`specific` |
| 摸花品质 | `union.flower_take_qualities` | 品质筛选 |
| 摸花花朵 | `union.flower_take_ids` | 指定花朵 |

实现状态：已接入 `25.107.fmlShare`、`25.108.otherMbFmlShare`，可自动 `fmlFlowerShare.recvRwd {slotIds}` 领取分享槽位奖励，并按花朵/品质策略执行 `fmlFlowerShare.take {dstUid, slotId}` 摸取成员分享。`fmlFlowerShare.refresh` 和 `getFmlOtherShareList` 用作未观测状态的同步入口。自动分享花到公会仍需确认是否占用/消耗库存和槽位策略，暂不自动执行。

### 公会竞赛

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 自动完成 | `union.race_enabled` | 自动领取并完成任务 |
| 自动启用模块 | `union.race.auto_enable_modules` | 按任务临时启用种植、订单、花艺等，完成后恢复 |
| 种植任务用加速卡 | `union.race.use_speedup_ticket_in_task` | 任务期间忽略上限，完成后恢复 |
| 限制分数 | `union.race.min_task_score` | 0 不限制，范围可参考 0-60 |
| 只接已升级任务 | `union.race.only_upgrade_task` | 系统升级或用户升级任务 |
| 排除他人升级任务 | `union.race.exclude_others_upgrade_task` | 避免接他人升级任务 |
| 任务优先级 | `union.race.task_type_priority` | 0 表示不接，数字越小越优先 |
| 自动升级任务 | `union.race.upgrade_task` | 元宝成本 |
| 删除低分任务 | `union.race.delete_low_score_task` | 会长/副会长权限 |
| 删除分数上限 | `union.race.delete_task_max_score` | 低于此分数删除 |

公会竞赛任务类型建议先保留映射表：

| 任务 | 默认优先级 |
| --- | --- |
| 2004 | 0 |
| 3006 | 2 |
| 3016 | 2 |
| 3017 | 3 |
| 3018 | 2 |
| 3023 | 3 |
| 3024 | 3 |
| 3030 | 2 |
| 3034 | 2 |
| 3035 | 3 |
| 3036 | 1 |
| 3044 | 3 |
| 3052 | 3 |

后续应根据协议捕获补齐中文任务名。

### 公会其他

| 功能 | 字段建议 | 说明 |
| --- | --- | --- |
| 公会红包 | `union.red_packet_enabled` | 自动领取公会红包 |
| 能量森林 | `union.forest_enabled` | 基于 `25.127.fmlForestEnergy` 自动 `fmlForest.refresh {isAutoCollect:1}` |

实现状态：能量森林收能量已接入 `25.127`，当 `dailyTempEnergyMap` 中存在待收临时能量时生成 `fmlForest.refresh {isAutoCollect:1}`；首次未观测森林状态时也会先执行一次刷新同步。种树、证书、周统计和树池管理仍保留待确认。

## 活动 Activity

活动页应采用“活动注册表 + 模块参数”的方式。每个活动都有：

- `enabled`：是否启用。
- `status`：当前实现状态。
- `params`：活动特有参数。
- `cost_guard`：元宝、体力、道具、次数限制。
- `valid_until`：如果能观测到活动结束时间，应展示。

### 活动模块清单

| 活动 | module id | 参数 | 说明 |
| --- | --- | --- | --- |
| 花笺集芳 | `cyclicNote` | `unlock_slot`、`auto_enable_modules` | 自动完成任务和阶段宝箱；可按任务临时启用模块 |
| 莳花纪闻 | `actCyclicStory` | `refresh_enabled`、`max_finish_count_per_batch` | 订单型活动；缺花时进入种植需求 |
| 丰仓鱼干 | `fishMerge` | `show_result`、`auto_restart` | 合成/小游戏类 |
| 奇妙泡泡 | `magicBubble` | 无额外参数 | 小游戏类 |
| 花香满园 | `zooGameElim` | 无额外参数 | 消消乐类 |
| 鱼乐无穷 | `fishFun` | `auto_claim_energy`、`speed`、`show_result`、`auto_restart` | 倍速 1/4/8/16 |
| 花漾物语 | `actElim` | `auto_claim_energy`、`speed` | 倍速 1/5/10/25/100 |
| 梳丝引线 | `actSpool` | `auto_claim_energy`、`speed` | 倍速 1/5/10/25/100 |
| 红包雨 | `redPacket` | 无额外参数 | 自动抢红包 |
| 迎新接福 | `recvLuck` | 无额外参数 | 自动领取福袋 |
| 为紫打 call | `yzCall` | 无额外参数 | 自动打 call |
| 摇钱树 | `moneyTree` | 无额外参数 | 自动摇钱 |
| 元宵灯谜 | `lanternFestival` | 无额外参数 | 自动答题并领奖 |
| 龙舟竞渡 | `actDuanWu` | `claim_box`、`open_box` | 自动划龙舟、进度宝箱、舟赛宝箱 |
| 香卉甜糕 | `actDessert` | `auto_claim_energy`、`use_items`、`speed` | 倍速普通/快速/高速/极速/神速 |
| 田园奇趣 | `actMerge2` | `auto_claim_energy`、`speed` | 倍速 1x/2x/4x/8x/16x/32x |

## 策略模型落地

当前 `policy.proto` 已经按完整设置树拆成结构化子策略。后续新增执行器时应优先复用这些子策略，不再新增同级扁平开关。

### BasicPolicy 扩展

- `ReputationPolicy reputation`
- `BasicTaskPolicy task`
- `BenefitPolicy benefit`
- `SignPolicy sign`
- `PearlPolicy pearl`
- `ShopPolicy shop`
- `FeedCatPolicy feed_cat`
- 水滴领取保留在基础域：`waterwheel_enabled`、`free_water_enabled`、`water_claim_threshold`

### PlantPolicy 扩展

- `CultivatePolicy cultivate`
- `FlowerPlantPolicy flower`
- `FriendStealPolicy friend_steal`
- `FlowerElvesPolicy elves`
- `FlowerMarketPolicy market`

### OrderPolicy 扩展

- `ResidentOrderPolicy` 已包含 daily limit 和 qualities。
- `CustomerOrderPolicy` 已包含 `enabled`、`reject_unavailable_enabled`；花艺制作是顾客订单内建步骤。
- `PalaceOrderPolicy` 已包含 qualities。
- `TeamOrderPolicy` 已包含 `one_more_enabled`、`submit_only_cultivated`、qualities 和元宝上限。
- `FlowerArtPolicy` 已包含 `allowed_art_ids`、花架数量、图鉴/经验奖励、提前下架。

### UnionPolicy 扩展

- `UnionBuildPolicy build`
- `UnionLandPolicy land`
- `UnionFlowerPolicy flower`
- `UnionRacePolicy race`
- 保留 red packet、forest 开关。

### ActivityPolicy 扩展

现有 `ActivityModulePolicy` 已支持通用参数：

```proto
message ActivityModulePolicy {
  bool enabled = 1;
  map<string, int64> int_params = 2;
  map<string, bool> bool_params = 3;
  map<string, string> string_params = 4;
  map<string, IntList> int_list_params = 5;
}
```

更推荐长期方案是活动注册表驱动的 typed policy：常驻活动用 typed message，短期活动用 param map。

## 状态与协议依赖

| 功能组 | 依赖状态/namespace | 当前缺口 |
| --- | --- | --- |
| 土地、浇水、收获、种植 | `100`、`7`、`114`、`117` | 核心已建模 |
| 培育、升级 | `101`、`7` | 已有基础 |
| 居民订单 | `105`、`124`、`7` | 普通订单开关、日上限和品质限定已接；绸缎/建材已观察但缺可提交需求列表，仍阻塞 |
| 顾客订单 | `109`、`101`、`102`、`106`、`7` | 完成/制作/暂时无货链路已接；花艺成品、配方、花朵、花瓶链路按实际数量解释 |
| 宫廷/组团订单 | `108`、`107`、`124`、`7`、`101` | 已接状态解析，但本轮保持 `sync_only`，不执行 finish/submit |
| 花艺/花架 | `102`、`103`、`104`、`106`、花艺/图鉴静态表、`7` | `allowed_art_ids`、最高价值制作/上架、花架普通收益领取已接；花架解锁/提前下架成本仍需确认；花艺经验和图鉴奖励已接自动领取 |
| 任务奖励 | `22`、`119`、`124` | 已有部分 |
| 福利/宝箱 | `116`、`117`、`118`、`114`、`7.13.1 usrExtra` | 福利宝箱、限时水滴、水车、防骗宝箱已接入；双倍金币已接状态但执行受 SDK token 阻塞；分享奖励仍待确认 |
| 商城材料 | `113`、`c_shop_cultivate`、`7` | 材料商店已接入 enter/buy；元宝成本仍默认阻塞 |
| 商城礼包 | `112`、`c_shop_giftbag`、`7` | 视频免费礼包已接入 enter/buy；充值/VIP 礼包仍阻塞 |
| 珍珠 | `115`、`c_pearl`、`c_pearlDraw`、`7` | 已接入 refresh/free/draw/protect/place recv；雇佣和买书仍阻塞 |
| VIP 商城、花灵、花贸 | 待捕获确认 | 先登记 sync_only/adapter_missing |
| 猫/动物 | `33`、`c_zoo*` | 喂猫/撸猫已接；买猫粮、召回仍阻塞 |
| 公会 | `25`、`152` 等待确认 | 建设、土地收获、摸花、分享奖励、森林收能量已接；后续公会扩展按当前决策暂停 |
| 活动 | 活动 namespace 分散 | 用活动注册表逐个接入 |

## 未完成项状态

| 模块 | 功能 | 状态 | 停止前结论 |
| --- | --- | --- | --- |
| 基础 | 双倍金币 | `adapter_missing` | 状态 `118` 已接；自动执行缺合法 SDK 广告 token |
| 基础 | 分享奖励 | `sync_only` | `usr.share/afterShare` 与具体分享场景需分功能确认 |
| 基础 | 自动补签 | `adapter_missing` | 补签消耗未确认，成本操作不自动放开 |
| 基础 | VIP 商店 | `adapter_missing` | 商品状态、花坊币/元宝成本未完成协议确认 |
| 基础 | 雇佣劳工/买雇佣书 | `adapter_missing` | 候选 UID、雇佣券、元宝成本仍需确认 |
| 基础 | 买猫粮/召回猫 | `adapter_missing` | 成本和事件链路未确认；喂猫/撸猫已可执行 |
| 种植 | 好友偷花、花灵、花贸市场、花/花灵密令 | `sync_only` | 已登记产品能力，执行器未接 |
| 订单 | 居民绸缎订单、居民建材订单 | `adapter_missing` | 仅普通居民订单执行化；绸缎/建材开启时只展示阻塞解释 |
| 订单 | 宫廷订单、组团订单 | `sync_only` | 状态解析保留，finish/submit 本轮不执行化 |
| 订单 | 组团再来一单 | `adapter_missing` | 涉及元宝成本，默认不自动执行 |
| 订单 | 花架解锁 | `adapter_missing` | 解锁成本未确认 |
| 公会 | 竞赛、红包、自动分享花、公会土地种植 | `paused` | 按当前决策先不继续拓展公会相关功能 |
| 活动 | 大多数活动模块 | `sync_only`/`adapter_missing` | 保留注册表方向，逐个按协议成熟度接入 |

## ADAPTER_MISSING 抓包清单

以下清单按“先非公会、先能转成确定执行”的顺序排列。抓包时每项至少记录：入口页面、触发前 namespace 快照、RPC 名称和参数、返回 `v` 片段、库存/金币/元宝/次数变化。

| 优先级 | 功能 | 当前阻塞点 | 已知线索 | 抓包目标 |
| --- | --- | --- | --- | --- |
| P0 | 居民绸缎订单 | `105.0.6` 只解析到聚合 `flowers`，缺安全提交需求列表 | RPC 已知 `orderFlower.finishSatinOrder`，请求为 raw | 打开订单页、完成一次绸缎订单；确认请求字段、需求来源、统计 `124` 增量 |
| P0 | 居民建材订单 | `105.0.7` 只解析到聚合 `flowers`，缺安全提交需求列表 | RPC 已知 `orderFlower.finishDecorateOrder`，请求为 raw | 打开订单页、完成一次建材订单；确认请求字段、需求来源、统计 `124` 增量 |
| P0 | 花架解锁 | 成本和可解锁状态未确认 | RPC 已知 `flowerRack.unlockStand {standId}` | 点击解锁花架；确认 `standId`、金币/元宝/物品成本、`104` 状态变化 |
| P0 | 花架提前下架 | 暂未形成策略和成本/返还规则 | RPC 已知 `flowerRack.cancelSell {rackId}` | 对上架中花艺执行下架；确认返还物品、收入损失、`104` 状态变化 |
| P0 | 组团再来一单/接单 | 元宝成本和请求语义未放开 | RPC 已知 `orderTeam.takeOrder {isAgree,isCost}`、`storeOrder`、`takeStoredOrder` | 触发“再来一单”、存单、取存单；确认 `isCost` 与元宝/次数变化 |
| P1 | 双倍金币/视频类奖励 | 依赖客户端 SDK token，本地 runner 不伪造 | `UT.share(11,{opType:1})` 后调用 `usr.share {shareId,ext}`，另有 `usr.afterShare` raw | 完整录一条视频成功链路；确认 `shareId`、`ext`、token 字段和服务端校验错误 |
| P1 | 分享奖励 | 分享场景和领奖链路分散 | 已有 `usr.share`、`usr.afterShare`、`frdShare.*`、`usrExtra.shareMsg` | 分别抓普通分享、好友分享奖励、分享宝箱；确认可无 SDK 执行的奖励边界 |
| P1 | 自动补签 | 补签成本未确认 | `signType.sign` 请求含 `patchDay` 字段；已有每日签到链路 | 对漏签日补签；确认 `patchDay`、金币/元宝/道具成本、`140` 状态变化 |
| P1 | 珍珠雇佣劳工 | 候选 UID、雇佣券、成本和失败规则未确认 | 已知 `pearl.getRecommendList`、`pearl.getHireStateByUids`、`pearlPlace.hire` | 推荐列表、指定 UID 雇佣、雇佣失败各抓一次；确认 `placeId/dstUid` 和成本 |
| P1 | 买雇佣书 | 元宝成本默认不放开 | 可能走珍珠或商店链路 | 点击购买雇佣书；确认 RPC、商品 ID、成本物品和购买次数 |
| P1 | 珍珠防身符不足时启用防身 | 缺防身符来源和购买链路 | 已接 `pearl.setProtectState {protectState}`，缺来源 | 防身符不足时点击开启；确认是否弹购买、消耗什么、是否有独立 RPC |
| P1 | VIP 商店 | 商品状态、花坊币/元宝成本未确认 | 已知 `vip.recv`、`actVipTimeShop.giftBuy`、`usr.updateVipService` 等相关 RPC | 进入 VIP/限时 VIP 商店并购买一项；确认商品 namespace、价格类型、领取/购买 RPC |
| P1 | 买猫粮/加食物 | 商品选择和成本未确认 | 已知 `zoo.addFoodstuff {foodstuffIds}`，喂猫已可执行 | 猫碗无食物时购买/添加猫粮；确认食物 ID、来源库存、购买成本 |
| P1 | 自动召回猫 | 事件链路和成本未确认 | 相关 RPC 包括 `zoo.findPet`、`zoo.handleEvent`，字段含 `isShareVideo` | 触发召回/寻找/事件处理；确认是否视频、金币、道具或免费 |
| P2 | 花艺/培育静态配置缺口 | 缺配方或培育材料时会阻塞制作/培育 | 依赖花艺配方表、培育成本表和运行时 `102/103/106` | 优先补静态表来源；抓包只用于确认客户端实际选择的配方和材料扣减 |
| P2 | 礼仪分监控 | 缺礼仪分状态来源和阈值语义 | schema 中有 `usrTot.reputationTot` 线索 | 打开礼仪分页面；确认 namespace、字段、扣分/恢复记录 |
| P2 | 福利泛入口 | `basic.welfare` 仍是泛能力，缺具体可执行 adapter | 已接水车、限时水滴、福利宝箱、防骗宝箱，剩余需拆项 | 逐个点击福利页剩余红点；把泛入口拆成具体 RPC 和状态 |
| P2 | 元宝成本放行模型 | 材料商店、买雇佣书、公会建设等元宝动作默认拦截 | `CostGate` 已能表达元宝成本，缺单功能授权和预算策略 | 不是单纯抓包项；需要为每个元宝功能确认成本、上限、回滚风险和 UI 二次授权 |
| 暂停 | 公会竞赛升级/红包/自动分享/公会土地种植 | 当前阶段不继续拓展公会 | RPC/namespace 线索已存在但暂停 | 暂不主动抓；若顺手抓到，仅归档 |
| 暂停 | 活动小游戏/节日模块 | 活动协议分散，优先级低于非公会订单链路 | feature catalog 已登记 adapter_missing/sync_only | 暂不主动抓；等核心链路稳定后按活动逐个拆 |

## UI 信息架构

设置页建议采用五个一级 Tab：

1. 基础
2. 种植
3. 订单
4. 公会
5. 活动

每个功能项用统一组件：

- 左侧：名称、风险标签、状态标签。
- 右侧：开关、数值、选择器。
- 下方：解释文本、成本门槛、阻塞原因。

高级设置默认折叠，例如：

- 珍珠雇佣参数。
- 花贸市场。
- 公会竞赛任务优先级。
- 活动倍速。
- 自动启用模块。

## 实现分期

### P0：核心可解释自动化

- 基础：主线/每日/每周、邮件、水车、限时水滴、福利宝箱、随机事件。
- 种植：收获、浇水、种植、土地解锁、培育、鲜花升级。
- 订单：居民、顾客、花艺制作/上架。
- UI：需求队列、库存账本、操作队列、订单统计、阻塞汇总。

### P1：策略细化

- 任务优先模式 UI。
- 目标优先级配置。
- 居民订单单日上限和品质限定已接；绸缎/建材订单继续等待协议执行化。
- 花艺指定上架、指定制作、花架数量、图鉴/经验奖励已接；提前下架待成本/协议确认。
- 宫廷订单、组团订单已接状态同步、需求、账本和提交计划；组团再来一单待元宝成本策略。
- 成本门槛体系：`PlanStatus` enum 与 `CostGate` 已进入查询 schema；操作、需求、领域状态、待办任务使用统一状态，金币、元宝、道具、水滴会形成结构化门槛。
- 执行入口：runner operation registry 已承接计划操作的参数构造和实际 RPC 执行。

### P2：社交与公会（暂停拓展）

- 已接入能力保留运行：公会建设、公会土地收获、摸花、分享奖励、森林收能量。
- 未完成能力暂不推进：自动分享花、公会竞赛、公会红包、公会土地种植、能量森林扩展。

### P3：活动平台化

- 活动注册表。
- 活动参数 schema。
- 活动状态观测与有效期。
- 常驻活动优先接入：花笺集芳、莳花纪闻、红包雨、摇钱树。
- 小游戏类活动按协议成熟度逐步接入。

## 对当前代码的直接落点

| 文件/模块 | 建议动作 |
| --- | --- |
| `proto/mygardenworld/v1/policy.proto` | 已完成树形策略 schema；后续允许合理破坏性调整 |
| `proto/mygardenworld/v1/query_service.proto` | 已接 `PlanStatus`、`CostGate`、订单统计、库存账本、阻塞汇总 |
| `internal/policycfg` | 继续维护默认值和 normalize；旧策略 JSON 不做兼容包袱 |
| `internal/automation/features.go` | 将本文功能登记为 feature catalog |
| `internal/automation/automation.go` | 继续让 demand/ledger 驱动种植、制作、提交和阻塞解释 |
| `internal/state` | 已补花瓶、花艺能力、订单统计、宫廷/组团状态；活动状态继续按协议推进 |
| `internal/runner` | 已用 operation registry 承接新增执行器 |
| `web/src/app/page.tsx` | 已有五类 Tab、功能状态、成本门槛、结构化账本和阻塞汇总 |

## 非目标

- 不复制第三方页面视觉资源。
- 不依赖第三方页面保护逻辑的移除。
- 不把元宝成本操作默认打开。
- 不为了凑设置项而绕过状态建模；没有状态支撑的功能先标记为 `sync_only` 或 `adapter_missing`。
