package state

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// PassTaskTypeDaily / PassTaskTypeChallenge match c_flowerPassTask.taskType.
	PassTaskTypeDaily     int32 = 1
	PassTaskTypeChallenge int32 = 2

	// PassRwdTypeFree matches CONST.FLOWER_PASS.RWD_TYPE for recv RPC args.
	// Live rwdMap keys are "free"/"pay"/"addPay" (see passRwdTypeFromKey).
	PassRwdTypeFree   int32 = 1
	PassRwdTypePay    int32 = 2
	PassRwdTypeAddPay int32 = 3

	FlowerPassActTmpType      int32 = 4501
	FlowerElvesPassActTmpType int32 = 4601
)

// PassTaskDef is one c_flowerPassTask / c_flowerElvesPassTask row.
type PassTaskDef struct {
	ID        int32
	TaskType  int32
	Type      int32
	Param     int32
	Value     int32
	RewardExp int32
}

// PassTaskSlotView is one monitor/planner row for a pass task.
type PassTaskSlotView struct {
	TaskID       int32
	TaskType     int32
	ProgressType int32
	Param        int32
	Title        string
	Target       int32
	Progress     int32
	Received     bool
	CatalogKnown bool
	RewardExp    int32
	Ready        bool
}

// PassBoardView is the shared monitor snapshot for 花之密令 / 花灵密令.
type PassBoardView struct {
	Observed            bool
	Found               bool
	Bid                 int32
	Name                string
	Lvl                 int32
	Exp                 int32
	PassType            int32
	BuyLvl              int32
	LvlMax              int32
	Tasks               []PassTaskSlotView
	ReadyTaskCount      int32
	ReadyFreeLevelCount int32
	ReadyFreeLevels     []int32
}

type passRuntime struct {
	Bid      int32
	Lvl      int32
	Exp      int32
	PassType int32
	BuyLvl   int32
	// rwdMap keyed by rwdType string ("1","2","3") -> claimed levels.
	RwdMap map[int32]map[int32]struct{}
	// RwdMapObserved is true once field 6 has been applied (including {}).
	// Partial lvl/exp deltas omit field 6; treating that as "nothing claimed"
	// causes flowerPass.recv to spam already-claimed levels.
	RwdMapObserved bool
}

type passTaskRuntime struct {
	Bid               int32
	DailyTaskIDs      []int32
	ProgressDaily     map[string]int32
	ProgressChallenge map[string]int32
	RecvDaily         map[int32]int32
	RecvChallenge     map[int32]int32
	DailyIDsObserved  bool
	ProgressDObserved bool
	ProgressCObserved bool
	RecvDailyObserved bool
	RecvChalObserved  bool
}

func (s *State) applyFlowerPassLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if s.flowerPassByBid == nil {
		s.flowerPassByBid = make(map[int32]*passRuntime)
	}
	if s.flowerPassTaskByBid == nil {
		s.flowerPassTaskByBid = make(map[int32]*passTaskRuntime)
	}
	if rawMap, ok := fields["0"]; ok && !isJSONNull(rawMap) {
		applyPassMapLocked(s.flowerPassByBid, rawMap)
		s.flowerPassObserved = true
	}
	if rawTasks, ok := fields["1"]; ok && !isJSONNull(rawTasks) {
		applyPassTaskMapLocked(s.flowerPassTaskByBid, rawTasks)
		s.flowerPassObserved = true
	}
}

func (s *State) applyFlowerElvesPassLocked(rawPass, rawTasks json.RawMessage) {
	if s.flowerElvesPassByBid == nil {
		s.flowerElvesPassByBid = make(map[int32]*passRuntime)
	}
	if s.flowerElvesPassTaskByBid == nil {
		s.flowerElvesPassTaskByBid = make(map[int32]*passTaskRuntime)
	}
	if len(rawPass) > 0 && !isJSONNull(rawPass) {
		applyPassMapLocked(s.flowerElvesPassByBid, rawPass)
		s.flowerElvesPassObserved = true
	}
	if len(rawTasks) > 0 && !isJSONNull(rawTasks) {
		applyPassTaskMapLocked(s.flowerElvesPassTaskByBid, rawTasks)
		s.flowerElvesPassObserved = true
	}
}

