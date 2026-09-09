package runner

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestActivitySyncRespectsSendTimePolicy(t *testing.T) {
	r := &Runner{policy: automation.DefaultPolicy()}
	r.policy.AutomationEnabled = true
	r.policy.Activity.CyclicNote.Enabled = true
	ctx := context.WithValue(context.Background(), activitySyncContextKey{}, true)
	if err := r.validateActivitySyncBeforeSend(ctx, clientproto.RPCActSyncBatchInfo.String()); err != nil {
		t.Fatal(err)
	}
	r.policy.AutomationEnabled = false
	if err := r.validateActivitySyncBeforeSend(ctx, clientproto.RPCActSyncBatchInfo.String()); err == nil {
		t.Fatal("paused automation still synced")
	}
	if err := r.validateActivitySyncBeforeSend(ctx, clientproto.RPCUsrHeartTick.String()); err != nil {
		t.Fatal("activity switch blocked heartbeat")
	}
}

func TestActivityBatchSyncCoalescesAndBoundsRetries(t *testing.T) {
	r := &Runner{}
	now := time.Now()
	r.queueActivityBatchSync([]int32{0, 9002, 9001, 9001})
	ids := r.activitySyncTargets(now)
	if !slices.Equal(ids, []int32{9001, 9002}) {
		t.Fatalf("ids=%v", ids)
	}
	versions := r.activitySyncRevisions(ids)
	r.queueActivityBatchSync([]int32{9001}) // A notice arriving during the RPC must survive its result.
	r.finishActivityBatchSync(versions, now, true)
	if got := r.activitySyncTargets(now.Add(59 * time.Second)); len(got) != 0 {
		t.Fatal("sync was not paced")
	}
	if got := r.activitySyncTargets(now.Add(time.Minute)); !slices.Equal(got, []int32{9001}) {
		t.Fatalf("new notice lost: %v", got)
	}
	for range 3 {
		now = now.Add(3 * time.Minute)
		ids = r.activitySyncTargets(now)
		if len(ids) != 1 {
			t.Fatalf("retry target missing: %v", ids)
		}
		r.finishActivityBatchSync(r.activitySyncRevisions(ids), now, false)
	}
	if len(r.activityBatchSync) != 0 {
		t.Fatal("unbounded retry")
	}
	for i := int32(1); i <= 200; i++ {
		r.queueActivityBatchSync([]int32{i})
	}
	if len(r.activityBatchSync) != 128 || len(r.activitySyncTargets(now.Add(time.Hour))) != 32 {
		t.Fatal("unbounded queue or request")
	}
}

func TestActivityWaitReason(t *testing.T) {
	for _, tc := range []struct {
		observed, found, enterReady, valid bool
		want                               string
	}{
		{false, false, false, false, "尚未收到"}, {true, false, false, false, "没有可识别"},
		{true, true, true, false, "待初始化"}, {true, true, false, false, "信息不足"}, {true, true, true, true, ""},
	} {
		got := activityWaitReason("活动", tc.observed, tc.found, tc.enterReady, tc.valid, 9001)
		if (tc.want == "" && got != "") || (tc.want != "" && !strings.Contains(got, tc.want)) {
			t.Fatalf("reason=%q", got)
		}
	}
}

func TestCyclicNoteWaitReasonForValidBatchWithoutProgress(t *testing.T) {
	for _, tc := range []struct {
		name                                           string
		needsTasks, list, unlocked, progress, receipts bool
		phase                                          int32
		want                                           string
	}{
		{"missing list", true, false, false, false, false, 2, "任务列表待初始化"},
		{"new account record absent", true, true, true, false, false, 2, "记录尚未下发"},
		{"receipt absent", true, true, true, true, false, 2, "记录尚未下发"},
		{"complete evidence", true, true, true, true, true, 2, ""},
		{"locked slot", true, true, false, false, false, 2, ""},
		{"milestones only", false, false, true, false, false, 2, ""},
		{"reward grace", true, true, true, false, false, 3, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := state.CyclicNoteView{Observed: true, Found: true, Valid: true, BatchID: 9001, Phase: tc.phase, TaskListObserved: tc.list,
				Tasks: []state.CyclicNoteTaskSlotView{{Unlocked: tc.unlocked, ProgressObserved: tc.progress, ReceiptObserved: tc.receipts}}}
			got := cyclicNoteWaitReason(v, tc.needsTasks)
			if (tc.want == "" && got != "") || (tc.want != "" && !strings.Contains(got, tc.want)) {
				t.Fatalf("reason=%q want=%q", got, tc.want)
			}
		})
	}
}
