package automation

import "testing"

func TestOrderScheduleStagesExpressProductOrdering(t *testing.T) {
	stages := []orderScheduleStage{
		orderStageCustomerFinish,
		orderStageFlowerRackClaim,
		orderStageFlowerRackSell,
		orderStageFlowerRackCraft,
		orderStageCustomerGenerate,
		orderStageCustomerReject,
		orderStageCustomerCraft,
	}
	for i := 0; i < len(stages)-1; i++ {
		higher := PlannedOp{Lane: LaneSide, Category: CategoryOrder, Priority: orderSchedulePriority(stages[i])}
		lower := PlannedOp{Lane: LaneSide, Category: CategoryOrder, Priority: orderSchedulePriority(stages[i+1])}
		if !operationComesBefore(higher, lower) {
			t.Fatalf("stage %d priority=%d did not precede stage %d priority=%d", stages[i], higher.Priority, stages[i+1], lower.Priority)
		}
	}
}