func applyPassMapLocked(dst map[int32]*passRuntime, raw json.RawMessage) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return
	}
	for bidStr, rawPass := range entries {
		bid := atoi32(bidStr)
		if bid <= 0 {
			continue
		}
		if isJSONNull(rawPass) {
			delete(dst, bid)
			continue
		}
		view := dst[bid]
		if view == nil {
			view = &passRuntime{Bid: bid, RwdMap: map[int32]map[int32]struct{}{}}
			dst[bid] = view
		}
		applyPassRuntimeFields(view, rawPass)
	}
}

func applyPassRuntimeFields(view *passRuntime, raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
		view.Bid = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.Lvl = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.Exp = n
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.PassType = n
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.BuyLvl = n
	}
	if rawRwd, ok := fields["6"]; ok {
		// Null means "no change" in partial deltas — do not wipe claims.
		if !isJSONNull(rawRwd) {
			view.RwdMap = decodePassRwdMap(rawRwd)
			view.RwdMapObserved = true
		}
	}
}

func applyPassTaskMapLocked(dst map[int32]*passTaskRuntime, raw json.RawMessage) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return
	}
	for bidStr, rawTask := range entries {
		bid := atoi32(bidStr)
		if bid <= 0 {
			continue
		}
		if isJSONNull(rawTask) {
			delete(dst, bid)
			continue
		}
		view := dst[bid]
		if view == nil {
			view = &passTaskRuntime{
				Bid:               bid,
				ProgressDaily:     map[string]int32{},
				ProgressChallenge: map[string]int32{},
				RecvDaily:         map[int32]int32{},
				RecvChallenge:     map[int32]int32{},
			}
			dst[bid] = view
		}
		applyPassTaskRuntimeFields(view, rawTask)
	}
}

func applyPassTaskRuntimeFields(view *passTaskRuntime, raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
		view.Bid = n
	}
	if rawProg, ok := fields["2"]; ok {
		view.ProgressChallenge = decodePassProgressMap(rawProg)
		view.ProgressCObserved = true
	}
	if rawRecv, ok := fields["4"]; ok {
		view.RecvChallenge = readInt32RawMap(rawRecv)
		view.RecvChalObserved = true
	}
	if rawIDs, ok := fields["5"]; ok {
		view.DailyTaskIDs = decodeInt32ListFlexible(rawIDs)
		view.DailyIDsObserved = true
	}
	if rawProg, ok := fields["6"]; ok {
		view.ProgressDaily = decodePassProgressMap(rawProg)
		view.ProgressDObserved = true
	}
	if rawRecv, ok := fields["8"]; ok {
		view.RecvDaily = readInt32RawMap(rawRecv)
		view.RecvDailyObserved = true
	}
}

func decodePassRwdMap(raw json.RawMessage) map[int32]map[int32]struct{} {
	out := map[int32]map[int32]struct{}{}
	if len(raw) == 0 || isJSONNull(raw) {
		return out
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return out
	}
	for key, rawLevels := range values {
		rwdType := passRwdTypeFromKey(key)
		if rwdType <= 0 {
			continue
		}
		levels := map[int32]struct{}{}
		var list []int32
		if json.Unmarshal(rawLevels, &list) == nil {
			for _, lvl := range list {
				if lvl > 0 {
					levels[lvl] = struct{}{}
				}
			}
		} else {
			// Some payloads encode claimed levels as {"1":1,"2":1} instead of [1,2].
			var asMap map[string]json.RawMessage
			if json.Unmarshal(rawLevels, &asMap) == nil {
				for lvlStr, rawVal := range asMap {
					lvl := atoi32(lvlStr)
					if lvl <= 0 {
						continue
					}
					if n, ok := readInt32Raw(rawVal); ok && n == 0 {
						continue
					}
					levels[lvl] = struct{}{}
				}
			}
		}
		out[rwdType] = levels
	}
	return out
}

