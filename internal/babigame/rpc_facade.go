package babigame

import "context"

// Code in this file is based on statically observed gs.* calls in tmp/mini
// game.js. Long-tail request objects come from static extraction; core request
// structs are intentionally tightened to the concrete types used by the
// protocol and runner. Do not blindly regenerate this file back to all-any
// fields.
//
// Each method keeps the request object explicit and decodes responses as
// StateDelta.

// Act returns typed RPC helpers for the act namespace.
func (c *RPCClient) Act() ActRPC { return ActRPC{c: c} }

type ActRPC struct{ c *RPCClient }

// ActBuyRequest is the request body for gs.act.buy.
type ActBuyRequest struct {
	BatchId    RPCID  `json:"batchId,omitempty"`
	ShopIdx    RPCInt `json:"shopIdx,omitempty"`
	ShopItemId RPCID  `json:"shopItemId,omitempty"`
	Count      RPCInt `json:"count,omitempty"`
}

// ActBuyResponse is the namespace-delta response for gs.act.buy.
type ActBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.act.buy. Request fields inferred from game.js: batchId, shopIdx, shopItemId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) Buy(ctx context.Context, req ActBuyRequest, opts ...RequestOption) (ActBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActBuy, req, opts...)
}

// ActGetOneOrderAwardRequest is the request body for gs.act.getOneOrderAward.
type ActGetOneOrderAwardRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	ID      RPCID  `json:"id,omitempty"`
	Lvl     RPCInt `json:"lvl,omitempty"`
}

// ActGetOneOrderAwardResponse is the namespace-delta response for gs.act.getOneOrderAward.
type ActGetOneOrderAwardResponse = RPCResponse[StateDelta]

// GetOneOrderAward calls gs.act.getOneOrderAward. Request fields inferred from game.js: batchId, id, lvl.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) GetOneOrderAward(ctx context.Context, req ActGetOneOrderAwardRequest, opts ...RequestOption) (ActGetOneOrderAwardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGetOneOrderAward, req, opts...)
}

// ActGetOrderAwardRequest is the request body for gs.act.getOrderAward.
type ActGetOrderAwardRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActGetOrderAwardResponse is the namespace-delta response for gs.act.getOrderAward.
type ActGetOrderAwardResponse = RPCResponse[StateDelta]

// GetOrderAward calls gs.act.getOrderAward. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) GetOrderAward(ctx context.Context, req ActGetOrderAwardRequest, opts ...RequestOption) (ActGetOrderAwardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGetOrderAward, req, opts...)
}

// ActGetRankAwardRequest carries JSON fields for gs.act.getRankAward; game.js did not expose a stable object literal for this request.
type ActGetRankAwardRequest RawRequest

// ActGetRankAwardResponse is the namespace-delta response for gs.act.getRankAward.
type ActGetRankAwardResponse = RPCResponse[StateDelta]

// GetRankAward calls gs.act.getRankAward. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) GetRankAward(ctx context.Context, req ActGetRankAwardRequest, opts ...RequestOption) (ActGetRankAwardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGetRankAward, req, opts...)
}

// ActGetStatRequest is the request body for gs.act.getStat.
type ActGetStatRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActGetStatResponse is the namespace-delta response for gs.act.getStat.
type ActGetStatResponse = RPCResponse[StateDelta]

// GetStat calls gs.act.getStat. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) GetStat(ctx context.Context, req ActGetStatRequest, opts ...RequestOption) (ActGetStatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGetStat, req, opts...)
}

// ActGiftBuyRequest is the request body for gs.act.giftBuy.
type ActGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActGiftBuyResponse is the namespace-delta response for gs.act.giftBuy.
type ActGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.act.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) GiftBuy(ctx context.Context, req ActGiftBuyRequest, opts ...RequestOption) (ActGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGiftBuy, req, opts...)
}

// ActRecvRequest is the request body for gs.act.recv.
type ActRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	TaskIdx RPCInt `json:"taskIdx,omitempty"`
	TaskId  RPCID  `json:"taskId,omitempty"`
}

// ActRecvResponse is the namespace-delta response for gs.act.recv.
type ActRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.act.recv. Request fields inferred from game.js: batchId, taskIdx, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) Recv(ctx context.Context, req ActRecvRequest, opts ...RequestOption) (ActRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRecv, req, opts...)
}

// ActRecvBoxesRequest is the request body for gs.act.recvBoxes.
type ActRecvBoxesRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	TaskId  RPCID  `json:"taskId,omitempty"`
	WayType RPCInt `json:"wayType,omitempty"`
}

// ActRecvBoxesResponse is the namespace-delta response for gs.act.recvBoxes.
type ActRecvBoxesResponse = RPCResponse[StateDelta]

// RecvBoxes calls gs.act.recvBoxes. Request fields inferred from game.js: batchId, taskId, wayType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) RecvBoxes(ctx context.Context, req ActRecvBoxesRequest, opts ...RequestOption) (ActRecvBoxesResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRecvBoxes, req, opts...)
}

// ActRecvTLAwardRequest is the request body for gs.act.recvTLAward.
type ActRecvTLAwardRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActRecvTLAwardResponse is the namespace-delta response for gs.act.recvTLAward.
type ActRecvTLAwardResponse = RPCResponse[StateDelta]

// RecvTLAward calls gs.act.recvTLAward. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) RecvTLAward(ctx context.Context, req ActRecvTLAwardRequest, opts ...RequestOption) (ActRecvTLAwardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRecvTLAward, req, opts...)
}

// ActRefreshDailyGiftRequest is the request body for gs.act.refreshDailyGift.
type ActRefreshDailyGiftRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActRefreshDailyGiftResponse is the namespace-delta response for gs.act.refreshDailyGift.
type ActRefreshDailyGiftResponse = RPCResponse[StateDelta]

// RefreshDailyGift calls gs.act.refreshDailyGift. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) RefreshDailyGift(ctx context.Context, req ActRefreshDailyGiftRequest, opts ...RequestOption) (ActRefreshDailyGiftResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRefreshDailyGift, req, opts...)
}

// ActRefreshTaskRequest is the request body for gs.act.refreshTask.
type ActRefreshTaskRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActRefreshTaskResponse is the namespace-delta response for gs.act.refreshTask.
type ActRefreshTaskResponse = RPCResponse[StateDelta]

// RefreshTask calls gs.act.refreshTask. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) RefreshTask(ctx context.Context, req ActRefreshTaskRequest, opts ...RequestOption) (ActRefreshTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRefreshTask, req, opts...)
}

// ActSyncBatchInfoRequest is the request body for gs.act.syncBatchInfo.
type ActSyncBatchInfoRequest struct {
	BatchIdList RPCIDList `json:"batchIdList,omitempty"`
}

// ActSyncBatchInfoResponse is the namespace-delta response for gs.act.syncBatchInfo.
type ActSyncBatchInfoResponse = RPCResponse[StateDelta]

// SyncBatchInfo calls gs.act.syncBatchInfo. Request fields inferred from game.js: batchIdList.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRPC) SyncBatchInfo(ctx context.Context, req ActSyncBatchInfoRequest, opts ...RequestOption) (ActSyncBatchInfoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSyncBatchInfo, req, opts...)
}

// ActCallBack returns typed RPC helpers for the actCallBack namespace.
func (c *RPCClient) ActCallBack() ActCallBackRPC { return ActCallBackRPC{c: c} }

type ActCallBackRPC struct{ c *RPCClient }

// ActCallBackActCallBackBindRequest carries JSON fields for gs.actCallBack.actCallBackBind; game.js did not expose a stable object literal for this request.
type ActCallBackActCallBackBindRequest RawRequest

// ActCallBackActCallBackBindResponse is the namespace-delta response for gs.actCallBack.actCallBackBind.
type ActCallBackActCallBackBindResponse = RPCResponse[StateDelta]

// ActCallBackBind calls gs.actCallBack.actCallBackBind. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCallBackRPC) ActCallBackBind(ctx context.Context, req ActCallBackActCallBackBindRequest, opts ...RequestOption) (ActCallBackActCallBackBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCallBackActCallBackBind, req, opts...)
}

// ActCallBackActCallBackEnterRequest carries JSON fields for gs.actCallBack.actCallBackEnter; game.js did not expose a stable object literal for this request.
type ActCallBackActCallBackEnterRequest RawRequest

// ActCallBackActCallBackEnterResponse is the namespace-delta response for gs.actCallBack.actCallBackEnter.
type ActCallBackActCallBackEnterResponse = RPCResponse[StateDelta]

// ActCallBackEnter calls gs.actCallBack.actCallBackEnter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCallBackRPC) ActCallBackEnter(ctx context.Context, req ActCallBackActCallBackEnterRequest, opts ...RequestOption) (ActCallBackActCallBackEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCallBackActCallBackEnter, req, opts...)
}

// ActCallBackActCallBackRecvRequest carries JSON fields for gs.actCallBack.actCallBackRecv; game.js did not expose a stable object literal for this request.
type ActCallBackActCallBackRecvRequest RawRequest

// ActCallBackActCallBackRecvResponse is the namespace-delta response for gs.actCallBack.actCallBackRecv.
type ActCallBackActCallBackRecvResponse = RPCResponse[StateDelta]

// ActCallBackRecv calls gs.actCallBack.actCallBackRecv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCallBackRPC) ActCallBackRecv(ctx context.Context, req ActCallBackActCallBackRecvRequest, opts ...RequestOption) (ActCallBackActCallBackRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCallBackActCallBackRecv, req, opts...)
}

// ActCardCollect returns typed RPC helpers for the actCardCollect namespace.
func (c *RPCClient) ActCardCollect() ActCardCollectRPC { return ActCardCollectRPC{c: c} }

type ActCardCollectRPC struct{ c *RPCClient }

// ActCardCollectCheckLuckyCardSendRequest is the request body for gs.actCardCollect.checkLuckyCardSend.
type ActCardCollectCheckLuckyCardSendRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActCardCollectCheckLuckyCardSendResponse is the namespace-delta response for gs.actCardCollect.checkLuckyCardSend.
type ActCardCollectCheckLuckyCardSendResponse = RPCResponse[StateDelta]

// CheckLuckyCardSend calls gs.actCardCollect.checkLuckyCardSend. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) CheckLuckyCardSend(ctx context.Context, req ActCardCollectCheckLuckyCardSendRequest, opts ...RequestOption) (ActCardCollectCheckLuckyCardSendResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectCheckLuckyCardSend, req, opts...)
}

// ActCardCollectDeckShopExchangeRequest is the request body for gs.actCardCollect.deckShopExchange.
type ActCardCollectDeckShopExchangeRequest struct {
	BatchId  RPCID  `json:"batchId,omitempty"`
	CostStar RPCInt `json:"costStar,omitempty"`
}

// ActCardCollectDeckShopExchangeResponse is the namespace-delta response for gs.actCardCollect.deckShopExchange.
type ActCardCollectDeckShopExchangeResponse = RPCResponse[StateDelta]

// DeckShopExchange calls gs.actCardCollect.deckShopExchange. Request fields inferred from game.js: batchId, costStar.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) DeckShopExchange(ctx context.Context, req ActCardCollectDeckShopExchangeRequest, opts ...RequestOption) (ActCardCollectDeckShopExchangeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectDeckShopExchange, req, opts...)
}

// ActCardCollectNextRoundRequest is the request body for gs.actCardCollect.nextRound.
type ActCardCollectNextRoundRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActCardCollectNextRoundResponse is the namespace-delta response for gs.actCardCollect.nextRound.
type ActCardCollectNextRoundResponse = RPCResponse[StateDelta]

// NextRound calls gs.actCardCollect.nextRound. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) NextRound(ctx context.Context, req ActCardCollectNextRoundRequest, opts ...RequestOption) (ActCardCollectNextRoundResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectNextRound, req, opts...)
}

// ActCardCollectOpenCardPackRequest is the request body for gs.actCardCollect.openCardPack.
type ActCardCollectOpenCardPackRequest struct {
	CardPackId RPCID  `json:"cardPackId,omitempty"`
	Num        RPCInt `json:"num,omitempty"`
}

// ActCardCollectOpenCardPackResponse is the namespace-delta response for gs.actCardCollect.openCardPack.
type ActCardCollectOpenCardPackResponse = RPCResponse[StateDelta]

// OpenCardPack calls gs.actCardCollect.openCardPack. Request fields inferred from game.js: cardPackId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) OpenCardPack(ctx context.Context, req ActCardCollectOpenCardPackRequest, opts ...RequestOption) (ActCardCollectOpenCardPackResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectOpenCardPack, req, opts...)
}

// ActCardCollectRecvBoxRewardRequest is the request body for gs.actCardCollect.recvBoxReward.
type ActCardCollectRecvBoxRewardRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	BoxId   RPCID `json:"boxId,omitempty"`
}

// ActCardCollectRecvBoxRewardResponse is the namespace-delta response for gs.actCardCollect.recvBoxReward.
type ActCardCollectRecvBoxRewardResponse = RPCResponse[StateDelta]

// RecvBoxReward calls gs.actCardCollect.recvBoxReward. Request fields inferred from game.js: batchId, boxId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) RecvBoxReward(ctx context.Context, req ActCardCollectRecvBoxRewardRequest, opts ...RequestOption) (ActCardCollectRecvBoxRewardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectRecvBoxReward, req, opts...)
}

// ActCardCollectRecvCardAlbumRewardRequest is the request body for gs.actCardCollect.recvCardAlbumReward.
type ActCardCollectRecvCardAlbumRewardRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActCardCollectRecvCardAlbumRewardResponse is the namespace-delta response for gs.actCardCollect.recvCardAlbumReward.
type ActCardCollectRecvCardAlbumRewardResponse = RPCResponse[StateDelta]

// RecvCardAlbumReward calls gs.actCardCollect.recvCardAlbumReward. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) RecvCardAlbumReward(ctx context.Context, req ActCardCollectRecvCardAlbumRewardRequest, opts ...RequestOption) (ActCardCollectRecvCardAlbumRewardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectRecvCardAlbumReward, req, opts...)
}

// ActCardCollectRecvCollectRewardRequest is the request body for gs.actCardCollect.recvCollectReward.
type ActCardCollectRecvCollectRewardRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActCardCollectRecvCollectRewardResponse is the namespace-delta response for gs.actCardCollect.recvCollectReward.
type ActCardCollectRecvCollectRewardResponse = RPCResponse[StateDelta]

// RecvCollectReward calls gs.actCardCollect.recvCollectReward. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) RecvCollectReward(ctx context.Context, req ActCardCollectRecvCollectRewardRequest, opts ...RequestOption) (ActCardCollectRecvCollectRewardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectRecvCollectReward, req, opts...)
}

// ActCardCollectRecvTaskRewardRequest is the request body for gs.actCardCollect.recvTaskReward.
type ActCardCollectRecvTaskRewardRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	TaskIdx RPCInt `json:"taskIdx,omitempty"`
	TaskId  RPCID  `json:"taskId,omitempty"`
}

// ActCardCollectRecvTaskRewardResponse is the namespace-delta response for gs.actCardCollect.recvTaskReward.
type ActCardCollectRecvTaskRewardResponse = RPCResponse[StateDelta]

// RecvTaskReward calls gs.actCardCollect.recvTaskReward. Request fields inferred from game.js: batchId, taskIdx, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) RecvTaskReward(ctx context.Context, req ActCardCollectRecvTaskRewardRequest, opts ...RequestOption) (ActCardCollectRecvTaskRewardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectRecvTaskReward, req, opts...)
}

// ActCardCollectRefreshTaskDataRequest is the request body for gs.actCardCollect.refreshTaskData.
type ActCardCollectRefreshTaskDataRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActCardCollectRefreshTaskDataResponse is the namespace-delta response for gs.actCardCollect.refreshTaskData.
type ActCardCollectRefreshTaskDataResponse = RPCResponse[StateDelta]

// RefreshTaskData calls gs.actCardCollect.refreshTaskData. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) RefreshTaskData(ctx context.Context, req ActCardCollectRefreshTaskDataRequest, opts ...RequestOption) (ActCardCollectRefreshTaskDataResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectRefreshTaskData, req, opts...)
}

// ActCardCollectUseSelectedCardRequest is the request body for gs.actCardCollect.useSelectedCard.
type ActCardCollectUseSelectedCardRequest struct {
	CardId RPCID `json:"cardId,omitempty"`
}

// ActCardCollectUseSelectedCardResponse is the namespace-delta response for gs.actCardCollect.useSelectedCard.
type ActCardCollectUseSelectedCardResponse = RPCResponse[StateDelta]

// UseSelectedCard calls gs.actCardCollect.useSelectedCard. Request fields inferred from game.js: cardId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCardCollectRPC) UseSelectedCard(ctx context.Context, req ActCardCollectUseSelectedCardRequest, opts ...RequestOption) (ActCardCollectUseSelectedCardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCardCollectUseSelectedCard, req, opts...)
}

// ActCyclicNote returns typed RPC helpers for the actCyclicNote namespace.
func (c *RPCClient) ActCyclicNote() ActCyclicNoteRPC { return ActCyclicNoteRPC{c: c} }

type ActCyclicNoteRPC struct{ c *RPCClient }

// ActCyclicNoteDirectRecvTaskRwdRequest is the request body for gs.actCyclicNote.directRecvTaskRwd.
type ActCyclicNoteDirectRecvTaskRwdRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActCyclicNoteDirectRecvTaskRwdResponse is the namespace-delta response for gs.actCyclicNote.directRecvTaskRwd.
type ActCyclicNoteDirectRecvTaskRwdResponse = RPCResponse[StateDelta]

// DirectRecvTaskRwd calls gs.actCyclicNote.directRecvTaskRwd. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) DirectRecvTaskRwd(ctx context.Context, req ActCyclicNoteDirectRecvTaskRwdRequest, opts ...RequestOption) (ActCyclicNoteDirectRecvTaskRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteDirectRecvTaskRwd, req, opts...)
}

// ActCyclicNoteGiftBuyRequest is the request body for gs.actCyclicNote.giftBuy.
type ActCyclicNoteGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActCyclicNoteGiftBuyResponse is the namespace-delta response for gs.actCyclicNote.giftBuy.
type ActCyclicNoteGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actCyclicNote.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) GiftBuy(ctx context.Context, req ActCyclicNoteGiftBuyRequest, opts ...RequestOption) (ActCyclicNoteGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteGiftBuy, req, opts...)
}

// ActCyclicNoteRecvRequest is the request body for gs.actCyclicNote.recv.
type ActCyclicNoteRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActCyclicNoteRecvResponse is the namespace-delta response for gs.actCyclicNote.recv.
type ActCyclicNoteRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actCyclicNote.recv. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) Recv(ctx context.Context, req ActCyclicNoteRecvRequest, opts ...RequestOption) (ActCyclicNoteRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteRecv, req, opts...)
}

// ActCyclicNoteRecvTaskRwdRequest is the request body for gs.actCyclicNote.recvTaskRwd.
type ActCyclicNoteRecvTaskRwdRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActCyclicNoteRecvTaskRwdResponse is the namespace-delta response for gs.actCyclicNote.recvTaskRwd.
type ActCyclicNoteRecvTaskRwdResponse = RPCResponse[StateDelta]

// RecvTaskRwd calls gs.actCyclicNote.recvTaskRwd. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) RecvTaskRwd(ctx context.Context, req ActCyclicNoteRecvTaskRwdRequest, opts ...RequestOption) (ActCyclicNoteRecvTaskRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteRecvTaskRwd, req, opts...)
}

// ActCyclicNoteReRandomTaskRequest is the request body for gs.actCyclicNote.reRandomTask.
type ActCyclicNoteReRandomTaskRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActCyclicNoteReRandomTaskResponse is the namespace-delta response for gs.actCyclicNote.reRandomTask.
type ActCyclicNoteReRandomTaskResponse = RPCResponse[StateDelta]

// ReRandomTask calls gs.actCyclicNote.reRandomTask. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) ReRandomTask(ctx context.Context, req ActCyclicNoteReRandomTaskRequest, opts ...RequestOption) (ActCyclicNoteReRandomTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteReRandomTask, req, opts...)
}

// ActCyclicNoteResetGiftCdRequest is the request body for gs.actCyclicNote.resetGiftCd.
type ActCyclicNoteResetGiftCdRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GiftId  RPCID `json:"giftId,omitempty"`
}

// ActCyclicNoteResetGiftCdResponse is the namespace-delta response for gs.actCyclicNote.resetGiftCd.
type ActCyclicNoteResetGiftCdResponse = RPCResponse[StateDelta]

// ResetGiftCd calls gs.actCyclicNote.resetGiftCd. Request fields inferred from game.js: batchId, giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) ResetGiftCd(ctx context.Context, req ActCyclicNoteResetGiftCdRequest, opts ...RequestOption) (ActCyclicNoteResetGiftCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteResetGiftCd, req, opts...)
}

// ActCyclicNoteUnlockTaskSlotRequest is the request body for gs.actCyclicNote.unlockTaskSlot.
type ActCyclicNoteUnlockTaskSlotRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	SlotId  RPCID `json:"slotId,omitempty"`
}

// ActCyclicNoteUnlockTaskSlotResponse is the namespace-delta response for gs.actCyclicNote.unlockTaskSlot.
type ActCyclicNoteUnlockTaskSlotResponse = RPCResponse[StateDelta]

// UnlockTaskSlot calls gs.actCyclicNote.unlockTaskSlot. Request fields inferred from game.js: batchId, slotId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicNoteRPC) UnlockTaskSlot(ctx context.Context, req ActCyclicNoteUnlockTaskSlotRequest, opts ...RequestOption) (ActCyclicNoteUnlockTaskSlotResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicNoteUnlockTaskSlot, req, opts...)
}

// ActCyclicStory returns typed RPC helpers for the actCyclicStory namespace.
func (c *RPCClient) ActCyclicStory() ActCyclicStoryRPC { return ActCyclicStoryRPC{c: c} }

type ActCyclicStoryRPC struct{ c *RPCClient }

// ActCyclicStoryGiftBuyRequest is the request body for gs.actCyclicStory.giftBuy.
type ActCyclicStoryGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActCyclicStoryGiftBuyResponse is the namespace-delta response for gs.actCyclicStory.giftBuy.
type ActCyclicStoryGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actCyclicStory.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicStoryRPC) GiftBuy(ctx context.Context, req ActCyclicStoryGiftBuyRequest, opts ...RequestOption) (ActCyclicStoryGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicStoryGiftBuy, req, opts...)
}

// ActCyclicStoryRecvRequest is the request body for gs.actCyclicStory.recv.
type ActCyclicStoryRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActCyclicStoryRecvResponse is the namespace-delta response for gs.actCyclicStory.recv.
type ActCyclicStoryRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actCyclicStory.recv. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicStoryRPC) Recv(ctx context.Context, req ActCyclicStoryRecvRequest, opts ...RequestOption) (ActCyclicStoryRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicStoryRecv, req, opts...)
}

// ActCyclicStoryRecvOrderRwdRequest is the request body for gs.actCyclicStory.recvOrderRwd.
type ActCyclicStoryRecvOrderRwdRequest struct {
	BatchId  RPCID `json:"batchId,omitempty"`
	OrderIdx RPCID `json:"orderIdx,omitempty"`
}

// ActCyclicStoryRecvOrderRwdResponse is the namespace-delta response for gs.actCyclicStory.recvOrderRwd.
type ActCyclicStoryRecvOrderRwdResponse = RPCResponse[StateDelta]

// RecvOrderRwd calls gs.actCyclicStory.recvOrderRwd. Request fields inferred from game.js: batchId, orderIdx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicStoryRPC) RecvOrderRwd(ctx context.Context, req ActCyclicStoryRecvOrderRwdRequest, opts ...RequestOption) (ActCyclicStoryRecvOrderRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicStoryRecvOrderRwd, req, opts...)
}

// ActCyclicStoryRemoveOrderCdRequest is the request body for gs.actCyclicStory.removeOrderCd.
type ActCyclicStoryRemoveOrderCdRequest struct {
	BatchId  RPCID `json:"batchId,omitempty"`
	OrderIdx RPCID `json:"orderIdx,omitempty"`
}

// ActCyclicStoryRemoveOrderCdResponse is the namespace-delta response for gs.actCyclicStory.removeOrderCd.
type ActCyclicStoryRemoveOrderCdResponse = RPCResponse[StateDelta]

// RemoveOrderCd calls gs.actCyclicStory.removeOrderCd. Request fields inferred from game.js: batchId, orderIdx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicStoryRPC) RemoveOrderCd(ctx context.Context, req ActCyclicStoryRemoveOrderCdRequest, opts ...RequestOption) (ActCyclicStoryRemoveOrderCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicStoryRemoveOrderCd, req, opts...)
}

// ActCyclicStoryReRandomOrderRequest is the request body for gs.actCyclicStory.reRandomOrder.
type ActCyclicStoryReRandomOrderRequest struct {
	BatchId  RPCID `json:"batchId,omitempty"`
	OrderIdx RPCID `json:"orderIdx,omitempty"`
}

// ActCyclicStoryReRandomOrderResponse is the namespace-delta response for gs.actCyclicStory.reRandomOrder.
type ActCyclicStoryReRandomOrderResponse = RPCResponse[StateDelta]

// ReRandomOrder calls gs.actCyclicStory.reRandomOrder. Request fields inferred from game.js: batchId, orderIdx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicStoryRPC) ReRandomOrder(ctx context.Context, req ActCyclicStoryReRandomOrderRequest, opts ...RequestOption) (ActCyclicStoryReRandomOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicStoryReRandomOrder, req, opts...)
}

// ActCyclicStoryResetGiftCdRequest is the request body for gs.actCyclicStory.resetGiftCd.
type ActCyclicStoryResetGiftCdRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GiftId  RPCID `json:"giftId,omitempty"`
}

// ActCyclicStoryResetGiftCdResponse is the namespace-delta response for gs.actCyclicStory.resetGiftCd.
type ActCyclicStoryResetGiftCdResponse = RPCResponse[StateDelta]

// ResetGiftCd calls gs.actCyclicStory.resetGiftCd. Request fields inferred from game.js: batchId, giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicStoryRPC) ResetGiftCd(ctx context.Context, req ActCyclicStoryResetGiftCdRequest, opts ...RequestOption) (ActCyclicStoryResetGiftCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicStoryResetGiftCd, req, opts...)
}

// ActCyclicVase returns typed RPC helpers for the actCyclicVase namespace.
func (c *RPCClient) ActCyclicVase() ActCyclicVaseRPC { return ActCyclicVaseRPC{c: c} }

type ActCyclicVaseRPC struct{ c *RPCClient }

// ActCyclicVaseGiftBuyRequest is the request body for gs.actCyclicVase.giftBuy.
type ActCyclicVaseGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActCyclicVaseGiftBuyResponse is the namespace-delta response for gs.actCyclicVase.giftBuy.
type ActCyclicVaseGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actCyclicVase.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicVaseRPC) GiftBuy(ctx context.Context, req ActCyclicVaseGiftBuyRequest, opts ...RequestOption) (ActCyclicVaseGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicVaseGiftBuy, req, opts...)
}

// ActCyclicVaseRecvRequest is the request body for gs.actCyclicVase.recv.
type ActCyclicVaseRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActCyclicVaseRecvResponse is the namespace-delta response for gs.actCyclicVase.recv.
type ActCyclicVaseRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actCyclicVase.recv. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicVaseRPC) Recv(ctx context.Context, req ActCyclicVaseRecvRequest, opts ...RequestOption) (ActCyclicVaseRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicVaseRecv, req, opts...)
}

// ActCyclicVaseResetGiftCdRequest is the request body for gs.actCyclicVase.resetGiftCd.
type ActCyclicVaseResetGiftCdRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GiftId  RPCID `json:"giftId,omitempty"`
}

// ActCyclicVaseResetGiftCdResponse is the namespace-delta response for gs.actCyclicVase.resetGiftCd.
type ActCyclicVaseResetGiftCdResponse = RPCResponse[StateDelta]

// ResetGiftCd calls gs.actCyclicVase.resetGiftCd. Request fields inferred from game.js: batchId, giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActCyclicVaseRPC) ResetGiftCd(ctx context.Context, req ActCyclicVaseResetGiftCdRequest, opts ...RequestOption) (ActCyclicVaseResetGiftCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActCyclicVaseResetGiftCd, req, opts...)
}

// ActDessert returns typed RPC helpers for the actDessert namespace.
func (c *RPCClient) ActDessert() ActDessertRPC { return ActDessertRPC{c: c} }

type ActDessertRPC struct{ c *RPCClient }

// ActDessertEnterRequest is the request body for gs.actDessert.enter.
type ActDessertEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActDessertEnterResponse is the namespace-delta response for gs.actDessert.enter.
type ActDessertEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actDessert.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDessertRPC) Enter(ctx context.Context, req ActDessertEnterRequest, opts ...RequestOption) (ActDessertEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDessertEnter, req, opts...)
}

// ActDessertGameOverRequest is the request body for gs.actDessert.gameOver.
type ActDessertGameOverRequest struct {
	BatchId  RPCID  `json:"batchId,omitempty"`
	GameType RPCInt `json:"gameType,omitempty"`
}

// ActDessertGameOverResponse is the namespace-delta response for gs.actDessert.gameOver.
type ActDessertGameOverResponse = RPCResponse[StateDelta]

// GameOver calls gs.actDessert.gameOver. Request fields inferred from game.js: batchId, gameType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDessertRPC) GameOver(ctx context.Context, req ActDessertGameOverRequest, opts ...RequestOption) (ActDessertGameOverResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDessertGameOver, req, opts...)
}

// ActDessertGameStartRequest is the request body for gs.actDessert.gameStart.
type ActDessertGameStartRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActDessertGameStartResponse is the namespace-delta response for gs.actDessert.gameStart.
type ActDessertGameStartResponse = RPCResponse[StateDelta]

// GameStart calls gs.actDessert.gameStart. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDessertRPC) GameStart(ctx context.Context, req ActDessertGameStartRequest, opts ...RequestOption) (ActDessertGameStartResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDessertGameStart, req, opts...)
}

// ActDessertGameSyncRequest is the request body for gs.actDessert.gameSync.
type ActDessertGameSyncRequest struct {
	BatchId  RPCID     `json:"batchId,omitempty"`
	GameType RPCInt    `json:"gameType,omitempty"`
	Args     RPCObject `json:"args,omitempty"`
}

// ActDessertGameSyncResponse is the namespace-delta response for gs.actDessert.gameSync.
type ActDessertGameSyncResponse = RPCResponse[StateDelta]

// GameSync calls gs.actDessert.gameSync. Request fields inferred from game.js: batchId, gameType, args.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDessertRPC) GameSync(ctx context.Context, req ActDessertGameSyncRequest, opts ...RequestOption) (ActDessertGameSyncResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDessertGameSync, req, opts...)
}

// ActDessertGiftBuyRequest is the request body for gs.actDessert.giftBuy.
type ActDessertGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDessertGiftBuyResponse is the namespace-delta response for gs.actDessert.giftBuy.
type ActDessertGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actDessert.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDessertRPC) GiftBuy(ctx context.Context, req ActDessertGiftBuyRequest, opts ...RequestOption) (ActDessertGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDessertGiftBuy, req, opts...)
}

// ActDessertOpenBoxRequest is the request body for gs.actDessert.openBox.
type ActDessertOpenBoxRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Num     RPCInt `json:"num,omitempty"`
}

// ActDessertOpenBoxResponse is the namespace-delta response for gs.actDessert.openBox.
type ActDessertOpenBoxResponse = RPCResponse[StateDelta]

// OpenBox calls gs.actDessert.openBox. Request fields inferred from game.js: batchId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDessertRPC) OpenBox(ctx context.Context, req ActDessertOpenBoxRequest, opts ...RequestOption) (ActDessertOpenBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDessertOpenBox, req, opts...)
}

// ActDraw returns typed RPC helpers for the actDraw namespace.
func (c *RPCClient) ActDraw() ActDrawRPC { return ActDrawRPC{c: c} }

type ActDrawRPC struct{ c *RPCClient }

// ActDrawDrawRequest is the request body for gs.actDraw.draw.
type ActDrawDrawRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawDrawResponse is the namespace-delta response for gs.actDraw.draw.
type ActDrawDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.actDraw.draw. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawRPC) Draw(ctx context.Context, req ActDrawDrawRequest, opts ...RequestOption) (ActDrawDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawDraw, req, opts...)
}

// ActDrawChristmas returns typed RPC helpers for the actDrawChristmas namespace.
func (c *RPCClient) ActDrawChristmas() ActDrawChristmasRPC { return ActDrawChristmasRPC{c: c} }

type ActDrawChristmasRPC struct{ c *RPCClient }

// ActDrawChristmasDrawRequest is the request body for gs.actDrawChristmas.draw.
type ActDrawChristmasDrawRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawChristmasDrawResponse is the namespace-delta response for gs.actDrawChristmas.draw.
type ActDrawChristmasDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.actDrawChristmas.draw. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawChristmasRPC) Draw(ctx context.Context, req ActDrawChristmasDrawRequest, opts ...RequestOption) (ActDrawChristmasDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawChristmasDraw, req, opts...)
}

// ActDrawChristmasEnterRequest is the request body for gs.actDrawChristmas.enter.
type ActDrawChristmasEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActDrawChristmasEnterResponse is the namespace-delta response for gs.actDrawChristmas.enter.
type ActDrawChristmasEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actDrawChristmas.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawChristmasRPC) Enter(ctx context.Context, req ActDrawChristmasEnterRequest, opts ...RequestOption) (ActDrawChristmasEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawChristmasEnter, req, opts...)
}

// ActDrawChristmasGiftBuyRequest is the request body for gs.actDrawChristmas.giftBuy.
type ActDrawChristmasGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawChristmasGiftBuyResponse is the namespace-delta response for gs.actDrawChristmas.giftBuy.
type ActDrawChristmasGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actDrawChristmas.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawChristmasRPC) GiftBuy(ctx context.Context, req ActDrawChristmasGiftBuyRequest, opts ...RequestOption) (ActDrawChristmasGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawChristmasGiftBuy, req, opts...)
}

// ActDrawDragon returns typed RPC helpers for the actDrawDragon namespace.
func (c *RPCClient) ActDrawDragon() ActDrawDragonRPC { return ActDrawDragonRPC{c: c} }

type ActDrawDragonRPC struct{ c *RPCClient }

// ActDrawDragonDrawRequest is the request body for gs.actDrawDragon.draw.
type ActDrawDragonDrawRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawDragonDrawResponse is the namespace-delta response for gs.actDrawDragon.draw.
type ActDrawDragonDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.actDrawDragon.draw. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawDragonRPC) Draw(ctx context.Context, req ActDrawDragonDrawRequest, opts ...RequestOption) (ActDrawDragonDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawDragonDraw, req, opts...)
}

// ActDrawDragonGiftBuyRequest is the request body for gs.actDrawDragon.giftBuy.
type ActDrawDragonGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawDragonGiftBuyResponse is the namespace-delta response for gs.actDrawDragon.giftBuy.
type ActDrawDragonGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actDrawDragon.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawDragonRPC) GiftBuy(ctx context.Context, req ActDrawDragonGiftBuyRequest, opts ...RequestOption) (ActDrawDragonGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawDragonGiftBuy, req, opts...)
}

// ActDrawDragonRecvRequest is the request body for gs.actDrawDragon.recv.
type ActDrawDragonRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActDrawDragonRecvResponse is the namespace-delta response for gs.actDrawDragon.recv.
type ActDrawDragonRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actDrawDragon.recv. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawDragonRPC) Recv(ctx context.Context, req ActDrawDragonRecvRequest, opts ...RequestOption) (ActDrawDragonRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawDragonRecv, req, opts...)
}

// ActDrawGift returns typed RPC helpers for the actDrawGift namespace.
func (c *RPCClient) ActDrawGift() ActDrawGiftRPC { return ActDrawGiftRPC{c: c} }

type ActDrawGiftRPC struct{ c *RPCClient }

// ActDrawGiftGiftBuyRequest is the request body for gs.actDrawGift.giftBuy.
type ActDrawGiftGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawGiftGiftBuyResponse is the namespace-delta response for gs.actDrawGift.giftBuy.
type ActDrawGiftGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actDrawGift.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawGiftRPC) GiftBuy(ctx context.Context, req ActDrawGiftGiftBuyRequest, opts ...RequestOption) (ActDrawGiftGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawGiftGiftBuy, req, opts...)
}

// ActDrawSprSkin returns typed RPC helpers for the actDrawSprSkin namespace.
func (c *RPCClient) ActDrawSprSkin() ActDrawSprSkinRPC { return ActDrawSprSkinRPC{c: c} }

type ActDrawSprSkinRPC struct{ c *RPCClient }

// ActDrawSprSkinDrawRequest is the request body for gs.actDrawSprSkin.draw.
type ActDrawSprSkinDrawRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawSprSkinDrawResponse is the namespace-delta response for gs.actDrawSprSkin.draw.
type ActDrawSprSkinDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.actDrawSprSkin.draw. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawSprSkinRPC) Draw(ctx context.Context, req ActDrawSprSkinDrawRequest, opts ...RequestOption) (ActDrawSprSkinDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawSprSkinDraw, req, opts...)
}

// ActDrawSprSkinEnterRequest is the request body for gs.actDrawSprSkin.enter.
type ActDrawSprSkinEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActDrawSprSkinEnterResponse is the namespace-delta response for gs.actDrawSprSkin.enter.
type ActDrawSprSkinEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actDrawSprSkin.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawSprSkinRPC) Enter(ctx context.Context, req ActDrawSprSkinEnterRequest, opts ...RequestOption) (ActDrawSprSkinEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawSprSkinEnter, req, opts...)
}

// ActDrawSprSkinGiftBuyRequest is the request body for gs.actDrawSprSkin.giftBuy.
type ActDrawSprSkinGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawSprSkinGiftBuyResponse is the namespace-delta response for gs.actDrawSprSkin.giftBuy.
type ActDrawSprSkinGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actDrawSprSkin.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawSprSkinRPC) GiftBuy(ctx context.Context, req ActDrawSprSkinGiftBuyRequest, opts ...RequestOption) (ActDrawSprSkinGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawSprSkinGiftBuy, req, opts...)
}

// ActDrawZb returns typed RPC helpers for the actDrawZb namespace.
func (c *RPCClient) ActDrawZb() ActDrawZbRPC { return ActDrawZbRPC{c: c} }

type ActDrawZbRPC struct{ c *RPCClient }

// ActDrawZbDrawRequest is the request body for gs.actDrawZb.draw.
type ActDrawZbDrawRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawZbDrawResponse is the namespace-delta response for gs.actDrawZb.draw.
type ActDrawZbDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.actDrawZb.draw. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawZbRPC) Draw(ctx context.Context, req ActDrawZbDrawRequest, opts ...RequestOption) (ActDrawZbDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawZbDraw, req, opts...)
}

// ActDrawZbEnterRequest is the request body for gs.actDrawZb.enter.
type ActDrawZbEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActDrawZbEnterResponse is the namespace-delta response for gs.actDrawZb.enter.
type ActDrawZbEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actDrawZb.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawZbRPC) Enter(ctx context.Context, req ActDrawZbEnterRequest, opts ...RequestOption) (ActDrawZbEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawZbEnter, req, opts...)
}

// ActDrawZbGiftBuyRequest is the request body for gs.actDrawZb.giftBuy.
type ActDrawZbGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActDrawZbGiftBuyResponse is the namespace-delta response for gs.actDrawZb.giftBuy.
type ActDrawZbGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actDrawZb.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActDrawZbRPC) GiftBuy(ctx context.Context, req ActDrawZbGiftBuyRequest, opts ...RequestOption) (ActDrawZbGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActDrawZbGiftBuy, req, opts...)
}

// ActElim returns typed RPC helpers for the actElim namespace.
func (c *RPCClient) ActElim() ActElimRPC { return ActElimRPC{c: c} }

type ActElimRPC struct{ c *RPCClient }

// ActElimEnterRequest is the request body for gs.actElim.enter.
type ActElimEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActElimEnterResponse is the namespace-delta response for gs.actElim.enter.
type ActElimEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actElim.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) Enter(ctx context.Context, req ActElimEnterRequest, opts ...RequestOption) (ActElimEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimEnter, req, opts...)
}

// ActElimGiftBuyRequest is the request body for gs.actElim.giftBuy.
type ActElimGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActElimGiftBuyResponse is the namespace-delta response for gs.actElim.giftBuy.
type ActElimGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actElim.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) GiftBuy(ctx context.Context, req ActElimGiftBuyRequest, opts ...RequestOption) (ActElimGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimGiftBuy, req, opts...)
}

// ActElimMoveRequest is the request body for gs.actElim.move.
type ActElimMoveRequest struct {
	BatchId   RPCID  `json:"batchId,omitempty"`
	Model     RPCInt `json:"model,omitempty"`
	RowBefore RPCInt `json:"rowBefore,omitempty"`
	ColBefore RPCInt `json:"colBefore,omitempty"`
	RowAfter  RPCInt `json:"rowAfter,omitempty"`
	ColAfter  RPCInt `json:"colAfter,omitempty"`
}

// ActElimMoveResponse is the namespace-delta response for gs.actElim.move.
type ActElimMoveResponse = RPCResponse[StateDelta]

// Move calls gs.actElim.move. Request fields inferred from game.js: batchId, model, rowBefore, colBefore, rowAfter, colAfter.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) Move(ctx context.Context, req ActElimMoveRequest, opts ...RequestOption) (ActElimMoveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimMove, req, opts...)
}

// ActElimOpenBoxRequest is the request body for gs.actElim.openBox.
type ActElimOpenBoxRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Num     RPCInt `json:"num,omitempty"`
}

// ActElimOpenBoxResponse is the namespace-delta response for gs.actElim.openBox.
type ActElimOpenBoxResponse = RPCResponse[StateDelta]

// OpenBox calls gs.actElim.openBox. Request fields inferred from game.js: batchId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) OpenBox(ctx context.Context, req ActElimOpenBoxRequest, opts ...RequestOption) (ActElimOpenBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimOpenBox, req, opts...)
}

// ActElimRefreshMapRequest is the request body for gs.actElim.refreshMap.
type ActElimRefreshMapRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Model   RPCInt `json:"model,omitempty"`
}

// ActElimRefreshMapResponse is the namespace-delta response for gs.actElim.refreshMap.
type ActElimRefreshMapResponse = RPCResponse[StateDelta]

// RefreshMap calls gs.actElim.refreshMap. Request fields inferred from game.js: batchId, model.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) RefreshMap(ctx context.Context, req ActElimRefreshMapRequest, opts ...RequestOption) (ActElimRefreshMapResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimRefreshMap, req, opts...)
}

// ActElimUseItem1Request is the request body for gs.actElim.useItem1.
type ActElimUseItem1Request struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Model   RPCInt `json:"model,omitempty"`
}

// ActElimUseItem1Response is the namespace-delta response for gs.actElim.useItem1.
type ActElimUseItem1Response = RPCResponse[StateDelta]

// UseItem1 calls gs.actElim.useItem1. Request fields inferred from game.js: batchId, model.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) UseItem1(ctx context.Context, req ActElimUseItem1Request, opts ...RequestOption) (ActElimUseItem1Response, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimUseItem1, req, opts...)
}

// ActElimUseItem2Request is the request body for gs.actElim.useItem2.
type ActElimUseItem2Request struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Model   RPCInt `json:"model,omitempty"`
	Row     RPCInt `json:"row,omitempty"`
	Col     RPCInt `json:"col,omitempty"`
}

// ActElimUseItem2Response is the namespace-delta response for gs.actElim.useItem2.
type ActElimUseItem2Response = RPCResponse[StateDelta]

// UseItem2 calls gs.actElim.useItem2. Request fields inferred from game.js: batchId, model, row, col.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActElimRPC) UseItem2(ctx context.Context, req ActElimUseItem2Request, opts ...RequestOption) (ActElimUseItem2Response, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActElimUseItem2, req, opts...)
}

// ActFlowerBattle returns typed RPC helpers for the actFlowerBattle namespace.
func (c *RPCClient) ActFlowerBattle() ActFlowerBattleRPC { return ActFlowerBattleRPC{c: c} }

type ActFlowerBattleRPC struct{ c *RPCClient }

// ActFlowerBattleChooseFlowerArtRequest is the request body for gs.actFlowerBattle.chooseFlowerArt.
type ActFlowerBattleChooseFlowerArtRequest struct {
	BatchId     RPCID `json:"batchId,omitempty"`
	FlowerArtId RPCID `json:"flowerArtId,omitempty"`
}

// ActFlowerBattleChooseFlowerArtResponse is the namespace-delta response for gs.actFlowerBattle.chooseFlowerArt.
type ActFlowerBattleChooseFlowerArtResponse = RPCResponse[StateDelta]

// ChooseFlowerArt calls gs.actFlowerBattle.chooseFlowerArt. Request fields inferred from game.js: batchId, flowerArtId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) ChooseFlowerArt(ctx context.Context, req ActFlowerBattleChooseFlowerArtRequest, opts ...RequestOption) (ActFlowerBattleChooseFlowerArtResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleChooseFlowerArt, req, opts...)
}

// ActFlowerBattleEnterRequest is the request body for gs.actFlowerBattle.enter.
type ActFlowerBattleEnterRequest struct {
	BatchId    RPCID   `json:"batchId,omitempty"`
	IsRefresh  RPCBool `json:"isRefresh,omitempty"`
	IsCrossDay RPCBool `json:"isCrossDay,omitempty"`
}

// ActFlowerBattleEnterResponse is the namespace-delta response for gs.actFlowerBattle.enter.
type ActFlowerBattleEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actFlowerBattle.enter. Request fields inferred from game.js: batchId, isRefresh, isCrossDay.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) Enter(ctx context.Context, req ActFlowerBattleEnterRequest, opts ...RequestOption) (ActFlowerBattleEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleEnter, req, opts...)
}

// ActFlowerBattleGetGiftBuyRecordsRequest is the request body for gs.actFlowerBattle.getGiftBuyRecords.
type ActFlowerBattleGetGiftBuyRecordsRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActFlowerBattleGetGiftBuyRecordsResponse is the namespace-delta response for gs.actFlowerBattle.getGiftBuyRecords.
type ActFlowerBattleGetGiftBuyRecordsResponse = RPCResponse[StateDelta]

// GetGiftBuyRecords calls gs.actFlowerBattle.getGiftBuyRecords. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) GetGiftBuyRecords(ctx context.Context, req ActFlowerBattleGetGiftBuyRecordsRequest, opts ...RequestOption) (ActFlowerBattleGetGiftBuyRecordsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleGetGiftBuyRecords, req, opts...)
}

// ActFlowerBattleGiftBuyRequest is the request body for gs.actFlowerBattle.giftBuy.
type ActFlowerBattleGiftBuyRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GiftId  RPCID `json:"giftId,omitempty"`
}

// ActFlowerBattleGiftBuyResponse is the namespace-delta response for gs.actFlowerBattle.giftBuy.
type ActFlowerBattleGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actFlowerBattle.giftBuy. Request fields inferred from game.js: batchId, giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) GiftBuy(ctx context.Context, req ActFlowerBattleGiftBuyRequest, opts ...RequestOption) (ActFlowerBattleGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleGiftBuy, req, opts...)
}

// ActFlowerBattleLikeRequest is the request body for gs.actFlowerBattle.like.
type ActFlowerBattleLikeRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActFlowerBattleLikeResponse is the namespace-delta response for gs.actFlowerBattle.like.
type ActFlowerBattleLikeResponse = RPCResponse[StateDelta]

// Like calls gs.actFlowerBattle.like. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) Like(ctx context.Context, req ActFlowerBattleLikeRequest, opts ...RequestOption) (ActFlowerBattleLikeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleLike, req, opts...)
}

// ActFlowerBattleRecvBoxesPrizeRequest is the request body for gs.actFlowerBattle.recvBoxesPrize.
type ActFlowerBattleRecvBoxesPrizeRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActFlowerBattleRecvBoxesPrizeResponse is the namespace-delta response for gs.actFlowerBattle.recvBoxesPrize.
type ActFlowerBattleRecvBoxesPrizeResponse = RPCResponse[StateDelta]

// RecvBoxesPrize calls gs.actFlowerBattle.recvBoxesPrize. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) RecvBoxesPrize(ctx context.Context, req ActFlowerBattleRecvBoxesPrizeRequest, opts ...RequestOption) (ActFlowerBattleRecvBoxesPrizeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleRecvBoxesPrize, req, opts...)
}

// ActFlowerBattleSetIsAnonymousRequest is the request body for gs.actFlowerBattle.setIsAnonymous.
type ActFlowerBattleSetIsAnonymousRequest struct {
	BatchId     RPCID   `json:"batchId,omitempty"`
	IsAnonymous RPCBool `json:"isAnonymous,omitempty"`
}

// ActFlowerBattleSetIsAnonymousResponse is the namespace-delta response for gs.actFlowerBattle.setIsAnonymous.
type ActFlowerBattleSetIsAnonymousResponse = RPCResponse[StateDelta]

// SetIsAnonymous calls gs.actFlowerBattle.setIsAnonymous. Request fields inferred from game.js: batchId, isAnonymous.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFlowerBattleRPC) SetIsAnonymous(ctx context.Context, req ActFlowerBattleSetIsAnonymousRequest, opts ...RequestOption) (ActFlowerBattleSetIsAnonymousResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFlowerBattleSetIsAnonymous, req, opts...)
}

// ActFmlRedEnvelope returns typed RPC helpers for the actFmlRedEnvelope namespace.
func (c *RPCClient) ActFmlRedEnvelope() ActFmlRedEnvelopeRPC { return ActFmlRedEnvelopeRPC{c: c} }

type ActFmlRedEnvelopeRPC struct{ c *RPCClient }

// ActFmlRedEnvelopeEnterRequest is the request body for gs.actFmlRedEnvelope.enter.
type ActFmlRedEnvelopeEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActFmlRedEnvelopeEnterResponse is the namespace-delta response for gs.actFmlRedEnvelope.enter.
type ActFmlRedEnvelopeEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actFmlRedEnvelope.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFmlRedEnvelopeRPC) Enter(ctx context.Context, req ActFmlRedEnvelopeEnterRequest, opts ...RequestOption) (ActFmlRedEnvelopeEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFmlRedEnvelopeEnter, req, opts...)
}

// ActFmlRedEnvelopeGetDetailRequest is the request body for gs.actFmlRedEnvelope.getDetail.
type ActFmlRedEnvelopeGetDetailRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	ID      RPCID `json:"id,omitempty"`
}

// ActFmlRedEnvelopeGetDetailResponse is the namespace-delta response for gs.actFmlRedEnvelope.getDetail.
type ActFmlRedEnvelopeGetDetailResponse = RPCResponse[StateDelta]

// GetDetail calls gs.actFmlRedEnvelope.getDetail. Request fields inferred from game.js: batchId, id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFmlRedEnvelopeRPC) GetDetail(ctx context.Context, req ActFmlRedEnvelopeGetDetailRequest, opts ...RequestOption) (ActFmlRedEnvelopeGetDetailResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFmlRedEnvelopeGetDetail, req, opts...)
}

// ActFmlRedEnvelopeGetRecordRequest is the request body for gs.actFmlRedEnvelope.getRecord.
type ActFmlRedEnvelopeGetRecordRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActFmlRedEnvelopeGetRecordResponse is the namespace-delta response for gs.actFmlRedEnvelope.getRecord.
type ActFmlRedEnvelopeGetRecordResponse = RPCResponse[StateDelta]

// GetRecord calls gs.actFmlRedEnvelope.getRecord. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFmlRedEnvelopeRPC) GetRecord(ctx context.Context, req ActFmlRedEnvelopeGetRecordRequest, opts ...RequestOption) (ActFmlRedEnvelopeGetRecordResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFmlRedEnvelopeGetRecord, req, opts...)
}

// ActFmlRedEnvelopeListRequest is the request body for gs.actFmlRedEnvelope.list.
type ActFmlRedEnvelopeListRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActFmlRedEnvelopeListResponse is the namespace-delta response for gs.actFmlRedEnvelope.list.
type ActFmlRedEnvelopeListResponse = RPCResponse[StateDelta]

// List calls gs.actFmlRedEnvelope.list. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFmlRedEnvelopeRPC) List(ctx context.Context, req ActFmlRedEnvelopeListRequest, opts ...RequestOption) (ActFmlRedEnvelopeListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFmlRedEnvelopeList, req, opts...)
}

// ActFmlRedEnvelopePickRequest is the request body for gs.actFmlRedEnvelope.pick.
type ActFmlRedEnvelopePickRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	ID      RPCID `json:"id,omitempty"`
}

// ActFmlRedEnvelopePickResponse is the namespace-delta response for gs.actFmlRedEnvelope.pick.
type ActFmlRedEnvelopePickResponse = RPCResponse[StateDelta]

// Pick calls gs.actFmlRedEnvelope.pick. Request fields inferred from game.js: batchId, id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFmlRedEnvelopeRPC) Pick(ctx context.Context, req ActFmlRedEnvelopePickRequest, opts ...RequestOption) (ActFmlRedEnvelopePickResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFmlRedEnvelopePick, req, opts...)
}

// ActFmlRedEnvelopeSendRequest is the request body for gs.actFmlRedEnvelope.send.
type ActFmlRedEnvelopeSendRequest struct {
	BatchId RPCID     `json:"batchId,omitempty"`
	ItemId  RPCID     `json:"itemId,omitempty"`
	Count   RPCInt    `json:"count,omitempty"`
	Msg     RPCString `json:"msg,omitempty"`
}

// ActFmlRedEnvelopeSendResponse is the namespace-delta response for gs.actFmlRedEnvelope.send.
type ActFmlRedEnvelopeSendResponse = RPCResponse[StateDelta]

// Send calls gs.actFmlRedEnvelope.send. Request fields inferred from game.js: batchId, itemId, count, msg.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActFmlRedEnvelopeRPC) Send(ctx context.Context, req ActFmlRedEnvelopeSendRequest, opts ...RequestOption) (ActFmlRedEnvelopeSendResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActFmlRedEnvelopeSend, req, opts...)
}

// ActGame2048 returns typed RPC helpers for the actGame2048 namespace.
func (c *RPCClient) ActGame2048() ActGame2048RPC { return ActGame2048RPC{c: c} }

type ActGame2048RPC struct{ c *RPCClient }

// ActGame2048EnterRequest is the request body for gs.actGame2048.enter.
type ActGame2048EnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActGame2048EnterResponse is the namespace-delta response for gs.actGame2048.enter.
type ActGame2048EnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actGame2048.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) Enter(ctx context.Context, req ActGame2048EnterRequest, opts ...RequestOption) (ActGame2048EnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048Enter, req, opts...)
}

// ActGame2048GiftBuyRequest is the request body for gs.actGame2048.giftBuy.
type ActGame2048GiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActGame2048GiftBuyResponse is the namespace-delta response for gs.actGame2048.giftBuy.
type ActGame2048GiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actGame2048.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) GiftBuy(ctx context.Context, req ActGame2048GiftBuyRequest, opts ...RequestOption) (ActGame2048GiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048GiftBuy, req, opts...)
}

// ActGame2048MoveRequest is the request body for gs.actGame2048.move.
type ActGame2048MoveRequest struct {
	BatchId RPCID     `json:"batchId,omitempty"`
	Map     RPCObject `json:"map,omitempty"`
	Model   RPCInt    `json:"model,omitempty"`
	Dir     RPCInt    `json:"dir,omitempty"`
}

// ActGame2048MoveResponse is the namespace-delta response for gs.actGame2048.move.
type ActGame2048MoveResponse = RPCResponse[StateDelta]

// Move calls gs.actGame2048.move. Request fields inferred from game.js: batchId, map, model, dir.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) Move(ctx context.Context, req ActGame2048MoveRequest, opts ...RequestOption) (ActGame2048MoveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048Move, req, opts...)
}

// ActGame2048OpenBoxRequest is the request body for gs.actGame2048.openBox.
type ActGame2048OpenBoxRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Num     RPCInt `json:"num,omitempty"`
}

// ActGame2048OpenBoxResponse is the namespace-delta response for gs.actGame2048.openBox.
type ActGame2048OpenBoxResponse = RPCResponse[StateDelta]

// OpenBox calls gs.actGame2048.openBox. Request fields inferred from game.js: batchId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) OpenBox(ctx context.Context, req ActGame2048OpenBoxRequest, opts ...RequestOption) (ActGame2048OpenBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048OpenBox, req, opts...)
}

// ActGame2048RestartRequest is the request body for gs.actGame2048.restart.
type ActGame2048RestartRequest struct {
	BatchId RPCID     `json:"batchId,omitempty"`
	Map     RPCObject `json:"map,omitempty"`
}

// ActGame2048RestartResponse is the namespace-delta response for gs.actGame2048.restart.
type ActGame2048RestartResponse = RPCResponse[StateDelta]

// Restart calls gs.actGame2048.restart. Request fields inferred from game.js: batchId, map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) Restart(ctx context.Context, req ActGame2048RestartRequest, opts ...RequestOption) (ActGame2048RestartResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048Restart, req, opts...)
}

// ActGame2048UseChangeRequest is the request body for gs.actGame2048.useChange.
type ActGame2048UseChangeRequest struct {
	BatchId RPCID     `json:"batchId,omitempty"`
	Map     RPCObject `json:"map,omitempty"`
	Cells   RPCArray  `json:"cells,omitempty"`
}

// ActGame2048UseChangeResponse is the namespace-delta response for gs.actGame2048.useChange.
type ActGame2048UseChangeResponse = RPCResponse[StateDelta]

// UseChange calls gs.actGame2048.useChange. Request fields inferred from game.js: batchId, map, cells.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) UseChange(ctx context.Context, req ActGame2048UseChangeRequest, opts ...RequestOption) (ActGame2048UseChangeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048UseChange, req, opts...)
}

// ActGame2048UseEliminateRequest is the request body for gs.actGame2048.useEliminate.
type ActGame2048UseEliminateRequest struct {
	BatchId RPCID     `json:"batchId,omitempty"`
	Map     RPCObject `json:"map,omitempty"`
	Cells   RPCArray  `json:"cells,omitempty"`
}

// ActGame2048UseEliminateResponse is the namespace-delta response for gs.actGame2048.useEliminate.
type ActGame2048UseEliminateResponse = RPCResponse[StateDelta]

// UseEliminate calls gs.actGame2048.useEliminate. Request fields inferred from game.js: batchId, map, cells.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActGame2048RPC) UseEliminate(ctx context.Context, req ActGame2048UseEliminateRequest, opts ...RequestOption) (ActGame2048UseEliminateResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActGame2048UseEliminate, req, opts...)
}

// ActHoney returns typed RPC helpers for the actHoney namespace.
func (c *RPCClient) ActHoney() ActHoneyRPC { return ActHoneyRPC{c: c} }

type ActHoneyRPC struct{ c *RPCClient }

// ActHoneyGiftBuyRequest is the request body for gs.actHoney.giftBuy.
type ActHoneyGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActHoneyGiftBuyResponse is the namespace-delta response for gs.actHoney.giftBuy.
type ActHoneyGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actHoney.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActHoneyRPC) GiftBuy(ctx context.Context, req ActHoneyGiftBuyRequest, opts ...RequestOption) (ActHoneyGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActHoneyGiftBuy, req, opts...)
}

// ActHoneyRecvRequest is the request body for gs.actHoney.recv.
type ActHoneyRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActHoneyRecvResponse is the namespace-delta response for gs.actHoney.recv.
type ActHoneyRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actHoney.recv. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActHoneyRPC) Recv(ctx context.Context, req ActHoneyRecvRequest, opts ...RequestOption) (ActHoneyRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActHoneyRecv, req, opts...)
}

// ActHoneyResetGiftCdRequest is the request body for gs.actHoney.resetGiftCd.
type ActHoneyResetGiftCdRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GiftId  RPCID `json:"giftId,omitempty"`
}

// ActHoneyResetGiftCdResponse is the namespace-delta response for gs.actHoney.resetGiftCd.
type ActHoneyResetGiftCdResponse = RPCResponse[StateDelta]

// ResetGiftCd calls gs.actHoney.resetGiftCd. Request fields inferred from game.js: batchId, giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActHoneyRPC) ResetGiftCd(ctx context.Context, req ActHoneyResetGiftCdRequest, opts ...RequestOption) (ActHoneyResetGiftCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActHoneyResetGiftCd, req, opts...)
}

// ActIPDmdGift returns typed RPC helpers for the actIPDmdGift namespace.
func (c *RPCClient) ActIPDmdGift() ActIPDmdGiftRPC { return ActIPDmdGiftRPC{c: c} }

type ActIPDmdGiftRPC struct{ c *RPCClient }

// ActIPDmdGiftGiftBuyRequest is the request body for gs.actIPDmdGift.giftBuy.
type ActIPDmdGiftGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActIPDmdGiftGiftBuyResponse is the namespace-delta response for gs.actIPDmdGift.giftBuy.
type ActIPDmdGiftGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actIPDmdGift.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActIPDmdGiftRPC) GiftBuy(ctx context.Context, req ActIPDmdGiftGiftBuyRequest, opts ...RequestOption) (ActIPDmdGiftGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActIPDmdGiftGiftBuy, req, opts...)
}

// ActIPFlowerGuard returns typed RPC helpers for the actIPFlowerGuard namespace.
func (c *RPCClient) ActIPFlowerGuard() ActIPFlowerGuardRPC { return ActIPFlowerGuardRPC{c: c} }

type ActIPFlowerGuardRPC struct{ c *RPCClient }

// ActIPFlowerGuardOpenBoxRequest is the request body for gs.actIPFlowerGuard.openBox.
type ActIPFlowerGuardOpenBoxRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	BoxId   RPCID `json:"boxId,omitempty"`
}

// ActIPFlowerGuardOpenBoxResponse is the namespace-delta response for gs.actIPFlowerGuard.openBox.
type ActIPFlowerGuardOpenBoxResponse = RPCResponse[StateDelta]

// OpenBox calls gs.actIPFlowerGuard.openBox. Request fields inferred from game.js: batchId, boxId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActIPFlowerGuardRPC) OpenBox(ctx context.Context, req ActIPFlowerGuardOpenBoxRequest, opts ...RequestOption) (ActIPFlowerGuardOpenBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActIPFlowerGuardOpenBox, req, opts...)
}

// ActMerge2 returns typed RPC helpers for the actMerge2 namespace.
func (c *RPCClient) ActMerge2() ActMerge2RPC { return ActMerge2RPC{c: c} }

type ActMerge2RPC struct{ c *RPCClient }

// ActMerge2EnterRequest is the request body for gs.actMerge2.enter.
type ActMerge2EnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActMerge2EnterResponse is the namespace-delta response for gs.actMerge2.enter.
type ActMerge2EnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actMerge2.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) Enter(ctx context.Context, req ActMerge2EnterRequest, opts ...RequestOption) (ActMerge2EnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2Enter, req, opts...)
}

// ActMerge2MoveRequest is the request body for gs.actMerge2.move.
type ActMerge2MoveRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	ORc     RPCInt `json:"oRc,omitempty"`
	TRc     RPCInt `json:"tRc,omitempty"`
}

// ActMerge2MoveResponse is the namespace-delta response for gs.actMerge2.move.
type ActMerge2MoveResponse = RPCResponse[StateDelta]

// Move calls gs.actMerge2.move. Request fields inferred from game.js: batchId, oRc, tRc.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) Move(ctx context.Context, req ActMerge2MoveRequest, opts ...RequestOption) (ActMerge2MoveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2Move, req, opts...)
}

// ActMerge2OpenBoxRequest is the request body for gs.actMerge2.openBox.
type ActMerge2OpenBoxRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Num     RPCInt `json:"num,omitempty"`
}

// ActMerge2OpenBoxResponse is the namespace-delta response for gs.actMerge2.openBox.
type ActMerge2OpenBoxResponse = RPCResponse[StateDelta]

// OpenBox calls gs.actMerge2.openBox. Request fields inferred from game.js: batchId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) OpenBox(ctx context.Context, req ActMerge2OpenBoxRequest, opts ...RequestOption) (ActMerge2OpenBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2OpenBox, req, opts...)
}

// ActMerge2PutInWarehouseRequest is the request body for gs.actMerge2.putInWarehouse.
type ActMerge2PutInWarehouseRequest struct {
	BatchId RPCID    `json:"batchId,omitempty"`
	Cell    RPCValue `json:"cell,omitempty"`
}

// ActMerge2PutInWarehouseResponse is the namespace-delta response for gs.actMerge2.putInWarehouse.
type ActMerge2PutInWarehouseResponse = RPCResponse[StateDelta]

// PutInWarehouse calls gs.actMerge2.putInWarehouse. Request fields inferred from game.js: batchId, cell.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) PutInWarehouse(ctx context.Context, req ActMerge2PutInWarehouseRequest, opts ...RequestOption) (ActMerge2PutInWarehouseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2PutInWarehouse, req, opts...)
}

// ActMerge2PutOutTempRequest is the request body for gs.actMerge2.putOutTemp.
type ActMerge2PutOutTempRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActMerge2PutOutTempResponse is the namespace-delta response for gs.actMerge2.putOutTemp.
type ActMerge2PutOutTempResponse = RPCResponse[StateDelta]

// PutOutTemp calls gs.actMerge2.putOutTemp. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) PutOutTemp(ctx context.Context, req ActMerge2PutOutTempRequest, opts ...RequestOption) (ActMerge2PutOutTempResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2PutOutTemp, req, opts...)
}

// ActMerge2PutOutWarehouseRequest is the request body for gs.actMerge2.putOutWarehouse.
type ActMerge2PutOutWarehouseRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActMerge2PutOutWarehouseResponse is the namespace-delta response for gs.actMerge2.putOutWarehouse.
type ActMerge2PutOutWarehouseResponse = RPCResponse[StateDelta]

// PutOutWarehouse calls gs.actMerge2.putOutWarehouse. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) PutOutWarehouse(ctx context.Context, req ActMerge2PutOutWarehouseRequest, opts ...RequestOption) (ActMerge2PutOutWarehouseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2PutOutWarehouse, req, opts...)
}

// ActMerge2RecvOrderRequest is the request body for gs.actMerge2.recvOrder.
type ActMerge2RecvOrderRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	OrderId RPCID `json:"orderId,omitempty"`
}

// ActMerge2RecvOrderResponse is the namespace-delta response for gs.actMerge2.recvOrder.
type ActMerge2RecvOrderResponse = RPCResponse[StateDelta]

// RecvOrder calls gs.actMerge2.recvOrder. Request fields inferred from game.js: batchId, orderId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) RecvOrder(ctx context.Context, req ActMerge2RecvOrderRequest, opts ...RequestOption) (ActMerge2RecvOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2RecvOrder, req, opts...)
}

// ActMerge2RecvProgressRequest is the request body for gs.actMerge2.recvProgress.
type ActMerge2RecvProgressRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActMerge2RecvProgressResponse is the namespace-delta response for gs.actMerge2.recvProgress.
type ActMerge2RecvProgressResponse = RPCResponse[StateDelta]

// RecvProgress calls gs.actMerge2.recvProgress. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) RecvProgress(ctx context.Context, req ActMerge2RecvProgressRequest, opts ...RequestOption) (ActMerge2RecvProgressResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2RecvProgress, req, opts...)
}

// ActMerge2RefreshOrderRequest is the request body for gs.actMerge2.refreshOrder.
type ActMerge2RefreshOrderRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActMerge2RefreshOrderResponse is the namespace-delta response for gs.actMerge2.refreshOrder.
type ActMerge2RefreshOrderResponse = RPCResponse[StateDelta]

// RefreshOrder calls gs.actMerge2.refreshOrder. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) RefreshOrder(ctx context.Context, req ActMerge2RefreshOrderRequest, opts ...RequestOption) (ActMerge2RefreshOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2RefreshOrder, req, opts...)
}

// ActMerge2SaveGuideRequest is the request body for gs.actMerge2.saveGuide.
type ActMerge2SaveGuideRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GuideId RPCID `json:"guideId,omitempty"`
}

// ActMerge2SaveGuideResponse is the namespace-delta response for gs.actMerge2.saveGuide.
type ActMerge2SaveGuideResponse = RPCResponse[StateDelta]

// SaveGuide calls gs.actMerge2.saveGuide. Request fields inferred from game.js: batchId, guideId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) SaveGuide(ctx context.Context, req ActMerge2SaveGuideRequest, opts ...RequestOption) (ActMerge2SaveGuideResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2SaveGuide, req, opts...)
}

// ActMerge2SellItemRequest is the request body for gs.actMerge2.sellItem.
type ActMerge2SellItemRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Rc      RPCInt `json:"rc,omitempty"`
}

// ActMerge2SellItemResponse is the namespace-delta response for gs.actMerge2.sellItem.
type ActMerge2SellItemResponse = RPCResponse[StateDelta]

// SellItem calls gs.actMerge2.sellItem. Request fields inferred from game.js: batchId, rc.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) SellItem(ctx context.Context, req ActMerge2SellItemRequest, opts ...RequestOption) (ActMerge2SellItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2SellItem, req, opts...)
}

// ActMerge2SplitItemRequest is the request body for gs.actMerge2.splitItem.
type ActMerge2SplitItemRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	SRc     RPCInt `json:"sRc,omitempty"`
	TRc     RPCInt `json:"tRc,omitempty"`
}

// ActMerge2SplitItemResponse is the namespace-delta response for gs.actMerge2.splitItem.
type ActMerge2SplitItemResponse = RPCResponse[StateDelta]

// SplitItem calls gs.actMerge2.splitItem. Request fields inferred from game.js: batchId, sRc, tRc.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) SplitItem(ctx context.Context, req ActMerge2SplitItemRequest, opts ...RequestOption) (ActMerge2SplitItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2SplitItem, req, opts...)
}

// ActMerge2SwitchModeRequest is the request body for gs.actMerge2.switchMode.
type ActMerge2SwitchModeRequest struct {
	BatchId  RPCID  `json:"batchId,omitempty"`
	ModeType RPCInt `json:"modeType,omitempty"`
}

// ActMerge2SwitchModeResponse is the namespace-delta response for gs.actMerge2.switchMode.
type ActMerge2SwitchModeResponse = RPCResponse[StateDelta]

// SwitchMode calls gs.actMerge2.switchMode. Request fields inferred from game.js: batchId, modeType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) SwitchMode(ctx context.Context, req ActMerge2SwitchModeRequest, opts ...RequestOption) (ActMerge2SwitchModeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2SwitchMode, req, opts...)
}

// ActMerge2UnlockWarehouseRequest is the request body for gs.actMerge2.unlockWarehouse.
type ActMerge2UnlockWarehouseRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActMerge2UnlockWarehouseResponse is the namespace-delta response for gs.actMerge2.unlockWarehouse.
type ActMerge2UnlockWarehouseResponse = RPCResponse[StateDelta]

// UnlockWarehouse calls gs.actMerge2.unlockWarehouse. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) UnlockWarehouse(ctx context.Context, req ActMerge2UnlockWarehouseRequest, opts ...RequestOption) (ActMerge2UnlockWarehouseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2UnlockWarehouse, req, opts...)
}

// ActMerge2UseItemRequest is the request body for gs.actMerge2.useItem.
type ActMerge2UseItemRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Rc      RPCInt `json:"rc,omitempty"`
}

// ActMerge2UseItemResponse is the namespace-delta response for gs.actMerge2.useItem.
type ActMerge2UseItemResponse = RPCResponse[StateDelta]

// UseItem calls gs.actMerge2.useItem. Request fields inferred from game.js: batchId, rc.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActMerge2RPC) UseItem(ctx context.Context, req ActMerge2UseItemRequest, opts ...RequestOption) (ActMerge2UseItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActMerge2UseItem, req, opts...)
}

// ActOfficials returns typed RPC helpers for the actOfficials namespace.
func (c *RPCClient) ActOfficials() ActOfficialsRPC { return ActOfficialsRPC{c: c} }

type ActOfficialsRPC struct{ c *RPCClient }

// ActOfficialsBuyItemRequest is the request body for gs.actOfficials.buyItem.
type ActOfficialsBuyItemRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	ItemId  RPCID  `json:"itemId,omitempty"`
	BuyNum  RPCInt `json:"buyNum,omitempty"`
}

// ActOfficialsBuyItemResponse is the namespace-delta response for gs.actOfficials.buyItem.
type ActOfficialsBuyItemResponse = RPCResponse[StateDelta]

// BuyItem calls gs.actOfficials.buyItem. Request fields inferred from game.js: batchId, itemId, buyNum.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActOfficialsRPC) BuyItem(ctx context.Context, req ActOfficialsBuyItemRequest, opts ...RequestOption) (ActOfficialsBuyItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActOfficialsBuyItem, req, opts...)
}

// ActOfficialsEnterRequest is the request body for gs.actOfficials.enter.
type ActOfficialsEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActOfficialsEnterResponse is the namespace-delta response for gs.actOfficials.enter.
type ActOfficialsEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actOfficials.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActOfficialsRPC) Enter(ctx context.Context, req ActOfficialsEnterRequest, opts ...RequestOption) (ActOfficialsEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActOfficialsEnter, req, opts...)
}

// ActOfficialsRecvGrpReachPrizeRequest is the request body for gs.actOfficials.recvGrpReachPrize.
type ActOfficialsRecvGrpReachPrizeRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActOfficialsRecvGrpReachPrizeResponse is the namespace-delta response for gs.actOfficials.recvGrpReachPrize.
type ActOfficialsRecvGrpReachPrizeResponse = RPCResponse[StateDelta]

// RecvGrpReachPrize calls gs.actOfficials.recvGrpReachPrize. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActOfficialsRPC) RecvGrpReachPrize(ctx context.Context, req ActOfficialsRecvGrpReachPrizeRequest, opts ...RequestOption) (ActOfficialsRecvGrpReachPrizeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActOfficialsRecvGrpReachPrize, req, opts...)
}

// ActOfficialsUseItemRequest is the request body for gs.actOfficials.useItem.
type ActOfficialsUseItemRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	ItemId  RPCID  `json:"itemId,omitempty"`
	UseNum  RPCInt `json:"useNum,omitempty"`
}

// ActOfficialsUseItemResponse is the namespace-delta response for gs.actOfficials.useItem.
type ActOfficialsUseItemResponse = RPCResponse[StateDelta]

// UseItem calls gs.actOfficials.useItem. Request fields inferred from game.js: batchId, itemId, useNum.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActOfficialsRPC) UseItem(ctx context.Context, req ActOfficialsUseItemRequest, opts ...RequestOption) (ActOfficialsUseItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActOfficialsUseItem, req, opts...)
}

// ActPaper returns typed RPC helpers for the actPaper namespace.
func (c *RPCClient) ActPaper() ActPaperRPC { return ActPaperRPC{c: c} }

type ActPaperRPC struct{ c *RPCClient }

// ActPaperEnterRequest is the request body for gs.actPaper.enter.
type ActPaperEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActPaperEnterResponse is the namespace-delta response for gs.actPaper.enter.
type ActPaperEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actPaper.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActPaperRPC) Enter(ctx context.Context, req ActPaperEnterRequest, opts ...RequestOption) (ActPaperEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActPaperEnter, req, opts...)
}

// ActPaperRecvRequest is the request body for gs.actPaper.recv.
type ActPaperRecvRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Idx     RPCInt `json:"idx,omitempty"`
}

// ActPaperRecvResponse is the namespace-delta response for gs.actPaper.recv.
type ActPaperRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actPaper.recv. Request fields inferred from game.js: batchId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActPaperRPC) Recv(ctx context.Context, req ActPaperRecvRequest, opts ...RequestOption) (ActPaperRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActPaperRecv, req, opts...)
}

// ActPaperRecvGamePrizeRequest is the request body for gs.actPaper.recvGamePrize.
type ActPaperRecvGamePrizeRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActPaperRecvGamePrizeResponse is the namespace-delta response for gs.actPaper.recvGamePrize.
type ActPaperRecvGamePrizeResponse = RPCResponse[StateDelta]

// RecvGamePrize calls gs.actPaper.recvGamePrize. Request fields inferred from game.js: batchId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActPaperRPC) RecvGamePrize(ctx context.Context, req ActPaperRecvGamePrizeRequest, opts ...RequestOption) (ActPaperRecvGamePrizeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActPaperRecvGamePrize, req, opts...)
}

// ActPaperRecvTaskPrizeRequest is the request body for gs.actPaper.recvTaskPrize.
type ActPaperRecvTaskPrizeRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActPaperRecvTaskPrizeResponse is the namespace-delta response for gs.actPaper.recvTaskPrize.
type ActPaperRecvTaskPrizeResponse = RPCResponse[StateDelta]

// RecvTaskPrize calls gs.actPaper.recvTaskPrize. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActPaperRPC) RecvTaskPrize(ctx context.Context, req ActPaperRecvTaskPrizeRequest, opts ...RequestOption) (ActPaperRecvTaskPrizeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActPaperRecvTaskPrize, req, opts...)
}

// ActRchgRwd returns typed RPC helpers for the actRchgRwd namespace.
func (c *RPCClient) ActRchgRwd() ActRchgRwdRPC { return ActRchgRwdRPC{c: c} }

type ActRchgRwdRPC struct{ c *RPCClient }

// ActRchgRwdEnterRequest carries JSON fields for gs.actRchgRwd.enter; game.js did not expose a stable object literal for this request.
type ActRchgRwdEnterRequest RawRequest

// ActRchgRwdEnterResponse is the namespace-delta response for gs.actRchgRwd.enter.
type ActRchgRwdEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actRchgRwd.enter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRchgRwdRPC) Enter(ctx context.Context, req ActRchgRwdEnterRequest, opts ...RequestOption) (ActRchgRwdEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRchgRwdEnter, req, opts...)
}

// ActRchgRwdRecvRequest carries JSON fields for gs.actRchgRwd.recv; game.js did not expose a stable object literal for this request.
type ActRchgRwdRecvRequest RawRequest

// ActRchgRwdRecvResponse is the namespace-delta response for gs.actRchgRwd.recv.
type ActRchgRwdRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.actRchgRwd.recv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRchgRwdRPC) Recv(ctx context.Context, req ActRchgRwdRecvRequest, opts ...RequestOption) (ActRchgRwdRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRchgRwdRecv, req, opts...)
}

// ActRchgWheel returns typed RPC helpers for the actRchgWheel namespace.
func (c *RPCClient) ActRchgWheel() ActRchgWheelRPC { return ActRchgWheelRPC{c: c} }

type ActRchgWheelRPC struct{ c *RPCClient }

// ActRchgWheelEnterRequest is the request body for gs.actRchgWheel.enter.
type ActRchgWheelEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActRchgWheelEnterResponse is the namespace-delta response for gs.actRchgWheel.enter.
type ActRchgWheelEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actRchgWheel.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRchgWheelRPC) Enter(ctx context.Context, req ActRchgWheelEnterRequest, opts ...RequestOption) (ActRchgWheelEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRchgWheelEnter, req, opts...)
}

// ActRchgWheelGetMyLogRequest is the request body for gs.actRchgWheel.getMyLog.
type ActRchgWheelGetMyLogRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Index   RPCInt `json:"index,omitempty"`
	Num     RPCInt `json:"num,omitempty"`
}

// ActRchgWheelGetMyLogResponse is the namespace-delta response for gs.actRchgWheel.getMyLog.
type ActRchgWheelGetMyLogResponse = RPCResponse[StateDelta]

// GetMyLog calls gs.actRchgWheel.getMyLog. Request fields inferred from game.js: batchId, index, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRchgWheelRPC) GetMyLog(ctx context.Context, req ActRchgWheelGetMyLogRequest, opts ...RequestOption) (ActRchgWheelGetMyLogResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRchgWheelGetMyLog, req, opts...)
}

// ActRchgWheelStartWheelRequest is the request body for gs.actRchgWheel.startWheel.
type ActRchgWheelStartWheelRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DrawNum RPCInt `json:"drawNum,omitempty"`
}

// ActRchgWheelStartWheelResponse is the namespace-delta response for gs.actRchgWheel.startWheel.
type ActRchgWheelStartWheelResponse = RPCResponse[StateDelta]

// StartWheel calls gs.actRchgWheel.startWheel. Request fields inferred from game.js: batchId, drawNum.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActRchgWheelRPC) StartWheel(ctx context.Context, req ActRchgWheelStartWheelRequest, opts ...RequestOption) (ActRchgWheelStartWheelResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActRchgWheelStartWheel, req, opts...)
}

// ActSpool returns typed RPC helpers for the actSpool namespace.
func (c *RPCClient) ActSpool() ActSpoolRPC { return ActSpoolRPC{c: c} }

type ActSpoolRPC struct{ c *RPCClient }

// ActSpoolEnterRequest is the request body for gs.actSpool.enter.
type ActSpoolEnterRequest struct {
	BatchId  RPCID   `json:"batchId,omitempty"`
	IsInPage RPCBool `json:"isInPage,omitempty"`
}

// ActSpoolEnterResponse is the namespace-delta response for gs.actSpool.enter.
type ActSpoolEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.actSpool.enter. Request fields inferred from game.js: batchId, isInPage.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) Enter(ctx context.Context, req ActSpoolEnterRequest, opts ...RequestOption) (ActSpoolEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolEnter, req, opts...)
}

// ActSpoolGameOverRequest is the request body for gs.actSpool.gameOver.
type ActSpoolGameOverRequest struct {
	BatchId  RPCID  `json:"batchId,omitempty"`
	GameType RPCInt `json:"gameType,omitempty"`
}

// ActSpoolGameOverResponse is the namespace-delta response for gs.actSpool.gameOver.
type ActSpoolGameOverResponse = RPCResponse[StateDelta]

// GameOver calls gs.actSpool.gameOver. Request fields inferred from game.js: batchId, gameType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) GameOver(ctx context.Context, req ActSpoolGameOverRequest, opts ...RequestOption) (ActSpoolGameOverResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolGameOver, req, opts...)
}

// ActSpoolGameStartRequest is the request body for gs.actSpool.gameStart.
type ActSpoolGameStartRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActSpoolGameStartResponse is the namespace-delta response for gs.actSpool.gameStart.
type ActSpoolGameStartResponse = RPCResponse[StateDelta]

// GameStart calls gs.actSpool.gameStart. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) GameStart(ctx context.Context, req ActSpoolGameStartRequest, opts ...RequestOption) (ActSpoolGameStartResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolGameStart, req, opts...)
}

// ActSpoolGameSyncRequest is the request body for gs.actSpool.gameSync.
type ActSpoolGameSyncRequest struct {
	BatchId  RPCID     `json:"batchId,omitempty"`
	GameType RPCInt    `json:"gameType,omitempty"`
	Args     RPCObject `json:"args,omitempty"`
}

// ActSpoolGameSyncResponse is the namespace-delta response for gs.actSpool.gameSync.
type ActSpoolGameSyncResponse = RPCResponse[StateDelta]

// GameSync calls gs.actSpool.gameSync. Request fields inferred from game.js: batchId, gameType, args.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) GameSync(ctx context.Context, req ActSpoolGameSyncRequest, opts ...RequestOption) (ActSpoolGameSyncResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolGameSync, req, opts...)
}

// ActSpoolGiftBuyRequest is the request body for gs.actSpool.giftBuy.
type ActSpoolGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActSpoolGiftBuyResponse is the namespace-delta response for gs.actSpool.giftBuy.
type ActSpoolGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actSpool.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) GiftBuy(ctx context.Context, req ActSpoolGiftBuyRequest, opts ...RequestOption) (ActSpoolGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolGiftBuy, req, opts...)
}

// ActSpoolOpenBoxRequest is the request body for gs.actSpool.openBox.
type ActSpoolOpenBoxRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	Num     RPCInt `json:"num,omitempty"`
}

// ActSpoolOpenBoxResponse is the namespace-delta response for gs.actSpool.openBox.
type ActSpoolOpenBoxResponse = RPCResponse[StateDelta]

// OpenBox calls gs.actSpool.openBox. Request fields inferred from game.js: batchId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) OpenBox(ctx context.Context, req ActSpoolOpenBoxRequest, opts ...RequestOption) (ActSpoolOpenBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolOpenBox, req, opts...)
}

// ActSpoolRiseRequest is the request body for gs.actSpool.rise.
type ActSpoolRiseRequest struct {
	BatchId  RPCID  `json:"batchId,omitempty"`
	GameType RPCInt `json:"gameType,omitempty"`
}

// ActSpoolRiseResponse is the namespace-delta response for gs.actSpool.rise.
type ActSpoolRiseResponse = RPCResponse[StateDelta]

// Rise calls gs.actSpool.rise. Request fields inferred from game.js: batchId, gameType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) Rise(ctx context.Context, req ActSpoolRiseRequest, opts ...RequestOption) (ActSpoolRiseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolRise, req, opts...)
}

// ActSpoolSetGuideStatusRequest is the request body for gs.actSpool.setGuideStatus.
type ActSpoolSetGuideStatusRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActSpoolSetGuideStatusResponse is the namespace-delta response for gs.actSpool.setGuideStatus.
type ActSpoolSetGuideStatusResponse = RPCResponse[StateDelta]

// SetGuideStatus calls gs.actSpool.setGuideStatus. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpoolRPC) SetGuideStatus(ctx context.Context, req ActSpoolSetGuideStatusRequest, opts ...RequestOption) (ActSpoolSetGuideStatusResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpoolSetGuideStatus, req, opts...)
}

// ActSpringTotRchg returns typed RPC helpers for the actSpringTotRchg namespace.
func (c *RPCClient) ActSpringTotRchg() ActSpringTotRchgRPC { return ActSpringTotRchgRPC{c: c} }

type ActSpringTotRchgRPC struct{ c *RPCClient }

// ActSpringTotRchgRecvTLAwardRequest is the request body for gs.actSpringTotRchg.recvTLAward.
type ActSpringTotRchgRecvTLAwardRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	TaskId  RPCID `json:"taskId,omitempty"`
}

// ActSpringTotRchgRecvTLAwardResponse is the namespace-delta response for gs.actSpringTotRchg.recvTLAward.
type ActSpringTotRchgRecvTLAwardResponse = RPCResponse[StateDelta]

// RecvTLAward calls gs.actSpringTotRchg.recvTLAward. Request fields inferred from game.js: batchId, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActSpringTotRchgRPC) RecvTLAward(ctx context.Context, req ActSpringTotRchgRecvTLAwardRequest, opts ...RequestOption) (ActSpringTotRchgRecvTLAwardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActSpringTotRchgRecvTLAward, req, opts...)
}

// ActVipTimeShop returns typed RPC helpers for the actVipTimeShop namespace.
func (c *RPCClient) ActVipTimeShop() ActVipTimeShopRPC { return ActVipTimeShopRPC{c: c} }

type ActVipTimeShopRPC struct{ c *RPCClient }

// ActVipTimeShopGiftBuyRequest is the request body for gs.actVipTimeShop.giftBuy.
type ActVipTimeShopGiftBuyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	GiftId  RPCID  `json:"giftId,omitempty"`
	Count   RPCInt `json:"count,omitempty"`
}

// ActVipTimeShopGiftBuyResponse is the namespace-delta response for gs.actVipTimeShop.giftBuy.
type ActVipTimeShopGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.actVipTimeShop.giftBuy. Request fields inferred from game.js: batchId, giftId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActVipTimeShopRPC) GiftBuy(ctx context.Context, req ActVipTimeShopGiftBuyRequest, opts ...RequestOption) (ActVipTimeShopGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActVipTimeShopGiftBuy, req, opts...)
}

// ActZFBForest returns typed RPC helpers for the actZFBForest namespace.
func (c *RPCClient) ActZFBForest() ActZFBForestRPC { return ActZFBForestRPC{c: c} }

type ActZFBForestRPC struct{ c *RPCClient }

// ActZFBForestBrowseWebRequest is the request body for gs.actZFBForest.browseWeb.
type ActZFBForestBrowseWebRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActZFBForestBrowseWebResponse is the namespace-delta response for gs.actZFBForest.browseWeb.
type ActZFBForestBrowseWebResponse = RPCResponse[StateDelta]

// BrowseWeb calls gs.actZFBForest.browseWeb. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActZFBForestRPC) BrowseWeb(ctx context.Context, req ActZFBForestBrowseWebRequest, opts ...RequestOption) (ActZFBForestBrowseWebResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActZFBForestBrowseWeb, req, opts...)
}

// ActZFBForestBrowseWeb2Request is the request body for gs.actZFBForest.browseWeb2.
type ActZFBForestBrowseWeb2Request struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ActZFBForestBrowseWeb2Response is the namespace-delta response for gs.actZFBForest.browseWeb2.
type ActZFBForestBrowseWeb2Response = RPCResponse[StateDelta]

// BrowseWeb2 calls gs.actZFBForest.browseWeb2. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ActZFBForestRPC) BrowseWeb2(ctx context.Context, req ActZFBForestBrowseWeb2Request, opts ...RequestOption) (ActZFBForestBrowseWeb2Response, error) {
	return callRPC[StateDelta](ctx, r.c, RPCActZFBForestBrowseWeb2, req, opts...)
}

// Bag returns typed RPC helpers for the bag namespace.
func (c *RPCClient) Bag() BagRPC { return BagRPC{c: c} }

type BagRPC struct{ c *RPCClient }

// BagCombineRequest is the request body for gs.bag.combine.
type BagCombineRequest struct {
	Iid RPCID  `json:"iid,omitempty"`
	Num RPCInt `json:"num,omitempty"`
}

// BagCombineResponse is the namespace-delta response for gs.bag.combine.
type BagCombineResponse = RPCResponse[StateDelta]

// Combine calls gs.bag.combine. Request fields inferred from game.js: iid, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BagRPC) Combine(ctx context.Context, req BagCombineRequest, opts ...RequestOption) (BagCombineResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBagCombine, req, opts...)
}

// BagSellRequest is the request body for gs.bag.sell.
type BagSellRequest struct {
	IidMap RPCObject `json:"iidMap,omitempty"`
}

// BagSellResponse is the namespace-delta response for gs.bag.sell.
type BagSellResponse = RPCResponse[StateDelta]

// Sell calls gs.bag.sell. Request fields inferred from game.js: iidMap.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BagRPC) Sell(ctx context.Context, req BagSellRequest, opts ...RequestOption) (BagSellResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBagSell, req, opts...)
}

// BagUseRequest is the request body for gs.bag.use.
type BagUseRequest struct {
	Iid         RPCID   `json:"iid,omitempty"`
	Num         RPCInt  `json:"num,omitempty"`
	UseDstValue RPCBool `json:"useDstValue,omitempty"`
}

// BagUseResponse is the namespace-delta response for gs.bag.use.
type BagUseResponse = RPCResponse[StateDelta]

// Use calls gs.bag.use. Request fields inferred from game.js: iid, num, useDstValue.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BagRPC) Use(ctx context.Context, req BagUseRequest, opts ...RequestOption) (BagUseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBagUse, req, opts...)
}

// BattlePass returns typed RPC helpers for the battlePass namespace.
func (c *RPCClient) BattlePass() BattlePassRPC { return BattlePassRPC{c: c} }

type BattlePassRPC struct{ c *RPCClient }

// BattlePassBuyLvlRequest is the request body for gs.battlePass.buyLvl.
type BattlePassBuyLvlRequest struct {
	Bid   RPCInt `json:"bid,omitempty"`
	Count RPCInt `json:"count,omitempty"`
}

// BattlePassBuyLvlResponse is the namespace-delta response for gs.battlePass.buyLvl.
type BattlePassBuyLvlResponse = RPCResponse[StateDelta]

// BuyLvl calls gs.battlePass.buyLvl. Request fields inferred from game.js: bid, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BattlePassRPC) BuyLvl(ctx context.Context, req BattlePassBuyLvlRequest, opts ...RequestOption) (BattlePassBuyLvlResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBattlePassBuyLvl, req, opts...)
}

// BattlePassRecvRequest carries JSON fields for gs.battlePass.recv; game.js did not expose a stable object literal for this request.
type BattlePassRecvRequest RawRequest

// BattlePassRecvResponse is the namespace-delta response for gs.battlePass.recv.
type BattlePassRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.battlePass.recv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BattlePassRPC) Recv(ctx context.Context, req BattlePassRecvRequest, opts ...RequestOption) (BattlePassRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBattlePassRecv, req, opts...)
}

// BattlePassRecvAllRequest is the request body for gs.battlePass.recvAll.
type BattlePassRecvAllRequest struct {
	Bid RPCInt `json:"bid,omitempty"`
}

// BattlePassRecvAllResponse is the namespace-delta response for gs.battlePass.recvAll.
type BattlePassRecvAllResponse = RPCResponse[StateDelta]

// RecvAll calls gs.battlePass.recvAll. Request fields inferred from game.js: bid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BattlePassRPC) RecvAll(ctx context.Context, req BattlePassRecvAllRequest, opts ...RequestOption) (BattlePassRecvAllResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBattlePassRecvAll, req, opts...)
}

// BattlePassTaskDoneRequest is the request body for gs.battlePass.taskDone.
type BattlePassTaskDoneRequest struct {
	Bid    RPCInt `json:"bid,omitempty"`
	TaskId RPCID  `json:"taskId,omitempty"`
}

// BattlePassTaskDoneResponse is the namespace-delta response for gs.battlePass.taskDone.
type BattlePassTaskDoneResponse = RPCResponse[StateDelta]

// TaskDone calls gs.battlePass.taskDone. Request fields inferred from game.js: bid, taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BattlePassRPC) TaskDone(ctx context.Context, req BattlePassTaskDoneRequest, opts ...RequestOption) (BattlePassTaskDoneResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBattlePassTaskDone, req, opts...)
}

// BenefitBox returns typed RPC helpers for the benefitBox namespace.
func (c *RPCClient) BenefitBox() BenefitBoxRPC { return BenefitBoxRPC{c: c} }

type BenefitBoxRPC struct{ c *RPCClient }

// BenefitBoxDrawRequest is the empty request body for gs.benefitBox.draw.
type BenefitBoxDrawRequest struct{}

// BenefitBoxDrawResponse is the namespace-delta response for gs.benefitBox.draw.
type BenefitBoxDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.benefitBox.draw. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BenefitBoxRPC) Draw(ctx context.Context, req BenefitBoxDrawRequest, opts ...RequestOption) (BenefitBoxDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBenefitBoxDraw, req, opts...)
}

// Bestie returns typed RPC helpers for the bestie namespace.
func (c *RPCClient) Bestie() BestieRPC { return BestieRPC{c: c} }

type BestieRPC struct{ c *RPCClient }

// BestieApplyRequest is the request body for gs.bestie.apply.
type BestieApplyRequest struct {
	TargetUID RPCUID `json:"targetUid,omitempty"`
}

// BestieApplyResponse is the namespace-delta response for gs.bestie.apply.
type BestieApplyResponse = RPCResponse[StateDelta]

// Apply calls gs.bestie.apply. Request fields inferred from game.js: targetUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) Apply(ctx context.Context, req BestieApplyRequest, opts ...RequestOption) (BestieApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieApply, req, opts...)
}

// BestieCancelDissolveRequest is the request body for gs.bestie.cancelDissolve.
type BestieCancelDissolveRequest struct {
	BestieUID RPCUID `json:"bestieUid,omitempty"`
}

// BestieCancelDissolveResponse is the namespace-delta response for gs.bestie.cancelDissolve.
type BestieCancelDissolveResponse = RPCResponse[StateDelta]

// CancelDissolve calls gs.bestie.cancelDissolve. Request fields inferred from game.js: bestieUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) CancelDissolve(ctx context.Context, req BestieCancelDissolveRequest, opts ...RequestOption) (BestieCancelDissolveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieCancelDissolve, req, opts...)
}

// BestieCheckApplyRequest is the request body for gs.bestie.checkApply.
type BestieCheckApplyRequest struct {
	TargetUID RPCUID `json:"targetUid,omitempty"`
}

// BestieCheckApplyResponse is the namespace-delta response for gs.bestie.checkApply.
type BestieCheckApplyResponse = RPCResponse[StateDelta]

// CheckApply calls gs.bestie.checkApply. Request fields inferred from game.js: targetUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) CheckApply(ctx context.Context, req BestieCheckApplyRequest, opts ...RequestOption) (BestieCheckApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieCheckApply, req, opts...)
}

// BestieDissolveRequest is the request body for gs.bestie.dissolve.
type BestieDissolveRequest struct {
	BestieUID RPCUID `json:"bestieUid,omitempty"`
}

// BestieDissolveResponse is the namespace-delta response for gs.bestie.dissolve.
type BestieDissolveResponse = RPCResponse[StateDelta]

// Dissolve calls gs.bestie.dissolve. Request fields inferred from game.js: bestieUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) Dissolve(ctx context.Context, req BestieDissolveRequest, opts ...RequestOption) (BestieDissolveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieDissolve, req, opts...)
}

// BestieEnterRequest is the empty request body for gs.bestie.enter.
type BestieEnterRequest struct{}

// BestieEnterResponse is the namespace-delta response for gs.bestie.enter.
type BestieEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.bestie.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) Enter(ctx context.Context, req BestieEnterRequest, opts ...RequestOption) (BestieEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieEnter, req, opts...)
}

// BestieGetFrdBestieCntMapRequest is the empty request body for gs.bestie.getFrdBestieCntMap.
type BestieGetFrdBestieCntMapRequest struct{}

// BestieGetFrdBestieCntMapResponse is the namespace-delta response for gs.bestie.getFrdBestieCntMap.
type BestieGetFrdBestieCntMapResponse = RPCResponse[StateDelta]

// GetFrdBestieCntMap calls gs.bestie.getFrdBestieCntMap. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) GetFrdBestieCntMap(ctx context.Context, req BestieGetFrdBestieCntMapRequest, opts ...RequestOption) (BestieGetFrdBestieCntMapResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieGetFrdBestieCntMap, req, opts...)
}

// BestieHandleApplyRequest is the request body for gs.bestie.handleApply.
type BestieHandleApplyRequest struct {
	ApplyUID RPCUID  `json:"applyUid,omitempty"`
	Accept   RPCBool `json:"accept,omitempty"`
}

// BestieHandleApplyResponse is the namespace-delta response for gs.bestie.handleApply.
type BestieHandleApplyResponse = RPCResponse[StateDelta]

// HandleApply calls gs.bestie.handleApply. Request fields inferred from game.js: applyUid, accept.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) HandleApply(ctx context.Context, req BestieHandleApplyRequest, opts ...RequestOption) (BestieHandleApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieHandleApply, req, opts...)
}

// BestieImmediateDissolveRequest is the request body for gs.bestie.immediateDissolve.
type BestieImmediateDissolveRequest struct {
	BestieUID RPCUID `json:"bestieUid,omitempty"`
}

// BestieImmediateDissolveResponse is the namespace-delta response for gs.bestie.immediateDissolve.
type BestieImmediateDissolveResponse = RPCResponse[StateDelta]

// ImmediateDissolve calls gs.bestie.immediateDissolve. Request fields inferred from game.js: bestieUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) ImmediateDissolve(ctx context.Context, req BestieImmediateDissolveRequest, opts ...RequestOption) (BestieImmediateDissolveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieImmediateDissolve, req, opts...)
}

// BestieSetSceneSkinRequest is the request body for gs.bestie.setSceneSkin.
type BestieSetSceneSkinRequest struct {
	BestieUID RPCUID `json:"bestieUid,omitempty"`
	SkinId    RPCInt `json:"skinId,omitempty"`
}

// BestieSetSceneSkinResponse is the namespace-delta response for gs.bestie.setSceneSkin.
type BestieSetSceneSkinResponse = RPCResponse[StateDelta]

// SetSceneSkin calls gs.bestie.setSceneSkin. Request fields inferred from game.js: bestieUid, skinId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) SetSceneSkin(ctx context.Context, req BestieSetSceneSkinRequest, opts ...RequestOption) (BestieSetSceneSkinResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieSetSceneSkin, req, opts...)
}

// BestieUnlockSlotRequest is the empty request body for gs.bestie.unlockSlot.
type BestieUnlockSlotRequest struct{}

// BestieUnlockSlotResponse is the namespace-delta response for gs.bestie.unlockSlot.
type BestieUnlockSlotResponse = RPCResponse[StateDelta]

// UnlockSlot calls gs.bestie.unlockSlot. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BestieRPC) UnlockSlot(ctx context.Context, req BestieUnlockSlotRequest, opts ...RequestOption) (BestieUnlockSlotResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBestieUnlockSlot, req, opts...)
}

// Boost returns typed RPC helpers for the boost namespace.
func (c *RPCClient) Boost() BoostRPC { return BoostRPC{c: c} }

type BoostRPC struct{ c *RPCClient }

// BoostRecvRwdRequest is the request body for gs.boost.recvRwd.
type BoostRecvRwdRequest struct {
	Type  RPCInt `json:"type,omitempty"`
	Idx   RPCInt `json:"idx,omitempty"`
	BoxId RPCID  `json:"boxId,omitempty"`
}

// BoostRecvRwdResponse is the namespace-delta response for gs.boost.recvRwd.
type BoostRecvRwdResponse = RPCResponse[StateDelta]

// RecvRwd calls gs.boost.recvRwd. Request fields inferred from game.js: type, idx, boxId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BoostRPC) RecvRwd(ctx context.Context, req BoostRecvRwdRequest, opts ...RequestOption) (BoostRecvRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBoostRecvRwd, req, opts...)
}

// BoostRefreshRequest is the request body for gs.boost.refresh.
type BoostRefreshRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// BoostRefreshResponse is the namespace-delta response for gs.boost.refresh.
type BoostRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.boost.refresh. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BoostRPC) Refresh(ctx context.Context, req BoostRefreshRequest, opts ...RequestOption) (BoostRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBoostRefresh, req, opts...)
}

// Bubble returns typed RPC helpers for the bubble namespace.
func (c *RPCClient) Bubble() BubbleRPC { return BubbleRPC{c: c} }

type BubbleRPC struct{ c *RPCClient }

// BubbleActiveBubbleRequest is the request body for gs.bubble.activeBubble.
type BubbleActiveBubbleRequest struct {
	BubbleId RPCID `json:"bubbleId,omitempty"`
}

// BubbleActiveBubbleResponse is the namespace-delta response for gs.bubble.activeBubble.
type BubbleActiveBubbleResponse = RPCResponse[StateDelta]

// ActiveBubble calls gs.bubble.activeBubble. Request fields inferred from game.js: bubbleId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BubbleRPC) ActiveBubble(ctx context.Context, req BubbleActiveBubbleRequest, opts ...RequestOption) (BubbleActiveBubbleResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBubbleActiveBubble, req, opts...)
}

// BubbleChgBubbleRequest is the request body for gs.bubble.chgBubble.
type BubbleChgBubbleRequest struct {
	BubbleId RPCID `json:"bubbleId,omitempty"`
}

// BubbleChgBubbleResponse is the namespace-delta response for gs.bubble.chgBubble.
type BubbleChgBubbleResponse = RPCResponse[StateDelta]

// ChgBubble calls gs.bubble.chgBubble. Request fields inferred from game.js: bubbleId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r BubbleRPC) ChgBubble(ctx context.Context, req BubbleChgBubbleRequest, opts ...RequestOption) (BubbleChgBubbleResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCBubbleChgBubble, req, opts...)
}

// CallFriend returns typed RPC helpers for the callFriend namespace.
func (c *RPCClient) CallFriend() CallFriendRPC { return CallFriendRPC{c: c} }

type CallFriendRPC struct{ c *RPCClient }

// CallFriendEnterRequest is the empty request body for gs.callFriend.enter.
type CallFriendEnterRequest struct{}

// CallFriendEnterResponse is the namespace-delta response for gs.callFriend.enter.
type CallFriendEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.callFriend.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CallFriendRPC) Enter(ctx context.Context, req CallFriendEnterRequest, opts ...RequestOption) (CallFriendEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCallFriendEnter, req, opts...)
}

// CallFriendRecvRequest is the request body for gs.callFriend.recv.
type CallFriendRecvRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// CallFriendRecvResponse is the namespace-delta response for gs.callFriend.recv.
type CallFriendRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.callFriend.recv. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CallFriendRPC) Recv(ctx context.Context, req CallFriendRecvRequest, opts ...RequestOption) (CallFriendRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCallFriendRecv, req, opts...)
}

// CallFriendUseCodeRequest is the request body for gs.callFriend.useCode.
type CallFriendUseCodeRequest struct {
	Code RPCString `json:"code,omitempty"`
}

// CallFriendUseCodeResponse is the namespace-delta response for gs.callFriend.useCode.
type CallFriendUseCodeResponse = RPCResponse[StateDelta]

// UseCode calls gs.callFriend.useCode. Request fields inferred from game.js: code.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CallFriendRPC) UseCode(ctx context.Context, req CallFriendUseCodeRequest, opts ...RequestOption) (CallFriendUseCodeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCallFriendUseCode, req, opts...)
}

// Celebrity returns typed RPC helpers for the celebrity namespace.
func (c *RPCClient) Celebrity() CelebrityRPC { return CelebrityRPC{c: c} }

type CelebrityRPC struct{ c *RPCClient }

// CelebrityGetAllTypesRequest is the empty request body for gs.celebrity.getAllTypes.
type CelebrityGetAllTypesRequest struct{}

// CelebrityGetAllTypesResponse is the namespace-delta response for gs.celebrity.getAllTypes.
type CelebrityGetAllTypesResponse = RPCResponse[StateDelta]

// GetAllTypes calls gs.celebrity.getAllTypes. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CelebrityRPC) GetAllTypes(ctx context.Context, req CelebrityGetAllTypesRequest, opts ...RequestOption) (CelebrityGetAllTypesResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCelebrityGetAllTypes, req, opts...)
}

// CelebrityGetAllTypesInfoRequest is the empty request body for gs.celebrity.getAllTypesInfo.
type CelebrityGetAllTypesInfoRequest struct{}

// CelebrityGetAllTypesInfoResponse is the namespace-delta response for gs.celebrity.getAllTypesInfo.
type CelebrityGetAllTypesInfoResponse = RPCResponse[StateDelta]

// GetAllTypesInfo calls gs.celebrity.getAllTypesInfo. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CelebrityRPC) GetAllTypesInfo(ctx context.Context, req CelebrityGetAllTypesInfoRequest, opts ...RequestOption) (CelebrityGetAllTypesInfoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCelebrityGetAllTypesInfo, req, opts...)
}

// CelebrityGetInfoByTypeRequest is the request body for gs.celebrity.getInfoByType.
type CelebrityGetInfoByTypeRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// CelebrityGetInfoByTypeResponse is the namespace-delta response for gs.celebrity.getInfoByType.
type CelebrityGetInfoByTypeResponse = RPCResponse[StateDelta]

// GetInfoByType calls gs.celebrity.getInfoByType. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CelebrityRPC) GetInfoByType(ctx context.Context, req CelebrityGetInfoByTypeRequest, opts ...RequestOption) (CelebrityGetInfoByTypeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCelebrityGetInfoByType, req, opts...)
}

// CelebrityLikeCelebrityRequest is the request body for gs.celebrity.likeCelebrity.
type CelebrityLikeCelebrityRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// CelebrityLikeCelebrityResponse is the namespace-delta response for gs.celebrity.likeCelebrity.
type CelebrityLikeCelebrityResponse = RPCResponse[StateDelta]

// LikeCelebrity calls gs.celebrity.likeCelebrity. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CelebrityRPC) LikeCelebrity(ctx context.Context, req CelebrityLikeCelebrityRequest, opts ...RequestOption) (CelebrityLikeCelebrityResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCelebrityLikeCelebrity, req, opts...)
}

// ChannelRwd returns typed RPC helpers for the channelRwd namespace.
func (c *RPCClient) ChannelRwd() ChannelRwdRPC { return ChannelRwdRPC{c: c} }

type ChannelRwdRPC struct{ c *RPCClient }

// ChannelRwdRecvDailyDesktopRwdRequest is the empty request body for gs.channelRwd.recvDailyDesktopRwd.
type ChannelRwdRecvDailyDesktopRwdRequest struct{}

// ChannelRwdRecvDailyDesktopRwdResponse is the namespace-delta response for gs.channelRwd.recvDailyDesktopRwd.
type ChannelRwdRecvDailyDesktopRwdResponse = RPCResponse[StateDelta]

// RecvDailyDesktopRwd calls gs.channelRwd.recvDailyDesktopRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ChannelRwdRPC) RecvDailyDesktopRwd(ctx context.Context, req ChannelRwdRecvDailyDesktopRwdRequest, opts ...RequestOption) (ChannelRwdRecvDailyDesktopRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCChannelRwdRecvDailyDesktopRwd, req, opts...)
}

// ChannelRwdRecvFstDesktopRwdRequest is the empty request body for gs.channelRwd.recvFstDesktopRwd.
type ChannelRwdRecvFstDesktopRwdRequest struct{}

// ChannelRwdRecvFstDesktopRwdResponse is the namespace-delta response for gs.channelRwd.recvFstDesktopRwd.
type ChannelRwdRecvFstDesktopRwdResponse = RPCResponse[StateDelta]

// RecvFstDesktopRwd calls gs.channelRwd.recvFstDesktopRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ChannelRwdRPC) RecvFstDesktopRwd(ctx context.Context, req ChannelRwdRecvFstDesktopRwdRequest, opts ...RequestOption) (ChannelRwdRecvFstDesktopRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCChannelRwdRecvFstDesktopRwd, req, opts...)
}

// ChannelRwdRecvFstSidebarRwdRequest is the empty request body for gs.channelRwd.recvFstSidebarRwd.
type ChannelRwdRecvFstSidebarRwdRequest struct{}

// ChannelRwdRecvFstSidebarRwdResponse is the namespace-delta response for gs.channelRwd.recvFstSidebarRwd.
type ChannelRwdRecvFstSidebarRwdResponse = RPCResponse[StateDelta]

// RecvFstSidebarRwd calls gs.channelRwd.recvFstSidebarRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ChannelRwdRPC) RecvFstSidebarRwd(ctx context.Context, req ChannelRwdRecvFstSidebarRwdRequest, opts ...RequestOption) (ChannelRwdRecvFstSidebarRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCChannelRwdRecvFstSidebarRwd, req, opts...)
}

// ChannelRwdRecvLoginRwdRequest is the request body for gs.channelRwd.recvLoginRwd.
type ChannelRwdRecvLoginRwdRequest struct {
	Day RPCInt `json:"day,omitempty"`
}

// ChannelRwdRecvLoginRwdResponse is the namespace-delta response for gs.channelRwd.recvLoginRwd.
type ChannelRwdRecvLoginRwdResponse = RPCResponse[StateDelta]

// RecvLoginRwd calls gs.channelRwd.recvLoginRwd. Request fields inferred from game.js: day.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ChannelRwdRPC) RecvLoginRwd(ctx context.Context, req ChannelRwdRecvLoginRwdRequest, opts ...RequestOption) (ChannelRwdRecvLoginRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCChannelRwdRecvLoginRwd, req, opts...)
}

// Cheater returns typed RPC helpers for the cheater namespace.
func (c *RPCClient) Cheater() CheaterRPC { return CheaterRPC{c: c} }

type CheaterRPC struct{ c *RPCClient }

// CheaterDoCheatRequest is the request body for gs.cheater.doCheat.
type CheaterDoCheatRequest struct {
	Sl RPCInt `json:"sl,omitempty"`
}

// CheaterDoCheatResponse is the namespace-delta response for gs.cheater.doCheat.
type CheaterDoCheatResponse = RPCResponse[StateDelta]

// DoCheat calls gs.cheater.doCheat. Request fields inferred from game.js: sl.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CheaterRPC) DoCheat(ctx context.Context, req CheaterDoCheatRequest, opts ...RequestOption) (CheaterDoCheatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCheaterDoCheat, req, opts...)
}

// CollectRwd returns typed RPC helpers for the collectRwd namespace.
func (c *RPCClient) CollectRwd() CollectRwdRPC { return CollectRwdRPC{c: c} }

type CollectRwdRPC struct{ c *RPCClient }

// CollectRwdRecvRequest is the request body for gs.collectRwd.recv.
type CollectRwdRecvRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// CollectRwdRecvResponse is the namespace-delta response for gs.collectRwd.recv.
type CollectRwdRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.collectRwd.recv. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CollectRwdRPC) Recv(ctx context.Context, req CollectRwdRecvRequest, opts ...RequestOption) (CollectRwdRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCollectRwdRecv, req, opts...)
}

// CollectRwdRecvArtCreateRwdRequest is the request body for gs.collectRwd.recvArtCreateRwd.
type CollectRwdRecvArtCreateRwdRequest struct {
	FlowerArtId RPCID `json:"flowerArtId,omitempty"`
}

// CollectRwdRecvArtCreateRwdResponse is the namespace-delta response for gs.collectRwd.recvArtCreateRwd.
type CollectRwdRecvArtCreateRwdResponse = RPCResponse[StateDelta]

// RecvArtCreateRwd calls gs.collectRwd.recvArtCreateRwd. Request fields inferred from game.js: flowerArtId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CollectRwdRPC) RecvArtCreateRwd(ctx context.Context, req CollectRwdRecvArtCreateRwdRequest, opts ...RequestOption) (CollectRwdRecvArtCreateRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCollectRwdRecvArtCreateRwd, req, opts...)
}

// CollectRwdRecvArtCreateRwdByVaseRequest is the request body for gs.collectRwd.recvArtCreateRwdByVase.
type CollectRwdRecvArtCreateRwdByVaseRequest struct {
	FlowerArtId RPCID `json:"flowerArtId,omitempty"`
}

// CollectRwdRecvArtCreateRwdByVaseResponse is the namespace-delta response for gs.collectRwd.recvArtCreateRwdByVase.
type CollectRwdRecvArtCreateRwdByVaseResponse = RPCResponse[StateDelta]

// RecvArtCreateRwdByVase calls gs.collectRwd.recvArtCreateRwdByVase. Request fields inferred from game.js: flowerArtId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CollectRwdRPC) RecvArtCreateRwdByVase(ctx context.Context, req CollectRwdRecvArtCreateRwdByVaseRequest, opts ...RequestOption) (CollectRwdRecvArtCreateRwdByVaseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCollectRwdRecvArtCreateByVase, req, opts...)
}

// Cultivate returns typed RPC helpers for the cultivate namespace.
func (c *RPCClient) Cultivate() CultivateRPC { return CultivateRPC{c: c} }

type CultivateRPC struct{ c *RPCClient }

// CultivateChooseSkillRequest is the request body for gs.cultivate.chooseSkill.
type CultivateChooseSkillRequest struct {
	FlowerId   RPCID  `json:"flowerId,omitempty"`
	SlotId     RPCID  `json:"slotId,omitempty"`
	ChooseType RPCInt `json:"chooseType,omitempty"`
}

// CultivateChooseSkillResponse is the namespace-delta response for gs.cultivate.chooseSkill.
type CultivateChooseSkillResponse = RPCResponse[StateDelta]

// ChooseSkill calls gs.cultivate.chooseSkill. Request fields inferred from game.js: flowerId, slotId, chooseType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) ChooseSkill(ctx context.Context, req CultivateChooseSkillRequest, opts ...RequestOption) (CultivateChooseSkillResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateChooseSkill, req, opts...)
}

// CultivateClearCulCdRequest is the request body for gs.cultivate.clearCulCd.
type CultivateClearCulCdRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// CultivateClearCulCdResponse is the namespace-delta response for gs.cultivate.clearCulCd.
type CultivateClearCulCdResponse = RPCResponse[StateDelta]

// ClearCulCd calls gs.cultivate.clearCulCd. Request fields inferred from game.js: flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) ClearCulCd(ctx context.Context, req CultivateClearCulCdRequest, opts ...RequestOption) (CultivateClearCulCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateClearCulCD, req, opts...)
}

// CultivateCultivateRequest is the request body for gs.cultivate.cultivate.
type CultivateCultivateRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// CultivateCultivateResponse is the namespace-delta response for gs.cultivate.cultivate.
type CultivateCultivateResponse = RPCResponse[StateDelta]

// Cultivate calls gs.cultivate.cultivate. Request fields inferred from game.js: flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) Cultivate(ctx context.Context, req CultivateCultivateRequest, opts ...RequestOption) (CultivateCultivateResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateCultivate, req, opts...)
}

// CultivateRandomSkillRequest is the request body for gs.cultivate.randomSkill.
type CultivateRandomSkillRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
	SlotId   RPCID `json:"slotId,omitempty"`
}

// CultivateRandomSkillResponse is the namespace-delta response for gs.cultivate.randomSkill.
type CultivateRandomSkillResponse = RPCResponse[StateDelta]

// RandomSkill calls gs.cultivate.randomSkill. Request fields inferred from game.js: flowerId, slotId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) RandomSkill(ctx context.Context, req CultivateRandomSkillRequest, opts ...RequestOption) (CultivateRandomSkillResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateRandomSkill, req, opts...)
}

// CultivateRecvRequest is the request body for gs.cultivate.recv.
type CultivateRecvRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// CultivateRecvResponse is the namespace-delta response for gs.cultivate.recv.
type CultivateRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.cultivate.recv. Request fields inferred from game.js: flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) Recv(ctx context.Context, req CultivateRecvRequest, opts ...RequestOption) (CultivateRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateRecv, req, opts...)
}

// CultivateReduceByHelpRequest is the request body for gs.cultivate.reduceByHelp.
type CultivateReduceByHelpRequest struct {
	FlowerId RPCID  `json:"flowerId,omitempty"`
	HelpUID  RPCInt `json:"helpUid,omitempty"`
}

// CultivateReduceByHelpResponse is the namespace-delta response for gs.cultivate.reduceByHelp.
type CultivateReduceByHelpResponse = RPCResponse[StateDelta]

// ReduceByHelp calls gs.cultivate.reduceByHelp. Request fields inferred from game.js: flowerId, helpUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) ReduceByHelp(ctx context.Context, req CultivateReduceByHelpRequest, opts ...RequestOption) (CultivateReduceByHelpResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateReduceByHelp, req, opts...)
}

// CultivateReduceByItemRequest carries JSON fields for gs.cultivate.reduceByItem; game.js did not expose a stable object literal for this request.
type CultivateReduceByItemRequest RawRequest

// CultivateReduceByItemResponse is the namespace-delta response for gs.cultivate.reduceByItem.
type CultivateReduceByItemResponse = RPCResponse[StateDelta]

// ReduceByItem calls gs.cultivate.reduceByItem. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) ReduceByItem(ctx context.Context, req CultivateReduceByItemRequest, opts ...RequestOption) (CultivateReduceByItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateReduceByItem, req, opts...)
}

// CultivateUnlockSlotRequest is the request body for gs.cultivate.unlockSlot.
type CultivateUnlockSlotRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
	SlotId   RPCID `json:"slotId,omitempty"`
}

// CultivateUnlockSlotResponse is the namespace-delta response for gs.cultivate.unlockSlot.
type CultivateUnlockSlotResponse = RPCResponse[StateDelta]

// UnlockSlot calls gs.cultivate.unlockSlot. Request fields inferred from game.js: flowerId, slotId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) UnlockSlot(ctx context.Context, req CultivateUnlockSlotRequest, opts ...RequestOption) (CultivateUnlockSlotResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateUnlockSlot, req, opts...)
}

// CultivateUpgradeRequest is the request body for gs.cultivate.upgrade.
type CultivateUpgradeRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// CultivateUpgradeResponse is the namespace-delta response for gs.cultivate.upgrade.
type CultivateUpgradeResponse = RPCResponse[StateDelta]

// Upgrade calls gs.cultivate.upgrade. Request fields inferred from game.js: flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CultivateRPC) Upgrade(ctx context.Context, req CultivateUpgradeRequest, opts ...RequestOption) (CultivateUpgradeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCultivateUpgrade, req, opts...)
}

// CustomerOrderRqst returns typed RPC helpers for the customerOrderRqst namespace.
func (c *RPCClient) CustomerOrderRqst() CustomerOrderRqstRPC { return CustomerOrderRqstRPC{c: c} }

type CustomerOrderRqstRPC struct{ c *RPCClient }

// CustomerOrderRqstDkgkckRequest is the request body for gs.customerOrderRqst.dkgkck.
type CustomerOrderRqstDkgkckRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// CustomerOrderRqstDkgkckResponse is the namespace-delta response for gs.customerOrderRqst.dkgkck.
type CustomerOrderRqstDkgkckResponse = RPCResponse[StateDelta]

// Dkgkck calls gs.customerOrderRqst.dkgkck. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r CustomerOrderRqstRPC) Dkgkck(ctx context.Context, req CustomerOrderRqstDkgkckRequest, opts ...RequestOption) (CustomerOrderRqstDkgkckResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCCustomerOrderRqstDkgkck, req, opts...)
}

// Decorate returns typed RPC helpers for the decorate namespace.
func (c *RPCClient) Decorate() DecorateRPC { return DecorateRPC{c: c} }

type DecorateRPC struct{ c *RPCClient }

// DecorateBuildRequest carries JSON fields for gs.decorate.build; game.js did not expose a stable object literal for this request.
type DecorateBuildRequest RawRequest

// DecorateBuildResponse is the namespace-delta response for gs.decorate.build.
type DecorateBuildResponse = RPCResponse[StateDelta]

// Build calls gs.decorate.build. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DecorateRPC) Build(ctx context.Context, req DecorateBuildRequest, opts ...RequestOption) (DecorateBuildResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDecorateBuild, req, opts...)
}

// DecorateBuildSuccessRequest carries JSON fields for gs.decorate.buildSuccess; game.js did not expose a stable object literal for this request.
type DecorateBuildSuccessRequest RawRequest

// DecorateBuildSuccessResponse is the namespace-delta response for gs.decorate.buildSuccess.
type DecorateBuildSuccessResponse = RPCResponse[StateDelta]

// BuildSuccess calls gs.decorate.buildSuccess. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DecorateRPC) BuildSuccess(ctx context.Context, req DecorateBuildSuccessRequest, opts ...RequestOption) (DecorateBuildSuccessResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDecorateBuildSuccess, req, opts...)
}

// DecorateClearBuildCdRequest carries JSON fields for gs.decorate.clearBuildCd; game.js did not expose a stable object literal for this request.
type DecorateClearBuildCdRequest RawRequest

// DecorateClearBuildCdResponse is the namespace-delta response for gs.decorate.clearBuildCd.
type DecorateClearBuildCdResponse = RPCResponse[StateDelta]

// ClearBuildCd calls gs.decorate.clearBuildCd. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DecorateRPC) ClearBuildCd(ctx context.Context, req DecorateClearBuildCdRequest, opts ...RequestOption) (DecorateClearBuildCdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDecorateClearBuildCd, req, opts...)
}

// DecorateEquipRequest carries JSON fields for gs.decorate.equip; game.js did not expose a stable object literal for this request.
type DecorateEquipRequest RawRequest

// DecorateEquipResponse is the namespace-delta response for gs.decorate.equip.
type DecorateEquipResponse = RPCResponse[StateDelta]

// Equip calls gs.decorate.equip. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DecorateRPC) Equip(ctx context.Context, req DecorateEquipRequest, opts ...RequestOption) (DecorateEquipResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDecorateEquip, req, opts...)
}

// DecorateRecvRequest carries JSON fields for gs.decorate.recv; game.js did not expose a stable object literal for this request.
type DecorateRecvRequest RawRequest

// DecorateRecvResponse is the namespace-delta response for gs.decorate.recv.
type DecorateRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.decorate.recv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DecorateRPC) Recv(ctx context.Context, req DecorateRecvRequest, opts ...RequestOption) (DecorateRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDecorateRecv, req, opts...)
}

// DecorateUpdateReadLvlListRequest carries JSON fields for gs.decorate.updateReadLvlList; game.js did not expose a stable object literal for this request.
type DecorateUpdateReadLvlListRequest RawRequest

// DecorateUpdateReadLvlListResponse is the namespace-delta response for gs.decorate.updateReadLvlList.
type DecorateUpdateReadLvlListResponse = RPCResponse[StateDelta]

// UpdateReadLvlList calls gs.decorate.updateReadLvlList. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DecorateRPC) UpdateReadLvlList(ctx context.Context, req DecorateUpdateReadLvlListRequest, opts ...RequestOption) (DecorateUpdateReadLvlListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDecorateUpdateReadLvlList, req, opts...)
}

// Draw returns typed RPC helpers for the draw namespace.
func (c *RPCClient) Draw() DrawRPC { return DrawRPC{c: c} }

type DrawRPC struct{ c *RPCClient }

// DrawDrawRequest is the request body for gs.draw.draw.
type DrawDrawRequest struct {
	Num RPCInt `json:"num,omitempty"`
}

// DrawDrawResponse is the namespace-delta response for gs.draw.draw.
type DrawDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.draw.draw. Request fields inferred from game.js: num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DrawRPC) Draw(ctx context.Context, req DrawDrawRequest, opts ...RequestOption) (DrawDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDrawDraw, req, opts...)
}

// DrawTestDrawVirtualRequest is the request body for gs.draw.testDrawVirtual.
type DrawTestDrawVirtualRequest struct {
	Num RPCInt `json:"num,omitempty"`
}

// DrawTestDrawVirtualResponse is the namespace-delta response for gs.draw.testDrawVirtual.
type DrawTestDrawVirtualResponse = RPCResponse[StateDelta]

// TestDrawVirtual calls gs.draw.testDrawVirtual. Request fields inferred from game.js: num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r DrawRPC) TestDrawVirtual(ctx context.Context, req DrawTestDrawVirtualRequest, opts ...RequestOption) (DrawTestDrawVirtualResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCDrawTestDrawVirtual, req, opts...)
}

// Fashion returns typed RPC helpers for the fashion namespace.
func (c *RPCClient) Fashion() FashionRPC { return FashionRPC{c: c} }

type FashionRPC struct{ c *RPCClient }

// FashionEquipRequest carries JSON fields for gs.fashion.equip; game.js did not expose a stable object literal for this request.
type FashionEquipRequest RawRequest

// FashionEquipResponse is the namespace-delta response for gs.fashion.equip.
type FashionEquipResponse = RPCResponse[StateDelta]

// Equip calls gs.fashion.equip. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FashionRPC) Equip(ctx context.Context, req FashionEquipRequest, opts ...RequestOption) (FashionEquipResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFashionEquip, req, opts...)
}

// FashionReadRequest carries JSON fields for gs.fashion.read; game.js did not expose a stable object literal for this request.
type FashionReadRequest RawRequest

// FashionReadResponse is the namespace-delta response for gs.fashion.read.
type FashionReadResponse = RPCResponse[StateDelta]

// Read calls gs.fashion.read. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FashionRPC) Read(ctx context.Context, req FashionReadRequest, opts ...RequestOption) (FashionReadResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFashionRead, req, opts...)
}

// FlowerArt returns typed RPC helpers for the flowerArt namespace.
func (c *RPCClient) FlowerArt() FlowerArtRPC { return FlowerArtRPC{c: c} }

type FlowerArtRPC struct{ c *RPCClient }

// FlowerArtMakeFlowerArtRequest is the request body for gs.flowerArt.makeFlowerArt.
type FlowerArtMakeFlowerArtRequest struct {
	VaseId     RPCID     `json:"vaseId,omitempty"`
	FlowersIds RPCIDList `json:"flowersIds,omitempty"`
	Num        RPCInt    `json:"num,omitempty"`
}

// FlowerArtMakeFlowerArtResponse is the namespace-delta response for gs.flowerArt.makeFlowerArt.
type FlowerArtMakeFlowerArtResponse = RPCResponse[StateDelta]

// MakeFlowerArt calls gs.flowerArt.makeFlowerArt. Request fields inferred from game.js: vaseId, flowersIds, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerArtRPC) MakeFlowerArt(ctx context.Context, req FlowerArtMakeFlowerArtRequest, opts ...RequestOption) (FlowerArtMakeFlowerArtResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerArtMakeFlowerArt, req, opts...)
}

// FlowerElves returns typed RPC helpers for the flowerElves namespace.
func (c *RPCClient) FlowerElves() FlowerElvesRPC { return FlowerElvesRPC{c: c} }

type FlowerElvesRPC struct{ c *RPCClient }

// FlowerElvesCheckConvertRequest is the empty request body for gs.flowerElves.checkConvert.
type FlowerElvesCheckConvertRequest struct{}

// FlowerElvesCheckConvertResponse is the namespace-delta response for gs.flowerElves.checkConvert.
type FlowerElvesCheckConvertResponse = RPCResponse[StateDelta]

// CheckConvert calls gs.flowerElves.checkConvert. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesRPC) CheckConvert(ctx context.Context, req FlowerElvesCheckConvertRequest, opts ...RequestOption) (FlowerElvesCheckConvertResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesCheckConvert, req, opts...)
}

// FlowerElvesAid returns typed RPC helpers for the flowerElvesAid namespace.
func (c *RPCClient) FlowerElvesAid() FlowerElvesAidRPC { return FlowerElvesAidRPC{c: c} }

type FlowerElvesAidRPC struct{ c *RPCClient }

// FlowerElvesAidHelpFrdRequest is the request body for gs.flowerElvesAid.helpFrd.
type FlowerElvesAidHelpFrdRequest struct {
	DstUID RPCUID `json:"dstUid,omitempty"`
}

// FlowerElvesAidHelpFrdResponse is the namespace-delta response for gs.flowerElvesAid.helpFrd.
type FlowerElvesAidHelpFrdResponse = RPCResponse[StateDelta]

// HelpFrd calls gs.flowerElvesAid.helpFrd. Request fields inferred from game.js: dstUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesAidRPC) HelpFrd(ctx context.Context, req FlowerElvesAidHelpFrdRequest, opts ...RequestOption) (FlowerElvesAidHelpFrdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesAidHelpFrd, req, opts...)
}

// FlowerElvesAidRecvAidEffRequest is the empty request body for gs.flowerElvesAid.recvAidEff.
type FlowerElvesAidRecvAidEffRequest struct{}

// FlowerElvesAidRecvAidEffResponse is the namespace-delta response for gs.flowerElvesAid.recvAidEff.
type FlowerElvesAidRecvAidEffResponse = RPCResponse[StateDelta]

// RecvAidEff calls gs.flowerElvesAid.recvAidEff. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesAidRPC) RecvAidEff(ctx context.Context, req FlowerElvesAidRecvAidEffRequest, opts ...RequestOption) (FlowerElvesAidRecvAidEffResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesAidRecvAidEff, req, opts...)
}

// FlowerElvesAidReqAidRequest is the empty request body for gs.flowerElvesAid.reqAid.
type FlowerElvesAidReqAidRequest struct{}

// FlowerElvesAidReqAidResponse is the namespace-delta response for gs.flowerElvesAid.reqAid.
type FlowerElvesAidReqAidResponse = RPCResponse[StateDelta]

// ReqAid calls gs.flowerElvesAid.reqAid. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesAidRPC) ReqAid(ctx context.Context, req FlowerElvesAidReqAidRequest, opts ...RequestOption) (FlowerElvesAidReqAidResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesAidReqAid, req, opts...)
}

// FlowerElvesBook returns typed RPC helpers for the flowerElvesBook namespace.
func (c *RPCClient) FlowerElvesBook() FlowerElvesBookRPC { return FlowerElvesBookRPC{c: c} }

type FlowerElvesBookRPC struct{ c *RPCClient }

// FlowerElvesBookUpgradeRequest is the request body for gs.flowerElvesBook.upgrade.
type FlowerElvesBookUpgradeRequest struct {
	BookId RPCID `json:"bookId,omitempty"`
}

// FlowerElvesBookUpgradeResponse is the namespace-delta response for gs.flowerElvesBook.upgrade.
type FlowerElvesBookUpgradeResponse = RPCResponse[StateDelta]

// Upgrade calls gs.flowerElvesBook.upgrade. Request fields inferred from game.js: bookId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesBookRPC) Upgrade(ctx context.Context, req FlowerElvesBookUpgradeRequest, opts ...RequestOption) (FlowerElvesBookUpgradeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesBookUpgrade, req, opts...)
}

// FlowerElvesBookDraw returns typed RPC helpers for the flowerElvesBookDraw namespace.
func (c *RPCClient) FlowerElvesBookDraw() FlowerElvesBookDrawRPC { return FlowerElvesBookDrawRPC{c: c} }

type FlowerElvesBookDrawRPC struct{ c *RPCClient }

// FlowerElvesBookDrawDrawRequest is the request body for gs.flowerElvesBookDraw.draw.
type FlowerElvesBookDrawDrawRequest struct {
	PeriodId RPCID  `json:"periodId,omitempty"`
	GridPos  RPCInt `json:"gridPos,omitempty"`
}

// FlowerElvesBookDrawDrawResponse is the namespace-delta response for gs.flowerElvesBookDraw.draw.
type FlowerElvesBookDrawDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.flowerElvesBookDraw.draw. Request fields inferred from game.js: periodId, gridPos.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesBookDrawRPC) Draw(ctx context.Context, req FlowerElvesBookDrawDrawRequest, opts ...RequestOption) (FlowerElvesBookDrawDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesBookDrawDraw, req, opts...)
}

// FlowerElvesBookDrawRefreshRequest is the request body for gs.flowerElvesBookDraw.refresh.
type FlowerElvesBookDrawRefreshRequest struct {
	PeriodId RPCID `json:"periodId,omitempty"`
}

// FlowerElvesBookDrawRefreshResponse is the namespace-delta response for gs.flowerElvesBookDraw.refresh.
type FlowerElvesBookDrawRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.flowerElvesBookDraw.refresh. Request fields inferred from game.js: periodId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesBookDrawRPC) Refresh(ctx context.Context, req FlowerElvesBookDrawRefreshRequest, opts ...RequestOption) (FlowerElvesBookDrawRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesBookDrawRefresh, req, opts...)
}

// FlowerElvesPlace returns typed RPC helpers for the flowerElvesPlace namespace.
func (c *RPCClient) FlowerElvesPlace() FlowerElvesPlaceRPC { return FlowerElvesPlaceRPC{c: c} }

type FlowerElvesPlaceRPC struct{ c *RPCClient }

// FlowerElvesPlaceDispatchRequest is the request body for gs.flowerElvesPlace.dispatch.
type FlowerElvesPlaceDispatchRequest struct {
	PlaceId  RPCID  `json:"placeId,omitempty"`
	ElvesId  RPCID  `json:"elvesId,omitempty"`
	ElvesNum RPCInt `json:"elvesNum,omitempty"`
	Iid      RPCID  `json:"iid,omitempty"`
}

// FlowerElvesPlaceDispatchResponse is the namespace-delta response for gs.flowerElvesPlace.dispatch.
type FlowerElvesPlaceDispatchResponse = RPCResponse[StateDelta]

// Dispatch calls gs.flowerElvesPlace.dispatch. Request fields inferred from game.js: placeId, elvesId, elvesNum, iid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesPlaceRPC) Dispatch(ctx context.Context, req FlowerElvesPlaceDispatchRequest, opts ...RequestOption) (FlowerElvesPlaceDispatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesPlaceDispatch, req, opts...)
}

// FlowerElvesPlaceRecvRequest is the request body for gs.flowerElvesPlace.recv.
type FlowerElvesPlaceRecvRequest struct {
	PlaceId RPCID `json:"placeId,omitempty"`
}

// FlowerElvesPlaceRecvResponse is the namespace-delta response for gs.flowerElvesPlace.recv.
type FlowerElvesPlaceRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.flowerElvesPlace.recv. Request fields inferred from game.js: placeId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesPlaceRPC) Recv(ctx context.Context, req FlowerElvesPlaceRecvRequest, opts ...RequestOption) (FlowerElvesPlaceRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesPlaceRecv, req, opts...)
}

// FlowerElvesPlaceRecvAllRewardRequest is the empty request body for gs.flowerElvesPlace.recvAllReward.
type FlowerElvesPlaceRecvAllRewardRequest struct{}

// FlowerElvesPlaceRecvAllRewardResponse is the namespace-delta response for gs.flowerElvesPlace.recvAllReward.
type FlowerElvesPlaceRecvAllRewardResponse = RPCResponse[StateDelta]

// RecvAllReward calls gs.flowerElvesPlace.recvAllReward. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesPlaceRPC) RecvAllReward(ctx context.Context, req FlowerElvesPlaceRecvAllRewardRequest, opts ...RequestOption) (FlowerElvesPlaceRecvAllRewardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesPlaceRecvAllReward, req, opts...)
}

// FlowerElvesPlaceSpeedUpRequest is the request body for gs.flowerElvesPlace.speedUp.
type FlowerElvesPlaceSpeedUpRequest struct {
	PlaceId RPCID `json:"placeId,omitempty"`
}

// FlowerElvesPlaceSpeedUpResponse is the namespace-delta response for gs.flowerElvesPlace.speedUp.
type FlowerElvesPlaceSpeedUpResponse = RPCResponse[StateDelta]

// SpeedUp calls gs.flowerElvesPlace.speedUp. Request fields inferred from game.js: placeId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesPlaceRPC) SpeedUp(ctx context.Context, req FlowerElvesPlaceSpeedUpRequest, opts ...RequestOption) (FlowerElvesPlaceSpeedUpResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesPlaceSpeedUp, req, opts...)
}

// FlowerElvesPlaceUnlockRequest is the request body for gs.flowerElvesPlace.unlock.
type FlowerElvesPlaceUnlockRequest struct {
	PlaceId RPCID `json:"placeId,omitempty"`
}

// FlowerElvesPlaceUnlockResponse is the namespace-delta response for gs.flowerElvesPlace.unlock.
type FlowerElvesPlaceUnlockResponse = RPCResponse[StateDelta]

// Unlock calls gs.flowerElvesPlace.unlock. Request fields inferred from game.js: placeId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerElvesPlaceRPC) Unlock(ctx context.Context, req FlowerElvesPlaceUnlockRequest, opts ...RequestOption) (FlowerElvesPlaceUnlockResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerElvesPlaceUnlock, req, opts...)
}

// FlowerGift returns typed RPC helpers for the flowerGift namespace.
func (c *RPCClient) FlowerGift() FlowerGiftRPC { return FlowerGiftRPC{c: c} }

type FlowerGiftRPC struct{ c *RPCClient }

// FlowerGiftRecvBoxRequest is the request body for gs.flowerGift.recvBox.
type FlowerGiftRecvBoxRequest struct {
	PageId RPCID  `json:"pageId,omitempty"`
	Idx    RPCInt `json:"idx,omitempty"`
}

// FlowerGiftRecvBoxResponse is the namespace-delta response for gs.flowerGift.recvBox.
type FlowerGiftRecvBoxResponse = RPCResponse[StateDelta]

// RecvBox calls gs.flowerGift.recvBox. Request fields inferred from game.js: pageId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerGiftRPC) RecvBox(ctx context.Context, req FlowerGiftRecvBoxRequest, opts ...RequestOption) (FlowerGiftRecvBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerGiftRecvBox, req, opts...)
}

// FlowerMarket returns typed RPC helpers for the flowerMarket namespace.
func (c *RPCClient) FlowerMarket() FlowerMarketRPC { return FlowerMarketRPC{c: c} }

type FlowerMarketRPC struct{ c *RPCClient }

// FlowerMarketBuyFlowerRequest is the request body for gs.flowerMarket.buyFlower.
type FlowerMarketBuyFlowerRequest struct {
	SellerUID RPCUID    `json:"sellerUid,omitempty"`
	ShelfId   RPCID     `json:"shelfId,omitempty"`
	Flower    RPCValue  `json:"flower,omitempty"`
	Password  RPCString `json:"password,omitempty"`
}

// FlowerMarketBuyFlowerResponse is the namespace-delta response for gs.flowerMarket.buyFlower.
type FlowerMarketBuyFlowerResponse = RPCResponse[StateDelta]

// BuyFlower calls gs.flowerMarket.buyFlower. Request fields inferred from game.js: sellerUid, shelfId, flower, password.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) BuyFlower(ctx context.Context, req FlowerMarketBuyFlowerRequest, opts ...RequestOption) (FlowerMarketBuyFlowerResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketBuyFlower, req, opts...)
}

// FlowerMarketBuyPutCountRequest is the request body for gs.flowerMarket.buyPutCount.
type FlowerMarketBuyPutCountRequest struct {
	Count RPCInt `json:"count,omitempty"`
}

// FlowerMarketBuyPutCountResponse is the namespace-delta response for gs.flowerMarket.buyPutCount.
type FlowerMarketBuyPutCountResponse = RPCResponse[StateDelta]

// BuyPutCount calls gs.flowerMarket.buyPutCount. Request fields inferred from game.js: count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) BuyPutCount(ctx context.Context, req FlowerMarketBuyPutCountRequest, opts ...RequestOption) (FlowerMarketBuyPutCountResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketBuyPutCount, req, opts...)
}

// FlowerMarketCheckPasswordRequest is the request body for gs.flowerMarket.checkPassword.
type FlowerMarketCheckPasswordRequest struct {
	SellerUID RPCUID    `json:"sellerUid,omitempty"`
	ShelfId   RPCID     `json:"shelfId,omitempty"`
	Password  RPCString `json:"password,omitempty"`
}

// FlowerMarketCheckPasswordResponse is the namespace-delta response for gs.flowerMarket.checkPassword.
type FlowerMarketCheckPasswordResponse = RPCResponse[StateDelta]

// CheckPassword calls gs.flowerMarket.checkPassword. Request fields inferred from game.js: sellerUid, shelfId, password.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) CheckPassword(ctx context.Context, req FlowerMarketCheckPasswordRequest, opts ...RequestOption) (FlowerMarketCheckPasswordResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketCheckPassword, req, opts...)
}

// FlowerMarketEnterRequest is the empty request body for gs.flowerMarket.enter.
type FlowerMarketEnterRequest struct{}

// FlowerMarketEnterResponse is the namespace-delta response for gs.flowerMarket.enter.
type FlowerMarketEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.flowerMarket.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) Enter(ctx context.Context, req FlowerMarketEnterRequest, opts ...RequestOption) (FlowerMarketEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketEnter, req, opts...)
}

// FlowerMarketGetFriendRequest is the request body for gs.flowerMarket.getFriend.
type FlowerMarketGetFriendRequest struct {
	FrdUID RPCUID `json:"frdUid,omitempty"`
}

// FlowerMarketGetFriendResponse is the namespace-delta response for gs.flowerMarket.getFriend.
type FlowerMarketGetFriendResponse = RPCResponse[StateDelta]

// GetFriend calls gs.flowerMarket.getFriend. Request fields inferred from game.js: frdUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) GetFriend(ctx context.Context, req FlowerMarketGetFriendRequest, opts ...RequestOption) (FlowerMarketGetFriendResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketGetFriend, req, opts...)
}

// FlowerMarketGetFriendListRequest is the request body for gs.flowerMarket.getFriendList.
type FlowerMarketGetFriendListRequest struct {
	SpecificFriendIds RPCIDList `json:"specificFriendIds,omitempty"`
}

// FlowerMarketGetFriendListResponse is the namespace-delta response for gs.flowerMarket.getFriendList.
type FlowerMarketGetFriendListResponse = RPCResponse[StateDelta]

// GetFriendList calls gs.flowerMarket.getFriendList. Request fields inferred from game.js: specificFriendIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) GetFriendList(ctx context.Context, req FlowerMarketGetFriendListRequest, opts ...RequestOption) (FlowerMarketGetFriendListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketGetFriendList, req, opts...)
}

// FlowerMarketGetTradeRecordsRequest is the empty request body for gs.flowerMarket.getTradeRecords.
type FlowerMarketGetTradeRecordsRequest struct{}

// FlowerMarketGetTradeRecordsResponse is the namespace-delta response for gs.flowerMarket.getTradeRecords.
type FlowerMarketGetTradeRecordsResponse = RPCResponse[StateDelta]

// GetTradeRecords calls gs.flowerMarket.getTradeRecords. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) GetTradeRecords(ctx context.Context, req FlowerMarketGetTradeRecordsRequest, opts ...RequestOption) (FlowerMarketGetTradeRecordsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketGetTradeRecords, req, opts...)
}

// FlowerMarketHarvestIncomeRequest is the empty request body for gs.flowerMarket.harvestIncome.
type FlowerMarketHarvestIncomeRequest struct{}

// FlowerMarketHarvestIncomeResponse is the namespace-delta response for gs.flowerMarket.harvestIncome.
type FlowerMarketHarvestIncomeResponse = RPCResponse[StateDelta]

// HarvestIncome calls gs.flowerMarket.harvestIncome. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) HarvestIncome(ctx context.Context, req FlowerMarketHarvestIncomeRequest, opts ...RequestOption) (FlowerMarketHarvestIncomeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketHarvestIncome, req, opts...)
}

// FlowerMarketPutFlowerRequest is the request body for gs.flowerMarket.putFlower.
type FlowerMarketPutFlowerRequest struct {
	ShelfId  RPCID     `json:"shelfId,omitempty"`
	FlowerId RPCID     `json:"flowerId,omitempty"`
	Count    RPCInt    `json:"count,omitempty"`
	PriceIdx RPCID     `json:"priceIdx,omitempty"`
	Password RPCString `json:"password,omitempty"`
}

// FlowerMarketPutFlowerResponse is the namespace-delta response for gs.flowerMarket.putFlower.
type FlowerMarketPutFlowerResponse = RPCResponse[StateDelta]

// PutFlower calls gs.flowerMarket.putFlower. Request fields inferred from game.js: shelfId, flowerId, count, priceIdx, password.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) PutFlower(ctx context.Context, req FlowerMarketPutFlowerRequest, opts ...RequestOption) (FlowerMarketPutFlowerResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketPutFlower, req, opts...)
}

// FlowerMarketPutFlowerBatchRequest is the request body for gs.flowerMarket.putFlowerBatch.
type FlowerMarketPutFlowerBatchRequest struct {
	ShelfIds RPCIDList `json:"shelfIds,omitempty"`
	FlowerId RPCID     `json:"flowerId,omitempty"`
	Count    RPCInt    `json:"count,omitempty"`
	PriceIdx RPCID     `json:"priceIdx,omitempty"`
	Password RPCString `json:"password,omitempty"`
}

// FlowerMarketPutFlowerBatchResponse is the namespace-delta response for gs.flowerMarket.putFlowerBatch.
type FlowerMarketPutFlowerBatchResponse = RPCResponse[StateDelta]

// PutFlowerBatch calls gs.flowerMarket.putFlowerBatch. Request fields inferred from game.js: shelfIds, flowerId, count, priceIdx, password.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) PutFlowerBatch(ctx context.Context, req FlowerMarketPutFlowerBatchRequest, opts ...RequestOption) (FlowerMarketPutFlowerBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketPutFlowerBatch, req, opts...)
}

// FlowerMarketTakeDownFlowerRequest is the request body for gs.flowerMarket.takeDownFlower.
type FlowerMarketTakeDownFlowerRequest struct {
	ShelfId RPCID `json:"shelfId,omitempty"`
}

// FlowerMarketTakeDownFlowerResponse is the namespace-delta response for gs.flowerMarket.takeDownFlower.
type FlowerMarketTakeDownFlowerResponse = RPCResponse[StateDelta]

// TakeDownFlower calls gs.flowerMarket.takeDownFlower. Request fields inferred from game.js: shelfId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) TakeDownFlower(ctx context.Context, req FlowerMarketTakeDownFlowerRequest, opts ...RequestOption) (FlowerMarketTakeDownFlowerResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketTakeDownFlower, req, opts...)
}

// FlowerMarketUnlockShelfRequest is the request body for gs.flowerMarket.unlockShelf.
type FlowerMarketUnlockShelfRequest struct {
	ShelfId RPCID `json:"shelfId,omitempty"`
}

// FlowerMarketUnlockShelfResponse is the namespace-delta response for gs.flowerMarket.unlockShelf.
type FlowerMarketUnlockShelfResponse = RPCResponse[StateDelta]

// UnlockShelf calls gs.flowerMarket.unlockShelf. Request fields inferred from game.js: shelfId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerMarketRPC) UnlockShelf(ctx context.Context, req FlowerMarketUnlockShelfRequest, opts ...RequestOption) (FlowerMarketUnlockShelfResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerMarketUnlockShelf, req, opts...)
}

// FlowerOrderRqst returns typed RPC helpers for the flowerOrderRqst namespace.
func (c *RPCClient) FlowerOrderRqst() FlowerOrderRqstRPC { return FlowerOrderRqstRPC{c: c} }

type FlowerOrderRqstRPC struct{ c *RPCClient }

// FlowerOrderRqstShowRRequest is the request body for gs.flowerOrderRqst.showR.
type FlowerOrderRqstShowRRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// FlowerOrderRqstShowRResponse is the namespace-delta response for gs.flowerOrderRqst.showR.
type FlowerOrderRqstShowRResponse = RPCResponse[StateDelta]

// ShowR calls gs.flowerOrderRqst.showR. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerOrderRqstRPC) ShowR(ctx context.Context, req FlowerOrderRqstShowRRequest, opts ...RequestOption) (FlowerOrderRqstShowRResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerOrderRqstShowR, req, opts...)
}

// FlowerRack returns typed RPC helpers for the flowerRack namespace.
func (c *RPCClient) FlowerRack() FlowerRackRPC { return FlowerRackRPC{c: c} }

type FlowerRackRPC struct{ c *RPCClient }

// FlowerRackCancelSellRequest is the request body for gs.flowerRack.cancelSell.
type FlowerRackCancelSellRequest struct {
	RackId RPCID `json:"rackId,omitempty"`
}

// FlowerRackCancelSellResponse is the namespace-delta response for gs.flowerRack.cancelSell.
type FlowerRackCancelSellResponse = RPCResponse[StateDelta]

// CancelSell calls gs.flowerRack.cancelSell. Request fields inferred from game.js: rackId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerRackRPC) CancelSell(ctx context.Context, req FlowerRackCancelSellRequest, opts ...RequestOption) (FlowerRackCancelSellResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerRackCancelSell, req, opts...)
}

// FlowerRackRecvOneKeyRequest is the request body for gs.flowerRack.recvOneKey.
type FlowerRackRecvOneKeyRequest struct {
	StandId RPCID `json:"standId,omitempty"`
}

// FlowerRackRecvOneKeyResponse is the namespace-delta response for gs.flowerRack.recvOneKey.
type FlowerRackRecvOneKeyResponse = RPCResponse[StateDelta]

// RecvOneKey calls gs.flowerRack.recvOneKey. Request fields inferred from game.js: standId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerRackRPC) RecvOneKey(ctx context.Context, req FlowerRackRecvOneKeyRequest, opts ...RequestOption) (FlowerRackRecvOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerRackRecvOneKey, req, opts...)
}

// FlowerRackRecvSellMoneyRequest is the request body for gs.flowerRack.recvSellMoney.
type FlowerRackRecvSellMoneyRequest struct {
	RackId RPCID `json:"rackId,omitempty"`
}

// FlowerRackRecvSellMoneyResponse is the namespace-delta response for gs.flowerRack.recvSellMoney.
type FlowerRackRecvSellMoneyResponse = RPCResponse[StateDelta]

// RecvSellMoney calls gs.flowerRack.recvSellMoney. Request fields inferred from game.js: rackId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerRackRPC) RecvSellMoney(ctx context.Context, req FlowerRackRecvSellMoneyRequest, opts ...RequestOption) (FlowerRackRecvSellMoneyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerRackRecvSellMoney, req, opts...)
}

// FlowerRackSellRequest is the request body for gs.flowerRack.sell.
type FlowerRackSellRequest struct {
	RackId RPCID  `json:"rackId,omitempty"`
	Iid    RPCID  `json:"iid,omitempty"`
	Num    RPCInt `json:"num,omitempty"`
}

// FlowerRackSellResponse is the namespace-delta response for gs.flowerRack.sell.
type FlowerRackSellResponse = RPCResponse[StateDelta]

// Sell calls gs.flowerRack.sell. Request fields inferred from game.js: rackId, iid, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerRackRPC) Sell(ctx context.Context, req FlowerRackSellRequest, opts ...RequestOption) (FlowerRackSellResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerRackSell, req, opts...)
}

// FlowerRackUnlockStandRequest is the request body for gs.flowerRack.unlockStand.
type FlowerRackUnlockStandRequest struct {
	StandId RPCID `json:"standId,omitempty"`
}

// FlowerRackUnlockStandResponse is the namespace-delta response for gs.flowerRack.unlockStand.
type FlowerRackUnlockStandResponse = RPCResponse[StateDelta]

// UnlockStand calls gs.flowerRack.unlockStand. Request fields inferred from game.js: standId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FlowerRackRPC) UnlockStand(ctx context.Context, req FlowerRackUnlockStandRequest, opts ...RequestOption) (FlowerRackUnlockStandResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFlowerRackUnlockStand, req, opts...)
}

// Fml returns typed RPC helpers for the fml namespace.
func (c *RPCClient) Fml() FmlRPC { return FmlRPC{c: c} }

type FmlRPC struct{ c *RPCClient }

// FmlAutoJoinRequest is the empty request body for gs.fml.autoJoin.
type FmlAutoJoinRequest struct{}

// FmlAutoJoinResponse is the namespace-delta response for gs.fml.autoJoin.
type FmlAutoJoinResponse = RPCResponse[StateDelta]

// AutoJoin calls gs.fml.autoJoin. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) AutoJoin(ctx context.Context, req FmlAutoJoinRequest, opts ...RequestOption) (FmlAutoJoinResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlAutoJoin, req, opts...)
}

// FmlBldRequest carries JSON fields for gs.fml.bld; game.js did not expose a stable object literal for this request.
type FmlBldRequest RawRequest

// FmlBldResponse is the namespace-delta response for gs.fml.bld.
type FmlBldResponse = RPCResponse[StateDelta]

// Bld calls gs.fml.bld. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Bld(ctx context.Context, req FmlBldRequest, opts ...RequestOption) (FmlBldResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlBld, req, opts...)
}

// FmlBuyRaceBoatRequest is the request body for gs.fml.buyRaceBoat.
type FmlBuyRaceBoatRequest struct {
	BoatId RPCID `json:"boatId,omitempty"`
}

// FmlBuyRaceBoatResponse is the namespace-delta response for gs.fml.buyRaceBoat.
type FmlBuyRaceBoatResponse = RPCResponse[StateDelta]

// BuyRaceBoat calls gs.fml.buyRaceBoat. Request fields inferred from game.js: boatId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) BuyRaceBoat(ctx context.Context, req FmlBuyRaceBoatRequest, opts ...RequestOption) (FmlBuyRaceBoatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlBuyRaceBoat, req, opts...)
}

// FmlChgPosRequest carries JSON fields for gs.fml.chgPos; game.js did not expose a stable object literal for this request.
type FmlChgPosRequest RawRequest

// FmlChgPosResponse is the namespace-delta response for gs.fml.chgPos.
type FmlChgPosResponse = RPCResponse[StateDelta]

// ChgPos calls gs.fml.chgPos. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) ChgPos(ctx context.Context, req FmlChgPosRequest, opts ...RequestOption) (FmlChgPosResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlChgPos, req, opts...)
}

// FmlChgTitleRequest is the request body for gs.fml.chgTitle.
type FmlChgTitleRequest struct {
	TitleId RPCID `json:"titleId,omitempty"`
}

// FmlChgTitleResponse is the namespace-delta response for gs.fml.chgTitle.
type FmlChgTitleResponse = RPCResponse[StateDelta]

// ChgTitle calls gs.fml.chgTitle. Request fields inferred from game.js: titleId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) ChgTitle(ctx context.Context, req FmlChgTitleRequest, opts ...RequestOption) (FmlChgTitleResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlChgTitle, req, opts...)
}

// FmlClearQuitTimeRequest is the empty request body for gs.fml.clearQuitTime.
type FmlClearQuitTimeRequest struct{}

// FmlClearQuitTimeResponse is the namespace-delta response for gs.fml.clearQuitTime.
type FmlClearQuitTimeResponse = RPCResponse[StateDelta]

// ClearQuitTime calls gs.fml.clearQuitTime. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) ClearQuitTime(ctx context.Context, req FmlClearQuitTimeRequest, opts ...RequestOption) (FmlClearQuitTimeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlClearQuitTime, req, opts...)
}

// FmlCreateRequest carries JSON fields for gs.fml.create; game.js did not expose a stable object literal for this request.
type FmlCreateRequest RawRequest

// FmlCreateResponse is the namespace-delta response for gs.fml.create.
type FmlCreateResponse = RPCResponse[StateDelta]

// Create calls gs.fml.create. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Create(ctx context.Context, req FmlCreateRequest, opts ...RequestOption) (FmlCreateResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlCreate, req, opts...)
}

// FmlDissolveRequest carries JSON fields for gs.fml.dissolve; game.js did not expose a stable object literal for this request.
type FmlDissolveRequest RawRequest

// FmlDissolveResponse is the namespace-delta response for gs.fml.dissolve.
type FmlDissolveResponse = RPCResponse[StateDelta]

// Dissolve calls gs.fml.dissolve. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Dissolve(ctx context.Context, req FmlDissolveRequest, opts ...RequestOption) (FmlDissolveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlDissolve, req, opts...)
}

// FmlEnterRequest carries JSON fields for gs.fml.enter; game.js did not expose a stable object literal for this request.
type FmlEnterRequest RawRequest

// FmlEnterResponse is the namespace-delta response for gs.fml.enter.
type FmlEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.fml.enter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Enter(ctx context.Context, req FmlEnterRequest, opts ...RequestOption) (FmlEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlEnter, req, opts...)
}

// FmlEquipRaceBoatRequest is the request body for gs.fml.equipRaceBoat.
type FmlEquipRaceBoatRequest struct {
	BoatId RPCID  `json:"boatId,omitempty"`
	Idx    RPCInt `json:"idx,omitempty"`
}

// FmlEquipRaceBoatResponse is the namespace-delta response for gs.fml.equipRaceBoat.
type FmlEquipRaceBoatResponse = RPCResponse[StateDelta]

// EquipRaceBoat calls gs.fml.equipRaceBoat. Request fields inferred from game.js: boatId, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) EquipRaceBoat(ctx context.Context, req FmlEquipRaceBoatRequest, opts ...RequestOption) (FmlEquipRaceBoatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlEquipRaceBoat, req, opts...)
}

// FmlGetHonorRequest is the empty request body for gs.fml.getHonor.
type FmlGetHonorRequest struct{}

// FmlGetHonorResponse is the namespace-delta response for gs.fml.getHonor.
type FmlGetHonorResponse = RPCResponse[StateDelta]

// GetHonor calls gs.fml.getHonor. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) GetHonor(ctx context.Context, req FmlGetHonorRequest, opts ...RequestOption) (FmlGetHonorResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlGetHonor, req, opts...)
}

// FmlGetLogRequest is the request body for gs.fml.getLog.
type FmlGetLogRequest struct {
	Fid RPCID `json:"fid,omitempty"`
}

// FmlGetLogResponse is the namespace-delta response for gs.fml.getLog.
type FmlGetLogResponse = RPCResponse[StateDelta]

// GetLog calls gs.fml.getLog. Request fields inferred from game.js: fid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) GetLog(ctx context.Context, req FmlGetLogRequest, opts ...RequestOption) (FmlGetLogResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlGetLog, req, opts...)
}

// FmlGetMedalRwdRequest is the request body for gs.fml.getMedalRwd.
type FmlGetMedalRwdRequest struct {
	MedalId RPCID `json:"medalId,omitempty"`
}

// FmlGetMedalRwdResponse is the namespace-delta response for gs.fml.getMedalRwd.
type FmlGetMedalRwdResponse = RPCResponse[StateDelta]

// GetMedalRwd calls gs.fml.getMedalRwd. Request fields inferred from game.js: medalId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) GetMedalRwd(ctx context.Context, req FmlGetMedalRwdRequest, opts ...RequestOption) (FmlGetMedalRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlGetMedalRwd, req, opts...)
}

// FmlGetRecFmlListRequest is the empty request body for gs.fml.getRecFmlList.
type FmlGetRecFmlListRequest struct{}

// FmlGetRecFmlListResponse is the namespace-delta response for gs.fml.getRecFmlList.
type FmlGetRecFmlListResponse = RPCResponse[StateDelta]

// GetRecFmlList calls gs.fml.getRecFmlList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) GetRecFmlList(ctx context.Context, req FmlGetRecFmlListRequest, opts ...RequestOption) (FmlGetRecFmlListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlGetRecFmlList, req, opts...)
}

// FmlGetTitleLogListRequest is the request body for gs.fml.getTitleLogList.
type FmlGetTitleLogListRequest struct {
	TitleId RPCID `json:"titleId,omitempty"`
}

// FmlGetTitleLogListResponse is the namespace-delta response for gs.fml.getTitleLogList.
type FmlGetTitleLogListResponse = RPCResponse[StateDelta]

// GetTitleLogList calls gs.fml.getTitleLogList. Request fields inferred from game.js: titleId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) GetTitleLogList(ctx context.Context, req FmlGetTitleLogListRequest, opts ...RequestOption) (FmlGetTitleLogListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlGetTitleLogList, req, opts...)
}

// FmlHandleApplyRequest carries JSON fields for gs.fml.handleApply; game.js did not expose a stable object literal for this request.
type FmlHandleApplyRequest RawRequest

// FmlHandleApplyResponse is the namespace-delta response for gs.fml.handleApply.
type FmlHandleApplyResponse = RPCResponse[StateDelta]

// HandleApply calls gs.fml.handleApply. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) HandleApply(ctx context.Context, req FmlHandleApplyRequest, opts ...RequestOption) (FmlHandleApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlHandleApply, req, opts...)
}

// FmlHandleApplyAllRequest carries JSON fields for gs.fml.handleApplyAll; game.js did not expose a stable object literal for this request.
type FmlHandleApplyAllRequest RawRequest

// FmlHandleApplyAllResponse is the namespace-delta response for gs.fml.handleApplyAll.
type FmlHandleApplyAllResponse = RPCResponse[StateDelta]

// HandleApplyAll calls gs.fml.handleApplyAll. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) HandleApplyAll(ctx context.Context, req FmlHandleApplyAllRequest, opts ...RequestOption) (FmlHandleApplyAllResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlHandleApplyAll, req, opts...)
}

// FmlHandleInvRequest carries JSON fields for gs.fml.handleInv; game.js did not expose a stable object literal for this request.
type FmlHandleInvRequest RawRequest

// FmlHandleInvResponse is the namespace-delta response for gs.fml.handleInv.
type FmlHandleInvResponse = RPCResponse[StateDelta]

// HandleInv calls gs.fml.handleInv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) HandleInv(ctx context.Context, req FmlHandleInvRequest, opts ...RequestOption) (FmlHandleInvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlHandleInv, req, opts...)
}

// FmlInvRequest carries JSON fields for gs.fml.inv; game.js did not expose a stable object literal for this request.
type FmlInvRequest RawRequest

// FmlInvResponse is the namespace-delta response for gs.fml.inv.
type FmlInvResponse = RPCResponse[StateDelta]

// Inv calls gs.fml.inv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Inv(ctx context.Context, req FmlInvRequest, opts ...RequestOption) (FmlInvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlInv, req, opts...)
}

// FmlJoinRequest carries JSON fields for gs.fml.join; game.js did not expose a stable object literal for this request.
type FmlJoinRequest RawRequest

// FmlJoinResponse is the namespace-delta response for gs.fml.join.
type FmlJoinResponse = RPCResponse[StateDelta]

// Join calls gs.fml.join. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Join(ctx context.Context, req FmlJoinRequest, opts ...RequestOption) (FmlJoinResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlJoin, req, opts...)
}

// FmlKickRequest carries JSON fields for gs.fml.kick; game.js did not expose a stable object literal for this request.
type FmlKickRequest RawRequest

// FmlKickResponse is the namespace-delta response for gs.fml.kick.
type FmlKickResponse = RPCResponse[StateDelta]

// Kick calls gs.fml.kick. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Kick(ctx context.Context, req FmlKickRequest, opts ...RequestOption) (FmlKickResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlKick, req, opts...)
}

// FmlOpenFmlRaceBoxRequest is the request body for gs.fml.openFmlRaceBox.
type FmlOpenFmlRaceBoxRequest struct {
	IsAll RPCBool `json:"isAll,omitempty"`
}

// FmlOpenFmlRaceBoxResponse is the namespace-delta response for gs.fml.openFmlRaceBox.
type FmlOpenFmlRaceBoxResponse = RPCResponse[StateDelta]

// OpenFmlRaceBox calls gs.fml.openFmlRaceBox. Request fields inferred from game.js: isAll.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) OpenFmlRaceBox(ctx context.Context, req FmlOpenFmlRaceBoxRequest, opts ...RequestOption) (FmlOpenFmlRaceBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlOpenFmlRaceBox, req, opts...)
}

// FmlQuitRequest is the empty request body for gs.fml.quit.
type FmlQuitRequest struct{}

// FmlQuitResponse is the namespace-delta response for gs.fml.quit.
type FmlQuitResponse = RPCResponse[StateDelta]

// Quit calls gs.fml.quit. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Quit(ctx context.Context, req FmlQuitRequest, opts ...RequestOption) (FmlQuitResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlQuit, req, opts...)
}

// FmlRecvBoxRequest carries JSON fields for gs.fml.recvBox; game.js did not expose a stable object literal for this request.
type FmlRecvBoxRequest RawRequest

// FmlRecvBoxResponse is the namespace-delta response for gs.fml.recvBox.
type FmlRecvBoxResponse = RPCResponse[StateDelta]

// RecvBox calls gs.fml.recvBox. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) RecvBox(ctx context.Context, req FmlRecvBoxRequest, opts ...RequestOption) (FmlRecvBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRecvBox, req, opts...)
}

// FmlRefreshRaceBoatRequest is the empty request body for gs.fml.refreshRaceBoat.
type FmlRefreshRaceBoatRequest struct{}

// FmlRefreshRaceBoatResponse is the namespace-delta response for gs.fml.refreshRaceBoat.
type FmlRefreshRaceBoatResponse = RPCResponse[StateDelta]

// RefreshRaceBoat calls gs.fml.refreshRaceBoat. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) RefreshRaceBoat(ctx context.Context, req FmlRefreshRaceBoatRequest, opts ...RequestOption) (FmlRefreshRaceBoatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRefreshRaceBoat, req, opts...)
}

// FmlRefreshTitleRequest is the empty request body for gs.fml.refreshTitle.
type FmlRefreshTitleRequest struct{}

// FmlRefreshTitleResponse is the namespace-delta response for gs.fml.refreshTitle.
type FmlRefreshTitleResponse = RPCResponse[StateDelta]

// RefreshTitle calls gs.fml.refreshTitle. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) RefreshTitle(ctx context.Context, req FmlRefreshTitleRequest, opts ...RequestOption) (FmlRefreshTitleResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRefreshTitle, req, opts...)
}

// FmlSearchRequest is the request body for gs.fml.search.
type FmlSearchRequest struct {
	Fid    RPCID   `json:"fid,omitempty"`
	WithMb RPCBool `json:"withMb,omitempty"`
}

// FmlSearchResponse is the namespace-delta response for gs.fml.search.
type FmlSearchResponse = RPCResponse[StateDelta]

// Search calls gs.fml.search. Request fields inferred from game.js: fid, withMb.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Search(ctx context.Context, req FmlSearchRequest, opts ...RequestOption) (FmlSearchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlSearch, req, opts...)
}

// FmlSettingRequest carries JSON fields for gs.fml.setting; game.js did not expose a stable object literal for this request.
type FmlSettingRequest RawRequest

// FmlSettingResponse is the namespace-delta response for gs.fml.setting.
type FmlSettingResponse = RPCResponse[StateDelta]

// Setting calls gs.fml.setting. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) Setting(ctx context.Context, req FmlSettingRequest, opts ...RequestOption) (FmlSettingResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlSetting, req, opts...)
}

// FmlUnbindUnionGroupRequest is the request body for gs.fml.unbindUnionGroup.
type FmlUnbindUnionGroupRequest struct {
	FmlId RPCID `json:"fmlId,omitempty"`
}

// FmlUnbindUnionGroupResponse is the namespace-delta response for gs.fml.unbindUnionGroup.
type FmlUnbindUnionGroupResponse = RPCResponse[StateDelta]

// UnbindUnionGroup calls gs.fml.unbindUnionGroup. Request fields inferred from game.js: fmlId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) UnbindUnionGroup(ctx context.Context, req FmlUnbindUnionGroupRequest, opts ...RequestOption) (FmlUnbindUnionGroupResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlUnbindUnionGroup, req, opts...)
}

// FmlUnloadRaceBoatRequest is the request body for gs.fml.unloadRaceBoat.
type FmlUnloadRaceBoatRequest struct {
	BoatId RPCID `json:"boatId,omitempty"`
}

// FmlUnloadRaceBoatResponse is the namespace-delta response for gs.fml.unloadRaceBoat.
type FmlUnloadRaceBoatResponse = RPCResponse[StateDelta]

// UnloadRaceBoat calls gs.fml.unloadRaceBoat. Request fields inferred from game.js: boatId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) UnloadRaceBoat(ctx context.Context, req FmlUnloadRaceBoatRequest, opts ...RequestOption) (FmlUnloadRaceBoatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlUnloadRaceBoat, req, opts...)
}

// FmlUpgradeFmlRequest is the empty request body for gs.fml.upgradeFml.
type FmlUpgradeFmlRequest struct{}

// FmlUpgradeFmlResponse is the namespace-delta response for gs.fml.upgradeFml.
type FmlUpgradeFmlResponse = RPCResponse[StateDelta]

// UpgradeFml calls gs.fml.upgradeFml. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) UpgradeFml(ctx context.Context, req FmlUpgradeFmlRequest, opts ...RequestOption) (FmlUpgradeFmlResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlUpgradeFml, req, opts...)
}

// FmlUpgradeRaceBoatRequest is the request body for gs.fml.upgradeRaceBoat.
type FmlUpgradeRaceBoatRequest struct {
	BoatId RPCID `json:"boatId,omitempty"`
}

// FmlUpgradeRaceBoatResponse is the namespace-delta response for gs.fml.upgradeRaceBoat.
type FmlUpgradeRaceBoatResponse = RPCResponse[StateDelta]

// UpgradeRaceBoat calls gs.fml.upgradeRaceBoat. Request fields inferred from game.js: boatId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRPC) UpgradeRaceBoat(ctx context.Context, req FmlUpgradeRaceBoatRequest, opts ...RequestOption) (FmlUpgradeRaceBoatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlUpgradeRaceBoat, req, opts...)
}

// FmlFlowerShare returns typed RPC helpers for the fmlFlowerShare namespace.
func (c *RPCClient) FmlFlowerShare() FmlFlowerShareRPC { return FmlFlowerShareRPC{c: c} }

type FmlFlowerShareRPC struct{ c *RPCClient }

// FmlFlowerShareAddTakeCntRequest is the empty request body for gs.fmlFlowerShare.addTakeCnt.
type FmlFlowerShareAddTakeCntRequest struct{}

// FmlFlowerShareAddTakeCntResponse is the namespace-delta response for gs.fmlFlowerShare.addTakeCnt.
type FmlFlowerShareAddTakeCntResponse = RPCResponse[StateDelta]

// AddTakeCnt calls gs.fmlFlowerShare.addTakeCnt. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) AddTakeCnt(ctx context.Context, req FmlFlowerShareAddTakeCntRequest, opts ...RequestOption) (FmlFlowerShareAddTakeCntResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareAddTakeCnt, req, opts...)
}

// FmlFlowerShareGetFmlOtherShareListRequest is the empty request body for gs.fmlFlowerShare.getFmlOtherShareList.
type FmlFlowerShareGetFmlOtherShareListRequest struct{}

// FmlFlowerShareGetFmlOtherShareListResponse is the namespace-delta response for gs.fmlFlowerShare.getFmlOtherShareList.
type FmlFlowerShareGetFmlOtherShareListResponse = RPCResponse[StateDelta]

// GetFmlOtherShareList calls gs.fmlFlowerShare.getFmlOtherShareList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) GetFmlOtherShareList(ctx context.Context, req FmlFlowerShareGetFmlOtherShareListRequest, opts ...RequestOption) (FmlFlowerShareGetFmlOtherShareListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareGetFmlOtherShareList, req, opts...)
}

// FmlFlowerShareGetShareLogListRequest is the empty request body for gs.fmlFlowerShare.getShareLogList.
type FmlFlowerShareGetShareLogListRequest struct{}

// FmlFlowerShareGetShareLogListResponse is the namespace-delta response for gs.fmlFlowerShare.getShareLogList.
type FmlFlowerShareGetShareLogListResponse = RPCResponse[StateDelta]

// GetShareLogList calls gs.fmlFlowerShare.getShareLogList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) GetShareLogList(ctx context.Context, req FmlFlowerShareGetShareLogListRequest, opts ...RequestOption) (FmlFlowerShareGetShareLogListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareGetShareLogList, req, opts...)
}

// FmlFlowerShareRecvRwdRequest is the request body for gs.fmlFlowerShare.recvRwd.
type FmlFlowerShareRecvRwdRequest struct {
	SlotIds RPCIDList `json:"slotIds,omitempty"`
}

// FmlFlowerShareRecvRwdResponse is the namespace-delta response for gs.fmlFlowerShare.recvRwd.
type FmlFlowerShareRecvRwdResponse = RPCResponse[StateDelta]

// RecvRwd calls gs.fmlFlowerShare.recvRwd. Request fields inferred from game.js: slotIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) RecvRwd(ctx context.Context, req FmlFlowerShareRecvRwdRequest, opts ...RequestOption) (FmlFlowerShareRecvRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareRecvRwd, req, opts...)
}

// FmlFlowerShareRefreshRequest is the empty request body for gs.fmlFlowerShare.refresh.
type FmlFlowerShareRefreshRequest struct{}

// FmlFlowerShareRefreshResponse is the namespace-delta response for gs.fmlFlowerShare.refresh.
type FmlFlowerShareRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.fmlFlowerShare.refresh. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) Refresh(ctx context.Context, req FmlFlowerShareRefreshRequest, opts ...RequestOption) (FmlFlowerShareRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareRefresh, req, opts...)
}

// FmlFlowerShareShareRequest is the request body for gs.fmlFlowerShare.share.
type FmlFlowerShareShareRequest struct {
	SlotId   RPCID `json:"slotId,omitempty"`
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// FmlFlowerShareShareResponse is the namespace-delta response for gs.fmlFlowerShare.share.
type FmlFlowerShareShareResponse = RPCResponse[StateDelta]

// Share calls gs.fmlFlowerShare.share. Request fields inferred from game.js: slotId, flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) Share(ctx context.Context, req FmlFlowerShareShareRequest, opts ...RequestOption) (FmlFlowerShareShareResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareShare, req, opts...)
}

// FmlFlowerShareTakeRequest is the request body for gs.fmlFlowerShare.take.
type FmlFlowerShareTakeRequest struct {
	DstUID RPCUID `json:"dstUid,omitempty"`
	SlotId RPCID  `json:"slotId,omitempty"`
}

// FmlFlowerShareTakeResponse is the namespace-delta response for gs.fmlFlowerShare.take.
type FmlFlowerShareTakeResponse = RPCResponse[StateDelta]

// Take calls gs.fmlFlowerShare.take. Request fields inferred from game.js: dstUid, slotId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) Take(ctx context.Context, req FmlFlowerShareTakeRequest, opts ...RequestOption) (FmlFlowerShareTakeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareTake, req, opts...)
}

// FmlFlowerShareUnlockRequest is the request body for gs.fmlFlowerShare.unlock.
type FmlFlowerShareUnlockRequest struct {
	SlotId RPCID `json:"slotId,omitempty"`
}

// FmlFlowerShareUnlockResponse is the namespace-delta response for gs.fmlFlowerShare.unlock.
type FmlFlowerShareUnlockResponse = RPCResponse[StateDelta]

// Unlock calls gs.fmlFlowerShare.unlock. Request fields inferred from game.js: slotId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShareRPC) Unlock(ctx context.Context, req FmlFlowerShareUnlockRequest, opts ...RequestOption) (FmlFlowerShareUnlockResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShareUnlock, req, opts...)
}

// FmlFlowerShow returns typed RPC helpers for the fmlFlowerShow namespace.
func (c *RPCClient) FmlFlowerShow() FmlFlowerShowRPC { return FmlFlowerShowRPC{c: c} }

type FmlFlowerShowRPC struct{ c *RPCClient }

// FmlFlowerShowCancelLikeOtherRequest is the request body for gs.fmlFlowerShow.cancelLikeOther.
type FmlFlowerShowCancelLikeOtherRequest struct {
	TargetUID RPCUID `json:"targetUid,omitempty"`
}

// FmlFlowerShowCancelLikeOtherResponse is the namespace-delta response for gs.fmlFlowerShow.cancelLikeOther.
type FmlFlowerShowCancelLikeOtherResponse = RPCResponse[StateDelta]

// CancelLikeOther calls gs.fmlFlowerShow.cancelLikeOther. Request fields inferred from game.js: targetUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) CancelLikeOther(ctx context.Context, req FmlFlowerShowCancelLikeOtherRequest, opts ...RequestOption) (FmlFlowerShowCancelLikeOtherResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowCancelLikeOther, req, opts...)
}

// FmlFlowerShowGetLikeOtherRecordRequest is the empty request body for gs.fmlFlowerShow.getLikeOtherRecord.
type FmlFlowerShowGetLikeOtherRecordRequest struct{}

// FmlFlowerShowGetLikeOtherRecordResponse is the namespace-delta response for gs.fmlFlowerShow.getLikeOtherRecord.
type FmlFlowerShowGetLikeOtherRecordResponse = RPCResponse[StateDelta]

// GetLikeOtherRecord calls gs.fmlFlowerShow.getLikeOtherRecord. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) GetLikeOtherRecord(ctx context.Context, req FmlFlowerShowGetLikeOtherRecordRequest, opts ...RequestOption) (FmlFlowerShowGetLikeOtherRecordResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowGetLikeOtherRecord, req, opts...)
}

// FmlFlowerShowGetLikeOtherRecord5LimitRequest is the request body for gs.fmlFlowerShow.getLikeOtherRecord5Limit.
type FmlFlowerShowGetLikeOtherRecord5LimitRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// FmlFlowerShowGetLikeOtherRecord5LimitResponse is the namespace-delta response for gs.fmlFlowerShow.getLikeOtherRecord5Limit.
type FmlFlowerShowGetLikeOtherRecord5LimitResponse = RPCResponse[StateDelta]

// GetLikeOtherRecord5Limit calls gs.fmlFlowerShow.getLikeOtherRecord5Limit. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) GetLikeOtherRecord5Limit(ctx context.Context, req FmlFlowerShowGetLikeOtherRecord5LimitRequest, opts ...RequestOption) (FmlFlowerShowGetLikeOtherRecord5LimitResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowGetLikeOtherRecord5Limit, req, opts...)
}

// FmlFlowerShowGetShowInfoRequest is the request body for gs.fmlFlowerShow.getShowInfo.
type FmlFlowerShowGetShowInfoRequest struct {
	TargetUID RPCUID `json:"targetUid,omitempty"`
}

// FmlFlowerShowGetShowInfoResponse is the namespace-delta response for gs.fmlFlowerShow.getShowInfo.
type FmlFlowerShowGetShowInfoResponse = RPCResponse[StateDelta]

// GetShowInfo calls gs.fmlFlowerShow.getShowInfo. Request fields inferred from game.js: targetUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) GetShowInfo(ctx context.Context, req FmlFlowerShowGetShowInfoRequest, opts ...RequestOption) (FmlFlowerShowGetShowInfoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowGetShowInfo, req, opts...)
}

// FmlFlowerShowLikeOtherRequest is the request body for gs.fmlFlowerShow.likeOther.
type FmlFlowerShowLikeOtherRequest struct {
	TargetUID RPCUID `json:"targetUid,omitempty"`
}

// FmlFlowerShowLikeOtherResponse is the namespace-delta response for gs.fmlFlowerShow.likeOther.
type FmlFlowerShowLikeOtherResponse = RPCResponse[StateDelta]

// LikeOther calls gs.fmlFlowerShow.likeOther. Request fields inferred from game.js: targetUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) LikeOther(ctx context.Context, req FmlFlowerShowLikeOtherRequest, opts ...RequestOption) (FmlFlowerShowLikeOtherResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowLikeOther, req, opts...)
}

// FmlFlowerShowSaveShowRequest is the request body for gs.fmlFlowerShow.saveShow.
type FmlFlowerShowSaveShowRequest struct {
	ShowFlowers RPCObject `json:"showFlowers,omitempty"`
}

// FmlFlowerShowSaveShowResponse is the namespace-delta response for gs.fmlFlowerShow.saveShow.
type FmlFlowerShowSaveShowResponse = RPCResponse[StateDelta]

// SaveShow calls gs.fmlFlowerShow.saveShow. Request fields inferred from game.js: showFlowers.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) SaveShow(ctx context.Context, req FmlFlowerShowSaveShowRequest, opts ...RequestOption) (FmlFlowerShowSaveShowResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowSaveShow, req, opts...)
}

// FmlFlowerShowSetVisitTypeRequest is the request body for gs.fmlFlowerShow.setVisitType.
type FmlFlowerShowSetVisitTypeRequest struct {
	VisitType RPCInt `json:"visitType,omitempty"`
}

// FmlFlowerShowSetVisitTypeResponse is the namespace-delta response for gs.fmlFlowerShow.setVisitType.
type FmlFlowerShowSetVisitTypeResponse = RPCResponse[StateDelta]

// SetVisitType calls gs.fmlFlowerShow.setVisitType. Request fields inferred from game.js: visitType.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) SetVisitType(ctx context.Context, req FmlFlowerShowSetVisitTypeRequest, opts ...RequestOption) (FmlFlowerShowSetVisitTypeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowSetVisitType, req, opts...)
}

// FmlFlowerShowSwitchMapRequest is the request body for gs.fmlFlowerShow.switchMap.
type FmlFlowerShowSwitchMapRequest struct {
	MapId RPCID `json:"mapId,omitempty"`
}

// FmlFlowerShowSwitchMapResponse is the namespace-delta response for gs.fmlFlowerShow.switchMap.
type FmlFlowerShowSwitchMapResponse = RPCResponse[StateDelta]

// SwitchMap calls gs.fmlFlowerShow.switchMap. Request fields inferred from game.js: mapId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) SwitchMap(ctx context.Context, req FmlFlowerShowSwitchMapRequest, opts ...RequestOption) (FmlFlowerShowSwitchMapResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowSwitchMap, req, opts...)
}

// FmlFlowerShowUnlockSlotRequest is the request body for gs.fmlFlowerShow.unlockSlot.
type FmlFlowerShowUnlockSlotRequest struct {
	Index RPCInt `json:"index,omitempty"`
}

// FmlFlowerShowUnlockSlotResponse is the namespace-delta response for gs.fmlFlowerShow.unlockSlot.
type FmlFlowerShowUnlockSlotResponse = RPCResponse[StateDelta]

// UnlockSlot calls gs.fmlFlowerShow.unlockSlot. Request fields inferred from game.js: index.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlFlowerShowRPC) UnlockSlot(ctx context.Context, req FmlFlowerShowUnlockSlotRequest, opts ...RequestOption) (FmlFlowerShowUnlockSlotResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlFlowerShowUnlockSlot, req, opts...)
}

// FmlForest returns typed RPC helpers for the fmlForest namespace.
func (c *RPCClient) FmlForest() FmlForestRPC { return FmlForestRPC{c: c} }

type FmlForestRPC struct{ c *RPCClient }

// FmlForestApplyPlantRequest is the request body for gs.fmlForest.applyPlant.
type FmlForestApplyPlantRequest struct {
	TreeId RPCID `json:"treeId,omitempty"`
}

// FmlForestApplyPlantResponse is the namespace-delta response for gs.fmlForest.applyPlant.
type FmlForestApplyPlantResponse = RPCResponse[StateDelta]

// ApplyPlant calls gs.fmlForest.applyPlant. Request fields inferred from game.js: treeId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) ApplyPlant(ctx context.Context, req FmlForestApplyPlantRequest, opts ...RequestOption) (FmlForestApplyPlantResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestApplyPlant, req, opts...)
}

// FmlForestCollectEnergyRequest is the request body for gs.fmlForest.collectEnergy.
type FmlForestCollectEnergyRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// FmlForestCollectEnergyResponse is the namespace-delta response for gs.fmlForest.collectEnergy.
type FmlForestCollectEnergyResponse = RPCResponse[StateDelta]

// CollectEnergy calls gs.fmlForest.collectEnergy. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) CollectEnergy(ctx context.Context, req FmlForestCollectEnergyRequest, opts ...RequestOption) (FmlForestCollectEnergyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestCollectEnergy, req, opts...)
}

// FmlForestEnterRequest is the empty request body for gs.fmlForest.enter.
type FmlForestEnterRequest struct{}

// FmlForestEnterResponse is the namespace-delta response for gs.fmlForest.enter.
type FmlForestEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.fmlForest.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) Enter(ctx context.Context, req FmlForestEnterRequest, opts ...RequestOption) (FmlForestEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestEnter, req, opts...)
}

// FmlForestGetCertDetailRequest is the request body for gs.fmlForest.getCertDetail.
type FmlForestGetCertDetailRequest struct {
	TreeCodeList RPCIDList `json:"treeCodeList,omitempty"`
}

// FmlForestGetCertDetailResponse is the namespace-delta response for gs.fmlForest.getCertDetail.
type FmlForestGetCertDetailResponse = RPCResponse[StateDelta]

// GetCertDetail calls gs.fmlForest.getCertDetail. Request fields inferred from game.js: treeCodeList.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) GetCertDetail(ctx context.Context, req FmlForestGetCertDetailRequest, opts ...RequestOption) (FmlForestGetCertDetailResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestGetCertDetail, req, opts...)
}

// FmlForestGetCertDetailByFidRequest is the request body for gs.fmlForest.getCertDetailByFid.
type FmlForestGetCertDetailByFidRequest struct {
	Fid RPCID `json:"fid,omitempty"`
}

// FmlForestGetCertDetailByFidResponse is the namespace-delta response for gs.fmlForest.getCertDetailByFid.
type FmlForestGetCertDetailByFidResponse = RPCResponse[StateDelta]

// GetCertDetailByFid calls gs.fmlForest.getCertDetailByFid. Request fields inferred from game.js: fid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) GetCertDetailByFid(ctx context.Context, req FmlForestGetCertDetailByFidRequest, opts ...RequestOption) (FmlForestGetCertDetailByFidResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestGetCertDetailByFid, req, opts...)
}

// FmlForestGetLogListRequest is the empty request body for gs.fmlForest.getLogList.
type FmlForestGetLogListRequest struct{}

// FmlForestGetLogListResponse is the namespace-delta response for gs.fmlForest.getLogList.
type FmlForestGetLogListResponse = RPCResponse[StateDelta]

// GetLogList calls gs.fmlForest.getLogList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) GetLogList(ctx context.Context, req FmlForestGetLogListRequest, opts ...RequestOption) (FmlForestGetLogListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestGetLogList, req, opts...)
}

// FmlForestGetTreeListRequest is the empty request body for gs.fmlForest.getTreeList.
type FmlForestGetTreeListRequest struct{}

// FmlForestGetTreeListResponse is the namespace-delta response for gs.fmlForest.getTreeList.
type FmlForestGetTreeListResponse = RPCResponse[StateDelta]

// GetTreeList calls gs.fmlForest.getTreeList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) GetTreeList(ctx context.Context, req FmlForestGetTreeListRequest, opts ...RequestOption) (FmlForestGetTreeListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestGetTreeList, req, opts...)
}

// FmlForestGetWeekCollectRequest is the empty request body for gs.fmlForest.getWeekCollect.
type FmlForestGetWeekCollectRequest struct{}

// FmlForestGetWeekCollectResponse is the namespace-delta response for gs.fmlForest.getWeekCollect.
type FmlForestGetWeekCollectResponse = RPCResponse[StateDelta]

// GetWeekCollect calls gs.fmlForest.getWeekCollect. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) GetWeekCollect(ctx context.Context, req FmlForestGetWeekCollectRequest, opts ...RequestOption) (FmlForestGetWeekCollectResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestGetWeekCollect, req, opts...)
}

// FmlForestGetWeekStatRequest is the empty request body for gs.fmlForest.getWeekStat.
type FmlForestGetWeekStatRequest struct{}

// FmlForestGetWeekStatResponse is the namespace-delta response for gs.fmlForest.getWeekStat.
type FmlForestGetWeekStatResponse = RPCResponse[StateDelta]

// GetWeekStat calls gs.fmlForest.getWeekStat. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) GetWeekStat(ctx context.Context, req FmlForestGetWeekStatRequest, opts ...RequestOption) (FmlForestGetWeekStatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestGetWeekStat, req, opts...)
}

// FmlForestRefreshRequest is the request body for gs.fmlForest.refresh.
type FmlForestRefreshRequest struct {
	IsAutoCollect RPCBool `json:"isAutoCollect,omitempty"`
}

// FmlForestRefreshResponse is the namespace-delta response for gs.fmlForest.refresh.
type FmlForestRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.fmlForest.refresh. Request fields inferred from game.js: isAutoCollect.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlForestRPC) Refresh(ctx context.Context, req FmlForestRefreshRequest, opts ...RequestOption) (FmlForestRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlForestRefresh, req, opts...)
}

// FmlLand returns typed RPC helpers for the fmlLand namespace.
func (c *RPCClient) FmlLand() FmlLandRPC { return FmlLandRPC{c: c} }

type FmlLandRPC struct{ c *RPCClient }

// FmlLandHarvestRequest is the request body for gs.fmlLand.harvest.
type FmlLandHarvestRequest struct {
	LandIds RPCIDList `json:"landIds,omitempty"`
}

// FmlLandHarvestResponse is the namespace-delta response for gs.fmlLand.harvest.
type FmlLandHarvestResponse = RPCResponse[StateDelta]

// Harvest calls gs.fmlLand.harvest. Request fields inferred from game.js: landIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlLandRPC) Harvest(ctx context.Context, req FmlLandHarvestRequest, opts ...RequestOption) (FmlLandHarvestResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlLandHarvest, req, opts...)
}

// FmlLandHarvestAllRequest is the empty request body for gs.fmlLand.harvestAll.
type FmlLandHarvestAllRequest struct{}

// FmlLandHarvestAllResponse is the namespace-delta response for gs.fmlLand.harvestAll.
type FmlLandHarvestAllResponse = RPCResponse[StateDelta]

// HarvestAll calls gs.fmlLand.harvestAll. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlLandRPC) HarvestAll(ctx context.Context, req FmlLandHarvestAllRequest, opts ...RequestOption) (FmlLandHarvestAllResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlLandHarvestAll, req, opts...)
}

// FmlLandPlantRequest is the request body for gs.fmlLand.plant.
type FmlLandPlantRequest struct {
	LandIds RPCIDList `json:"landIds,omitempty"`
	FlwId   RPCID     `json:"flwId,omitempty"`
}

// FmlLandPlantResponse is the namespace-delta response for gs.fmlLand.plant.
type FmlLandPlantResponse = RPCResponse[StateDelta]

// Plant calls gs.fmlLand.plant. Request fields inferred from game.js: landIds, flwId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlLandRPC) Plant(ctx context.Context, req FmlLandPlantRequest, opts ...RequestOption) (FmlLandPlantResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlLandPlant, req, opts...)
}

// FmlLandUnlockRequest is the request body for gs.fmlLand.unlock.
type FmlLandUnlockRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// FmlLandUnlockResponse is the namespace-delta response for gs.fmlLand.unlock.
type FmlLandUnlockResponse = RPCResponse[StateDelta]

// Unlock calls gs.fmlLand.unlock. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlLandRPC) Unlock(ctx context.Context, req FmlLandUnlockRequest, opts ...RequestOption) (FmlLandUnlockResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlLandUnlock, req, opts...)
}

// FmlLandUpgradeRequest is the request body for gs.fmlLand.upgrade.
type FmlLandUpgradeRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// FmlLandUpgradeResponse is the namespace-delta response for gs.fmlLand.upgrade.
type FmlLandUpgradeResponse = RPCResponse[StateDelta]

// Upgrade calls gs.fmlLand.upgrade. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlLandRPC) Upgrade(ctx context.Context, req FmlLandUpgradeRequest, opts ...RequestOption) (FmlLandUpgradeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlLandUpgrade, req, opts...)
}

// FmlRace returns typed RPC helpers for the fmlRace namespace.
func (c *RPCClient) FmlRace() FmlRaceRPC { return FmlRaceRPC{c: c} }

type FmlRaceRPC struct{ c *RPCClient }

// FmlRaceBuyTaskNumRequest is the request body for gs.fmlRace.buyTaskNum.
type FmlRaceBuyTaskNumRequest struct {
	Num RPCInt `json:"num,omitempty"`
}

// FmlRaceBuyTaskNumResponse is the namespace-delta response for gs.fmlRace.buyTaskNum.
type FmlRaceBuyTaskNumResponse = RPCResponse[StateDelta]

// BuyTaskNum calls gs.fmlRace.buyTaskNum. Request fields inferred from game.js: num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) BuyTaskNum(ctx context.Context, req FmlRaceBuyTaskNumRequest, opts ...RequestOption) (FmlRaceBuyTaskNumResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceBuyTaskNum, req, opts...)
}

// FmlRaceDelTaskRequest is the request body for gs.fmlRace.delTask.
type FmlRaceDelTaskRequest struct {
	TaskMsId RPCID `json:"taskMsId,omitempty"`
}

// FmlRaceDelTaskResponse is the namespace-delta response for gs.fmlRace.delTask.
type FmlRaceDelTaskResponse = RPCResponse[StateDelta]

// DelTask calls gs.fmlRace.delTask. Request fields inferred from game.js: taskMsId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) DelTask(ctx context.Context, req FmlRaceDelTaskRequest, opts ...RequestOption) (FmlRaceDelTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceDelTask, req, opts...)
}

// FmlRaceEnterRequest is the empty request body for gs.fmlRace.enter.
type FmlRaceEnterRequest struct{}

// FmlRaceEnterResponse is the namespace-delta response for gs.fmlRace.enter.
type FmlRaceEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.fmlRace.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) Enter(ctx context.Context, req FmlRaceEnterRequest, opts ...RequestOption) (FmlRaceEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceEnter, req, opts...)
}

// FmlRaceFinishTaskRequest is the request body for gs.fmlRace.finishTask.
type FmlRaceFinishTaskRequest struct {
	TaskMsId RPCID `json:"taskMsId,omitempty"`
}

// FmlRaceFinishTaskResponse is the namespace-delta response for gs.fmlRace.finishTask.
type FmlRaceFinishTaskResponse = RPCResponse[StateDelta]

// FinishTask calls gs.fmlRace.finishTask. Request fields inferred from game.js: taskMsId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) FinishTask(ctx context.Context, req FmlRaceFinishTaskRequest, opts ...RequestOption) (FmlRaceFinishTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceFinishTask, req, opts...)
}

// FmlRaceGetFmlRaceEndDisplayDataRequest is the empty request body for gs.fmlRace.getFmlRaceEndDisplayData.
type FmlRaceGetFmlRaceEndDisplayDataRequest struct{}

// FmlRaceGetFmlRaceEndDisplayDataResponse is the namespace-delta response for gs.fmlRace.getFmlRaceEndDisplayData.
type FmlRaceGetFmlRaceEndDisplayDataResponse = RPCResponse[StateDelta]

// GetFmlRaceEndDisplayData calls gs.fmlRace.getFmlRaceEndDisplayData. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetFmlRaceEndDisplayData(ctx context.Context, req FmlRaceGetFmlRaceEndDisplayDataRequest, opts ...RequestOption) (FmlRaceGetFmlRaceEndDisplayDataResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetFmlRaceEndDisplayData, req, opts...)
}

// FmlRaceGetFmlRaceHistRcdListRequest is the empty request body for gs.fmlRace.getFmlRaceHistRcdList.
type FmlRaceGetFmlRaceHistRcdListRequest struct{}

// FmlRaceGetFmlRaceHistRcdListResponse is the namespace-delta response for gs.fmlRace.getFmlRaceHistRcdList.
type FmlRaceGetFmlRaceHistRcdListResponse = RPCResponse[StateDelta]

// GetFmlRaceHistRcdList calls gs.fmlRace.getFmlRaceHistRcdList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetFmlRaceHistRcdList(ctx context.Context, req FmlRaceGetFmlRaceHistRcdListRequest, opts ...RequestOption) (FmlRaceGetFmlRaceHistRcdListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetFmlRaceHistRcdList, req, opts...)
}

// FmlRaceGetFmlRaceTaskScoreRequest is the empty request body for gs.fmlRace.getFmlRaceTaskScore.
type FmlRaceGetFmlRaceTaskScoreRequest struct{}

// FmlRaceGetFmlRaceTaskScoreResponse is the namespace-delta response for gs.fmlRace.getFmlRaceTaskScore.
type FmlRaceGetFmlRaceTaskScoreResponse = RPCResponse[StateDelta]

// GetFmlRaceTaskScore calls gs.fmlRace.getFmlRaceTaskScore. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetFmlRaceTaskScore(ctx context.Context, req FmlRaceGetFmlRaceTaskScoreRequest, opts ...RequestOption) (FmlRaceGetFmlRaceTaskScoreResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetFmlRaceTaskScore, req, opts...)
}

// FmlRaceGetFmlRaceUsrRankListRequest is the request body for gs.fmlRace.getFmlRaceUsrRankList.
type FmlRaceGetFmlRaceUsrRankListRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// FmlRaceGetFmlRaceUsrRankListResponse is the namespace-delta response for gs.fmlRace.getFmlRaceUsrRankList.
type FmlRaceGetFmlRaceUsrRankListResponse = RPCResponse[StateDelta]

// GetFmlRaceUsrRankList calls gs.fmlRace.getFmlRaceUsrRankList. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetFmlRaceUsrRankList(ctx context.Context, req FmlRaceGetFmlRaceUsrRankListRequest, opts ...RequestOption) (FmlRaceGetFmlRaceUsrRankListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetFmlRaceUsrRankList, req, opts...)
}

// FmlRaceGetGroupFmlRaceRcdListRequest is the request body for gs.fmlRace.getGroupFmlRaceRcdList.
type FmlRaceGetGroupFmlRaceRcdListRequest struct {
	BatchId   RPCID   `json:"batchId,omitempty"`
	GroupId   RPCID   `json:"groupId,omitempty"`
	IsRefresh RPCBool `json:"isRefresh,omitempty"`
}

// FmlRaceGetGroupFmlRaceRcdListResponse is the namespace-delta response for gs.fmlRace.getGroupFmlRaceRcdList.
type FmlRaceGetGroupFmlRaceRcdListResponse = RPCResponse[StateDelta]

// GetGroupFmlRaceRcdList calls gs.fmlRace.getGroupFmlRaceRcdList. Request fields inferred from game.js: batchId, groupId, isRefresh.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetGroupFmlRaceRcdList(ctx context.Context, req FmlRaceGetGroupFmlRaceRcdListRequest, opts ...RequestOption) (FmlRaceGetGroupFmlRaceRcdListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetGroupFmlRaceRcdList, req, opts...)
}

// FmlRaceGetNewMbScoreRankRequest is the empty request body for gs.fmlRace.getNewMbScoreRank.
type FmlRaceGetNewMbScoreRankRequest struct{}

// FmlRaceGetNewMbScoreRankResponse is the namespace-delta response for gs.fmlRace.getNewMbScoreRank.
type FmlRaceGetNewMbScoreRankResponse = RPCResponse[StateDelta]

// GetNewMbScoreRank calls gs.fmlRace.getNewMbScoreRank. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetNewMbScoreRank(ctx context.Context, req FmlRaceGetNewMbScoreRankRequest, opts ...RequestOption) (FmlRaceGetNewMbScoreRankResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetNewMbScoreRank, req, opts...)
}

// FmlRaceGetTaskListRequest is the empty request body for gs.fmlRace.getTaskList.
type FmlRaceGetTaskListRequest struct{}

// FmlRaceGetTaskListResponse is the namespace-delta response for gs.fmlRace.getTaskList.
type FmlRaceGetTaskListResponse = RPCResponse[StateDelta]

// GetTaskList calls gs.fmlRace.getTaskList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetTaskList(ctx context.Context, req FmlRaceGetTaskListRequest, opts ...RequestOption) (FmlRaceGetTaskListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetTaskList, req, opts...)
}

// FmlRaceGetTaskLogListRequest is the request body for gs.fmlRace.getTaskLogList.
type FmlRaceGetTaskLogListRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// FmlRaceGetTaskLogListResponse is the namespace-delta response for gs.fmlRace.getTaskLogList.
type FmlRaceGetTaskLogListResponse = RPCResponse[StateDelta]

// GetTaskLogList calls gs.fmlRace.getTaskLogList. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GetTaskLogList(ctx context.Context, req FmlRaceGetTaskLogListRequest, opts ...RequestOption) (FmlRaceGetTaskLogListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGetTaskLogList, req, opts...)
}

// FmlRaceGiveUpTaskRequest is the empty request body for gs.fmlRace.giveUpTask.
type FmlRaceGiveUpTaskRequest struct{}

// FmlRaceGiveUpTaskResponse is the namespace-delta response for gs.fmlRace.giveUpTask.
type FmlRaceGiveUpTaskResponse = RPCResponse[StateDelta]

// GiveUpTask calls gs.fmlRace.giveUpTask. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) GiveUpTask(ctx context.Context, req FmlRaceGiveUpTaskRequest, opts ...RequestOption) (FmlRaceGiveUpTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceGiveUpTask, req, opts...)
}

// FmlRaceRefreshFmlRaceBatchRequest is the empty request body for gs.fmlRace.refreshFmlRaceBatch.
type FmlRaceRefreshFmlRaceBatchRequest struct{}

// FmlRaceRefreshFmlRaceBatchResponse is the namespace-delta response for gs.fmlRace.refreshFmlRaceBatch.
type FmlRaceRefreshFmlRaceBatchResponse = RPCResponse[StateDelta]

// RefreshFmlRaceBatch calls gs.fmlRace.refreshFmlRaceBatch. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) RefreshFmlRaceBatch(ctx context.Context, req FmlRaceRefreshFmlRaceBatchRequest, opts ...RequestOption) (FmlRaceRefreshFmlRaceBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceRefreshFmlRaceBatch, req, opts...)
}

// FmlRaceRefreshFmlRaceBoxRequest is the empty request body for gs.fmlRace.refreshFmlRaceBox.
type FmlRaceRefreshFmlRaceBoxRequest struct{}

// FmlRaceRefreshFmlRaceBoxResponse is the namespace-delta response for gs.fmlRace.refreshFmlRaceBox.
type FmlRaceRefreshFmlRaceBoxResponse = RPCResponse[StateDelta]

// RefreshFmlRaceBox calls gs.fmlRace.refreshFmlRaceBox. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) RefreshFmlRaceBox(ctx context.Context, req FmlRaceRefreshFmlRaceBoxRequest, opts ...RequestOption) (FmlRaceRefreshFmlRaceBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceRefreshFmlRaceBox, req, opts...)
}

// FmlRaceRefreshTaskRequest is the request body for gs.fmlRace.refreshTask.
type FmlRaceRefreshTaskRequest struct {
	Idx RPCInt `json:"idx,omitempty"`
}

// FmlRaceRefreshTaskResponse is the namespace-delta response for gs.fmlRace.refreshTask.
type FmlRaceRefreshTaskResponse = RPCResponse[StateDelta]

// RefreshTask calls gs.fmlRace.refreshTask. Request fields inferred from game.js: idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) RefreshTask(ctx context.Context, req FmlRaceRefreshTaskRequest, opts ...RequestOption) (FmlRaceRefreshTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceRefreshTask, req, opts...)
}

// FmlRaceTakeTaskRequest is the request body for gs.fmlRace.takeTask.
type FmlRaceTakeTaskRequest struct {
	TaskMsId RPCID `json:"taskMsId,omitempty"`
}

// FmlRaceTakeTaskResponse is the namespace-delta response for gs.fmlRace.takeTask.
type FmlRaceTakeTaskResponse = RPCResponse[StateDelta]

// TakeTask calls gs.fmlRace.takeTask. Request fields inferred from game.js: taskMsId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) TakeTask(ctx context.Context, req FmlRaceTakeTaskRequest, opts ...RequestOption) (FmlRaceTakeTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceTakeTask, req, opts...)
}

// FmlRaceUpgradeTaskRequest is the empty request body for gs.fmlRace.upgradeTask.
type FmlRaceUpgradeTaskRequest struct{}

// FmlRaceUpgradeTaskResponse is the namespace-delta response for gs.fmlRace.upgradeTask.
type FmlRaceUpgradeTaskResponse = RPCResponse[StateDelta]

// UpgradeTask calls gs.fmlRace.upgradeTask. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRPC) UpgradeTask(ctx context.Context, req FmlRaceUpgradeTaskRequest, opts ...RequestOption) (FmlRaceUpgradeTaskResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceUpgradeTask, req, opts...)
}

// FmlRaceRqst returns typed RPC helpers for the fmlRaceRqst namespace.
func (c *RPCClient) FmlRaceRqst() FmlRaceRqstRPC { return FmlRaceRqstRPC{c: c} }

type FmlRaceRqstRPC struct{ c *RPCClient }

// FmlRaceRqstShowShipRequest is the request body for gs.fmlRaceRqst.showShip.
type FmlRaceRqstShowShipRequest struct {
	Time RPCInt `json:"time,omitempty"`
}

// FmlRaceRqstShowShipResponse is the namespace-delta response for gs.fmlRaceRqst.showShip.
type FmlRaceRqstShowShipResponse = RPCResponse[StateDelta]

// ShowShip calls gs.fmlRaceRqst.showShip. Request fields inferred from game.js: time.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlRaceRqstRPC) ShowShip(ctx context.Context, req FmlRaceRqstShowShipRequest, opts ...RequestOption) (FmlRaceRqstShowShipResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlRaceRqstShowShip, req, opts...)
}

// FmlTaskEnter returns typed RPC helpers for the fmlTaskEnter namespace.
func (c *RPCClient) FmlTaskEnter() FmlTaskEnterRPC { return FmlTaskEnterRPC{c: c} }

type FmlTaskEnterRPC struct{ c *RPCClient }

// FmlTaskEnterShowtcrwRequest is the request body for gs.fmlTaskEnter.showtcrw.
type FmlTaskEnterShowtcrwRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// FmlTaskEnterShowtcrwResponse is the namespace-delta response for gs.fmlTaskEnter.showtcrw.
type FmlTaskEnterShowtcrwResponse = RPCResponse[StateDelta]

// Showtcrw calls gs.fmlTaskEnter.showtcrw. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FmlTaskEnterRPC) Showtcrw(ctx context.Context, req FmlTaskEnterShowtcrwRequest, opts ...RequestOption) (FmlTaskEnterShowtcrwResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFmlTaskEnterShowtcrw, req, opts...)
}

// Frd returns typed RPC helpers for the frd namespace.
func (c *RPCClient) Frd() FrdRPC { return FrdRPC{c: c} }

type FrdRPC struct{ c *RPCClient }

// FrdAddBlackRequest is the request body for gs.frd.addBlack.
type FrdAddBlackRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// FrdAddBlackResponse is the namespace-delta response for gs.frd.addBlack.
type FrdAddBlackResponse = RPCResponse[StateDelta]

// AddBlack calls gs.frd.addBlack. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) AddBlack(ctx context.Context, req FrdAddBlackRequest, opts ...RequestOption) (FrdAddBlackResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdAddBlack, req, opts...)
}

// FrdApplyFrdRequest is the request body for gs.frd.applyFrd.
type FrdApplyFrdRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// FrdApplyFrdResponse is the namespace-delta response for gs.frd.applyFrd.
type FrdApplyFrdResponse = RPCResponse[StateDelta]

// ApplyFrd calls gs.frd.applyFrd. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) ApplyFrd(ctx context.Context, req FrdApplyFrdRequest, opts ...RequestOption) (FrdApplyFrdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdApplyFrd, req, opts...)
}

// FrdApplyFrdBatchRequest is the request body for gs.frd.applyFrdBatch.
type FrdApplyFrdBatchRequest struct {
	UIDs RPCUIDList `json:"uids,omitempty"`
}

// FrdApplyFrdBatchResponse is the namespace-delta response for gs.frd.applyFrdBatch.
type FrdApplyFrdBatchResponse = RPCResponse[StateDelta]

// ApplyFrdBatch calls gs.frd.applyFrdBatch. Request fields inferred from game.js: uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) ApplyFrdBatch(ctx context.Context, req FrdApplyFrdBatchRequest, opts ...RequestOption) (FrdApplyFrdBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdApplyFrdBatch, req, opts...)
}

// FrdDelRequest is the request body for gs.frd.del.
type FrdDelRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// FrdDelResponse is the namespace-delta response for gs.frd.del.
type FrdDelResponse = RPCResponse[StateDelta]

// Del calls gs.frd.del. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) Del(ctx context.Context, req FrdDelRequest, opts ...RequestOption) (FrdDelResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdDel, req, opts...)
}

// FrdDelBlackRequest is the request body for gs.frd.delBlack.
type FrdDelBlackRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// FrdDelBlackResponse is the namespace-delta response for gs.frd.delBlack.
type FrdDelBlackResponse = RPCResponse[StateDelta]

// DelBlack calls gs.frd.delBlack. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) DelBlack(ctx context.Context, req FrdDelBlackRequest, opts ...RequestOption) (FrdDelBlackResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdDelBlack, req, opts...)
}

// FrdEnterRequest is the request body for gs.frd.enter.
type FrdEnterRequest struct {
	NeedBlackList  RPCInt `json:"needBlackList,omitempty"`
	NeedApplyList  RPCInt `json:"needApplyList,omitempty"`
	NeedFriendList RPCInt `json:"needFriendList,omitempty"`
}

// FrdEnterResponse is the namespace-delta response for gs.frd.enter.
type FrdEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.frd.enter. Request fields inferred from game.js: needBlackList, needApplyList, needFriendList.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) Enter(ctx context.Context, req FrdEnterRequest, opts ...RequestOption) (FrdEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdEnter, req, opts...)
}

// FrdGetApplyListRequest is the empty request body for gs.frd.getApplyList.
type FrdGetApplyListRequest struct{}

// FrdGetApplyListResponse is the namespace-delta response for gs.frd.getApplyList.
type FrdGetApplyListResponse = RPCResponse[StateDelta]

// GetApplyList calls gs.frd.getApplyList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) GetApplyList(ctx context.Context, req FrdGetApplyListRequest, opts ...RequestOption) (FrdGetApplyListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdGetApplyList, req, opts...)
}

// FrdGetBlackListRequest is the empty request body for gs.frd.getBlackList.
type FrdGetBlackListRequest struct{}

// FrdGetBlackListResponse is the namespace-delta response for gs.frd.getBlackList.
type FrdGetBlackListResponse = RPCResponse[StateDelta]

// GetBlackList calls gs.frd.getBlackList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) GetBlackList(ctx context.Context, req FrdGetBlackListRequest, opts ...RequestOption) (FrdGetBlackListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdGetBlackList, req, opts...)
}

// FrdGetFriendListRequest is the empty request body for gs.frd.getFriendList.
type FrdGetFriendListRequest struct{}

// FrdGetFriendListResponse is the namespace-delta response for gs.frd.getFriendList.
type FrdGetFriendListResponse = RPCResponse[StateDelta]

// GetFriendList calls gs.frd.getFriendList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) GetFriendList(ctx context.Context, req FrdGetFriendListRequest, opts ...RequestOption) (FrdGetFriendListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdGetFriendList, req, opts...)
}

// FrdHandleApplyRequest is the request body for gs.frd.handleApply.
type FrdHandleApplyRequest struct {
	UID   RPCUID  `json:"uid,omitempty"`
	Agree RPCBool `json:"agree,omitempty"`
}

// FrdHandleApplyResponse is the namespace-delta response for gs.frd.handleApply.
type FrdHandleApplyResponse = RPCResponse[StateDelta]

// HandleApply calls gs.frd.handleApply. Request fields inferred from game.js: uid, agree.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) HandleApply(ctx context.Context, req FrdHandleApplyRequest, opts ...RequestOption) (FrdHandleApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdHandleApply, req, opts...)
}

// FrdHandleApplyBatchRequest is the request body for gs.frd.handleApplyBatch.
type FrdHandleApplyBatchRequest struct {
	UIDs  RPCUIDList `json:"uids,omitempty"`
	Agree RPCBool    `json:"agree,omitempty"`
}

// FrdHandleApplyBatchResponse is the namespace-delta response for gs.frd.handleApplyBatch.
type FrdHandleApplyBatchResponse = RPCResponse[StateDelta]

// HandleApplyBatch calls gs.frd.handleApplyBatch. Request fields inferred from game.js: uids, agree.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) HandleApplyBatch(ctx context.Context, req FrdHandleApplyBatchRequest, opts ...RequestOption) (FrdHandleApplyBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdHandleApplyBatch, req, opts...)
}

// FrdLikeRequest is the request body for gs.frd.like.
type FrdLikeRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// FrdLikeResponse is the namespace-delta response for gs.frd.like.
type FrdLikeResponse = RPCResponse[StateDelta]

// Like calls gs.frd.like. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) Like(ctx context.Context, req FrdLikeRequest, opts ...RequestOption) (FrdLikeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdLike, req, opts...)
}

// FrdRefreshRecListRequest is the request body for gs.frd.refreshRecList.
type FrdRefreshRecListRequest struct {
	IsFree RPCBool `json:"isFree,omitempty"`
}

// FrdRefreshRecListResponse is the namespace-delta response for gs.frd.refreshRecList.
type FrdRefreshRecListResponse = RPCResponse[StateDelta]

// RefreshRecList calls gs.frd.refreshRecList. Request fields inferred from game.js: isFree.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) RefreshRecList(ctx context.Context, req FrdRefreshRecListRequest, opts ...RequestOption) (FrdRefreshRecListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdRefreshRecList, req, opts...)
}

// FrdSetFrdRjtRequest is the request body for gs.frd.setFrdRjt.
type FrdSetFrdRjtRequest struct {
	IsRjtFrd RPCBool `json:"isRjtFrd,omitempty"`
}

// FrdSetFrdRjtResponse is the namespace-delta response for gs.frd.setFrdRjt.
type FrdSetFrdRjtResponse = RPCResponse[StateDelta]

// SetFrdRjt calls gs.frd.setFrdRjt. Request fields inferred from game.js: isRjtFrd.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdRPC) SetFrdRjt(ctx context.Context, req FrdSetFrdRjtRequest, opts ...RequestOption) (FrdSetFrdRjtResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdSetFrdRjt, req, opts...)
}

// FrdExt returns typed RPC helpers for the frdExt namespace.
func (c *RPCClient) FrdExt() FrdExtRPC { return FrdExtRPC{c: c} }

type FrdExtRPC struct{ c *RPCClient }

// FrdExtBuyStealCntRequest is the request body for gs.frdExt.buyStealCnt.
type FrdExtBuyStealCntRequest struct {
	FrdUID RPCUID `json:"frdUid,omitempty"`
	BuyCnt RPCInt `json:"buyCnt,omitempty"`
}

// FrdExtBuyStealCntResponse is the namespace-delta response for gs.frdExt.buyStealCnt.
type FrdExtBuyStealCntResponse = RPCResponse[StateDelta]

// BuyStealCnt calls gs.frdExt.buyStealCnt. Request fields inferred from game.js: frdUid, buyCnt.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdExtRPC) BuyStealCnt(ctx context.Context, req FrdExtBuyStealCntRequest, opts ...RequestOption) (FrdExtBuyStealCntResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdExtBuyStealCnt, req, opts...)
}

// FrdExtCancelFollowRequest is the request body for gs.frdExt.cancelFollow.
type FrdExtCancelFollowRequest struct {
	FrdUID RPCUID `json:"frdUid,omitempty"`
}

// FrdExtCancelFollowResponse is the namespace-delta response for gs.frdExt.cancelFollow.
type FrdExtCancelFollowResponse = RPCResponse[StateDelta]

// CancelFollow calls gs.frdExt.cancelFollow. Request fields inferred from game.js: frdUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdExtRPC) CancelFollow(ctx context.Context, req FrdExtCancelFollowRequest, opts ...RequestOption) (FrdExtCancelFollowResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdExtCancelFollow, req, opts...)
}

// FrdExtFollowRequest is the request body for gs.frdExt.follow.
type FrdExtFollowRequest struct {
	FrdUID RPCUID `json:"frdUid,omitempty"`
}

// FrdExtFollowResponse is the namespace-delta response for gs.frdExt.follow.
type FrdExtFollowResponse = RPCResponse[StateDelta]

// Follow calls gs.frdExt.follow. Request fields inferred from game.js: frdUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdExtRPC) Follow(ctx context.Context, req FrdExtFollowRequest, opts ...RequestOption) (FrdExtFollowResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdExtFollow, req, opts...)
}

// FrdExtGetFrdOtherInfoByUidsRequest carries JSON fields for gs.frdExt.getFrdOtherInfoByUids; game.js did not expose a stable object literal for this request.
type FrdExtGetFrdOtherInfoByUidsRequest RawRequest

// FrdExtGetFrdOtherInfoByUidsResponse is the namespace-delta response for gs.frdExt.getFrdOtherInfoByUids.
type FrdExtGetFrdOtherInfoByUidsResponse = RPCResponse[StateDelta]

// GetFrdOtherInfoByUids calls gs.frdExt.getFrdOtherInfoByUids. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdExtRPC) GetFrdOtherInfoByUids(ctx context.Context, req FrdExtGetFrdOtherInfoByUidsRequest, opts ...RequestOption) (FrdExtGetFrdOtherInfoByUidsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdExtGetFrdOtherInfoByUids, req, opts...)
}

// FrdExtSearchUserRequest is the request body for gs.frdExt.searchUser.
type FrdExtSearchUserRequest struct {
	Keyword RPCString `json:"keyword,omitempty"`
}

// FrdExtSearchUserResponse is the namespace-delta response for gs.frdExt.searchUser.
type FrdExtSearchUserResponse = RPCResponse[StateDelta]

// SearchUser calls gs.frdExt.searchUser. Request fields inferred from game.js: keyword.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdExtRPC) SearchUser(ctx context.Context, req FrdExtSearchUserRequest, opts ...RequestOption) (FrdExtSearchUserResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdExtSearchUser, req, opts...)
}

// FrdHome returns typed RPC helpers for the frdHome namespace.
func (c *RPCClient) FrdHome() FrdHomeRPC { return FrdHomeRPC{c: c} }

type FrdHomeRPC struct{ c *RPCClient }

// FrdHomeGetFrdHomeInfoRequest is the request body for gs.frdHome.getFrdHomeInfo.
type FrdHomeGetFrdHomeInfoRequest struct {
	FrdUID RPCUID `json:"frdUid,omitempty"`
}

// FrdHomeGetFrdHomeInfoResponse is the namespace-delta response for gs.frdHome.getFrdHomeInfo.
type FrdHomeGetFrdHomeInfoResponse = RPCResponse[StateDelta]

// GetFrdHomeInfo calls gs.frdHome.getFrdHomeInfo. Request fields inferred from game.js: frdUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdHomeRPC) GetFrdHomeInfo(ctx context.Context, req FrdHomeGetFrdHomeInfoRequest, opts ...RequestOption) (FrdHomeGetFrdHomeInfoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdHomeGetFrdHomeInfo, req, opts...)
}

// FrdShare returns typed RPC helpers for the frdShare namespace.
func (c *RPCClient) FrdShare() FrdShareRPC { return FrdShareRPC{c: c} }

type FrdShareRPC struct{ c *RPCClient }

// FrdShareEnterRequest is the empty request body for gs.frdShare.enter.
type FrdShareEnterRequest struct{}

// FrdShareEnterResponse is the namespace-delta response for gs.frdShare.enter.
type FrdShareEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.frdShare.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdShareRPC) Enter(ctx context.Context, req FrdShareEnterRequest, opts ...RequestOption) (FrdShareEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdShareEnter, req, opts...)
}

// FrdShareRecvBoxRwdRequest is the request body for gs.frdShare.recvBoxRwd.
type FrdShareRecvBoxRwdRequest struct {
	Idx RPCInt `json:"idx,omitempty"`
}

// FrdShareRecvBoxRwdResponse is the namespace-delta response for gs.frdShare.recvBoxRwd.
type FrdShareRecvBoxRwdResponse = RPCResponse[StateDelta]

// RecvBoxRwd calls gs.frdShare.recvBoxRwd. Request fields inferred from game.js: idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdShareRPC) RecvBoxRwd(ctx context.Context, req FrdShareRecvBoxRwdRequest, opts ...RequestOption) (FrdShareRecvBoxRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdShareRecvBoxRwd, req, opts...)
}

// FrdShareRecvSelfRwdRequest is the request body for gs.frdShare.recvSelfRwd.
type FrdShareRecvSelfRwdRequest struct {
	Idx RPCInt `json:"idx,omitempty"`
}

// FrdShareRecvSelfRwdResponse is the namespace-delta response for gs.frdShare.recvSelfRwd.
type FrdShareRecvSelfRwdResponse = RPCResponse[StateDelta]

// RecvSelfRwd calls gs.frdShare.recvSelfRwd. Request fields inferred from game.js: idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdShareRPC) RecvSelfRwd(ctx context.Context, req FrdShareRecvSelfRwdRequest, opts ...RequestOption) (FrdShareRecvSelfRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdShareRecvSelfRwd, req, opts...)
}

// FrdShareRecvShareRwdRequest is the request body for gs.frdShare.recvShareRwd.
type FrdShareRecvShareRwdRequest struct {
	WeekId     RPCID  `json:"weekId,omitempty"`
	InviterUID RPCUID `json:"inviterUid,omitempty"`
	Idx        RPCInt `json:"idx,omitempty"`
}

// FrdShareRecvShareRwdResponse is the namespace-delta response for gs.frdShare.recvShareRwd.
type FrdShareRecvShareRwdResponse = RPCResponse[StateDelta]

// RecvShareRwd calls gs.frdShare.recvShareRwd. Request fields inferred from game.js: weekId, inviterUid, idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdShareRPC) RecvShareRwd(ctx context.Context, req FrdShareRecvShareRwdRequest, opts ...RequestOption) (FrdShareRecvShareRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdShareRecvShareRwd, req, opts...)
}

// FrdSteal returns typed RPC helpers for the frdSteal namespace.
func (c *RPCClient) FrdSteal() FrdStealRPC { return FrdStealRPC{c: c} }

type FrdStealRPC struct{ c *RPCClient }

// FrdStealGetFrdStealRcdListRequest is the empty request body for gs.frdSteal.getFrdStealRcdList.
type FrdStealGetFrdStealRcdListRequest struct{}

// FrdStealGetFrdStealRcdListResponse is the namespace-delta response for gs.frdSteal.getFrdStealRcdList.
type FrdStealGetFrdStealRcdListResponse = RPCResponse[StateDelta]

// GetFrdStealRcdList calls gs.frdSteal.getFrdStealRcdList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdStealRPC) GetFrdStealRcdList(ctx context.Context, req FrdStealGetFrdStealRcdListRequest, opts ...RequestOption) (FrdStealGetFrdStealRcdListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdStealGetFrdStealRcdList, req, opts...)
}

// FrdStealGetStealStateByUidsRequest is the request body for gs.frdSteal.getStealStateByUids.
type FrdStealGetStealStateByUidsRequest struct {
	UIDs RPCUIDList `json:"uids,omitempty"`
}

// FrdStealGetStealStateByUidsResponse is the namespace-delta response for gs.frdSteal.getStealStateByUids.
type FrdStealGetStealStateByUidsResponse = RPCResponse[StateDelta]

// GetStealStateByUids calls gs.frdSteal.getStealStateByUids. Request fields inferred from game.js: uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdStealRPC) GetStealStateByUids(ctx context.Context, req FrdStealGetStealStateByUidsRequest, opts ...RequestOption) (FrdStealGetStealStateByUidsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdStealGetStealStateByUids, req, opts...)
}

// FrdStealStealRequest is the request body for gs.frdSteal.steal.
type FrdStealStealRequest struct {
	FrdUID     RPCUID `json:"frdUid,omitempty"`
	LandId     RPCID  `json:"landId,omitempty"`
	StealElves RPCInt `json:"stealElves,omitempty"`
}

// FrdStealStealResponse is the namespace-delta response for gs.frdSteal.steal.
type FrdStealStealResponse = RPCResponse[StateDelta]

// Steal calls gs.frdSteal.steal. Request fields inferred from game.js: frdUid, landId, stealElves.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdStealRPC) Steal(ctx context.Context, req FrdStealStealRequest, opts ...RequestOption) (FrdStealStealResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdStealSteal, req, opts...)
}

// FrdStealStealOneKeyRequest is the request body for gs.frdSteal.stealOneKey.
type FrdStealStealOneKeyRequest struct {
	FrdUID RPCUID `json:"frdUid,omitempty"`
}

// FrdStealStealOneKeyResponse is the namespace-delta response for gs.frdSteal.stealOneKey.
type FrdStealStealOneKeyResponse = RPCResponse[StateDelta]

// StealOneKey calls gs.frdSteal.stealOneKey. Request fields inferred from game.js: frdUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FrdStealRPC) StealOneKey(ctx context.Context, req FrdStealStealOneKeyRequest, opts ...RequestOption) (FrdStealStealOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFrdStealStealOneKey, req, opts...)
}

// FreeWater returns typed RPC helpers for the freeWater namespace.
func (c *RPCClient) FreeWater() FreeWaterRPC { return FreeWaterRPC{c: c} }

type FreeWaterRPC struct{ c *RPCClient }

// FreeWaterRecvRequest is the request body for gs.freeWater.recv.
type FreeWaterRecvRequest struct {
	Idx RPCInt `json:"idx,omitempty"`
}

// FreeWaterRecvResponse is the namespace-delta response for gs.freeWater.recv.
type FreeWaterRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.freeWater.recv. Request fields inferred from game.js: idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r FreeWaterRPC) Recv(ctx context.Context, req FreeWaterRecvRequest, opts ...RequestOption) (FreeWaterRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCFreeWaterRecv, req, opts...)
}

// GameClub returns typed RPC helpers for the gameClub namespace.
func (c *RPCClient) GameClub() GameClubRPC { return GameClubRPC{c: c} }

type GameClubRPC struct{ c *RPCClient }

// GameClubEnterRequest carries JSON fields for gs.gameClub.enter; game.js did not expose a stable object literal for this request.
type GameClubEnterRequest RawRequest

// GameClubEnterResponse is the namespace-delta response for gs.gameClub.enter.
type GameClubEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.gameClub.enter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GameClubRPC) Enter(ctx context.Context, req GameClubEnterRequest, opts ...RequestOption) (GameClubEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGameClubEnter, req, opts...)
}

// GameClubEnterClubRequest carries JSON fields for gs.gameClub.enterClub; game.js did not expose a stable object literal for this request.
type GameClubEnterClubRequest RawRequest

// GameClubEnterClubResponse is the namespace-delta response for gs.gameClub.enterClub.
type GameClubEnterClubResponse = RPCResponse[StateDelta]

// EnterClub calls gs.gameClub.enterClub. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GameClubRPC) EnterClub(ctx context.Context, req GameClubEnterClubRequest, opts ...RequestOption) (GameClubEnterClubResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGameClubEnterClub, req, opts...)
}

// GameClubRecvTaskRwdRequest is the request body for gs.gameClub.recvTaskRwd.
type GameClubRecvTaskRwdRequest struct {
	TaskId RPCID `json:"taskId,omitempty"`
}

// GameClubRecvTaskRwdResponse is the namespace-delta response for gs.gameClub.recvTaskRwd.
type GameClubRecvTaskRwdResponse = RPCResponse[StateDelta]

// RecvTaskRwd calls gs.gameClub.recvTaskRwd. Request fields inferred from game.js: taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GameClubRPC) RecvTaskRwd(ctx context.Context, req GameClubRecvTaskRwdRequest, opts ...RequestOption) (GameClubRecvTaskRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGameClubRecvTaskRwd, req, opts...)
}

// GirlsDay returns typed RPC helpers for the girlsDay namespace.
func (c *RPCClient) GirlsDay() GirlsDayRPC { return GirlsDayRPC{c: c} }

type GirlsDayRPC struct{ c *RPCClient }

// GirlsDayApplyRequest is the request body for gs.girlsDay.apply.
type GirlsDayApplyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// GirlsDayApplyResponse is the namespace-delta response for gs.girlsDay.apply.
type GirlsDayApplyResponse = RPCResponse[StateDelta]

// Apply calls gs.girlsDay.apply. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) Apply(ctx context.Context, req GirlsDayApplyRequest, opts ...RequestOption) (GirlsDayApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayApply, req, opts...)
}

// GirlsDayBindRequest is the request body for gs.girlsDay.bind.
type GirlsDayBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// GirlsDayBindResponse is the namespace-delta response for gs.girlsDay.bind.
type GirlsDayBindResponse = RPCResponse[StateDelta]

// Bind calls gs.girlsDay.bind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) Bind(ctx context.Context, req GirlsDayBindRequest, opts ...RequestOption) (GirlsDayBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayBind, req, opts...)
}

// GirlsDayEnterRequest is the request body for gs.girlsDay.enter.
type GirlsDayEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// GirlsDayEnterResponse is the namespace-delta response for gs.girlsDay.enter.
type GirlsDayEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.girlsDay.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) Enter(ctx context.Context, req GirlsDayEnterRequest, opts ...RequestOption) (GirlsDayEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayEnter, req, opts...)
}

// GirlsDayFrdStatesRequest is the request body for gs.girlsDay.frdStates.
type GirlsDayFrdStatesRequest struct {
	BatchId RPCID      `json:"batchId,omitempty"`
	UIDs    RPCUIDList `json:"uids,omitempty"`
}

// GirlsDayFrdStatesResponse is the namespace-delta response for gs.girlsDay.frdStates.
type GirlsDayFrdStatesResponse = RPCResponse[StateDelta]

// FrdStates calls gs.girlsDay.frdStates. Request fields inferred from game.js: batchId, uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) FrdStates(ctx context.Context, req GirlsDayFrdStatesRequest, opts ...RequestOption) (GirlsDayFrdStatesResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayFrdStates, req, opts...)
}

// GirlsDayRecvRequest is the request body for gs.girlsDay.recv.
type GirlsDayRecvRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// GirlsDayRecvResponse is the namespace-delta response for gs.girlsDay.recv.
type GirlsDayRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.girlsDay.recv. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) Recv(ctx context.Context, req GirlsDayRecvRequest, opts ...RequestOption) (GirlsDayRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayRecv, req, opts...)
}

// GirlsDayRejectRequest is the request body for gs.girlsDay.reject.
type GirlsDayRejectRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// GirlsDayRejectResponse is the namespace-delta response for gs.girlsDay.reject.
type GirlsDayRejectResponse = RPCResponse[StateDelta]

// Reject calls gs.girlsDay.reject. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) Reject(ctx context.Context, req GirlsDayRejectRequest, opts ...RequestOption) (GirlsDayRejectResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayReject, req, opts...)
}

// GirlsDayUnBindRequest is the request body for gs.girlsDay.unBind.
type GirlsDayUnBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// GirlsDayUnBindResponse is the namespace-delta response for gs.girlsDay.unBind.
type GirlsDayUnBindResponse = RPCResponse[StateDelta]

// UnBind calls gs.girlsDay.unBind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GirlsDayRPC) UnBind(ctx context.Context, req GirlsDayUnBindRequest, opts ...RequestOption) (GirlsDayUnBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGirlsDayUnBind, req, opts...)
}

// GiveGift returns typed RPC helpers for the giveGift namespace.
func (c *RPCClient) GiveGift() GiveGiftRPC { return GiveGiftRPC{c: c} }

type GiveGiftRPC struct{ c *RPCClient }

// GiveGiftGetGiveUidListRequest is the request body for gs.giveGift.getGiveUidList.
type GiveGiftGetGiveUidListRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
	GiftId  RPCID `json:"giftId,omitempty"`
}

// GiveGiftGetGiveUidListResponse is the namespace-delta response for gs.giveGift.getGiveUidList.
type GiveGiftGetGiveUidListResponse = RPCResponse[StateDelta]

// GetGiveUidList calls gs.giveGift.getGiveUidList. Request fields inferred from game.js: batchId, giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r GiveGiftRPC) GetGiveUidList(ctx context.Context, req GiveGiftGetGiveUidListRequest, opts ...RequestOption) (GiveGiftGetGiveUidListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCGiveGiftGetGiveUidList, req, opts...)
}

// HomeRqst returns typed RPC helpers for the homeRqst namespace.
func (c *RPCClient) HomeRqst() HomeRqstRPC { return HomeRqstRPC{c: c} }

type HomeRqstRPC struct{ c *RPCClient }

// HomeRqstShowBirdRequest is the request body for gs.homeRqst.showBird.
type HomeRqstShowBirdRequest struct {
	Time RPCInt `json:"time,omitempty"`
}

// HomeRqstShowBirdResponse is the namespace-delta response for gs.homeRqst.showBird.
type HomeRqstShowBirdResponse = RPCResponse[StateDelta]

// ShowBird calls gs.homeRqst.showBird. Request fields inferred from game.js: time.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r HomeRqstRPC) ShowBird(ctx context.Context, req HomeRqstShowBirdRequest, opts ...RequestOption) (HomeRqstShowBirdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCHomeRqstShowBird, req, opts...)
}

// IcoFrame returns typed RPC helpers for the icoFrame namespace.
func (c *RPCClient) IcoFrame() IcoFrameRPC { return IcoFrameRPC{c: c} }

type IcoFrameRPC struct{ c *RPCClient }

// IcoFrameActiveIcoFrameRequest is the request body for gs.icoFrame.activeIcoFrame.
type IcoFrameActiveIcoFrameRequest struct {
	FrameId RPCID `json:"frameId,omitempty"`
}

// IcoFrameActiveIcoFrameResponse is the namespace-delta response for gs.icoFrame.activeIcoFrame.
type IcoFrameActiveIcoFrameResponse = RPCResponse[StateDelta]

// ActiveIcoFrame calls gs.icoFrame.activeIcoFrame. Request fields inferred from game.js: frameId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r IcoFrameRPC) ActiveIcoFrame(ctx context.Context, req IcoFrameActiveIcoFrameRequest, opts ...RequestOption) (IcoFrameActiveIcoFrameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCIcoFrameActiveIcoFrame, req, opts...)
}

// IcoFrameChgIcoFrameRequest is the request body for gs.icoFrame.chgIcoFrame.
type IcoFrameChgIcoFrameRequest struct {
	FrameId RPCID `json:"frameId,omitempty"`
}

// IcoFrameChgIcoFrameResponse is the namespace-delta response for gs.icoFrame.chgIcoFrame.
type IcoFrameChgIcoFrameResponse = RPCResponse[StateDelta]

// ChgIcoFrame calls gs.icoFrame.chgIcoFrame. Request fields inferred from game.js: frameId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r IcoFrameRPC) ChgIcoFrame(ctx context.Context, req IcoFrameChgIcoFrameRequest, opts ...RequestOption) (IcoFrameChgIcoFrameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCIcoFrameChgIcoFrame, req, opts...)
}

// Im returns typed RPC helpers for the im namespace.
func (c *RPCClient) Im() ImRPC { return ImRPC{c: c} }

type ImRPC struct{ c *RPCClient }

// ImChangeChannelRequest is the empty request body for gs.im.changeChannel.
type ImChangeChannelRequest struct{}

// ImChangeChannelResponse is the namespace-delta response for gs.im.changeChannel.
type ImChangeChannelResponse = RPCResponse[StateDelta]

// ChangeChannel calls gs.im.changeChannel. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) ChangeChannel(ctx context.Context, req ImChangeChannelRequest, opts ...RequestOption) (ImChangeChannelResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImChangeChannel, req, opts...)
}

// ImDelChatPriRequest is the request body for gs.im.delChatPri.
type ImDelChatPriRequest struct {
	ToUID RPCUID `json:"toUid,omitempty"`
}

// ImDelChatPriResponse is the namespace-delta response for gs.im.delChatPri.
type ImDelChatPriResponse = RPCResponse[StateDelta]

// DelChatPri calls gs.im.delChatPri. Request fields inferred from game.js: toUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) DelChatPri(ctx context.Context, req ImDelChatPriRequest, opts ...RequestOption) (ImDelChatPriResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImDelChatPri, req, opts...)
}

// ImDelChatPriRedRequest is the request body for gs.im.delChatPriRed.
type ImDelChatPriRedRequest struct {
	ToUID RPCUID `json:"toUid,omitempty"`
}

// ImDelChatPriRedResponse is the namespace-delta response for gs.im.delChatPriRed.
type ImDelChatPriRedResponse = RPCResponse[StateDelta]

// DelChatPriRed calls gs.im.delChatPriRed. Request fields inferred from game.js: toUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) DelChatPriRed(ctx context.Context, req ImDelChatPriRedRequest, opts ...RequestOption) (ImDelChatPriRedResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImDelChatPriRed, req, opts...)
}

// ImEnterRequest is the request body for gs.im.enter.
type ImEnterRequest struct {
	RoomId        RPCID    `json:"roomId,omitempty"`
	LastIdx       RPCInt   `json:"lastIdx,omitempty"`
	LastEnterIdx  RPCInt   `json:"lastEnterIdx,omitempty"`
	MissingRanges RPCArray `json:"missingRanges,omitempty"`
}

// ImEnterResponse is the namespace-delta response for gs.im.enter.
type ImEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.im.enter. Request fields inferred from game.js: roomId, lastIdx, lastEnterIdx, missingRanges.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) Enter(ctx context.Context, req ImEnterRequest, opts ...RequestOption) (ImEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImEnter, req, opts...)
}

// ImGetChannelIdRequest is the empty request body for gs.im.getChannelId.
type ImGetChannelIdRequest struct{}

// ImGetChannelIdResponse is the namespace-delta response for gs.im.getChannelId.
type ImGetChannelIdResponse = RPCResponse[StateDelta]

// GetChannelId calls gs.im.getChannelId. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) GetChannelId(ctx context.Context, req ImGetChannelIdRequest, opts ...RequestOption) (ImGetChannelIdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImGetChannelId, req, opts...)
}

// ImReadRequest is the request body for gs.im.read.
type ImReadRequest struct {
	MsgId RPCID `json:"msgId,omitempty"`
}

// ImReadResponse is the namespace-delta response for gs.im.read.
type ImReadResponse = RPCResponse[StateDelta]

// Read calls gs.im.read. Request fields inferred from game.js: msgId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) Read(ctx context.Context, req ImReadRequest, opts ...RequestOption) (ImReadResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImRead, req, opts...)
}

// ImRefuseStrangerRequest is the request body for gs.im.refuseStranger.
type ImRefuseStrangerRequest struct {
	IsRefuse RPCBool `json:"isRefuse,omitempty"`
}

// ImRefuseStrangerResponse is the namespace-delta response for gs.im.refuseStranger.
type ImRefuseStrangerResponse = RPCResponse[StateDelta]

// RefuseStranger calls gs.im.refuseStranger. Request fields inferred from game.js: isRefuse.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) RefuseStranger(ctx context.Context, req ImRefuseStrangerRequest, opts ...RequestOption) (ImRefuseStrangerResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImRefuseStranger, req, opts...)
}

// ImSendRequest is the request body for gs.im.send.
type ImSendRequest struct {
	RoomId  RPCID     `json:"roomId,omitempty"`
	Type    RPCInt    `json:"type,omitempty"`
	Content RPCString `json:"content,omitempty"`
	Cms     RPCString `json:"cms,omitempty"`
	Ext     RPCObject `json:"ext,omitempty"`
}

// ImSendResponse is the namespace-delta response for gs.im.send.
type ImSendResponse = RPCResponse[StateDelta]

// Send calls gs.im.send. Request fields inferred from game.js: roomId, type, content, cms, ext.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ImRPC) Send(ctx context.Context, req ImSendRequest, opts ...RequestOption) (ImSendResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCImSend, req, opts...)
}

// Index returns typed RPC helpers for the index namespace.
func (c *RPCClient) Index() IndexRPC { return IndexRPC{c: c} }

type IndexRPC struct{ c *RPCClient }

// IndexCreateUsrRequest is the request body for gs.index.createUsr.
type IndexCreateUsrRequest struct {
	AID      RPCUID    `json:"aid,omitempty"`
	GsIdx    RPCInt    `json:"gsIdx,omitempty"`
	Token    RPCString `json:"token,omitempty"`
	IsNative RPCBool   `json:"isNative,omitempty"`
	Nick     RPCString `json:"nick,omitempty"`
	Sex      RPCInt    `json:"sex,omitempty"`
	Ico      RPCString `json:"ico,omitempty"`
	Ext      RPCObject `json:"ext,omitempty"`
	Inviter  RPCObject `json:"inviter,omitempty"`
}

// IndexCreateUsrResponse is the namespace-delta response for gs.index.createUsr.
type IndexCreateUsrResponse = RPCResponse[StateDelta]

// CreateUsr calls gs.index.createUsr. Request fields inferred from game.js: aid, gsIdx, token, isNative, nick, sex, ico, ext, inviter.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r IndexRPC) CreateUsr(ctx context.Context, req IndexCreateUsrRequest, opts ...RequestOption) (IndexCreateUsrResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCIndexCreateUsr, req, opts...)
}

// IndexDeviceInfo is the nested deviceInfo object sent during gs.index.login
// and gs.index.reLogin.
type IndexDeviceInfo struct {
	OSType         RPCString `json:"osType,omitempty"`
	DeviceID       RPCString `json:"deviceId,omitempty"`
	IsEmulator     RPCValue  `json:"isEmulator,omitempty"`
	OSVersion      RPCString `json:"osVersion,omitempty"`
	Brand          RPCString `json:"brand,omitempty"`
	Model          RPCString `json:"model,omitempty"`
	NetworkType    RPCString `json:"networkType,omitempty"`
	SysLanguage    RPCString `json:"sysLanguage,omitempty"`
	ScreenWidthPx  RPCString `json:"screenWidthPx,omitempty"`
	ScreenHeightPx RPCString `json:"screenHeightPx,omitempty"`
	DeviceType     RPCString `json:"deviceType,omitempty"`
	AppVersion     RPCString `json:"appVersion,omitempty"`
}

// IndexLoginRequest is the request body for gs.index.login.
type IndexLoginRequest struct {
	AID         RPCUID          `json:"aid,omitempty"`
	GsIdx       RPCInt          `json:"gsIdx,omitempty"`
	Token       RPCString       `json:"token,omitempty"`
	OSType      RPCInt          `json:"osType,omitempty"`
	IsNative    RPCBool         `json:"isNative,omitempty"`
	DeviceID    RPCString       `json:"deviceId,omitempty"`
	IsSimulator RPCInt          `json:"isSimulator,omitempty"`
	DeviceInfo  IndexDeviceInfo `json:"deviceInfo,omitempty"`
	Inviter     RPCObject       `json:"inviter,omitempty"`
	ShareExt    RPCObject       `json:"shareExt,omitempty"`
	Version     RPCString       `json:"version,omitempty"`
	Area        RPCString       `json:"area,omitempty"`
	ChnID       RPCInt          `json:"chnId,omitempty"`
}

// IndexLoginResponse is the namespace-delta response for gs.index.login.
type IndexLoginResponse = RPCResponse[StateDelta]

// Login calls gs.index.login. Request fields inferred from game.js: aid, gsIdx, token, osType, isNative, deviceId, isSimulator, deviceInfo, inviter, shareExt, version, area, chnId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r IndexRPC) Login(ctx context.Context, req IndexLoginRequest, opts ...RequestOption) (IndexLoginResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCIndexLogin, req, opts...)
}

// IndexReLoginRequest has the same request body shape as gs.index.login.
type IndexReLoginRequest = IndexLoginRequest

// IndexReLoginResponse is the namespace-delta response for gs.index.reLogin.
type IndexReLoginResponse = RPCResponse[StateDelta]

// ReLogin calls gs.index.reLogin. Request fields inferred from game.js: aid, gsIdx, token, osType, isNative, deviceId, isSimulator, deviceInfo, inviter, shareExt, version, area, chnId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r IndexRPC) ReLogin(ctx context.Context, req IndexReLoginRequest, opts ...RequestOption) (IndexReLoginResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCIndexReLogin, req, opts...)
}

// Mail returns typed RPC helpers for the mail namespace.
func (c *RPCClient) Mail() MailRPC { return MailRPC{c: c} }

type MailRPC struct{ c *RPCClient }

// MailDelRequest is the request body for gs.mail.del.
type MailDelRequest struct {
	MsId  RPCID     `json:"msId,omitempty"`
	AllId RPCIDList `json:"allId,omitempty"`
}

// MailDelResponse is the namespace-delta response for gs.mail.del.
type MailDelResponse = RPCResponse[StateDelta]

// Del calls gs.mail.del. Request fields inferred from game.js: msId, allId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) Del(ctx context.Context, req MailDelRequest, opts ...RequestOption) (MailDelResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailDel, req, opts...)
}

// MailDelOneKeyRequest is the request body for gs.mail.delOneKey.
type MailDelOneKeyRequest struct {
	Mode RPCInt `json:"mode,omitempty"`
}

// MailDelOneKeyResponse is the namespace-delta response for gs.mail.delOneKey.
type MailDelOneKeyResponse = RPCResponse[StateDelta]

// DelOneKey calls gs.mail.delOneKey. Request fields inferred from game.js: mode.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) DelOneKey(ctx context.Context, req MailDelOneKeyRequest, opts ...RequestOption) (MailDelOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailDelOneKey, req, opts...)
}

// MailGetListRequest is the empty request body for gs.mail.getList.
type MailGetListRequest struct{}

// MailGetListResponse is the namespace-delta response for gs.mail.getList.
type MailGetListResponse = RPCResponse[StateDelta]

// GetList calls gs.mail.getList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) GetList(ctx context.Context, req MailGetListRequest, opts ...RequestOption) (MailGetListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailGetList, req, opts...)
}

// MailOperRequest is the request body for gs.mail.oper.
type MailOperRequest struct {
	MsId  RPCID     `json:"msId,omitempty"`
	AllId RPCIDList `json:"allId,omitempty"`
}

// MailOperResponse is the namespace-delta response for gs.mail.oper.
type MailOperResponse = RPCResponse[StateDelta]

// Oper calls gs.mail.oper. Request fields inferred from game.js: msId, allId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) Oper(ctx context.Context, req MailOperRequest, opts ...RequestOption) (MailOperResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailOper, req, opts...)
}

// MailPickRequest is the request body for gs.mail.pick.
type MailPickRequest struct {
	MsId  RPCID     `json:"msId,omitempty"`
	AllId RPCIDList `json:"allId,omitempty"`
}

// MailPickResponse is the namespace-delta response for gs.mail.pick.
type MailPickResponse = RPCResponse[StateDelta]

// Pick calls gs.mail.pick. Request fields inferred from game.js: msId, allId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) Pick(ctx context.Context, req MailPickRequest, opts ...RequestOption) (MailPickResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailPick, req, opts...)
}

// MailPickOneKeyRequest is the empty request body for gs.mail.pickOneKey.
type MailPickOneKeyRequest struct{}

// MailPickOneKeyResponse is the namespace-delta response for gs.mail.pickOneKey.
type MailPickOneKeyResponse = RPCResponse[StateDelta]

// PickOneKey calls gs.mail.pickOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) PickOneKey(ctx context.Context, req MailPickOneKeyRequest, opts ...RequestOption) (MailPickOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailPickOneKey, req, opts...)
}

// MailReadRequest is the request body for gs.mail.read.
type MailReadRequest struct {
	MsId  RPCID     `json:"msId,omitempty"`
	AllId RPCIDList `json:"allId,omitempty"`
}

// MailReadResponse is the namespace-delta response for gs.mail.read.
type MailReadResponse = RPCResponse[StateDelta]

// Read calls gs.mail.read. Request fields inferred from game.js: msId, allId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MailRPC) Read(ctx context.Context, req MailReadRequest, opts ...RequestOption) (MailReadResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMailRead, req, opts...)
}

// MiniGame returns typed RPC helpers for the miniGame namespace.
func (c *RPCClient) MiniGame() MiniGameRPC { return MiniGameRPC{c: c} }

type MiniGameRPC struct{ c *RPCClient }

// MiniGameEndMiniGameRequest is the request body for gs.miniGame.endMiniGame.
type MiniGameEndMiniGameRequest struct {
	CopyId RPCID  `json:"copyId,omitempty"`
	Type   RPCInt `json:"type,omitempty"`
}

// MiniGameEndMiniGameResponse is the namespace-delta response for gs.miniGame.endMiniGame.
type MiniGameEndMiniGameResponse = RPCResponse[StateDelta]

// EndMiniGame calls gs.miniGame.endMiniGame. Request fields inferred from game.js: copyId, type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiniGameRPC) EndMiniGame(ctx context.Context, req MiniGameEndMiniGameRequest, opts ...RequestOption) (MiniGameEndMiniGameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiniGameEndMiniGame, req, opts...)
}

// MiniGameEnterMiniGameRequest is the request body for gs.miniGame.enterMiniGame.
type MiniGameEnterMiniGameRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// MiniGameEnterMiniGameResponse is the namespace-delta response for gs.miniGame.enterMiniGame.
type MiniGameEnterMiniGameResponse = RPCResponse[StateDelta]

// EnterMiniGame calls gs.miniGame.enterMiniGame. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiniGameRPC) EnterMiniGame(ctx context.Context, req MiniGameEnterMiniGameRequest, opts ...RequestOption) (MiniGameEnterMiniGameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiniGameEnterMiniGame, req, opts...)
}

// MiniGameStartMiniGameRequest is the request body for gs.miniGame.startMiniGame.
type MiniGameStartMiniGameRequest struct {
	CopyId RPCID  `json:"copyId,omitempty"`
	Type   RPCInt `json:"type,omitempty"`
}

// MiniGameStartMiniGameResponse is the namespace-delta response for gs.miniGame.startMiniGame.
type MiniGameStartMiniGameResponse = RPCResponse[StateDelta]

// StartMiniGame calls gs.miniGame.startMiniGame. Request fields inferred from game.js: copyId, type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiniGameRPC) StartMiniGame(ctx context.Context, req MiniGameStartMiniGameRequest, opts ...RequestOption) (MiniGameStartMiniGameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiniGameStartMiniGame, req, opts...)
}

// Misc returns typed RPC helpers for the misc namespace.
func (c *RPCClient) Misc() MiscRPC { return MiscRPC{c: c} }

type MiscRPC struct{ c *RPCClient }

// MiscBuyMonthCardRequest is the request body for gs.misc.buyMonthCard.
type MiscBuyMonthCardRequest struct {
	Num RPCInt `json:"num,omitempty"`
}

// MiscBuyMonthCardResponse is the namespace-delta response for gs.misc.buyMonthCard.
type MiscBuyMonthCardResponse = RPCResponse[StateDelta]

// BuyMonthCard calls gs.misc.buyMonthCard. Request fields inferred from game.js: num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiscRPC) BuyMonthCard(ctx context.Context, req MiscBuyMonthCardRequest, opts ...RequestOption) (MiscBuyMonthCardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiscBuyMonthCard, req, opts...)
}

// MiscGetAdvanceWashItemRequest is the request body for gs.misc.getAdvanceWashItem.
type MiscGetAdvanceWashItemRequest struct {
	ItemId RPCID  `json:"itemId,omitempty"`
	Num    RPCInt `json:"num,omitempty"`
}

// MiscGetAdvanceWashItemResponse is the namespace-delta response for gs.misc.getAdvanceWashItem.
type MiscGetAdvanceWashItemResponse = RPCResponse[StateDelta]

// GetAdvanceWashItem calls gs.misc.getAdvanceWashItem. Request fields inferred from game.js: itemId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiscRPC) GetAdvanceWashItem(ctx context.Context, req MiscGetAdvanceWashItemRequest, opts ...RequestOption) (MiscGetAdvanceWashItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiscGetAdvanceWashItem, req, opts...)
}

// MiscRecvMsgPushRwdRequest is the empty request body for gs.misc.recvMsgPushRwd.
type MiscRecvMsgPushRwdRequest struct{}

// MiscRecvMsgPushRwdResponse is the namespace-delta response for gs.misc.recvMsgPushRwd.
type MiscRecvMsgPushRwdResponse = RPCResponse[StateDelta]

// RecvMsgPushRwd calls gs.misc.recvMsgPushRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiscRPC) RecvMsgPushRwd(ctx context.Context, req MiscRecvMsgPushRwdRequest, opts ...RequestOption) (MiscRecvMsgPushRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiscRecvMsgPushRwd, req, opts...)
}

// MiscReportCheckBwRequest is the request body for gs.misc.reportCheckBw.
type MiscReportCheckBwRequest struct {
	DstUID RPCUID `json:"dstUid,omitempty"`
}

// MiscReportCheckBwResponse is the namespace-delta response for gs.misc.reportCheckBw.
type MiscReportCheckBwResponse = RPCResponse[StateDelta]

// ReportCheckBw calls gs.misc.reportCheckBw. Request fields inferred from game.js: dstUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiscRPC) ReportCheckBw(ctx context.Context, req MiscReportCheckBwRequest, opts ...RequestOption) (MiscReportCheckBwResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiscReportCheckBw, req, opts...)
}

// MiscSellFlowerRequest is the request body for gs.misc.sellFlower.
type MiscSellFlowerRequest struct {
	FlowerId RPCID  `json:"flowerId,omitempty"`
	Num      RPCInt `json:"num,omitempty"`
}

// MiscSellFlowerResponse is the namespace-delta response for gs.misc.sellFlower.
type MiscSellFlowerResponse = RPCResponse[StateDelta]

// SellFlower calls gs.misc.sellFlower. Request fields inferred from game.js: flowerId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiscRPC) SellFlower(ctx context.Context, req MiscSellFlowerRequest, opts ...RequestOption) (MiscSellFlowerResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiscSellFlower, req, opts...)
}

// MiscSyncItemHideRequest is the request body for gs.misc.syncItemHide.
type MiscSyncItemHideRequest struct {
	Version RPCString `json:"version,omitempty"`
	ItemIds RPCIDList `json:"itemIds,omitempty"`
}

// MiscSyncItemHideResponse is the namespace-delta response for gs.misc.syncItemHide.
type MiscSyncItemHideResponse = RPCResponse[StateDelta]

// SyncItemHide calls gs.misc.syncItemHide. Request fields inferred from game.js: version, itemIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MiscRPC) SyncItemHide(ctx context.Context, req MiscSyncItemHideRequest, opts ...RequestOption) (MiscSyncItemHideResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMiscSyncItemHide, req, opts...)
}

// MonthFlower returns typed RPC helpers for the monthFlower namespace.
func (c *RPCClient) MonthFlower() MonthFlowerRPC { return MonthFlowerRPC{c: c} }

type MonthFlowerRPC struct{ c *RPCClient }

// MonthFlowerBuyRequest is the empty request body for gs.monthFlower.buy.
type MonthFlowerBuyRequest struct{}

// MonthFlowerBuyResponse is the namespace-delta response for gs.monthFlower.buy.
type MonthFlowerBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.monthFlower.buy. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MonthFlowerRPC) Buy(ctx context.Context, req MonthFlowerBuyRequest, opts ...RequestOption) (MonthFlowerBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMonthFlowerBuy, req, opts...)
}

// MonthFlowerEnterRequest is the empty request body for gs.monthFlower.enter.
type MonthFlowerEnterRequest struct{}

// MonthFlowerEnterResponse is the namespace-delta response for gs.monthFlower.enter.
type MonthFlowerEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.monthFlower.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r MonthFlowerRPC) Enter(ctx context.Context, req MonthFlowerEnterRequest, opts ...RequestOption) (MonthFlowerEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCMonthFlowerEnter, req, opts...)
}

// Oppt returns typed RPC helpers for the oppt namespace.
func (c *RPCClient) Oppt() OpptRPC { return OpptRPC{c: c} }

type OpptRPC struct{ c *RPCClient }

// OpptGetDetailOpptsRequest is the request body for gs.oppt.getDetailOppts.
type OpptGetDetailOpptsRequest struct {
	UIDs    RPCUIDList    `json:"uids,omitempty"`
	ExtKeys RPCStringList `json:"extKeys,omitempty"`
}

// OpptGetDetailOpptsResponse is the namespace-delta response for gs.oppt.getDetailOppts.
type OpptGetDetailOpptsResponse = RPCResponse[StateDelta]

// GetDetailOppts calls gs.oppt.getDetailOppts. Request fields inferred from game.js: uids, extKeys.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OpptRPC) GetDetailOppts(ctx context.Context, req OpptGetDetailOpptsRequest, opts ...RequestOption) (OpptGetDetailOpptsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOpptGetDetailOppts, req, opts...)
}

// OpptGetOpptRequest is the request body for gs.oppt.getOppt.
type OpptGetOpptRequest struct {
	UID RPCUID `json:"uid,omitempty"`
}

// OpptGetOpptResponse is the namespace-delta response for gs.oppt.getOppt.
type OpptGetOpptResponse = RPCResponse[StateDelta]

// GetOppt calls gs.oppt.getOppt. Request fields inferred from game.js: uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OpptRPC) GetOppt(ctx context.Context, req OpptGetOpptRequest, opts ...RequestOption) (OpptGetOpptResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOpptGetOppt, req, opts...)
}

// OpptGetOpptsRequest is the request body for gs.oppt.getOppts.
type OpptGetOpptsRequest struct {
	UIDs  RPCUIDList `json:"uids,omitempty"`
	Force RPCBool    `json:"force,omitempty"`
}

// OpptGetOpptsResponse is the namespace-delta response for gs.oppt.getOppts.
type OpptGetOpptsResponse = RPCResponse[StateDelta]

// GetOppts calls gs.oppt.getOppts. Request fields inferred from game.js: uids, force.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OpptRPC) GetOppts(ctx context.Context, req OpptGetOpptsRequest, opts ...RequestOption) (OpptGetOpptsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOpptGetOppts, req, opts...)
}

// OrderCustomer returns typed RPC helpers for the orderCustomer namespace.
func (c *RPCClient) OrderCustomer() OrderCustomerRPC { return OrderCustomerRPC{c: c} }

type OrderCustomerRPC struct{ c *RPCClient }

// OrderCustomerFinishOrderRequest is the request body for gs.orderCustomer.finishOrder.
type OrderCustomerFinishOrderRequest struct {
	NPCId RPCID `json:"npcId,omitempty"`
}

// OrderCustomerFinishOrderResponse is the namespace-delta response for gs.orderCustomer.finishOrder.
type OrderCustomerFinishOrderResponse = RPCResponse[StateDelta]

// FinishOrder calls gs.orderCustomer.finishOrder. Request fields inferred from game.js: npcId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderCustomerRPC) FinishOrder(ctx context.Context, req OrderCustomerFinishOrderRequest, opts ...RequestOption) (OrderCustomerFinishOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderCustomerFinishOrder, req, opts...)
}

// OrderCustomerGenOrderRequest is the request body for gs.orderCustomer.genOrder.
type OrderCustomerGenOrderRequest struct {
	GuestNPCIdList []int32 `json:"guestNpcIdList,omitempty"`
}

// OrderCustomerGenOrderResponse is the namespace-delta response for gs.orderCustomer.genOrder.
type OrderCustomerGenOrderResponse = RPCResponse[StateDelta]

// GenOrder calls gs.orderCustomer.genOrder. Request fields inferred from game.js: guestNpcIdList.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderCustomerRPC) GenOrder(ctx context.Context, req OrderCustomerGenOrderRequest, opts ...RequestOption) (OrderCustomerGenOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderCustomerGenOrder, req, opts...)
}

// OrderCustomerRejectOrderRequest is the request body for gs.orderCustomer.rejectOrder.
type OrderCustomerRejectOrderRequest struct {
	NPCId RPCID `json:"npcId,omitempty"`
}

// OrderCustomerRejectOrderResponse is the namespace-delta response for gs.orderCustomer.rejectOrder.
type OrderCustomerRejectOrderResponse = RPCResponse[StateDelta]

// RejectOrder calls gs.orderCustomer.rejectOrder. Request fields inferred from game.js: npcId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderCustomerRPC) RejectOrder(ctx context.Context, req OrderCustomerRejectOrderRequest, opts ...RequestOption) (OrderCustomerRejectOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderCustomerRejectOrder, req, opts...)
}

// OrderFlower returns typed RPC helpers for the orderFlower namespace.
func (c *RPCClient) OrderFlower() OrderFlowerRPC { return OrderFlowerRPC{c: c} }

type OrderFlowerRPC struct{ c *RPCClient }

// OrderFlowerEnterRequest is the empty request body for gs.orderFlower.enter.
type OrderFlowerEnterRequest struct{}

// OrderFlowerEnterResponse is the namespace-delta response for gs.orderFlower.enter.
type OrderFlowerEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.orderFlower.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderFlowerRPC) Enter(ctx context.Context, req OrderFlowerEnterRequest, opts ...RequestOption) (OrderFlowerEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderFlowerEnter, req, opts...)
}

// OrderFlowerFinishDecorateOrderRequest is the empty request body for gs.orderFlower.finishDecorateOrder.
type OrderFlowerFinishDecorateOrderRequest struct{}

// OrderFlowerFinishDecorateOrderResponse is the namespace-delta response for gs.orderFlower.finishDecorateOrder.
type OrderFlowerFinishDecorateOrderResponse = RPCResponse[StateDelta]

// FinishDecorateOrder calls gs.orderFlower.finishDecorateOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderFlowerRPC) FinishDecorateOrder(ctx context.Context, req OrderFlowerFinishDecorateOrderRequest, opts ...RequestOption) (OrderFlowerFinishDecorateOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderFlowerFinishDecorateOrder, req, opts...)
}

// OrderFlowerFinishOrderRequest is the request body for gs.orderFlower.finishOrder.
type OrderFlowerFinishOrderRequest struct {
	BoxId RPCID `json:"boxId,omitempty"`
}

// OrderFlowerFinishOrderResponse is the namespace-delta response for gs.orderFlower.finishOrder.
type OrderFlowerFinishOrderResponse = RPCResponse[StateDelta]

// FinishOrder calls gs.orderFlower.finishOrder. Request fields inferred from game.js: boxId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderFlowerRPC) FinishOrder(ctx context.Context, req OrderFlowerFinishOrderRequest, opts ...RequestOption) (OrderFlowerFinishOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderFlowerFinishOrder, req, opts...)
}

// OrderFlowerFinishSatinOrderRequest is the empty request body for gs.orderFlower.finishSatinOrder.
type OrderFlowerFinishSatinOrderRequest struct{}

// OrderFlowerFinishSatinOrderResponse is the namespace-delta response for gs.orderFlower.finishSatinOrder.
type OrderFlowerFinishSatinOrderResponse = RPCResponse[StateDelta]

// FinishSatinOrder calls gs.orderFlower.finishSatinOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderFlowerRPC) FinishSatinOrder(ctx context.Context, req OrderFlowerFinishSatinOrderRequest, opts ...RequestOption) (OrderFlowerFinishSatinOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderFlowerFinishSatinOrder, req, opts...)
}

// OrderFlowerRecvOrderRwdRequest is the request body for gs.orderFlower.recvOrderRwd.
type OrderFlowerRecvOrderRwdRequest struct {
	Target int32 `json:"target,omitempty"`
}

// OrderFlowerRecvOrderRwdResponse is the namespace-delta response for gs.orderFlower.recvOrderRwd.
type OrderFlowerRecvOrderRwdResponse = RPCResponse[StateDelta]

// RecvOrderRwd calls gs.orderFlower.recvOrderRwd. Request fields inferred from game.js: target.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderFlowerRPC) RecvOrderRwd(ctx context.Context, req OrderFlowerRecvOrderRwdRequest, opts ...RequestOption) (OrderFlowerRecvOrderRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderFlowerRecvOrderRwd, req, opts...)
}

// OrderFlowerRessuieOrderRwdRequest is the empty request body for gs.orderFlower.ressuieOrderRwd.
type OrderFlowerRessuieOrderRwdRequest struct{}

// OrderFlowerRessuieOrderRwdResponse is the namespace-delta response for gs.orderFlower.ressuieOrderRwd.
type OrderFlowerRessuieOrderRwdResponse = RPCResponse[StateDelta]

// RessuieOrderRwd calls gs.orderFlower.ressuieOrderRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderFlowerRPC) RessuieOrderRwd(ctx context.Context, req OrderFlowerRessuieOrderRwdRequest, opts ...RequestOption) (OrderFlowerRessuieOrderRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderFlowerRessuieOrderRwd, req, opts...)
}

// OrderPalace returns typed RPC helpers for the orderPalace namespace.
func (c *RPCClient) OrderPalace() OrderPalaceRPC { return OrderPalaceRPC{c: c} }

type OrderPalaceRPC struct{ c *RPCClient }

// OrderPalaceEnterRequest is the empty request body for gs.orderPalace.enter.
type OrderPalaceEnterRequest struct{}

// OrderPalaceEnterResponse is the namespace-delta response for gs.orderPalace.enter.
type OrderPalaceEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.orderPalace.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderPalaceRPC) Enter(ctx context.Context, req OrderPalaceEnterRequest, opts ...RequestOption) (OrderPalaceEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderPalaceEnter, req, opts...)
}

// OrderPalaceFinishOrderRequest is the empty request body for gs.orderPalace.finishOrder.
type OrderPalaceFinishOrderRequest struct{}

// OrderPalaceFinishOrderResponse is the namespace-delta response for gs.orderPalace.finishOrder.
type OrderPalaceFinishOrderResponse = RPCResponse[StateDelta]

// FinishOrder calls gs.orderPalace.finishOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderPalaceRPC) FinishOrder(ctx context.Context, req OrderPalaceFinishOrderRequest, opts ...RequestOption) (OrderPalaceFinishOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderPalaceFinishOrder, req, opts...)
}

// OrderPalaceGetOrderRcdListRequest is the empty request body for gs.orderPalace.getOrderRcdList.
type OrderPalaceGetOrderRcdListRequest struct{}

// OrderPalaceGetOrderRcdListResponse is the namespace-delta response for gs.orderPalace.getOrderRcdList.
type OrderPalaceGetOrderRcdListResponse = RPCResponse[StateDelta]

// GetOrderRcdList calls gs.orderPalace.getOrderRcdList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderPalaceRPC) GetOrderRcdList(ctx context.Context, req OrderPalaceGetOrderRcdListRequest, opts ...RequestOption) (OrderPalaceGetOrderRcdListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderPalaceGetOrderRcdList, req, opts...)
}

// OrderPalaceRefreshOrderRequest is the empty request body for gs.orderPalace.refreshOrder.
type OrderPalaceRefreshOrderRequest struct{}

// OrderPalaceRefreshOrderResponse is the namespace-delta response for gs.orderPalace.refreshOrder.
type OrderPalaceRefreshOrderResponse = RPCResponse[StateDelta]

// RefreshOrder calls gs.orderPalace.refreshOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderPalaceRPC) RefreshOrder(ctx context.Context, req OrderPalaceRefreshOrderRequest, opts ...RequestOption) (OrderPalaceRefreshOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderPalaceRefreshOrder, req, opts...)
}

// OrderTeam returns typed RPC helpers for the orderTeam namespace.
func (c *RPCClient) OrderTeam() OrderTeamRPC { return OrderTeamRPC{c: c} }

type OrderTeamRPC struct{ c *RPCClient }

// OrderTeamRecvRwdRequest is the empty request body for gs.orderTeam.recvRwd.
type OrderTeamRecvRwdRequest struct{}

// OrderTeamRecvRwdResponse is the namespace-delta response for gs.orderTeam.recvRwd.
type OrderTeamRecvRwdResponse = RPCResponse[StateDelta]

// RecvRwd calls gs.orderTeam.recvRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderTeamRPC) RecvRwd(ctx context.Context, req OrderTeamRecvRwdRequest, opts ...RequestOption) (OrderTeamRecvRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderTeamRecvRwd, req, opts...)
}

// OrderTeamRefreshOrderRequest is the empty request body for gs.orderTeam.refreshOrder.
type OrderTeamRefreshOrderRequest struct{}

// OrderTeamRefreshOrderResponse is the namespace-delta response for gs.orderTeam.refreshOrder.
type OrderTeamRefreshOrderResponse = RPCResponse[StateDelta]

// RefreshOrder calls gs.orderTeam.refreshOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderTeamRPC) RefreshOrder(ctx context.Context, req OrderTeamRefreshOrderRequest, opts ...RequestOption) (OrderTeamRefreshOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderTeamRefreshOrder, req, opts...)
}

// OrderTeamStoreOrderRequest is the empty request body for gs.orderTeam.storeOrder.
type OrderTeamStoreOrderRequest struct{}

// OrderTeamStoreOrderResponse is the namespace-delta response for gs.orderTeam.storeOrder.
type OrderTeamStoreOrderResponse = RPCResponse[StateDelta]

// StoreOrder calls gs.orderTeam.storeOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderTeamRPC) StoreOrder(ctx context.Context, req OrderTeamStoreOrderRequest, opts ...RequestOption) (OrderTeamStoreOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderTeamStoreOrder, req, opts...)
}

// OrderTeamSubmitOrderRequest is the empty request body for gs.orderTeam.submitOrder.
type OrderTeamSubmitOrderRequest struct{}

// OrderTeamSubmitOrderResponse is the namespace-delta response for gs.orderTeam.submitOrder.
type OrderTeamSubmitOrderResponse = RPCResponse[StateDelta]

// SubmitOrder calls gs.orderTeam.submitOrder. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderTeamRPC) SubmitOrder(ctx context.Context, req OrderTeamSubmitOrderRequest, opts ...RequestOption) (OrderTeamSubmitOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderTeamSubmitOrder, req, opts...)
}

// OrderTeamTakeOrderRequest is the request body for gs.orderTeam.takeOrder.
type OrderTeamTakeOrderRequest struct {
	IsAgree RPCBool `json:"isAgree,omitempty"`
	IsCost  RPCBool `json:"isCost,omitempty"`
}

// OrderTeamTakeOrderResponse is the namespace-delta response for gs.orderTeam.takeOrder.
type OrderTeamTakeOrderResponse = RPCResponse[StateDelta]

// TakeOrder calls gs.orderTeam.takeOrder. Request fields inferred from game.js: isAgree, isCost.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderTeamRPC) TakeOrder(ctx context.Context, req OrderTeamTakeOrderRequest, opts ...RequestOption) (OrderTeamTakeOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderTeamTakeOrder, req, opts...)
}

// OrderTeamTakeStoredOrderRequest is the request body for gs.orderTeam.takeStoredOrder.
type OrderTeamTakeStoredOrderRequest struct {
	NPCId RPCID `json:"npcId,omitempty"`
}

// OrderTeamTakeStoredOrderResponse is the namespace-delta response for gs.orderTeam.takeStoredOrder.
type OrderTeamTakeStoredOrderResponse = RPCResponse[StateDelta]

// TakeStoredOrder calls gs.orderTeam.takeStoredOrder. Request fields inferred from game.js: npcId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r OrderTeamRPC) TakeStoredOrder(ctx context.Context, req OrderTeamTakeStoredOrderRequest, opts ...RequestOption) (OrderTeamTakeStoredOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCOrderTeamTakeStoredOrder, req, opts...)
}

// Pearl returns typed RPC helpers for the pearl namespace.
func (c *RPCClient) Pearl() PearlRPC { return PearlRPC{c: c} }

type PearlRPC struct{ c *RPCClient }

// PearlDrawRequest is the request body for gs.pearl.draw.
type PearlDrawRequest struct {
	Count RPCInt `json:"count,omitempty"`
}

// PearlDrawResponse is the namespace-delta response for gs.pearl.draw.
type PearlDrawResponse = RPCResponse[StateDelta]

// Draw calls gs.pearl.draw. Request fields inferred from game.js: count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) Draw(ctx context.Context, req PearlDrawRequest, opts ...RequestOption) (PearlDrawResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlDraw, req, opts...)
}

// PearlGetHireMyLogRequest is the empty request body for gs.pearl.getHireMyLog.
type PearlGetHireMyLogRequest struct{}

// PearlGetHireMyLogResponse is the namespace-delta response for gs.pearl.getHireMyLog.
type PearlGetHireMyLogResponse = RPCResponse[StateDelta]

// GetHireMyLog calls gs.pearl.getHireMyLog. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) GetHireMyLog(ctx context.Context, req PearlGetHireMyLogRequest, opts ...RequestOption) (PearlGetHireMyLogResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlGetHireMyLog, req, opts...)
}

// PearlGetHireStateByUidsRequest is the request body for gs.pearl.getHireStateByUids.
type PearlGetHireStateByUidsRequest struct {
	UIDs RPCUIDList `json:"uids,omitempty"`
}

// PearlGetHireStateByUidsResponse is the namespace-delta response for gs.pearl.getHireStateByUids.
type PearlGetHireStateByUidsResponse = RPCResponse[StateDelta]

// GetHireStateByUids calls gs.pearl.getHireStateByUids. Request fields inferred from game.js: uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) GetHireStateByUids(ctx context.Context, req PearlGetHireStateByUidsRequest, opts ...RequestOption) (PearlGetHireStateByUidsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlGetHireStateByUids, req, opts...)
}

// PearlGetMyHireLogRequest is the empty request body for gs.pearl.getMyHireLog.
type PearlGetMyHireLogRequest struct{}

// PearlGetMyHireLogResponse is the namespace-delta response for gs.pearl.getMyHireLog.
type PearlGetMyHireLogResponse = RPCResponse[StateDelta]

// GetMyHireLog calls gs.pearl.getMyHireLog. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) GetMyHireLog(ctx context.Context, req PearlGetMyHireLogRequest, opts ...RequestOption) (PearlGetMyHireLogResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlGetMyHireLog, req, opts...)
}

// PearlGetRecommendListRequest is the empty request body for gs.pearl.getRecommendList.
type PearlGetRecommendListRequest struct{}

// PearlGetRecommendListResponse is the namespace-delta response for gs.pearl.getRecommendList.
type PearlGetRecommendListResponse = RPCResponse[StateDelta]

// GetRecommendList calls gs.pearl.getRecommendList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) GetRecommendList(ctx context.Context, req PearlGetRecommendListRequest, opts ...RequestOption) (PearlGetRecommendListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlGetRecommendList, req, opts...)
}

// PearlRecvDailyFreeRequest is the empty request body for gs.pearl.recvDailyFree.
type PearlRecvDailyFreeRequest struct{}

// PearlRecvDailyFreeResponse is the namespace-delta response for gs.pearl.recvDailyFree.
type PearlRecvDailyFreeResponse = RPCResponse[StateDelta]

// RecvDailyFree calls gs.pearl.recvDailyFree. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) RecvDailyFree(ctx context.Context, req PearlRecvDailyFreeRequest, opts ...RequestOption) (PearlRecvDailyFreeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlRecvDailyFree, req, opts...)
}

// PearlRefreshRequest is the empty request body for gs.pearl.refresh.
type PearlRefreshRequest struct{}

// PearlRefreshResponse is the namespace-delta response for gs.pearl.refresh.
type PearlRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.pearl.refresh. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) Refresh(ctx context.Context, req PearlRefreshRequest, opts ...RequestOption) (PearlRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlRefresh, req, opts...)
}

// PearlSetProtectStateRequest is the request body for gs.pearl.setProtectState.
type PearlSetProtectStateRequest struct {
	ProtectState RPCInt `json:"protectState,omitempty"`
}

// PearlSetProtectStateResponse is the namespace-delta response for gs.pearl.setProtectState.
type PearlSetProtectStateResponse = RPCResponse[StateDelta]

// SetProtectState calls gs.pearl.setProtectState. Request fields inferred from game.js: protectState.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlRPC) SetProtectState(ctx context.Context, req PearlSetProtectStateRequest, opts ...RequestOption) (PearlSetProtectStateResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlSetProtectState, req, opts...)
}

// PearlPlace returns typed RPC helpers for the pearlPlace namespace.
func (c *RPCClient) PearlPlace() PearlPlaceRPC { return PearlPlaceRPC{c: c} }

type PearlPlaceRPC struct{ c *RPCClient }

// PearlPlaceHireRequest is the request body for gs.pearlPlace.hire.
type PearlPlaceHireRequest struct {
	PlaceId RPCID  `json:"placeId,omitempty"`
	DstUID  RPCUID `json:"dstUid,omitempty"`
}

// PearlPlaceHireResponse is the namespace-delta response for gs.pearlPlace.hire.
type PearlPlaceHireResponse = RPCResponse[StateDelta]

// Hire calls gs.pearlPlace.hire. Request fields inferred from game.js: placeId, dstUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlPlaceRPC) Hire(ctx context.Context, req PearlPlaceHireRequest, opts ...RequestOption) (PearlPlaceHireResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlPlaceHire, req, opts...)
}

// PearlPlaceRecvRequest is the request body for gs.pearlPlace.recv.
type PearlPlaceRecvRequest struct {
	PlaceId RPCID `json:"placeId,omitempty"`
}

// PearlPlaceRecvResponse is the namespace-delta response for gs.pearlPlace.recv.
type PearlPlaceRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.pearlPlace.recv. Request fields inferred from game.js: placeId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlPlaceRPC) Recv(ctx context.Context, req PearlPlaceRecvRequest, opts ...RequestOption) (PearlPlaceRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlPlaceRecv, req, opts...)
}

// PearlPlaceRecvOneKeyRequest is the empty request body for gs.pearlPlace.recvOneKey.
type PearlPlaceRecvOneKeyRequest struct{}

// PearlPlaceRecvOneKeyResponse is the namespace-delta response for gs.pearlPlace.recvOneKey.
type PearlPlaceRecvOneKeyResponse = RPCResponse[StateDelta]

// RecvOneKey calls gs.pearlPlace.recvOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlPlaceRPC) RecvOneKey(ctx context.Context, req PearlPlaceRecvOneKeyRequest, opts ...RequestOption) (PearlPlaceRecvOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlPlaceRecvOneKey, req, opts...)
}

// PearlPlaceUnlockPlaceRequest is the request body for gs.pearlPlace.unlockPlace.
type PearlPlaceUnlockPlaceRequest struct {
	PlaceId RPCID `json:"placeId,omitempty"`
}

// PearlPlaceUnlockPlaceResponse is the namespace-delta response for gs.pearlPlace.unlockPlace.
type PearlPlaceUnlockPlaceResponse = RPCResponse[StateDelta]

// UnlockPlace calls gs.pearlPlace.unlockPlace. Request fields inferred from game.js: placeId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PearlPlaceRPC) UnlockPlace(ctx context.Context, req PearlPlaceUnlockPlaceRequest, opts ...RequestOption) (PearlPlaceUnlockPlaceResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPearlPlaceUnlockPlace, req, opts...)
}

// Photo returns typed RPC helpers for the photo namespace.
func (c *RPCClient) Photo() PhotoRPC { return PhotoRPC{c: c} }

type PhotoRPC struct{ c *RPCClient }

// PhotoBuyRequest is the request body for gs.photo.buy.
type PhotoBuyRequest struct {
	TempId RPCID `json:"tempId,omitempty"`
}

// PhotoBuyResponse is the namespace-delta response for gs.photo.buy.
type PhotoBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.photo.buy. Request fields inferred from game.js: tempId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) Buy(ctx context.Context, req PhotoBuyRequest, opts ...RequestOption) (PhotoBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoBuy, req, opts...)
}

// PhotoBuyTicketRequest is the request body for gs.photo.buyTicket.
type PhotoBuyTicketRequest struct {
	Num RPCInt `json:"num,omitempty"`
}

// PhotoBuyTicketResponse is the namespace-delta response for gs.photo.buyTicket.
type PhotoBuyTicketResponse = RPCResponse[StateDelta]

// BuyTicket calls gs.photo.buyTicket. Request fields inferred from game.js: num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) BuyTicket(ctx context.Context, req PhotoBuyTicketRequest, opts ...RequestOption) (PhotoBuyTicketResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoBuyTicket, req, opts...)
}

// PhotoCheckInviteRequest is the request body for gs.photo.checkInvite.
type PhotoCheckInviteRequest struct {
	InviteId RPCID `json:"inviteId,omitempty"`
}

// PhotoCheckInviteResponse is the namespace-delta response for gs.photo.checkInvite.
type PhotoCheckInviteResponse = RPCResponse[StateDelta]

// CheckInvite calls gs.photo.checkInvite. Request fields inferred from game.js: inviteId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) CheckInvite(ctx context.Context, req PhotoCheckInviteRequest, opts ...RequestOption) (PhotoCheckInviteResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoCheckInvite, req, opts...)
}

// PhotoCloseRoomRequest is the request body for gs.photo.closeRoom.
type PhotoCloseRoomRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// PhotoCloseRoomResponse is the namespace-delta response for gs.photo.closeRoom.
type PhotoCloseRoomResponse = RPCResponse[StateDelta]

// CloseRoom calls gs.photo.closeRoom. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) CloseRoom(ctx context.Context, req PhotoCloseRoomRequest, opts ...RequestOption) (PhotoCloseRoomResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoCloseRoom, req, opts...)
}

// PhotoDelRoomUsrRequest is the request body for gs.photo.delRoomUsr.
type PhotoDelRoomUsrRequest struct {
	Type   RPCInt `json:"type,omitempty"`
	DstUID RPCUID `json:"dstUid,omitempty"`
}

// PhotoDelRoomUsrResponse is the namespace-delta response for gs.photo.delRoomUsr.
type PhotoDelRoomUsrResponse = RPCResponse[StateDelta]

// DelRoomUsr calls gs.photo.delRoomUsr. Request fields inferred from game.js: type, dstUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) DelRoomUsr(ctx context.Context, req PhotoDelRoomUsrRequest, opts ...RequestOption) (PhotoDelRoomUsrResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoDelRoomUsr, req, opts...)
}

// PhotoEnterRequest is the empty request body for gs.photo.enter.
type PhotoEnterRequest struct{}

// PhotoEnterResponse is the namespace-delta response for gs.photo.enter.
type PhotoEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.photo.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) Enter(ctx context.Context, req PhotoEnterRequest, opts ...RequestOption) (PhotoEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoEnter, req, opts...)
}

// PhotoEnterRoomRequest is the request body for gs.photo.enterRoom.
type PhotoEnterRoomRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// PhotoEnterRoomResponse is the namespace-delta response for gs.photo.enterRoom.
type PhotoEnterRoomResponse = RPCResponse[StateDelta]

// EnterRoom calls gs.photo.enterRoom. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) EnterRoom(ctx context.Context, req PhotoEnterRoomRequest, opts ...RequestOption) (PhotoEnterRoomResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoEnterRoom, req, opts...)
}

// PhotoFinishRoomRequest is the request body for gs.photo.finishRoom.
type PhotoFinishRoomRequest struct {
	Type RPCInt    `json:"type,omitempty"`
	Info RPCObject `json:"info,omitempty"`
}

// PhotoFinishRoomResponse is the namespace-delta response for gs.photo.finishRoom.
type PhotoFinishRoomResponse = RPCResponse[StateDelta]

// FinishRoom calls gs.photo.finishRoom. Request fields inferred from game.js: type, info.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) FinishRoom(ctx context.Context, req PhotoFinishRoomRequest, opts ...RequestOption) (PhotoFinishRoomResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoFinishRoom, req, opts...)
}

// PhotoGetBase64Request is the request body for gs.photo.getBase64.
type PhotoGetBase64Request struct {
	List RPCArray `json:"list,omitempty"`
}

// PhotoGetBase64Response is the namespace-delta response for gs.photo.getBase64.
type PhotoGetBase64Response = RPCResponse[StateDelta]

// GetBase64 calls gs.photo.getBase64. Request fields inferred from game.js: list.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) GetBase64(ctx context.Context, req PhotoGetBase64Request, opts ...RequestOption) (PhotoGetBase64Response, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoGetBase64, req, opts...)
}

// PhotoGetFriendListRequest is the request body for gs.photo.getFriendList.
type PhotoGetFriendListRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// PhotoGetFriendListResponse is the namespace-delta response for gs.photo.getFriendList.
type PhotoGetFriendListResponse = RPCResponse[StateDelta]

// GetFriendList calls gs.photo.getFriendList. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) GetFriendList(ctx context.Context, req PhotoGetFriendListRequest, opts ...RequestOption) (PhotoGetFriendListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoGetFriendList, req, opts...)
}

// PhotoGetInviteListRequest is the empty request body for gs.photo.getInviteList.
type PhotoGetInviteListRequest struct{}

// PhotoGetInviteListResponse is the namespace-delta response for gs.photo.getInviteList.
type PhotoGetInviteListResponse = RPCResponse[StateDelta]

// GetInviteList calls gs.photo.getInviteList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) GetInviteList(ctx context.Context, req PhotoGetInviteListRequest, opts ...RequestOption) (PhotoGetInviteListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoGetInviteList, req, opts...)
}

// PhotoGetPhotoListRequest is the empty request body for gs.photo.getPhotoList.
type PhotoGetPhotoListRequest struct{}

// PhotoGetPhotoListResponse is the namespace-delta response for gs.photo.getPhotoList.
type PhotoGetPhotoListResponse = RPCResponse[StateDelta]

// GetPhotoList calls gs.photo.getPhotoList. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) GetPhotoList(ctx context.Context, req PhotoGetPhotoListRequest, opts ...RequestOption) (PhotoGetPhotoListResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoGetPhotoList, req, opts...)
}

// PhotoInviteRequest is the request body for gs.photo.invite.
type PhotoInviteRequest struct {
	Type    RPCInt     `json:"type,omitempty"`
	DstUIDs RPCUIDList `json:"dstUids,omitempty"`
}

// PhotoInviteResponse is the namespace-delta response for gs.photo.invite.
type PhotoInviteResponse = RPCResponse[StateDelta]

// Invite calls gs.photo.invite. Request fields inferred from game.js: type, dstUids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) Invite(ctx context.Context, req PhotoInviteRequest, opts ...RequestOption) (PhotoInviteResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoInvite, req, opts...)
}

// PhotoInviteDealRequest is the request body for gs.photo.inviteDeal.
type PhotoInviteDealRequest struct {
	InviteId RPCID     `json:"inviteId,omitempty"`
	Info     RPCObject `json:"info,omitempty"`
}

// PhotoInviteDealResponse is the namespace-delta response for gs.photo.inviteDeal.
type PhotoInviteDealResponse = RPCResponse[StateDelta]

// InviteDeal calls gs.photo.inviteDeal. Request fields inferred from game.js: inviteId, info.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) InviteDeal(ctx context.Context, req PhotoInviteDealRequest, opts ...RequestOption) (PhotoInviteDealResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoInviteDeal, req, opts...)
}

// PhotoPushBase64Request is the request body for gs.photo.pushBase64.
type PhotoPushBase64Request struct {
	PlainText RPCString `json:"plainText,omitempty"`
	Md5       RPCString `json:"md5,omitempty"`
	Idx       RPCInt    `json:"idx,omitempty"`
	MaxIdx    RPCInt    `json:"maxIdx,omitempty"`
	UsePNG    RPCBool   `json:"usePNG,omitempty"`
}

// PhotoPushBase64Response is the namespace-delta response for gs.photo.pushBase64.
type PhotoPushBase64Response = RPCResponse[StateDelta]

// PushBase64 calls gs.photo.pushBase64. Request fields inferred from game.js: plainText, md5, idx, maxIdx, usePNG.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) PushBase64(ctx context.Context, req PhotoPushBase64Request, opts ...RequestOption) (PhotoPushBase64Response, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoPushBase64, req, opts...)
}

// PhotoReadInviteRequest is the request body for gs.photo.readInvite.
type PhotoReadInviteRequest struct {
	InviteIds RPCIDList `json:"inviteIds,omitempty"`
}

// PhotoReadInviteResponse is the namespace-delta response for gs.photo.readInvite.
type PhotoReadInviteResponse = RPCResponse[StateDelta]

// ReadInvite calls gs.photo.readInvite. Request fields inferred from game.js: inviteIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) ReadInvite(ctx context.Context, req PhotoReadInviteRequest, opts ...RequestOption) (PhotoReadInviteResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoReadInvite, req, opts...)
}

// PhotoReadPhotoRequest is the request body for gs.photo.readPhoto.
type PhotoReadPhotoRequest struct {
	Md5List RPCStringList `json:"md5List,omitempty"`
}

// PhotoReadPhotoResponse is the namespace-delta response for gs.photo.readPhoto.
type PhotoReadPhotoResponse = RPCResponse[StateDelta]

// ReadPhoto calls gs.photo.readPhoto. Request fields inferred from game.js: md5List.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) ReadPhoto(ctx context.Context, req PhotoReadPhotoRequest, opts ...RequestOption) (PhotoReadPhotoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoReadPhoto, req, opts...)
}

// PhotoReadRoomMsgRequest is the request body for gs.photo.readRoomMsg.
type PhotoReadRoomMsgRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// PhotoReadRoomMsgResponse is the namespace-delta response for gs.photo.readRoomMsg.
type PhotoReadRoomMsgResponse = RPCResponse[StateDelta]

// ReadRoomMsg calls gs.photo.readRoomMsg. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) ReadRoomMsg(ctx context.Context, req PhotoReadRoomMsgRequest, opts ...RequestOption) (PhotoReadRoomMsgResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoReadRoomMsg, req, opts...)
}

// PhotoReclaimRoomRequest is the request body for gs.photo.reclaimRoom.
type PhotoReclaimRoomRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// PhotoReclaimRoomResponse is the namespace-delta response for gs.photo.reclaimRoom.
type PhotoReclaimRoomResponse = RPCResponse[StateDelta]

// ReclaimRoom calls gs.photo.reclaimRoom. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) ReclaimRoom(ctx context.Context, req PhotoReclaimRoomRequest, opts ...RequestOption) (PhotoReclaimRoomResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoReclaimRoom, req, opts...)
}

// PhotoRejectInviteRequest is the request body for gs.photo.rejectInvite.
type PhotoRejectInviteRequest struct {
	DstUID RPCUID `json:"dstUid,omitempty"`
	RoomId RPCID  `json:"roomId,omitempty"`
}

// PhotoRejectInviteResponse is the namespace-delta response for gs.photo.rejectInvite.
type PhotoRejectInviteResponse = RPCResponse[StateDelta]

// RejectInvite calls gs.photo.rejectInvite. Request fields inferred from game.js: dstUid, roomId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) RejectInvite(ctx context.Context, req PhotoRejectInviteRequest, opts ...RequestOption) (PhotoRejectInviteResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoRejectInvite, req, opts...)
}

// PhotoReshootRequest is the request body for gs.photo.reshoot.
type PhotoReshootRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// PhotoReshootResponse is the namespace-delta response for gs.photo.reshoot.
type PhotoReshootResponse = RPCResponse[StateDelta]

// Reshoot calls gs.photo.reshoot. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) Reshoot(ctx context.Context, req PhotoReshootRequest, opts ...RequestOption) (PhotoReshootResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoReshoot, req, opts...)
}

// PhotoSavePhotoRequest is the request body for gs.photo.savePhoto.
type PhotoSavePhotoRequest struct {
	InviteId RPCID `json:"inviteId,omitempty"`
}

// PhotoSavePhotoResponse is the namespace-delta response for gs.photo.savePhoto.
type PhotoSavePhotoResponse = RPCResponse[StateDelta]

// SavePhoto calls gs.photo.savePhoto. Request fields inferred from game.js: inviteId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) SavePhoto(ctx context.Context, req PhotoSavePhotoRequest, opts ...RequestOption) (PhotoSavePhotoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoSavePhoto, req, opts...)
}

// PhotoSaveRoomPhotoRequest is the request body for gs.photo.saveRoomPhoto.
type PhotoSaveRoomPhotoRequest struct {
	Type   RPCInt  `json:"type,omitempty"`
	IsSave RPCBool `json:"isSave,omitempty"`
}

// PhotoSaveRoomPhotoResponse is the namespace-delta response for gs.photo.saveRoomPhoto.
type PhotoSaveRoomPhotoResponse = RPCResponse[StateDelta]

// SaveRoomPhoto calls gs.photo.saveRoomPhoto. Request fields inferred from game.js: type, isSave.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) SaveRoomPhoto(ctx context.Context, req PhotoSaveRoomPhotoRequest, opts ...RequestOption) (PhotoSaveRoomPhotoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoSaveRoomPhoto, req, opts...)
}

// PhotoSaveRoomProRequest is the request body for gs.photo.saveRoomPro.
type PhotoSaveRoomProRequest struct {
	Type     RPCInt   `json:"type,omitempty"`
	Progress RPCValue `json:"progress,omitempty"`
}

// PhotoSaveRoomProResponse is the namespace-delta response for gs.photo.saveRoomPro.
type PhotoSaveRoomProResponse = RPCResponse[StateDelta]

// SaveRoomPro calls gs.photo.saveRoomPro. Request fields inferred from game.js: type, progress.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) SaveRoomPro(ctx context.Context, req PhotoSaveRoomProRequest, opts ...RequestOption) (PhotoSaveRoomProResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoSaveRoomPro, req, opts...)
}

// PhotoSaveRoomUsrRequest is the request body for gs.photo.saveRoomUsr.
type PhotoSaveRoomUsrRequest struct {
	Type RPCInt    `json:"type,omitempty"`
	Info RPCObject `json:"info,omitempty"`
}

// PhotoSaveRoomUsrResponse is the namespace-delta response for gs.photo.saveRoomUsr.
type PhotoSaveRoomUsrResponse = RPCResponse[StateDelta]

// SaveRoomUsr calls gs.photo.saveRoomUsr. Request fields inferred from game.js: type, info.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) SaveRoomUsr(ctx context.Context, req PhotoSaveRoomUsrRequest, opts ...RequestOption) (PhotoSaveRoomUsrResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoSaveRoomUsr, req, opts...)
}

// PhotoSetPhotoStatusRequest is the request body for gs.photo.setPhotoStatus.
type PhotoSetPhotoStatusRequest struct {
	Md5List RPCStringList `json:"md5List,omitempty"`
	Status  RPCInt        `json:"status,omitempty"`
}

// PhotoSetPhotoStatusResponse is the namespace-delta response for gs.photo.setPhotoStatus.
type PhotoSetPhotoStatusResponse = RPCResponse[StateDelta]

// SetPhotoStatus calls gs.photo.setPhotoStatus. Request fields inferred from game.js: md5List, status.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) SetPhotoStatus(ctx context.Context, req PhotoSetPhotoStatusRequest, opts ...RequestOption) (PhotoSetPhotoStatusResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoSetPhotoStatus, req, opts...)
}

// PhotoSetRefuseInviteRequest is the request body for gs.photo.setRefuseInvite.
type PhotoSetRefuseInviteRequest struct {
	IsRefuse RPCBool `json:"isRefuse,omitempty"`
}

// PhotoSetRefuseInviteResponse is the namespace-delta response for gs.photo.setRefuseInvite.
type PhotoSetRefuseInviteResponse = RPCResponse[StateDelta]

// SetRefuseInvite calls gs.photo.setRefuseInvite. Request fields inferred from game.js: isRefuse.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) SetRefuseInvite(ctx context.Context, req PhotoSetRefuseInviteRequest, opts ...RequestOption) (PhotoSetRefuseInviteResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoSetRefuseInvite, req, opts...)
}

// PhotoTakePhotoRequest is the request body for gs.photo.takePhoto.
type PhotoTakePhotoRequest struct {
	Type RPCInt    `json:"type,omitempty"`
	Md5  RPCString `json:"md5,omitempty"`
}

// PhotoTakePhotoResponse is the namespace-delta response for gs.photo.takePhoto.
type PhotoTakePhotoResponse = RPCResponse[StateDelta]

// TakePhoto calls gs.photo.takePhoto. Request fields inferred from game.js: type, md5.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) TakePhoto(ctx context.Context, req PhotoTakePhotoRequest, opts ...RequestOption) (PhotoTakePhotoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoTakePhoto, req, opts...)
}

// PhotoTransmitRoomRequest is the request body for gs.photo.transmitRoom.
type PhotoTransmitRoomRequest struct {
	Type     RPCInt `json:"type,omitempty"`
	OperaUID RPCUID `json:"operaUid,omitempty"`
}

// PhotoTransmitRoomResponse is the namespace-delta response for gs.photo.transmitRoom.
type PhotoTransmitRoomResponse = RPCResponse[StateDelta]

// TransmitRoom calls gs.photo.transmitRoom. Request fields inferred from game.js: type, operaUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PhotoRPC) TransmitRoom(ctx context.Context, req PhotoTransmitRoomRequest, opts ...RequestOption) (PhotoTransmitRoomResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPhotoTransmitRoom, req, opts...)
}

// PlantRqst returns typed RPC helpers for the PlantRqst namespace.
func (c *RPCClient) PlantRqst() PlantRqstRPC { return PlantRqstRPC{c: c} }

type PlantRqstRPC struct{ c *RPCClient }

// PlantRqstZhtcRequest is the request body for gs.PlantRqst.zhtc.
type PlantRqstZhtcRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// PlantRqstZhtcResponse is the namespace-delta response for gs.PlantRqst.zhtc.
type PlantRqstZhtcResponse = RPCResponse[StateDelta]

// Zhtc calls gs.PlantRqst.zhtc. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlantRqstRPC) Zhtc(ctx context.Context, req PlantRqstZhtcRequest, opts ...RequestOption) (PlantRqstZhtcResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlantRqstZhtc, req, opts...)
}

// PlayerBack returns typed RPC helpers for the playerBack namespace.
func (c *RPCClient) PlayerBack() PlayerBackRPC { return PlayerBackRPC{c: c} }

type PlayerBackRPC struct{ c *RPCClient }

// PlayerBackPlayerBackPassEnterRequest carries JSON fields for gs.playerBack.playerBackPassEnter; game.js did not expose a stable object literal for this request.
type PlayerBackPlayerBackPassEnterRequest RawRequest

// PlayerBackPlayerBackPassEnterResponse is the namespace-delta response for gs.playerBack.playerBackPassEnter.
type PlayerBackPlayerBackPassEnterResponse = RPCResponse[StateDelta]

// PlayerBackPassEnter calls gs.playerBack.playerBackPassEnter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) PlayerBackPassEnter(ctx context.Context, req PlayerBackPlayerBackPassEnterRequest, opts ...RequestOption) (PlayerBackPlayerBackPassEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackPlayerBackPassEnter, req, opts...)
}

// PlayerBackPlayerBackPassRecvRequest carries JSON fields for gs.playerBack.playerBackPassRecv; game.js did not expose a stable object literal for this request.
type PlayerBackPlayerBackPassRecvRequest RawRequest

// PlayerBackPlayerBackPassRecvResponse is the namespace-delta response for gs.playerBack.playerBackPassRecv.
type PlayerBackPlayerBackPassRecvResponse = RPCResponse[StateDelta]

// PlayerBackPassRecv calls gs.playerBack.playerBackPassRecv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) PlayerBackPassRecv(ctx context.Context, req PlayerBackPlayerBackPassRecvRequest, opts ...RequestOption) (PlayerBackPlayerBackPassRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackPlayerBackPassRecv, req, opts...)
}

// PlayerBackPlayerBackPassRecvOneKeyRequest carries JSON fields for gs.playerBack.playerBackPassRecvOneKey; game.js did not expose a stable object literal for this request.
type PlayerBackPlayerBackPassRecvOneKeyRequest RawRequest

// PlayerBackPlayerBackPassRecvOneKeyResponse is the namespace-delta response for gs.playerBack.playerBackPassRecvOneKey.
type PlayerBackPlayerBackPassRecvOneKeyResponse = RPCResponse[StateDelta]

// PlayerBackPassRecvOneKey calls gs.playerBack.playerBackPassRecvOneKey. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) PlayerBackPassRecvOneKey(ctx context.Context, req PlayerBackPlayerBackPassRecvOneKeyRequest, opts ...RequestOption) (PlayerBackPlayerBackPassRecvOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackPlayerBackPassRecvOneKey, req, opts...)
}

// PlayerBackPlayerBackPassTaskDoneRequest carries JSON fields for gs.playerBack.playerBackPassTaskDone; game.js did not expose a stable object literal for this request.
type PlayerBackPlayerBackPassTaskDoneRequest RawRequest

// PlayerBackPlayerBackPassTaskDoneResponse is the namespace-delta response for gs.playerBack.playerBackPassTaskDone.
type PlayerBackPlayerBackPassTaskDoneResponse = RPCResponse[StateDelta]

// PlayerBackPassTaskDone calls gs.playerBack.playerBackPassTaskDone. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) PlayerBackPassTaskDone(ctx context.Context, req PlayerBackPlayerBackPassTaskDoneRequest, opts ...RequestOption) (PlayerBackPlayerBackPassTaskDoneResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackPlayerBackPassTaskDone, req, opts...)
}

// PlayerBackSignRequest carries JSON fields for gs.playerBack.sign; game.js did not expose a stable object literal for this request.
type PlayerBackSignRequest RawRequest

// PlayerBackSignResponse is the namespace-delta response for gs.playerBack.sign.
type PlayerBackSignResponse = RPCResponse[StateDelta]

// Sign calls gs.playerBack.sign. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) Sign(ctx context.Context, req PlayerBackSignRequest, opts ...RequestOption) (PlayerBackSignResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackSign, req, opts...)
}

// PlayerBackSignEnterRequest carries JSON fields for gs.playerBack.signEnter; game.js did not expose a stable object literal for this request.
type PlayerBackSignEnterRequest RawRequest

// PlayerBackSignEnterResponse is the namespace-delta response for gs.playerBack.signEnter.
type PlayerBackSignEnterResponse = RPCResponse[StateDelta]

// SignEnter calls gs.playerBack.signEnter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) SignEnter(ctx context.Context, req PlayerBackSignEnterRequest, opts ...RequestOption) (PlayerBackSignEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackSignEnter, req, opts...)
}

// PlayerBackSignRecvRequest carries JSON fields for gs.playerBack.signRecv; game.js did not expose a stable object literal for this request.
type PlayerBackSignRecvRequest RawRequest

// PlayerBackSignRecvResponse is the namespace-delta response for gs.playerBack.signRecv.
type PlayerBackSignRecvResponse = RPCResponse[StateDelta]

// SignRecv calls gs.playerBack.signRecv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) SignRecv(ctx context.Context, req PlayerBackSignRecvRequest, opts ...RequestOption) (PlayerBackSignRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackSignRecv, req, opts...)
}

// PlayerBackUpdateGuildIdsRequest carries JSON fields for gs.playerBack.updateGuildIds; game.js did not expose a stable object literal for this request.
type PlayerBackUpdateGuildIdsRequest RawRequest

// PlayerBackUpdateGuildIdsResponse is the namespace-delta response for gs.playerBack.updateGuildIds.
type PlayerBackUpdateGuildIdsResponse = RPCResponse[StateDelta]

// UpdateGuildIds calls gs.playerBack.updateGuildIds. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r PlayerBackRPC) UpdateGuildIds(ctx context.Context, req PlayerBackUpdateGuildIdsRequest, opts ...RequestOption) (PlayerBackUpdateGuildIdsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCPlayerBackUpdateGuildIds, req, opts...)
}

// RandomEvent returns typed RPC helpers for the randomEvent namespace.
func (c *RPCClient) RandomEvent() RandomEventRPC { return RandomEventRPC{c: c} }

type RandomEventRPC struct{ c *RPCClient }

// RandomEventDoAffairRequest is the request body for gs.randomEvent.doAffair.
type RandomEventDoAffairRequest struct {
	EventId RPCID `json:"eventId,omitempty"`
}

// RandomEventDoAffairResponse is the namespace-delta response for gs.randomEvent.doAffair.
type RandomEventDoAffairResponse = RPCResponse[StateDelta]

// DoAffair calls gs.randomEvent.doAffair. Request fields inferred from game.js: eventId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RandomEventRPC) DoAffair(ctx context.Context, req RandomEventDoAffairRequest, opts ...RequestOption) (RandomEventDoAffairResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRandomEventDoAffair, req, opts...)
}

// RandomEventEnterRequest is the empty request body for gs.randomEvent.enter.
type RandomEventEnterRequest struct{}

// RandomEventEnterResponse is the namespace-delta response for gs.randomEvent.enter.
type RandomEventEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.randomEvent.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RandomEventRPC) Enter(ctx context.Context, req RandomEventEnterRequest, opts ...RequestOption) (RandomEventEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRandomEventEnter, req, opts...)
}

// Rank returns typed RPC helpers for the rank namespace.
func (c *RPCClient) Rank() RankRPC { return RankRPC{c: c} }

type RankRPC struct{ c *RPCClient }

// RankGetRanksRequest is the request body for gs.rank.getRanks.
type RankGetRanksRequest struct {
	List  RPCArray `json:"list,omitempty"`
	Masks RPCArray `json:"masks,omitempty"`
}

// RankGetRanksResponse is the namespace-delta response for gs.rank.getRanks.
type RankGetRanksResponse = RPCResponse[StateDelta]

// GetRanks calls gs.rank.getRanks. Request fields inferred from game.js: list, masks.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RankRPC) GetRanks(ctx context.Context, req RankGetRanksRequest, opts ...RequestOption) (RankGetRanksResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRankGetRanks, req, opts...)
}

// RchgCard returns typed RPC helpers for the rchgCard namespace.
func (c *RPCClient) RchgCard() RchgCardRPC { return RchgCardRPC{c: c} }

type RchgCardRPC struct{ c *RPCClient }

// RchgCardRecvRequest is the request body for gs.rchgCard.recv.
type RchgCardRecvRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// RchgCardRecvResponse is the namespace-delta response for gs.rchgCard.recv.
type RchgCardRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.rchgCard.recv. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RchgCardRPC) Recv(ctx context.Context, req RchgCardRecvRequest, opts ...RequestOption) (RchgCardRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRchgCardRecv, req, opts...)
}

// RchgDay returns typed RPC helpers for the rchgDay namespace.
func (c *RPCClient) RchgDay() RchgDayRPC { return RchgDayRPC{c: c} }

type RchgDayRPC struct{ c *RPCClient }

// RchgDayEnterRequest is the empty request body for gs.rchgDay.enter.
type RchgDayEnterRequest struct{}

// RchgDayEnterResponse is the namespace-delta response for gs.rchgDay.enter.
type RchgDayEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.rchgDay.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RchgDayRPC) Enter(ctx context.Context, req RchgDayEnterRequest, opts ...RequestOption) (RchgDayEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRchgDayEnter, req, opts...)
}

// RchgDayReceiveRequest is the request body for gs.rchgDay.receive.
type RchgDayReceiveRequest struct {
	Index RPCInt `json:"index,omitempty"`
}

// RchgDayReceiveResponse is the namespace-delta response for gs.rchgDay.receive.
type RchgDayReceiveResponse = RPCResponse[StateDelta]

// Receive calls gs.rchgDay.receive. Request fields inferred from game.js: index.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RchgDayRPC) Receive(ctx context.Context, req RchgDayReceiveRequest, opts ...RequestOption) (RchgDayReceiveResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRchgDayReceive, req, opts...)
}

// RchgOrderToMoney returns typed RPC helpers for the rchgOrderToMoney namespace.
func (c *RPCClient) RchgOrderToMoney() RchgOrderToMoneyRPC { return RchgOrderToMoneyRPC{c: c} }

type RchgOrderToMoneyRPC struct{ c *RPCClient }

// RchgOrderToMoneyConvertMoneyRequest is the request body for gs.rchgOrderToMoney.convertMoney.
type RchgOrderToMoneyConvertMoneyRequest struct {
	OrderNo RPCString `json:"orderNo,omitempty"`
}

// RchgOrderToMoneyConvertMoneyResponse is the namespace-delta response for gs.rchgOrderToMoney.convertMoney.
type RchgOrderToMoneyConvertMoneyResponse = RPCResponse[StateDelta]

// ConvertMoney calls gs.rchgOrderToMoney.convertMoney. Request fields inferred from game.js: orderNo.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RchgOrderToMoneyRPC) ConvertMoney(ctx context.Context, req RchgOrderToMoneyConvertMoneyRequest, opts ...RequestOption) (RchgOrderToMoneyConvertMoneyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRchgOrderToMoneyConvertMoney, req, opts...)
}

// RchgSum returns typed RPC helpers for the rchgSum namespace.
func (c *RPCClient) RchgSum() RchgSumRPC { return RchgSumRPC{c: c} }

type RchgSumRPC struct{ c *RPCClient }

// RchgSumRecvRequest is the request body for gs.rchgSum.recv.
type RchgSumRecvRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// RchgSumRecvResponse is the namespace-delta response for gs.rchgSum.recv.
type RchgSumRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.rchgSum.recv. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RchgSumRPC) Recv(ctx context.Context, req RchgSumRecvRequest, opts ...RequestOption) (RchgSumRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRchgSumRecv, req, opts...)
}

// ReapPopup returns typed RPC helpers for the ReapPopup namespace.
func (c *RPCClient) ReapPopup() ReapPopupRPC { return ReapPopupRPC{c: c} }

type ReapPopupRPC struct{ c *RPCClient }

// ReapPopupShjmRequest is the request body for gs.ReapPopup.shjm.
type ReapPopupShjmRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// ReapPopupShjmResponse is the namespace-delta response for gs.ReapPopup.shjm.
type ReapPopupShjmResponse = RPCResponse[StateDelta]

// Shjm calls gs.ReapPopup.shjm. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ReapPopupRPC) Shjm(ctx context.Context, req ReapPopupShjmRequest, opts ...RequestOption) (ReapPopupShjmResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCReapPopupShjm, req, opts...)
}

// Redeem returns typed RPC helpers for the redeem namespace.
func (c *RPCClient) Redeem() RedeemRPC { return RedeemRPC{c: c} }

type RedeemRPC struct{ c *RPCClient }

// RedeemGetInfoRequest is the empty request body for gs.redeem.getInfo.
type RedeemGetInfoRequest struct{}

// RedeemGetInfoResponse is the namespace-delta response for gs.redeem.getInfo.
type RedeemGetInfoResponse = RPCResponse[StateDelta]

// GetInfo calls gs.redeem.getInfo. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RedeemRPC) GetInfo(ctx context.Context, req RedeemGetInfoRequest, opts ...RequestOption) (RedeemGetInfoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRedeemGetInfo, req, opts...)
}

// RedeemUseCodeRequest carries JSON fields for gs.redeem.useCode; game.js did not expose a stable object literal for this request.
type RedeemUseCodeRequest RawRequest

// RedeemUseCodeResponse is the namespace-delta response for gs.redeem.useCode.
type RedeemUseCodeResponse = RPCResponse[StateDelta]

// UseCode calls gs.redeem.useCode. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RedeemRPC) UseCode(ctx context.Context, req RedeemUseCodeRequest, opts ...RequestOption) (RedeemUseCodeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRedeemUseCode, req, opts...)
}

// RedeemCodeShow returns typed RPC helpers for the redeemCodeShow namespace.
func (c *RPCClient) RedeemCodeShow() RedeemCodeShowRPC { return RedeemCodeShowRPC{c: c} }

type RedeemCodeShowRPC struct{ c *RPCClient }

// RedeemCodeShowDjdkRequest is the request body for gs.redeemCodeShow.djdk.
type RedeemCodeShowDjdkRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// RedeemCodeShowDjdkResponse is the namespace-delta response for gs.redeemCodeShow.djdk.
type RedeemCodeShowDjdkResponse = RPCResponse[StateDelta]

// Djdk calls gs.redeemCodeShow.djdk. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RedeemCodeShowRPC) Djdk(ctx context.Context, req RedeemCodeShowDjdkRequest, opts ...RequestOption) (RedeemCodeShowDjdkResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRedeemCodeShowDjdk, req, opts...)
}

// Reputation returns typed RPC helpers for the reputation namespace.
func (c *RPCClient) Reputation() ReputationRPC { return ReputationRPC{c: c} }

type ReputationRPC struct{ c *RPCClient }

// ReputationAppealRequest is the request body for gs.reputation.appeal.
type ReputationAppealRequest struct {
	Reason RPCString `json:"reason,omitempty"`
	MsIds  RPCIDList `json:"msIds,omitempty"`
}

// ReputationAppealResponse is the namespace-delta response for gs.reputation.appeal.
type ReputationAppealResponse = RPCResponse[StateDelta]

// Appeal calls gs.reputation.appeal. Request fields inferred from game.js: reason, msIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ReputationRPC) Appeal(ctx context.Context, req ReputationAppealRequest, opts ...RequestOption) (ReputationAppealResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCReputationAppeal, req, opts...)
}

// ReputationGetLogsRequest is the empty request body for gs.reputation.getLogs.
type ReputationGetLogsRequest struct{}

// ReputationGetLogsResponse is the namespace-delta response for gs.reputation.getLogs.
type ReputationGetLogsResponse = RPCResponse[StateDelta]

// GetLogs calls gs.reputation.getLogs. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ReputationRPC) GetLogs(ctx context.Context, req ReputationGetLogsRequest, opts ...RequestOption) (ReputationGetLogsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCReputationGetLogs, req, opts...)
}

// ReputationViewRequest is the empty request body for gs.reputation.view.
type ReputationViewRequest struct{}

// ReputationViewResponse is the namespace-delta response for gs.reputation.view.
type ReputationViewResponse = RPCResponse[StateDelta]

// View calls gs.reputation.view. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ReputationRPC) View(ctx context.Context, req ReputationViewRequest, opts ...RequestOption) (ReputationViewResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCReputationView, req, opts...)
}

// Reserve returns typed RPC helpers for the reserve namespace.
func (c *RPCClient) Reserve() ReserveRPC { return ReserveRPC{c: c} }

type ReserveRPC struct{ c *RPCClient }

// ReserveCheckRwdRequest is the empty request body for gs.reserve.checkRwd.
type ReserveCheckRwdRequest struct{}

// ReserveCheckRwdResponse is the namespace-delta response for gs.reserve.checkRwd.
type ReserveCheckRwdResponse = RPCResponse[StateDelta]

// CheckRwd calls gs.reserve.checkRwd. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ReserveRPC) CheckRwd(ctx context.Context, req ReserveCheckRwdRequest, opts ...RequestOption) (ReserveCheckRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCReserveCheckRwd, req, opts...)
}

// RoadGrow returns typed RPC helpers for the roadGrow namespace.
func (c *RPCClient) RoadGrow() RoadGrowRPC { return RoadGrowRPC{c: c} }

type RoadGrowRPC struct{ c *RPCClient }

// RoadGrowRecvRequest is the request body for gs.roadGrow.recv.
type RoadGrowRecvRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// RoadGrowRecvResponse is the namespace-delta response for gs.roadGrow.recv.
type RoadGrowRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.roadGrow.recv. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RoadGrowRPC) Recv(ctx context.Context, req RoadGrowRecvRequest, opts ...RequestOption) (RoadGrowRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRoadGrowRecv, req, opts...)
}

// RoadGrowRecvBoxRequest is the request body for gs.roadGrow.recvBox.
type RoadGrowRecvBoxRequest struct {
	Idx RPCInt `json:"idx,omitempty"`
}

// RoadGrowRecvBoxResponse is the namespace-delta response for gs.roadGrow.recvBox.
type RoadGrowRecvBoxResponse = RPCResponse[StateDelta]

// RecvBox calls gs.roadGrow.recvBox. Request fields inferred from game.js: idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RoadGrowRPC) RecvBox(ctx context.Context, req RoadGrowRecvBoxRequest, opts ...RequestOption) (RoadGrowRecvBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRoadGrowRecvBox, req, opts...)
}

// Rwd returns typed RPC helpers for the rwd namespace.
func (c *RPCClient) Rwd() RwdRPC { return RwdRPC{c: c} }

type RwdRPC struct{ c *RPCClient }

// RwdRecvRequest is the request body for gs.rwd.recv.
type RwdRecvRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// RwdRecvResponse is the namespace-delta response for gs.rwd.recv.
type RwdRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.rwd.recv. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RwdRPC) Recv(ctx context.Context, req RwdRecvRequest, opts ...RequestOption) (RwdRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRwdRecv, req, opts...)
}

// RwdSetCanRecvRequest is the request body for gs.rwd.setCanRecv.
type RwdSetCanRecvRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// RwdSetCanRecvResponse is the namespace-delta response for gs.rwd.setCanRecv.
type RwdSetCanRecvResponse = RPCResponse[StateDelta]

// SetCanRecv calls gs.rwd.setCanRecv. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r RwdRPC) SetCanRecv(ctx context.Context, req RwdSetCanRecvRequest, opts ...RequestOption) (RwdSetCanRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCRwdSetCanRecv, req, opts...)
}

// Sdk returns typed RPC helpers for the sdk namespace.
func (c *RPCClient) Sdk() SdkRPC { return SdkRPC{c: c} }

type SdkRPC struct{ c *RPCClient }

// SdkCheckRchgRequest is the request body for gs.sdk.checkRchg.
type SdkCheckRchgRequest struct {
	RchgId               RPCID     `json:"rchgId,omitempty"`
	Type                 RPCInt    `json:"type,omitempty"`
	Value                RPCValue  `json:"value,omitempty"`
	Ext                  RPCObject `json:"ext,omitempty"`
	UseMoney             RPCInt    `json:"useMoney,omitempty"`
	UseMoneyCount        RPCInt    `json:"useMoneyCount,omitempty"`
	RequestFriendPayment RPCBool   `json:"requestFriendPayment,omitempty"`
}

// SdkCheckRchgResponse is the namespace-delta response for gs.sdk.checkRchg.
type SdkCheckRchgResponse = RPCResponse[StateDelta]

// CheckRchg calls gs.sdk.checkRchg. Request fields inferred from game.js: rchgId, type, value, ext, useMoney, useMoneyCount, requestFriendPayment.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SdkRPC) CheckRchg(ctx context.Context, req SdkCheckRchgRequest, opts ...RequestOption) (SdkCheckRchgResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSdkCheckRchg, req, opts...)
}

// SdkMoniPayRequest carries JSON fields for gs.sdk.moniPay; game.js did not expose a stable object literal for this request.
type SdkMoniPayRequest RawRequest

// SdkMoniPayResponse is the namespace-delta response for gs.sdk.moniPay.
type SdkMoniPayResponse = RPCResponse[StateDelta]

// MoniPay calls gs.sdk.moniPay. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SdkRPC) MoniPay(ctx context.Context, req SdkMoniPayRequest, opts ...RequestOption) (SdkMoniPayResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSdkMoniPay, req, opts...)
}

// SdkPayByMoneyRequest is the request body for gs.sdk.payByMoney.
type SdkPayByMoneyRequest struct {
	ID                RPCID     `json:"id,omitempty"`
	Orderno           RPCString `json:"orderno,omitempty"`
	Sign              RPCString `json:"sign,omitempty"`
	RechargeType      RPCInt    `json:"rechargeType,omitempty"`
	RechargeTypeValue RPCInt    `json:"rechargeTypeValue,omitempty"`
	SubjectName       RPCString `json:"subjectName,omitempty"`
	DisplayName       RPCString `json:"displayName,omitempty"`
	Ext               RPCObject `json:"ext,omitempty"`
	Serverid          RPCString `json:"serverid,omitempty"`
	Serverindex       RPCString `json:"serverindex,omitempty"`
	Servername        RPCString `json:"servername,omitempty"`
	Rolename          RPCString `json:"rolename,omitempty"`
	Roleid            RPCString `json:"roleid,omitempty"`
	Accountid         RPCString `json:"accountid,omitempty"`
	Roledid           RPCString `json:"roledid,omitempty"`
	MaybeFirst        RPCBool   `json:"maybeFirst,omitempty"`
	BiOpt             RPCValue  `json:"biOpt,omitempty"`
}

// SdkPayByMoneyResponse is the namespace-delta response for gs.sdk.payByMoney.
type SdkPayByMoneyResponse = RPCResponse[StateDelta]

// PayByMoney calls gs.sdk.payByMoney. Request fields inferred from game.js: id, orderno, sign, rechargeType, rechargeTypeValue, subjectName, displayName, ext, serverid, serverindex, servername, rolename, roleid, accountid, roledid, maybeFirst, biOpt.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SdkRPC) PayByMoney(ctx context.Context, req SdkPayByMoneyRequest, opts ...RequestOption) (SdkPayByMoneyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSdkPayByMoney, req, opts...)
}

// SdkSendGoodsRequest is the empty request body for gs.sdk.sendGoods.
type SdkSendGoodsRequest struct{}

// SdkSendGoodsResponse is the namespace-delta response for gs.sdk.sendGoods.
type SdkSendGoodsResponse = RPCResponse[StateDelta]

// SendGoods calls gs.sdk.sendGoods. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SdkRPC) SendGoods(ctx context.Context, req SdkSendGoodsRequest, opts ...RequestOption) (SdkSendGoodsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSdkSendGoods, req, opts...)
}

// SecPwd returns typed RPC helpers for the secPwd namespace.
func (c *RPCClient) SecPwd() SecPwdRPC { return SecPwdRPC{c: c} }

type SecPwdRPC struct{ c *RPCClient }

// SecPwdChangePwdRequest is the request body for gs.secPwd.changePwd.
type SecPwdChangePwdRequest struct {
	OldPwd RPCString `json:"oldPwd,omitempty"`
	NewPwd RPCString `json:"newPwd,omitempty"`
}

// SecPwdChangePwdResponse is the namespace-delta response for gs.secPwd.changePwd.
type SecPwdChangePwdResponse = RPCResponse[StateDelta]

// ChangePwd calls gs.secPwd.changePwd. Request fields inferred from game.js: oldPwd, newPwd.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) ChangePwd(ctx context.Context, req SecPwdChangePwdRequest, opts ...RequestOption) (SecPwdChangePwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdChangePwd, req, opts...)
}

// SecPwdCheckPwdRequest is the request body for gs.secPwd.checkPwd.
type SecPwdCheckPwdRequest struct {
	Pwd RPCString `json:"pwd,omitempty"`
}

// SecPwdCheckPwdResponse is the namespace-delta response for gs.secPwd.checkPwd.
type SecPwdCheckPwdResponse = RPCResponse[StateDelta]

// CheckPwd calls gs.secPwd.checkPwd. Request fields inferred from game.js: pwd.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) CheckPwd(ctx context.Context, req SecPwdCheckPwdRequest, opts ...RequestOption) (SecPwdCheckPwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdCheckPwd, req, opts...)
}

// SecPwdCloseSecPwdRequest is the request body for gs.secPwd.closeSecPwd.
type SecPwdCloseSecPwdRequest struct {
	Pwd RPCString `json:"pwd,omitempty"`
}

// SecPwdCloseSecPwdResponse is the namespace-delta response for gs.secPwd.closeSecPwd.
type SecPwdCloseSecPwdResponse = RPCResponse[StateDelta]

// CloseSecPwd calls gs.secPwd.closeSecPwd. Request fields inferred from game.js: pwd.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) CloseSecPwd(ctx context.Context, req SecPwdCloseSecPwdRequest, opts ...RequestOption) (SecPwdCloseSecPwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdCloseSecPwd, req, opts...)
}

// SecPwdFirstUseRequest is the empty request body for gs.secPwd.firstUse.
type SecPwdFirstUseRequest struct{}

// SecPwdFirstUseResponse is the namespace-delta response for gs.secPwd.firstUse.
type SecPwdFirstUseResponse = RPCResponse[StateDelta]

// FirstUse calls gs.secPwd.firstUse. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) FirstUse(ctx context.Context, req SecPwdFirstUseRequest, opts ...RequestOption) (SecPwdFirstUseResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdFirstUse, req, opts...)
}

// SecPwdGetQuestionRequest is the empty request body for gs.secPwd.getQuestion.
type SecPwdGetQuestionRequest struct{}

// SecPwdGetQuestionResponse is the namespace-delta response for gs.secPwd.getQuestion.
type SecPwdGetQuestionResponse = RPCResponse[StateDelta]

// GetQuestion calls gs.secPwd.getQuestion. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) GetQuestion(ctx context.Context, req SecPwdGetQuestionRequest, opts ...RequestOption) (SecPwdGetQuestionResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdGetQuestion, req, opts...)
}

// SecPwdResetPwdRequest is the request body for gs.secPwd.resetPwd.
type SecPwdResetPwdRequest struct {
	NewPwd    RPCString `json:"newPwd,omitempty"`
	SelectIdx RPCInt    `json:"selectIdx,omitempty"`
	Answer    RPCString `json:"answer,omitempty"`
}

// SecPwdResetPwdResponse is the namespace-delta response for gs.secPwd.resetPwd.
type SecPwdResetPwdResponse = RPCResponse[StateDelta]

// ResetPwd calls gs.secPwd.resetPwd. Request fields inferred from game.js: newPwd, selectIdx, answer.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) ResetPwd(ctx context.Context, req SecPwdResetPwdRequest, opts ...RequestOption) (SecPwdResetPwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdResetPwd, req, opts...)
}

// SecPwdSetPwdRequest is the request body for gs.secPwd.setPwd.
type SecPwdSetPwdRequest struct {
	Pwd       RPCString `json:"pwd,omitempty"`
	Question  RPCString `json:"question,omitempty"`
	Answer    RPCString `json:"answer,omitempty"`
	Question2 RPCString `json:"question2,omitempty"`
	Answer2   RPCString `json:"answer2,omitempty"`
}

// SecPwdSetPwdResponse is the namespace-delta response for gs.secPwd.setPwd.
type SecPwdSetPwdResponse = RPCResponse[StateDelta]

// SetPwd calls gs.secPwd.setPwd. Request fields inferred from game.js: pwd, question, answer, question2, answer2.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SecPwdRPC) SetPwd(ctx context.Context, req SecPwdSetPwdRequest, opts ...RequestOption) (SecPwdSetPwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSecPwdSetPwd, req, opts...)
}

// Shop returns typed RPC helpers for the shop namespace.
func (c *RPCClient) Shop() ShopRPC { return ShopRPC{c: c} }

type ShopRPC struct{ c *RPCClient }

// ShopBuyRequest is the request body for gs.shop.buy.
type ShopBuyRequest struct {
	TempId RPCID  `json:"tempId,omitempty"`
	ItemId RPCID  `json:"itemId,omitempty"`
	Count  RPCInt `json:"count,omitempty"`
}

// ShopBuyResponse is the namespace-delta response for gs.shop.buy.
type ShopBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.shop.buy. Request fields inferred from game.js: tempId, itemId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopRPC) Buy(ctx context.Context, req ShopBuyRequest, opts ...RequestOption) (ShopBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopBuy, req, opts...)
}

// ShopEnterRequest is the request body for gs.shop.enter.
type ShopEnterRequest struct {
	TempId RPCID `json:"tempId,omitempty"`
}

// ShopEnterResponse is the namespace-delta response for gs.shop.enter.
type ShopEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.shop.enter. Request fields inferred from game.js: tempId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopRPC) Enter(ctx context.Context, req ShopEnterRequest, opts ...RequestOption) (ShopEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopEnter, req, opts...)
}

// ShopRefreshRequest is the request body for gs.shop.refresh.
type ShopRefreshRequest struct {
	TempId RPCID  `json:"tempId,omitempty"`
	Type   RPCInt `json:"type,omitempty"`
}

// ShopRefreshResponse is the namespace-delta response for gs.shop.refresh.
type ShopRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.shop.refresh. Request fields inferred from game.js: tempId, type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopRPC) Refresh(ctx context.Context, req ShopRefreshRequest, opts ...RequestOption) (ShopRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopRefresh, req, opts...)
}

// ShopSyncRequest is the request body for gs.shop.sync.
type ShopSyncRequest struct {
	TempIds RPCIDList `json:"tempIds,omitempty"`
}

// ShopSyncResponse is the namespace-delta response for gs.shop.sync.
type ShopSyncResponse = RPCResponse[StateDelta]

// Sync calls gs.shop.sync. Request fields inferred from game.js: tempIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopRPC) Sync(ctx context.Context, req ShopSyncRequest, opts ...RequestOption) (ShopSyncResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopSync, req, opts...)
}

// ShopCultivate returns typed RPC helpers for the shopCultivate namespace.
func (c *RPCClient) ShopCultivate() ShopCultivateRPC { return ShopCultivateRPC{c: c} }

type ShopCultivateRPC struct{ c *RPCClient }

// ShopCultivateBuyRequest is the request body for gs.shopCultivate.buy.
type ShopCultivateBuyRequest struct {
	ShopId RPCID `json:"shopId,omitempty"`
}

// ShopCultivateBuyResponse is the namespace-delta response for gs.shopCultivate.buy.
type ShopCultivateBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.shopCultivate.buy. Request fields inferred from game.js: shopId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopCultivateRPC) Buy(ctx context.Context, req ShopCultivateBuyRequest, opts ...RequestOption) (ShopCultivateBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopCultivateBuy, req, opts...)
}

// ShopCultivateBuyOneKeyRequest is the empty request body for gs.shopCultivate.buyOneKey.
type ShopCultivateBuyOneKeyRequest struct{}

// ShopCultivateBuyOneKeyResponse is the namespace-delta response for gs.shopCultivate.buyOneKey.
type ShopCultivateBuyOneKeyResponse = RPCResponse[StateDelta]

// BuyOneKey calls gs.shopCultivate.buyOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopCultivateRPC) BuyOneKey(ctx context.Context, req ShopCultivateBuyOneKeyRequest, opts ...RequestOption) (ShopCultivateBuyOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopCultivateBuyOneKey, req, opts...)
}

// ShopCultivateEnterRequest is the empty request body for gs.shopCultivate.enter.
type ShopCultivateEnterRequest struct{}

// ShopCultivateEnterResponse is the namespace-delta response for gs.shopCultivate.enter.
type ShopCultivateEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.shopCultivate.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopCultivateRPC) Enter(ctx context.Context, req ShopCultivateEnterRequest, opts ...RequestOption) (ShopCultivateEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopCultivateEnter, req, opts...)
}

// ShopCultivateRefreshRequest is the empty request body for gs.shopCultivate.refresh.
type ShopCultivateRefreshRequest struct{}

// ShopCultivateRefreshResponse is the namespace-delta response for gs.shopCultivate.refresh.
type ShopCultivateRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.shopCultivate.refresh. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopCultivateRPC) Refresh(ctx context.Context, req ShopCultivateRefreshRequest, opts ...RequestOption) (ShopCultivateRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopCultivateRefresh, req, opts...)
}

// ShopFlowerElves returns typed RPC helpers for the shopFlowerElves namespace.
func (c *RPCClient) ShopFlowerElves() ShopFlowerElvesRPC { return ShopFlowerElvesRPC{c: c} }

type ShopFlowerElvesRPC struct{ c *RPCClient }

// ShopFlowerElvesBuyRequest is the request body for gs.shopFlowerElves.buy.
type ShopFlowerElvesBuyRequest struct {
	ShopId RPCID  `json:"shopId,omitempty"`
	Num    RPCInt `json:"num,omitempty"`
}

// ShopFlowerElvesBuyResponse is the namespace-delta response for gs.shopFlowerElves.buy.
type ShopFlowerElvesBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.shopFlowerElves.buy. Request fields inferred from game.js: shopId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFlowerElvesRPC) Buy(ctx context.Context, req ShopFlowerElvesBuyRequest, opts ...RequestOption) (ShopFlowerElvesBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFlowerElvesBuy, req, opts...)
}

// ShopFlowerElvesEnterRequest is the empty request body for gs.shopFlowerElves.enter.
type ShopFlowerElvesEnterRequest struct{}

// ShopFlowerElvesEnterResponse is the namespace-delta response for gs.shopFlowerElves.enter.
type ShopFlowerElvesEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.shopFlowerElves.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFlowerElvesRPC) Enter(ctx context.Context, req ShopFlowerElvesEnterRequest, opts ...RequestOption) (ShopFlowerElvesEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFlowerElvesEnter, req, opts...)
}

// ShopFmlRace returns typed RPC helpers for the shopFmlRace namespace.
func (c *RPCClient) ShopFmlRace() ShopFmlRaceRPC { return ShopFmlRaceRPC{c: c} }

type ShopFmlRaceRPC struct{ c *RPCClient }

// ShopFmlRaceBuyRequest is the request body for gs.shopFmlRace.buy.
type ShopFmlRaceBuyRequest struct {
	IsAll RPCBool `json:"isAll,omitempty"`
}

// ShopFmlRaceBuyResponse is the namespace-delta response for gs.shopFmlRace.buy.
type ShopFmlRaceBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.shopFmlRace.buy. Request fields inferred from game.js: isAll.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlRaceRPC) Buy(ctx context.Context, req ShopFmlRaceBuyRequest, opts ...RequestOption) (ShopFmlRaceBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlRaceBuy, req, opts...)
}

// ShopFmlUsr returns typed RPC helpers for the shopFmlUsr namespace.
func (c *RPCClient) ShopFmlUsr() ShopFmlUsrRPC { return ShopFmlUsrRPC{c: c} }

type ShopFmlUsrRPC struct{ c *RPCClient }

// ShopFmlUsrBuildShopRequest is the request body for gs.shopFmlUsr.buildShop.
type ShopFmlUsrBuildShopRequest struct {
	SkillId RPCID `json:"skillId,omitempty"`
}

// ShopFmlUsrBuildShopResponse is the namespace-delta response for gs.shopFmlUsr.buildShop.
type ShopFmlUsrBuildShopResponse = RPCResponse[StateDelta]

// BuildShop calls gs.shopFmlUsr.buildShop. Request fields inferred from game.js: skillId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlUsrRPC) BuildShop(ctx context.Context, req ShopFmlUsrBuildShopRequest, opts ...RequestOption) (ShopFmlUsrBuildShopResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlUsrBuildShop, req, opts...)
}

// ShopFmlUsrBuyRequest is the request body for gs.shopFmlUsr.buy.
type ShopFmlUsrBuyRequest struct {
	SlotId RPCID  `json:"slotId,omitempty"`
	Count  RPCInt `json:"count,omitempty"`
}

// ShopFmlUsrBuyResponse is the namespace-delta response for gs.shopFmlUsr.buy.
type ShopFmlUsrBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.shopFmlUsr.buy. Request fields inferred from game.js: slotId, count.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlUsrRPC) Buy(ctx context.Context, req ShopFmlUsrBuyRequest, opts ...RequestOption) (ShopFmlUsrBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlUsrBuy, req, opts...)
}

// ShopFmlUsrBuyAllRequest is the empty request body for gs.shopFmlUsr.buyAll.
type ShopFmlUsrBuyAllRequest struct{}

// ShopFmlUsrBuyAllResponse is the namespace-delta response for gs.shopFmlUsr.buyAll.
type ShopFmlUsrBuyAllResponse = RPCResponse[StateDelta]

// BuyAll calls gs.shopFmlUsr.buyAll. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlUsrRPC) BuyAll(ctx context.Context, req ShopFmlUsrBuyAllRequest, opts ...RequestOption) (ShopFmlUsrBuyAllResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlUsrBuyAll, req, opts...)
}

// ShopFmlUsrEnterRequest is the empty request body for gs.shopFmlUsr.enter.
type ShopFmlUsrEnterRequest struct{}

// ShopFmlUsrEnterResponse is the namespace-delta response for gs.shopFmlUsr.enter.
type ShopFmlUsrEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.shopFmlUsr.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlUsrRPC) Enter(ctx context.Context, req ShopFmlUsrEnterRequest, opts ...RequestOption) (ShopFmlUsrEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlUsrEnter, req, opts...)
}

// ShopFmlUsrRefreshRequest is the empty request body for gs.shopFmlUsr.refresh.
type ShopFmlUsrRefreshRequest struct{}

// ShopFmlUsrRefreshResponse is the namespace-delta response for gs.shopFmlUsr.refresh.
type ShopFmlUsrRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.shopFmlUsr.refresh. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlUsrRPC) Refresh(ctx context.Context, req ShopFmlUsrRefreshRequest, opts ...RequestOption) (ShopFmlUsrRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlUsrRefresh, req, opts...)
}

// ShopFmlUsrUnlockSlotRequest is the request body for gs.shopFmlUsr.unlockSlot.
type ShopFmlUsrUnlockSlotRequest struct {
	SlotId RPCID `json:"slotId,omitempty"`
}

// ShopFmlUsrUnlockSlotResponse is the namespace-delta response for gs.shopFmlUsr.unlockSlot.
type ShopFmlUsrUnlockSlotResponse = RPCResponse[StateDelta]

// UnlockSlot calls gs.shopFmlUsr.unlockSlot. Request fields inferred from game.js: slotId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopFmlUsrRPC) UnlockSlot(ctx context.Context, req ShopFmlUsrUnlockSlotRequest, opts ...RequestOption) (ShopFmlUsrUnlockSlotResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopFmlUsrUnlockSlot, req, opts...)
}

// ShopGiftbag returns typed RPC helpers for the shopGiftbag namespace.
func (c *RPCClient) ShopGiftbag() ShopGiftbagRPC { return ShopGiftbagRPC{c: c} }

type ShopGiftbagRPC struct{ c *RPCClient }

// ShopGiftbagBuyRequest is the request body for gs.shopGiftbag.buy.
type ShopGiftbagBuyRequest struct {
	ShopId RPCID  `json:"shopId,omitempty"`
	Num    RPCInt `json:"num,omitempty"`
}

// ShopGiftbagBuyResponse is the namespace-delta response for gs.shopGiftbag.buy.
type ShopGiftbagBuyResponse = RPCResponse[StateDelta]

// Buy calls gs.shopGiftbag.buy. Request fields inferred from game.js: shopId, num.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopGiftbagRPC) Buy(ctx context.Context, req ShopGiftbagBuyRequest, opts ...RequestOption) (ShopGiftbagBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopGiftbagBuy, req, opts...)
}

// ShopGiftbagEnterRequest is the empty request body for gs.shopGiftbag.enter.
type ShopGiftbagEnterRequest struct{}

// ShopGiftbagEnterResponse is the namespace-delta response for gs.shopGiftbag.enter.
type ShopGiftbagEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.shopGiftbag.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ShopGiftbagRPC) Enter(ctx context.Context, req ShopGiftbagEnterRequest, opts ...RequestOption) (ShopGiftbagEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCShopGiftbagEnter, req, opts...)
}

// Sign returns typed RPC helpers for the sign namespace.
func (c *RPCClient) Sign() SignRPC { return SignRPC{c: c} }

type SignRPC struct{ c *RPCClient }

// SignRecvGradeRwdRequest is the request body for gs.sign.recvGradeRwd.
type SignRecvGradeRwdRequest struct {
	GradeIdx RPCID `json:"gradeIdx,omitempty"`
}

// SignRecvGradeRwdResponse is the namespace-delta response for gs.sign.recvGradeRwd.
type SignRecvGradeRwdResponse = RPCResponse[StateDelta]

// RecvGradeRwd calls gs.sign.recvGradeRwd. Request fields inferred from game.js: gradeIdx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SignRPC) RecvGradeRwd(ctx context.Context, req SignRecvGradeRwdRequest, opts ...RequestOption) (SignRecvGradeRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSignRecvGradeRwd, req, opts...)
}

// SignSignRequest is the request body for gs.sign.sign.
type SignSignRequest struct {
	PatchDay RPCInt `json:"patchDay,omitempty"`
}

// SignSignResponse is the namespace-delta response for gs.sign.sign.
type SignSignResponse = RPCResponse[StateDelta]

// Sign calls gs.sign.sign. Request fields inferred from game.js: patchDay.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SignRPC) Sign(ctx context.Context, req SignSignRequest, opts ...RequestOption) (SignSignResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSignSign, req, opts...)
}

// SignSignSevenRequest is the request body for gs.sign.sign_seven.
type SignSignSevenRequest struct {
	Day RPCInt `json:"day,omitempty"`
}

// SignSignSevenResponse is the namespace-delta response for gs.sign.sign_seven.
type SignSignSevenResponse = RPCResponse[StateDelta]

// SignSeven calls gs.sign.sign_seven. Request fields inferred from game.js: day.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SignRPC) SignSeven(ctx context.Context, req SignSignSevenRequest, opts ...RequestOption) (SignSignSevenResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSignSignSeven, req, opts...)
}

// SignType returns typed RPC helpers for the signType namespace.
func (c *RPCClient) SignType() SignTypeRPC { return SignTypeRPC{c: c} }

type SignTypeRPC struct{ c *RPCClient }

// SignTypeEnterRequest carries JSON fields for gs.signType.enter; game.js did not expose a stable object literal for this request.
type SignTypeEnterRequest RawRequest

// SignTypeEnterResponse is the namespace-delta response for gs.signType.enter.
type SignTypeEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.signType.enter. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SignTypeRPC) Enter(ctx context.Context, req SignTypeEnterRequest, opts ...RequestOption) (SignTypeEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSignTypeEnter, req, opts...)
}

// SignTypeRecvRequest carries JSON fields for gs.signType.recv; game.js did not expose a stable object literal for this request.
type SignTypeRecvRequest RawRequest

// SignTypeRecvResponse is the namespace-delta response for gs.signType.recv.
type SignTypeRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.signType.recv. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SignTypeRPC) Recv(ctx context.Context, req SignTypeRecvRequest, opts ...RequestOption) (SignTypeRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSignTypeRecv, req, opts...)
}

// SignTypeSignRequest carries JSON fields for gs.signType.sign; game.js did not expose a stable object literal for this request.
type SignTypeSignRequest RawRequest

// SignTypeSignResponse is the namespace-delta response for gs.signType.sign.
type SignTypeSignResponse = RPCResponse[StateDelta]

// Sign calls gs.signType.sign. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SignTypeRPC) Sign(ctx context.Context, req SignTypeSignRequest, opts ...RequestOption) (SignTypeSignResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSignTypeSign, req, opts...)
}

// StoryMain returns typed RPC helpers for the storyMain namespace.
func (c *RPCClient) StoryMain() StoryMainRPC { return StoryMainRPC{c: c} }

type StoryMainRPC struct{ c *RPCClient }

// StoryMainEnterRequest is the empty request body for gs.storyMain.enter.
type StoryMainEnterRequest struct{}

// StoryMainEnterResponse is the namespace-delta response for gs.storyMain.enter.
type StoryMainEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.storyMain.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r StoryMainRPC) Enter(ctx context.Context, req StoryMainEnterRequest, opts ...RequestOption) (StoryMainEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCStoryMainEnter, req, opts...)
}

// StoryMainUnlockRequest is the empty request body for gs.storyMain.unlock.
type StoryMainUnlockRequest struct{}

// StoryMainUnlockResponse is the namespace-delta response for gs.storyMain.unlock.
type StoryMainUnlockResponse = RPCResponse[StateDelta]

// Unlock calls gs.storyMain.unlock. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r StoryMainRPC) Unlock(ctx context.Context, req StoryMainUnlockRequest, opts ...RequestOption) (StoryMainUnlockResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCStoryMainUnlock, req, opts...)
}

// Sys returns typed RPC helpers for the sys namespace.
func (c *RPCClient) Sys() SysRPC { return SysRPC{c: c} }

type SysRPC struct{ c *RPCClient }

// SysInformChatRequest carries JSON fields for gs.sys.informChat; game.js did not expose a stable object literal for this request.
type SysInformChatRequest RawRequest

// SysInformChatResponse is the namespace-delta response for gs.sys.informChat.
type SysInformChatResponse = RPCResponse[StateDelta]

// InformChat calls gs.sys.informChat. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SysRPC) InformChat(ctx context.Context, req SysInformChatRequest, opts ...RequestOption) (SysInformChatResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSysInformChat, req, opts...)
}

// SysInformFmlRequest carries JSON fields for gs.sys.informFml; game.js did not expose a stable object literal for this request.
type SysInformFmlRequest RawRequest

// SysInformFmlResponse is the namespace-delta response for gs.sys.informFml.
type SysInformFmlResponse = RPCResponse[StateDelta]

// InformFml calls gs.sys.informFml. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SysRPC) InformFml(ctx context.Context, req SysInformFmlRequest, opts ...RequestOption) (SysInformFmlResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSysInformFml, req, opts...)
}

// SysInformUsrRequest carries JSON fields for gs.sys.informUsr; game.js did not expose a stable object literal for this request.
type SysInformUsrRequest RawRequest

// SysInformUsrResponse is the namespace-delta response for gs.sys.informUsr.
type SysInformUsrResponse = RPCResponse[StateDelta]

// InformUsr calls gs.sys.informUsr. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r SysRPC) InformUsr(ctx context.Context, req SysInformUsrRequest, opts ...RequestOption) (SysInformUsrResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCSysInformUsr, req, opts...)
}

// TaskAch returns typed RPC helpers for the taskAch namespace.
func (c *RPCClient) TaskAch() TaskAchRPC { return TaskAchRPC{c: c} }

type TaskAchRPC struct{ c *RPCClient }

// TaskAchRecvRequest is the request body for gs.taskAch.recv.
type TaskAchRecvRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// TaskAchRecvResponse is the namespace-delta response for gs.taskAch.recv.
type TaskAchRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.taskAch.recv. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskAchRPC) Recv(ctx context.Context, req TaskAchRecvRequest, opts ...RequestOption) (TaskAchRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskAchRecv, req, opts...)
}

// TaskAchRecvOneKeyRequest is the request body for gs.taskAch.recvOneKey.
type TaskAchRecvOneKeyRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// TaskAchRecvOneKeyResponse is the namespace-delta response for gs.taskAch.recvOneKey.
type TaskAchRecvOneKeyResponse = RPCResponse[StateDelta]

// RecvOneKey calls gs.taskAch.recvOneKey. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskAchRPC) RecvOneKey(ctx context.Context, req TaskAchRecvOneKeyRequest, opts ...RequestOption) (TaskAchRecvOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskAchRecvOneKey, req, opts...)
}

// TaskDly returns typed RPC helpers for the taskDly namespace.
func (c *RPCClient) TaskDly() TaskDlyRPC { return TaskDlyRPC{c: c} }

type TaskDlyRPC struct{ c *RPCClient }

// TaskDlyEnterRequest is the empty request body for gs.taskDly.enter.
type TaskDlyEnterRequest struct{}

// TaskDlyEnterResponse is the namespace-delta response for gs.taskDly.enter.
type TaskDlyEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.taskDly.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskDlyRPC) Enter(ctx context.Context, req TaskDlyEnterRequest, opts ...RequestOption) (TaskDlyEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskDlyEnter, req, opts...)
}

// TaskDlyRecvRequest is the request body for gs.taskDly.recv.
type TaskDlyRecvRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// TaskDlyRecvResponse is the namespace-delta response for gs.taskDly.recv.
type TaskDlyRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.taskDly.recv. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskDlyRPC) Recv(ctx context.Context, req TaskDlyRecvRequest, opts ...RequestOption) (TaskDlyRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskDlyRecv, req, opts...)
}

// TaskDlyRecvBoxRequest is the request body for gs.taskDly.recvBox.
type TaskDlyRecvBoxRequest struct {
	Idx RPCInt `json:"idx,omitempty"`
}

// TaskDlyRecvBoxResponse is the namespace-delta response for gs.taskDly.recvBox.
type TaskDlyRecvBoxResponse = RPCResponse[StateDelta]

// RecvBox calls gs.taskDly.recvBox. Request fields inferred from game.js: idx.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskDlyRPC) RecvBox(ctx context.Context, req TaskDlyRecvBoxRequest, opts ...RequestOption) (TaskDlyRecvBoxResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskDlyRecvBox, req, opts...)
}

// TaskInv returns typed RPC helpers for the taskInv namespace.
func (c *RPCClient) TaskInv() TaskInvRPC { return TaskInvRPC{c: c} }

type TaskInvRPC struct{ c *RPCClient }

// TaskInvRecvRequest is the request body for gs.taskInv.recv.
type TaskInvRecvRequest struct {
	ID    RPCID   `json:"id,omitempty"`
	IsPro RPCBool `json:"isPro,omitempty"`
}

// TaskInvRecvResponse is the namespace-delta response for gs.taskInv.recv.
type TaskInvRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.taskInv.recv. Request fields inferred from game.js: id, isPro.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskInvRPC) Recv(ctx context.Context, req TaskInvRecvRequest, opts ...RequestOption) (TaskInvRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskInvRecv, req, opts...)
}

// TaskInvRecvOneKeyRequest is the request body for gs.taskInv.recvOneKey.
type TaskInvRecvOneKeyRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// TaskInvRecvOneKeyResponse is the namespace-delta response for gs.taskInv.recvOneKey.
type TaskInvRecvOneKeyResponse = RPCResponse[StateDelta]

// RecvOneKey calls gs.taskInv.recvOneKey. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskInvRPC) RecvOneKey(ctx context.Context, req TaskInvRecvOneKeyRequest, opts ...RequestOption) (TaskInvRecvOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskInvRecvOneKey, req, opts...)
}

// TaskMain returns typed RPC helpers for the taskMain namespace.
func (c *RPCClient) TaskMain() TaskMainRPC { return TaskMainRPC{c: c} }

type TaskMainRPC struct{ c *RPCClient }

// TaskMainRecvRequest is the empty request body for gs.taskMain.recv.
type TaskMainRecvRequest struct{}

// TaskMainRecvResponse is the namespace-delta response for gs.taskMain.recv.
type TaskMainRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.taskMain.recv. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskMainRPC) Recv(ctx context.Context, req TaskMainRecvRequest, opts ...RequestOption) (TaskMainRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskMainRecv, req, opts...)
}

// TaskSys returns typed RPC helpers for the taskSys namespace.
func (c *RPCClient) TaskSys() TaskSysRPC { return TaskSysRPC{c: c} }

type TaskSysRPC struct{ c *RPCClient }

// TaskSysGiftBuyRequest is the request body for gs.taskSys.giftBuy.
type TaskSysGiftBuyRequest struct {
	GiftId RPCID `json:"giftId,omitempty"`
}

// TaskSysGiftBuyResponse is the namespace-delta response for gs.taskSys.giftBuy.
type TaskSysGiftBuyResponse = RPCResponse[StateDelta]

// GiftBuy calls gs.taskSys.giftBuy. Request fields inferred from game.js: giftId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskSysRPC) GiftBuy(ctx context.Context, req TaskSysGiftBuyRequest, opts ...RequestOption) (TaskSysGiftBuyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskSysGiftBuy, req, opts...)
}

// TaskSysRecvRequest is the request body for gs.taskSys.recv.
type TaskSysRecvRequest struct {
	TaskId RPCID `json:"taskId,omitempty"`
}

// TaskSysRecvResponse is the namespace-delta response for gs.taskSys.recv.
type TaskSysRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.taskSys.recv. Request fields inferred from game.js: taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskSysRPC) Recv(ctx context.Context, req TaskSysRecvRequest, opts ...RequestOption) (TaskSysRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskSysRecv, req, opts...)
}

// TaskSysRecvLvlRwdRequest is the request body for gs.taskSys.recvLvlRwd.
type TaskSysRecvLvlRwdRequest struct {
	Classify RPCInt `json:"classify,omitempty"`
	Lvl      RPCInt `json:"lvl,omitempty"`
}

// TaskSysRecvLvlRwdResponse is the namespace-delta response for gs.taskSys.recvLvlRwd.
type TaskSysRecvLvlRwdResponse = RPCResponse[StateDelta]

// RecvLvlRwd calls gs.taskSys.recvLvlRwd. Request fields inferred from game.js: classify, lvl.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskSysRPC) RecvLvlRwd(ctx context.Context, req TaskSysRecvLvlRwdRequest, opts ...RequestOption) (TaskSysRecvLvlRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskSysRecvLvlRwd, req, opts...)
}

// TaskSysRecvOneKeyRequest is the request body for gs.taskSys.recvOneKey.
type TaskSysRecvOneKeyRequest struct {
	Classify RPCInt `json:"classify,omitempty"`
	GrpId    RPCInt `json:"grpId,omitempty"`
}

// TaskSysRecvOneKeyResponse is the namespace-delta response for gs.taskSys.recvOneKey.
type TaskSysRecvOneKeyResponse = RPCResponse[StateDelta]

// RecvOneKey calls gs.taskSys.recvOneKey. Request fields inferred from game.js: classify, grpId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskSysRPC) RecvOneKey(ctx context.Context, req TaskSysRecvOneKeyRequest, opts ...RequestOption) (TaskSysRecvOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskSysRecvOneKey, req, opts...)
}

// TaskWeek returns typed RPC helpers for the taskWeek namespace.
func (c *RPCClient) TaskWeek() TaskWeekRPC { return TaskWeekRPC{c: c} }

type TaskWeekRPC struct{ c *RPCClient }

// TaskWeekRecvRequest is the request body for gs.taskWeek.recv.
type TaskWeekRecvRequest struct {
	ID RPCID `json:"id,omitempty"`
}

// TaskWeekRecvResponse is the namespace-delta response for gs.taskWeek.recv.
type TaskWeekRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.taskWeek.recv. Request fields inferred from game.js: id.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TaskWeekRPC) Recv(ctx context.Context, req TaskWeekRecvRequest, opts ...RequestOption) (TaskWeekRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTaskWeekRecv, req, opts...)
}

// TeamOrderPopup returns typed RPC helpers for the teamOrderPopup namespace.
func (c *RPCClient) TeamOrderPopup() TeamOrderPopupRPC { return TeamOrderPopupRPC{c: c} }

type TeamOrderPopupRPC struct{ c *RPCClient }

// TeamOrderPopupShowTRequest is the request body for gs.teamOrderPopup.showT.
type TeamOrderPopupShowTRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// TeamOrderPopupShowTResponse is the namespace-delta response for gs.teamOrderPopup.showT.
type TeamOrderPopupShowTResponse = RPCResponse[StateDelta]

// ShowT calls gs.teamOrderPopup.showT. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TeamOrderPopupRPC) ShowT(ctx context.Context, req TeamOrderPopupShowTRequest, opts ...RequestOption) (TeamOrderPopupShowTResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTeamOrderPopupShowT, req, opts...)
}

// Thirdparty returns typed RPC helpers for the thirdparty namespace.
func (c *RPCClient) Thirdparty() ThirdpartyRPC { return ThirdpartyRPC{c: c} }

type ThirdpartyRPC struct{ c *RPCClient }

// ThirdpartyApplyTokenRequest is the request body for gs.thirdparty.applyToken.
type ThirdpartyApplyTokenRequest struct {
	Type RPCInt `json:"type,omitempty"`
	UID  RPCUID `json:"uid,omitempty"`
}

// ThirdpartyApplyTokenResponse is the namespace-delta response for gs.thirdparty.applyToken.
type ThirdpartyApplyTokenResponse = RPCResponse[StateDelta]

// ApplyToken calls gs.thirdparty.applyToken. Request fields inferred from game.js: type, uid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ThirdpartyRPC) ApplyToken(ctx context.Context, req ThirdpartyApplyTokenRequest, opts ...RequestOption) (ThirdpartyApplyTokenResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCThirdpartyApplyToken, req, opts...)
}

// Title returns typed RPC helpers for the title namespace.
func (c *RPCClient) Title() TitleRPC { return TitleRPC{c: c} }

type TitleRPC struct{ c *RPCClient }

// TitleActiveTitleRequest is the request body for gs.title.activeTitle.
type TitleActiveTitleRequest struct {
	TitleId RPCID `json:"titleId,omitempty"`
}

// TitleActiveTitleResponse is the namespace-delta response for gs.title.activeTitle.
type TitleActiveTitleResponse = RPCResponse[StateDelta]

// ActiveTitle calls gs.title.activeTitle. Request fields inferred from game.js: titleId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TitleRPC) ActiveTitle(ctx context.Context, req TitleActiveTitleRequest, opts ...RequestOption) (TitleActiveTitleResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTitleActiveTitle, req, opts...)
}

// TitleChgTitleRequest is the request body for gs.title.chgTitle.
type TitleChgTitleRequest struct {
	TitleId RPCID `json:"titleId,omitempty"`
}

// TitleChgTitleResponse is the namespace-delta response for gs.title.chgTitle.
type TitleChgTitleResponse = RPCResponse[StateDelta]

// ChgTitle calls gs.title.chgTitle. Request fields inferred from game.js: titleId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TitleRPC) ChgTitle(ctx context.Context, req TitleChgTitleRequest, opts ...RequestOption) (TitleChgTitleResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTitleChgTitle, req, opts...)
}

// TitleSetTitleShowRequest is the request body for gs.title.setTitleShow.
type TitleSetTitleShowRequest struct {
	TitleIds RPCIDList `json:"titleIds,omitempty"`
}

// TitleSetTitleShowResponse is the namespace-delta response for gs.title.setTitleShow.
type TitleSetTitleShowResponse = RPCResponse[StateDelta]

// SetTitleShow calls gs.title.setTitleShow. Request fields inferred from game.js: titleIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TitleRPC) SetTitleShow(ctx context.Context, req TitleSetTitleShowRequest, opts ...RequestOption) (TitleSetTitleShowResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTitleSetTitleShow, req, opts...)
}

// TokenInfo returns typed RPC helpers for the tokenInfo namespace.
func (c *RPCClient) TokenInfo() TokenInfoRPC { return TokenInfoRPC{c: c} }

type TokenInfoRPC struct{ c *RPCClient }

// TokenInfoGetTokenRequest is the request body for gs.tokenInfo.getToken.
type TokenInfoGetTokenRequest struct {
	Type  RPCInt   `json:"type,omitempty"`
	Param RPCValue `json:"param,omitempty"`
}

// TokenInfoGetTokenResponse is the namespace-delta response for gs.tokenInfo.getToken.
type TokenInfoGetTokenResponse = RPCResponse[StateDelta]

// GetToken calls gs.tokenInfo.getToken. Request fields inferred from game.js: type, param.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TokenInfoRPC) GetToken(ctx context.Context, req TokenInfoGetTokenRequest, opts ...RequestOption) (TokenInfoGetTokenResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTokenInfoGetToken, req, opts...)
}

// TtMoneyTask returns typed RPC helpers for the ttMoneyTask namespace.
func (c *RPCClient) TtMoneyTask() TtMoneyTaskRPC { return TtMoneyTaskRPC{c: c} }

type TtMoneyTaskRPC struct{ c *RPCClient }

// TtMoneyTaskGenGldOrderRequest is the request body for gs.ttMoneyTask.genGldOrder.
type TtMoneyTaskGenGldOrderRequest struct {
	TaskId RPCID `json:"taskId,omitempty"`
}

// TtMoneyTaskGenGldOrderResponse is the namespace-delta response for gs.ttMoneyTask.genGldOrder.
type TtMoneyTaskGenGldOrderResponse = RPCResponse[StateDelta]

// GenGldOrder calls gs.ttMoneyTask.genGldOrder. Request fields inferred from game.js: taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TtMoneyTaskRPC) GenGldOrder(ctx context.Context, req TtMoneyTaskGenGldOrderRequest, opts ...RequestOption) (TtMoneyTaskGenGldOrderResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTtMoneyTaskGenGldOrder, req, opts...)
}

// TtMoneyTaskRecvRequest is the request body for gs.ttMoneyTask.recv.
type TtMoneyTaskRecvRequest struct {
	TaskId RPCID `json:"taskId,omitempty"`
}

// TtMoneyTaskRecvResponse is the namespace-delta response for gs.ttMoneyTask.recv.
type TtMoneyTaskRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.ttMoneyTask.recv. Request fields inferred from game.js: taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TtMoneyTaskRPC) Recv(ctx context.Context, req TtMoneyTaskRecvRequest, opts ...RequestOption) (TtMoneyTaskRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTtMoneyTaskRecv, req, opts...)
}

// TtMoneyTaskRefreshRequest is the empty request body for gs.ttMoneyTask.refresh.
type TtMoneyTaskRefreshRequest struct{}

// TtMoneyTaskRefreshResponse is the namespace-delta response for gs.ttMoneyTask.refresh.
type TtMoneyTaskRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.ttMoneyTask.refresh. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r TtMoneyTaskRPC) Refresh(ctx context.Context, req TtMoneyTaskRefreshRequest, opts ...RequestOption) (TtMoneyTaskRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCTtMoneyTaskRefresh, req, opts...)
}

// Usr returns typed RPC helpers for the usr namespace.
func (c *RPCClient) Usr() UsrRPC { return UsrRPC{c: c} }

type UsrRPC struct{ c *RPCClient }

// UsrActiveCardRequest is the request body for gs.usr.activeCard.
type UsrActiveCardRequest struct {
	CardId RPCID `json:"cardId,omitempty"`
}

// UsrActiveCardResponse is the namespace-delta response for gs.usr.activeCard.
type UsrActiveCardResponse = RPCResponse[StateDelta]

// ActiveCard calls gs.usr.activeCard. Request fields inferred from game.js: cardId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ActiveCard(ctx context.Context, req UsrActiveCardRequest, opts ...RequestOption) (UsrActiveCardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrActiveCard, req, opts...)
}

// UsrActiveEmojiRequest carries JSON fields for gs.usr.activeEmoji; game.js did not expose a stable object literal for this request.
type UsrActiveEmojiRequest RawRequest

// UsrActiveEmojiResponse is the namespace-delta response for gs.usr.activeEmoji.
type UsrActiveEmojiResponse = RPCResponse[StateDelta]

// ActiveEmoji calls gs.usr.activeEmoji. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ActiveEmoji(ctx context.Context, req UsrActiveEmojiRequest, opts ...RequestOption) (UsrActiveEmojiResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrActiveEmoji, req, opts...)
}

// UsrActiveHeadRequest is the request body for gs.usr.activeHead.
type UsrActiveHeadRequest struct {
	HeadId RPCID `json:"headId,omitempty"`
}

// UsrActiveHeadResponse is the namespace-delta response for gs.usr.activeHead.
type UsrActiveHeadResponse = RPCResponse[StateDelta]

// ActiveHead calls gs.usr.activeHead. Request fields inferred from game.js: headId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ActiveHead(ctx context.Context, req UsrActiveHeadRequest, opts ...RequestOption) (UsrActiveHeadResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrActiveHead, req, opts...)
}

// UsrActiveMedalRequest is the request body for gs.usr.activeMedal.
type UsrActiveMedalRequest struct {
	MedalId RPCID `json:"medalId,omitempty"`
}

// UsrActiveMedalResponse is the namespace-delta response for gs.usr.activeMedal.
type UsrActiveMedalResponse = RPCResponse[StateDelta]

// ActiveMedal calls gs.usr.activeMedal. Request fields inferred from game.js: medalId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ActiveMedal(ctx context.Context, req UsrActiveMedalRequest, opts ...RequestOption) (UsrActiveMedalResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrActiveMedal, req, opts...)
}

// UsrAfterShareRequest is the request body for gs.usr.afterShare.
type UsrAfterShareRequest struct {
	ShareId RPCID     `json:"shareId,omitempty"`
	Ext     RPCObject `json:"ext,omitempty"`
}

// UsrAfterShareResponse is the namespace-delta response for gs.usr.afterShare.
type UsrAfterShareResponse = RPCResponse[StateDelta]

// AfterShare calls gs.usr.afterShare. Request fields inferred from game.js: shareId, ext.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) AfterShare(ctx context.Context, req UsrAfterShareRequest, opts ...RequestOption) (UsrAfterShareResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrAfterShare, req, opts...)
}

// UsrChgCardRequest is the request body for gs.usr.chgCard.
type UsrChgCardRequest struct {
	CardId RPCID `json:"cardId,omitempty"`
}

// UsrChgCardResponse is the namespace-delta response for gs.usr.chgCard.
type UsrChgCardResponse = RPCResponse[StateDelta]

// ChgCard calls gs.usr.chgCard. Request fields inferred from game.js: cardId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ChgCard(ctx context.Context, req UsrChgCardRequest, opts ...RequestOption) (UsrChgCardResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrChgCard, req, opts...)
}

// UsrChgFaceRequest is the request body for gs.usr.chgFace.
type UsrChgFaceRequest struct {
	FaceId RPCID `json:"faceId,omitempty"`
}

// UsrChgFaceResponse is the namespace-delta response for gs.usr.chgFace.
type UsrChgFaceResponse = RPCResponse[StateDelta]

// ChgFace calls gs.usr.chgFace. Request fields inferred from game.js: faceId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ChgFace(ctx context.Context, req UsrChgFaceRequest, opts ...RequestOption) (UsrChgFaceResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrChgFace, req, opts...)
}

// UsrChgIcoRequest is the request body for gs.usr.chgIco.
type UsrChgIcoRequest struct {
	Ico RPCString `json:"ico,omitempty"`
}

// UsrChgIcoResponse is the namespace-delta response for gs.usr.chgIco.
type UsrChgIcoResponse = RPCResponse[StateDelta]

// ChgIco calls gs.usr.chgIco. Request fields inferred from game.js: ico.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ChgIco(ctx context.Context, req UsrChgIcoRequest, opts ...RequestOption) (UsrChgIcoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrChgIco, req, opts...)
}

// UsrChgNameRequest is the request body for gs.usr.chgName.
type UsrChgNameRequest struct {
	Name   RPCString `json:"name,omitempty"`
	IsFree RPCBool   `json:"isFree,omitempty"`
}

// UsrChgNameResponse is the namespace-delta response for gs.usr.chgName.
type UsrChgNameResponse = RPCResponse[StateDelta]

// ChgName calls gs.usr.chgName. Request fields inferred from game.js: name, isFree.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ChgName(ctx context.Context, req UsrChgNameRequest, opts ...RequestOption) (UsrChgNameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrChgName, req, opts...)
}

// UsrChgSexRequest is the request body for gs.usr.chgSex.
type UsrChgSexRequest struct {
	SexId RPCInt `json:"sexId,omitempty"`
}

// UsrChgSexResponse is the namespace-delta response for gs.usr.chgSex.
type UsrChgSexResponse = RPCResponse[StateDelta]

// ChgSex calls gs.usr.chgSex. Request fields inferred from game.js: sexId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ChgSex(ctx context.Context, req UsrChgSexRequest, opts ...RequestOption) (UsrChgSexResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrChgSex, req, opts...)
}

// UsrChgSignRequest is the request body for gs.usr.chgSign.
type UsrChgSignRequest struct {
	Sign RPCString `json:"sign,omitempty"`
}

// UsrChgSignResponse is the namespace-delta response for gs.usr.chgSign.
type UsrChgSignResponse = RPCResponse[StateDelta]

// ChgSign calls gs.usr.chgSign. Request fields inferred from game.js: sign.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ChgSign(ctx context.Context, req UsrChgSignRequest, opts ...RequestOption) (UsrChgSignResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrChgSign, req, opts...)
}

// UsrClearVipServiceRequest carries JSON fields for gs.usr.clearVipService; game.js did not expose a stable object literal for this request.
type UsrClearVipServiceRequest RawRequest

// UsrClearVipServiceResponse is the namespace-delta response for gs.usr.clearVipService.
type UsrClearVipServiceResponse = RPCResponse[StateDelta]

// ClearVipService calls gs.usr.clearVipService. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) ClearVipService(ctx context.Context, req UsrClearVipServiceRequest, opts ...RequestOption) (UsrClearVipServiceResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrClearVipService, req, opts...)
}

// UsrGetSalaryRequest is the empty request body for gs.usr.getSalary.
type UsrGetSalaryRequest struct{}

// UsrGetSalaryResponse is the namespace-delta response for gs.usr.getSalary.
type UsrGetSalaryResponse = RPCResponse[StateDelta]

// GetSalary calls gs.usr.getSalary. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) GetSalary(ctx context.Context, req UsrGetSalaryRequest, opts ...RequestOption) (UsrGetSalaryResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrGetSalary, req, opts...)
}

// UsrHeartTickRequest is the empty request body for gs.usr.heartTick.
type UsrHeartTickRequest struct{}

// UsrHeartTickResponse is the namespace-delta response for gs.usr.heartTick.
type UsrHeartTickResponse = RPCResponse[StateDelta]

// HeartTick calls gs.usr.heartTick. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) HeartTick(ctx context.Context, req UsrHeartTickRequest, opts ...RequestOption) (UsrHeartTickResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrHeartTick, req, opts...)
}

// UsrLazySyncRequest is the empty request body for gs.usr.lazySync.
type UsrLazySyncRequest struct{}

// UsrLazySyncResponse is the namespace-delta response for gs.usr.lazySync.
type UsrLazySyncResponse = RPCResponse[StateDelta]

// LazySync calls gs.usr.lazySync. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) LazySync(ctx context.Context, req UsrLazySyncRequest, opts ...RequestOption) (UsrLazySyncResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLazySync, req, opts...)
}

// UsrRecvSignRwdRequest is the request body for gs.usr.recvSignRwd.
type UsrRecvSignRwdRequest struct {
	MedalId RPCID `json:"medalId,omitempty"`
}

// UsrRecvSignRwdResponse is the namespace-delta response for gs.usr.recvSignRwd.
type UsrRecvSignRwdResponse = RPCResponse[StateDelta]

// RecvSignRwd calls gs.usr.recvSignRwd. Request fields inferred from game.js: medalId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) RecvSignRwd(ctx context.Context, req UsrRecvSignRwdRequest, opts ...RequestOption) (UsrRecvSignRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrRecvSignRwd, req, opts...)
}

// UsrRefreshMedalRequest is the request body for gs.usr.refreshMedal.
type UsrRefreshMedalRequest struct {
	MedalId RPCID `json:"medalId,omitempty"`
}

// UsrRefreshMedalResponse is the namespace-delta response for gs.usr.refreshMedal.
type UsrRefreshMedalResponse = RPCResponse[StateDelta]

// RefreshMedal calls gs.usr.refreshMedal. Request fields inferred from game.js: medalId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) RefreshMedal(ctx context.Context, req UsrRefreshMedalRequest, opts ...RequestOption) (UsrRefreshMedalResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrRefreshMedal, req, opts...)
}

// UsrSaveAuthInfoRequest is the request body for gs.usr.saveAuthInfo.
type UsrSaveAuthInfoRequest struct {
	AuthInfo RPCObject `json:"authInfo,omitempty"`
}

// UsrSaveAuthInfoResponse is the namespace-delta response for gs.usr.saveAuthInfo.
type UsrSaveAuthInfoResponse = RPCResponse[StateDelta]

// SaveAuthInfo calls gs.usr.saveAuthInfo. Request fields inferred from game.js: authInfo.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) SaveAuthInfo(ctx context.Context, req UsrSaveAuthInfoRequest, opts ...RequestOption) (UsrSaveAuthInfoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSaveAuthInfo, req, opts...)
}

// UsrSaveCustIcoRequest is the request body for gs.usr.saveCustIco.
type UsrSaveCustIcoRequest struct {
	IcoMD5 RPCString `json:"icoMD5,omitempty"`
	Ico64  RPCString `json:"ico64,omitempty"`
}

// UsrSaveCustIcoResponse is the namespace-delta response for gs.usr.saveCustIco.
type UsrSaveCustIcoResponse = RPCResponse[StateDelta]

// SaveCustIco calls gs.usr.saveCustIco. Request fields inferred from game.js: icoMD5, ico64.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) SaveCustIco(ctx context.Context, req UsrSaveCustIcoRequest, opts ...RequestOption) (UsrSaveCustIcoResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSaveCustIco, req, opts...)
}

// UsrSetMedalShowRequest is the request body for gs.usr.setMedalShow.
type UsrSetMedalShowRequest struct {
	MedalIds RPCIDList `json:"medalIds,omitempty"`
}

// UsrSetMedalShowResponse is the namespace-delta response for gs.usr.setMedalShow.
type UsrSetMedalShowResponse = RPCResponse[StateDelta]

// SetMedalShow calls gs.usr.setMedalShow. Request fields inferred from game.js: medalIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) SetMedalShow(ctx context.Context, req UsrSetMedalShowRequest, opts ...RequestOption) (UsrSetMedalShowResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSetMedalShow, req, opts...)
}

// UsrShareRequest is the request body for gs.usr.share.
type UsrShareRequest struct {
	ShareId int32     `json:"shareId,omitempty"`
	Ext     RPCObject `json:"ext,omitempty"`
}

// UsrShareResponse is the namespace-delta response for gs.usr.share.
type UsrShareResponse = RPCResponse[StateDelta]

// Share calls gs.usr.share. Request fields inferred from game.js: shareId, ext.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) Share(ctx context.Context, req UsrShareRequest, opts ...RequestOption) (UsrShareResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrShare, req, opts...)
}

// UsrTriggerEventRequest is the request body for gs.usr.triggerEvent.
type UsrTriggerEventRequest struct {
	Key    RPCString `json:"key,omitempty"`
	SubKey RPCString `json:"subKey,omitempty"`
	Param  RPCValue  `json:"param,omitempty"`
}

// UsrTriggerEventResponse is the namespace-delta response for gs.usr.triggerEvent.
type UsrTriggerEventResponse = RPCResponse[StateDelta]

// TriggerEvent calls gs.usr.triggerEvent. Request fields inferred from game.js: key, subKey, param.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) TriggerEvent(ctx context.Context, req UsrTriggerEventRequest, opts ...RequestOption) (UsrTriggerEventResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrTriggerEvent, req, opts...)
}

// UsrUpdateGuideRequest is the request body for gs.usr.updateGuide.
type UsrUpdateGuideRequest struct {
	GuideId RPCID `json:"guideId,omitempty"`
}

// UsrUpdateGuideResponse is the namespace-delta response for gs.usr.updateGuide.
type UsrUpdateGuideResponse = RPCResponse[StateDelta]

// UpdateGuide calls gs.usr.updateGuide. Request fields inferred from game.js: guideId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) UpdateGuide(ctx context.Context, req UsrUpdateGuideRequest, opts ...RequestOption) (UsrUpdateGuideResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrUpdateGuide, req, opts...)
}

// UsrUpdateSoftGuideRequest carries JSON fields for gs.usr.updateSoftGuide; game.js did not expose a stable object literal for this request.
type UsrUpdateSoftGuideRequest RawRequest

// UsrUpdateSoftGuideResponse is the namespace-delta response for gs.usr.updateSoftGuide.
type UsrUpdateSoftGuideResponse = RPCResponse[StateDelta]

// UpdateSoftGuide calls gs.usr.updateSoftGuide. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) UpdateSoftGuide(ctx context.Context, req UsrUpdateSoftGuideRequest, opts ...RequestOption) (UsrUpdateSoftGuideResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrUpdateSoftGuide, req, opts...)
}

// UsrUpdateUsrExtRequest is the request body for gs.usr.updateUsrExt.
type UsrUpdateUsrExtRequest struct {
	K RPCString `json:"k,omitempty"`
	V RPCValue  `json:"v,omitempty"`
}

// UsrUpdateUsrExtResponse is the namespace-delta response for gs.usr.updateUsrExt.
type UsrUpdateUsrExtResponse = RPCResponse[StateDelta]

// UpdateUsrExt calls gs.usr.updateUsrExt. Request fields inferred from game.js: k, v.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) UpdateUsrExt(ctx context.Context, req UsrUpdateUsrExtRequest, opts ...RequestOption) (UsrUpdateUsrExtResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrUpdateUsrExt, req, opts...)
}

// UsrUpdateUsrSetRequest is the request body for gs.usr.updateUsrSet.
type UsrUpdateUsrSetRequest struct {
	Type  RPCInt   `json:"type,omitempty"`
	Value RPCValue `json:"value,omitempty"`
}

// UsrUpdateUsrSetResponse is the namespace-delta response for gs.usr.updateUsrSet.
type UsrUpdateUsrSetResponse = RPCResponse[StateDelta]

// UpdateUsrSet calls gs.usr.updateUsrSet. Request fields inferred from game.js: type, value.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) UpdateUsrSet(ctx context.Context, req UsrUpdateUsrSetRequest, opts ...RequestOption) (UsrUpdateUsrSetResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrUpdateUsrSet, req, opts...)
}

// UsrUpdateVipServiceRequest carries JSON fields for gs.usr.updateVipService; game.js did not expose a stable object literal for this request.
type UsrUpdateVipServiceRequest RawRequest

// UsrUpdateVipServiceResponse is the namespace-delta response for gs.usr.updateVipService.
type UsrUpdateVipServiceResponse = RPCResponse[StateDelta]

// UpdateVipService calls gs.usr.updateVipService. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) UpdateVipService(ctx context.Context, req UsrUpdateVipServiceRequest, opts ...RequestOption) (UsrUpdateVipServiceResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrUpdateVipService, req, opts...)
}

// UsrUpgradeRequest is the empty request body for gs.usr.upgrade.
type UsrUpgradeRequest struct{}

// UsrUpgradeResponse is the namespace-delta response for gs.usr.upgrade.
type UsrUpgradeResponse = RPCResponse[StateDelta]

// Upgrade calls gs.usr.upgrade. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) Upgrade(ctx context.Context, req UsrUpgradeRequest, opts ...RequestOption) (UsrUpgradeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrUpgrade, req, opts...)
}

// UsrWorshipRequest is the request body for gs.usr.worship.
type UsrWorshipRequest struct {
	Type RPCInt `json:"type,omitempty"`
}

// UsrWorshipResponse is the namespace-delta response for gs.usr.worship.
type UsrWorshipResponse = RPCResponse[StateDelta]

// Worship calls gs.usr.worship. Request fields inferred from game.js: type.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRPC) Worship(ctx context.Context, req UsrWorshipRequest, opts ...RequestOption) (UsrWorshipResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrWorship, req, opts...)
}

// UsrExtra returns typed RPC helpers for the usrExtra namespace.
func (c *RPCClient) UsrExtra() UsrExtraRPC { return UsrExtraRPC{c: c} }

type UsrExtraRPC struct{ c *RPCClient }

// UsrExtraRecvAntiFraudQARwdRequest carries JSON fields for gs.usrExtra.recvAntiFraudQARwd; game.js did not expose a stable object literal for this request.
type UsrExtraRecvAntiFraudQARwdRequest RawRequest

// UsrExtraRecvAntiFraudQARwdResponse is the namespace-delta response for gs.usrExtra.recvAntiFraudQARwd.
type UsrExtraRecvAntiFraudQARwdResponse = RPCResponse[StateDelta]

// RecvAntiFraudQARwd calls gs.usrExtra.recvAntiFraudQARwd. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) RecvAntiFraudQARwd(ctx context.Context, req UsrExtraRecvAntiFraudQARwdRequest, opts ...RequestOption) (UsrExtraRecvAntiFraudQARwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraRecvAntiFraudQARwd, req, opts...)
}

// UsrExtraRecvTtFansRwdRequest is the request body for gs.usrExtra.recvTtFansRwd.
type UsrExtraRecvTtFansRwdRequest struct {
	TermId   RPCID `json:"termId,omitempty"`
	RewardId RPCID `json:"rewardId,omitempty"`
}

// UsrExtraRecvTtFansRwdResponse is the namespace-delta response for gs.usrExtra.recvTtFansRwd.
type UsrExtraRecvTtFansRwdResponse = RPCResponse[StateDelta]

// RecvTtFansRwd calls gs.usrExtra.recvTtFansRwd. Request fields inferred from game.js: termId, rewardId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) RecvTtFansRwd(ctx context.Context, req UsrExtraRecvTtFansRwdRequest, opts ...RequestOption) (UsrExtraRecvTtFansRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraRecvTtFansRwd, req, opts...)
}

// UsrExtraRecvVersionUpdateRwdRequest is the request body for gs.usrExtra.recvVersionUpdateRwd.
type UsrExtraRecvVersionUpdateRwdRequest struct {
	Version RPCString `json:"version,omitempty"`
}

// UsrExtraRecvVersionUpdateRwdResponse is the namespace-delta response for gs.usrExtra.recvVersionUpdateRwd.
type UsrExtraRecvVersionUpdateRwdResponse = RPCResponse[StateDelta]

// RecvVersionUpdateRwd calls gs.usrExtra.recvVersionUpdateRwd. Request fields inferred from game.js: version.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) RecvVersionUpdateRwd(ctx context.Context, req UsrExtraRecvVersionUpdateRwdRequest, opts ...RequestOption) (UsrExtraRecvVersionUpdateRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraRecvVersionUpdateRwd, req, opts...)
}

// UsrExtraRecvWbFansRwdRequest is the request body for gs.usrExtra.recvWbFansRwd.
type UsrExtraRecvWbFansRwdRequest struct {
	TermId   RPCID `json:"termId,omitempty"`
	RewardId RPCID `json:"rewardId,omitempty"`
}

// UsrExtraRecvWbFansRwdResponse is the namespace-delta response for gs.usrExtra.recvWbFansRwd.
type UsrExtraRecvWbFansRwdResponse = RPCResponse[StateDelta]

// RecvWbFansRwd calls gs.usrExtra.recvWbFansRwd. Request fields inferred from game.js: termId, rewardId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) RecvWbFansRwd(ctx context.Context, req UsrExtraRecvWbFansRwdRequest, opts ...RequestOption) (UsrExtraRecvWbFansRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraRecvWbFansRwd, req, opts...)
}

// UsrExtraReportUsrRequest carries JSON fields for gs.usrExtra.reportUsr; game.js did not expose a stable object literal for this request.
type UsrExtraReportUsrRequest RawRequest

// UsrExtraReportUsrResponse is the namespace-delta response for gs.usrExtra.reportUsr.
type UsrExtraReportUsrResponse = RPCResponse[StateDelta]

// ReportUsr calls gs.usrExtra.reportUsr. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) ReportUsr(ctx context.Context, req UsrExtraReportUsrRequest, opts ...RequestOption) (UsrExtraReportUsrResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraReportUsr, req, opts...)
}

// UsrExtraSetMsPwdRequest carries JSON fields for gs.usrExtra.setMsPwd; game.js did not expose a stable object literal for this request.
type UsrExtraSetMsPwdRequest RawRequest

// UsrExtraSetMsPwdResponse is the namespace-delta response for gs.usrExtra.setMsPwd.
type UsrExtraSetMsPwdResponse = RPCResponse[StateDelta]

// SetMsPwd calls gs.usrExtra.setMsPwd. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) SetMsPwd(ctx context.Context, req UsrExtraSetMsPwdRequest, opts ...RequestOption) (UsrExtraSetMsPwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraSetMsPwd, req, opts...)
}

// UsrExtraSetShowAddressRequest is the request body for gs.usrExtra.setShowAddress.
type UsrExtraSetShowAddressRequest struct {
	ShowAddress RPCBool `json:"showAddress,omitempty"`
}

// UsrExtraSetShowAddressResponse is the namespace-delta response for gs.usrExtra.setShowAddress.
type UsrExtraSetShowAddressResponse = RPCResponse[StateDelta]

// SetShowAddress calls gs.usrExtra.setShowAddress. Request fields inferred from game.js: showAddress.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) SetShowAddress(ctx context.Context, req UsrExtraSetShowAddressRequest, opts ...RequestOption) (UsrExtraSetShowAddressResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraSetShowAddress, req, opts...)
}

// UsrExtraShareMsgRequest carries JSON fields for gs.usrExtra.shareMsg; game.js did not expose a stable object literal for this request.
type UsrExtraShareMsgRequest RawRequest

// UsrExtraShareMsgResponse is the namespace-delta response for gs.usrExtra.shareMsg.
type UsrExtraShareMsgResponse = RPCResponse[StateDelta]

// ShareMsg calls gs.usrExtra.shareMsg. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) ShareMsg(ctx context.Context, req UsrExtraShareMsgRequest, opts ...RequestOption) (UsrExtraShareMsgResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraShareMsg, req, opts...)
}

// UsrExtraSyncAddressRequest is the empty request body for gs.usrExtra.syncAddress.
type UsrExtraSyncAddressRequest struct{}

// UsrExtraSyncAddressResponse is the namespace-delta response for gs.usrExtra.syncAddress.
type UsrExtraSyncAddressResponse = RPCResponse[StateDelta]

// SyncAddress calls gs.usrExtra.syncAddress. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) SyncAddress(ctx context.Context, req UsrExtraSyncAddressRequest, opts ...RequestOption) (UsrExtraSyncAddressResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraSyncAddress, req, opts...)
}

// UsrExtraUpdateAntiFraudQAStatusRequest carries JSON fields for gs.usrExtra.updateAntiFraudQAStatus; game.js did not expose a stable object literal for this request.
type UsrExtraUpdateAntiFraudQAStatusRequest RawRequest

// UsrExtraUpdateAntiFraudQAStatusResponse is the namespace-delta response for gs.usrExtra.updateAntiFraudQAStatus.
type UsrExtraUpdateAntiFraudQAStatusResponse = RPCResponse[StateDelta]

// UpdateAntiFraudQAStatus calls gs.usrExtra.updateAntiFraudQAStatus. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) UpdateAntiFraudQAStatus(ctx context.Context, req UsrExtraUpdateAntiFraudQAStatusRequest, opts ...RequestOption) (UsrExtraUpdateAntiFraudQAStatusResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraUpdateAntiFraudQAStatus, req, opts...)
}

// UsrExtraUpdateTaskMapRequest is the request body for gs.usrExtra.updateTaskMap.
type UsrExtraUpdateTaskMapRequest struct {
	TaskId RPCID `json:"taskId,omitempty"`
}

// UsrExtraUpdateTaskMapResponse is the namespace-delta response for gs.usrExtra.updateTaskMap.
type UsrExtraUpdateTaskMapResponse = RPCResponse[StateDelta]

// UpdateTaskMap calls gs.usrExtra.updateTaskMap. Request fields inferred from game.js: taskId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) UpdateTaskMap(ctx context.Context, req UsrExtraUpdateTaskMapRequest, opts ...RequestOption) (UsrExtraUpdateTaskMapResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraUpdateTaskMap, req, opts...)
}

// UsrExtraUpdateTtSubscribeRequest is the request body for gs.usrExtra.updateTtSubscribe.
type UsrExtraUpdateTtSubscribeRequest struct {
	Status RPCInt `json:"status,omitempty"`
}

// UsrExtraUpdateTtSubscribeResponse is the namespace-delta response for gs.usrExtra.updateTtSubscribe.
type UsrExtraUpdateTtSubscribeResponse = RPCResponse[StateDelta]

// UpdateTtSubscribe calls gs.usrExtra.updateTtSubscribe. Request fields inferred from game.js: status.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrExtraRPC) UpdateTtSubscribe(ctx context.Context, req UsrExtraUpdateTtSubscribeRequest, opts ...RequestOption) (UsrExtraUpdateTtSubscribeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrExtraUpdateTtSubscribe, req, opts...)
}

// UsrLand returns typed RPC helpers for the usrLand namespace.
func (c *RPCClient) UsrLand() UsrLandRPC { return UsrLandRPC{c: c} }

type UsrLandRPC struct{ c *RPCClient }

// UsrLandClearRequest is the request body for gs.usrLand.clear.
type UsrLandClearRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// UsrLandClearResponse is the namespace-delta response for gs.usrLand.clear.
type UsrLandClearResponse = RPCResponse[StateDelta]

// Clear calls gs.usrLand.clear. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) Clear(ctx context.Context, req UsrLandClearRequest, opts ...RequestOption) (UsrLandClearResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandClear, req, opts...)
}

// UsrLandClearBatchRequest is the request body for gs.usrLand.clearBatch.
type UsrLandClearBatchRequest struct {
	LandIds RPCIDList `json:"landIds,omitempty"`
}

// UsrLandClearBatchResponse is the namespace-delta response for gs.usrLand.clearBatch.
type UsrLandClearBatchResponse = RPCResponse[StateDelta]

// ClearBatch calls gs.usrLand.clearBatch. Request fields inferred from game.js: landIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) ClearBatch(ctx context.Context, req UsrLandClearBatchRequest, opts ...RequestOption) (UsrLandClearBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandClearBatch, req, opts...)
}

// UsrLandClearOneKeyRequest is the empty request body for gs.usrLand.clearOneKey.
type UsrLandClearOneKeyRequest struct{}

// UsrLandClearOneKeyResponse is the namespace-delta response for gs.usrLand.clearOneKey.
type UsrLandClearOneKeyResponse = RPCResponse[StateDelta]

// ClearOneKey calls gs.usrLand.clearOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) ClearOneKey(ctx context.Context, req UsrLandClearOneKeyRequest, opts ...RequestOption) (UsrLandClearOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandClearOneKey, req, opts...)
}

// UsrLandHarvestRequest is the request body for gs.usrLand.harvest.
type UsrLandHarvestRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// UsrLandHarvestResponse is the namespace-delta response for gs.usrLand.harvest.
type UsrLandHarvestResponse = RPCResponse[StateDelta]

// Harvest calls gs.usrLand.harvest. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) Harvest(ctx context.Context, req UsrLandHarvestRequest, opts ...RequestOption) (UsrLandHarvestResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandHarvest, req, opts...)
}

// UsrLandHarvestOneKeyRequest is the empty request body for gs.usrLand.harvestOneKey.
type UsrLandHarvestOneKeyRequest struct{}

// UsrLandHarvestOneKeyResponse is the namespace-delta response for gs.usrLand.harvestOneKey.
type UsrLandHarvestOneKeyResponse = RPCResponse[StateDelta]

// HarvestOneKey calls gs.usrLand.harvestOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) HarvestOneKey(ctx context.Context, req UsrLandHarvestOneKeyRequest, opts ...RequestOption) (UsrLandHarvestOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandHarvestOneKey, req, opts...)
}

// UsrLandPlantRequest is the request body for gs.usrLand.plant.
type UsrLandPlantRequest struct {
	LandId   RPCID `json:"landId,omitempty"`
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// UsrLandPlantResponse is the namespace-delta response for gs.usrLand.plant.
type UsrLandPlantResponse = RPCResponse[StateDelta]

// Plant calls gs.usrLand.plant. Request fields inferred from game.js: landId, flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) Plant(ctx context.Context, req UsrLandPlantRequest, opts ...RequestOption) (UsrLandPlantResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandPlant, req, opts...)
}

// UsrLandPlantBatchRequest is the request body for gs.usrLand.plantBatch.
type UsrLandPlantBatchRequest struct {
	LandIds  RPCIDList `json:"landIds,omitempty"`
	FlowerId RPCID     `json:"flowerId,omitempty"`
}

// UsrLandPlantBatchResponse is the namespace-delta response for gs.usrLand.plantBatch.
type UsrLandPlantBatchResponse = RPCResponse[StateDelta]

// PlantBatch calls gs.usrLand.plantBatch. Request fields inferred from game.js: landIds, flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) PlantBatch(ctx context.Context, req UsrLandPlantBatchRequest, opts ...RequestOption) (UsrLandPlantBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandPlantBatch, req, opts...)
}

// UsrLandPlantOneKeyRequest is the request body for gs.usrLand.plantOneKey.
type UsrLandPlantOneKeyRequest struct {
	FlowerId RPCID `json:"flowerId,omitempty"`
}

// UsrLandPlantOneKeyResponse is the namespace-delta response for gs.usrLand.plantOneKey.
type UsrLandPlantOneKeyResponse = RPCResponse[StateDelta]

// PlantOneKey calls gs.usrLand.plantOneKey. Request fields inferred from game.js: flowerId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) PlantOneKey(ctx context.Context, req UsrLandPlantOneKeyRequest, opts ...RequestOption) (UsrLandPlantOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandPlantOneKey, req, opts...)
}

// UsrLandRefreshRequest is the empty request body for gs.usrLand.refresh.
type UsrLandRefreshRequest struct{}

// UsrLandRefreshResponse is the namespace-delta response for gs.usrLand.refresh.
type UsrLandRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.usrLand.refresh. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) Refresh(ctx context.Context, req UsrLandRefreshRequest, opts ...RequestOption) (UsrLandRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandRefresh, req, opts...)
}

// UsrLandSpeedUpRequest is the request body for gs.usrLand.speedUp.
type UsrLandSpeedUpRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// UsrLandSpeedUpResponse is the namespace-delta response for gs.usrLand.speedUp.
type UsrLandSpeedUpResponse = RPCResponse[StateDelta]

// SpeedUp calls gs.usrLand.speedUp. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) SpeedUp(ctx context.Context, req UsrLandSpeedUpRequest, opts ...RequestOption) (UsrLandSpeedUpResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandSpeedUp, req, opts...)
}

// UsrLandSpeedUpBatchRequest is the request body for gs.usrLand.speedUpBatch.
type UsrLandSpeedUpBatchRequest struct {
	LandIds RPCIDList `json:"landIds,omitempty"`
}

// UsrLandSpeedUpBatchResponse is the namespace-delta response for gs.usrLand.speedUpBatch.
type UsrLandSpeedUpBatchResponse = RPCResponse[StateDelta]

// SpeedUpBatch calls gs.usrLand.speedUpBatch. Request fields inferred from game.js: landIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) SpeedUpBatch(ctx context.Context, req UsrLandSpeedUpBatchRequest, opts ...RequestOption) (UsrLandSpeedUpBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandSpeedUpBatch, req, opts...)
}

// UsrLandSpeedUpFreeRequest is the empty request body for gs.usrLand.speedUpFree.
type UsrLandSpeedUpFreeRequest struct{}

// UsrLandSpeedUpFreeResponse is the namespace-delta response for gs.usrLand.speedUpFree.
type UsrLandSpeedUpFreeResponse = RPCResponse[StateDelta]

// SpeedUpFree calls gs.usrLand.speedUpFree. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) SpeedUpFree(ctx context.Context, req UsrLandSpeedUpFreeRequest, opts ...RequestOption) (UsrLandSpeedUpFreeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandSpeedUpFree, req, opts...)
}

// UsrLandSpeedUpOneKeyRequest is the empty request body for gs.usrLand.speedUpOneKey.
type UsrLandSpeedUpOneKeyRequest struct{}

// UsrLandSpeedUpOneKeyResponse is the namespace-delta response for gs.usrLand.speedUpOneKey.
type UsrLandSpeedUpOneKeyResponse = RPCResponse[StateDelta]

// SpeedUpOneKey calls gs.usrLand.speedUpOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) SpeedUpOneKey(ctx context.Context, req UsrLandSpeedUpOneKeyRequest, opts ...RequestOption) (UsrLandSpeedUpOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandSpeedUpOneKey, req, opts...)
}

// UsrLandUnlockLandRequest is the request body for gs.usrLand.unlockLand.
type UsrLandUnlockLandRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// UsrLandUnlockLandResponse is the namespace-delta response for gs.usrLand.unlockLand.
type UsrLandUnlockLandResponse = RPCResponse[StateDelta]

// UnlockLand calls gs.usrLand.unlockLand. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) UnlockLand(ctx context.Context, req UsrLandUnlockLandRequest, opts ...RequestOption) (UsrLandUnlockLandResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandUnlockLand, req, opts...)
}

// UsrLandWaterRequest is the request body for gs.usrLand.water.
type UsrLandWaterRequest struct {
	LandId RPCID `json:"landId,omitempty"`
}

// UsrLandWaterResponse is the namespace-delta response for gs.usrLand.water.
type UsrLandWaterResponse = RPCResponse[StateDelta]

// Water calls gs.usrLand.water. Request fields inferred from game.js: landId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) Water(ctx context.Context, req UsrLandWaterRequest, opts ...RequestOption) (UsrLandWaterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandWater, req, opts...)
}

// UsrLandWaterBatchRequest is the request body for gs.usrLand.waterBatch.
type UsrLandWaterBatchRequest struct {
	LandIds RPCIDList `json:"landIds,omitempty"`
}

// UsrLandWaterBatchResponse is the namespace-delta response for gs.usrLand.waterBatch.
type UsrLandWaterBatchResponse = RPCResponse[StateDelta]

// WaterBatch calls gs.usrLand.waterBatch. Request fields inferred from game.js: landIds.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) WaterBatch(ctx context.Context, req UsrLandWaterBatchRequest, opts ...RequestOption) (UsrLandWaterBatchResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandWaterBatch, req, opts...)
}

// UsrLandWaterOneKeyRequest is the empty request body for gs.usrLand.waterOneKey.
type UsrLandWaterOneKeyRequest struct{}

// UsrLandWaterOneKeyResponse is the namespace-delta response for gs.usrLand.waterOneKey.
type UsrLandWaterOneKeyResponse = RPCResponse[StateDelta]

// WaterOneKey calls gs.usrLand.waterOneKey. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrLandRPC) WaterOneKey(ctx context.Context, req UsrLandWaterOneKeyRequest, opts ...RequestOption) (UsrLandWaterOneKeyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrLandWaterOneKey, req, opts...)
}

// UsrRed returns typed RPC helpers for the usrRed namespace.
func (c *RPCClient) UsrRed() UsrRedRPC { return UsrRedRPC{c: c} }

type UsrRedRPC struct{ c *RPCClient }

// UsrRedDelRedRequest carries JSON fields for gs.usrRed.delRed; game.js did not expose a stable object literal for this request.
type UsrRedDelRedRequest RawRequest

// UsrRedDelRedResponse is the namespace-delta response for gs.usrRed.delRed.
type UsrRedDelRedResponse = RPCResponse[StateDelta]

// DelRed calls gs.usrRed.delRed. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrRedRPC) DelRed(ctx context.Context, req UsrRedDelRedRequest, opts ...RequestOption) (UsrRedDelRedResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrRedDelRed, req, opts...)
}

// UsrSubscribePush returns typed RPC helpers for the usrSubscribePush namespace.
func (c *RPCClient) UsrSubscribePush() UsrSubscribePushRPC { return UsrSubscribePushRPC{c: c} }

type UsrSubscribePushRPC struct{ c *RPCClient }

// UsrSubscribePushAddSubscribeNumRequest is the request body for gs.usrSubscribePush.addSubscribeNum.
type UsrSubscribePushAddSubscribeNumRequest struct {
	TypeList RPCIDList `json:"typeList,omitempty"`
}

// UsrSubscribePushAddSubscribeNumResponse is the namespace-delta response for gs.usrSubscribePush.addSubscribeNum.
type UsrSubscribePushAddSubscribeNumResponse = RPCResponse[StateDelta]

// AddSubscribeNum calls gs.usrSubscribePush.addSubscribeNum. Request fields inferred from game.js: typeList.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrSubscribePushRPC) AddSubscribeNum(ctx context.Context, req UsrSubscribePushAddSubscribeNumRequest, opts ...RequestOption) (UsrSubscribePushAddSubscribeNumResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSubscribePushAddSubscribeNum, req, opts...)
}

// UsrSubscribePushAddSubscribeNumPermanentRequest is the request body for gs.usrSubscribePush.addSubscribeNumPermanent.
type UsrSubscribePushAddSubscribeNumPermanentRequest struct {
	TypeList RPCIDList `json:"typeList,omitempty"`
}

// UsrSubscribePushAddSubscribeNumPermanentResponse is the namespace-delta response for gs.usrSubscribePush.addSubscribeNumPermanent.
type UsrSubscribePushAddSubscribeNumPermanentResponse = RPCResponse[StateDelta]

// AddSubscribeNumPermanent calls gs.usrSubscribePush.addSubscribeNumPermanent. Request fields inferred from game.js: typeList.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrSubscribePushRPC) AddSubscribeNumPermanent(ctx context.Context, req UsrSubscribePushAddSubscribeNumPermanentRequest, opts ...RequestOption) (UsrSubscribePushAddSubscribeNumPermanentResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSubscribePushAddSubscribeNumPermanent, req, opts...)
}

// UsrSubscribePushMsgPushSettingRequest is the request body for gs.usrSubscribePush.msgPushSetting.
type UsrSubscribePushMsgPushSettingRequest struct {
	SettingMap RPCObject `json:"settingMap,omitempty"`
}

// UsrSubscribePushMsgPushSettingResponse is the namespace-delta response for gs.usrSubscribePush.msgPushSetting.
type UsrSubscribePushMsgPushSettingResponse = RPCResponse[StateDelta]

// MsgPushSetting calls gs.usrSubscribePush.msgPushSetting. Request fields inferred from game.js: settingMap.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrSubscribePushRPC) MsgPushSetting(ctx context.Context, req UsrSubscribePushMsgPushSettingRequest, opts ...RequestOption) (UsrSubscribePushMsgPushSettingResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSubscribePushMsgPushSetting, req, opts...)
}

// UsrSubscribePushMsgPushSettingGlobalRequest is the request body for gs.usrSubscribePush.msgPushSettingGlobal.
type UsrSubscribePushMsgPushSettingGlobalRequest struct {
	IsOpen          RPCBool `json:"isOpen,omitempty"`
	IsSubscribeOpen RPCBool `json:"isSubscribeOpen,omitempty"`
}

// UsrSubscribePushMsgPushSettingGlobalResponse is the namespace-delta response for gs.usrSubscribePush.msgPushSettingGlobal.
type UsrSubscribePushMsgPushSettingGlobalResponse = RPCResponse[StateDelta]

// MsgPushSettingGlobal calls gs.usrSubscribePush.msgPushSettingGlobal. Request fields inferred from game.js: isOpen, isSubscribeOpen.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrSubscribePushRPC) MsgPushSettingGlobal(ctx context.Context, req UsrSubscribePushMsgPushSettingGlobalRequest, opts ...RequestOption) (UsrSubscribePushMsgPushSettingGlobalResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrSubscribePushMsgPushSettingGlobal, req, opts...)
}

// UsrVerInfo returns typed RPC helpers for the usrVerInfo namespace.
func (c *RPCClient) UsrVerInfo() UsrVerInfoRPC { return UsrVerInfoRPC{c: c} }

type UsrVerInfoRPC struct{ c *RPCClient }

// UsrVerInfoRefreshRequest is the request body for gs.usrVerInfo.refresh.
type UsrVerInfoRefreshRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// UsrVerInfoRefreshResponse is the namespace-delta response for gs.usrVerInfo.refresh.
type UsrVerInfoRefreshResponse = RPCResponse[StateDelta]

// Refresh calls gs.usrVerInfo.refresh. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r UsrVerInfoRPC) Refresh(ctx context.Context, req UsrVerInfoRefreshRequest, opts ...RequestOption) (UsrVerInfoRefreshResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCUsrVerInfoRefresh, req, opts...)
}

// Valentines returns typed RPC helpers for the valentines namespace.
func (c *RPCClient) Valentines() ValentinesRPC { return ValentinesRPC{c: c} }

type ValentinesRPC struct{ c *RPCClient }

// ValentinesApplyRequest is the request body for gs.valentines.apply.
type ValentinesApplyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// ValentinesApplyResponse is the namespace-delta response for gs.valentines.apply.
type ValentinesApplyResponse = RPCResponse[StateDelta]

// Apply calls gs.valentines.apply. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) Apply(ctx context.Context, req ValentinesApplyRequest, opts ...RequestOption) (ValentinesApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesApply, req, opts...)
}

// ValentinesBindRequest is the request body for gs.valentines.bind.
type ValentinesBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// ValentinesBindResponse is the namespace-delta response for gs.valentines.bind.
type ValentinesBindResponse = RPCResponse[StateDelta]

// Bind calls gs.valentines.bind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) Bind(ctx context.Context, req ValentinesBindRequest, opts ...RequestOption) (ValentinesBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesBind, req, opts...)
}

// ValentinesEnterRequest is the request body for gs.valentines.enter.
type ValentinesEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ValentinesEnterResponse is the namespace-delta response for gs.valentines.enter.
type ValentinesEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.valentines.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) Enter(ctx context.Context, req ValentinesEnterRequest, opts ...RequestOption) (ValentinesEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesEnter, req, opts...)
}

// ValentinesFrdStatesRequest is the request body for gs.valentines.frdStates.
type ValentinesFrdStatesRequest struct {
	BatchId RPCID      `json:"batchId,omitempty"`
	UIDs    RPCUIDList `json:"uids,omitempty"`
}

// ValentinesFrdStatesResponse is the namespace-delta response for gs.valentines.frdStates.
type ValentinesFrdStatesResponse = RPCResponse[StateDelta]

// FrdStates calls gs.valentines.frdStates. Request fields inferred from game.js: batchId, uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) FrdStates(ctx context.Context, req ValentinesFrdStatesRequest, opts ...RequestOption) (ValentinesFrdStatesResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesFrdStates, req, opts...)
}

// ValentinesRecvRequest is the request body for gs.valentines.recv.
type ValentinesRecvRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// ValentinesRecvResponse is the namespace-delta response for gs.valentines.recv.
type ValentinesRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.valentines.recv. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) Recv(ctx context.Context, req ValentinesRecvRequest, opts ...RequestOption) (ValentinesRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesRecv, req, opts...)
}

// ValentinesRejectRequest is the request body for gs.valentines.reject.
type ValentinesRejectRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// ValentinesRejectResponse is the namespace-delta response for gs.valentines.reject.
type ValentinesRejectResponse = RPCResponse[StateDelta]

// Reject calls gs.valentines.reject. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) Reject(ctx context.Context, req ValentinesRejectRequest, opts ...RequestOption) (ValentinesRejectResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesReject, req, opts...)
}

// ValentinesUnBindRequest is the request body for gs.valentines.unBind.
type ValentinesUnBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// ValentinesUnBindResponse is the namespace-delta response for gs.valentines.unBind.
type ValentinesUnBindResponse = RPCResponse[StateDelta]

// UnBind calls gs.valentines.unBind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ValentinesRPC) UnBind(ctx context.Context, req ValentinesUnBindRequest, opts ...RequestOption) (ValentinesUnBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCValentinesUnBind, req, opts...)
}

// Verify returns typed RPC helpers for the verify namespace.
func (c *RPCClient) Verify() VerifyRPC { return VerifyRPC{c: c} }

type VerifyRPC struct{ c *RPCClient }

// VerifyCheckVerificationRequest is the request body for gs.verify.checkVerification.
type VerifyCheckVerificationRequest struct {
	Type      RPCInt    `json:"type,omitempty"`
	RequestId RPCID     `json:"requestId,omitempty"`
	Code      RPCString `json:"code,omitempty"`
}

// VerifyCheckVerificationResponse is the namespace-delta response for gs.verify.checkVerification.
type VerifyCheckVerificationResponse = RPCResponse[StateDelta]

// CheckVerification calls gs.verify.checkVerification. Request fields inferred from game.js: type, requestId, code.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r VerifyRPC) CheckVerification(ctx context.Context, req VerifyCheckVerificationRequest, opts ...RequestOption) (VerifyCheckVerificationResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCVerifyCheckVerification, req, opts...)
}

// VerifyRefreshVerificationRequest is the request body for gs.verify.refreshVerification.
type VerifyRefreshVerificationRequest struct {
	Type     RPCInt  `json:"type,omitempty"`
	IsManual RPCBool `json:"isManual,omitempty"`
}

// VerifyRefreshVerificationResponse is the namespace-delta response for gs.verify.refreshVerification.
type VerifyRefreshVerificationResponse = RPCResponse[StateDelta]

// RefreshVerification calls gs.verify.refreshVerification. Request fields inferred from game.js: type, isManual.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r VerifyRPC) RefreshVerification(ctx context.Context, req VerifyRefreshVerificationRequest, opts ...RequestOption) (VerifyRefreshVerificationResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCVerifyRefreshVerification, req, opts...)
}

// Vip returns typed RPC helpers for the vip namespace.
func (c *RPCClient) Vip() VipRPC { return VipRPC{c: c} }

type VipRPC struct{ c *RPCClient }

// VipRecvRequest is the request body for gs.vip.recv.
type VipRecvRequest struct {
	Vip RPCID `json:"vip,omitempty"`
}

// VipRecvResponse is the namespace-delta response for gs.vip.recv.
type VipRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.vip.recv. Request fields inferred from game.js: vip.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r VipRPC) Recv(ctx context.Context, req VipRecvRequest, opts ...RequestOption) (VipRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCVipRecv, req, opts...)
}

// WaterRqst returns typed RPC helpers for the waterRqst namespace.
func (c *RPCClient) WaterRqst() WaterRqstRPC { return WaterRqstRPC{c: c} }

type WaterRqstRPC struct{ c *RPCClient }

// WaterRqstDjstRequest is the request body for gs.waterRqst.djst.
type WaterRqstDjstRequest struct {
	Point RPCPoint `json:"point,omitempty"`
}

// WaterRqstDjstResponse is the namespace-delta response for gs.waterRqst.djst.
type WaterRqstDjstResponse = RPCResponse[StateDelta]

// Djst calls gs.waterRqst.djst. Request fields inferred from game.js: point.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WaterRqstRPC) Djst(ctx context.Context, req WaterRqstDjstRequest, opts ...RequestOption) (WaterRqstDjstResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWaterRqstDjst, req, opts...)
}

// Waterwheel returns typed RPC helpers for the waterwheel namespace.
func (c *RPCClient) Waterwheel() WaterwheelRPC { return WaterwheelRPC{c: c} }

type WaterwheelRPC struct{ c *RPCClient }

// WaterwheelEnterRequest is the empty request body for gs.waterwheel.enter.
type WaterwheelEnterRequest struct{}

// WaterwheelEnterResponse is the namespace-delta response for gs.waterwheel.enter.
type WaterwheelEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.waterwheel.enter. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WaterwheelRPC) Enter(ctx context.Context, req WaterwheelEnterRequest, opts ...RequestOption) (WaterwheelEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWaterwheelEnter, req, opts...)
}

// WaterwheelRecvRequest is the empty request body for gs.waterwheel.recv.
type WaterwheelRecvRequest struct{}

// WaterwheelRecvResponse is the namespace-delta response for gs.waterwheel.recv.
type WaterwheelRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.waterwheel.recv. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WaterwheelRPC) Recv(ctx context.Context, req WaterwheelRecvRequest, opts ...RequestOption) (WaterwheelRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWaterwheelRecv, req, opts...)
}

// WaterwheelSkipRequest is the empty request body for gs.waterwheel.skip.
type WaterwheelSkipRequest struct{}

// WaterwheelSkipResponse is the namespace-delta response for gs.waterwheel.skip.
type WaterwheelSkipResponse = RPCResponse[StateDelta]

// Skip calls gs.waterwheel.skip. game.js sends an empty request object.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WaterwheelRPC) Skip(ctx context.Context, req WaterwheelSkipRequest, opts ...RequestOption) (WaterwheelSkipResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWaterwheelSkip, req, opts...)
}

// WhiteDay26 returns typed RPC helpers for the whiteDay26 namespace.
func (c *RPCClient) WhiteDay26() WhiteDay26RPC { return WhiteDay26RPC{c: c} }

type WhiteDay26RPC struct{ c *RPCClient }

// WhiteDay26ApplyRequest is the request body for gs.whiteDay26.apply.
type WhiteDay26ApplyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteDay26ApplyResponse is the namespace-delta response for gs.whiteDay26.apply.
type WhiteDay26ApplyResponse = RPCResponse[StateDelta]

// Apply calls gs.whiteDay26.apply. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) Apply(ctx context.Context, req WhiteDay26ApplyRequest, opts ...RequestOption) (WhiteDay26ApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26Apply, req, opts...)
}

// WhiteDay26BindRequest is the request body for gs.whiteDay26.bind.
type WhiteDay26BindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteDay26BindResponse is the namespace-delta response for gs.whiteDay26.bind.
type WhiteDay26BindResponse = RPCResponse[StateDelta]

// Bind calls gs.whiteDay26.bind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) Bind(ctx context.Context, req WhiteDay26BindRequest, opts ...RequestOption) (WhiteDay26BindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26Bind, req, opts...)
}

// WhiteDay26EnterRequest is the request body for gs.whiteDay26.enter.
type WhiteDay26EnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// WhiteDay26EnterResponse is the namespace-delta response for gs.whiteDay26.enter.
type WhiteDay26EnterResponse = RPCResponse[StateDelta]

// Enter calls gs.whiteDay26.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) Enter(ctx context.Context, req WhiteDay26EnterRequest, opts ...RequestOption) (WhiteDay26EnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26Enter, req, opts...)
}

// WhiteDay26FrdStatesRequest is the request body for gs.whiteDay26.frdStates.
type WhiteDay26FrdStatesRequest struct {
	BatchId RPCID      `json:"batchId,omitempty"`
	UIDs    RPCUIDList `json:"uids,omitempty"`
}

// WhiteDay26FrdStatesResponse is the namespace-delta response for gs.whiteDay26.frdStates.
type WhiteDay26FrdStatesResponse = RPCResponse[StateDelta]

// FrdStates calls gs.whiteDay26.frdStates. Request fields inferred from game.js: batchId, uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) FrdStates(ctx context.Context, req WhiteDay26FrdStatesRequest, opts ...RequestOption) (WhiteDay26FrdStatesResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26FrdStates, req, opts...)
}

// WhiteDay26RecvRequest is the request body for gs.whiteDay26.recv.
type WhiteDay26RecvRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// WhiteDay26RecvResponse is the namespace-delta response for gs.whiteDay26.recv.
type WhiteDay26RecvResponse = RPCResponse[StateDelta]

// Recv calls gs.whiteDay26.recv. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) Recv(ctx context.Context, req WhiteDay26RecvRequest, opts ...RequestOption) (WhiteDay26RecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26Recv, req, opts...)
}

// WhiteDay26RejectRequest is the request body for gs.whiteDay26.reject.
type WhiteDay26RejectRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteDay26RejectResponse is the namespace-delta response for gs.whiteDay26.reject.
type WhiteDay26RejectResponse = RPCResponse[StateDelta]

// Reject calls gs.whiteDay26.reject. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) Reject(ctx context.Context, req WhiteDay26RejectRequest, opts ...RequestOption) (WhiteDay26RejectResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26Reject, req, opts...)
}

// WhiteDay26UnBindRequest is the request body for gs.whiteDay26.unBind.
type WhiteDay26UnBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteDay26UnBindResponse is the namespace-delta response for gs.whiteDay26.unBind.
type WhiteDay26UnBindResponse = RPCResponse[StateDelta]

// UnBind calls gs.whiteDay26.unBind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteDay26RPC) UnBind(ctx context.Context, req WhiteDay26UnBindRequest, opts ...RequestOption) (WhiteDay26UnBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteDay26UnBind, req, opts...)
}

// WhiteValentine returns typed RPC helpers for the whiteValentine namespace.
func (c *RPCClient) WhiteValentine() WhiteValentineRPC { return WhiteValentineRPC{c: c} }

type WhiteValentineRPC struct{ c *RPCClient }

// WhiteValentineApplyRequest is the request body for gs.whiteValentine.apply.
type WhiteValentineApplyRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteValentineApplyResponse is the namespace-delta response for gs.whiteValentine.apply.
type WhiteValentineApplyResponse = RPCResponse[StateDelta]

// Apply calls gs.whiteValentine.apply. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) Apply(ctx context.Context, req WhiteValentineApplyRequest, opts ...RequestOption) (WhiteValentineApplyResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineApply, req, opts...)
}

// WhiteValentineBindRequest is the request body for gs.whiteValentine.bind.
type WhiteValentineBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteValentineBindResponse is the namespace-delta response for gs.whiteValentine.bind.
type WhiteValentineBindResponse = RPCResponse[StateDelta]

// Bind calls gs.whiteValentine.bind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) Bind(ctx context.Context, req WhiteValentineBindRequest, opts ...RequestOption) (WhiteValentineBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineBind, req, opts...)
}

// WhiteValentineEnterRequest is the request body for gs.whiteValentine.enter.
type WhiteValentineEnterRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// WhiteValentineEnterResponse is the namespace-delta response for gs.whiteValentine.enter.
type WhiteValentineEnterResponse = RPCResponse[StateDelta]

// Enter calls gs.whiteValentine.enter. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) Enter(ctx context.Context, req WhiteValentineEnterRequest, opts ...RequestOption) (WhiteValentineEnterResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineEnter, req, opts...)
}

// WhiteValentineFrdStatesRequest is the request body for gs.whiteValentine.frdStates.
type WhiteValentineFrdStatesRequest struct {
	BatchId RPCID      `json:"batchId,omitempty"`
	UIDs    RPCUIDList `json:"uids,omitempty"`
}

// WhiteValentineFrdStatesResponse is the namespace-delta response for gs.whiteValentine.frdStates.
type WhiteValentineFrdStatesResponse = RPCResponse[StateDelta]

// FrdStates calls gs.whiteValentine.frdStates. Request fields inferred from game.js: batchId, uids.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) FrdStates(ctx context.Context, req WhiteValentineFrdStatesRequest, opts ...RequestOption) (WhiteValentineFrdStatesResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineFrdStates, req, opts...)
}

// WhiteValentineRecvRequest is the request body for gs.whiteValentine.recv.
type WhiteValentineRecvRequest struct {
	BatchId RPCID `json:"batchId,omitempty"`
}

// WhiteValentineRecvResponse is the namespace-delta response for gs.whiteValentine.recv.
type WhiteValentineRecvResponse = RPCResponse[StateDelta]

// Recv calls gs.whiteValentine.recv. Request fields inferred from game.js: batchId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) Recv(ctx context.Context, req WhiteValentineRecvRequest, opts ...RequestOption) (WhiteValentineRecvResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineRecv, req, opts...)
}

// WhiteValentineRejectRequest is the request body for gs.whiteValentine.reject.
type WhiteValentineRejectRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteValentineRejectResponse is the namespace-delta response for gs.whiteValentine.reject.
type WhiteValentineRejectResponse = RPCResponse[StateDelta]

// Reject calls gs.whiteValentine.reject. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) Reject(ctx context.Context, req WhiteValentineRejectRequest, opts ...RequestOption) (WhiteValentineRejectResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineReject, req, opts...)
}

// WhiteValentineUnBindRequest is the request body for gs.whiteValentine.unBind.
type WhiteValentineUnBindRequest struct {
	BatchId RPCID  `json:"batchId,omitempty"`
	DUID    RPCUID `json:"dUid,omitempty"`
}

// WhiteValentineUnBindResponse is the namespace-delta response for gs.whiteValentine.unBind.
type WhiteValentineUnBindResponse = RPCResponse[StateDelta]

// UnBind calls gs.whiteValentine.unBind. Request fields inferred from game.js: batchId, dUid.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r WhiteValentineRPC) UnBind(ctx context.Context, req WhiteValentineUnBindRequest, opts ...RequestOption) (WhiteValentineUnBindResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCWhiteValentineUnBind, req, opts...)
}

// Zoo returns typed RPC helpers for the zoo namespace.
func (c *RPCClient) Zoo() ZooRPC { return ZooRPC{c: c} }

type ZooRPC struct{ c *RPCClient }

// ZooAddFoodstuffRequest carries JSON fields for gs.zoo.addFoodstuff; game.js did not expose a stable object literal for this request.
type ZooAddFoodstuffRequest RawRequest

// ZooAddFoodstuffResponse is the namespace-delta response for gs.zoo.addFoodstuff.
type ZooAddFoodstuffResponse = RPCResponse[StateDelta]

// AddFoodstuff calls gs.zoo.addFoodstuff. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) AddFoodstuff(ctx context.Context, req ZooAddFoodstuffRequest, opts ...RequestOption) (ZooAddFoodstuffResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooAddFoodstuff, req, opts...)
}

// ZooCalNaturalAttRequest carries JSON fields for gs.zoo.calNaturalAtt; game.js did not expose a stable object literal for this request.
type ZooCalNaturalAttRequest RawRequest

// ZooCalNaturalAttResponse is the namespace-delta response for gs.zoo.calNaturalAtt.
type ZooCalNaturalAttResponse = RPCResponse[StateDelta]

// CalNaturalAtt calls gs.zoo.calNaturalAtt. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) CalNaturalAtt(ctx context.Context, req ZooCalNaturalAttRequest, opts ...RequestOption) (ZooCalNaturalAttResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooCalNaturalAtt, req, opts...)
}

// ZooChangePetNameRequest carries JSON fields for gs.zoo.changePetName; game.js did not expose a stable object literal for this request.
type ZooChangePetNameRequest RawRequest

// ZooChangePetNameResponse is the namespace-delta response for gs.zoo.changePetName.
type ZooChangePetNameResponse = RPCResponse[StateDelta]

// ChangePetName calls gs.zoo.changePetName. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) ChangePetName(ctx context.Context, req ZooChangePetNameRequest, opts ...RequestOption) (ZooChangePetNameResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooChangePetName, req, opts...)
}

// ZooEnterZooRequest carries JSON fields for gs.zoo.enterZoo; game.js did not expose a stable object literal for this request.
type ZooEnterZooRequest RawRequest

// ZooEnterZooResponse is the namespace-delta response for gs.zoo.enterZoo.
type ZooEnterZooResponse = RPCResponse[StateDelta]

// EnterZoo calls gs.zoo.enterZoo. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) EnterZoo(ctx context.Context, req ZooEnterZooRequest, opts ...RequestOption) (ZooEnterZooResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooEnterZoo, req, opts...)
}

// ZooFeedOtherPetRequest carries JSON fields for gs.zoo.feedOtherPet; game.js did not expose a stable object literal for this request.
type ZooFeedOtherPetRequest RawRequest

// ZooFeedOtherPetResponse is the namespace-delta response for gs.zoo.feedOtherPet.
type ZooFeedOtherPetResponse = RPCResponse[StateDelta]

// FeedOtherPet calls gs.zoo.feedOtherPet. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) FeedOtherPet(ctx context.Context, req ZooFeedOtherPetRequest, opts ...RequestOption) (ZooFeedOtherPetResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooFeedOtherPet, req, opts...)
}

// ZooFeedPetsRequest carries JSON fields for gs.zoo.feedPets; game.js did not expose a stable object literal for this request.
type ZooFeedPetsRequest RawRequest

// ZooFeedPetsResponse is the namespace-delta response for gs.zoo.feedPets.
type ZooFeedPetsResponse = RPCResponse[StateDelta]

// FeedPets calls gs.zoo.feedPets. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) FeedPets(ctx context.Context, req ZooFeedPetsRequest, opts ...RequestOption) (ZooFeedPetsResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooFeedPets, req, opts...)
}

// ZooFindPetRequest carries JSON fields for gs.zoo.findPet; game.js did not expose a stable object literal for this request.
type ZooFindPetRequest RawRequest

// ZooFindPetResponse is the namespace-delta response for gs.zoo.findPet.
type ZooFindPetResponse = RPCResponse[StateDelta]

// FindPet calls gs.zoo.findPet. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) FindPet(ctx context.Context, req ZooFindPetRequest, opts ...RequestOption) (ZooFindPetResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooFindPet, req, opts...)
}

// ZooFindPetByUsrBackRequest carries JSON fields for gs.zoo.findPetByUsrBack; game.js did not expose a stable object literal for this request.
type ZooFindPetByUsrBackRequest RawRequest

// ZooFindPetByUsrBackResponse is the namespace-delta response for gs.zoo.findPetByUsrBack.
type ZooFindPetByUsrBackResponse = RPCResponse[StateDelta]

// FindPetByUsrBack calls gs.zoo.findPetByUsrBack. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) FindPetByUsrBack(ctx context.Context, req ZooFindPetByUsrBackRequest, opts ...RequestOption) (ZooFindPetByUsrBackResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooFindPetByUsrBack, req, opts...)
}

// ZooGetGuideEventRwdRequest carries JSON fields for gs.zoo.getGuideEventRwd; game.js did not expose a stable object literal for this request.
type ZooGetGuideEventRwdRequest RawRequest

// ZooGetGuideEventRwdResponse is the namespace-delta response for gs.zoo.getGuideEventRwd.
type ZooGetGuideEventRwdResponse = RPCResponse[StateDelta]

// GetGuideEventRwd calls gs.zoo.getGuideEventRwd. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) GetGuideEventRwd(ctx context.Context, req ZooGetGuideEventRwdRequest, opts ...RequestOption) (ZooGetGuideEventRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooGetGuideEventRwd, req, opts...)
}

// ZooHandBeOverdueEventRequest carries JSON fields for gs.zoo.handBeOverdueEvent; game.js did not expose a stable object literal for this request.
type ZooHandBeOverdueEventRequest RawRequest

// ZooHandBeOverdueEventResponse is the namespace-delta response for gs.zoo.handBeOverdueEvent.
type ZooHandBeOverdueEventResponse = RPCResponse[StateDelta]

// HandBeOverdueEvent calls gs.zoo.handBeOverdueEvent. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) HandBeOverdueEvent(ctx context.Context, req ZooHandBeOverdueEventRequest, opts ...RequestOption) (ZooHandBeOverdueEventResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooHandBeOverdueEvent, req, opts...)
}

// ZooHandleEventRequest carries JSON fields for gs.zoo.handleEvent; game.js did not expose a stable object literal for this request.
type ZooHandleEventRequest RawRequest

// ZooHandleEventResponse is the namespace-delta response for gs.zoo.handleEvent.
type ZooHandleEventResponse = RPCResponse[StateDelta]

// HandleEvent calls gs.zoo.handleEvent. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) HandleEvent(ctx context.Context, req ZooHandleEventRequest, opts ...RequestOption) (ZooHandleEventResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooHandleEvent, req, opts...)
}

// ZooInitZooRequest carries JSON fields for gs.zoo.initZoo; game.js did not expose a stable object literal for this request.
type ZooInitZooRequest RawRequest

// ZooInitZooResponse is the namespace-delta response for gs.zoo.initZoo.
type ZooInitZooResponse = RPCResponse[StateDelta]

// InitZoo calls gs.zoo.initZoo. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) InitZoo(ctx context.Context, req ZooInitZooRequest, opts ...RequestOption) (ZooInitZooResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooInitZoo, req, opts...)
}

// ZooReadLogRequest is the request body for gs.zoo.readLog.
type ZooReadLogRequest struct {
	PetId RPCID `json:"petId,omitempty"`
}

// ZooReadLogResponse is the namespace-delta response for gs.zoo.readLog.
type ZooReadLogResponse = RPCResponse[StateDelta]

// ReadLog calls gs.zoo.readLog. Request fields inferred from game.js: petId.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) ReadLog(ctx context.Context, req ZooReadLogRequest, opts ...RequestOption) (ZooReadLogResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooReadLog, req, opts...)
}

// ZooReadSouvenirRequest carries JSON fields for gs.zoo.readSouvenir; game.js did not expose a stable object literal for this request.
type ZooReadSouvenirRequest RawRequest

// ZooReadSouvenirResponse is the namespace-delta response for gs.zoo.readSouvenir.
type ZooReadSouvenirResponse = RPCResponse[StateDelta]

// ReadSouvenir calls gs.zoo.readSouvenir. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) ReadSouvenir(ctx context.Context, req ZooReadSouvenirRequest, opts ...RequestOption) (ZooReadSouvenirResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooReadSouvenir, req, opts...)
}

// ZooRecvSouvenirRwdRequest carries JSON fields for gs.zoo.recvSouvenirRwd; game.js did not expose a stable object literal for this request.
type ZooRecvSouvenirRwdRequest RawRequest

// ZooRecvSouvenirRwdResponse is the namespace-delta response for gs.zoo.recvSouvenirRwd.
type ZooRecvSouvenirRwdResponse = RPCResponse[StateDelta]

// RecvSouvenirRwd calls gs.zoo.recvSouvenirRwd. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) RecvSouvenirRwd(ctx context.Context, req ZooRecvSouvenirRwdRequest, opts ...RequestOption) (ZooRecvSouvenirRwdResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooRecvSouvenirRwd, req, opts...)
}

// ZooRefreshPetStatusRequest carries JSON fields for gs.zoo.refreshPetStatus; game.js did not expose a stable object literal for this request.
type ZooRefreshPetStatusRequest RawRequest

// ZooRefreshPetStatusResponse is the namespace-delta response for gs.zoo.refreshPetStatus.
type ZooRefreshPetStatusResponse = RPCResponse[StateDelta]

// RefreshPetStatus calls gs.zoo.refreshPetStatus. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) RefreshPetStatus(ctx context.Context, req ZooRefreshPetStatusRequest, opts ...RequestOption) (ZooRefreshPetStatusResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooRefreshPetStatus, req, opts...)
}

// ZooSetUpSleepTimeRequest carries JSON fields for gs.zoo.setUpSleepTime; game.js did not expose a stable object literal for this request.
type ZooSetUpSleepTimeRequest RawRequest

// ZooSetUpSleepTimeResponse is the namespace-delta response for gs.zoo.setUpSleepTime.
type ZooSetUpSleepTimeResponse = RPCResponse[StateDelta]

// SetUpSleepTime calls gs.zoo.setUpSleepTime. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) SetUpSleepTime(ctx context.Context, req ZooSetUpSleepTimeRequest, opts ...RequestOption) (ZooSetUpSleepTimeResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooSetUpSleepTime, req, opts...)
}

// ZooStrokePetRequest carries JSON fields for gs.zoo.strokePet; game.js did not expose a stable object literal for this request.
type ZooStrokePetRequest RawRequest

// ZooStrokePetResponse is the namespace-delta response for gs.zoo.strokePet.
type ZooStrokePetResponse = RPCResponse[StateDelta]

// StrokePet calls gs.zoo.strokePet. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) StrokePet(ctx context.Context, req ZooStrokePetRequest, opts ...RequestOption) (ZooStrokePetResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooStrokePet, req, opts...)
}

// ZooUsePetItemRequest carries JSON fields for gs.zoo.usePetItem; game.js did not expose a stable object literal for this request.
type ZooUsePetItemRequest RawRequest

// ZooUsePetItemResponse is the namespace-delta response for gs.zoo.usePetItem.
type ZooUsePetItemResponse = RPCResponse[StateDelta]

// UsePetItem calls gs.zoo.usePetItem. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) UsePetItem(ctx context.Context, req ZooUsePetItemRequest, opts ...RequestOption) (ZooUsePetItemResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooUsePetItem, req, opts...)
}

// ZooVisitZooRequest carries JSON fields for gs.zoo.visitZoo; game.js did not expose a stable object literal for this request.
type ZooVisitZooRequest RawRequest

// ZooVisitZooResponse is the namespace-delta response for gs.zoo.visitZoo.
type ZooVisitZooResponse = RPCResponse[StateDelta]

// VisitZoo calls gs.zoo.visitZoo. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooRPC) VisitZoo(ctx context.Context, req ZooVisitZooRequest, opts ...RequestOption) (ZooVisitZooResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooVisitZoo, req, opts...)
}

// ZooDecorate returns typed RPC helpers for the zooDecorate namespace.
func (c *RPCClient) ZooDecorate() ZooDecorateRPC { return ZooDecorateRPC{c: c} }

type ZooDecorateRPC struct{ c *RPCClient }

// ZooDecorateEquipRequest carries JSON fields for gs.zooDecorate.equip; game.js did not expose a stable object literal for this request.
type ZooDecorateEquipRequest RawRequest

// ZooDecorateEquipResponse is the namespace-delta response for gs.zooDecorate.equip.
type ZooDecorateEquipResponse = RPCResponse[StateDelta]

// Equip calls gs.zooDecorate.equip. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooDecorateRPC) Equip(ctx context.Context, req ZooDecorateEquipRequest, opts ...RequestOption) (ZooDecorateEquipResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooDecorateEquip, req, opts...)
}

// ZooDecorateReadRequest carries JSON fields for gs.zooDecorate.read; game.js did not expose a stable object literal for this request.
type ZooDecorateReadRequest RawRequest

// ZooDecorateReadResponse is the namespace-delta response for gs.zooDecorate.read.
type ZooDecorateReadResponse = RPCResponse[StateDelta]

// Read calls gs.zooDecorate.read. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.
// The response payload is decoded as StateDelta and also kept raw on RPCResponse.Payload.
func (r ZooDecorateRPC) Read(ctx context.Context, req ZooDecorateReadRequest, opts ...RequestOption) (ZooDecorateReadResponse, error) {
	return callRPC[StateDelta](ctx, r.c, RPCZooDecorateRead, req, opts...)
}
