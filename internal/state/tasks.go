package state

import (
	"encoding/json"
	"sort"
	"strconv"
)

func (s *State) applyTasksLocked(raw json.RawMessage) {
	var ns22 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns22); err != nil {
		return
	}
	if rawMain, ok := ns22["0"]; ok {
		s.applyMainTaskLocked(rawMain)
	}
	if rawDaily, ok := ns22["1"]; ok {
		s.applyDailyTasksLocked(rawDaily)
	}
	if rawAchievement, ok := ns22["2"]; ok {
		s.applyAchievementTasksLocked(rawAchievement)
	}
	if rawWeekly, ok := ns22["100"]; ok {
		s.applyWeeklyTasksLocked(rawWeekly)
	}
}

func (s *State) applyMainTaskLocked(rawMain json.RawMessage) {
	if isJSONNull(rawMain) {
		s.invalidateMainTaskLocked()
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(rawMain, &fields) != nil {
		s.invalidateMainTaskLocked()
		return
	}
	view := MainTaskView{}
	if s.mainTask != nil {
		view = *s.mainTask
	}
	view.Observed = true
	previousTaskID := view.TaskID
	previousTaskObserved := view.TaskIDObserved
	_, progressPresent := fields["2"]
	if rawTaskID, present := fields["1"]; present {
		view.TaskID = 0
		view.TaskIDObserved = false
		if taskID, valid := readStoryMainInt32(rawTaskID); valid && taskID > 0 {
			view.TaskID = taskID
			view.TaskIDObserved = true
			if (!previousTaskObserved || taskID != previousTaskID) && !progressPresent {
				view.Finished = 0
				view.ProgressObserved = false
			}
		}
	}
	if rawProgress, present := fields["2"]; present {
		view.Finished = 0
		view.ProgressObserved = false
		if progress, valid := readStoryMainInt32(rawProgress); valid && progress >= 0 {
			view.Finished = progress
			view.ProgressObserved = true
		}
	}
	if rawReceipts, present := fields["4"]; present {
		receipts, valid := decodeMainTaskReceipts(rawReceipts)
		if valid {
			s.mainTaskReceipts = receipts
			s.mainTaskRecvObserved = true
		} else {
			s.mainTaskReceipts = nil
			s.mainTaskRecvObserved = false
		}
	}

	view.Valid = false
	view.Complete = false
	view.Target = 0
	view.NextTaskID = 0
	view.ReceiptObserved = s.mainTaskRecvObserved
	view.Receipted = false
	if view.TaskIDObserved {
		if definition, valid, complete := ResolveMainTaskDefinition(view.TaskID); valid {
			view.Valid = true
			view.Complete = complete
			if !complete {
				view.Target = definition.Target
				view.NextTaskID = definition.NextTaskID
			}
			if view.ReceiptObserved {
				_, view.Receipted = s.mainTaskReceipts[view.TaskID]
			}
		}
	}
	s.mainTask = &view
}

func (s *State) invalidateMainTaskLocked() {
	s.mainTask = &MainTaskView{Observed: true}
	s.mainTaskReceipts = nil
	s.mainTaskRecvObserved = false
}

func decodeMainTaskReceipts(raw json.RawMessage) (map[int32]int32, bool) {
	if isJSONNull(raw) {
		return map[int32]int32{}, true
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return nil, false
	}
	receipts := make(map[int32]int32, len(values))
	for key, rawValue := range values {
		parsedTaskID, err := strconv.ParseInt(key, 10, 32)
		taskID := int32(parsedTaskID)
		value, valid := readStoryMainInt32(rawValue)
		if err != nil || taskID <= 0 || strconv.FormatInt(parsedTaskID, 10) != key || !valid || value < 0 {
			return nil, false
		}
		receipts[taskID] = value
	}
	return receipts, true
}

func (s *State) applyDailyTasksLocked(rawDaily json.RawMessage) {
	var daily map[string]json.RawMessage
	if err := json.Unmarshal(rawDaily, &daily); err != nil {
		return
	}

	rawProgress, progressObserved := daily["1"]
	rawRecv, recvObserved := daily["3"]
	progressMap := readInt32RawMap(rawProgress)
	recvMap := readInt32RawMap(rawRecv)

	rawTaskMap, ok := daily["100"]
	if !ok {
		s.mergeDailyTaskDeltaLocked(progressMap, progressObserved, recvMap, recvObserved)
		return
	}
	var tasks map[string]json.RawMessage
	if err := json.Unmarshal(rawTaskMap, &tasks); err != nil {
		return
	}
	prev := s.dailyTasks
	next := make(map[int32]*DailyTaskView, len(tasks))
	for idStr, rawTask := range tasks {
		id := atoi32(idStr)
		if id == 0 {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal(rawTask, &fields); err != nil {
			continue
		}
		taskID := int32(readInt(fields, "0"))
		if taskID == 0 {
			taskID = id
		}
		progressType, _ := DailyTaskProgressType(taskID)
		finished := readInt32Any(fields["2"])
		if old := prev[id]; old != nil && !progressObserved {
			finished = old.Finished
		}
		if progressObserved && progressType > 0 {
			if progress := progressMap[progressType]; progress > finished {
				finished = progress
			}
		}
		receipted := int32(0)
		if old := prev[id]; old != nil && !recvObserved {
			receipted = old.Receipted
		}
		if recvObserved {
			receipted = dailyTaskReceipt(recvMap, id, taskID, progressType)
		}
		target := readInt32Any(fields["1"])
		next[id] = &DailyTaskView{
			TaskID:       taskID,
			ProgressType: progressType,
			Target:       target,
			Finished:     finished,
			Status:       dailyTaskStatus(target, finished, receipted),
			Receipted:    receipted,
		}
	}
	s.dailyTasks = next
}

func (s *State) mergeDailyTaskDeltaLocked(progressMap map[int32]int32, progressObserved bool, recvMap map[int32]int32, recvObserved bool) {
	if !progressObserved && !recvObserved {
		return
	}
	for id, task := range s.dailyTasks {
		if task == nil {
			continue
		}
		if progressObserved && task.ProgressType > 0 {
			if progress := progressMap[task.ProgressType]; progress > task.Finished {
				task.Finished = progress
			}
		}
		if recvObserved {
			if receipt := dailyTaskReceipt(recvMap, id, task.TaskID, task.ProgressType); receipt != 0 {
				task.Receipted = receipt
			}
		}
		task.Status = dailyTaskStatus(task.Target, task.Finished, task.Receipted)
	}
}

func dailyTaskReceipt(recvMap map[int32]int32, id, taskID, progressType int32) int32 {
	for _, key := range []int32{progressType, id, taskID} {
		if key == 0 {
			continue
		}
		if receipt := recvMap[key]; receipt != 0 {
			return receipt
		}
	}
	return 0
}

func dailyTaskStatus(target, finished, receipted int32) int32 {
	if receipted != 0 {
		return 3
	}
	if target > 0 && finished >= target {
		return 1
	}
	return 2
}

func (s *State) applyWeeklyTasksLocked(rawWeekly json.RawMessage) {
	var weekly map[string]json.RawMessage
	if err := json.Unmarshal(rawWeekly, &weekly); err != nil {
		return
	}
	rawProgress, progressObserved := weekly["1"]
	rawRecv, recvObserved := weekly["3"]
	progressMap := readInt32RawMap(rawProgress)
	recvMap := readInt32RawMap(rawRecv)
	defs := WeeklyTaskDefinitions()
	prev := s.weeklyTasks
	s.weeklyTasks = make(map[int32]*WeeklyTaskView, len(defs))
	for _, def := range defs {
		finished := progressMap[def.ProgressType]
		receipted := recvMap[def.TaskID]
		if old := prev[def.TaskID]; old != nil {
			if !progressObserved {
				finished = old.Finished
			}
			if !recvObserved {
				receipted = old.Receipted
			}
		}
		status := int32(2)
		if receipted != 0 {
			status = 3
		} else if def.Target > 0 && finished >= def.Target {
			status = 1
		}
		s.weeklyTasks[def.TaskID] = &WeeklyTaskView{
			TaskID:    def.TaskID,
			Target:    def.Target,
			Finished:  finished,
			Status:    status,
			Receipted: receipted,
		}
	}
}

func (s *State) applyAchievementTasksLocked(rawAchievement json.RawMessage) {
	var achievement map[string]json.RawMessage
	if err := json.Unmarshal(rawAchievement, &achievement); err != nil {
		return
	}
	rawProgress, progressObserved := achievement["1"]
	rawRecv, recvObserved := achievement["3"]
	progressMap := readInt32RawMap(rawProgress)
	recvMap := readInt32RawMap(rawRecv)
	defs := AchievementTaskDefinitions()
	prev := s.achievementTasks
	prevProgress := achievementProgressByType(prev)
	prevReceived := achievementReceivedByGroup(prev)
	s.achievementTasks = make(map[int32]*AchievementTaskView, len(defs))
	for _, def := range defs {
		finished, ok := progressMap[def.ProgressType]
		if !progressObserved || !ok {
			finished = prevProgress[def.ProgressType]
		}
		groupReceived, ok := recvMap[def.GroupID]
		if !recvObserved || !ok {
			groupReceived = prevReceived[def.GroupID]
		}
		if groupReceived < 0 {
			groupReceived = 0
		}
		receipted := int32(0)
		status := int32(2)
		current := groupReceived+1 == def.StageIndex
		if groupReceived >= def.StageIndex {
			receipted = 1
			status = 3
			current = false
		} else if current && def.Target > 0 && finished >= def.Target {
			status = 1
		}
		s.achievementTasks[def.TaskID] = &AchievementTaskView{
			TaskID:        def.TaskID,
			GroupID:       def.GroupID,
			StageIndex:    def.StageIndex,
			ProgressType:  def.ProgressType,
			Target:        def.Target,
			Finished:      finished,
			Status:        status,
			Receipted:     receipted,
			GroupReceived: groupReceived,
			Current:       current,
		}
	}
}

func achievementProgressByType(tasks map[int32]*AchievementTaskView) map[int32]int32 {
	out := map[int32]int32{}
	for _, task := range tasks {
		if task == nil || task.ProgressType <= 0 {
			continue
		}
		if task.Finished > out[task.ProgressType] {
			out[task.ProgressType] = task.Finished
		}
	}
	return out
}

func achievementReceivedByGroup(tasks map[int32]*AchievementTaskView) map[int32]int32 {
	out := map[int32]int32{}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		groupID := task.GroupID
		if groupID <= 0 {
			groupID = task.TaskID / 10000
		}
		if groupID <= 0 {
			continue
		}
		received := task.GroupReceived
		if received == 0 && task.Receipted != 0 && task.StageIndex > 0 {
			received = task.StageIndex
		}
		if received > out[groupID] {
			out[groupID] = received
		}
	}
	return out
}

// MainTask returns the current main task progress when namespace 22.0 has
// been observed.
func (s *State) MainTask() (MainTaskView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainTask == nil {
		return MainTaskView{}, false
	}
	return *s.mainTask, true
}

// MainTaskClaimSnapshot returns a fail-closed claim snapshot only when the
// current task, progress, receipt map, and exact catalog target are observed.
func (s *State) MainTaskClaimSnapshot() (MainTaskClaimSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainTask == nil {
		return MainTaskClaimSnapshot{}, false
	}
	view := *s.mainTask
	if !view.Observed || !view.Valid || view.Complete || !view.TaskIDObserved ||
		!view.ProgressObserved || !view.ReceiptObserved || view.Receipted {
		return MainTaskClaimSnapshot{}, false
	}
	definition, valid, complete := ResolveMainTaskDefinition(view.TaskID)
	if !valid || complete || definition.Target <= 0 || definition.NextTaskID <= 0 ||
		definition.Target != view.Target || definition.NextTaskID != view.NextTaskID || view.Finished < definition.Target {
		return MainTaskClaimSnapshot{}, false
	}
	return MainTaskClaimSnapshot{
		TaskID: view.TaskID, Target: definition.Target, NextTaskID: definition.NextTaskID, Finished: view.Finished,
	}, true
}

// MainTaskClaimApplied confirms a reward response only when it contains both
// the exact catalog task switch and a receipt key for the task that was claimed.
func (s *State) MainTaskClaimApplied(snapshot MainTaskClaimSnapshot) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	definition, valid, complete := ResolveMainTaskDefinition(snapshot.TaskID)
	if !valid || complete || definition.Target != snapshot.Target || definition.NextTaskID != snapshot.NextTaskID {
		return false
	}
	if !s.mainTaskRecvObserved {
		return false
	}
	if _, receipted := s.mainTaskReceipts[snapshot.TaskID]; !receipted {
		return false
	}
	if s.mainTask == nil || !s.mainTask.TaskIDObserved || !s.mainTask.Valid ||
		s.mainTask.TaskID != snapshot.NextTaskID {
		return false
	}
	return s.mainTask.Complete || s.mainTask.ProgressObserved
}

