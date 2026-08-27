package state

import "testing"

func TestDailyTaskExecutionFeature(t *testing.T) {
	tests := []struct {
		progressType int32
		feature      TaskExecutionFeature
		supported    bool
	}{
		{3014, TaskExecutionFeaturePlanting, true},
		{3006, TaskExecutionFeatureResident, true},
		{3018, TaskExecutionFeaturePalace, false},
		{3025, TaskExecutionFeatureVideo, false},
		{3052, TaskExecutionFeatureZooStroke, true},
		{999999, TaskExecutionFeatureUnspecified, false},
	}
	for _, test := range tests {
		feature, supported := DailyTaskExecutionFeature(test.progressType)
		if feature != test.feature || supported != test.supported {
			t.Fatalf("DailyTaskExecutionFeature(%d)=(%q,%t), want (%q,%t)", test.progressType, feature, supported, test.feature, test.supported)
		}
	}
}
