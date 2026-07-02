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

func (svc *Services) ExportPolicy(ctx context.Context, req *connect.Request[pb.ExportPolicyRequest]) (*connect.Response[pb.ExportPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy, err := svc.policyFor(ctx, acc.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	raw, err := policycfg.ToJSON(policy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&pb.ExportPolicyResponse{PolicyJson: raw}), nil
}

func (svc *Services) ImportPolicy(ctx context.Context, req *connect.Request[pb.ImportPolicyRequest]) (*connect.Response[pb.ImportPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy, err := policycfg.FromJSON(req.Msg.GetPolicyJson())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := svc.persistPolicy(ctx, acc.ID, policy); err != nil {
		return nil, mapErr(err)
	}
	if r := svc.Manager.Get(acc.ID); r != nil {
		r.SetPolicy(policy)
		r.Emit(policyUpdatedEvent(policy.GetAutomationEnabled()))
	}
	return connect.NewResponse(&pb.ImportPolicyResponse{Policy: policy}), nil
}

func (svc *Services) CopyPolicy(ctx context.Context, req *connect.Request[pb.CopyPolicyRequest]) (*connect.Response[pb.CopyPolicyResponse], error) {
	source, err := svc.resolveAccount(ctx, req.Msg.GetSourceAccountId(), req.Msg.GetSourceAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	target, err := svc.resolveAccount(ctx, req.Msg.GetTargetAccountId(), req.Msg.GetTargetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy, err := svc.policyFor(ctx, source.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.persistPolicy(ctx, target.ID, policy); err != nil {
		return nil, mapErr(err)
	}
	if r := svc.Manager.Get(target.ID); r != nil {
		r.SetPolicy(policy)
		r.Emit(policyUpdatedEvent(policy.GetAutomationEnabled()))
	}
	return connect.NewResponse(&pb.CopyPolicyResponse{Policy: policy}), nil
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
