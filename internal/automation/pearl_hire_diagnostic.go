package automation

import (
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// PearlHireDiagnostic is a bounded trace of the actual planner, not a second
// eligibility implementation. Unvisited sources must never be reported empty.
type PearlHireDiagnostic struct {
	AutomationEnabled bool                        `json:"automationEnabled"`
	AutoHireEnabled   bool                        `json:"autoHireEnabled"`
	MaxLevel          int32                       `json:"maxLevel"`
	MaxWorkers        int32                       `json:"maxWorkers"`
	DailyLimit        int32                       `json:"dailyLimit"`
	ResourcesObserved bool                        `json:"resourcesObserved"`
	Tickets           int32                       `json:"tickets"`
	UsedToday         int32                       `json:"usedToday"`
	ActiveWorkers     int32                       `json:"activeWorkers"`
	WorkersKnown      bool                        `json:"workersKnown"`
	Status            string                      `json:"status"`
	Reason            string                      `json:"reason"`
	NextOperation     string                      `json:"nextOperation,omitempty"`
	TargetUID         int64                       `json:"targetUid,omitempty,string"`
	PlaceID           int32                       `json:"placeId,omitempty"`
	Sources           []PearlHireSourceDiagnostic `json:"sources"`
}

type PearlHireSourceDiagnostic struct {
	Source       string                      `json:"source"`
	Checked      bool                        `json:"checked"`
	Candidates   int                         `json:"candidates"`
	LevelChecked int                         `json:"levelChecked"`
	LevelMatched int                         `json:"levelMatched"`
	MinLevel     int32                       `json:"minLevel"`
	MaxLevel     int32                       `json:"maxLevel"`
	Reasons      map[string]int              `json:"reasons"`
	Examples     []PearlHireCandidateExample `json:"examples,omitempty"`
}

type PearlHireCandidateExample struct {
	UID    int64  `json:"uid,string"`
	Level  int32  `json:"level,omitempty"`
	Reason string `json:"reason"`
}

// DiagnosePearlHire only evaluates existing in-memory observations. It neither
// fetches more candidates nor sends a hire; "ready" means waiting for scheduling.
func DiagnosePearlHire(s *state.State, policy *pb.Policy, now time.Time) PearlHireDiagnostic {
	p := policy.GetBasic().GetPearl()
	d := PearlHireDiagnostic{
		AutomationEnabled: policy.GetAutomationEnabled(), AutoHireEnabled: p.GetAutoHireEnabled(),
		MaxLevel: p.GetMaxHireLevel(), MaxWorkers: p.GetMaxHireTicketUsage(), DailyLimit: p.GetDailyHireTicketLimit(),
		Status: "disabled", Reason: "自动雇佣开关已关闭，未筛选候选",
		Sources: []PearlHireSourceDiagnostic{{Source: "friend"}, {Source: "recommend"}, {Source: "enemy"}},
	}
	if !d.AutoHireEnabled {
		return d
	}
	if !d.AutomationEnabled {
		d.Status, d.Reason = "paused", "账号自动化已暂停，未筛选候选"
		return d
	}
	if s == nil {
		d.Status, d.Reason = "blocked", "账号状态尚未同步，未筛选候选"
		return d
	}
	op, ok := planOneSafePearlHire(s, p, now, PearlHireIntent{}, &d)
	if !ok {
		return d
	}
	d.Reason = op.Reason
	d.NextOperation = op.Kind
	d.Status = "sync"
	if !op.Executable {
		d.Status = "blocked"
	} else if op.Kind == clientproto.RPCPearlPlaceHire.String() {
		d.Status = "ready"
		d.TargetUID, d.PlaceID = op.TargetUID, op.TargetID
		d.Reason = "候选通过筛选，等待调度；不代表已雇佣成功"
	}
	return d
}

func (d *PearlHireDiagnostic) source(index int) *PearlHireSourceDiagnostic {
	if d == nil {
		return nil
	}
	return &d.Sources[index]
}

func (d *PearlHireSourceDiagnostic) observeLevel(level, limit int32) {
	if d == nil {
		return
	}
	d.LevelChecked++
	if d.MinLevel == 0 || level < d.MinLevel {
		d.MinLevel = level
	}
	d.MaxLevel = max(d.MaxLevel, level)
	if limit == 0 || level <= limit {
		d.LevelMatched++
	}
}

func (d *PearlHireSourceDiagnostic) record(uid int64, reason string, level int32) {
	if d == nil {
		return
	}
	if d.Reasons == nil {
		d.Reasons = make(map[string]int)
	}
	d.Reasons[reason]++
	// At most one example per reason and eight per source. No names, full
	// profiles, credentials or unbounded candidate lists enter the event log.
	if d.Reasons[reason] == 1 && len(d.Examples) < 8 && uid > 0 {
		d.Examples = append(d.Examples, PearlHireCandidateExample{UID: uid, Level: level, Reason: reason})
	}
}

var pearlDiagnosticReasons = []struct{ key, label string }{
	{"eligible", "可雇佣"}, {"level_exceeded", "超过等级上限"},
	{"profile_unavailable", "详情缺失/过期"}, {"level_unknown", "等级未知"},
	{"protection_unavailable", "保护状态缺失/过期"}, {"protected", "在岗/保护期"},
	{"protection_invalid", "保护时间异常"}, {"in_slot", "本方槽位已有"},
	{"failure_cooldown", "失败冷却"}, {"session_skip", "会话内跳过"},
	{"self_or_invalid", "自己/无效UID"}, {"duplicate", "重复"},
}

// Summary keeps the counts readable in the console and normal Web logs. The
// event payload adds bounded UID/level examples for protocol investigation.
func (d PearlHireDiagnostic) Summary() string {
	level := fmt.Sprintf("≤%d", d.MaxLevel)
	if d.MaxLevel == 0 {
		level = "不限"
	}
	parts := []string{fmt.Sprintf("珍珠雇佣诊断：等级%s，在岗上限%d，每日用券上限%d（0=不限）；%s", level, d.MaxWorkers, d.DailyLimit, d.Reason)}
	if d.ResourcesObserved {
		parts = append(parts, fmt.Sprintf("雇佣券%d，今日已用%d", d.Tickets, d.UsedToday))
	}
	for _, source := range d.Sources {
		label := map[string]string{"friend": "好友", "recommend": "推荐", "enemy": "三日内仇人"}[source.Source]
		if !source.Checked {
			parts = append(parts, label+"未检查")
			continue
		}
		detail := fmt.Sprintf("%s%d人，有效等级%d人", label, source.Candidates, source.LevelChecked)
		if source.LevelChecked > 0 {
			detail += fmt.Sprintf("（%d–%d级，满足等级%d人）", source.MinLevel, source.MaxLevel, source.LevelMatched)
		}
		for _, reason := range pearlDiagnosticReasons {
			if n := source.Reasons[reason.key]; n > 0 {
				detail += fmt.Sprintf("，%s%d", reason.label, n)
			}
		}
		parts = append(parts, detail)
	}
	return strings.Join(parts, "；")
}
