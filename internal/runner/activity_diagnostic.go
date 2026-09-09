package runner

import (
	"fmt"
	"strings"
	"time"
)

func (r *Runner) emitActivityDiagnostic(snapshot tickSnapshot, now time.Time) {
	if !snapshot.policy.GetAutomationEnabled() || snapshot.client == nil || snapshot.sessionInvalidated {
		return
	}
	var reasons []string
	var batches []int32
	if snapshot.policy.GetActivity().GetCyclicNote().GetEnabled() {
		v, _ := r.state.CyclicNoteView(now)
		if v.Found && !v.Valid {
			batches = append(batches, v.BatchID)
		}
		if reason := activityWaitReason("花笺集芳", v.Observed, v.Found, v.EnterReady, v.Valid, v.BatchID); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	if snapshot.policy.GetActivity().GetCyclicStory().GetEnabled() {
		v, _ := r.state.CyclicStoryView(now)
		if v.Found && !v.Valid {
			batches = append(batches, v.BatchID)
		}
		if reason := activityWaitReason("莳花纪闻", v.Observed, v.Found, v.EnterReady, v.Valid, v.BatchID); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	reason := strings.Join(reasons, "；")
	r.mu.Lock()
	changed := reason != r.lastActivityDiagnostic
	if changed {
		r.lastActivityDiagnostic = reason
	}
	r.mu.Unlock()
	if reason == "" || !changed {
		return
	}
	r.queueActivityBatchSync(batches)
	r.emit(Event{Kind: "activity_diagnostic", Category: "activity", Domain: "activity.sync", Action: "wait", Label: "活动同步诊断", Message: reason})
}

func activityWaitReason(name string, observed, found, enterReady, valid bool, batch int32) string {
	switch {
	case !observed:
		return name + "：尚未收到活动状态，等待服务器批次信息；不会猜测批次"
	case !found:
		return name + "：已收到活动状态，但没有可识别的开放批次；若游戏内已开放，请提供活动状态记录"
	case !valid && enterReady:
		return fmt.Sprintf("%s：批次 %d 待初始化，等待调度或同步重试；完整状态确认前不提交任务", name, batch)
	case !valid:
		return fmt.Sprintf("%s：批次 %d 的身份、阶段或模板信息不足，暂不能初始化或提交", name, batch)
	default:
		return ""
	}
}
