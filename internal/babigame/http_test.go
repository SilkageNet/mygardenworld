package babigame

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestDecompressBodyLimitedRejectsExpansionBeyondLimit(t *testing.T) {
	const limit = int64(1024)
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write([]byte(strings.Repeat("x", int(limit)+1))); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	decoded, err := decompressBodyLimited(compressed.Bytes(), "gzip", limit)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("decompress error = %v, want size limit error", err)
	}
	if int64(len(decoded)) != limit {
		t.Fatalf("decoded length = %d, want %d", len(decoded), limit)
	}
}
