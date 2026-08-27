package state

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"time"
)

const cyclicNoteTaskRecordIndex int32 = 0

type activityBatchState struct {
	BatchID             int32
	IdentityValid       bool
	TmpID               int32
	TmpType             int32
	Status              int32
	BeginMs             int64
	EndMs               int64
	DurationBeforeMs    int64
	DurationAfterMs     int64
	Score               int32
	ScoreObserved       bool
	ScoreValid          bool
	Bag                 map[int32]int32
	BagObserved         bool
	BagValid            bool
	ClaimedBoxes        []int32
	BoxesObserved       bool
	BoxesValid          bool
	TaskList            []int32
	TaskListObserved    bool
	TaskListValid       bool
	FinishCount         int32
	FinishCountObserved bool
	FinishCountValid    bool
	LastRefreshTimeMs   int64
	Story               cyclicStoryActivityState
}

type activityTemplateState struct {
	TmpID         int32
	IdentityValid bool
	Name          string
	Description   string
	TmpType       int32
	Milestones    []CyclicNoteMilestoneInfo
	BoxesObserved bool
	BoxesValid    bool
}

type activityTaskRecordState struct {
	BatchID          int32
	Index            int32
	IdentityValid    bool
	Progress         map[int32]int32
	ProgressObserved bool
	ProgressValid    bool
	Receipts         map[int32]int32
	ReceiptsObserved bool
	ReceiptsValid    bool
	RefreshMs        int64
	UpdateTimeMs     int64
	CreateTimeMs     int64
}

func (s *State) applyActivitiesLocked(raw json.RawMessage) {
	s.activityObserved = true
	if s.activityBatches == nil {
		s.activityBatches = make(map[int32]*activityBatchState)
	}
	if s.activityTemplates == nil {
		s.activityTemplates = make(map[int32]*activityTemplateState)
	}
	if s.activityTaskRecords == nil {
		s.activityTaskRecords = make(map[string]*activityTaskRecordState)
	}

	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return
	}
	if rawBatches, ok := fields["0"]; ok {
		s.mergeActivityBatchesLocked(rawBatches)
	}
	if rawTemplates, ok := fields["1"]; ok {
		s.mergeActivityTemplatesLocked(rawTemplates)
	}
	if rawRecords, ok := fields["3"]; ok {
		s.mergeActivityTaskRecordsLocked(rawRecords)
	}
}

func (s *State) mergeActivityBatchesLocked(raw json.RawMessage) {
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil {
		return
	}
	for key, rawEntry := range entries {
		batchID, keyOK := parsePositiveActivityIDKey(key)
		if !keyOK {
			continue
		}
		if isJSONNull(rawEntry) {
			delete(s.activityBatches, batchID)
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEntry, &fields) != nil || fields == nil {
			continue
		}
		batch := s.activityBatches[batchID]
		if batch == nil {
			batch = &activityBatchState{
				BatchID:       batchID,
				IdentityValid: true,
				ScoreValid:    true,
				BagValid:      true,
				BoxesValid:    true,
				TaskListValid: true,
			}
			s.activityBatches[batchID] = batch
		}
		mergeActivityBatchFields(batch, fields)
	}
}

func mergeActivityBatchFields(batch *activityBatchState, fields map[string]json.RawMessage) {
	if rawID, present := fields["0"]; present {
		value, ok := readActivityInt32Raw(rawID)
		batch.IdentityValid = ok && value == batch.BatchID
	}
	if value, ok := readActivityInt32Field(fields, "1"); ok {
		batch.TmpID = value
	}
	if value, ok := readActivityInt32Field(fields, "2"); ok {
		batch.TmpType = value
	}
	if value, ok := readActivityInt32Field(fields, "3"); ok {
		batch.Status = value
	}
	if value, ok := readActivityInt64Field(fields, "5"); ok {
		batch.BeginMs = value
	}
	if value, ok := readActivityInt64Field(fields, "7"); ok {
		batch.EndMs = value
	}
	if value, ok := readActivityInt64Field(fields, "8"); ok {
		batch.DurationBeforeMs = value
	}
	if value, ok := readActivityInt64Field(fields, "9"); ok {
		batch.DurationAfterMs = value
	}
	if rawScore, present := fields["11"]; present {
		value, valid := readActivityInt32Raw(rawScore)
		batch.ScoreObserved = true
		batch.ScoreValid = valid
		if valid {
			batch.Score = value
		} else {
			batch.Score = 0
		}
	}
	if rawBag, present := fields["12"]; present {
		values, parsed, valid := decodeActivityInt32Map(rawBag)
		batch.Bag = values
		batch.BagObserved = true
		batch.BagValid = parsed && valid
	}
	if rawBoxes, present := fields["13"]; present {
		values, parsed, valid := decodeActivityInt32List(rawBoxes, false)
		batch.ClaimedBoxes = values
		batch.BoxesObserved = true
		batch.BoxesValid = parsed && valid
	}
	if rawExt, present := fields["14"]; present {
		mergeCyclicNoteExtension(batch, rawExt)
		mergeCyclicStoryExtension(batch, rawExt)
	}
}

