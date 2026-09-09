package runner

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

type activitySyncContextKey struct{}

func (r *Runner) validateActivitySyncBeforeSend(ctx context.Context, name string) error {
	if scheduled, _ := ctx.Value(activitySyncContextKey{}).(bool); !scheduled || name != clientproto.RPCActSyncBatchInfo.String() {
		return nil
	}
	policy := r.Policy()
	if !policy.GetAutomationEnabled() || (!policy.GetActivity().GetCyclicNote().GetEnabled() && !policy.GetActivity().GetCyclicStory().GetEnabled()) {
		return errors.New("活动自动化已关闭，取消尚未发送的批次同步")
	}
	return nil
}

type activitySyncEntry struct {
	attempts int
	next     time.Time
	revision uint64
}

// Notification callbacks only enqueue evidence. Actual requests stay in the
// existing decision loop, never the WebSocket reader or a second connection.
func (r *Runner) observeActivityRefresh(items []json.RawMessage) {
	r.queueActivityBatchSync(babigame.ActivityRefreshBatchIDs(items))
}

func (r *Runner) queueActivityBatchSync(ids []int32) {
	if len(ids) == 0 {
		return
	}
	r.mu.Lock()
	if r.activityBatchSync == nil {
		r.activityBatchSync = make(map[int32]activitySyncEntry)
	}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		entry, exists := r.activityBatchSync[id]
		if !exists && len(r.activityBatchSync) >= 128 {
			continue
		}
		entry.revision++
		r.activityBatchSync[id] = entry
	}
	r.mu.Unlock()
	r.wakeDecision()
}

func (r *Runner) activitySyncTargets(now time.Time) []int32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if now.Before(r.nextActivityBatchSync) {
		return nil
	}
	var ids []int32
	for id, entry := range r.activityBatchSync {
		if !now.Before(entry.next) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	if len(ids) > 32 {
		ids = ids[:32]
	}
	return ids
}

func (r *Runner) activitySyncRevisions(ids []int32) map[int32]uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions := make(map[int32]uint64, len(ids))
	for _, id := range ids {
		versions[id] = r.activityBatchSync[id].revision
	}
	return versions
}

func (r *Runner) finishActivityBatchSync(versions map[int32]uint64, now time.Time, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextActivityBatchSync = now.Add(time.Minute)
	for id, revision := range versions {
		entry := r.activityBatchSync[id]
		entry.attempts = min(3, entry.attempts+1)
		if entry.revision == revision && (success || entry.attempts >= 3) {
			delete(r.activityBatchSync, id)
			continue
		}
		if success {
			entry.attempts = 0
		}
		entry.next = now.Add(time.Duration(entry.attempts) * time.Minute)
		r.activityBatchSync[id] = entry
	}
}

func (r *Runner) tickActivityBatchSync(ctx context.Context, snapshot tickSnapshot, now time.Time) bool {
	policy := snapshot.policy
	if policy == nil || !policy.GetAutomationEnabled() ||
		(!policy.GetActivity().GetCyclicNote().GetEnabled() && !policy.GetActivity().GetCyclicStory().GetEnabled()) {
		return false
	}
	ids := r.activitySyncTargets(now)
	if len(ids) == 0 || r.pacer.delay(clientproto.RPCActSyncBatchInfo.String(), now) > 0 {
		return false
	}
	r.operationMu.Lock()
	defer r.operationMu.Unlock()
	if ctx.Err() != nil || r.isSessionInvalidated() || r.restrictionError() != nil {
		return false
	}
	live := r.readTickSnapshot()
	if live.client != snapshot.client || live.session != snapshot.session || !live.policy.GetAutomationEnabled() ||
		(!live.policy.GetActivity().GetCyclicNote().GetEnabled() && !live.policy.GetActivity().GetCyclicStory().GetEnabled()) {
		return false
	}
	versions := r.activitySyncRevisions(ids)
	ctx = context.WithValue(ctx, activitySyncContextKey{}, true)
	_, err := checkedStateDelta(r.runnerRPC(snapshot.client, snapshot.session).Act().SyncBatchInfo(ctx,
		clientproto.ActSyncBatchInfoRequest{"batchIdList": ids}))
	r.finishActivityBatchSync(versions, time.Now(), err == nil)
	if err != nil {
		r.emit(Event{Kind: "activity_batch_sync", Category: "activity", Domain: "activity.sync", Action: "failed", Label: "活动批次同步", Level: "warn",
			Message: "活动批次同步失败，将限频重试；连续失败三次后等待新的批次通知", PayloadJSON: activityBatchPayload(ids)})
	} else {
		r.emit(Event{Kind: "activity_batch_sync", Category: "activity", Domain: "activity.sync", Action: "sync", Label: "活动批次同步",
			Message: "已刷新服务器提供的活动批次信息", PayloadJSON: activityBatchPayload(ids)})
	}
	return true
}

func activityBatchPayload(ids []int32) string {
	raw, _ := json.Marshal(map[string]any{"batch_ids": ids})
	return string(raw)
}
