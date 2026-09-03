package apiserver

import (
	"context"
	"encoding/json"
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
	r, err := svc.Manager.StartWithSource(ctx, acc.ID, runner.StartSourceAutomationEnable)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.enableAutomation(ctx, acc.ID, r); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.EnableAutomationResponse{}), nil
}

func (svc *Services) DisableAutomation(ctx context.Context, req *connect.Request[pb.DisableAutomationRequest]) (*connect.Response[pb.DisableAutomationResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.disableAutomation(ctx, acc.ID, svc.Manager.Get(acc.ID)); err != nil {
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

func policyEvent(enabled bool) runner.Event {
	payload, _ := json.Marshal(map[string]any{"automation_enabled": enabled})
	message := "自动化已停止"
	if enabled {
		message = "自动化已启动"
	}
	return runner.Event{Kind: "policy_changed", Category: "system", Domain: "policy", Action: "set", Message: message, PayloadJSON: string(payload)}
}

func (svc *Services) enableAutomation(ctx context.Context, accountID int64, r *runner.Runner) error {
	if r == nil {
		return nil
	}
	p := r.Policy()
	wasEnabled := p.GetAutomationEnabled()
	p.AutomationEnabled = true
	if err := svc.persistPolicy(ctx, accountID, p); err != nil {
		return err
	}
	if !wasEnabled {
		r.SetPolicy(p)
		r.Emit(policyEvent(true))
	}
	return nil
}

func (svc *Services) disableAutomation(ctx context.Context, accountID int64, r *runner.Runner) error {
	p, err := svc.policyFor(ctx, accountID)
	if err != nil {
		return err
	}
	p.AutomationEnabled = false
	if r != nil {
		live := r.Policy()
		if live.GetAutomationEnabled() {
			live.AutomationEnabled = false
			r.SetPolicy(live)
			r.Emit(policyEvent(false))
		}
	}
	return svc.persistPolicy(ctx, accountID, p)
}