func mergeCyclicNoteExtension(batch *activityBatchState, raw json.RawMessage) {
	var ext map[string]json.RawMessage
	if json.Unmarshal(raw, &ext) != nil || ext == nil {
		return
	}
	rawCyclic, present := ext["105"]
	if !present {
		return
	}
	var cyclic map[string]json.RawMessage
	if json.Unmarshal(rawCyclic, &cyclic) != nil || cyclic == nil {
		return
	}
	if rawTasks, present := cyclic["0"]; present {
		values, parsed, valid := decodeActivityInt32List(rawTasks, true)
		batch.TaskList = values
		batch.TaskListObserved = true
		batch.TaskListValid = parsed && valid
	}
	if rawFinishCount, present := cyclic["1"]; present {
		value, valid := readActivityInt32Raw(rawFinishCount)
		batch.FinishCountObserved = true
		batch.FinishCountValid = valid && value >= 0
		if batch.FinishCountValid {
			batch.FinishCount = value
		} else {
			batch.FinishCount = 0
		}
	}
	if value, ok := readActivityInt64Field(cyclic, "2"); ok {
		batch.LastRefreshTimeMs = value
	}
}

func (s *State) mergeActivityTemplatesLocked(raw json.RawMessage) {
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil {
		return
	}
	for key, rawEntry := range entries {
		tmpID, keyOK := parsePositiveActivityIDKey(key)
		if !keyOK {
			continue
		}
		if isJSONNull(rawEntry) {
			delete(s.activityTemplates, tmpID)
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEntry, &fields) != nil || fields == nil {
			continue
		}
		template := s.activityTemplates[tmpID]
		if template == nil {
			template = &activityTemplateState{TmpID: tmpID, IdentityValid: true, BoxesValid: true}
			s.activityTemplates[tmpID] = template
		}
		if rawID, present := fields["0"]; present {
			value, ok := readActivityInt32Raw(rawID)
			template.IdentityValid = ok && value == tmpID
		}
		if rawName, present := fields["1"]; present {
			var value string
			if json.Unmarshal(rawName, &value) == nil {
				template.Name = value
			}
		}
		if rawDescription, present := fields["2"]; present {
			var value string
			if json.Unmarshal(rawDescription, &value) == nil {
				template.Description = value
			}
		}
		if value, ok := readActivityInt32Field(fields, "3"); ok {
			template.TmpType = value
		}
		if rawBoxes, present := fields["9"]; present {
			milestones, valid := ParseCyclicNoteTemplateBoxes(rawBoxes)
			template.Milestones = milestones
			template.BoxesObserved = true
			template.BoxesValid = valid
		}
	}
}