// passRwdTypeFromKey maps observed rwdMap keys. Live reLogin uses
// "free"/"pay"/"addPay"; numeric "1"/"2"/"3" are kept for fixtures/deltas.
func passRwdTypeFromKey(key string) int32 {
	switch key {
	case "free", "1":
		return PassRwdTypeFree
	case "pay", "2":
		return PassRwdTypePay
	case "addPay", "3":
		return PassRwdTypeAddPay
	default:
		return atoi32(key)
	}
}

func decodePassProgressMap(raw json.RawMessage) map[string]int32 {
	out := map[string]int32{}
	if len(raw) == 0 || isJSONNull(raw) {
		return out
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return out
	}
	for key, rawValue := range values {
		if n, ok := readInt32Raw(rawValue); ok {
			out[key] = n
		}
	}
	return out
}

func decodeInt32ListFlexible(raw json.RawMessage) []int32 {
	if len(raw) == 0 || isJSONNull(raw) {
		return nil
	}
	var list []int32
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var single int32
	if json.Unmarshal(raw, &single) == nil && single > 0 {
		return []int32{single}
	}
	var asMap map[string]json.RawMessage
	if json.Unmarshal(raw, &asMap) == nil {
		out := make([]int32, 0, len(asMap))
		for _, v := range asMap {
			if n, ok := readInt32Raw(v); ok && n > 0 {
				out = append(out, n)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}
	return nil
}

func passProgressKey(progressType, param int32) string {
	return fmt.Sprintf("%d_%d", progressType, param)
}

func passProgressLookup(m map[string]int32, key string, progressType int32) int32 {
	if m == nil {
		return 0
	}
	if n, ok := m[key]; ok {
		return n
	}
	// Some deltas key progress by bare type.
	if n, ok := m[strconv.FormatInt(int64(progressType), 10)]; ok {
		return n
	}
	return 0
}

// FlowerPassObserved reports whether namespace 131 has been applied.
func (s *State) FlowerPassObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flowerPassObserved
}

// FlowerElvesPassObserved reports whether namespace 132 pass fields have been applied.
func (s *State) FlowerElvesPassObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flowerElvesPassObserved
}

// FlowerPassView builds the monitor/planner snapshot for 花之密令.
func (s *State) FlowerPassView() PassBoardView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildPassBoardView(
		s.flowerPassObserved,
		s.flowerPassByBid,
		s.flowerPassTaskByBid,
		"c_flowerPass",
		"c_flowerPassTask",
		"c_flowerPassTerm",
	)
}

// FlowerElvesPassView builds the monitor/planner snapshot for 花灵密令.
func (s *State) FlowerElvesPassView() PassBoardView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildPassBoardView(
		s.flowerElvesPassObserved,
		s.flowerElvesPassByBid,
		s.flowerElvesPassTaskByBid,
		"c_flowerElvesPass",
		"c_flowerElvesPassTask",
		"c_flowerElvesPassTerm",
	)
}

func buildPassBoardView(
	observed bool,
	passes map[int32]*passRuntime,
	tasks map[int32]*passTaskRuntime,
	passTable, taskTable, termTable string,
) PassBoardView {
	out := PassBoardView{Observed: observed}
	bid := selectPassBid(passes, tasks)
	if bid <= 0 {
		return out
	}
	out.Found = true
	out.Bid = bid
	out.Name = passTermName(termTable, bid)
	out.LvlMax = passTermLvlMax(termTable, bid)

	if pass := passes[bid]; pass != nil {
		out.Lvl = pass.Lvl
		out.Exp = pass.Exp
		out.PassType = pass.PassType
		out.BuyLvl = pass.BuyLvl
		out.ReadyFreeLevels = readyFreePassLevels(pass, passTable, bid, out.LvlMax)
		out.ReadyFreeLevelCount = int32(len(out.ReadyFreeLevels))
	}

	taskRt := tasks[bid]
	defs := passTaskDefinitions(taskTable)
	slots := make([]PassTaskSlotView, 0, len(defs))
	for _, def := range defs {
		slot := passTaskSlotFromDef(def, taskRt)
		if def.TaskType == PassTaskTypeDaily {
			if taskRt == nil || !taskRt.DailyIDsObserved {
				continue
			}
			if !containsInt32(taskRt.DailyTaskIDs, def.ID) {
				continue
			}
		}
		slots = append(slots, slot)
		if slot.Ready {
			out.ReadyTaskCount++
		}
	}
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].TaskType != slots[j].TaskType {
			return slots[i].TaskType < slots[j].TaskType
		}
		return slots[i].TaskID < slots[j].TaskID
	})
	out.Tasks = slots
	return out
}

