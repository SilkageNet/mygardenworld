// Package apiserver hosts the Connect handler implementations for the
// mygardenworld.v1 services. Connect lets the same handler serve all three
// protocols simultaneously (Connect over HTTP/JSON, classic gRPC, and
// gRPC-Web) so cmd/gardend exposes a single bind address.
package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Services owns the shared application dependencies and service operations.
// Transport registration exposes it through the domain-specific handler
// wrappers returned by NewHandlers instead of mounting this core directly.
type Services struct {
	DB           *store.DB
	Manager      *runner.Manager
	JWT          *auth.JWT
	Log          *slog.Logger
	LoginLimiter *LoginLimiter
	AlipayLogins *AlipayLoginCoordinator
	// ListenAddr is the gardend bind address (host:port). Used to scope the
	// HttpOnly refresh cookie so dual local stacks on the same host (e.g.
	// :50051 and :50052) do not overwrite each other's browser cookies.
	ListenAddr string

	workspaceProjectionMu sync.Mutex
	workspaceProjections  map[int64]*workspaceProjectionCache
}

// resolveAccount resolves the only public account identity: its stable id.
// Non-admin users can only access their own accounts.
func (svc *Services) resolveAccount(ctx context.Context, id int64) (*store.Account, error) {
	identity := auth.IdentityFromContext(ctx)
	if id <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid account id required"))
	}
	acc, err := svc.DB.GetAccountByID(ctx, id)
	if err != nil {
		return nil, mapErr(err)
	}
	if identity != nil && identity.Role != "admin" && acc.UserID != identity.UserID {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not your account"))
	}
	return acc, nil
}

func (svc *Services) CreateAccount(ctx context.Context, req *connect.Request[pb.CreateAccountRequest]) (*connect.Response[pb.CreateAccountResponse], error) {
	in := req.Msg
	username := strings.TrimSpace(in.GetUsername())
	password := in.GetPassword()
	if username == "" || password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username/password required"))
	}
	channelStr := store.ChannelFromProto(in.GetChannel())
	if channelStr == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel required (one of %v)", supportedChannelStrings()))
	}
	if !babigame.IsSupported(babigame.Channel(channelStr)) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel %q not supported (one of %v)", channelStr, supportedChannelStrings()))
	}
	if channelStr == string(babigame.ChannelAlipay) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("alipay accounts must use StartAlipayLogin"))
	}
	userID := auth.UserIDFromContext(ctx)
	if userID > 0 {
		user, err := svc.DB.GetUserByID(ctx, userID)
		if err != nil {
			return nil, mapErr(err)
		}
		count, err := svc.DB.CountAccountsByUser(ctx, userID)
		if err != nil {
			return nil, mapErr(err)
		}
		if count >= user.MaxAccounts {
			return nil, connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("account quota reached (%d/%d)", count, user.MaxAccounts))
		}
	}
	session, err := svc.probeAccountIdentity(ctx, channelStr, username, password)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("login: %s", formatLoginErr(err)))
	}
	name, err := svc.DB.UniqueAccountName(ctx, userID, 0, babigame.DisplayNameFromSession(session, username))
	if err != nil {
		return nil, mapErr(err)
	}
	acc, err := svc.DB.CreateAccount(ctx, userID, name, channelStr, username, password)
	if err != nil {
		return nil, mapErr(err)
	}
	svc.saveLoginProbe(ctx, acc.ID, session)
	if updated, err := svc.DB.GetAccountByID(ctx, acc.ID); err == nil {
		acc = updated
	}
	resp := &pb.CreateAccountResponse{Account: store.AccountToProto(acc)}
	if r, err := svc.Manager.Start(ctx, acc.ID); err != nil {
		resp.LoginError = formatLoginErr(err)
	} else {
		if err := svc.enableAutomation(ctx, acc.ID, r); err != nil {
			resp.LoginError = formatLoginErr(err)
		}
		out := store.AccountToProto(r.Account())
		out.Connected = r.Connected()
		resp.Account = out
	}
	return connect.NewResponse(resp), nil
}

// formatLoginErr renders an error as a single ASCII-safe line. Upstream
// errors get their structured form (host/status/preview); everything else
// goes through SafeUTF8 to scrub potentially-non-UTF-8 bytes.
func formatLoginErr(err error) string {
	if err == nil {
		return ""
	}
	if ue := babigame.AsUpstreamError(err); ue != nil {
		// UpstreamError.Error() is already safe (preview is hex-encoded).
		return ue.Error()
	}
	return babigame.SafeUTF8(err.Error())
}

func (svc *Services) DeleteAccount(ctx context.Context, req *connect.Request[pb.DeleteAccountRequest]) (*connect.Response[pb.DeleteAccountResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapErr(err)
	}
	_ = svc.Manager.Stop(acc.ID)
	if err := svc.DB.DeleteAccount(ctx, acc.ID); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.DeleteAccountResponse{}), nil
}

