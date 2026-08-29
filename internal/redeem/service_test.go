package redeem

import (
	"strings"
	"testing"
)

func TestValidateSourceEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		native     bool
		wantErrSub string
	}{
		{name: "native local HTTP", url: "http://127.0.0.1:8080", native: true},
		{name: "custom requires HTTPS", url: "http://example.com/codes", wantErrSub: "HTTPS"},
		{name: "custom rejects literal private address", url: "https://127.0.0.1/codes", wantErrSub: "private or local"},
		{name: "credentials rejected", url: "https://user:secret@example.com", native: true, wantErrSub: "credentials"},
		{name: "fragment rejected", url: "https://example.com/codes#private", native: true, wantErrSub: "fragments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSourceEndpoint(tt.url, tt.native)
			if tt.wantErrSub == "" && err != nil {
				t.Fatalf("ValidateSourceEndpoint() error = %v", err)
			}
			if tt.wantErrSub != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrSub)) {
				t.Fatalf("ValidateSourceEndpoint() error = %v, want substring %q", err, tt.wantErrSub)
			}
		})
	}
}

func TestValidateCustomParserConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		wantErrSub string
	}{
		{name: "valid TTL", config: `{"type":"json_array","code_field":"code","time_field":"created","time_format":"unix","default_ttl_seconds":300}`},
		{name: "valid permanent", config: `{"type":"json_array","code_field":"gift","permanent":true}`},
		{name: "missing code", config: `{}`, wantErrSub: "code_field"},
		{name: "missing expiration rule", config: `{"code_field":"code"}`, wantErrSub: "expiration rule"},
		{name: "ambiguous expiration rule", config: `{"code_field":"code","permanent":true,"default_ttl_seconds":300}`, wantErrSub: "expiration rule"},
		{name: "negative TTL", config: `{"code_field":"code","default_ttl_seconds":-1}`, wantErrSub: "non-negative"},
		{name: "unsupported format", config: `{"code_field":"code","time_format":"date","default_ttl_seconds":300}`, wantErrSub: "time_format"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCustomParserConfig(tt.config)
			if tt.wantErrSub == "" && err != nil {
				t.Fatalf("ValidateCustomParserConfig() error = %v", err)
			}
			if tt.wantErrSub != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrSub)) {
				t.Fatalf("ValidateCustomParserConfig() error = %v, want substring %q", err, tt.wantErrSub)
			}
		})
	}
}
