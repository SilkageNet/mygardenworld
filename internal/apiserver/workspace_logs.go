package apiserver

import (
	"context"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultWorkspaceLogReplayLimit = 500

func (svc *Services) workspaceLogs(ctx context.Context, accountID, afterID int64) ([]*pb.Event, int64, error) {
	acc, err := svc.resolveAccount(ctx, accountID)
	if err != nil {
		return nil, afterID, err
	}
	rows, err := svc.DB.ListEventLogs(ctx, store.ListEventLogsOptions{
		AccountIDs: []int64{acc.ID},
		AfterID:    afterID,
		Limit:      defaultWorkspaceLogReplayLimit,
	})
	if err != nil {
		return nil, afterID, err
	}
	out := make([]*pb.Event, 0, len(rows))
	highWater := afterID
	for _, row := range rows {
		out = append(out, eventLogToProto(row))
		if row.ID > highWater {
			highWater = row.ID
		}
	}
	return out, highWater, nil
}

func eventToProto(e runner.Event) *pb.Event {
	return e.ToProto()
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
