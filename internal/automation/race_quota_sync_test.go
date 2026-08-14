package automation

import (
	"encoding/json"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestUnionRacePlansUsrRankWhenQuotaUnobserved(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1786291200000,"1":1,"2":1786410000000,"3":1786885200000},"112":[{"0":1786291200000,"1":82665,"5":4}],"0":{"0":82665,"103":4}}}`))
	v := s.FmlRace()
	if !v.BatchActive || v.BatchID == 0 || v.TaskQuotaObserved {
		t.Fatalf("precondition failed: %+v", v)
	}
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: true, MinTaskScore: 20}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String() {
		t.Fatalf("expected usr rank sync, got %+v", ops)
	}
	if ops[0].TaskMsID != v.BatchID {
		t.Fatalf("batchId on op = %d, want %d", ops[0].TaskMsID, v.BatchID)
	}
}
