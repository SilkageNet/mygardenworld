package state

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

const dessertFixtureNowMs int64 = 1783819000000

func TestDessertSanitizedCaptureFixture(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	view, ok := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || !view.Observed || !view.Found || !view.Valid {
		t.Fatalf("DessertView=(%+v,%t), want observed valid activity", view, ok)
	}
	if view.BatchID != 9101 || view.TmpID != 56019999 || view.TmpType != 5601 || view.Phase != 2 ||
		view.DropCount != 100 || !view.DropCountObserved || view.TotalScore != 2220 || !view.TotalScoreObserved {
		t.Fatalf("identity/progress=%+v", view)
	}
	if !view.BagObserved || view.EnergyBalance != 100 || view.CurrencyBalance != 217 || view.RewardBoxBalance != 13 ||
		view.EnergyItemID != 1342 || view.CurrencyItemID != 1343 || view.PointItemID != 1344 || view.RewardBoxItemID != 1347 {
		t.Fatalf("activity bag=%+v", view)
	}
	if len(view.Modes) != 5 || !view.Modes[0].Observed || !view.Modes[0].Valid || view.Modes[0].ObjectCount != 1 ||
		view.Modes[0].LevelCounts[1] != 1 || len(view.Modes[0].Objects) != 1 || len(view.Modes[0].Objects[0].Raw) == 0 ||
		view.Modes[1].Multiplier != 5 || view.Modes[4].UnlockScore != 160000 {
		t.Fatalf("modes=%+v", view.Modes)
	}
	if !view.TaskGroupsObserved || !view.TaskGroupsValid || !view.TaskRecordObserved || len(view.Tasks) != 6 {
		t.Fatalf("tasks=%+v", view.Tasks)
	}
	for index, task := range view.Tasks {
		want := int32(index + 1)
		if task.TaskIndex != 0 || task.Position != want || task.TaskID != want || task.TaskType != 18 || task.Target != want ||
			task.HasParam || !task.CatalogKnown || !task.ProgressObserved || task.Progress != 3 || !task.ReceiptObserved ||
			task.Received || !reflect.DeepEqual(task.Reward, []ItemCount{{ItemID: 1342, Count: 100}}) {
			t.Fatalf("task[%d]=%+v", index, task)
		}
	}
	if len(view.Milestones) != 4 || view.Milestones[0].Target != 600 || view.Milestones[3].Target != 2400 {
		t.Fatalf("milestones=%+v", view.Milestones)
	}
	if !view.Celebrity.Valid || !view.Celebrity.TypeListed || !view.Celebrity.RankingObserved ||
		view.Celebrity.RankingCount != 1 || !view.Celebrity.LikesObserved || view.Celebrity.LikedThisBatch {
		t.Fatalf("celebrity=%+v", view.Celebrity)
	}
}

