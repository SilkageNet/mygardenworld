package apiserver

import (
	"context"
	"encoding/json"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
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
	p := r.Policy()
	p.AutomationEnabled = true
	r.SetPolicy(p)
	_ = svc.DB.SetPolicyValue(ctx, acc.ID, policycfg.KeyAutomationEnabled, "true")
	r.Emit(policyEvent(true))
	return connect.NewResponse(&pb.StartResponse{}), nil
}

func (svc *Services) Stop(ctx context.Context, req *connect.Request[pb.StopRequest]) (*connect.Response[pb.StopResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	r := svc.Manager.Get(acc.ID)
	if r != nil {
		p := r.Policy()
		p.AutomationEnabled = false
		r.SetPolicy(p)
		r.Emit(policyEvent(false))
	}
	_ = svc.DB.SetPolicyValue(ctx, acc.ID, policycfg.KeyAutomationEnabled, "false")
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
	return runner.Event{Kind: "policy_changed", Message: message, PayloadJSON: string(payload)}
}
