package runner

import "testing"

func TestEventMetadataDefaults(t *testing.T) {
	tests := []struct {
		kind     string
		message  string
		category string
		label    string
		level    string
	}{
		{kind: "resource_changed", category: "resource", label: "资源", level: "info"},
		{kind: "land_changed", category: "land", label: "田地", level: "info"},
		{kind: "operation_failed", message: "浇水 失败: server", category: "operation", label: "失败", level: "error"},
		{kind: "land_unlock", message: "跳过开垦 #1025：需要 Lv.42", category: "land", label: "开垦", level: "warn"},
		{kind: "cultivate_new", category: "cultivation", label: "培育", level: "info"},
	}

	for _, tt := range tests {
		if got := eventCategory(tt.kind); got != tt.category {
			t.Fatalf("eventCategory(%q) = %q, want %q", tt.kind, got, tt.category)
		}
		if got := eventLabel(tt.kind); got != tt.label {
			t.Fatalf("eventLabel(%q) = %q, want %q", tt.kind, got, tt.label)
		}
		if got := eventLevel(tt.kind, tt.message); got != tt.level {
			t.Fatalf("eventLevel(%q, %q) = %q, want %q", tt.kind, tt.message, got, tt.level)
		}
	}
}