func passTaskSlotFromDef(def PassTaskDef, rt *passTaskRuntime) PassTaskSlotView {
	title, known := PassTaskTitle(def.Type, def.Value, def.Param)
	if title == "" {
		title = fmt.Sprintf("密令任务 #%d", def.ID)
	}
	slot := PassTaskSlotView{
		TaskID:       def.ID,
		TaskType:     def.TaskType,
		ProgressType: def.Type,
		Param:        def.Param,
		Title:        title,
		Target:       def.Value,
		CatalogKnown: known,
		RewardExp:    def.RewardExp,
	}
	key := passProgressKey(def.Type, def.Param)
	if rt != nil {
		switch def.TaskType {
		case PassTaskTypeChallenge:
			slot.Progress = passProgressLookup(rt.ProgressChallenge, key, def.Type)
			slot.Received = rt.RecvChallenge[def.ID] != 0
		default:
			slot.Progress = passProgressLookup(rt.ProgressDaily, key, def.Type)
			slot.Received = rt.RecvDaily[def.ID] != 0
		}
	}
	if !slot.Received && def.Value > 0 && slot.Progress >= def.Value {
		slot.Ready = true
	}
	return slot
}

func selectPassBid(passes map[int32]*passRuntime, tasks map[int32]*passTaskRuntime) int32 {
	ids := make([]int32, 0, len(passes)+len(tasks))
	seen := map[int32]struct{}{}
	for bid := range passes {
		if bid > 0 {
			seen[bid] = struct{}{}
			ids = append(ids, bid)
		}
	}
	for bid := range tasks {
		if _, ok := seen[bid]; ok || bid <= 0 {
			continue
		}
		ids = append(ids, bid)
	}
	if len(ids) == 0 {
		return 0
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	return ids[0]
}

func readyFreePassLevels(pass *passRuntime, passTable string, bid, lvlMax int32) []int32 {
	if pass == nil || pass.Lvl <= 0 || !pass.RwdMapObserved {
		return nil
	}
	claimed := pass.RwdMap[PassRwdTypeFree]
	max := pass.Lvl
	if lvlMax > 0 && max > lvlMax {
		max = lvlMax
	}
	out := make([]int32, 0, max)
	for lvl := int32(1); lvl <= max; lvl++ {
		if claimed != nil {
			if _, ok := claimed[lvl]; ok {
				continue
			}
		}
		if !passLevelHasFreeReward(passTable, bid, lvl) {
			continue
		}
		out = append(out, lvl)
	}
	return out
}

func passLevelHasFreeReward(table string, bid, lvl int32) bool {
	// c_flowerPass / c_flowerElvesPass rows use id = bid*1000+lvl in current catalog
	// (e.g. term 1 lvl 1 => 1001). Fall back to scanning time+lvl.
	if raw, ok := StaticRow(table, bid*1000+lvl); ok {
		return passRowHasFreeReward(raw)
	}
	tableData, ok := StaticTableByName(table)
	if !ok {
		return false
	}
	for _, raw := range tableData.Rows {
		var row struct {
			Time       int32           `json:"time"`
			Lvl        int32           `json:"lvl"`
			FreeReward json.RawMessage `json:"freeReward"`
		}
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		if row.Time == bid && row.Lvl == lvl {
			return len(row.FreeReward) > 0 && string(row.FreeReward) != "null" && string(row.FreeReward) != "[]"
		}
	}
	return false
}

func passRowHasFreeReward(raw json.RawMessage) bool {
	var row struct {
		FreeReward json.RawMessage `json:"freeReward"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return false
	}
	return len(row.FreeReward) > 0 && string(row.FreeReward) != "null" && string(row.FreeReward) != "[]"
}

func passTermName(table string, bid int32) string {
	raw, ok := StaticRow(table, bid)
	if !ok {
		return ""
	}
	var row struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return ""
	}
	return row.Name
}

func passTermLvlMax(table string, bid int32) int32 {
	raw, ok := StaticRow(table, bid)
	if !ok {
		return 0
	}
	var row struct {
		LvlMax int32 `json:"lvlMax"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return row.LvlMax
}

func passTaskDefinitions(table string) []PassTaskDef {
	data, ok := StaticTableByName(table)
	if !ok {
		return nil
	}
	out := make([]PassTaskDef, 0, len(data.Rows))
	for idStr, raw := range data.Rows {
		if idStr == "-1" {
			continue
		}
		var row struct {
			ID        int32 `json:"id"`
			TaskType  int32 `json:"taskType"`
			Type      int32 `json:"type"`
			Param     int32 `json:"param"`
			Value     int32 `json:"value"`
			Exp       int32 `json:"exp"`
			RewardExp int32 `json:"rewardExp"`
		}
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		id := row.ID
		if id <= 0 {
			id = atoi32(idStr)
		}
		if id <= 0 || row.Type <= 0 || row.Value <= 0 {
			continue
		}
		exp := row.RewardExp
		if exp == 0 {
			exp = row.Exp
		}
		taskType := row.TaskType
		if taskType == 0 {
			taskType = PassTaskTypeDaily
		}
		out = append(out, PassTaskDef{
			ID: id, TaskType: taskType, Type: row.Type, Param: row.Param,
			Value: row.Value, RewardExp: exp,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PassTaskTitle resolves c_task_type description for a pass progress type.
func PassTaskTitle(progressType, target, param int32) (string, bool) {
	raw, ok := StaticRow("c_task_type", progressType)
	if !ok {
		return "", false
	}
	var row struct {
		Desc string `json:"desc"`
	}
	if json.Unmarshal(raw, &row) != nil || strings.TrimSpace(row.Desc) == "" {
		return "", false
	}
	title := strings.ReplaceAll(row.Desc, "${value}", strconv.FormatInt(int64(target), 10))
	title = strings.ReplaceAll(title, "${param}", strconv.FormatInt(int64(param), 10))
	return title, true
}

func containsInt32(list []int32, want int32) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// ReadyFlowerPassTaskIDs returns claimable 花之密令 task ids for the active bid.
func (s *State) ReadyFlowerPassTaskIDs() (bid int32, taskIDs []int32) {
	view := s.FlowerPassView()
	return readyPassTaskIDs(view)
}

// ReadyFlowerElvesPassTaskIDs returns claimable 花灵密令 task ids for the active bid.
func (s *State) ReadyFlowerElvesPassTaskIDs() (bid int32, taskIDs []int32) {
	view := s.FlowerElvesPassView()
	return readyPassTaskIDs(view)
}

func readyPassTaskIDs(view PassBoardView) (int32, []int32) {
	if !view.Found || view.Bid <= 0 {
		return 0, nil
	}
	out := make([]int32, 0, len(view.Tasks))
	for _, task := range view.Tasks {
		if task.Ready {
			out = append(out, task.TaskID)
		}
	}
	return view.Bid, out
}

// ReadyFlowerPassFreeLevels returns free tier levels ready to claim.
func (s *State) ReadyFlowerPassFreeLevels() (bid int32, levels []int32) {
	view := s.FlowerPassView()
	if !view.Found {
		return 0, nil
	}
	return view.Bid, append([]int32(nil), view.ReadyFreeLevels...)
}

// ReadyFlowerElvesPassFreeLevels returns free tier levels ready to claim.
func (s *State) ReadyFlowerElvesPassFreeLevels() (bid int32, levels []int32) {
	view := s.FlowerElvesPassView()
	if !view.Found {
		return 0, nil
	}
	return view.Bid, append([]int32(nil), view.ReadyFreeLevels...)
}

// FlowerPassRwdMapObserved reports whether free/pay claim map has been synced.
func (s *State) FlowerPassRwdMapObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bid := selectPassBid(s.flowerPassByBid, s.flowerPassTaskByBid)
	if bid <= 0 {
		return false
	}
	pass := s.flowerPassByBid[bid]
	return pass != nil && pass.RwdMapObserved
}

// FlowerElvesPassRwdMapObserved reports whether elves pass claim map has been synced.
func (s *State) FlowerElvesPassRwdMapObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bid := selectPassBid(s.flowerElvesPassByBid, s.flowerElvesPassTaskByBid)
	if bid <= 0 {
		return false
	}
	pass := s.flowerElvesPassByBid[bid]
	return pass != nil && pass.RwdMapObserved
}

// NoteFlowerPassEnterSynced marks rwdMap as observed after a successful enter.
// Enter is authoritative even when field 6 is omitted (empty claims).
func (s *State) NoteFlowerPassEnterSynced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flowerPassEnterSynced = true
	notePassEnterSyncedLocked(s.flowerPassByBid, s.flowerPassTaskByBid)
}

// NoteFlowerElvesPassEnterSynced marks elves pass rwdMap observed after enter.
func (s *State) NoteFlowerElvesPassEnterSynced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flowerElvesPassEnterSynced = true
	notePassEnterSyncedLocked(s.flowerElvesPassByBid, s.flowerElvesPassTaskByBid)
}

// FlowerPassEnterSynced reports whether flowerPass.enter completed this session.
func (s *State) FlowerPassEnterSynced() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flowerPassEnterSynced
}

// FlowerElvesPassEnterSynced reports whether flowerElvesPass.enter completed this session.
func (s *State) FlowerElvesPassEnterSynced() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flowerElvesPassEnterSynced
}