func (s *State) mergeActivityTaskRecordsLocked(raw json.RawMessage) {
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || entries == nil {
		return
	}
	for key, rawEntry := range entries {
		batchID, index, keyOK := parseActivityTaskRecordKey(key)
		if !keyOK {
			continue
		}
		if isJSONNull(rawEntry) {
			delete(s.activityTaskRecords, key)
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEntry, &fields) != nil || fields == nil {
			continue
		}
		record := s.activityTaskRecords[key]
		if record == nil {
			record = &activityTaskRecordState{
				BatchID:       batchID,
				Index:         index,
				IdentityValid: true,
				ProgressValid: true,
				ReceiptsValid: true,
			}
			s.activityTaskRecords[key] = record
		}
		if rawBatchID, present := fields["1"]; present {
			value, ok := readActivityInt32Raw(rawBatchID)
			// A contradictory record identity taints this entry until the
			// server explicitly deletes it; later sparse fields must not make
			// the same map key trusted again by accident.
			record.IdentityValid = record.IdentityValid && ok && value == batchID
		}
		if rawIndex, present := fields["2"]; present {
			value, ok := readActivityInt32Raw(rawIndex)
			record.IdentityValid = record.IdentityValid && ok && value == index
		}
		if rawProgress, present := fields["3"]; present {
			values, parsed, valid := decodeActivityInt32Map(rawProgress)
			record.Progress = values
			record.ProgressObserved = true
			record.ProgressValid = parsed && valid
		}
		// Field 4 is an authoritative record map too, but cyclic-note
		// readiness does not infer anything from it. Its appearance must not
		// mutate progress or receipts.
		if rawReceipts, present := fields["5"]; present {
			values, parsed, valid := decodeActivityInt32Map(rawReceipts)
			record.Receipts = values
			record.ReceiptsObserved = true
			record.ReceiptsValid = parsed && valid
		}
		if value, ok := readActivityInt64Field(fields, "6"); ok {
			record.RefreshMs = value
		}
		if value, ok := readActivityInt64Field(fields, "7"); ok {
			record.UpdateTimeMs = value
		}
		if value, ok := readActivityInt64Field(fields, "8"); ok {
			record.CreateTimeMs = value
		}
	}
}

// CyclicNoteView returns the currently preferred 花笺集芳 activity snapshot.
// Selection mirrors the client-visible phases and never hard-codes a batch.
func (s *State) CyclicNoteView(now time.Time) (CyclicNoteView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := CyclicNoteView{Observed: s.activityObserved}
	config, catalogOK := CyclicNoteCatalogConfig()
	if catalogOK {
		out.TmpType = config.TmpType
		out.CurrencyItemID = config.CurrencyItemID
		out.Name = config.Name
	}
	batch, phase, visibleStart, graceEnd, phaseEnd := s.preferredCyclicNoteBatchLocked(now.UnixMilli())
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
	out.Score = batch.Score
	out.Bag = cloneInt32Map(batch.Bag)
	out.FinishCount = batch.FinishCount
	out.FinishCountObserved = batch.FinishCountObserved && batch.FinishCountValid
	out.LastRefreshTimeMs = batch.LastRefreshTimeMs
	out.TaskListObserved = batch.TaskListObserved
	out.MilestoneReceiptsObserved = batch.BoxesObserved && batch.BoxesValid
	out.ClaimedMilestoneIndexes = append([]int32(nil), batch.ClaimedBoxes...)

	if catalogOK {
		out.CurrencyItemID = config.CurrencyItemID
		out.CurrencyBalance = out.Bag[config.CurrencyItemID]
		out.Name = config.Name
	}
	template := s.activityTemplates[batch.TmpID]
	if template != nil {
		if template.Name != "" {
			out.Name = template.Name
		}
		out.Description = template.Description
	}

	recordKey := activityTaskRecordKey(batch.BatchID, cyclicNoteTaskRecordIndex)
	record := s.activityTaskRecords[recordKey]
	out.TaskRecordObserved = record != nil
	out.Valid = catalogOK && config.TmpType == batch.TmpType && batch.BatchID > 0 && batch.IdentityValid && batch.TmpID > 0 &&
		batch.Status == 1 && batch.BeginMs > 0 && batch.EndMs > batch.BeginMs && batch.DurationBeforeMs >= 0 &&
		batch.DurationAfterMs >= 0 && batch.ScoreObserved && batch.ScoreValid && batch.BagObserved && batch.BagValid &&
		(!batch.FinishCountObserved || batch.FinishCountValid) &&
		(!batch.BoxesObserved || batch.BoxesValid) &&
		template != nil && template.IdentityValid && template.TmpID == batch.TmpID && template.TmpType == config.TmpType &&
		template.BoxesObserved && template.BoxesValid && (!batch.TaskListObserved || batch.TaskListValid)
	if record != nil && (!record.IdentityValid || (record.ProgressObserved && !record.ProgressValid) || (record.ReceiptsObserved && !record.ReceiptsValid)) {
		out.Valid = false
	}

	slotCount := int32(len(batch.TaskList))
	if catalogOK && config.TaskSlotCount > slotCount {
		slotCount = config.TaskSlotCount
	}
	if slotCount > 0 {
		out.Tasks = make([]CyclicNoteTaskSlotView, 0, slotCount)
	}
	for slotIndex := int32(0); slotIndex < slotCount; slotIndex++ {
		taskID := int32(0)
		if int(slotIndex) < len(batch.TaskList) {
			taskID = batch.TaskList[slotIndex]
		}
		task := CyclicNoteTaskSlotView{SlotID: slotIndex + 1, Unlocked: taskID > 0, TaskID: taskID}
		if taskID > 0 {
			info := CyclicNoteTaskInfoByID(taskID)
			task.TaskType = info.TaskType
			task.Param = info.Param
			task.Title = info.Title
			task.Target = info.Target
			task.CatalogKnown = info.CatalogKnown
			task.Reward = cloneCyclicNoteItems(info.Reward)
			task.FinishCost = cloneCyclicNoteItems(info.FinishCost)
			if record != nil {
				task.ProgressObserved = record.ProgressObserved && record.ProgressValid
				task.ReceiptObserved = record.ReceiptsObserved && record.ReceiptsValid
				if task.ProgressObserved {
					task.Progress = record.Progress[taskID]
				}
				if task.ReceiptObserved {
					_, task.Received = record.Receipts[taskID]
				}
			}
		}
		out.Tasks = append(out.Tasks, task)
	}

	if template != nil {
		claimed := make(map[int32]struct{}, len(batch.ClaimedBoxes))
		for _, index := range batch.ClaimedBoxes {
			if index > 0 {
				claimed[index] = struct{}{}
			}
		}
		out.Milestones = make([]CyclicNoteMilestoneView, 0, len(template.Milestones))
		for _, milestone := range template.Milestones {
			_, received := claimed[milestone.Index]
			out.Milestones = append(out.Milestones, CyclicNoteMilestoneView{
				Index: milestone.Index, Target: milestone.Target, Received: received,
				Reward: cloneCyclicNoteItems(milestone.Reward),
			})
		}
	}
	return out, true
}

