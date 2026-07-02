package runner

import "testing"

func TestEventMetadataDefaults(t *testing.T) {
	tests := []struct {
		kind     string
		message  string
		category string
		domain   string
		action   string
		label    string
		level    string
	}{
		{kind: "resource_changed", category: "basic", domain: "basic.resource", action: "changed", label: "资源", level: "info"},
		{kind: "land_changed", category: "plant", domain: "farm.land", action: "changed", label: "田地", level: "info"},
		{kind: "operation_failed", message: "浇水 失败: server", category: "plant", domain: "farm.operation", action: "failed", label: "失败", level: "error"},
		{kind: "land_unlock", message: "跳过开垦 #1025：需要 Lv.42", category: "plant", domain: "farm.land", action: "unlock", label: "开垦", level: "warn"},
		{kind: "cultivate_new", category: "plant", domain: "farm.cultivate", action: "cultivate", label: "培育", level: "info"},
		{kind: "benefit_box", category: "basic", domain: "basic.benefit", action: "claim", label: "福利箱", level: "info"},
		{kind: "mail_claim", category: "basic", domain: "basic.mail", action: "claim", label: "邮件", level: "info"},
		{kind: "sign_claim", category: "basic", domain: "basic.sign", action: "claim", label: "签到", level: "info"},
		{kind: "task_weekly", category: "basic", domain: "basic.task.weekly", action: "claim", label: "每周任务", level: "info"},
		{kind: "order_ad", category: "order", domain: "order.resident", action: "order", label: "居民订单", level: "info"},
		{kind: "union_build", category: "union", domain: "union.build", action: "build", label: "公会建设", level: "info"},
	}

	for _, tt := range tests {
		if got := eventCategory(tt.kind); got != tt.category {
			t.Fatalf("eventCategory(%q) = %q, want %q", tt.kind, got, tt.category)
		}
		if got := eventDomain(tt.kind); got != tt.domain {
			t.Fatalf("eventDomain(%q) = %q, want %q", tt.kind, got, tt.domain)
		}
		if got := eventAction(tt.kind); got != tt.action {
			t.Fatalf("eventAction(%q) = %q, want %q", tt.kind, got, tt.action)
		}
		if got := eventLabel(tt.kind); got != tt.label {
			t.Fatalf("eventLabel(%q) = %q, want %q", tt.kind, got, tt.label)
		}
		if got := eventLevel(tt.kind, tt.message); got != tt.level {
			t.Fatalf("eventLevel(%q, %q) = %q, want %q", tt.kind, tt.message, got, tt.level)
		}
	}
}

func TestNormalizeEventCategoryRejectsLegacyBuckets(t *testing.T) {
	cases := map[string]string{
		"session":     "account",
		"operation":   "plant",
		"land":        "plant",
		"cultivation": "plant",
		"resource":    "basic",
		"reward":      "basic",
		"task":        "basic",
	}
	for legacy, want := range cases {
		if got := normalizeEventCategory(legacy, "resource_changed"); got != want {
			t.Fatalf("normalizeEventCategory(%q)=%q want %q", legacy, got, want)
		}
	}
}
