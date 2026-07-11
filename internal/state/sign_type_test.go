package state

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestBaseRewardAntiFraudGateFromCapture(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"7":{"2":{"0":77900091102482,"1":2,"2":2,"3":1780411445819,"4":1778405962518}}}}`))
	view, observed := s.BaseReward(BaseRewardAntiFraud)
	if !observed || !view.Observed || !view.Valid || view.Type != BaseRewardAntiFraud ||
		view.Status != BaseRewardStatusReceived || view.UpdatedAtMs != 1780411445819 {
		t.Fatalf("base reward = %+v, observed=%t", view, observed)
	}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if view.UpdatedToday(now) || !view.UpdatedBeforeToday(now) {
		t.Fatalf("captured base reward day gate mismatch: %+v", view)
	}

	// G.IRwd deltas are sparse and retain their validated type identity.
	s.ApplyV(json.RawMessage(fmt.Sprintf(`{"7":{"7":{"2":{"2":1,"3":%d}}}}`, now.Add(-time.Hour).UnixMilli())))
	view, _ = s.BaseReward(BaseRewardAntiFraud)
	if !view.Valid || view.Status != 1 || !view.UpdatedToday(now) || view.UID != 77900091102482 {
		t.Fatalf("sparse base reward delta = %+v", view)
	}
}

func TestBaseRewardMalformedStateFailsClosed(t *testing.T) {
	cases := []string{
		`{"7":{"7":{"2":{"2":1.5}}}}`,
		`{"7":{"7":{"2":{"1":3}}}}`,
		`{"7":{"7":{"2":{"3":"bad"}}}}`,
		`{"7":{"7":{"02":{"1":2,"2":2,"3":100}}}}`,
		`{"7":{"7":[]}}`,
	}
	for _, delta := range cases {
		s := New()
		s.ApplyV(json.RawMessage(`{"7":{"7":{"2":{"0":123,"1":2,"2":2,"3":100,"4":1}}}}`))
		s.ApplyV(json.RawMessage(delta))
		view, observed := s.BaseReward(BaseRewardAntiFraud)
		if !observed || view.Valid {
			t.Fatalf("malformed base reward remained valid: %s => %+v", delta, view)
		}
	}
}

func TestSignTypeSparseStateMachineFromCapture(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"0":77900091102482,"1":1,"2":1783215003424,"3":1,"4":0,"5":1783215003424,"6":1778405962460}}}}`))

	view, namespaceObserved := s.SignType(SignTypeAntiFraud)
	if !namespaceObserved || !view.Observed || !view.Valid || view.Type != SignTypeAntiFraud ||
		!view.TypeObserved || view.SignID != 1 || !view.SignIDObserved ||
		view.Status != SignTypeStatusCanSign || !view.StatusObserved {
		t.Fatalf("initial sign type = %+v, namespace observed=%t", view, namespaceObserved)
	}

	// Captured signType.sign response only carries lTime/status/uTime.
	s.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"2":1783696482105,"4":1,"5":1783696482105}}}}`))
	view, _ = s.SignType(SignTypeAntiFraud)
	if !view.Valid || view.Status != SignTypeStatusCanReceive || view.SignID != 1 || view.UID != 77900091102482 || view.CreatedAtMs != 1778405962460 {
		t.Fatalf("after sign sparse delta = %+v", view)
	}

	// Captured signType.recv response has the same sparse shape and status=2.
	s.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"2":1783696482147,"4":2,"5":1783696482147}}}}`))
	view, _ = s.SignType(SignTypeAntiFraud)
	if !view.Valid || view.Status != SignTypeStatusReceived || view.SignID != 1 {
		t.Fatalf("after recv sparse delta = %+v", view)
	}
	if s.UnknownNamespaceCount() != 0 {
		t.Fatalf("namespace 140 counted as unmodeled: %d", s.UnknownNamespaceCount())
	}
}

func TestSignTypeMalformedStateFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		delta string
	}{
		{name: "fractional status", delta: `{"140":{"0":{"1":{"4":0.5}}}}`},
		{name: "unknown status", delta: `{"140":{"0":{"1":{"4":3}}}}`},
		{name: "mismatched type", delta: `{"140":{"0":{"1":{"1":2}}}}`},
		{name: "unknown reward", delta: `{"140":{"0":{"1":{"3":999999}}}}`},
		{name: "null record", delta: `{"140":{"0":{"1":null}}}`},
		{name: "noncanonical key", delta: `{"140":{"0":{"01":{"1":1,"3":1,"4":0}}}}`},
		{name: "malformed map", delta: `{"140":{"0":[]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"0":123,"1":1,"2":100,"3":1,"4":0,"5":100,"6":1}}}}`))
			s.ApplyV(json.RawMessage(tc.delta))
			view, observed := s.SignType(SignTypeAntiFraud)
			if !observed || view.Valid {
				t.Fatalf("malformed delta remained actionable: observed=%t view=%+v", observed, view)
			}
		})
	}
}

func TestSignTypeSparseOtherTypeDoesNotEraseAutomatedType(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":0}}}}`))
	s.ApplyV(json.RawMessage(`{"140":{"0":{"4":{"0":123,"1":4,"3":31,"4":0}}}}`))
	view, observed := s.SignType(SignTypeAntiFraud)
	if !observed || !view.Valid || view.Status != SignTypeStatusCanSign {
		t.Fatalf("sparse type=4 delta erased type=1: observed=%t view=%+v", observed, view)
	}
}

func TestSignTypeCatalogAndCode3500Semantics(t *testing.T) {
	reward, ok := SignTypeRewardByID(1)
	if !ok || reward.ID != 1 || reward.Type != SignTypeAntiFraud || len(reward.Reward) != 1 ||
		reward.Reward[0].ItemID != 1223 || reward.Reward[0].Count != 1 {
		t.Fatalf("c_signReward[1] = %+v, ok=%t", reward, ok)
	}
	messages := map[int32]string{
		3500: "条件已达成，无需重复操作",
		3501: "今日奖励已领取",
		3502: "未达成领取条件，无法获取奖励",
		3503: "功能暂未解锁",
	}
	for code, text := range messages {
		raw, exists := StaticRow("c_msgCode", code)
		if !exists {
			t.Fatalf("missing c_msgCode[%d]", code)
		}
		var row struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &row) != nil || row.Text != text {
			t.Fatalf("c_msgCode[%d] = %s", code, raw)
		}
	}
}

func TestSignTypeEnterAttemptIsLocalDailyDedupOnly(t *testing.T) {
	s := New()
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if s.SignTypeEnterAttemptedToday(SignTypeAntiFraud, now) {
		t.Fatal("fresh state reported an enter attempt")
	}
	s.MarkSignTypeEnterAttempt(SignTypeAntiFraud, now)
	if !s.SignTypeEnterAttemptedToday(SignTypeAntiFraud, now.Add(3*time.Hour)) {
		t.Fatal("same-day enter attempt was not deduplicated")
	}
	if s.SignTypeEnterAttemptedToday(SignTypeAntiFraud, now.Add(24*time.Hour)) {
		t.Fatal("previous-day enter attempt suppressed the next day")
	}
	view, observed := s.SignType(SignTypeAntiFraud)
	if observed || view.Observed || view.Valid {
		t.Fatalf("local enter marker inferred server sign state: observed=%t view=%+v", observed, view)
	}
}