func (s *State) preferredCyclicNoteBatchLocked(nowMs int64) (*activityBatchState, int32, int64, int64, int64) {
	var selected *activityBatchState
	var selectedPhase int32
	var selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd int64
	for _, batch := range s.activityBatches {
		if batch == nil || batch.TmpType != 4002 || batch.Status != 1 {
			continue
		}
		phase, visibleStart, graceEnd, phaseEnd, ok := cyclicNotePhase(batch, nowMs)
		if !ok || (phase != 1 && phase != 2 && phase != 3) {
			continue
		}
		if selected != nil && !preferCyclicNoteBatch(batch, phase, selected, selectedPhase) {
			continue
		}
		selected = batch
		selectedPhase = phase
		selectedVisibleStart = visibleStart
		selectedGraceEnd = graceEnd
		selectedPhaseEnd = phaseEnd
	}
	return selected, selectedPhase, selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd
}

func cyclicNotePhase(batch *activityBatchState, nowMs int64) (phase int32, visibleStart, graceEnd, phaseEnd int64, ok bool) {
	if batch == nil || batch.BeginMs <= 0 || batch.EndMs <= batch.BeginMs || batch.DurationBeforeMs < 0 ||
		batch.DurationAfterMs < 0 || batch.DurationBeforeMs > batch.BeginMs ||
		batch.DurationAfterMs > math.MaxInt64-batch.EndMs {
		return 0, 0, 0, 0, false
	}
	visibleStart = batch.BeginMs - batch.DurationBeforeMs
	graceEnd = batch.EndMs + batch.DurationAfterMs
	switch {
	case nowMs < visibleStart:
		return 0, visibleStart, graceEnd, visibleStart, true
	case nowMs < batch.BeginMs:
		return 1, visibleStart, graceEnd, batch.BeginMs, true
	case nowMs < batch.EndMs:
		return 2, visibleStart, graceEnd, batch.EndMs, true
	case nowMs < graceEnd:
		return 3, visibleStart, graceEnd, graceEnd, true
	default:
		return 4, visibleStart, graceEnd, graceEnd, true
	}
}

