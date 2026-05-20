package apiserver

import (
	"context"
	"encoding/json"
	"strings"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
)

func (svc *Services) GetPolicy(ctx context.Context, req *connect.Request[pb.GetPolicyRequest]) (*connect.Response[pb.GetPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy := automation.DefaultPolicy()
	if r := svc.Manager.Get(acc.ID); r != nil {
		policy = r.Policy()
	} else {
		entries, _ := svc.DB.LoadPolicyValues(ctx, acc.ID)
		policy = policycfg.FromEntries(entries)
	}
	return connect.NewResponse(&pb.GetPolicyResponse{Policy: policy}), nil
}

func (svc *Services) SetPolicy(ctx context.Context, req *connect.Request[pb.SetPolicyRequest]) (*connect.Response[pb.SetPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	p := policycfg.Normalize(req.Msg.GetPolicy())
	r := svc.Manager.Get(acc.ID)
	if r != nil {
		r.SetPolicy(p)
	}
	if err := svc.persistPolicy(ctx, acc.ID, p); err != nil {
		return nil, mapErr(err)
	}
	if r != nil {
		r.Emit(policyUpdatedEvent(p.GetAutomationEnabled()))
	}
	return connect.NewResponse(&pb.SetPolicyResponse{Policy: p}), nil
}

func (svc *Services) UpdatePolicy(ctx context.Context, req *connect.Request[pb.UpdatePolicyRequest]) (*connect.Response[pb.UpdatePolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	policy := automation.DefaultPolicy()
	if r := svc.Manager.Get(acc.ID); r != nil {
		policy = r.Policy()
	} else {
		entries, _ := svc.DB.LoadPolicyValues(ctx, acc.ID)
		policy = policycfg.FromEntries(entries)
	}

	resp := &pb.UpdatePolicyResponse{}
	for _, entry := range req.Msg.GetEntries() {
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 {
			resp.Errors = append(resp.Errors, &pb.PolicyPatchError{Entry: entry, Message: "missing '='"})
			continue
		}
		key := strings.TrimSpace(entry[:eq])
		value := strings.TrimSpace(entry[eq+1:])
		if err := policycfg.SetKey(policy, key, value); err != nil {
			resp.Errors = append(resp.Errors, &pb.PolicyPatchError{Entry: entry, Message: babigame.SafeUTF8(err.Error())})
			continue
		}
		_ = svc.DB.SetPolicyValue(ctx, acc.ID, key, value)
	}
	if r := svc.Manager.Get(acc.ID); r != nil {
		r.SetPolicy(policy)
		r.Emit(policyUpdatedEvent(policy.GetAutomationEnabled()))
	}
	resp.Policy = policy
	return connect.NewResponse(resp), nil
}

func policyUpdatedEvent(enabled bool) runner.Event {
	payload, _ := json.Marshal(map[string]any{"automation_enabled": enabled})
	return runner.Event{Kind: "policy_changed", Message: "策略已更新", PayloadJSON: string(payload)}
}

func (svc *Services) persistPolicy(ctx context.Context, accountID int64, p *pb.Policy) error {
	if p == nil {
		return nil
	}
	for k, v := range policycfg.Flatten(p) {
		if err := svc.DB.SetPolicyValue(ctx, accountID, k, v); err != nil {
			return err
		}
	}
	return nil
}
