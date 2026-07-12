package state

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"time"
)

const dessertTmpType int32 = 5601

type dessertActivityState struct {
	ExtensionObserved  bool
	ExtensionValid     bool
	TotalScore         int32
	TotalScoreObserved bool
	TotalScoreValid    bool
	Modes              map[int32]*dessertModeState
	ModesObserved      bool
	ModesValid         bool
}

type dessertModeState struct {
	Step       int32
	ItemUse    map[int32]int32
	Objects    []DessertObjectView
	GameStatus int32
	FirstMerge map[int32]int32
	IsRunning  bool
	TotalGain  map[int32]int32
	CurID      int32
	Score      int32
	LevelMap   map[int32]int32
}

func mergeDessertExtension(batch *activityBatchState, raw json.RawMessage) {
	if batch == nil {
		return
	}
	if isJSONNull(raw) {
		batch.Dessert = dessertActivityState{ExtensionObserved: true, ExtensionValid: true}
		return
	}
	var ext map[string]json.RawMessage
	if json.Unmarshal(raw, &ext) != nil || ext == nil {
		batch.Dessert = dessertActivityState{ExtensionObserved: true, ExtensionValid: false}
		return
	}
	rawDessert, present := ext["121"]
	if !present {
		return
	}
	if isJSONNull(rawDessert) {
		batch.Dessert = dessertActivityState{ExtensionObserved: true, ExtensionValid: true}
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(rawDessert, &fields) != nil || fields == nil {
		batch.Dessert = dessertActivityState{ExtensionObserved: true, ExtensionValid: false}
		return
	}

	next := batch.Dessert
	firstObservation := !next.ExtensionObserved
	next.ExtensionObserved = true
	if firstObservation && fields["0"] == nil {
		// Observed enter/gameSync snapshots omit the numeric zero until the
		// first merge. The enclosing ext121 object is authoritative evidence
		// that the accumulated score is currently zero.
		next.TotalScore = 0
		next.TotalScoreObserved = true
		next.TotalScoreValid = true
	}
	if rawScore, present := fields["0"]; present {
		value, valid := readActivityInt32Raw(rawScore)
		next.TotalScoreObserved = true
		next.TotalScoreValid = valid && value >= 0
		if next.TotalScoreValid {
			next.TotalScore = value
		} else {
			next.TotalScore = 0
		}
	}
	if rawModes, present := fields["1"]; present {
		modes, valid := decodeDessertModeMap(rawModes)
		next.Modes = modes
		next.ModesObserved = true
		next.ModesValid = valid
	}
	next.ExtensionValid = (!next.TotalScoreObserved || next.TotalScoreValid) && (!next.ModesObserved || next.ModesValid)
	batch.Dessert = next
}

func decodeDessertModeMap(raw json.RawMessage) (map[int32]*dessertModeState, bool) {
	if isJSONNull(raw) {
		return make(map[int32]*dessertModeState), true
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil {
		return nil, false
	}
	out := make(map[int32]*dessertModeState, len(entries))
	valid := true
	for key, rawMode := range entries {
		mode64, err := strconv.ParseInt(key, 10, 32)
		if err != nil || mode64 <= 0 || strconv.FormatInt(mode64, 10) != key {
			valid = false
			continue
		}
		modeID := int32(mode64)
		if isJSONNull(rawMode) {
			continue
		}
		mode, modeValid := decodeDessertMode(rawMode)
		if !modeValid {
			valid = false
			continue
		}
		out[modeID] = mode
	}
	return out, valid
}

func decodeDessertMode(raw json.RawMessage) (*dessertModeState, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false
	}
	for index := 0; index <= 9; index++ {
		if _, present := fields[strconv.Itoa(index)]; !present {
			return nil, false
		}
	}
	step, stepOK := readActivityInt32Raw(fields["0"])
	itemUse, itemUseOK := decodeDessertNonnegativeMap(fields["1"])
	objects, objectsOK := decodeDessertObjects(fields["2"])
	gameStatus, gameStatusOK := readActivityInt32Raw(fields["3"])
	firstMerge, firstMergeOK := decodeDessertNonnegativeMap(fields["4"])
	var isRunning bool
	isRunningOK := json.Unmarshal(fields["5"], &isRunning) == nil
	totalGain, totalGainOK := decodeDessertNonnegativeMap(fields["6"])
	curID, curIDOK := readActivityInt32Raw(fields["7"])
	score, scoreOK := readActivityInt32Raw(fields["8"])
	levelMap, levelMapOK := decodeDessertNonnegativeMap(fields["9"])
	if !stepOK || step < 0 || !itemUseOK || !objectsOK || !gameStatusOK || gameStatus < 0 || !firstMergeOK ||
		!isRunningOK || !totalGainOK || !curIDOK || curID < 0 || !scoreOK || score < 0 || !levelMapOK {
		return nil, false
	}
	return &dessertModeState{
		Step: step, ItemUse: itemUse, Objects: objects, GameStatus: gameStatus, FirstMerge: firstMerge,
		IsRunning: isRunning, TotalGain: totalGain, CurID: curID, Score: score, LevelMap: levelMap,
	}, true
}

func decodeDessertNonnegativeMap(raw json.RawMessage) (map[int32]int32, bool) {
	values, parsed, valid := decodeActivityInt32Map(raw)
	if !parsed || !valid {
		return nil, false
	}
	for _, value := range values {
		if value < 0 {
			return nil, false
		}
	}
	return values, true
}

func decodeDessertObjects(raw json.RawMessage) ([]DessertObjectView, bool) {
	if isJSONNull(raw) {
		return nil, true
	}
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil || rows == nil {
		return nil, false
	}
	out := make([]DessertObjectView, 0, len(rows))
	for _, rawObject := range rows {
		object, ok := decodeDessertObject(rawObject)
		if !ok {
			return nil, false
		}
		out = append(out, object)
	}
	return out, true
}

func decodeDessertObject(raw json.RawMessage) (DessertObjectView, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return DessertObjectView{}, false
	}
	level, levelOK := readActivityInt32Raw(fields["lv"])
	var isSyn, isAwake bool
	isSynOK := json.Unmarshal(fields["isSyn"], &isSyn) == nil
	isAwakeOK := json.Unmarshal(fields["isAwake"], &isAwake) == nil
	position, positionOK := decodeDessertVector2(fields["pos"])
	linearVelocity, velocityOK := decodeDessertVector2(fields["linearVelocity"])
	angularVelocity, angularOK := decodeDessertFloat(fields["angularVelocity"])
	scale, scaleOK := decodeDessertVector3(fields["scale"])
	nodeAngle, nodeOK := decodeDessertFloat(fields["nodeAngle"])
	lineTime, lineOK := decodeDessertFloat(fields["_lineTime"])
	if !levelOK || level <= 0 || !isSynOK || !isAwakeOK || !positionOK || !velocityOK || !angularOK || !scaleOK ||
		!nodeOK || !lineOK || scale.X <= 0 || scale.Y <= 0 || scale.Z <= 0 {
		return DessertObjectView{}, false
	}
	object := DessertObjectView{
		Raw: cloneRaw(raw), Level: level, IsSyn: isSyn, Position: position, LinearVelocity: linearVelocity,
		AngularVelocity: angularVelocity, Scale: scale, NodeAngle: nodeAngle, IsAwake: isAwake, LineTime: lineTime,
	}
	if rawFall, present := fields["isFallBall"]; present {
		object.IsFallBallObserved = true
		if json.Unmarshal(rawFall, &object.IsFallBall) != nil {
			return DessertObjectView{}, false
		}
	}
	return object, true
}

