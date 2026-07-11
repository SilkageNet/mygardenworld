package runner

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// RuntimeStats tracks one in-memory account run, from runner start until stop.
// It intentionally does not persist; Manager keeps the last stopped snapshot
// so the UI can still show the latest run after a logout/stop.
type RuntimeStats struct {
	mu sync.RWMutex

	startedAt time.Time
	stoppedAt time.Time
	updatedAt time.Time
	running   bool

	lastResources     map[string]int64
	inventoryObserved bool

	resourceGains       map[string]RuntimeResourceTotal
	orderCompletions    map[string]RuntimeActionTotal
	taskCompletions     map[string]RuntimeActionTotal
	operationCompletion map[string]RuntimeActionTotal
	totalOperations     int64
}

type RuntimeStatsSnapshot struct {
	StartedAt            time.Time
	StoppedAt            time.Time
	UpdatedAt            time.Time
	Running              bool
	ResourceGains        []RuntimeResourceTotal
	OrderCompletions     []RuntimeActionTotal
	TaskCompletions      []RuntimeActionTotal
	OperationCompletions []RuntimeActionTotal
	TotalOperations      int64
}

type RuntimeResourceTotal struct {
	Key    string
	Label  string
	ItemID int32
	Gained int64
}

type RuntimeActionTotal struct {
	Key   string
	Label string
	Count int64
}

func newRuntimeStats(now time.Time) *RuntimeStats {
	now = normalizeStatsTime(now)
	return &RuntimeStats{
		startedAt:           now,
		updatedAt:           now,
		running:             true,
		lastResources:       make(map[string]int64),
		resourceGains:       make(map[string]RuntimeResourceTotal),
		orderCompletions:    make(map[string]RuntimeActionTotal),
		taskCompletions:     make(map[string]RuntimeActionTotal),
		operationCompletion: make(map[string]RuntimeActionTotal),
	}
}

func (s *RuntimeStats) ObserveResourceSnapshot(snap state.ResourceSnapshot, at time.Time) {
	if s == nil {
		return
	}
	at = normalizeStatsTime(at)
	values := runtimeResourceValues(snap)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range values {
		if prev, ok := s.lastResources[item.Key]; ok {
			if delta := item.Gained - prev; delta > 0 {
				s.addResourceGainLocked(item.Key, item.Label, 0, delta)
			}
		}
		s.lastResources[item.Key] = item.Gained
	}
	s.touchLocked(at)
}

func (s *RuntimeStats) ObserveInventorySnapshot(snap state.InventorySnapshot, at time.Time) {
	if s == nil {
		return
	}
	at = normalizeStatsTime(at)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.inventoryObserved {
		s.inventoryObserved = true
		s.touchLocked(at)
		return
	}
	for _, change := range snap.Changes {
		if change.ItemID == 0 || change.ItemID == waterDropsItemID {
			continue
		}
		if delta := int64(change.After) - int64(change.Before); delta > 0 {
			s.addResourceGainLocked(runtimeItemResourceKey(change.ItemID), runtimeItemResourceLabel(change.ItemID), change.ItemID, delta)
		}
	}
	s.touchLocked(at)
}

func (s *RuntimeStats) RecordOperationSuccess(op *automation.PlannedOp, at time.Time) {
	if s == nil || op == nil || op.Kind == "" {
		return
	}
	at = normalizeStatsTime(at)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalOperations++
	s.addActionTotalLocked(s.operationCompletion, op.Kind, runtimeOperationLabel(op.Kind), 1)
	if key, label, ok := runtimeOrderCompletion(op.Kind); ok {
		s.addActionTotalLocked(s.orderCompletions, key, label, 1)
	}
	if key, label, ok := runtimeTaskCompletion(op.Kind); ok {
		s.addActionTotalLocked(s.taskCompletions, key, label, 1)
	}
	s.touchLocked(at)
}

func (s *RuntimeStats) MarkStopped(at time.Time) {
	if s == nil {
		return
	}
	at = normalizeStatsTime(at)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.stoppedAt = at
	s.touchLocked(at)
}

