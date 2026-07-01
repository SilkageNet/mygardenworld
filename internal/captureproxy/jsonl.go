package captureproxy

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type jsonlWriter struct {
	mu   sync.Mutex
	file *os.File
}

func newJSONLWriter(path string) (*jsonlWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return &jsonlWriter{file: f}, nil
}

func (w *jsonlWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	return w.file.Close()
}

func (w *jsonlWriter) Write(v any) {
	if w == nil || w.file == nil {
		return
	}
	data, err := json.Marshal(v)
	if err != nil {
		data, _ = json.Marshal(map[string]any{
			"ts":    time.Now().Format(time.RFC3339Nano),
			"type":  "logger_error",
			"error": err.Error(),
		})
	}
	w.mu.Lock()
	_, _ = w.file.Write(append(data, '\n'))
	w.mu.Unlock()
}
