package autoredeem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CodeEntry is one redeem code from the external API.
type CodeEntry struct {
	Code        string `json:"code"`
	CreatedTime int64  `json:"createdTime"`
}

// CodeFetcher fetches redeem codes from an external source.
type CodeFetcher interface {
	FetchCodes(ctx context.Context) ([]CodeEntry, error)
}

// HTTPCodeFetcher fetches redeem codes from a JSON API endpoint.
type HTTPCodeFetcher struct {
	URL    string
	Client *http.Client
}

func (f *HTTPCodeFetcher) FetchCodes(ctx context.Context) ([]CodeEntry, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch codes: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch codes: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var entries []CodeEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parse codes: %w", err)
	}
	return entries, nil
}
