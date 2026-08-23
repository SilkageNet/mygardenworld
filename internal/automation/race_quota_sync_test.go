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

func TestUnionRacePlansUsrRankForScoreWhenIdle(t *testing.T) {
	s := state.New()
	// Quota observed via 110, pool observed, nothing to take/finish — still need score/rank.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":99}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000000},"117":{"5":4},"110":{"42":{"3":0}},"114":[{"0":1,"4":4001,"6":[23001],"10":9,"12":1,"14":0,"15":0}]}}`))
	got := s.FmlRace()
	if !got.TaskQuotaObserved || !got.TasksObserved || got.ScoreObserved || got.RankObserved {
		t.Fatalf("precondition: %+v", got)
	}
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: true, MinTaskScore: 20}
	ops := unionRaceOperations(s, policy, 99, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String() {
		t.Fatalf("expected score/rank sync when idle, got %+v", ops)
	}
	if ops[0].Reason != "公会竞赛同步个人得分与排名" {
		t.Fatalf("reason = %q", ops[0].Reason)
	}
}