func preferCyclicNoteBatch(candidate *activityBatchState, candidatePhase int32, selected *activityBatchState, selectedPhase int32) bool {
	candidateRank := cyclicNotePhaseRank(candidatePhase)
	selectedRank := cyclicNotePhaseRank(selectedPhase)
	if candidateRank != selectedRank {
		return candidateRank < selectedRank
	}
	if candidate.BeginMs != selected.BeginMs {
		return candidate.BeginMs > selected.BeginMs
	}
	return candidate.BatchID > selected.BatchID
}

func cyclicNotePhaseRank(phase int32) int {
	switch phase {
	case 2:
		return 0
	case 3:
		return 1
	case 1:
		return 2
	default:
		return 3
	}
}

func parseActivityTaskRecordKey(key string) (int32, int32, bool) {
	separator := bytes.IndexByte([]byte(key), '|')
	if separator <= 0 || separator >= len(key)-1 || bytes.IndexByte([]byte(key[separator+1:]), '|') >= 0 {
		return 0, 0, false
	}
	batchID64, batchErr := strconv.ParseInt(key[:separator], 10, 32)
	index64, indexErr := strconv.ParseInt(key[separator+1:], 10, 32)
	if batchErr != nil || indexErr != nil || batchID64 <= 0 || index64 < 0 ||
		strconv.FormatInt(batchID64, 10) != key[:separator] || strconv.FormatInt(index64, 10) != key[separator+1:] {
		return 0, 0, false
	}
	return int32(batchID64), int32(index64), true
}

func activityTaskRecordKey(batchID, index int32) string {
	return strconv.FormatInt(int64(batchID), 10) + "|" + strconv.FormatInt(int64(index), 10)
}

func parsePositiveActivityIDKey(key string) (int32, bool) {
	value, err := strconv.ParseInt(key, 10, 32)
	if err != nil || value <= 0 || strconv.FormatInt(value, 10) != key {
		return 0, false
	}
	return int32(value), true
}

func readActivityInt32Field(fields map[string]json.RawMessage, key string) (int32, bool) {
	raw, ok := fields[key]
	if !ok {
		return 0, false
	}
	return readActivityInt32Raw(raw)
}

func readActivityInt64Field(fields map[string]json.RawMessage, key string) (int64, bool) {
	raw, ok := fields[key]
	if !ok {
		return 0, false
	}
	return readActivityInt64Raw(raw)
}

func readActivityInt32Raw(raw json.RawMessage) (int32, bool) {
	value, ok := readActivityInt64Raw(raw)
	if !ok || value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false
	}
	return int32(value), true
}

func readActivityInt64Raw(raw json.RawMessage) (int64, bool) {
	var value int64
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	return value, true
}

func decodeActivityInt32Map(raw json.RawMessage) (map[int32]int32, bool, bool) {
	if isJSONNull(raw) {
		return map[int32]int32{}, true, true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false, false
	}
	out := make(map[int32]int32, len(fields))
	valid := true
	for key, rawValue := range fields {
		id64, err := strconv.ParseInt(key, 10, 32)
		if isJSONNull(rawValue) {
			if err != nil || id64 <= 0 || strconv.FormatInt(id64, 10) != key {
				valid = false
			}
			continue
		}
		value, valueOK := readActivityInt32Raw(rawValue)
		if err != nil || id64 <= 0 || strconv.FormatInt(id64, 10) != key || !valueOK {
			valid = false
			continue
		}
		out[int32(id64)] = value
	}
	return out, true, valid
}

func decodeActivityInt32List(raw json.RawMessage, allowZero bool) ([]int32, bool, bool) {
	if isJSONNull(raw) {
		return nil, true, true
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return nil, false, false
	}
	out := make([]int32, len(values))
	valid := true
	for index, rawValue := range values {
		if allowZero && isJSONNull(rawValue) {
			continue
		}
		value, ok := readActivityInt32Raw(rawValue)
		if !ok || value < 0 || (!allowZero && value == 0) {
			valid = false
			continue
		}
		out[index] = value
	}
	return out, true, valid
}

func cloneCyclicNoteItems(items []ItemCount) []ItemCount {
	if len(items) == 0 {
		return nil
	}
	return append([]ItemCount(nil), items...)
}
