package apiserver

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (svc *Services) initialAccountPolicy(ctx context.Context, sourceID int64) (string, error) {
	if sourceID == 0 {
		return "", nil
	}
	account, err := svc.resolveAccount(ctx, sourceID)
	if err != nil {
		return "", err
	}
	policy, err := svc.policyFor(ctx, account.ID)
	if err != nil {
		return "", err
	}
	return policycfg.ToJSON(policy)
}

func (svc *Services) ReauthenticateAccount(ctx context.Context, req *connect.Request[pb.ReauthenticateAccountRequest]) (*connect.Response[pb.ReauthenticateAccountResponse], error) {
	account, err := svc.resolveAccount(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if account.Channel != string(babigame.ChannelIOS) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("支付宝账号请重新扫码授权"))
	}
	if req.Msg.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("请输入当前密码"))
	}
	session, err := svc.probeAccountIdentity(ctx, account.Channel, account.Username, req.Msg.GetPassword())
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New(formatLoginErr(err)))
	}
	if err := svc.DB.UpdateAccountCredentials(ctx, account.ID, account.Username, req.Msg.GetPassword()); err != nil {
		return nil, mapErr(err)
	}
	svc.saveLoginProbe(ctx, account.ID, session)
	r, err := svc.Manager.ReloadWithSource(ctx, account.ID, runner.StartSourceControlPanel)
	if err != nil {
		return nil, mapErr(err)
	}
	out := store.AccountToProto(r.Account())
	out.Connected = r.Connected()
	return connect.NewResponse(&pb.ReauthenticateAccountResponse{Account: out, LoggedInAt: timestamppb.Now()}), nil
}
