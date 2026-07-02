package apiserver

import (
	"context"
	"encoding/json"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
)

func (svc *Services) GetPolicy(ctx context.Context, req *connect.Request[pb.GetPolicyRequest]) (*connect.Response[pb.GetPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy, err := svc.policyFor(ctx, acc.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.GetPolicyResponse{Policy: policy}), nil
}

func (svc *Services) SetPolicy(ctx context.Context, req *connect.Request[pb.SetPolicyRequest]) (*connect.Response[pb.SetPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy := policycfg.Normalize(req.Msg.GetPolicy())
	if err := svc.persistPolicy(ctx, acc.ID, policy); err != nil {
		return nil, mapErr(err)
	}
	if r := svc.Manager.Get(acc.ID); r != nil {
		r.SetPolicy(policy)
		r.Emit(policyUpdatedEvent(policy.GetAutomationEnabled()))
	}
	return connect.NewResponse(&pb.SetPolicyResponse{Policy: policy}), nil
}

func (svc *Services) policyFor(ctx context.Context, accountID int64) (*pb.Policy, error) {
	if r := svc.Manager.Get(accountID); r != nil {
		return r.Policy(), nil
	}
	raw, err := svc.DB.LoadPolicyJSON(ctx, accountID)
	if err != nil {
		return nil, err
	}
	policy, err := policycfg.FromJSON(raw)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func policyUpdatedEvent(enabled bool) runner.Event {
	payload, _ := json.Marshal(map[string]any{"automation_enabled": enabled})
	return runner.Event{
		Kind:        "policy_changed",
		Category:    "system",
		Domain:      "policy",
		Action:      "set",
		Message:     "策略已更新",
		PayloadJSON: string(payload),
	}
}

func (svc *Services) persistPolicy(ctx context.Context, accountID int64, p *pb.Policy) error {
	raw, err := policycfg.ToJSON(p)
	if err != nil {
		return err
	}
	return svc.DB.SavePolicyJSON(ctx, accountID, raw)
}
