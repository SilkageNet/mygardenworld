package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestPearlCollectionIsExplicitAndIndependent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy *pb.PearlPolicy
		want   bool
	}{
		{"none", nil, false},
		{"default", DefaultPolicy().Basic.Pearl, false},
		{"only hire", &pb.PearlPolicy{AutoHireEnabled: true}, false},
		{"only free", &pb.PearlPolicy{FreeEnabled: true}, false},
		{"only draw", &pb.PearlPolicy{DrawEnabled: true}, false},
		{"only protection", &pb.PearlPolicy{ProtectEnabled: true}, false},
		{"only ticket purchase", &pb.PearlPolicy{AutoBuyHireTicket: true}, false},
		{"all other switches", &pb.PearlPolicy{AutoHireEnabled: true, FreeEnabled: true, DrawEnabled: true, ProtectEnabled: true, AutoBuyHireTicket: true}, false},
		{"only collection", &pb.PearlPolicy{CollectEnabled: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := state.New()
			st.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{"1": map[string]any{
				"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0,
			}}}})
			found := false
			for _, op := range pearlOperations(st, tc.policy, time.UnixMilli(180_000)) {
				found = found || op.Kind == clientproto.RPCPearlPlaceRecvOneKey.String()
			}
			if found != tc.want {
				t.Fatalf("collection=%v want %v", found, tc.want)
			}
		})
	}
	st := state.New()
	ops := pearlOperations(st, &pb.PearlPolicy{CollectEnabled: true}, time.Now())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCPearlRefresh.String() {
		t.Fatalf("collection-only policy must still synchronize unobserved state: %+v", ops)
	}
}

func TestPearlCollectInterval(t *testing.T) {
	for _, tc := range []struct{ in, want int32 }{{-1, 300}, {0, 300}, {1, 60}, {59, 60}, {60, 60}, {300, 300}, {900, 900}, {3600, 3600}, {999999, 3600}} {
		if got := PearlCollectInterval(&pb.PearlPolicy{CollectIntervalSeconds: tc.in}); got != time.Duration(tc.want)*time.Second {
			t.Errorf("%d: got %s want %d seconds", tc.in, got, tc.want)
		}
	}
}
