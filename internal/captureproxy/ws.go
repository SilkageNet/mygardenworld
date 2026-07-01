package captureproxy

import (
	"encoding/base64"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/SilkageNet/mygardenworld/internal/captureanalysis"
)

type wsReadWriteCloser struct {
	inner io.ReadWriteCloser
	flow  string
	url   string
	log   *jsonlWriter
	dec   *captureanalysis.Decoder
	max   int64

	server *wsFrameParser
	client *wsFrameParser
}

func newWSReadWriteCloser(inner io.ReadWriteCloser, flowID, rawURL string, log *jsonlWriter, dec *captureanalysis.Decoder, maxFrameBytes int64) *wsReadWriteCloser {
	return &wsReadWriteCloser{
		inner:  inner,
		flow:   flowID,
		url:    rawURL,
		log:    log,
		dec:    dec,
		max:    maxFrameBytes,
		server: newWSFrameParser("server_to_client", flowID, rawURL, log, dec, maxFrameBytes),
		client: newWSFrameParser("client_to_server", flowID, rawURL, log, dec, maxFrameBytes),
	}
}

func (w *wsReadWriteCloser) Read(p []byte) (int, error) {
	n, err := w.inner.Read(p)
	if n > 0 {
		w.server.Feed(p[:n])
	}
	return n, err
}

func (w *wsReadWriteCloser) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.client.Feed(p)
	}
	return w.inner.Write(p)
}

func (w *wsReadWriteCloser) Close() error {
	return w.inner.Close()
}

type wsFrameParser struct {
	mu        sync.Mutex
	buf       []byte
	direction string
	flowID    string
	rawURL    string
	log       *jsonlWriter
	dec       *captureanalysis.Decoder
	max       int64
	frameNo   int64
}

func newWSFrameParser(direction, flowID, rawURL string, log *jsonlWriter, dec *captureanalysis.Decoder, max int64) *wsFrameParser {
	return &wsFrameParser{direction: direction, flowID: flowID, rawURL: rawURL, log: log, dec: dec, max: max}
}

func (p *wsFrameParser) Feed(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = append(p.buf, data...)
	for {
		frame, used, ok := parseWSFrame(p.buf)
		if !ok {
			if len(p.buf) > 16*1024*1024 {
				p.log.Write(map[string]any{
					"ts":        time.Now().Format(time.RFC3339Nano),
					"type":      "ws_parse_error",
					"flow_id":   p.flowID,
					"direction": p.direction,
					"url":       p.rawURL,
					"error":     "buffer exceeded 16MiB while waiting for a full frame",
				})
				p.buf = nil
			}
			return
		}
		p.buf = p.buf[used:]
		p.logFrame(frame)
	}
}

type wsFrame struct {
	Fin     bool
	Opcode  byte
	Masked  bool
	Payload []byte
}

func parseWSFrame(buf []byte) (wsFrame, int, bool) {
	if len(buf) < 2 {
		return wsFrame{}, 0, false
	}
	b0 := buf[0]
	b1 := buf[1]
	length := int64(b1 & 0x7f)
	pos := 2
	switch length {
	case 126:
		if len(buf) < pos+2 {
			return wsFrame{}, 0, false
		}
		length = int64(buf[pos])<<8 | int64(buf[pos+1])
		pos += 2
	case 127:
		if len(buf) < pos+8 {
			return wsFrame{}, 0, false
		}
		length = 0
		for i := 0; i < 8; i++ {
			length = (length << 8) | int64(buf[pos+i])
		}
		pos += 8
	}
	if length < 0 {
		return wsFrame{}, 0, false
	}
	masked := b1&0x80 != 0
	var mask [4]byte
	if masked {
		if len(buf) < pos+4 {
			return wsFrame{}, 0, false
		}
		copy(mask[:], buf[pos:pos+4])
		pos += 4
	}
	if int64(len(buf)-pos) < length {
		return wsFrame{}, 0, false
	}
	payload := append([]byte(nil), buf[pos:pos+int(length)]...)
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return wsFrame{
		Fin:     b0&0x80 != 0,
		Opcode:  b0 & 0x0f,
		Masked:  masked,
		Payload: payload,
	}, pos + int(length), true
}

func (p *wsFrameParser) logFrame(frame wsFrame) {
	payload := frame.Payload
	truncated := false
	p.frameNo++
	frameNo := p.frameNo
	ts := time.Now().Format(time.RFC3339Nano)
	if p.max > 0 && int64(len(payload)) > p.max {
		payload = payload[:p.max]
		truncated = true
	}
	rec := map[string]any{
		"ts":          ts,
		"type":        "ws_frame",
		"flow_id":     p.flowID,
		"frame_no":    frameNo,
		"direction":   p.direction,
		"url":         p.rawURL,
		"fin":         frame.Fin,
		"opcode":      frame.Opcode,
		"opcode_text": wsOpcodeText(frame.Opcode),
		"masked":      frame.Masked,
		"length":      len(frame.Payload),
	}
	text := ""
	if truncated {
		rec["truncated"] = true
		rec["stored"] = len(payload)
	}
	if len(payload) > 0 {
		if utf8.Valid(payload) {
			text = string(payload)
			rec["text"] = text
		} else {
			rec["base64"] = base64.StdEncoding.EncodeToString(payload)
		}
	}
	p.log.Write(rec)
	if p.dec != nil {
		p.dec.ProcessFrame(captureanalysis.WSFrame{
			TS:         ts,
			FlowID:     p.flowID,
			FrameNo:    frameNo,
			Direction:  p.direction,
			URL:        p.rawURL,
			OpcodeText: wsOpcodeText(frame.Opcode),
			Length:     len(frame.Payload),
			Text:       text,
			Payload:    frame.Payload,
		})
	}
}

func wsOpcodeText(op byte) string {
	switch op {
	case 0:
		return "continuation"
	case 1:
		return "text"
	case 2:
		return "binary"
	case 8:
		return "close"
	case 9:
		return "ping"
	case 10:
		return "pong"
	default:
		return "unknown"
	}
}