func (svc *Services) ListAccounts(ctx context.Context, _ *connect.Request[pb.ListAccountsRequest]) (*connect.Response[pb.ListAccountsResponse], error) {
	var userID int64
	if !auth.IsAdmin(ctx) {
		userID = auth.UserIDFromContext(ctx)
	}
	accounts, err := svc.DB.ListAccounts(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.ListAccountsResponse{Accounts: make([]*pb.Account, 0, len(accounts))}
	for _, a := range accounts {
		p := store.AccountToProto(a)
		if r := svc.Manager.Get(a.ID); r != nil {
			p.Connected = r.Connected()
		}
		resp.Accounts = append(resp.Accounts, p)
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) ConnectAccount(ctx context.Context, req *connect.Request[pb.ConnectAccountRequest]) (*connect.Response[pb.ConnectAccountResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapErr(err)
	}
	r, err := svc.Manager.Reload(ctx, acc.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	if err := svc.enableAutomation(ctx, acc.ID, r); err != nil {
		return nil, mapErr(err)
	}
	out := store.AccountToProto(r.Account())
	out.Connected = r.Connected()
	return connect.NewResponse(&pb.ConnectAccountResponse{
		Account:    out,
		LoggedInAt: timestamppb.Now(),
	}), nil
}

func (svc *Services) DisconnectAccount(ctx context.Context, req *connect.Request[pb.DisconnectAccountRequest]) (*connect.Response[pb.DisconnectAccountResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapErr(err)
	}
	r := svc.Manager.Get(acc.ID)
	if err := svc.disableAutomation(ctx, acc.ID, r); err != nil {
		return nil, mapErr(err)
	}
	_ = svc.Manager.Stop(acc.ID)
	// Stop is a no-op when the runner already exited after a kick; still clear
	// the cached 异常 reason so an intentional stop returns to plain offline.
	svc.Manager.ClearLastDiagnostics(acc.ID)
	out := store.AccountToProto(acc)
	out.Connected = false
	return connect.NewResponse(&pb.DisconnectAccountResponse{Account: out}), nil
}

func (svc *Services) RedeemCode(ctx context.Context, req *connect.Request[pb.RedeemCodeRequest]) (*connect.Response[pb.RedeemCodeResponse], error) {
	code := strings.TrimSpace(req.Msg.GetCode())
	if code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem code required"))
	}
	accounts, err := svc.resolveRedeemAccounts(ctx, req.Msg.GetAccountIds())
	if err != nil {
		return nil, mapErr(err)
	}

	resp := &pb.RedeemCodeResponse{Results: make([]*pb.RedeemCodeResult, 0, len(accounts))}
	for _, acc := range accounts {
		result := &pb.RedeemCodeResult{
			AccountId:   acc.ID,
			AccountName: acc.Name,
		}
		r := svc.Manager.Get(acc.ID)
		if r == nil || !r.Connected() {
			started, startErr := svc.Manager.Start(ctx, acc.ID)
			if startErr != nil {
				result.Message = formatLoginErr(startErr)
				resp.FailureCount++
				resp.Results = append(resp.Results, result)
				continue
			}
			r = started
		}
		if resultInfo, err := r.RedeemCode(ctx, code); err != nil {
			result.Message = babigame.SafeUTF8(err.Error())
			resp.FailureCount++
			resp.Results = append(resp.Results, result)
			continue
		} else {
			result.Ok = true
			result.Message = redeemResultMessage(resultInfo)
			resp.SuccessCount++
			resp.Results = append(resp.Results, result)
		}
	}
	return connect.NewResponse(resp), nil
}

func redeemResultMessage(result runner.RedeemResult) string {
	if result.Code == "" {
		return "ok"
	}
	parts := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		name := item.Name
		if name == "" {
			name = fmt.Sprintf("#%d", item.ItemID)
		}
		parts = append(parts, fmt.Sprintf("%sx%d", name, item.Count))
	}
	switch {
	case len(parts) > 0 && result.MailNew > 0:
		return fmt.Sprintf("%s；另有 %d 封奖励邮件", strings.Join(parts, "、"), result.MailNew)
	case len(parts) > 0:
		return strings.Join(parts, "、")
	case result.MailNew > 0:
		return fmt.Sprintf("奖励已入邮件（%d 封待领取）", result.MailNew)
	default:
		return "ok"
	}
}

func (svc *Services) resolveRedeemAccounts(ctx context.Context, accountIDs []int64) ([]*store.Account, error) {
	if len(accountIDs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem account ids required"))
	}
	out := make([]*store.Account, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	channel := ""
	for _, id := range accountIDs {
		if id <= 0 {
			continue
		}
		acc, err := svc.resolveAccount(ctx, id)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[acc.ID]; ok {
			continue
		}
		if len(out) == 0 {
			channel = acc.Channel
		} else if acc.Channel != channel {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem accounts must belong to one channel"))
		}
		seen[acc.ID] = struct{}{}
		out = append(out, acc)
	}
	if len(out) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem account ids required"))
	}
	return out, nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, store.ErrAccountNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrAccountExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, store.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrUserExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, store.ErrLastActiveAdmin):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, store.ErrTokenInvalid):
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, err)
}

func supportedChannelStrings() []string {
	all := babigame.SupportedChannels()
	out := make([]string, len(all))
	for i, c := range all {
		out[i] = string(c)
	}
	return out
}
