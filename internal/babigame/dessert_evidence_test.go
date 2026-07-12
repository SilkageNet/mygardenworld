package babigame

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestDessertCaptureEvidenceManifestIsSanitized(t *testing.T) {
	for name, raw := range map[string][]byte{
		"manifest":  dessertEvidenceManifestJSON,
		"open_box":  dessertOpenBoxEvidenceFixtureJSON,
		"lifecycle": dessertLifecycleEvidenceFixtureJSON,
	} {
		t.Run(name, func(t *testing.T) {
			var document any
			if err := json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			assertSanitizedDessertEvidenceValue(t, "", document)

			lower := strings.ToLower(string(raw))
			for _, forbidden := range []string{"1312", "http://", "https://", `:\\`, "/users/"} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("evidence contains forbidden identity/path marker %q", forbidden)
				}
			}
		})
	}
}

func TestDessertCaptureEvidenceGatesFailClosed(t *testing.T) {
	evidence, err := ReadDessertCaptureEvidence()
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.Replay.Verified || evidence.Replay.RequestCount != 181 ||
		evidence.Replay.DropCount != 100 || evidence.Replay.MergeCount != 81 {
		t.Fatalf("unexpected replay summary: %+v", evidence.Replay)
	}
	if !evidence.Replay.ResponseOrderVerified || !evidence.Replay.TopologyVerified ||
		!evidence.Replay.CheckpointRebuildVerified || evidence.Replay.CausalSequenceVerified ||
		evidence.Replay.TrajectoryVerified {
		t.Fatalf("unexpected replay verification boundary: %+v", evidence.Replay)
	}
	if !evidence.RewardBoxes.OpenBoxSuccess || evidence.RewardBoxes.OpenBoxRequestNum != 1 ||
		evidence.RewardBoxes.RecvBoxesSuccess {
		t.Fatalf("unexpected reward evidence boundary: %+v", evidence.RewardBoxes)
	}
	if !evidence.LiveAutoplay.GameStartFromIdleSuccess || !evidence.LiveAutoplay.NaturalEndSuccess ||
		!evidence.LiveAutoplay.TerminalGameOverSuccess || evidence.LiveAutoplay.PhaseThreeSuccess {
		t.Fatalf("unexpected lifecycle evidence boundary: %+v", evidence.LiveAutoplay)
	}
	if !DessertOpenRewardBoxEvidenceGate() {
		t.Fatal("single-box evidence gate remained closed after a reviewed fixture")
	}
	if DessertProgressBoxEvidenceGate() {
		t.Fatal("progress-box gate opened without an act.recvBoxes fixture")
	}
	if DessertRewardBoxEvidenceGate() {
		t.Fatal("combined reward-box gate opened without act.recvBoxes evidence")
	}
	if DessertLiveAutoplayEvidenceGate() {
		t.Fatal("live-autoplay evidence gate opened from replay evidence alone")
	}
}

func TestDessertEvidenceFixturesRequireReviewedSemantics(t *testing.T) {
	if !validDessertOpenBoxEvidenceFixture(dessertOpenBoxEvidenceFixtureJSON) {
		t.Fatal("reviewed open-box evidence fixture was rejected")
	}
	if !validDessertLifecycleEvidenceFixture(dessertLifecycleEvidenceFixtureJSON) {
		t.Fatal("reviewed lifecycle evidence fixture was rejected")
	}

	wrongBoxCount := []byte(strings.Replace(string(dessertOpenBoxEvidenceFixtureJSON), `"num": 1`, `"num": 2`, 1))
	if validDessertOpenBoxEvidenceFixture(wrongBoxCount) {
		t.Fatal("multi-box fixture passed single-box semantics")
	}
	wrongBalance := []byte(strings.Replace(string(dessertOpenBoxEvidenceFixtureJSON), `"reward_boxes": 4`, `"reward_boxes": 3`, 1))
	if validDessertOpenBoxEvidenceFixture(wrongBalance) {
		t.Fatal("non-unit reward-box decrement passed fixture semantics")
	}
	changedMode := []byte(strings.Replace(string(dessertOpenBoxEvidenceFixtureJSON), `"mode_unchanged": true`, `"mode_unchanged": false`, 1))
	if validDessertOpenBoxEvidenceFixture(changedMode) {
		t.Fatal("game-state-changing openBox fixture passed independent-action semantics")
	}

	shortLineHold := []byte(strings.Replace(string(dessertLifecycleEvidenceFixtureJSON), `"max_line_hold_ms": 5001`, `"max_line_hold_ms": 4999`, 1))
	if validDessertLifecycleEvidenceFixture(shortLineHold) {
		t.Fatal("pre-threshold checkpoint passed natural-terminal semantics")
	}
	unknown := []byte(strings.Replace(string(dessertOpenBoxEvidenceFixtureJSON), `"schema":`, `"unknown": true, "schema":`, 1))
	if validDessertOpenBoxEvidenceFixture(unknown) {
		t.Fatal("fixture with unknown fields passed strict decoding")
	}
}

