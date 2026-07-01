package captureproxy

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"unicode/utf8"
)

type bodyRecord struct {
	Length    int64  `json:"length"`
	Stored    int64  `json:"stored"`
	Truncated bool   `json:"truncated,omitempty"`
	Text      string `json:"text,omitempty"`
	Base64    string `json:"base64,omitempty"`
	Error     string `json:"error,omitempty"`
}

func readRequestBody(req *http.Request, max int64) *bodyRecord {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return nil
	}
	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(data))
	req.ContentLength = int64(len(data))
	if err != nil {
		return &bodyRecord{Error: err.Error()}
	}
	return makeBodyRecord(data, max)
}

func readResponseBody(res *http.Response, max int64) *bodyRecord {
	if res == nil || res.Body == nil || res.Body == http.NoBody {
		return nil
	}
	data, err := io.ReadAll(res.Body)
	_ = res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(data))
	res.ContentLength = int64(len(data))
	if err != nil {
		return &bodyRecord{Error: err.Error()}
	}
	return makeBodyRecord(data, max)
}

func makeBodyRecord(data []byte, max int64) *bodyRecord {
	rec := &bodyRecord{Length: int64(len(data))}
	if max <= 0 || int64(len(data)) <= max {
		rec.Stored = int64(len(data))
		setBodyPayload(rec, data)
		return rec
	}
	rec.Truncated = true
	rec.Stored = max
	setBodyPayload(rec, data[:max])
	return rec
}

func setBodyPayload(rec *bodyRecord, data []byte) {
	if len(data) == 0 {
		return
	}
	if utf8.Valid(data) {
		rec.Text = string(data)
		return
	}
	rec.Base64 = base64.StdEncoding.EncodeToString(data)
}
