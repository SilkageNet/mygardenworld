package apiserver

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildWorkspacePatchIgnoresCaptureTime(t *testing.T) {
	previous := &pb.WorkspaceState{
		Revision:  7,
		AccountId: 11,
		Basic:     &pb.BasicView{AccountId: 11, Gold: 8, CapturedAt: timestamppb.New(time.Unix(1, 0))},
	}
	next := &pb.WorkspaceState{
		Revision:  7,
		AccountId: 11,
		Basic:     &pb.BasicView{AccountId: 11, Gold: 8, CapturedAt: timestamppb.New(time.Unix(2, 0))},
	}
	if patch := buildWorkspacePatch(previous, next); patch != nil {
		t.Fatalf("capture-only change produced patch: %+v", patch)
	}
}

func TestBuildWorkspacePatchOnlyCarriesChangedDomain(t *testing.T) {
	previous := &pb.WorkspaceState{
		Revision:  7,
		AccountId: 11,
		Basic:     &pb.BasicView{AccountId: 11, Gold: 8},
		Garden:    &pb.GardenView{AccountId: 11},
	}
	next := &pb.WorkspaceState{
		Revision:  8,
		AccountId: 11,
		Basic:     &pb.BasicView{AccountId: 11, Gold: 8},
		Garden:    &pb.GardenView{AccountId: 11, FriendTouchFriendsObserved: true},
	}
	patch := buildWorkspacePatch(previous, next)
	if patch == nil || patch.GetGarden() == nil {
		t.Fatalf("garden change missing from patch: %+v", patch)
	}
	if patch.GetBasic() != nil || patch.GetOrders() != nil || patch.GetWarehouse() != nil {
		t.Fatalf("unchanged domains leaked into patch: %+v", patch)
	}
}

func TestBuildWorkspacePatchExplicitlyClearsOfflineDomains(t *testing.T) {
	previous := &pb.WorkspaceState{
		Revision:  7,
		AccountId: 11,
		Garden:    &pb.GardenView{AccountId: 11},
		Orders:    &pb.OrdersView{AccountId: 11},
	}
	next := &pb.WorkspaceState{AccountId: 11}
	patch := buildWorkspacePatch(previous, next)
	if patch == nil {
		t.Fatal("offline transition produced no patch")
	}
	cleared := map[pb.WorkspaceDomain]bool{}
	for _, domain := range patch.GetClearedDomains() {
		cleared[domain] = true
	}
	if !cleared[pb.WorkspaceDomain_WORKSPACE_DOMAIN_GARDEN] || !cleared[pb.WorkspaceDomain_WORKSPACE_DOMAIN_ORDERS] {
		t.Fatalf("cleared_domains=%v, want garden and orders", patch.GetClearedDomains())
	}
}
