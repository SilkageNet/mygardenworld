package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func (r *Runner) ensurePlannedOperationRqst(ctx context.Context, op *automation.PlannedOp) error {
	if op == nil {
		return nil
	}
	if isHarvestOp(op.Kind) {
		return r.ensureHarvestRqst(ctx)
	}
	if op.Kind == clientproto.RPCUsrLandPlant.String() ||
		op.Kind == clientproto.RPCUsrLandPlantBatch.String() {
		return r.ensurePlantRqst(ctx)
	}
	if isWaterOp(op.Kind) || op.Kind == clientproto.RPCWaterwheelRecv.String() {
		// Mini client calls usrStatsCtrl.reportWater() (waterRqst.djst) before
		// every waterwheel bucket click, same point type as land watering.
		return r.ensureWaterRqst(ctx)
	}
	if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerRecvOrderRwd.String() {
		return r.ensureFlowerOrderRqst(ctx)
	}
	if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() ||
		op.Kind == clientproto.RPCOrderCustomerGenOrder.String() ||
		op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() ||
		op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
		return r.ensureCustomerOrderRqst(ctx)
	}
	return nil
}

func isFlowerNotMatureError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "鲜花尚未成熟")
}

func isPearlHireCandidateFallbackError(kind string, err error) bool {
	if kind != clientproto.RPCPearlPlaceHire.String() || err == nil {
		return false
	}
	var fallbackErr *pearlHireCandidateFallbackError
	return errors.As(err, &fallbackErr)
}

func isResidentOrderCooldownError(kind string, err error) bool {
	return (kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String()) &&
		err != nil && strings.Contains(err.Error(), "冷却中")
}

func isResidentOrderDailyLimitError(kind string, err error) bool {
	return (kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String()) &&
		err != nil && strings.Contains(err.Error(), "今日完成订单次数已达上限")
}

func isWaterwheelInvalidDataError(kind string, err error) bool {
	return kind == clientproto.RPCWaterwheelRecv.String() && err != nil && strings.Contains(err.Error(), "数据有误")
}

func isWaterwheelDailyLimitError(kind string, err error) bool {
	return kind == clientproto.RPCWaterwheelRecv.String() && err != nil && strings.Contains(err.Error(), "已达到领取上限")
}

func isShopCultivateOfferExhaustedError(kind string, err error) bool {
	if kind != clientproto.RPCShopCultivateBuy.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil && rpcErr.Envelope.ErrorCode() == 312 {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, `"code":312`) || strings.Contains(msg, `"code": 312`) ||
		strings.Contains(msg, "无法再购买当前商品")
}

func isWaterDropResourceRejectedError(kind string, err error) bool {
	if !isWaterOp(kind) || err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, `"code":301`) && strings.Contains(msg, `"iid":7`)
}

func isCultivateUpgradeResourceRejectedError(kind string, err error) bool {
	return kind == clientproto.RPCCultivateUpgrade.String() && resourceRejectedItemID(err) > 0
}

func isFmlBuildDailyLimitError(kind string, err error) bool {
	if kind != clientproto.RPCFmlBld.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil && rpcErr.Envelope.ErrorCode() == 383 {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "每日建设次数已达上限") ||
		strings.Contains(msg, `"code":383`) || strings.Contains(msg, `"code": 383`)
}

// inventoryMaterialRejectedItemID returns param.iid when an inventory-consuming
// RPC fails with a material-shortage envelope (code 301). Zero means the error
// is not that case. Covers flower-art craft and customer-order finish, where
// stale local stock can otherwise loop the same rejected finish.
func inventoryMaterialRejectedItemID(kind string, err error) int32 {
	switch kind {
	case clientproto.RPCFlowerArtMakeFlowerArt.String(),
		clientproto.RPCOrderCustomerFinishOrder.String():
	default:
		return 0
	}
	return resourceRejectedItemID(err)
}

