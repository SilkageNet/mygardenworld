package state

// TaskExecutionFeature identifies the business module that can advance an
// incomplete game task. It is deliberately separate from reward claiming:
// every observed completed task can still be claimed by its task switch.
type TaskExecutionFeature string

const (
	TaskExecutionFeatureUnspecified   TaskExecutionFeature = ""
	TaskExecutionFeatureClaimOnly     TaskExecutionFeature = "claim_only"
	TaskExecutionFeatureStory         TaskExecutionFeature = "story"
	TaskExecutionFeaturePlanting      TaskExecutionFeature = "planting"
	TaskExecutionFeatureResident      TaskExecutionFeature = "resident_order"
	TaskExecutionFeatureFlowerRack    TaskExecutionFeature = "flower_rack"
	TaskExecutionFeatureCustomer      TaskExecutionFeature = "customer_order"
	TaskExecutionFeatureCultivateShop TaskExecutionFeature = "cultivate_shop"
	TaskExecutionFeaturePalace        TaskExecutionFeature = "palace_order"
	TaskExecutionFeaturePearlHire     TaskExecutionFeature = "pearl_hire"
	TaskExecutionFeatureFriendTouch   TaskExecutionFeature = "friend_touch"
	TaskExecutionFeatureVideo         TaskExecutionFeature = "video"
	TaskExecutionFeatureZooStroke     TaskExecutionFeature = "zoo_stroke"
	TaskExecutionFeatureCultivation   TaskExecutionFeature = "cultivation"
)

// DailyTaskExecutionFeature maps the catalog progress counter to the module
// that can make progress. The mapping only contains behavior observed in the
// client protocol; unknown task types remain pending and are never guessed.
func DailyTaskExecutionFeature(progressType int32) (TaskExecutionFeature, bool) {
	switch progressType {
	case 4:
		return TaskExecutionFeatureStory, true
	case 3006:
		return TaskExecutionFeatureResident, true
	case 3014:
		return TaskExecutionFeaturePlanting, true
	case 3015:
		return TaskExecutionFeatureFlowerRack, true
	case 3016:
		return TaskExecutionFeatureCustomer, true
	case 3017:
		return TaskExecutionFeatureCultivateShop, true
	case 3018:
		// The palace submit RPC is still sync-only, so the task remains visible
		// without authorizing speculative submission.
		return TaskExecutionFeaturePalace, false
	case 3023:
		return TaskExecutionFeaturePearlHire, true
	case 3024:
		return TaskExecutionFeatureFriendTouch, true
	case 3025:
		// Video completion needs a real advertising SDK callback.
		return TaskExecutionFeatureVideo, false
	case 3052:
		return TaskExecutionFeatureZooStroke, true
	default:
		return TaskExecutionFeatureUnspecified, false
	}
}