// StoryMain returns the observed main-story progress.
func (s *State) StoryMain() (StoryMainView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.storyMain.Observed {
		return StoryMainView{}, false
	}
	view := s.storyMain
	view.Cost = append([]ItemCount(nil), s.storyMain.Cost...)
	return view, true
}

// StoryMainObserved reports whether namespace 7.101 has been observed.
func (s *State) StoryMainObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.storyMain.Observed
}

// ReadyDailyTaskIDs returns daily task ids that look claimable from namespace
// 22. A status of 1 is treated as the client's explicit "ready" marker; when
// status is absent, completed target progress with no receipt is accepted.
func (s *State) ReadyDailyTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.dailyTasks))
	for id, task := range s.dailyTasks {
		if task != nil && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyWeeklyTaskIDs returns weekly task ids that look claimable from namespace
// 22.100 and the current c_task_week table.
func (s *State) ReadyWeeklyTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.weeklyTasks))
	for id, task := range s.weeklyTasks {
		if task != nil && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyAchievementTaskIDs returns achievement tasks that look claimable from
// namespace 22.2 and the current c_task_ach table.
func (s *State) ReadyAchievementTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.achievementTasks))
	for id, task := range s.achievementTasks {
		if task != nil && task.Current && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyRoadGrowTaskIDs returns growth-road rewards that can be claimed from
// the observed player state and client task table.
func (s *State) ReadyRoadGrowTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := RoadGrowLevelTasks()
	out := make([]int32, 0, len(tasks))
	for _, task := range tasks {
		if task.TaskID == 0 || s.roadGrowReceived[task.TaskID] {
			continue
		}
		if task.TargetLevel > 0 && s.level >= task.TargetLevel {
			out = append(out, task.TaskID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RoadGrowReceived returns a copy of the growth-road receipt map.
func (s *State) RoadGrowReceived() map[int32]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]bool, len(s.roadGrowReceived))
	for id, v := range s.roadGrowReceived {
		out[id] = v
	}
	return out
}

// ReadyRandomEventIDs returns structurally valid, catalog-known, cost-free map
// events in deterministic event-id order.
func (s *State) ReadyRandomEventIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.randomEventObserved || !s.randomEventMapValid {
		return nil
	}
	out := make([]int32, 0, len(s.randomEvents))
	for id, event := range s.randomEvents {
		if event != nil && event.Valid && event.EventID == id {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RandomEventObserved reports whether namespace 129 has been observed.
func (s *State) RandomEventObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.randomEventObserved
}

// RandomEventMapStatus reports whether the authoritative 129.0.1 table was
// observed and structurally decoded. A malformed replacement clears all
// executable candidates and supplies a diagnostic reason.
func (s *State) RandomEventMapStatus() (observed, valid bool, reason string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.randomEventObserved, s.randomEventMapValid, s.randomEventMapError
}

// RandomEventTableReady accepts both a valid non-empty table and a valid empty
// table, matching randomEvent.enter's postcondition.
func (s *State) RandomEventTableReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.randomEventObserved && s.randomEventMapValid
}

// RandomEvents returns the current map-random-event state.
func (s *State) RandomEvents() map[int32]RandomEventView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]RandomEventView, len(s.randomEvents))
	for id, event := range s.randomEvents {
		if event != nil {
			out[id] = *event
		}
	}
	return out
}