func TestDessertSparseDeltaAndClaimProgressReplacement(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	before, _ := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"11":101,"14":{"121":{"0":2225}}}}}}`))
	view, ok := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || !view.Valid || view.DropCount != 101 || view.TotalScore != 2225 || len(view.Modes) != 5 ||
		view.EnergyBalance != 100 || len(view.Tasks) != 6 || view.AuthorityRevision != before.AuthorityRevision ||
		view.BoardHash != before.BoardHash {
		t.Fatalf("sparse delta lost state: (%+v,%t)", view, ok)
	}

	// Captured act.recv semantics: progress is a whole replacement that drops
	// the claimed task, while receipt is authoritative.
	s.ApplyV(json.RawMessage(`{"23":{"3":{"9101|0":{"3":{"2":3,"3":3,"4":3,"5":3,"6":3},"5":{"1":1},"7":1783819865327}}}}`))
	view, ok = s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || !view.Valid || view.Tasks[0].ProgressObserved || !view.Tasks[0].ReceiptObserved || !view.Tasks[0].Received ||
		!view.Tasks[1].ProgressObserved || view.Tasks[1].Progress != 3 || view.Tasks[1].Received {
		t.Fatalf("claim replacement=%+v", view.Tasks)
	}
}

func TestDessertAuthorityRevisionIncludesRejectedBoardReplacements(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	now := time.UnixMilli(dessertFixtureNowMs)
	initial, ok := s.DessertView(now)
	if !ok || initial.AuthorityRevision == 0 || initial.BoardHash == "" || initial.Modes[0].BoardHash == "" ||
		initial.Modes[0].AuthorityRevision != initial.AuthorityRevision {
		t.Fatalf("initial authority metadata=%+v mode=%+v", initial, initial.Modes[0])
	}
	floors := s.DessertAuthorityRevisions()
	if floors[9101] != initial.AuthorityRevision {
		t.Fatalf("authority revision floors=%v, view=%d", floors, initial.AuthorityRevision)
	}
	floors[9101] = 999
	if got := s.DessertAuthorityRevisions()[9101]; got != initial.AuthorityRevision {
		t.Fatalf("authority revision floor mutation leaked: %d", got)
	}

	// A score-only sparse delta is not a board authority change.
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"0":2226}}}}}}`))
	scoreOnly, _ := s.DessertView(now)
	if scoreOnly.AuthorityRevision != initial.AuthorityRevision || scoreOnly.BoardHash != initial.BoardHash {
		t.Fatalf("score-only delta changed board authority: before=%+v after=%+v", initial, scoreOnly)
	}

	// An enclosing extension replacement without ext121 field 1 makes the
	// view non-executable, but cannot invent a new board hash at the old
	// revision.
	enclosing := applyDessertCaptureFixture(t)
	enclosing.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":null}}}}`))
	enclosingView, _ := enclosing.DessertView(now)
	if enclosingView.Valid || enclosingView.AuthorityRevision != initial.AuthorityRevision ||
		enclosingView.BoardHash != initial.BoardHash {
		t.Fatalf("enclosing null changed board authority without field 1: %+v", enclosingView)
	}

	// Explicit null is a valid observed-empty whole replacement.
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":null}}}}}}`))
	nullBoard, _ := s.DessertView(now)
	if nullBoard.AuthorityRevision != initial.AuthorityRevision+1 || !nullBoard.ModeMapValid ||
		nullBoard.BoardHash == "" || nullBoard.BoardHash == initial.BoardHash {
		t.Fatalf("null board authority=%+v", nullBoard)
	}

	// A malformed present field still supersedes the prior board and advances
	// the revision, while keeping the resulting view non-executable.
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":[]}}}}}}`))
	malformed, _ := s.DessertView(now)
	if malformed.AuthorityRevision != initial.AuthorityRevision+2 || malformed.ModeMapValid || malformed.Valid ||
		malformed.BoardHash != nullBoard.BoardHash {
		t.Fatalf("malformed board authority=%+v", malformed)
	}
}

