package apiserver

import (
	"context"
	"encoding/json"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/runner"
)

func (svc *Services) Start(ctx context.Context, req *connect.Request[pb.StartRequest]) (*connect.Response[pb.StartResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	r, err := svc.Manager.Start(ctx, acc.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.enableAutomation(ctx, acc.ID, r); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.StartResponse{}), nil
}

func (svc *Services) Stop(ctx context.Context, req *connect.Request[pb.StopRequest]) (*connect.Response[pb.StopResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.disableAutomation(ctx, acc.ID, svc.Manager.Get(acc.ID)); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.StopResponse{}), nil
}

func (svc *Services) Reload(ctx context.Context, req *connect.Request[pb.ReloadRequest]) (*connect.Response[pb.ReloadResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	if _, err := svc.Manager.Reload(ctx, acc.ID); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.ReloadResponse{}), nil
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
