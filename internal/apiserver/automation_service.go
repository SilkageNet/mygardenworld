package apiserver

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/runner"
)

func (svc *Services) EnableAutomation(ctx context.Context, req *connect.Request[pb.EnableAutomationRequest]) (*connect.Response[pb.EnableAutomationResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	if _, err := svc.startAutomation(ctx, acc.ID, runner.StartSourceAutomationEnable, false); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.EnableAutomationResponse{}), nil
}

func (svc *Services) DisableAutomation(ctx context.Context, req *connect.Request[pb.DisableAutomationRequest]) (*connect.Response[pb.DisableAutomationResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.disableAutomation(ctx, acc.ID); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.DisableAutomationResponse{}), nil
}

func (svc *Services) TakeUnionRaceTask(ctx context.Context, req *connect.Request[pb.TakeUnionRaceTaskRequest]) (*connect.Response[pb.TakeUnionRaceTaskResponse], error) {
	if req.Msg.GetTaskMsId() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("竞赛任务标识无效"))
	}
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	r, err := svc.Manager.StartWithSource(ctx, acc.ID, runner.StartSourceManualOperation)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := r.TakeUnionRaceTask(ctx, req.Msg.GetTaskMsId()); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&pb.TakeUnionRaceTaskResponse{}), nil
}

func (svc *Services) DeleteUnionRaceTask(ctx context.Context, req *connect.Request[pb.DeleteUnionRaceTaskRequest]) (*connect.Response[pb.DeleteUnionRaceTaskResponse], error) {
	if req.Msg.GetTaskMsId() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("竞赛任务标识无效"))
	}
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	r, err := svc.Manager.StartWithSource(ctx, acc.ID, runner.StartSourceManualOperation)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := r.DeleteUnionRaceTask(ctx, req.Msg.GetTaskMsId()); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&pb.DeleteUnionRaceTaskResponse{}), nil
}

func (svc *Services) startAutomation(ctx context.Context, accountID int64, source runner.StartSource, reconnect bool) (*runner.Runner, error) {
	r, err := svc.Manager.StartAutomation(ctx, accountID, source, reconnect)
	if err != nil {
		return nil, err
	}
	if svc.Redeem != nil {
		svc.Redeem.NotifyAccountPolicyChanged()
	}
	return r, nil
}

func (svc *Services) disableAutomation(ctx context.Context, accountID int64) error {
	if svc.Manager != nil {
		if err := svc.Manager.PauseAutomation(ctx, accountID, false); err != nil {
			return err
		}
		if svc.Redeem != nil {
			svc.Redeem.NotifyAccountPolicyChanged()
		}
		return nil
	}
	p, err := svc.policyFor(ctx, accountID)
	if err != nil {
		return err
	}
	p.AutomationEnabled = false
	return svc.persistPolicy(ctx, accountID, p)
}