func (s *RuntimeStats) Snapshot() RuntimeStatsSnapshot {
	if s == nil {
		return RuntimeStatsSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return RuntimeStatsSnapshot{
		StartedAt:            s.startedAt,
		StoppedAt:            s.stoppedAt,
		UpdatedAt:            s.updatedAt,
		Running:              s.running,
		ResourceGains:        sortedRuntimeResources(s.resourceGains),
		OrderCompletions:     sortedRuntimeActions(s.orderCompletions, runtimeOrderPriority),
		TaskCompletions:      sortedRuntimeActions(s.taskCompletions, runtimeTaskPriority),
		OperationCompletions: sortedRuntimeActions(s.operationCompletion, nil),
		TotalOperations:      s.totalOperations,
	}
}

func (s *RuntimeStats) addResourceGainLocked(key, label string, itemID int32, delta int64) {
	if delta <= 0 {
		return
	}
	total := s.resourceGains[key]
	total.Key = key
	total.Label = label
	total.ItemID = itemID
	total.Gained += delta
	s.resourceGains[key] = total
}

func (s *RuntimeStats) addActionTotalLocked(m map[string]RuntimeActionTotal, key, label string, delta int64) {
	if delta <= 0 {
		return
	}
	total := m[key]
	total.Key = key
	total.Label = label
	total.Count += delta
	m[key] = total
}

func (s *RuntimeStats) touchLocked(at time.Time) {
	if at.After(s.updatedAt) || s.updatedAt.IsZero() {
		s.updatedAt = at
	}
}

func normalizeStatsTime(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC()
}

func runtimeResourceValues(snap state.ResourceSnapshot) []RuntimeResourceTotal {
	return []RuntimeResourceTotal{
		{Key: "gold", Label: "金币", Gained: int64(snap.Gold)},
		{Key: "water_drops", Label: "水滴", Gained: int64(snap.WaterDrops)},
		{Key: "experience", Label: "经验", Gained: int64(snap.Experience)},
		{Key: "diamonds_free", Label: "元宝", Gained: int64(snap.DiamondsFree)},
		{Key: "diamonds_paid", Label: "付费元宝", Gained: int64(snap.DiamondsPaid)},
		{Key: "vip_exp", Label: "VIP经验", Gained: int64(snap.VipExp)},
	}
}

func runtimeItemResourceKey(itemID int32) string {
	return fmt.Sprintf("item:%d", itemID)
}

func runtimeItemResourceLabel(itemID int32) string {
	if name := state.ItemName(itemID); name != "" {
		return name
	}
	return fmt.Sprintf("#%d", itemID)
}

func runtimeOperationLabel(kind string) string {
	switch kind {
	case clientproto.RPCOrderFlowerFinishOrder.String():
		return "居民订单提交"
	case clientproto.RPCOrderCustomerFinishOrder.String():
		return "顾客订单提交"
	case clientproto.RPCOrderPalaceFinishOrder.String():
		return "宫廷订单提交"
	case clientproto.RPCOrderTeamSubmitOrder.String():
		return "组团订单提交"
	case clientproto.RPCOrderFlowerRecvOrderRwd.String():
		return "居民订单奖励"
	case clientproto.RPCOrderCustomerGenOrder.String():
		return "生成顾客订单"
	case clientproto.RPCOrderCustomerRejectOrder.String():
		return "顾客订单暂时无货"
	case clientproto.RPCFlowerArtMakeFlowerArt.String():
		return "制作花艺"
	case clientproto.RPCFlowerRackSell.String():
		return "花艺上架"
	case clientproto.RPCFlowerRackRecvSellMoney.String():
		return "领取花艺收益"
	case clientproto.RPCTaskMainRecv.String():
		return "领取主线任务"
	case clientproto.RPCTaskDlyRecv.String():
		return "领取日常任务"
	case clientproto.RPCTaskWeekRecv.String():
		return "领取周常任务"
	case clientproto.RPCTaskAchRecv.String():
		return "领取成就任务"
	case clientproto.RPCRoadGrowRecv.String():
		return "领取成长之路"
	case clientproto.RPCStoryMainUnlock.String():
		return "解锁主线剧情"
	case clientproto.RPCRandomEventDoAffair.String():
		return "处理地图随机事件"
	default:
		return opKindDesc(kind)
	}
}

func runtimeOrderCompletion(kind string) (key, label string, ok bool) {
	switch kind {
	case clientproto.RPCOrderFlowerFinishOrder.String():
		return "resident_normal", "居民订单", true
	case clientproto.RPCOrderFlowerFinishSatinOrder.String():
		return "resident_satin", "绸缎订单", true
	case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
		return "resident_decorate", "建材订单", true
	case clientproto.RPCOrderCustomerFinishOrder.String():
		return "customer", "顾客订单", true
	case clientproto.RPCOrderPalaceFinishOrder.String():
		return "palace", "宫廷订单", true
	case clientproto.RPCOrderTeamSubmitOrder.String():
		return "team", "组团订单", true
	case clientproto.RPCFlowerRackRecvSellMoney.String():
		return "flower_art", "花艺售卖", true
	default:
		return "", "", false
	}
}

func runtimeTaskCompletion(kind string) (key, label string, ok bool) {
	switch kind {
	case clientproto.RPCTaskMainRecv.String():
		return "main", "主线任务", true
	case clientproto.RPCTaskDlyRecv.String():
		return "daily", "日常任务", true
	case clientproto.RPCTaskWeekRecv.String():
		return "weekly", "周常任务", true
	case clientproto.RPCTaskAchRecv.String():
		return "achievement", "成就任务", true
	case clientproto.RPCRoadGrowRecv.String():
		return "road_grow", "成长之路", true
	case clientproto.RPCStoryMainUnlock.String():
		return "story", "主线剧情", true
	case clientproto.RPCRandomEventDoAffair.String():
		return "random_event", "地图随机事件", true
	case clientproto.RPCZooHandleEvent.String():
		return "zoo_event", "宠物事件", true
	case clientproto.RPCActCyclicNoteRecvTaskRwd.String():
		return "cyclic_note", "花笺集芳任务", true
	default:
		return "", "", false
	}
}

var runtimeResourcePriority = map[string]int{
	"gold":          0,
	"water_drops":   1,
	"experience":    2,
	"diamonds_free": 3,
	"diamonds_paid": 4,
	"vip_exp":       5,
}

var runtimeOrderPriority = map[string]int{
	"resident_normal":   0,
	"customer":          1,
	"palace":            2,
	"team":              3,
	"resident_satin":    4,
	"resident_decorate": 5,
	"flower_art":        6,
}

var runtimeTaskPriority = map[string]int{
	"main":         0,
	"daily":        1,
	"weekly":       2,
	"achievement":  3,
	"story":        4,
	"road_grow":    5,
	"random_event": 6,
	"zoo_event":    7,
	"cyclic_note":  8,
}

func sortedRuntimeResources(m map[string]RuntimeResourceTotal) []RuntimeResourceTotal {
	out := make([]RuntimeResourceTotal, 0, len(m))
	for _, item := range m {
		if item.Gained > 0 {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi, iPriority := runtimeResourcePriority[out[i].Key]
		pj, jPriority := runtimeResourcePriority[out[j].Key]
		if iPriority != jPriority {
			return iPriority
		}
		if iPriority && pi != pj {
			return pi < pj
		}
		if out[i].Gained != out[j].Gained {
			return out[i].Gained > out[j].Gained
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func sortedRuntimeActions(m map[string]RuntimeActionTotal, priority map[string]int) []RuntimeActionTotal {
	out := make([]RuntimeActionTotal, 0, len(m))
	for _, item := range m {
		if item.Count > 0 {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if priority != nil {
			pi, iPriority := priority[out[i].Key]
			pj, jPriority := priority[out[j].Key]
			if iPriority != jPriority {
				return iPriority
			}
			if iPriority && pi != pj {
				return pi < pj
			}
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Key < out[j].Key
	})
	return out
}
