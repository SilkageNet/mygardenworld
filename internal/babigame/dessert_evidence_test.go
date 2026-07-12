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
	var document any
	if err := json.Unmarshal(dessertEvidenceManifestJSON, &document); err != nil {
		t.Fatal(err)
	}
	assertSanitizedDessertEvidenceValue(t, "", document)

	raw := strings.ToLower(string(dessertEvidenceManifestJSON))
	for _, forbidden := range []string{"1312", "http://", "https://", `:\\`, "/users/"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("manifest contains forbidden identity/path marker %q", forbidden)
		}
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
	if DessertRewardBoxEvidenceGate() {
		t.Fatal("reward-box evidence gate opened without recvBoxes/openBox captures")
	}
	if DessertLiveAutoplayEvidenceGate() {
		t.Fatal("live-autoplay evidence gate opened from replay evidence alone")
	}
}

func TestReadDessertCaptureEvidenceRejectsMissingUnknownAndTampered(t *testing.T) {
	original := dessertEvidenceManifestJSON
	t.Cleanup(func() { dessertEvidenceManifestJSON = original })

	if _, err := readDessertCaptureEvidence(nil, dessertEvidenceManifestSHA256); err == nil {
		t.Fatal("missing manifest was accepted")
	}
	dessertEvidenceManifestJSON = nil
	if DessertRewardBoxEvidenceGate() || DessertLiveAutoplayEvidenceGate() {
		t.Fatal("missing manifest opened an evidence gate")
	}

	tampered := append([]byte(nil), original...)
	tampered[0] ^= 1
	if _, err := readDessertCaptureEvidence(tampered, dessertEvidenceManifestSHA256); err == nil {
		t.Fatal("tampered manifest was accepted")
	}
	dessertEvidenceManifestJSON = tampered
	if DessertRewardBoxEvidenceGate() || DessertLiveAutoplayEvidenceGate() {
		t.Fatal("tampered manifest opened an evidence gate")
	}

	unknown := append([]byte(nil), original...)
	unknown = []byte(strings.Replace(string(unknown), `"activity": "dessert",`, `"activity": "dessert", "unknown": true,`, 1))
	sum := sha256.Sum256(unknown)
	if _, err := readDessertCaptureEvidence(unknown, hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("manifest with unknown evidence was accepted")
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
