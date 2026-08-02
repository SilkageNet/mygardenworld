package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestExecuteBenefitBoxDrawDrainsAccruedBoxes(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	resetAt := time.Date(2026, 7, 29, 20, 0, 0, 0, shanghai)
	now := time.Date(2026, 7, 30, 4, 30, 0, 0, shanghai)
	st := state.New()
	st.ApplyVMap(map[string]any{
		"116": map[string]any{
			"0": map[string]any{
				"1": 0,
				"2": resetAt.UnixMilli(),
			},
		},
	})
	if got := st.BenefitBoxDrawsRemaining(now); got != 8 {
		t.Fatalf("preflight remaining=%d, want 8", got)
	}

	var draws int
	exec := benefitBoxDrawExecution{
		remaining: st.BenefitBoxDrawsRemaining,
		observed:  st.BenefitBoxObserved,
		now:       func() time.Time { return now },
		draw: func(context.Context) (json.RawMessage, error) {
			draws++
			left := 8 - draws
			if left < 0 {
				left = 0
			}
			// Server returns absolute drawCnt after each open.
			raw, _ := json.Marshal(map[string]any{
				"116": map[string]any{
					"0": map[string]any{
						"1": left,
						"2": now.UnixMilli(),
					},
				},
			})
			return raw, nil
		},
		apply: st.ApplyV,
	}

	raw, err := executeBenefitBoxDraw(context.Background(), exec)
	if err != nil {
		t.Fatalf("executeBenefitBoxDraw: %v", err)
	}
	if draws != 8 {
		t.Fatalf("draw calls=%d, want 8", draws)
	}
	var result benefitBoxDrawResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Opened != 8 {
		t.Fatalf("opened=%d, want 8", result.Opened)
	}
	if st.BenefitBoxDrawsRemaining(now) != 0 {
		t.Fatalf("remaining after drain=%d, want 0", st.BenefitBoxDrawsRemaining(now))
	}
}

func TestExecuteBenefitBoxDrawStopsWhenEmpty(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"116": map[string]any{"0": map[string]any{"1": 0, "2": int64(1)}},
	})
	exec := benefitBoxDrawExecution{
		remaining: st.BenefitBoxDrawsRemaining,
		observed:  st.BenefitBoxObserved,
		now:       time.Now,
		draw: func(context.Context) (json.RawMessage, error) {
			return nil, fmt.Errorf("should not draw")
		},
		apply: st.ApplyV,
	}
	if _, err := executeBenefitBoxDraw(context.Background(), exec); err == nil {
		t.Fatal("expected error when getBenefitBoxInfo reports 0")
	}
}

func TestExecuteBenefitBoxDrawBootstrapsWhenUnobserved(t *testing.T) {
	st := state.New()
	calls := 0
	exec := benefitBoxDrawExecution{
		remaining: st.BenefitBoxDrawsRemaining,
		observed:  st.BenefitBoxObserved,
		now:       time.Now,
		draw: func(context.Context) (json.RawMessage, error) {
			calls++
			raw, _ := json.Marshal(map[string]any{
				"116": map[string]any{"0": map[string]any{"1": 0, "2": time.Now().UnixMilli()}},
			})
			return raw, nil
		},
		apply: st.ApplyV,
	}
	raw, err := executeBenefitBoxDraw(context.Background(), exec)
	if err != nil {
		t.Fatalf("bootstrap draw: %v", err)
	}
	if calls != 1 {
		t.Fatalf("draw calls=%d, want 1 bootstrap", calls)
	}
	var result benefitBoxDrawResult
	if err := json.Unmarshal(raw, &result); err != nil || result.Opened != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
