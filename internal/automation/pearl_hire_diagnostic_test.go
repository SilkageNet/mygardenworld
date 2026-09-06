package automation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func pearlDiagnosticPolicy(limit int32) *pb.Policy {
	return &pb.Policy{AutomationEnabled: true, Basic: &pb.BasicPolicy{Pearl: &pb.PearlPolicy{
		AutoHireEnabled: true, MaxHireTicketUsage: 2, MaxHireLevel: limit,
	}}}
}

func TestPearlHireDiagnosticLevelBoundary(t *testing.T) {
	for _, tc := range []struct {
		name         string
		level, limit int32
		wantReady    bool
		wantReason   string
	}{
		{"below", 9, 10, true, "eligible"},
		{"equal", 10, 10, true, "eligible"},
		{"above", 11, 10, false, "level_exceeded"},
		{"unlimited", 99, 0, true, "eligible"},
		{"unknown is not low level", 0, 10, false, "level_unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newPearlHireStateForTest(t, 9001)
			applyPearlFriendForTest(t, s, 9001, 2001)
			applyMap(t, s, map[string]any{
				"28":  map[string]any{"5": []any{map[string]any{"0": 2001, "4": tc.level, "5": 99}}},
				"115": map[string]any{"5": map[string]any{"2001": 0}, "6": []any{}},
			})
			p, now := pearlDiagnosticPolicy(tc.limit), time.Now()
			d := DiagnosePearlHire(s, p, now)
			op, _ := PlanOneSafePearlHire(s, p.Basic.Pearl, now, PearlHireIntent{})
			if (d.Status == "ready") != tc.wantReady || d.NextOperation != op.Kind {
				t.Fatalf("diagnostic and planner disagree: %+v; op=%+v", d, op)
			}
			if d.Sources[0].Reasons[tc.wantReason] != 1 {
				t.Fatalf("wrong level classification: %+v", d.Sources[0])
			}
			if tc.wantReady {
				if err := ValidateSafePearlHire(s, p.Basic.Pearl, &op, now); err != nil {
					t.Fatal(err)
				}
				if d.Sources[1].Checked || d.Sources[2].Checked {
					t.Fatal("unvisited sources were reported checked")
				}
			} else if !d.Sources[1].Checked || !d.Sources[2].Checked {
				t.Fatalf("fresh empty sources were not reported checked: %+v", d)
			}
		})
	}
}

func TestPearlHireOverLevelDoesNotRequireProtectionOrBlockRecommendation(t *testing.T) {
	s := newPearlHireStateForTest(t, 9001)
	applyPearlFriendForTest(t, s, 9001, 2001)
	applyMap(t, s, map[string]any{
		"28": map[string]any{"5": []any{
			map[string]any{"0": 2001, "4": 11}, map[string]any{"0": 2002, "4": 10},
		}},
		"115": map[string]any{"5": map[string]any{"2002": 0}, "6": []any{2002}},
	})
	d := DiagnosePearlHire(s, pearlDiagnosticPolicy(10), time.Now())
	if d.Status != "ready" || d.TargetUID != 2002 || d.Sources[0].Reasons["level_exceeded"] != 1 || d.Sources[1].Reasons["eligible"] != 1 {
		t.Fatalf("over-level friend blocked valid recommendation: %+v", d)
	}
}

func TestPearlHireIncompleteCandidateDoesNotBlockKnownCandidate(t *testing.T) {
	for _, missing := range []string{"profile", "protection"} {
		t.Run(missing, func(t *testing.T) {
			s := newPearlHireStateForTest(t, 9001)
			applyPearlFriendForTest(t, s, 9001, 2001)
			applyMap(t, s, map[string]any{
				"24":  map[string]any{"1": []any{map[string]any{"0": 9001, "1": 2002}}},
				"28":  map[string]any{"5": []any{map[string]any{"0": 2002, "4": 10}}},
				"115": map[string]any{"5": map[string]any{"2002": 0}},
			})
			if missing == "protection" {
				applyMap(t, s, map[string]any{"28": map[string]any{"5": []any{map[string]any{"0": 2001, "4": 9}}}})
			}
			d := DiagnosePearlHire(s, pearlDiagnosticPolicy(10), time.Now())
			if d.Status != "ready" || d.TargetUID != 2002 || d.Sources[0].Reasons[missing+"_unavailable"] != 1 {
				t.Fatalf("incomplete candidate blocked ready one: %+v", d)
			}
		})
	}
}

func TestPearlHireDiagnosticUnknownDoesNotClaimNoMatchingLevel(t *testing.T) {
	s := newPearlHireStateForTest(t, 9001)
	applyPearlFriendForTest(t, s, 9001, 2001)
	d := DiagnosePearlHire(s, pearlDiagnosticPolicy(10), time.Now())
	if d.Status != "sync" || d.NextOperation != clientproto.RPCOpptGetDetailOppts.String() || d.Sources[0].Reasons["profile_unavailable"] != 1 || d.Sources[0].LevelChecked != 0 || d.Sources[1].Checked {
		t.Fatalf("unknown profile mislabeled: %+v", d)
	}
	if !strings.Contains(d.Summary(), "详情缺失/过期1") || !strings.Contains(d.Summary(), "推荐未检查") {
		t.Fatal(d.Summary())
	}
	p := pearlDiagnosticPolicy(10)
	p.Basic.Pearl.AutoHireEnabled = false
	d = DiagnosePearlHire(s, p, time.Now())
	if d.Status != "disabled" || d.Sources[0].Checked {
		t.Fatalf("disabled diagnostic performed filtering: %+v", d)
	}
}

func TestPearlHireDiagnosticBoundedExamplesAndExactUID(t *testing.T) {
	d := PearlHireSourceDiagnostic{}
	const uid int64 = 9007199254740993
	for i := 0; i < 1000; i++ {
		d.record(uid+int64(i), "level_exceeded", 50)
	}
	if d.Reasons["level_exceeded"] != 1000 || len(d.Examples) != 1 {
		t.Fatalf("unbounded or incorrect samples: %+v", d)
	}
	raw, err := json.Marshal(d)
	if err != nil || !strings.Contains(string(raw), `"uid":"9007199254740993"`) {
		t.Fatalf("UID was not preserved losslessly: %s, %v", raw, err)
	}
}
