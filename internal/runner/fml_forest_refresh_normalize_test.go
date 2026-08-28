package runner

import (
	"encoding/json"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestNormalizeFmlForestRefreshVWrapsBareFmlNamespace(t *testing.T) {
	bare := json.RawMessage(`{"127":{"0":11,"1":88,"8":{}}}`)
	s := state.New()
	s.ApplyV(normalizeFmlForestRefreshV(bare))
	if got := s.FmlForestEnergy(); !got.Observed || got.UID != 11 || got.FmlID != 88 {
		t.Fatalf("bare forest refresh=%+v, want observed energy", got)
	}
}

func TestNormalizeFmlForestRefreshVWrapsDirectEnergy(t *testing.T) {
	direct := json.RawMessage(`{"0":11,"1":88,"2":{"1":5},"8":{}}`)
	s := state.New()
	s.ApplyV(normalizeFmlForestRefreshV(direct))
	if got := s.FmlForestEnergy(); !got.Observed || got.EnergyByType[1] != 5 {
		t.Fatalf("direct forest refresh=%+v, want observed energy", got)
	}
}

func TestNormalizeFmlForestRefreshVPreservesTopLevelState(t *testing.T) {
	top := json.RawMessage(`{"7":{"0":{"0":11}}}`)
	if got := normalizeFmlForestRefreshV(top); string(got) != string(top) {
		t.Fatalf("unrelated top-level state changed: %s", got)
	}
}