func TestDessertCanonicalTypedHashSortsMapsPreservesObjectOrderAndIgnoresRaw(t *testing.T) {
	objectA := DessertObjectView{
		Raw: json.RawMessage(`{"debug":"first"}`), Level: 1, Position: DessertVector2{X: 1, Y: 2},
		LinearVelocity: DessertVector2{X: 3, Y: 4}, Scale: DessertVector3{X: 1, Y: 1, Z: 1}, IsAwake: true,
	}
	objectB := objectA
	objectB.Raw = json.RawMessage(`{"debug":"second"}`)
	objectB.Level = 2
	base := &dessertModeState{
		Step: 3, ItemUse: map[int32]int32{2: 20, 1: 10}, Objects: []DessertObjectView{objectA, objectB},
		GameStatus: 1, FirstMerge: map[int32]int32{3: 30, 1: 10}, IsRunning: true,
		TotalGain: map[int32]int32{1347: 1, 1343: 2}, CurID: 2, Score: 40, LevelMap: map[int32]int32{2: 1, 1: 1},
	}
	reorderedMaps := *base
	reorderedMaps.ItemUse = map[int32]int32{1: 10, 2: 20}
	reorderedMaps.FirstMerge = map[int32]int32{1: 10, 3: 30}
	reorderedMaps.TotalGain = map[int32]int32{1343: 2, 1347: 1}
	reorderedMaps.LevelMap = map[int32]int32{1: 1, 2: 1}
	reorderedMaps.Objects = append([]DessertObjectView(nil), base.Objects...)
	reorderedMaps.Objects[0].Raw = json.RawMessage(`{"completely":"different"}`)

	first := canonicalDessertModeHash(1, base)
	second := canonicalDessertModeHash(1, &reorderedMaps)
	if first == "" || first != second {
		t.Fatalf("canonical typed hashes differ for equivalent maps/raw: %q != %q", first, second)
	}

	reversed := reorderedMaps
	reversed.Objects = []DessertObjectView{objectB, objectA}
	if got := canonicalDessertModeHash(1, &reversed); got == first {
		t.Fatalf("physical server array order was erased from hash: %q", got)
	}
}

func TestDessertModeMapReplacementAndMalformedFailClosed(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	// One structurally valid mode replaces the authoritative five-mode map;
	// stale modes must not survive to make the view executable.
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":{"1":{"0":0,"1":{},"2":[],"3":0,"4":{},"5":false,"6":{},"7":1,"8":0,"9":{}}}}}}}}}`))
	view, ok := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || view.Valid || !view.ModeMapValid || len(s.activityBatches[9101].Dessert.Modes) != 1 ||
		view.Modes[0].Observed == false || view.Modes[1].Observed {
		t.Fatalf("whole mode replacement=(%+v,%t), state=%+v", view.Modes, ok, s.activityBatches[9101].Dessert)
	}

	// A present malformed authoritative map replaces the prior one and
	// clears executable mode state instead of falling back to stale values.
	s = applyDessertCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":{"1":{"0":0,"1":{},"2":[],"3":0,"4":{},"5":1,"6":{},"7":1,"8":0,"9":{}}}}}}}}}`))
	view, ok = s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || view.Valid || view.ModeMapValid || view.ExtensionValid || len(s.activityBatches[9101].Dessert.Modes) != 0 {
		t.Fatalf("malformed map retained executable state: (%+v,%t)", view, ok)
	}

	for name, raw := range map[string]json.RawMessage{
		"mode map is array":           json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":[]}}}}}}`),
		"item use is scalar":          json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":{"1":{"0":0,"1":1,"2":[],"3":0,"4":{},"5":false,"6":{},"7":1,"8":0,"9":{}}}}}}}}}`),
		"object coordinate is string": json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":{"1":{"0":0,"1":{},"2":[{"lv":1,"isSyn":false,"pos":{"x":"NaN","y":0},"linearVelocity":{"x":0,"y":0},"angularVelocity":0,"scale":{"x":1,"y":1,"z":1},"nodeAngle":0,"isAwake":true,"_lineTime":0}],"3":0,"4":{},"5":false,"6":{},"7":1,"8":0,"9":{}}}}}}}}}`),
	} {
		t.Run(name, func(t *testing.T) {
			s := applyDessertCaptureFixture(t)
			s.ApplyV(raw)
			view, ok := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
			if !ok || view.Valid || view.ModeMapValid || view.ExtensionValid || len(s.activityBatches[9101].Dessert.Modes) != 0 {
				t.Fatalf("malformed authoritative shape retained executable state: (%+v,%t)", view, ok)
			}
		})
	}
}

