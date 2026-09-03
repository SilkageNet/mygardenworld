package apiserver

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUpdateRedeemCodeExpiryRequiresAdminAndPersistsOverride(t *testing.T) {
	ctx := context.Background()
	svc := newAuthTestService(t, LoginLimiterConfig{})
	svc.Manager = runner.NewManager(svc.DB, runner.NewBus(), svc.Log)
	redeemService, err := redeemsvc.NewService(ctx, svc.DB, svc.Manager, svc.Log)
	if err != nil {
		t.Fatal(err)
	}
	svc.Redeem = redeemService
	entry, _, err := svc.DB.UpsertRedeemCode(ctx, store.RedeemCodeInput{
		Code: "ADMIN-CORRECT", Channel: "ios", SourceKey: "public:test",
	})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	req := connect.NewRequest(&pb.UpdateRedeemCodeExpiryRequest{
		Fingerprint: entry.Fingerprint,
		Mode:        pb.RedeemExpiryOverrideMode_REDEEM_EXPIRY_OVERRIDE_MODE_FINITE,
		ExpiresAt:   timestamppb.New(expires),
	})
	if _, err := svc.UpdateRedeemCodeExpiry(ctx, req); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("non-admin code=%s err=%v", connect.CodeOf(err), err)
	}
	adminCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: 7, Role: "admin"})
	resp, err := svc.UpdateRedeemCodeExpiry(adminCtx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Msg.GetCode().GetExpiryOverridden() || resp.Msg.GetCode().GetPermanent() {
		t.Fatalf("updated code=%+v", resp.Msg.GetCode())
	}
	clearResp, err := svc.UpdateRedeemCodeExpiry(adminCtx, connect.NewRequest(&pb.UpdateRedeemCodeExpiryRequest{
		Fingerprint: entry.Fingerprint,
		Mode:        pb.RedeemExpiryOverrideMode_REDEEM_EXPIRY_OVERRIDE_MODE_SOURCE,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if clearResp.Msg.GetCode().GetExpiryOverridden() || !clearResp.Msg.GetCode().GetPermanent() {
		t.Fatalf("cleared code=%+v", clearResp.Msg.GetCode())
	}
}
