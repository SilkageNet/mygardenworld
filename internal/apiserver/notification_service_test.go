package apiserver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestNotificationCommandsAndViewsAreUserScopedEvenForAdmin(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	owner, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := db.CreateUser(ctx, "admin", "admin@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db}
	ownerCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: owner.ID, Role: "user"})
	adminCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: admin.ID, Role: "admin"})
	endpoint := "https://example.com/hook?token=SECRET"
	if _, err := svc.SaveNotificationSettings(ownerCtx, connect.NewRequest(&pb.SaveNotificationSettingsRequest{Enabled: true, Endpoint: &endpoint, CooldownMinutes: 30})); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TestNotification(ownerCtx, connect.NewRequest(&pb.TestNotificationRequest{})); err != nil {
		t.Fatal(err)
	}
	view, err := svc.userNotifications(adminCtx, 0)
	if err != nil || view.Settings.HasEndpoint || view.Settings.Enabled || len(view.Deliveries) != 0 {
		t.Fatalf("admin sees owner: %+v %v", view, err)
	}
	view, err = svc.userNotifications(ownerCtx, 0)
	if err != nil || !view.Settings.HasEndpoint || len(view.Deliveries) != 1 {
		t.Fatalf("owner view: %+v %v", view, err)
	}
	encoded, err := protojson.Marshal(view)
	if err != nil || strings.Contains(string(encoded), "SECRET") || strings.Contains(string(encoded), "example.com") {
		t.Fatal("read model leaks endpoint", err)
	}
	if _, err := svc.SaveNotificationSettings(adminCtx, connect.NewRequest(&pb.SaveNotificationSettingsRequest{Enabled: true, Endpoint: &endpoint, CooldownMinutes: 60})); err != nil {
		t.Fatal(err)
	}
	view, _ = svc.userNotifications(ownerCtx, 0)
	if view.Settings.CooldownMinutes != 30 {
		t.Fatal("admin changed owner settings")
	}
	if _, err := svc.userNotifications(ctx, 0); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("anonymous read allowed", err)
	}
	if _, err := svc.SaveNotificationSettings(ctx, connect.NewRequest(&pb.SaveNotificationSettingsRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("anonymous save allowed", err)
	}
	if _, err := svc.TestNotification(ctx, connect.NewRequest(&pb.TestNotificationRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("anonymous test allowed", err)
	}
	unsafe := "http://169.254.169.254/"
	if _, err := svc.SaveNotificationSettings(ownerCtx, connect.NewRequest(&pb.SaveNotificationSettingsRequest{Enabled: true, Endpoint: &unsafe, CooldownMinutes: 30})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatal("unsafe destination saved", err)
	}
}
