package automation

import "testing"

func TestFeatureCatalogIsDefensive(t *testing.T) {
	catalog := FeatureCatalog()
	if len(catalog) == 0 {
		t.Fatal("feature catalog is empty")
	}

	for _, spec := range catalog {
		if spec.ID == "" {
			t.Fatal("feature catalog contains an empty id")
		}
	}

	originalID := catalog[0].ID
	catalog[0].ID = "mutated"
	var blockedID string
	for i := range catalog {
		if len(catalog[i].BlockedReasons) == 0 {
			continue
		}
		blockedID = catalog[i].ID
		catalog[i].BlockedReasons[0] = "mutated"
		break
	}
	fresh := FeatureCatalog()
	if fresh[0].ID != originalID {
		t.Fatalf("catalog mutation leaked: id=%q, want %q", fresh[0].ID, originalID)
	}
	for _, spec := range fresh {
		if spec.ID == blockedID && (len(spec.BlockedReasons) == 0 || spec.BlockedReasons[0] == "mutated") {
			t.Fatalf("blocked reasons mutation leaked for %q", blockedID)
		}
	}
}

func TestFeatureCatalogMarksRaceUpgradeExecutable(t *testing.T) {
	for _, spec := range FeatureCatalog() {
		if spec.ID != "union.race.upgrade" {
			continue
		}
		if spec.Status != PlanStatusManaged || !spec.Executable || spec.SyncOnly || len(spec.BlockedReasons) != 0 {
			t.Fatalf("race upgrade capability=%+v, want managed executable", spec)
		}
		return
	}
	t.Fatal("race upgrade capability is missing")
}
