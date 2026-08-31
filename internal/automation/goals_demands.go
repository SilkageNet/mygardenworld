package automation

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func enabledGoals(policy *pb.Policy) []Goal {
	plant := policy.GetPlant()
	priorities := plant.GetPlanting().GetDemandPriority()
	var goals []Goal
	add := func(enabled bool, id, category, domain, label string) {
		if !enabled {
			return
		}
		goals = append(goals, Goal{
			ID:       id,
			Category: category,
			Domain:   domain,
			Label:    label,
			Priority: priorityFor(priorities, id),
		})
	}
	basic := policy.GetBasic()
	task := basic.GetTask()
	order := policy.GetOrder()
	add(order.GetResident().GetNormalEnabled() || order.GetResident().GetRewardEnabled() ||
		order.GetResident().GetDecorateEnabled() || order.GetResident().GetSatinEnabled(), GoalResidentOrder, CategoryOrder, "order.resident", "居民订单")
	add(order.GetCustomer().GetEnabled(), GoalCustomerOrder, CategoryOrder, "order.customer", "顾客订单")
	add(order.GetPalace().GetEnabled(), GoalPalaceOrder, CategoryOrder, "order.palace", "宫廷订单")
	add(order.GetTeam().GetEnabled(), GoalTeamOrder, CategoryOrder, "order.team", "组团订单")
	flowerArt := order.GetFlowerArt()
	add(flowerArt.GetSellEnabled() || flowerArt.GetCraftEnabled() || flowerArt.GetCreateRewardEnabled() || flowerArt.GetCollectRewardEnabled(), GoalFlowerArt, CategoryOrder, "order.flower_art", "花艺/花架")
	add(task.GetMainEnabled(), GoalMainTask, CategoryBasic, "basic.task.main", "主线任务")
	add(task.GetDailyEnabled(), GoalDailyTask, CategoryBasic, "basic.task.daily", "每日任务")
	add(task.GetWeeklyEnabled(), GoalWeeklyTask, CategoryBasic, "basic.task.weekly", "每周任务")
	return goals
}

func priorityFor(priorities map[string]int32, id string) int32 {
	if v := priorities[id]; v != 0 {
		return v
	}
	return defaultDemandPriority()[id]
}

func goalByID(goals []Goal, id string) (Goal, bool) {
	for _, goal := range goals {
		if goal.ID == id {
			return goal, true
		}
	}
	return Goal{}, false
}

func buildDirectDemands(s *state.State, policy *pb.Policy, goals []Goal, now time.Time) []Demand {
	inventory := s.Inventory()
	var out []Demand
	add := func(goal Goal, source, kind string, itemID, count int32, entityID, label string, blocked []string) {
		if itemID <= 0 || count <= 0 {
			return
		}
		have := inventory[itemID]
		missing := count - have
		if missing < 0 {
			missing = 0
		}
		out = append(out, Demand{
			ID:             demandID(goal.ID, entityID, source, kind, itemID),
			GoalID:         goal.ID,
			Category:       goal.Category,
			Domain:         goal.Domain,
			EntityID:       entityID,
			Source:         source,
			Label:          label,
			Kind:           kind,
			ItemID:         itemID,
			Count:          count,
			Have:           have,
			Available:      have,
			Missing:        missing,
			Priority:       goal.Priority,
			BlockedReasons: append([]string(nil), blocked...),
		})
	}
	if goal, ok := goalByID(goals, GoalResidentOrder); ok {
		resident := policy.GetOrder().GetResident()
		if resident.GetNormalEnabled() {
			if _, limited := residentNormalDailyLimitReached(s, resident, now); !limited {
				for boxID, order := range s.FlowerOrders() {
					if order == nil || order.IsVideo != 0 || !residentFlowerOrderAllowed(order, resident) {
						continue
					}
					entityID := strconv.FormatInt(int64(boxID), 10)
					for _, req := range order.Requires {
						add(goal, "direct", DemandKindFlower, req.FlowerID, req.Count, entityID, fmt.Sprintf("居民订单 #%d", boxID), nil)
					}
				}
			}
		}
		if resident.GetSatinEnabled() {
			if _, limited := residentSatinDailyLimitReached(s, resident, now); !limited {
				satin := s.ResidentSatinOrder()
				if satin.IsVideo == 0 && residentSpecialOrderAllowed(satin, resident) && satin.CooldownReady(now) {
					for _, req := range satin.Requires {
						add(goal, "direct", DemandKindFlower, req.FlowerID, req.Count, "satin", "绸缎居民订单", nil)
					}
				}
			}
		}
		if resident.GetDecorateEnabled() {
			if _, limited := residentDecorateDailyLimitReached(s, resident, now); !limited {
				decorate := s.ResidentDecorateOrder()
				if decorate.IsVideo == 0 && residentSpecialOrderAllowed(decorate, resident) && decorate.CooldownReady(now) {
					for _, req := range decorate.Requires {
						add(goal, "direct", DemandKindFlower, req.FlowerID, req.Count, "decorate", "建材居民订单", nil)
					}
				}
			}
		}
	}
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		if _, limited := customerDailyLimitReached(s, policy.GetOrder().GetCustomer(), now); !limited {
			customerPolicy := policy.GetOrder().GetCustomer()
			bypassMinArt := RaceHoldsUnfinishedCustomerOrder(s.FmlRace())
			for npcID, order := range s.CustomerOrderDetails() {
				if order == nil {
					continue
				}
				if !customerOrderMeetsMinFlowerArt(order, customerPolicy, bypassMinArt) {
					continue
				}
				entityID := strconv.FormatInt(int64(npcID), 10)
				label := fmt.Sprintf("顾客订单 NPC=%d", npcID)
				for _, req := range order.Requires {
					add(goal, "direct", DemandKindFlower, req.FlowerID, req.Count, entityID, label, nil)
				}
				for _, req := range order.ItemRequires {
					if req.ItemID <= 0 || req.Count <= 0 {
						continue
					}
					missingArt := req.Count - inventory[req.ItemID]
					if missingArt < 0 {
						missingArt = 0
					}
					recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
					if !ok {
						add(goal, "direct", DemandKindFlowerArt, req.ItemID, req.Count, entityID, label, []string{"缺少花艺配方"})
						continue
					}
					var blocked []string
					if missingArt > 0 {
						blocked = artBlockedReasons(s, recipe)
					}
					add(goal, "direct", DemandKindFlowerArt, req.ItemID, req.Count, entityID, label, blocked)
				}
			}
		}
	}
	if goal, ok := goalByID(goals, GoalPalaceOrder); ok {
		order := s.PalaceOrder()
		palace := policy.GetOrder().GetPalace()
		if palaceOrderAllowed(order, palace) {
			add(goal, "direct", DemandKindFlower, order.FlowerID, order.Num, "current", "宫廷订单", nil)
		}
	}
	if goal, ok := goalByID(goals, GoalTeamOrder); ok {
		order := s.TeamOrder()
		team := policy.GetOrder().GetTeam()
		if teamOrderAllowed(order, team, s) {
			add(goal, "direct", DemandKindFlower, order.FlowerID, teamOrderNeedCount(order), "current", "组团订单", nil)
		}
	}
	if goal, ok := goalByID(goals, GoalMainTask); ok {
		if task, taskOK := s.MainTask(); taskOK {
			if task.Valid && !task.Complete && task.ProgressObserved {
				if flowerID, missing, reqOK := state.MainTaskFlowerRequirement(task.TaskID, task.Finished); reqOK {
					add(goal, "direct", DemandKindFlower, flowerID, missing, strconv.FormatInt(int64(task.TaskID), 10), state.MainTaskTitle(task.TaskID), nil)
				}
			}
		}
	}
	sortDemands(out)
	return out
}

