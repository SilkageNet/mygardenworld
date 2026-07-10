package state

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
)

func TestCatalogInvariantStoryReferencesAndDecodedCost(t *testing.T) {
	chapters, ok := StaticTableByName("c_storyMainChapter")
	if !ok {
		t.Fatal("StaticTableByName(c_storyMainChapter) ok=false")
	}
	for idText, raw := range chapters.Rows {
		id, err := strconv.ParseInt(idText, 10, 32)
		if err != nil {
			t.Fatalf("c_storyMainChapter row key %q is not an integer: %v", idText, err)
		}
		if id <= 0 {
			continue
		}
		var chapter struct {
			SectionIDs []int32 `json:"sectionId"`
		}
		if err := json.Unmarshal(raw, &chapter); err != nil {
			t.Fatalf("decode c_storyMainChapter/%s: %v", idText, err)
		}
		for _, sectionID := range chapter.SectionIDs {
			if _, ok := StaticRow("c_storyMainSection", sectionID); !ok {
				t.Errorf("c_storyMainChapter/%s references missing c_storyMainSection/%d", idText, sectionID)
			}
		}
	}

	chapter32 := mustCatalogInvariantRow[struct {
		SectionIDs []int32 `json:"sectionId"`
	}](t, "c_storyMainChapter", 32)
	if len(chapter32.SectionIDs) == 0 || chapter32.SectionIDs[0] != 4101 {
		t.Fatalf("c_storyMainChapter/32 sectionId=%v, want first section 4101", chapter32.SectionIDs)
	}

	section4101 := mustCatalogInvariantRow[struct {
		Cost [][]int32 `json:"cost"`
	}](t, "c_storyMainSection", 4101)
	wantCost := [][]int32{{56, 85}}
	if !reflect.DeepEqual(section4101.Cost, wantCost) {
		t.Fatalf("c_storyMainSection/4101 cost=%v, want %v", section4101.Cost, wantCost)
	}
}

func TestCatalogInvariantMainTaskChain(t *testing.T) {
	table, ok := StaticTableByName("c_task_main")
	if !ok {
		t.Fatal("StaticTableByName(c_task_main) ok=false")
	}
	type taskRow struct {
		ID     int32  `json:"id"`
		NextID *int32 `json:"nextId"`
		Value  int32  `json:"value"`
	}
	nodes := make(map[int32]taskRow, len(table.Rows))
	for idText, raw := range table.Rows {
		id, err := strconv.ParseInt(idText, 10, 32)
		if err != nil {
			t.Fatalf("c_task_main row key %q is not an integer: %v", idText, err)
		}
		if id <= 0 {
			continue
		}
		var row taskRow
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatalf("decode c_task_main/%s: %v", idText, err)
		}
		if row.ID != int32(id) {
			t.Fatalf("c_task_main/%s decoded id=%d", idText, row.ID)
		}
		nodes[row.ID] = row
	}

	meta := mustCatalogInvariantRow[struct {
		InitID int32 `json:"$initId"`
		EndID  int32 `json:"$endId"`
	}](t, "c_task_main", -1)
	if _, ok := nodes[meta.InitID]; meta.InitID <= 0 || !ok {
		t.Fatalf("c_task_main $initId=%d does not reference a task row", meta.InitID)
	}
	if _, ok := nodes[meta.EndID]; meta.EndID <= 0 || !ok {
		t.Fatalf("c_task_main $endId=%d does not reference a task row", meta.EndID)
	}

	for id, row := range nodes {
		if row.NextID == nil || *row.NextID == 0 {
			continue
		}
		if _, ok := nodes[*row.NextID]; !ok {
			t.Errorf("c_task_main/%d nextId=%d references a missing task", id, *row.NextID)
		}
	}

	colors := make(map[int32]uint8, len(nodes))
	var visit func(int32)
	visit = func(id int32) {
		switch colors[id] {
		case 1:
			t.Fatalf("c_task_main chain contains a cycle at task %d", id)
		case 2:
			return
		}
		colors[id] = 1
		row := nodes[id]
		if row.NextID != nil && *row.NextID != 0 {
			if _, ok := nodes[*row.NextID]; ok {
				visit(*row.NextID)
			}
		}
		colors[id] = 2
	}
	for id := range nodes {
		visit(id)
	}

	foundEnd := false
	seen := make(map[int32]bool, len(nodes))
	for id := meta.InitID; id != 0 && !seen[id]; {
		seen[id] = true
		if id == meta.EndID {
			foundEnd = true
		}
		row, ok := nodes[id]
		if !ok || row.NextID == nil {
			break
		}
		id = *row.NextID
	}
	if !foundEnd {
		t.Fatalf("c_task_main $endId=%d is not reachable from $initId=%d", meta.EndID, meta.InitID)
	}

	assertTask := func(id, wantNextID, wantValue int32) {
		t.Helper()
		row, ok := nodes[id]
		if !ok {
			t.Fatalf("missing c_task_main/%d", id)
		}
		if row.NextID == nil || *row.NextID != wantNextID || row.Value != wantValue {
			t.Fatalf("c_task_main/%d nextId/value=(%v,%d), want (%d,%d)", id, row.NextID, row.Value, wantNextID, wantValue)
		}
	}
	assertTask(910001, 920001, 14)
	assertTask(920001, 930001, 24)
}

