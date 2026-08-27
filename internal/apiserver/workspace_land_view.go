package apiserver

import (
	"fmt"
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func buildFmlLandViews(lands map[int32]state.FmlLandView, cultivations map[int32]state.CultivateView, now time.Time) []*pb.FmlLandView {
	if len(lands) == 0 {
		return nil
	}
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.FmlLandView, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmlLandViewProto(lands[id], cultivations, now))
	}
	return out
}

func fmlLandViewProto(land state.FmlLandView, cultivations map[int32]state.CultivateView, now time.Time) *pb.FmlLandView {
	pending := state.FmlLandPendingHarvest(land, now)
	kind, reason := "wait", "成长中"
	switch {
	case pending > 0:
		kind, reason = "harvest", fmt.Sprintf("可收获 %d 朵", pending)
	case land.FlowerID <= 0:
		kind, reason = "plant", "空地可种植"
	}
	var stockCap, timeSec int32
	if cfg, ok := state.FmlLandLvlByID(land.Level); ok {
		stockCap = cfg.Stock
		timeSec = cfg.TimeSec
	}
	var flowerLvl int32
	if land.FlowerID > 0 {
		if cv, ok := cultivations[land.FlowerID]; ok {
			flowerLvl = cv.Lvl
		}
	}
	return &pb.FmlLandView{
		LandId:            land.LandID,
		Level:             land.Level,
		FlowerId:          land.FlowerID,
		StartTimeMs:       land.StartTimeMs,
		MatureFlowerCount: land.MatureFlowerCnt,
		HarvestedCount:    land.HarvestedCnt,
		LastCalcTimeMs:    land.LastCalcTimeMs,
		PendingHarvest:    pending,
		StockCap:          stockCap,
		TimeSec:           timeSec,
		NextMatureMs:      state.FmlLandNextMatureMs(land, now),
		Recommendation:    kind,
		Reason:            reason,
		FlowerLvl:         flowerLvl,
	}
}

func buildLandViews(lands map[int32]state.LandView, farmLands []state.FarmLandInfo, rosterObserved bool, farmLandObserved bool, level int32, now time.Time, harvestDelay time.Duration) []*pb.LandView {
	out := make([]*pb.LandView, 0, len(lands))
	seen := make(map[int32]struct{}, len(lands))
	unopenedCount := 0
	for _, info := range farmLands {
		l, observed := lands[info.ID]
		isUnopened := !observed && rosterObserved && farmLandObserved
		if isUnopened {
			unopenedCount++
		}
		out = append(out, landViewProtoWithLimit(info.ID, l, info, observed, rosterObserved, farmLandObserved, level, now, unopenedCount, harvestDelay))
		seen[info.ID] = struct{}{}
	}
	extraIDs := make([]int32, 0)
	for id := range lands {
		if _, ok := seen[id]; ok {
			continue
		}
		extraIDs = append(extraIDs, id)
	}
	sort.Slice(extraIDs, func(i, j int) bool { return extraIDs[i] < extraIDs[j] })
	for _, id := range extraIDs {
		out = append(out, landViewProtoWithLimit(id, lands[id], state.FarmLandInfo{}, true, rosterObserved, farmLandObserved, level, now, 0, harvestDelay))
	}
	return out
}

const maxReclaimableLands = 6

func landViewProtoWithLimit(id int32, l state.LandView, info state.FarmLandInfo, observed bool, rosterObserved bool, farmLandObserved bool, level int32, now time.Time, unopenedIdx int, harvestDelay time.Duration) *pb.LandView {
	kind, reason := "unknown", "等待服务端土地清单"
	status := "locked"
	switch {
	case observed:
		kind, reason = automation.Recommend(l, now, harvestDelay)
		status = "opened"
	case !farmLandObserved:
		kind, reason = "unknown", "等待当前客户端土地配置"
	case rosterObserved:
		if unopenedIdx > 0 && unopenedIdx <= maxReclaimableLands {
			status = "unopened"
			if len(info.Cost) >= 2 {
				kind, reason = "unlock", "可开垦"
			} else {
				kind, reason = "unknown", "缺少开垦消耗配置"
			}
		} else {
			status = "locked"
			kind, reason = "locked", "未解锁"
		}
	}
	if observed && !l.Observed {
		observed = false
	}
	return &pb.LandView{
		LandId:         id,
		FlowerId:       int32(l.FlowerID),
		State:          int32(l.State),
		Lvl:            int32(l.Lvl),
		HarvestCnt:     int32(l.HarvestCnt),
		NextTimeMs:     l.NextTimeMs,
		PlantTimeMs:    l.PlantTimeMs,
		Recommendation: kind,
		Reason:         reason,
		LandStatus:     status,
		Observed:       observed,
		OpenLevel:      info.OpenLevel,
		UnlockCost:     farmLandActualCost(info.Cost),
		Wasteland:      append([]int32(nil), info.Wasteland...),
	}
}

func farmLandActualCost(cost []int32) []int32 {
	if len(cost) < 2 {
		return append([]int32(nil), cost...)
	}
	actualGold := cost[1] - cost[0] + 11
	return []int32{actualGold}
}
