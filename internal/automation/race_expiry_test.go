package automation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestExpiredRaceRestoresDeletionAfterPoolConfirmation(t *testing.T) {
	for _, deltaOnly := range []bool{false, true} {
		s := state.New()
		applyRaceState(s, [][5]int32{{99, 3036, 7, 0, 0}, {100, 3036, 10, 0, 0}})
		s.ApplyV(json.RawMessage(`{"25":{"1":{"0":999,"1":42,"2":1},"110":{"42":{"7":{"0":99,"1":3036,"2":280,"3":0,"4":[23001],"5":1}}}}}`))
		p := racePlantPolicy(false)
		p.Union.Race.MinTaskScore = 30
		p.Union.Race.DeleteLowScoreTask = true
		p.Union.Race.DeleteTaskMaxScore = 10
		p.Plant.Planting.AutoEnabled = false
		now := time.Now()
		before := BuildPlan(s, p, now)
		foundSync := false
		for _, op := range before.Operations {
			if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
				foundSync = true
			}
		}
		if !foundSync {
			t.Fatalf("missing post-expiry confirmation: %+v", before.Operations)
		}
		if deltaOnly {
			s.NoteFmlRaceTaskPoolSync(now)
		} else {
			s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":99,"4":3036,"6":[23001],"7":280,"8":0,"10":7},{"0":100,"4":3036,"6":[23001],"10":10}]}}`))
		}
		after := BuildPlan(s, p, time.Now())
		foundDelete := false
		for _, op := range after.Operations {
			if op.Kind == clientproto.RPCFmlRaceDelTask.String() && op.Executable {
				foundDelete = true
			}
			if op.Kind == clientproto.RPCFmlRaceFinishTask.String() || op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() || op.RaceHoldTaskMsID != 0 {
				t.Fatalf("expired hold still drives work: %+v", op)
			}
		}
		if !foundDelete {
			t.Fatalf("deletion still blocked: %+v", after.Operations)
		}
	}
}

func TestExpiredRaceFarmFollowsOrdinaryPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := raceTakenPlantState(t, 0, 10)
		s.ApplyV(json.RawMessage(`{"25":{"110":{"1783872000000":{"7":{"0":99,"1":4001,"2":10,"3":0,"4":[23001],"5":1}}}}}`))
		p := racePlantPolicy(true)
		p.Plant.Planting.AutoEnabled = enabled
		plan := BuildPlan(s, p, time.UnixMilli(1500000))
		plant := false
		for _, op := range plan.Operations {
			if isPlantOperation(op.Kind) && op.Executable {
				plant = true
				if op.GoalID == raceActionGoal {
					t.Fatal("expired race demand")
				}
			}
		}
		if plant != enabled {
			t.Fatalf("ordinary=%t plant=%t", enabled, plant)
		}
	}
}

func TestRacePlanBindsHeldWorkToTask(t *testing.T) {
	s := raceTakenPlantState(t, 0, 10)
	p := racePlantPolicy(false)
	plan := BuildPlan(s, p, time.UnixMilli(1500000))
	found := false
	for _, op := range plan.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal {
			found = true
			if op.RaceHoldTaskMsID != 99 || op.RaceBatchID == 0 {
				t.Fatalf("missing held-task guard: %+v", op)
			}
		}
	}
	if !found {
		t.Fatal("missing race planting")
	}
}

func TestExpiredRaceTakeRequiresConfirmationAndPreservesPoolOwnership(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{99, 3036, 7, 0, 0}, {100, 3036, 28, 0, 0}})
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"7":{"0":99,"1":3036,"2":280,"4":[23001],"5":1}}}}}`))
	p := racePlantPolicy(false)
	p.Union.Race.TaskTypePriority = map[int32]int32{3036: 5}
	if _, err := ManualRaceTakeOperation(s, p, 100, time.Now()); err == nil {
		t.Fatal("take admitted before post-expiry sync")
	}
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":99,"4":3036,"6":[23001],"10":7,"12":999,"13":1},{"0":100,"4":3036,"6":[23001],"10":28}]}}`))
	if _, err := ManualRaceTakeOperation(s, p, 100, time.Now()); err != nil {
		t.Fatal("fresh empty candidate blocked by historical hold", err)
	}
	if _, err := ManualRaceTakeOperation(s, p, 99, time.Now()); err == nil {
		t.Fatal("expiry incorrectly freed an occupied pool row")
	}
}
