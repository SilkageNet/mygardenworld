package apiserver

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/notification"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func (svc *Services) SaveNotificationSettings(ctx context.Context, req *connect.Request[pb.SaveNotificationSettingsRequest]) (*connect.Response[pb.SaveNotificationSettingsResponse], error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.Endpoint != nil && req.Msg.GetEndpoint() != "" {
		if err := notification.ValidateEndpoint(req.Msg.GetEndpoint()); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	err = svc.DB.SaveNotificationSettings(ctx, userID, req.Msg.GetEnabled(), req.Msg.Endpoint, int(req.Msg.GetCooldownMinutes()))
	if errors.Is(err, store.ErrNotificationSettings) {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("保存通知设置失败"))
	}
	return connect.NewResponse(&pb.SaveNotificationSettingsResponse{}), nil
}

func (svc *Services) TestNotification(ctx context.Context, _ *connect.Request[pb.TestNotificationRequest]) (*connect.Response[pb.TestNotificationResponse], error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	id, err := svc.DB.QueueNotificationTest(ctx, userID, time.Now().UTC())
	if errors.Is(err, store.ErrNotificationSettings) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("请先保存并启用通知设置"))
	}
	if errors.Is(err, store.ErrNotificationTestCooldown) || errors.Is(err, store.ErrNotificationQueueFull) {
		return nil, connect.NewError(connect.CodeResourceExhausted, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("无法加入测试通知队列"))
	}
	return connect.NewResponse(&pb.TestNotificationResponse{DeliveryId: id}), nil
}

func (svc *Services) userNotifications(ctx context.Context, beforeID int64) (*pb.UserNotificationsView, error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if beforeID < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("无效的通知分页游标"))
	}
	settings, err := svc.DB.NotificationSettings(ctx, userID)
	if err != nil {
		return nil, errors.New("读取通知设置失败")
	}
	rows, err := svc.DB.NotificationDeliveries(ctx, userID, beforeID)
	if err != nil {
		return nil, errors.New("读取通知记录失败")
	}
	view := &pb.UserNotificationsView{BeforeId: beforeID, Settings: &pb.UserNotificationSettings{Enabled: settings.Enabled, HasEndpoint: settings.HasEndpoint, CooldownMinutes: int32(settings.CooldownMinutes)}, HasMore: len(rows) > 5}
	if len(rows) > 5 {
		rows = rows[:5]
	}
	for _, row := range rows {
		view.Deliveries = append(view.Deliveries, &pb.NotificationDelivery{Id: row.ID, Title: row.Title, Status: row.Status, Attempts: int32(row.Attempts), CreatedMs: row.CreatedMS, LastError: row.LastError})
		view.NextBeforeId = row.ID
	}
	return view, nil
}
