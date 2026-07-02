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
	"strconv"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Services is the consolidated handler. One instance is mounted across all
// service path prefixes.
type Services struct {
	DB      *store.DB
	Manager *runner.Manager
	JWT     *auth.JWT
	Log     *slog.Logger
}

// Compile-time assertions: every service interface is implemented.
var (
	_ mygardenworldv1connect.AccountServiceHandler    = (*Services)(nil)
	_ mygardenworldv1connect.AutomationServiceHandler = (*Services)(nil)
	_ mygardenworldv1connect.PolicyServiceHandler     = (*Services)(nil)
	_ mygardenworldv1connect.QueryServiceHandler      = (*Services)(nil)
	_ mygardenworldv1connect.AuthServiceHandler       = (*Services)(nil)
	_ mygardenworldv1connect.AdminServiceHandler      = (*Services)(nil)
)

// resolveAccount picks the account by id (preferred) or name. Enforces
// ownership: non-admin users can only access their own accounts.
func (svc *Services) resolveAccount(ctx context.Context, id, name string) (*store.Account, error) {
	var acc *store.Account
	var err error
	identity := auth.IdentityFromContext(ctx)
	if id != "" {
		n, parseErr := strconv.ParseInt(id, 10, 64)
		if parseErr == nil && n > 0 {
			acc, err = svc.DB.GetAccountByID(ctx, n)
		}
	}
	if acc == nil && name != "" {
		var userID int64
		if identity != nil && identity.Role != "admin" {
			userID = identity.UserID
		}
		acc, err = svc.DB.GetAccountByName(ctx, userID, name)
	}
	if acc == nil && err == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account id or name required"))
	}
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
	if in.GetName() == "" || in.GetUsername() == "" || in.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name/username/password required"))
	}
	channelStr := store.ChannelFromProto(in.GetChannel())
	if channelStr == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel required (one of %v)", supportedChannelStrings()))
	}
	if !babigame.IsSupported(babigame.Channel(channelStr)) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("channel %q not supported (one of %v)", channelStr, supportedChannelStrings()))
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
	acc, err := svc.DB.CreateAccount(ctx, userID, in.GetName(), channelStr, in.GetUsername(), in.GetPassword())
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.CreateAccountResponse{Account: store.AccountToProto(acc)}
	if in.GetLoginNow() {
		if _, err := svc.Manager.Start(ctx, acc.ID); err != nil {
			resp.LoginError = formatLoginErr(err)
		}
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
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	_ = svc.Manager.Stop(acc.ID)
	if err := svc.DB.DeleteAccount(ctx, acc.ID, ""); err != nil {
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

func (svc *Services) GetAccount(ctx context.Context, req *connect.Request[pb.GetAccountRequest]) (*connect.Response[pb.GetAccountResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	p := store.AccountToProto(acc)
	if r := svc.Manager.Get(acc.ID); r != nil {
		p.Connected = r.Connected()
	}
	return connect.NewResponse(&pb.GetAccountResponse{Account: p}), nil
}

func (svc *Services) LoginAccount(ctx context.Context, req *connect.Request[pb.LoginAccountRequest]) (*connect.Response[pb.LoginAccountResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	r, err := svc.Manager.Reload(ctx, acc.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := store.AccountToProto(r.Account())
	out.Connected = r.Connected()
	return connect.NewResponse(&pb.LoginAccountResponse{
		Account:    out,
		LoggedInAt: timestamppb.Now(),
	}), nil
}

func (svc *Services) LogoutAccount(ctx context.Context, req *connect.Request[pb.LogoutAccountRequest]) (*connect.Response[pb.LogoutAccountResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetId(), req.Msg.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	_ = svc.Manager.Stop(acc.ID)
	out := store.AccountToProto(acc)
	out.Connected = false
	return connect.NewResponse(&pb.LogoutAccountResponse{Account: out}), nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, store.ErrAccountNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrAccountExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, store.ErrAccountAmbiguous):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("account name matches multiple users; use account id"))
	case errors.Is(err, store.ErrUserNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrUserExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
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