func decodeDessertVector2(raw json.RawMessage) (DessertVector2, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return DessertVector2{}, false
	}
	x, xOK := decodeDessertFloat(fields["x"])
	y, yOK := decodeDessertFloat(fields["y"])
	return DessertVector2{X: x, Y: y}, xOK && yOK
}

func decodeDessertVector3(raw json.RawMessage) (DessertVector3, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return DessertVector3{}, false
	}
	x, xOK := decodeDessertFloat(fields["x"])
	y, yOK := decodeDessertFloat(fields["y"])
	z, zOK := decodeDessertFloat(fields["z"])
	return DessertVector3{X: x, Y: y, Z: z}, xOK && yOK && zOK
}

func decodeDessertFloat(raw json.RawMessage) (float64, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == '"' {
		return 0, false
	}
	var value float64
	if json.Unmarshal(trimmed, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

// DessertView returns the preferred tmpType-5601 batch at the caller's server
// time. It remains visible while incomplete, but Valid is fail-closed.
func (s *State) DessertView(now time.Time) (DessertView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := DessertView{Observed: s.activityObserved}
	batch, phase, visibleStart, graceEnd, phaseEnd := s.preferredDessertBatchLocked(now.UnixMilli())
	if batch == nil {
		return out, false
	}
	out.Found = true
	out.BatchID = batch.BatchID
	out.TmpID = batch.TmpID
	out.TmpType = batch.TmpType
	out.Status = batch.Status
	out.Phase = phase
	out.VisibleStartMs = visibleStart
	out.BeginMs = batch.BeginMs
	out.EndMs = batch.EndMs
	out.GraceEndMs = graceEnd
	out.PhaseEndMs = phaseEnd
	out.DropCount = batch.Score
	out.DropCountObserved = batch.ScoreObserved && batch.ScoreValid
	out.BagObserved = batch.BagObserved && batch.BagValid
	out.Bag = cloneInt32Map(batch.Bag)
	out.MilestoneReceiptsObserved = batch.BoxesObserved && batch.BoxesValid
	out.ClaimedMilestoneIndexes = append([]int32(nil), batch.ClaimedBoxes...)
	out.ExtensionObserved = batch.Dessert.ExtensionObserved
	out.ExtensionValid = batch.Dessert.ExtensionValid
	out.TotalScore = batch.Dessert.TotalScore
	out.TotalScoreObserved = batch.Dessert.TotalScoreObserved && batch.Dessert.TotalScoreValid
	out.ModeMapObserved = batch.Dessert.ModesObserved
	out.ModeMapValid = batch.Dessert.ModesValid

	config, catalogOK := DessertCatalogConfig()
	if catalogOK {
		out.Name = config.Name
		out.EnergyItemID = config.EnergyItemID
		out.CurrencyItemID = config.CurrencyItemID
		out.PointItemID = config.PointItemID
		out.RewardBoxItemID = config.RewardBoxItemID
		out.EnergyBalance = out.Bag[config.EnergyItemID]
		out.CurrencyBalance = out.Bag[config.CurrencyItemID]
		out.RewardBoxBalance = out.Bag[config.RewardBoxItemID]
	}

	template := s.activityTemplates[batch.TmpID]
	if template != nil {
		if template.Name != "" {
			out.Name = template.Name
		}
		out.Description = template.Description
		out.TaskGroupsObserved = template.TasksObserved
		out.TaskGroupsValid = template.TasksValid
	}

	stateValid := catalogOK && config.TmpType == batch.TmpType && batch.BatchID > 0 && batch.IdentityValid && batch.TmpID > 0 &&
		batch.Status == 1 && batch.BeginMs > 0 && batch.EndMs > batch.BeginMs && batch.DurationBeforeMs >= 0 && batch.DurationAfterMs >= 0 &&
		batch.ScoreObserved && batch.ScoreValid && batch.Score >= 0 && batch.BagObserved && batch.BagValid && nonnegativeInt32Map(batch.Bag) &&
		(!batch.BoxesObserved || (batch.BoxesValid && uniquePositiveInt32s(batch.ClaimedBoxes))) &&
		template != nil && template.IdentityValid && template.TmpID == batch.TmpID && template.TmpType == config.TmpType &&
		template.TasksObserved && template.TasksValid && template.BoxesObserved && template.BoxesValid &&
		batch.Dessert.ExtensionObserved && batch.Dessert.ExtensionValid && batch.Dessert.TotalScoreObserved && batch.Dessert.TotalScoreValid &&
		batch.Dessert.ModesObserved && batch.Dessert.ModesValid

	if catalogOK {
		out.Modes = make([]DessertModeView, 0, len(config.Multipliers))
		if len(batch.Dessert.Modes) != len(config.Multipliers) {
			stateValid = false
		}
		for index, multiplier := range config.Multipliers {
			modeID := int32(index + 1)
			modeView := DessertModeView{Mode: modeID, Multiplier: multiplier, UnlockScore: config.UnlockScores[index]}
			mode := batch.Dessert.Modes[modeID]
			if mode != nil {
				modeView = cloneDessertModeView(modeID, multiplier, config.UnlockScores[index], mode)
				modeView.Observed = true
				modeView.Valid = dessertModeMatchesCatalog(mode, int32(len(config.Levels)))
				if !modeView.Valid {
					stateValid = false
				}
			} else {
				stateValid = false
			}
			out.Modes = append(out.Modes, modeView)
		}
	}

	if template != nil {
		out.Tasks, out.TaskRecordObserved, stateValid = s.dessertTasksLocked(batch, template, stateValid)
		claimed := make(map[int32]struct{}, len(batch.ClaimedBoxes))
		for _, index := range batch.ClaimedBoxes {
			claimed[index] = struct{}{}
		}
		out.Milestones = make([]DessertMilestoneView, 0, len(template.Milestones))
		for _, milestone := range template.Milestones {
			_, received := claimed[milestone.Index]
			out.Milestones = append(out.Milestones, DessertMilestoneView{
				Index: milestone.Index, Target: milestone.Target, Received: received, Reward: cloneCyclicNoteItems(milestone.Reward),
			})
		}
	}
	out.Celebrity = s.dessertCelebrityViewLocked(batch)
	out.Valid = stateValid
	return out, true
}

func (s *State) dessertTasksLocked(batch *activityBatchState, template *activityTemplateState, stateValid bool) ([]DessertTaskView, bool, bool) {
	if len(template.TaskGroups) == 0 {
		return nil, false, stateValid
	}
	taskCount := 0
	for _, group := range template.TaskGroups {
		taskCount += len(group.Tasks)
	}
	out := make([]DessertTaskView, 0, taskCount)
	allRecordsObserved := true
	for _, group := range template.TaskGroups {
		record := s.activityTaskRecords[activityTaskRecordKey(batch.BatchID, group.TaskIndex)]
		if record == nil {
			allRecordsObserved = false
		} else if !record.IdentityValid || (record.ProgressObserved && (!record.ProgressValid || !nonnegativeInt32Map(record.Progress))) ||
			(record.ReceiptsObserved && (!record.ReceiptsValid || !nonnegativeInt32Map(record.Receipts))) {
			stateValid = false
		}
		for _, definition := range group.Tasks {
			task := DessertTaskView{
				TaskIndex: group.TaskIndex, Position: definition.Position, TaskID: definition.TaskID, TaskType: definition.TaskType,
				Param: definition.Param, HasParam: definition.HasParam, Title: definition.Title, Target: definition.Target,
				CatalogKnown: definition.CatalogKnown, Reward: cloneCyclicNoteItems(definition.Reward),
			}
			if record != nil {
				if record.ProgressObserved && record.ProgressValid {
					task.Progress, task.ProgressObserved = record.Progress[definition.TaskID]
				}
				if record.ReceiptsObserved && record.ReceiptsValid {
					task.ReceiptObserved = true
					_, task.Received = record.Receipts[definition.TaskID]
				}
			}
			out = append(out, task)
		}
	}
	return out, allRecordsObserved, stateValid
}

func (s *State) preferredDessertBatchLocked(nowMs int64) (*activityBatchState, int32, int64, int64, int64) {
	var selected *activityBatchState
	var selectedPhase int32
	var selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd int64
	for _, batch := range s.activityBatches {
		if batch == nil || batch.TmpType != dessertTmpType || batch.Status != 1 {
			continue
		}
		phase, visibleStart, graceEnd, phaseEnd, ok := cyclicNotePhase(batch, nowMs)
		if !ok || (phase != 1 && phase != 2 && phase != 3) {
			continue
		}
		if selected != nil && !preferCyclicNoteBatch(batch, phase, selected, selectedPhase) {
			continue
		}
		selected, selectedPhase = batch, phase
		selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd = visibleStart, graceEnd, phaseEnd
	}
	return selected, selectedPhase, selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd
}

func cloneDessertModeView(modeID, multiplier, unlockScore int32, mode *dessertModeState) DessertModeView {
	objects := make([]DessertObjectView, len(mode.Objects))
	copy(objects, mode.Objects)
	levelCounts := make(map[int32]int32)
	for index := range objects {
		objects[index].Raw = cloneRaw(objects[index].Raw)
		levelCounts[objects[index].Level]++
	}
	return DessertModeView{
		Mode: modeID, Multiplier: multiplier, UnlockScore: unlockScore, Step: mode.Step,
		ItemUse: cloneInt32Map(mode.ItemUse), Objects: objects, ObjectCount: int32(len(objects)), GameStatus: mode.GameStatus,
		FirstMerge: cloneInt32Map(mode.FirstMerge), IsRunning: mode.IsRunning, TotalGain: cloneInt32Map(mode.TotalGain),
		CurID: mode.CurID, Score: mode.Score, LevelMap: cloneInt32Map(mode.LevelMap), LevelCounts: levelCounts,
	}
}

func dessertModeMatchesCatalog(mode *dessertModeState, levelMax int32) bool {
	if mode == nil || mode.Step < 0 || mode.GameStatus < 0 || mode.CurID < 0 || mode.CurID > levelMax || mode.Score < 0 ||
		!nonnegativeInt32Map(mode.ItemUse) || !nonnegativeInt32Map(mode.FirstMerge) || !nonnegativeInt32Map(mode.TotalGain) ||
		!nonnegativeInt32Map(mode.LevelMap) {
		return false
	}
	for level := range mode.LevelMap {
		if level <= 0 || level > levelMax {
			return false
		}
	}
	for _, object := range mode.Objects {
		if object.Level <= 0 || object.Level > levelMax {
			return false
		}
	}
	return true
}

func nonnegativeInt32Map(values map[int32]int32) bool {
	for key, value := range values {
		if key <= 0 || value < 0 {
			return false
		}
	}
	return true
}

func uniquePositiveInt32s(values []int32) bool {
	seen := make(map[int32]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
