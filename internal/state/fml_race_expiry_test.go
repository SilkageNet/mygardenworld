package state

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestFmlRaceEffectiveExpiryBoundary(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deadline int64
		active   bool
	}{
		{"unknown", 0, true}, {"before", 1001, true}, {"at", 1000, false}, {"after", 999, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := FmlRaceView{Taken: FmlRaceTakenView{HasTask: true, TaskMsId: 99, ExpireTime: tc.deadline}, LocalFinishCnt: 7, LocalFinishTaskMsId: 99,
				Tasks: []FmlRaceTaskView{{MsId: 99, UID: 123}}}
			got := v.EffectiveAt(time.UnixMilli(1000))
			if got.Taken.HasTask != tc.active || (!tc.active && got.LocalFinishCnt != 0) {
				t.Fatalf("effective=%+v", got)
			}
			if !v.Taken.HasTask || v.LocalFinishCnt != 7 || got.Tasks[0].UID != 123 {
				t.Fatal("projection destroyed evidence or freed pool ownership")
			}
		})
	}
}

func TestFmlRaceExpiredSnapshotsNeverDriveLocalProgress(t *testing.T) {
	// Fresh State also models reconnect / deleting and re-adding an account.
	for _, uid := range []int64{0, 999} {
		t.Run(fmt.Sprint(uid), func(t *testing.T) {
			s := New()
			s.ApplyV(json.RawMessage(fmt.Sprintf(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1},"110":{"42":{"7":{"0":99,"1":3028,"2":3,"3":0,"4":[23001],"5":1}}},"114":[{"0":99,"4":3028,"6":[23001],"7":3,"8":0,"12":%d}]},"101":{"0":{"23001":{"1":23001,"2":11,"4":2}}},"100":{"1":{"1001":{"0":23001,"1":2,"2":11,"3":0}}}}`, uid)))
			for cycle := 1; cycle <= 2; cycle++ {
				s.ApplyV(json.RawMessage(fmt.Sprintf(`{"100":{"1":{"1001":{"0":23001,"1":2,"2":11,"3":%d}}}}`, cycle)))
				s.ApplyVFullFmlRaceTaskPool(json.RawMessage(fmt.Sprintf(`{"25":{"114":[{"0":99,"4":3028,"6":[23001],"7":3,"8":0,"12":%d}]}}`, uid)))
				s.ApplyV(json.RawMessage(`{"25":{"134":{"42":{"3":{"0":99,"1":3028,"2":3,"3":3,"4":[23001],"5":1}}}}}`))
				if v := s.FmlRaceAt(time.Now()); v.Taken.HasTask || v.LocalFinishCnt != 0 {
					t.Fatalf("cycle %d: stale task revived: %+v", cycle, v)
				}
				if v := s.FmlRace(); v.LocalFinishCnt != 0 || v.Taken.ExpireTime != 1 {
					t.Fatalf("expired progress accumulated or deadline lost: %+v", v)
				}
			}
			// A genuinely new hold remains usable after the old one expired.
			s.ApplyV(json.RawMessage(fmt.Sprintf(`{"25":{"134":{"42":{"3":{"0":100,"1":3028,"2":3,"3":0,"4":[23001],"5":%d}}}}}`, time.Now().Add(time.Hour).UnixMilli())))
			if v := s.FmlRaceAt(time.Now()); !v.Taken.HasTask || v.Taken.TaskMsId != 100 {
				t.Fatalf("new hold lost: %+v", v)
			}
		})
	}
}