func buildProductionDemands(s *state.State, policy *pb.Policy, goals []Goal, direct []Demand, ledger *InventoryLedger) []Demand {
	var out []Demand
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		for _, demand := range direct {
			if demand.GoalID != goal.ID || demand.Kind != DemandKindFlowerArt || demand.Missing <= 0 || len(demand.BlockedReasons) > 0 {
				continue
			}
			out = appendCraftFlowerDemands(out, s, ledger, goal, demand.EntityID,
				"craft:"+strconv.FormatInt(int64(demand.ItemID), 10),
				fmt.Sprintf("%s 制作 %s", demand.Label, itemLabel(demand.ItemID)), demand.ItemID, demand.Missing)
		}
	}
	sortDemands(out)
	return out
}

func appendCraftFlowerDemands(out []Demand, s *state.State, ledger *InventoryLedger, goal Goal, entityID, source, label string, artID, craftCount int32) []Demand {
	if craftCount <= 0 {
		return out
	}
	recipe, ok := state.FlowerArtRecipeByID(artID)
	if !ok || len(artBlockedReasons(s, recipe)) > 0 {
		return out
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	for flowerID, count := range recipeFlowerCounts(recipe) {
		required := count * craftCount
		have := ledger.Owned(flowerID)
		available := ledger.Available(flowerID)
		missing := required - available
		if missing < 0 {
			missing = 0
		}
		out = append(out, Demand{
			ID:        demandID(goal.ID, entityID, source, DemandKindFlower, flowerID),
			GoalID:    goal.ID,
			Category:  goal.Category,
			Domain:    goal.Domain,
			EntityID:  entityID,
			Source:    source,
			Label:     label,
			Kind:      DemandKindFlower,
			ItemID:    flowerID,
			Count:     required,
			Have:      have,
			Available: available,
			Missing:   missing,
			Priority:  goal.Priority,
		})
	}
	return out
}

func applyLedgerAllocations(demands []Demand, ledger *InventoryLedger) {
	for i := range demands {
		demands[i].Have = ledger.Owned(demands[i].ItemID)
		demands[i].Allocated = ledger.Allocate(demands[i])
		demands[i].Available = ledger.Available(demands[i].ItemID)
		demands[i].Missing = demands[i].Count - demands[i].Allocated
		if demands[i].Missing < 0 {
			demands[i].Missing = 0
		}
	}
}

func annotateDemandStatuses(demands []Demand) {
	for i := range demands {
		d := &demands[i]
		if len(d.BlockedReasons) > 0 {
			d.Status = PlanStatusBlocked
			d.BlockingStage = inferBlockingStage(d.BlockedReasons)
			continue
		}
		if d.Missing > 0 {
			d.Status = PlanStatusManaged
			continue
		}
		d.Status = PlanStatusReady
	}
}

func sortDemands(demands []Demand) {
	sort.SliceStable(demands, func(i, j int) bool {
		if demands[i].Priority != demands[j].Priority {
			return demands[i].Priority > demands[j].Priority
		}
		if demands[i].Missing != demands[j].Missing {
			return demands[i].Missing > demands[j].Missing
		}
		return demands[i].ID < demands[j].ID
	})
}