// RandomEventClaimSnapshot revalidates the exact event immediately before a
// doAffair request.
func (s *State) RandomEventClaimSnapshot(eventID int32) (RandomEventClaimSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if eventID <= 0 || !s.randomEventObserved || !s.randomEventMapValid {
		return RandomEventClaimSnapshot{}, false
	}
	event := s.randomEvents[eventID]
	if event == nil || !event.Valid || event.EventID != eventID || !event.CatalogKnown || !event.CostFree {
		return RandomEventClaimSnapshot{}, false
	}
	return RandomEventClaimSnapshot{
		EventID: event.EventID, PositionIndex: event.PositionIndex, DialogID: event.DialogID,
	}, true
}

// RandomEventClaimApplied requires the target to disappear from a subsequent
// authoritative, structurally valid whole-table replacement.
func (s *State) RandomEventClaimApplied(snapshot RandomEventClaimSnapshot) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if snapshot.EventID <= 0 || !s.randomEventObserved || !s.randomEventMapValid {
		return false
	}
	_, stillPresent := s.randomEvents[snapshot.EventID]
	return !stillPresent
}

// AchievementTasks returns a copy of tracked achievement task progress.
func (s *State) AchievementTasks() map[int32]AchievementTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]AchievementTaskView, len(s.achievementTasks))
	for id, task := range s.achievementTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

// DailyTasks returns a copy of tracked daily task progress.
func (s *State) DailyTasks() map[int32]DailyTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]DailyTaskView, len(s.dailyTasks))
	for id, task := range s.dailyTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

// WeeklyTasks returns a copy of tracked weekly task progress.
func (s *State) WeeklyTasks() map[int32]WeeklyTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]WeeklyTaskView, len(s.weeklyTasks))
	for id, task := range s.weeklyTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

func taskClaimable(status, target, finished, receipted int32) bool {
	if receipted != 0 {
		return false
	}
	return status == 1 || (status == 0 && target > 0 && finished >= target)
}
