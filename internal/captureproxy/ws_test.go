package captureproxy

import (
	"bytes"
	"testing"
)

func TestParseWSFrameMaskedText(t *testing.T) {
	payload := []byte("hello")
	mask := []byte{1, 2, 3, 4}
	frame := []byte{0x81, 0x80 | byte(len(payload))}
	frame = append(frame, mask...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	got, used, ok := parseWSFrame(frame)
	if !ok {
		t.Fatal("parseWSFrame ok=false")
	}
	if used != len(frame) {
		t.Fatalf("used=%d want %d", used, len(frame))
	}
	if !got.Fin || got.Opcode != 1 || !got.Masked {
		t.Fatalf("bad frame metadata: %+v", got)
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Fatalf("payload=%q want %q", got.Payload, payload)
	}
}

func TestParseWSFramePartial(t *testing.T) {
	_, _, ok := parseWSFrame([]byte{0x81, 0x7e, 0x00})
	if ok {
		t.Fatal("partial frame parsed as complete")
	}
}
