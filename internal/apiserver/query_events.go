package apiserver

import (
	"context"

	connect "connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultStreamEventReplayLimit = 500

type streamEventFilter struct {
	kinds      map[string]struct{}
	kindList   []string
	accountIDs []int64
	allowedIDs map[int64]struct{}
}

// StreamEvents is a server-streaming RPC. It first replays persisted event_log
// rows, then forwards live events from the in-process bus. The subscription is
// opened before replay so events emitted during DB replay are either included
// in the replay result or delivered live.
func (svc *Services) StreamEvents(ctx context.Context, req *connect.Request[pb.StreamEventsRequest], stream *connect.ServerStream[pb.StreamEventsResponse]) error {
	ch, cancel := svc.Manager.Bus().SubscribeLive(256)
	defer cancel()

	filter, err := svc.streamEventFilter(ctx, req.Msg)
	if err != nil {
		return err
	}

	highWater := req.Msg.GetAfterEventId()
	if replayLimit := streamEventReplayLimit(req.Msg.GetReplayLimit()); replayLimit > 0 {
		events, err := svc.DB.ListEventLogs(ctx, store.ListEventLogsOptions{
			AccountIDs: filter.accountIDs,
			Kinds:      filter.kindList,
			AfterID:    req.Msg.GetAfterEventId(),
			Limit:      replayLimit,
		})
		if err != nil {
			return mapErr(err)
		}
		for _, event := range events {
			if event.ID > highWater {
				highWater = event.ID
			}
			if err := stream.Send(&pb.StreamEventsResponse{Event: eventLogToProto(event)}); err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if e.ID > 0 && e.ID <= highWater {
				continue
			}
			if !filter.matches(e) {
				continue
			}
			if err := stream.Send(&pb.StreamEventsResponse{Event: e.ToProto()}); err != nil {
				return err
			}
			if e.ID > highWater {
				highWater = e.ID
			}
		}
	}
}

func (svc *Services) streamEventFilter(ctx context.Context, req *pb.StreamEventsRequest) (streamEventFilter, error) {
	filter := streamEventFilter{}
	for _, kind := range req.GetKinds() {
		if kind == "" {
			continue
		}
		if filter.kinds == nil {
			filter.kinds = make(map[string]struct{}, len(req.GetKinds()))
		}
		if _, ok := filter.kinds[kind]; ok {
			continue
		}
		filter.kinds[kind] = struct{}{}
		filter.kindList = append(filter.kindList, kind)
	}

	if req.GetAccountId() != 0 {
		acc, err := svc.resolveAccount(ctx, req.GetAccountId())
		if err != nil {
			return filter, mapErr(err)
		}
		filter.accountIDs = []int64{acc.ID}
		filter.allowedIDs = map[int64]struct{}{acc.ID: {}}
		return filter, nil
	}

	if !auth.IsAdmin(ctx) {
		userID := auth.UserIDFromContext(ctx)
		accounts, err := svc.DB.ListAccounts(ctx, userID)
		if err != nil {
			return filter, mapErr(err)
		}
		filter.accountIDs = make([]int64, 0, len(accounts))
		filter.allowedIDs = make(map[int64]struct{}, len(accounts))
		for _, acc := range accounts {
			filter.accountIDs = append(filter.accountIDs, acc.ID)
			filter.allowedIDs[acc.ID] = struct{}{}
		}
	}

	return filter, nil
}

func (f streamEventFilter) matches(e runner.Event) bool {
	if len(f.kinds) > 0 {
		if _, ok := f.kinds[e.Kind]; !ok {
			return false
		}
	}
	if f.allowedIDs != nil {
		if _, ok := f.allowedIDs[e.AccountID]; !ok {
			return false
		}
	}
	return true
}

func streamEventReplayLimit(limit int32) int {
	if limit < 0 {
		return 0
	}
	if limit == 0 {
		return defaultStreamEventReplayLimit
	}
	return int(limit)
}

func eventLogToProto(e store.EventLog) *pb.Event {
	return &pb.Event{
		Id:          e.ID,
		Ts:          timestamppb.New(e.TS),
		AccountId:   e.AccountID,
		AccountName: e.AccountName,
		Kind:        e.Kind,
		Message:     e.Message,
		PayloadJson: e.PayloadJSON,
		Category:    e.Category,
		Domain:      e.Domain,
		Action:      e.Action,
		Label:       e.Label,
		Level:       e.Level,
	}
}
