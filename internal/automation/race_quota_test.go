package automation

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestPurchasedRaceQuotaSharedByEveryTakePath(t *testing.T) {
	for _, finished := range []int{17, 18, 19, 20} {
		t.Run(fmt.Sprint(finished), func(t *testing.T) {
			s := state.New()
			applyRaceState(s, [][5]int32{{1, 3036, 28, 0, 0}})
			s.ApplyV(json.RawMessage(fmt.Sprintf(`{"25":{"110":{"42":{"3":%d,"6":2}}}}`, finished)))
			p := testEnabledRaceFullPolicy()
			p.Union.Race.AutoStopOnQuotaDone = true
			now := time.Now()
			wantTake := finished < 20
			op, err := ManualRaceTakeOperation(s, p, 1, now)
			if (err == nil) != wantTake {
				t.Fatalf("manual take: %v", err)
			}
			if wantTake {
				if err := ValidateRaceTaskMutation(s, p, &op, now); err != nil {
					t.Fatal(err)
				}
			}
			hasTake := false
			for _, planned := range unionRaceOperations(s, p.Union.Race, s.RoleID(), now, raceGatesOn()) {
				hasTake = hasTake || planned.Kind == clientproto.RPCFmlRaceTakeTask.String()
				if planned.Kind == clientproto.RPCFmlRaceBuyTaskNum.String() {
					t.Fatal("using purchased slots planned a purchase")
				}
			}
			if hasTake != wantTake {
				t.Fatalf("automatic take=%v want=%v", hasTake, wantTake)
			}
			if wantTake {
				s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":20,"6":2}}}}`))
				if ValidateRaceTaskMutation(s, p, &op, now) == nil {
					t.Fatal("queued take bypassed newly exhausted quota")
				}
			}
		})
	}
}

func TestExhaustedRaceQuotaRefreshIsBoundedAndRecovers(t *testing.T) {
	for _, serverRejected := range []bool{false, true} {
		t.Run(fmt.Sprint(serverRejected), func(t *testing.T) {
			s := state.New()
			applyRaceState(s, [][5]int32{{1, 3036, 28, 0, 0}})
			s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":18,"6":0}}}}`))
			p := testEnabledRaceFullPolicy()
			// Even with local quota stopping disabled, server rejections must
			// remain recoverable without repeated takeTask probes.
			p.Union.Race.AutoStopOnQuotaDone = !serverRejected
			if serverRejected {
				s.MarkFmlRaceTakeQuotaExhausted()
			}
			now := time.UnixMilli(s.FmlRace().RaceQuotaSyncAtMs).Add(10 * time.Minute)
			ops := unionRaceOperations(s, p.Union.Race, s.RoleID(), now, raceGatesOn())
			if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String() || !ops[0].PreemptFarm {
				t.Fatalf("quota refresh starved: %+v", ops)
			}
			// Failed or successful runner attempts share this timestamp.
			s.MarkFmlRaceQuotaSyncAttempt()
			at := time.UnixMilli(s.FmlRace().RaceQuotaSyncAtMs)
			for _, dt := range []time.Duration{time.Second, 10*time.Minute - time.Millisecond} {
				for _, op := range unionRaceOperations(s, p.Union.Race, s.RoleID(), at.Add(dt), raceGatesOn()) {
					if op.Kind == clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String() || op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
						t.Fatalf("quota sync retried too soon or probed take: %+v", op)
					}
				}
			}
			ops = unionRaceOperations(s, p.Union.Race, s.RoleID(), at.Add(10*time.Minute), raceGatesOn())
			if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String() {
				t.Fatalf("quota refresh never retried: %+v", ops)
			}
			s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"6":2}}}}`))
			ops = unionRaceOperations(s, p.Union.Race, s.RoleID(), time.Now(), raceGatesOn())
			if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
				t.Fatalf("purchased slots did not resume take: %+v", ops)
			}
		})
	}
}

func TestRaceQuotaIsRecheckedBeforeTakingAfterEachConnection(t *testing.T) {
	s := state.New()
	applyRaceState(s, nil)
	p := testEnabledRaceFullPolicy()
	for range 2 {
		s.BeginFmlMembershipSnapshot()
		s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 28, 0, 0}})))
		s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":18,"6":2}}}}`))
		now := time.Now()
		ops := unionRaceOperations(s, p.Union.Race, s.RoleID(), now, raceGatesOn())
		if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String() {
			t.Fatalf("connection reused old quota sync evidence: %+v", ops)
		}
		s.MarkFmlRaceQuotaSyncAttempt()
		s.ApplyV(json.RawMessage(`{"25":{"116":[{"0":999,"1":42,"3":18,"6":2,"4":100}]}}`))
		ops = unionRaceOperations(s, p.Union.Race, s.RoleID(), now, raceGatesOn())
		if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("verified purchased quota did not allow take: %+v", ops)
		}
	}
}
