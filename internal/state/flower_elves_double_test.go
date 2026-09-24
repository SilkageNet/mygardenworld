package state

import (
	"testing"
)

func TestFlowerElvesDoubleCfgMergesSparseTemplate(t *testing.T) {
	template := &activityTemplateState{}
	mergeActivityTemplateCommonCfgLocked(template, []byte(`{"104":{"0":2,"2":[110001,110002]}}`))
	if !template.CommonCfgObserved || template.CommonCfgIV != 2 || len(template.CommonCfgIL) != 2 {
		t.Fatalf("common cfg=%+v", template)
	}
	mergeActivityTemplateCommonCfgLocked(template, []byte(`{"104":{"0":3}}`))
	if template.CommonCfgIV != 3 || len(template.CommonCfgIL) != 2 {
		t.Fatalf("sparse update lost IDs: %+v", template)
	}
	mergeActivityTemplateCommonCfgLocked(template, []byte(`null`))
	if template.CommonCfgObserved || len(template.CommonCfgIL) != 0 {
		t.Fatalf("null ext did not clear cfg: %+v", template)
	}
}
