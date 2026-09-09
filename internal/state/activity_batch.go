package state

// Batch discovery and execution readiness are separate facts: entering a
// server-identified active activity loads data needed by strict claim gates.
func activityBatchEnterReady(batch *activityBatchState, tmpType, phase int32) bool {
	return batch != nil && batch.BatchID > 0 && batch.IdentityValid && batch.TmpType == tmpType && batch.TmpID > 0 &&
		batch.Status == 1 && batch.BeginMs > 0 && batch.EndMs > batch.BeginMs &&
		batch.DurationBeforeMs >= 0 && batch.DurationAfterMs >= 0 && (phase == 2 || phase == 3)
}