func resourceRejectedItemID(err error) int32 {
	if err == nil {
		return 0
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if rpcErr.Envelope.ErrorCode() == 301 {
			return rpcErr.Envelope.MissingItemID()
		}
		return 0
	}
	msg := err.Error()
	if !strings.Contains(msg, `"code":301`) {
		return 0
	}
	const marker = `"iid":`
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return 0
	}
	rest := msg[idx+len(marker):]
	rest = strings.TrimLeft(rest, " \t\r\n")
	var n int32
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int32(c-'0')
		if n <= 0 {
			return 0
		}
	}
	return n
}

func flowerArtMaterialRejectedItemID(kind string, err error) int32 {
	return inventoryMaterialRejectedItemID(kind, err)
}

func isFlowerArtMaterialRejectedError(kind string, err error) bool {
	return inventoryMaterialRejectedItemID(kind, err) > 0
}

func isRaceTakeAlreadyTakenError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceTakeTask.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if !errors.As(err, &rpcErr) || rpcErr == nil {
		return false
	}
	return rpcErr.Envelope.ErrorCodeOfLangJS() == "fmlRace_tips1"
}

// isRaceTakeClaimedByOtherError matches takeTask when another guild member
// already holds the target taskMsId (stale local pool still showed UID==0).
func isRaceTakeClaimedByOtherError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceTakeTask.String() || err == nil {
		return false
	}
	const tip = "任务已被其他成员接取"
	if strings.Contains(err.Error(), tip) {
		return true
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if strings.Contains(rpcErr.Envelope.ErrorMsg(), tip) {
			return true
		}
	}
	return false
}

// isRaceTakeQuotaExceededError matches takeTask when the account has no
// remaining take slots for this race batch.
func isRaceTakeQuotaExceededError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceTakeTask.String() || err == nil {
		return false
	}
	const tip = "任务接取次数已达上限"
	if strings.Contains(err.Error(), tip) {
		return true
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if strings.Contains(rpcErr.Envelope.ErrorMsg(), tip) {
			return true
		}
	}
	return false
}

// isRaceTakeOnCooldownError matches takeTask when the pool row is still on
// AppearTime CD (common after a preemptive lead-window attempt).
const raceSyncRetryCooldown = 1 * time.Second

// raceTransientSessionCode is returned when the client race session is stale
// (common on getTaskList/takeTask before a fresh enter).
const raceTransientSessionCode = 221

func isRaceTransientSessionError(kind string, err error) bool {
	if err == nil || !isFmlRaceRPCKind(kind) {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil && rpcErr.Envelope.ErrorCode() == raceTransientSessionCode {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, `"code":221`) || strings.Contains(msg, `"code": 221`)
}

func isFmlRaceRPCKind(kind string) bool {
	return strings.HasPrefix(kind, "fmlRace.")
}

func isFmlNotJoinedError(kind string, err error) bool {
	if err == nil || (!strings.HasPrefix(kind, "fml.") &&
		!strings.HasPrefix(kind, "fmlRace.") &&
		!strings.HasPrefix(kind, "fmlLand.") &&
		!strings.HasPrefix(kind, "fmlFlowerShare.") &&
		!strings.HasPrefix(kind, "fmlForest.")) {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "未加入任何公会") || strings.Contains(msg, "未加入公会") {
		return true
	}
	// fml.enter returns bare code 109 for an account without a guild.
	if kind != clientproto.RPCFmlEnter.String() {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		return rpcErr.Envelope.ErrorCode() == 109
	}
	return strings.Contains(msg, `"code":109`) || strings.Contains(msg, `"code": 109`)
}

func isRaceTakeOnCooldownError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceTakeTask.String() || err == nil {
		return false
	}
	const tip = "任务冷却中"
	if strings.Contains(err.Error(), tip) {
		return true
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if strings.Contains(rpcErr.Envelope.ErrorMsg(), tip) {
			return true
		}
	}
	return false
}

func isRaceDeleteOnCooldownError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceDelTask.String() || err == nil {
		return false
	}
	const tip = "任务冷却中"
	if strings.Contains(err.Error(), tip) {
		return true
	}
	var rpcErr *babigame.RPCServerError
	return errors.As(err, &rpcErr) && rpcErr != nil && strings.Contains(rpcErr.Envelope.ErrorMsg(), tip)
}