func notePassEnterSyncedLocked(passes map[int32]*passRuntime, tasks map[int32]*passTaskRuntime) {
	bid := selectPassBid(passes, tasks)
	if bid <= 0 {
		return
	}
	pass := passes[bid]
	if pass == nil {
		pass = &passRuntime{Bid: bid, RwdMap: map[int32]map[int32]struct{}{}}
		passes[bid] = pass
	}
	if pass.RwdMap == nil {
		pass.RwdMap = map[int32]map[int32]struct{}{}
	}
	pass.RwdMapObserved = true
}

// MarkFlowerPassFreeRecvRejected records that free-tier recv was rejected for
// the active pass (typically already claimed / bad params). Marks every free
// level through current lvl so the planner stops retrying.
func (s *State) MarkFlowerPassFreeRecvRejected(bid int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	markPassFreeRecvRejectedLocked(s.flowerPassByBid, bid)
}

// MarkFlowerElvesPassFreeRecvRejected is the elves-pass counterpart.
func (s *State) MarkFlowerElvesPassFreeRecvRejected(bid int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	markPassFreeRecvRejectedLocked(s.flowerElvesPassByBid, bid)
}

func markPassFreeRecvRejectedLocked(passes map[int32]*passRuntime, bid int32) {
	if bid <= 0 || passes == nil {
		return
	}
	pass := passes[bid]
	if pass == nil {
		pass = &passRuntime{Bid: bid, RwdMap: map[int32]map[int32]struct{}{}}
		passes[bid] = pass
	}
	if pass.RwdMap == nil {
		pass.RwdMap = map[int32]map[int32]struct{}{}
	}
	claimed := pass.RwdMap[PassRwdTypeFree]
	if claimed == nil {
		claimed = map[int32]struct{}{}
		pass.RwdMap[PassRwdTypeFree] = claimed
	}
	max := pass.Lvl
	if max < 1 {
		max = 1
	}
	for lvl := int32(1); lvl <= max; lvl++ {
		claimed[lvl] = struct{}{}
	}
	pass.RwdMapObserved = true
}

// DailyTaskBoard builds the monitor board for daily tasks (includes claimed).
func (s *State) DailyTaskBoard() (observed bool, tasks []DailyTaskView) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.dailyTasks) == 0 {
		_, ns22 := s.rawNamespaces["22"]
		return ns22, nil
	}
	ids := make([]int32, 0, len(s.dailyTasks))
	for id := range s.dailyTasks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]DailyTaskView, 0, len(ids))
	for _, id := range ids {
		if task := s.dailyTasks[id]; task != nil {
			out = append(out, *task)
		}
	}
	return true, out
}
