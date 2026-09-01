package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"google.golang.org/protobuf/proto"
)

type policyRuntime interface {
	SetPolicy(*pb.Policy)
	Policy() *pb.Policy
	Emit(runner.Event)
}

func (svc *Services) GetPolicy(ctx context.Context, req *connect.Request[pb.GetPolicyRequest]) (*connect.Response[pb.GetPolicyResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
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
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, mapErr(err)
	}
	if features := policycfg.UnsupportedSDKAdFeatures(req.Msg.GetPolicy()); len(features) > 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
			"不支持需要客户端 SDK 广告回调的自动化，也不会编造回调参数或 token: %s",
			strings.Join(features, "、"),
		))
	}
	if err := policycfg.ValidateVersion(req.Msg.GetPolicy()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Preserve the live start/pause intent. Settings edits must not flip
	// automation back on after the operator paused via AutomationService.Stop.
	current, err := svc.policyFor(ctx, acc.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	policy := policycfg.Normalize(req.Msg.GetPolicy())
	policy.AutomationEnabled = current.GetAutomationEnabled()
	var runtime policyRuntime
	if r := svc.Manager.Get(acc.ID); r != nil {
		runtime = r
	}
	effective, err := svc.persistAndApplyPolicy(ctx, acc.ID, runtime, policy)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.SetPolicyResponse{Policy: effective}), nil
}

// persistAndApplyPolicy returns the runner's effective policy, which may
// differ from the request when a lifecycle safety transition happens during
// SetPolicy (for example, disabling a pending displaced-session relogin).
func (svc *Services) persistAndApplyPolicy(ctx context.Context, accountID int64, runtime policyRuntime, policy *pb.Policy) (*pb.Policy, error) {
	if err := svc.persistPolicy(ctx, accountID, policy); err != nil {
		return nil, err
	}
	if runtime == nil {
		return policy, nil
	}

	runtime.SetPolicy(policy)
	effective := runtime.Policy()
	if !proto.Equal(effective, policy) {
		if err := svc.persistPolicy(ctx, accountID, effective); err != nil {
			return nil, err
		}
	}
	runtime.Emit(policyUpdatedEvent(effective.GetAutomationEnabled()))
	return effective, nil
}

func (svc *Services) policyFor(ctx context.Context, accountID int64) (*pb.Policy, error) {
	if svc.Manager != nil {
		if r := svc.Manager.Get(accountID); r != nil {
			return r.Policy(), nil
		}
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
	if err := svc.DB.SavePolicyJSON(ctx, accountID, raw); err != nil {
		return err
	}
	if svc.Redeem != nil {
		svc.Redeem.NotifyAccountPolicyChanged()
	}
	return nil
}