// raceTakeOnCooldownWait returns how long to block take after a server CD tip.
// Prefer waiting until the pool row's AppearTime so we retry at refresh
// instead of burning a 60s ordinary side-op backoff.
func raceTakeOnCooldownWait(st *state.State, op *automation.PlannedOp, now time.Time) time.Duration {
	const (
		minWait  = 5 * time.Millisecond
		maxWait  = 2 * time.Minute
		fallback = 2 * time.Second
	)
	if st == nil || op == nil || op.TaskMsID == 0 {
		return fallback
	}
	for _, t := range st.FmlRace().Tasks {
		if t.MsId != op.TaskMsID || t.AppearTime <= 0 {
			continue
		}
		until := time.UnixMilli(t.AppearTime)
		if !until.After(now) {
			return minWait
		}
		d := until.Sub(now)
		if d > maxWait {
			return maxWait
		}
		if d < minWait {
			return minWait
		}
		return d
	}
	return fallback
}

// isCyclicStoryOrderNotReadyError matches actCyclicStory.recvOrderRwd code 259
// ("未达成领取奖励的条件!") — typically refreshCd / validTime not yet elapsed.
func isCyclicStoryOrderNotReadyError(kind string, err error) bool {
	if kind != clientproto.RPCActCyclicStoryRecvOrderRwd.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil && rpcErr.Envelope.ErrorCode() == 259 {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, `"code":259`) || strings.Contains(msg, `"code": 259`)
}

func cyclicStoryOrderCooldownUntil(st *state.State, op *automation.PlannedOp, now time.Time) time.Time {
	if st == nil || op == nil {
		return time.Time{}
	}
	view, ok := st.CyclicStoryView(now)
	if !ok {
		return time.Time{}
	}
	for _, order := range view.Orders {
		if order.OrderIdx != op.SlotID {
			continue
		}
		return state.CyclicStoryValidUntil(order.ValidTime)
	}
	return time.Time{}
}

func isFmlFlowerTakeDailyLimitError(kind string, err error) bool {
	if kind != clientproto.RPCFmlFlowerShareTake.String() || err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "今日拿取次数已达上限") || strings.Contains(msg, "fmlShare_tips8") {
		return true
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if rpcErr.Envelope.ErrorCodeOfLangJS() == "fmlShare_tips8" {
			return true
		}
		if strings.Contains(rpcErr.Envelope.ErrorMsg(), "今日拿取次数已达上限") {
			return true
		}
	}
	return false
}

func isTaskGroupFinishedError(kind string, err error) bool {
	if err == nil {
		return false
	}
	switch kind {
	case clientproto.RPCTaskDlyRecv.String(), clientproto.RPCTaskWeekRecv.String(), clientproto.RPCTaskAchRecv.String():
		return strings.Contains(err.Error(), "本组任务已经完结")
	default:
		return false
	}
}

func isMailAlreadyPickedError(kind string, err error) bool {
	if kind != clientproto.RPCMailPick.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		code := rpcErr.Envelope.ErrorCodeOfLangJS()
		if code == "mail_nonToPick" || code == "mail_alreadyPick" {
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "附件已领取") || strings.Contains(msg, "不存在可以领取的邮件") || strings.Contains(msg, "mail_nonToPick") || strings.Contains(msg, "mail_alreadyPick")
}

func waterResponseIncludesDrops(raw json.RawMessage) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return false
	}
	raw7, ok := top["7"]
	if !ok {
		return false
	}
	var ns7 map[string]json.RawMessage
	if err := json.Unmarshal(raw7, &ns7); err != nil {
		return false
	}
	if raw0, ok := ns7["0"]; ok && nestedMapHasItem(raw0, "32", "7") {
		return true
	}
	raw2, ok := ns7["2"]
	if !ok {
		return false
	}
	return nestedMapHasItem(raw2, "0", "7") || nestedMapHasItem(raw2, "2", "7")
}

func nestedMapHasItem(raw json.RawMessage, field, itemID string) bool {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return false
	}
	innerRaw, ok := outer[field]
	if !ok {
		return false
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(innerRaw, &inner); err != nil {
		return false
	}
	_, ok = inner[itemID]
	return ok
}
