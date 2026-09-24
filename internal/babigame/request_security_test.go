package babigame

import (
	"strings"
	"testing"
)

func TestRequestSecuritySelfCheckCanonical(t *testing.T) {
	// Mirrors RequestSecurity.selfCheckCanonical from moac_rn 3.3.2.54.
	inner := map[string]any{
		"appId":             "045243c82cf3bcbdd64bd0ef90c9e6a3",
		"name":              "placeholder",
		"value":             "placeholder",
		"platform":          "Android",
		"version":           "v2.3.2",
		"packageName":       "cn.lbwdhysj.gf",
		"clid":              "1",
		"deviceId":          "e2c4aa1a4c5da240",
		"app_instance_id":   "v5-http-test-app-instance",
		"model":             "Pixel 8",
		"cpu":               "Snapdragon 8 Gen 3",
		"gpu":               "Adreno 750",
		"ram_gb":            "12",
		"resolution":        "1080x2400",
		"os_version":        "34",
		"timezone":          "Asia/Shanghai",
		"language":          "zh-CN",
		"network_type":      "wifi",
		"carrier":           "46000",
		"android_id":        "v5-http-test-android",
		"build_fingerprint": "test/build/fingerprint",
		"sdk_version":       "34",
		"brand":             "Google",
		"manufacturer":      "Google",
		"device_product":    "shiba",
	}
	canonical := buildRequestSecurityCanonical("POST", accountLoginUsernameV5Path, inner)
	if !strings.HasPrefix(canonical, "POST\n"+accountLoginUsernameV5Path+"\n") {
		t.Fatalf("canonical prefix mismatch: %q", canonical[:min(80, len(canonical))])
	}
	if !strings.Contains(canonical, "packageName=cn.lbwdhysj.gf") {
		t.Fatalf("missing packageName in canonical: %s", canonical)
	}
	sig := signRequestSecurityCanonical(canonical, "e2c4aa1a4c5da240")
	if len(sig) != 64 {
		t.Fatalf("fullSignature len=%d want 64: %q", len(sig), sig)
	}
	for _, c := range sig {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Fatalf("fullSignature not lowercase hex: %q", sig)
		}
	}
}

func TestRequestSecurityEncryptRoundTrip(t *testing.T) {
	deviceID := "e2c4aa1a4c5da240"
	nonce := "v5-123-abc"
	ts := int64(1790000000000)
	inner := map[string]any{
		"appId":     "test",
		"deviceId":  deviceID,
		"name":      "u",
		"value":     "cA==",
		"platform":  "iOS",
		"clid":      "1",
		"packageName": "cn.lbwdhysj.gf.ios",
		"version":   "v1.1.23",
	}
	outer, err := buildEncryptedOuterBody(inner, deviceID, nonce, "POST", accountLoginUsernameV5Path, ts)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := outer["data"].(string)
	if data == "" {
		t.Fatal("empty data")
	}
	plain, err := decryptV5ResponseData(data, deviceID, nonce, ts)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain["name"] != "u" {
		t.Fatalf("name=%v", plain["name"])
	}
	sig, _ := plain["fullSignature"].(string)
	if len(sig) != 64 {
		t.Fatalf("missing fullSignature after decrypt: %#v", plain)
	}
}
