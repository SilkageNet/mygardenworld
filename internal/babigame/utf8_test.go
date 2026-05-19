package babigame

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestSafeUTF8_PassesThroughValid(t *testing.T) {
	in := "hello 世界"
	out := SafeUTF8(in)
	if out != in {
		t.Fatalf("valid UTF-8 mutated: %q -> %q", in, out)
	}
}

func TestSafeUTF8_DecodesGBK(t *testing.T) {
	// "正式2482区" encoded as GBK; this is the kind of payload babigame
	// emits for server names that don't go through their JSON serializer.
	gbk, err := simplifiedchinese.GBK.NewEncoder().String("正式2482区")
	if err != nil {
		t.Fatalf("encode setup: %v", err)
	}
	if gbk == "正式2482区" {
		t.Fatalf("expected GBK bytes to differ from UTF-8")
	}
	out := SafeUTF8(gbk)
	if out != "正式2482区" {
		t.Fatalf("GBK -> UTF-8 transcode mismatch: got %q want %q", out, "正式2482区")
	}
}

func TestSafeUTF8_FallbackReplacesInvalidBytes(t *testing.T) {
	in := "\xff\xfe garbage \x80"
	out := SafeUTF8(in)
	if !strings.Contains(out, "garbage") {
		t.Fatalf("ascii portion lost: %q", out)
	}
	// Output must be valid UTF-8 - that's the only invariant SafeUTF8
	// promises. Whether the fallback used GBK transcoding or pure
	// replacement is an implementation detail; what matters is the
	// downstream proto marshaler will accept the result.
	if !utf8.ValidString(out) {
		t.Fatalf("output is not valid UTF-8: % x", []byte(out))
	}
}

func TestSanitizeMap_NestedAndArrays(t *testing.T) {
	gbkName, _ := simplifiedchinese.GBK.NewEncoder().String("正式2482区")
	in := map[string]any{
		"status": "fail",
		"v": map[string]any{
			"name": gbkName,
			"list": []any{gbkName, "ascii", 42},
		},
	}
	out := SanitizeMap(in)
	v := out["v"].(map[string]any)
	if v["name"] != "正式2482区" {
		t.Fatalf("nested GBK not transcoded: %q", v["name"])
	}
	list := v["list"].([]any)
	if list[0] != "正式2482区" {
		t.Fatalf("array element not transcoded: %v", list[0])
	}
	if list[2] != 42 {
		t.Fatalf("non-string element mutated: %v", list[2])
	}
}