func TestCatalogInvariantCyclicNoteValues(t *testing.T) {
	type row struct {
		Value      int32     `json:"value"`
		Reward     [][]int32 `json:"reward"`
		FinishCost [][]int32 `json:"finishCost"`
	}
	task4003 := mustCatalogInvariantRow[row](t, "c_actCyclicNote", 4003)
	if task4003.Value != 80 || !reflect.DeepEqual(task4003.Reward, [][]int32{{1107, 4}}) || !reflect.DeepEqual(task4003.FinishCost, [][]int32{{1, 36}}) {
		t.Fatalf("c_actCyclicNote/4003=%+v, want value=80 reward=[[1107 4]] finishCost=[[1 36]]", task4003)
	}
	task1006 := mustCatalogInvariantRow[row](t, "c_actCyclicNote", 1006)
	if task1006.Value != 3 {
		t.Fatalf("c_actCyclicNote/1006 value=%d, want 3", task1006.Value)
	}
}

func TestCatalogInvariantPearlAndZooConstants(t *testing.T) {
	pearl := mustCatalogInvariantRow[struct {
		HireItem int32 `json:"$hireItem"`
		HireTime int32 `json:"$hireTime"`
		RestTime int32 `json:"$restTime"`
	}](t, "c_pearl", -1)
	if pearl.HireItem != 1003 || pearl.HireTime != 7200 || pearl.RestTime != 3600 {
		t.Fatalf("c_pearl/-1=%+v, want hireItem=1003 hireTime=7200 restTime=3600", pearl)
	}
	pearlEvent := mustCatalogInvariantRow[struct {
		GatherCD int32 `json:"$gatherCd"`
	}](t, "c_pearlEvent", -1)
	if pearlEvent.GatherCD != 180 {
		t.Fatalf("c_pearlEvent/-1 gatherCd=%d, want 180", pearlEvent.GatherCD)
	}
	zoo := mustCatalogInvariantRow[struct {
		CatBasinMax int32 `json:"$catBasinMax"`
	}](t, "c_zoo", -1)
	if zoo.CatBasinMax != 30 {
		t.Fatalf("c_zoo/-1 catBasinMax=%d, want 30", zoo.CatBasinMax)
	}
}

func mustCatalogInvariantRow[T any](t *testing.T, table string, id int32) T {
	t.Helper()
	var out T
	raw, ok := StaticRow(table, id)
	if !ok {
		t.Fatalf("StaticRow(%s, %d) ok=false", table, id)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s/%d: %v", table, id, err)
	}
	return out
}
