package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRaceFullEvidenceCannotBeRenewedByPushOrAcknowledgement(t *testing.T) {
	s := New()
	pool := json.RawMessage(`{"25":{"111":{"0":42,"1":1},"114":[{"0":1,"4":3036,"10":10}]}}`)
	s.ApplyV(pool)
	if s.FmlRace().FullTasksSyncedAtMs != 0 {
		t.Fatal("ordinary delta invented full-pool evidence")
	}
	s.ApplyVFullFmlRaceTaskPool(pool)
	fullAt := s.FmlRace().FullTasksSyncedAtMs
	if fullAt == 0 {
		t.Fatal("missing full-pool evidence")
	}
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"10":20}]}}`))
	s.NoteFmlRaceTaskPoolSync(time.Now().Add(time.Minute))
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":{}}}`))
	if s.FmlRace().FullTasksSyncedAtMs != fullAt {
		t.Fatal("partial/malformed response renewed evidence")
	}
	s.MarkFmlRaceTaskPoolStale()
	s.NoteFmlRaceTaskPoolSync(time.Now())
	if s.FmlRace().FullTasksSyncedAtMs != 0 {
		t.Fatal("empty acknowledgement restored stale evidence")
	}
	s.ApplyVFullFmlRaceTaskPool(pool)
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":43,"1":1}}}`))
	if s.FmlRace().FullTasksSyncedAtMs != 0 {
		t.Fatal("new batch retained evidence")
	}
}

func TestRaceChangeNotificationAndSnapshots(t *testing.T) {
	s := New()
	calls := 0
	s.SetOnRaceChange(func() { calls++; _ = s.FmlRace() }) // outside state lock
	pool := json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"10":10},{"0":2,"4":3036,"10":20}]}}`)
	s.ApplyVFullFmlRaceTaskPool(pool)
	if calls != 1 {
		t.Fatalf("initial notifications=%d", calls)
	}
	s.ApplyVFullFmlRaceTaskPool(pool)
	s.ApplyV(json.RawMessage(`{"7":{"1":5}}`))
	if calls != 1 {
		t.Fatal("identical pool/unrelated state caused wake storm")
	}
	previous := s.FmlRace()
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"10":30}]}}`))
	if calls != 2 || previous.Tasks[0].Score != 10 {
		t.Fatal("partial update missed or mutated prior snapshot")
	}
	previous.Tasks[1].Score = 999
	if s.FmlRace().Tasks[1].Score == 999 {
		t.Fatal("caller mutated authoritative state")
	}
}