func TestReadDessertCaptureEvidenceRejectsMissingUnknownAndTampered(t *testing.T) {
	original := dessertEvidenceManifestJSON
	originalOpenBox := dessertOpenBoxEvidenceFixtureJSON
	t.Cleanup(func() {
		dessertEvidenceManifestJSON = original
		dessertOpenBoxEvidenceFixtureJSON = originalOpenBox
	})

	if _, err := readDessertCaptureEvidence(nil, dessertEvidenceManifestSHA256); err == nil {
		t.Fatal("missing manifest was accepted")
	}
	dessertEvidenceManifestJSON = nil
	if DessertOpenRewardBoxEvidenceGate() || DessertProgressBoxEvidenceGate() || DessertRewardBoxEvidenceGate() || DessertLiveAutoplayEvidenceGate() {
		t.Fatal("missing manifest opened an evidence gate")
	}

	tampered := append([]byte(nil), original...)
	tampered[0] ^= 1
	if _, err := readDessertCaptureEvidence(tampered, dessertEvidenceManifestSHA256); err == nil {
		t.Fatal("tampered manifest was accepted")
	}
	dessertEvidenceManifestJSON = tampered
	if DessertOpenRewardBoxEvidenceGate() || DessertProgressBoxEvidenceGate() || DessertRewardBoxEvidenceGate() || DessertLiveAutoplayEvidenceGate() {
		t.Fatal("tampered manifest opened an evidence gate")
	}

	unknown := append([]byte(nil), original...)
	unknown = []byte(strings.Replace(string(unknown), `"activity": "dessert",`, `"activity": "dessert", "unknown": true,`, 1))
	sum := sha256.Sum256(unknown)
	if _, err := readDessertCaptureEvidence(unknown, hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("manifest with unknown evidence was accepted")
	}

	dessertEvidenceManifestJSON = original
	dessertOpenBoxEvidenceFixtureJSON = append([]byte(nil), originalOpenBox...)
	dessertOpenBoxEvidenceFixtureJSON[0] ^= 1
	if DessertOpenRewardBoxEvidenceGate() {
		t.Fatal("tampered open-box fixture opened its evidence gate")
	}
}

func assertSanitizedDessertEvidenceValue(t *testing.T, key string, value any) {
	t.Helper()
	normalizedKey := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key))
	for _, forbidden := range []string{"uid", "batch", "time", "timestamp", "path", "flow", "frame", "url"} {
		if strings.Contains(normalizedKey, forbidden) {
			t.Fatalf("manifest contains forbidden key %q", key)
		}
	}
	switch typed := value.(type) {
	case map[string]any:
		for nestedKey, nestedValue := range typed {
			assertSanitizedDessertEvidenceValue(t, nestedKey, nestedValue)
		}
	case []any:
		for _, nestedValue := range typed {
			assertSanitizedDessertEvidenceValue(t, "", nestedValue)
		}
	case float64:
		if math.Abs(typed) >= 1_000_000_000 {
			t.Fatalf("manifest contains UID/absolute-time-sized number %.0f", typed)
		}
	}
}
