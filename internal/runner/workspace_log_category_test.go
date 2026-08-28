package runner

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestWorkspaceLogCategoryUsesStableProductBoundaries(t *testing.T) {
	tests := []struct {
		category string
		domain   string
		want     pb.WorkspaceLogCategory
	}{
		{"account", "account.session", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_ACCOUNT},
		{"basic", "basic.task", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_BASIC},
		{"plant", "farm.land", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_GARDEN},
		{"order", "order.customer", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_ORDERS},
		{"race", "union.race", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_UNION},
		{"activity", "activity.cyclic_note", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_ACTIVITIES},
		{"basic", "basic.inventory", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_WAREHOUSE},
		{"basic", "basic.pearl.hire", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_HIRE},
		{"basic", "basic.pearl.place", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_HIRE},
		{"basic", "basic.pearl.free", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_WAREHOUSE},
		{"system", "policy", pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_SYSTEM},
	}
	for _, test := range tests {
		if got := WorkspaceLogCategory(test.category, test.domain); got != test.want {
			t.Errorf("WorkspaceLogCategory(%q, %q)=%s, want %s", test.category, test.domain, got, test.want)
		}
	}
}