func TestDessertTemplateTaskGroupReplacementFailsClosed(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"1":{"56019999":{"6":{}}}}}`))
	view, ok := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || view.Valid || view.TaskGroupsValid || len(view.Tasks) != 0 || len(s.activityTemplates[56019999].TaskGroups) != 0 {
		t.Fatalf("malformed task replacement=(%+v,%t), template=%+v", view, ok, s.activityTemplates[56019999])
	}
}

func TestDessertInitialMissingTotalScoreMeansObservedZero(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	batch := s.activityBatches[9101]
	batch.Dessert = dessertActivityState{}
	mergeDessertExtension(batch, json.RawMessage(`{"121":{"1":{"1":{"0":0,"1":{},"2":[],"3":0,"4":{},"5":false,"6":{},"7":1,"8":0,"9":{}}}}}`))
	if !batch.Dessert.TotalScoreObserved || !batch.Dessert.TotalScoreValid || batch.Dessert.TotalScore != 0 {
		t.Fatalf("initial total score=%+v", batch.Dessert)
	}
	mergeDessertExtension(batch, json.RawMessage(`{"121":{"0":10}}`))
	mergeDessertExtension(batch, json.RawMessage(`{"121":{"1":{}}}`))
	if batch.Dessert.TotalScore != 10 {
		t.Fatalf("later sparse ext reset total score: %+v", batch.Dessert)
	}
}

func TestDessertCandidateSelectionAndDefensiveCopy(t *testing.T) {
	const nowMs int64 = 10000
	batch := func(id int32, begin, end, before, after int64) *activityBatchState {
		return &activityBatchState{BatchID: id, TmpType: dessertTmpType, Status: 1, BeginMs: begin, EndMs: end, DurationBeforeMs: before, DurationAfterMs: after}
	}
	s := New()
	s.activityBatches = map[int32]*activityBatchState{
		1: batch(1, 11000, 13000, 2000, 0),
		2: batch(2, 7000, 9000, 0, 2000),
		3: batch(3, 8000, 11000, 0, 0),
		4: batch(4, 9000, 12000, 0, 0),
		5: {BatchID: 5, TmpType: 5602, Status: 1, BeginMs: 9000, EndMs: 12000},
	}
	selected, phase, _, _, _ := s.preferredDessertBatchLocked(nowMs)
	if selected == nil || selected.BatchID != 4 || phase != 2 {
		t.Fatalf("selected=(%+v,%d)", selected, phase)
	}

	s = applyDessertCaptureFixture(t)
	first, ok := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if !ok || !first.Valid {
		t.Fatalf("first=(%+v,%t)", first, ok)
	}
	first.Bag[1342] = 999
	first.Modes[0].TotalGain[1343] = 999
	first.Modes[0].Objects[0].Raw[0] = '['
	first.Tasks[0].Reward[0].Count = 999
	first.Milestones[0].Reward[0].Count = 999
	again, _ := s.DessertView(time.UnixMilli(dessertFixtureNowMs))
	if again.Bag[1342] != 100 || again.Modes[0].TotalGain[1343] != 217 || again.Modes[0].Objects[0].Raw[0] != '{' ||
		again.Tasks[0].Reward[0].Count != 100 || again.Milestones[0].Reward[0].Count != 20 {
		t.Fatalf("view mutation leaked: %+v", again)
	}
}

func TestDessertActionSnapshotsRequireExactFreeContracts(t *testing.T) {
	now := time.UnixMilli(dessertFixtureNowMs)
	s := applyDessertCaptureFixture(t)

	claim, ok := s.DessertTaskClaimSnapshot(now, 9101, 0, 1)
	if !ok || claim.TaskID != 1 || claim.Target != 1 || claim.Progress != 3 || claim.EnergyItemID != 1342 ||
		claim.EnergyBefore != 100 || claim.RewardCount != 100 {
		t.Fatalf("task claim snapshot=(%+v,%t)", claim, ok)
	}
	for _, target := range []struct {
		index int32
		id    int32
	}{{1, 1}, {0, 0}, {0, 99}} {
		if got, ready := s.DessertTaskClaimSnapshot(now, 9101, target.index, target.id); ready {
			t.Fatalf("unsafe task target (%d,%d) ready: %+v", target.index, target.id, got)
		}
	}

	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"12":{"1342":200,"1343":217,"1347":13}}},"3":{"9101|0":{"3":{"2":3,"3":3,"4":3,"5":3,"6":3},"5":{"1":1}}}}}`))
	if !s.DessertTaskClaimApplied(claim) {
		t.Fatal("task receipt plus exact activity energy reward was not accepted")
	}
	if _, ready := s.DessertTaskClaimSnapshot(now, 9101, 0, 1); ready {
		t.Fatal("received task remained executable")
	}
}

