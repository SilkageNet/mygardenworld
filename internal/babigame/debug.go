package babigame

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// DebugFrameWriter writes all WS frames and HTTP calls to a JSONL file.
type DebugFrameWriter struct {
	mu   sync.Mutex
	file *os.File
}

// NewDebugFrameWriter creates a debug writer that appends to the given path.
// Returns nil if path is empty. Returns an error if the file cannot be opened.
func NewDebugFrameWriter(path string) (*DebugFrameWriter, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open debug file %s: %w", path, err)
	}
	return &DebugFrameWriter{file: f}, nil
}

// Log writes a debug record.
func (w *DebugFrameWriter) Log(eventType, rpc, request, response string) {
	if w == nil || w.file == nil {
		return
	}
	record := map[string]any{
		"ts":   time.Now().UnixMilli(),
		"type": eventType,
	}
	if rpc != "" {
		record["rpc"] = rpc
	}
	if request != "" {
		if len(request) > 4096 {
			record["request"] = request[:4096] + "...(truncated)"
		} else {
			record["request"] = request
		}
	}
	if response != "" {
		if len(response) > 8192 {
			record["response"] = response[:8192] + "...(truncated)"
		} else {
			record["response"] = response
		}
	}
	data, _ := json.Marshal(record)
	w.mu.Lock()
	fmt.Fprintf(w.file, "%s\n", data)
	w.mu.Unlock()
}

// LogHTTP logs an HTTP request/response pair.
func (w *DebugFrameWriter) LogHTTP(method, url string, reqBody, respBody []byte, status int, err error) {
	if w == nil || w.file == nil {
		return
	}
	record := map[string]any{
		"ts":     time.Now().UnixMilli(),
		"type":   "http",
		"method": method,
		"url":    url,
		"status": status,
	}
	if err != nil {
		record["error"] = err.Error()
	}
	if len(reqBody) > 0 {
		if len(reqBody) > 4096 {
			record["request_body"] = string(reqBody[:4096]) + "...(truncated)"
		} else {
			record["request_body"] = string(reqBody)
		}
	}
	if len(respBody) > 0 {
		if len(respBody) > 8192 {
			record["response_body"] = string(respBody[:8192]) + "...(truncated)"
		} else {
			record["response_body"] = string(respBody)
		}
	}
	data, _ := json.Marshal(record)
	w.mu.Lock()
	fmt.Fprintf(w.file, "%s\n", data)
	w.mu.Unlock()
}

// Close closes the underlying file.
func (w *DebugFrameWriter) Close() {
	if w == nil || w.file == nil {
		return
	}
	w.file.Close()
}