func TestDessertControlledCelebritySyncAndLikeAreSessionScoped(t *testing.T) {
	now := time.UnixMilli(dessertFixtureNowMs)
	s := applyDessertCaptureFixture(t)
	syncBefore, ok := s.DessertCelebritySyncSnapshot(now)
	if !ok || syncBefore.BatchID != 9101 || !s.DessertCelebritySyncApplied(syncBefore) {
		t.Fatalf("controlled sync snapshot=(%+v,%t)", syncBefore, ok)
	}
	s.MarkDessertCelebritySynced(9101)
	if !s.DessertCelebritySynced(9101) {
		t.Fatal("successful controlled sync marker missing")
	}
	if _, ready := s.DessertCelebritySyncSnapshot(now); ready {
		t.Fatal("same batch requested a second controlled sync")
	}

	likeBefore, ok := s.DessertCelebrityLikeSnapshot(now, 9101)
	if !ok || likeBefore.EnergyBefore != 100 || likeBefore.ExpectedReward != 20 || likeBefore.BatchBeginMs <= 0 {
		t.Fatalf("like snapshot=(%+v,%t)", likeBefore, ok)
	}
	likeAt := likeBefore.BatchBeginMs + 1
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"12":{"1342":120,"1343":217,"1347":13}}}},"166":{"2":{"5601":{"0":700001,"1":5601,"2":` + strconv.FormatInt(likeAt, 10) + `,"3":` + strconv.FormatInt(likeAt, 10) + `}}}}`))
	if !s.DessertCelebrityLikeApplied(likeBefore) {
		t.Fatal("like timestamp plus exact activity energy reward was not accepted")
	}
	if _, ready := s.DessertCelebrityLikeSnapshot(now, 9101); ready {
		t.Fatal("already-liked batch remained executable")
	}

	s.ResetDessertSession()
	if s.DessertCelebritySynced(9101) {
		t.Fatal("fresh-session reset retained controlled sync marker")
	}
}

func TestDessertEnterOnlyRepairsMissingNotMalformedState(t *testing.T) {
	now := time.UnixMilli(dessertFixtureNowMs)
	s := applyDessertCaptureFixture(t)
	s.mu.Lock()
	batch := s.activityBatches[9101]
	batch.BagObserved = false
	batch.BagValid = false
	s.mu.Unlock()
	before, ok := s.DessertEnterSnapshot(now)
	if !ok || before.BatchID != 9101 || before.Phase != 2 {
		t.Fatalf("missing bag enter=(%+v,%t)", before, ok)
	}

	s = applyDessertCaptureFixture(t)
	s.mu.Lock()
	s.activityBatches[9101].BagValid = false
	s.mu.Unlock()
	if got, ready := s.DessertEnterSnapshot(now); ready {
		t.Fatalf("malformed observed bag was probed with enter: %+v", got)
	}
}

func applyDessertCaptureFixture(t *testing.T) *State {
	t.Helper()
	raw, err := os.ReadFile("testdata/dessert_activity.json")
	if err != nil {
		t.Fatalf("read dessert fixture: %v", err)
	}
	s := New()
	s.ApplyV(raw)
	return s
}
