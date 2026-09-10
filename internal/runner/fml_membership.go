package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type fmlExecutionContextKey struct{}

func (r *Runner) applyStartupLazySync(v json.RawMessage, err error) {
	before := r.state.FmlBuild()
	if err == nil {
		r.state.ApplyV(v)
		r.state.FinalizeFmlMembershipSnapshot()
	} else {
		r.log.Warn("ws lazy sync failed", "err", err)
	}
	r.emitFmlMembershipDiagnostic("usr.lazySync", before, err)
}

func (r *Runner) syncFmlMembership(fetch func() (json.RawMessage, error)) (json.RawMessage, error) {
	before := r.state.FmlBuild()
	v, err := fetch()
	now := time.Now()
	// Even an empty success counts as an attempt. A response must contain
	// fresh identity evidence to reopen execution, not simply acknowledge RPC.
	r.state.MarkFmlMembershipSyncAttemptAt(now)
	if err != nil {
		r.state.MarkFmlMemberPositionSyncAttemptAt(now)
		if !isFmlNotJoinedError(clientproto.RPCFmlEnter.String(), err) {
			r.emitFmlMembershipDiagnostic("fml.enter", before, err)
		}
		return nil, err
	}
	if babigame.HasPayload(v) {
		v = normalizeFmlEnterV(v)
		r.state.ApplyVFmlMembership(v)
	}
	r.state.MarkFmlMemberPositionSyncAttemptAt(now)
	r.emitFmlMembershipDiagnostic("fml.enter", before, nil)
	return v, nil
}

func isGuildRPC(name string) bool {
	group, _, _ := strings.Cut(name, ".")
	switch group {
	case "fml", "fmlRace", "fmlLand", "fmlFlowerShare", "fmlForest":
		return true
	default:
		return false
	}
}

// The serialized planner may have selected work before a push/spacing wait
// invalidates membership. Recheck at send time, including internal batch RPCs.
func (r *Runner) validateFmlMembershipBeforeSend(ctx context.Context, name string) error {
	if !isGuildRPC(name) {
		return nil
	}
	if name == clientproto.RPCFmlEnter.String() {
		if scheduled, _ := ctx.Value(scheduledOperationKey{}).(bool); scheduled &&
			(!r.Policy().GetAutomationEnabled() || !automation.UnionAutomationEnabled(r.Policy().GetUnion())) {
			return fmt.Errorf("公会自动化已关闭，取消身份同步")
		}
		return nil
	}
	build := r.state.FmlBuild()
	if !build.MembershipObserved || build.MemberFmlID <= 0 {
		return fmt.Errorf("公会身份尚未确认，取消尚未发送的公会请求")
	}
	if id, ok := ctx.Value(fmlExecutionContextKey{}).(int32); ok && id != build.MemberFmlID {
		return fmt.Errorf("公会身份已经变化，取消旧公会计划")
	}
	return nil
}

func fmlMembershipStatus(build state.FmlBuildView) string {
	if !build.MembershipObserved {
		return "待确认"
	}
	if build.MemberFmlID <= 0 {
		return "未加入"
	}
	return "已加入"
}

func (r *Runner) emitFmlMembershipDiagnostic(source string, before state.FmlBuildView, err error) {
	after := r.state.FmlBuild()
	next := automation.FmlMembershipNextSyncAt(after)
	message := fmt.Sprintf("公会身份确认: %s → %s（公会 %d → %d，来源 %s）",
		fmlMembershipStatus(before), fmlMembershipStatus(after), before.MemberFmlID, after.MemberFmlID, source)
	if !next.IsZero() {
		message += "；下次确认不早于 " + next.Format("01/02 15:04:05")
	}
	payload := map[string]any{"source_rpc": source, "before": before, "after": after, "next_check_at": next}
	level := "info"
	if err != nil {
		level = "warn"
		payload["error"] = err.Error()
		var rpcErr *babigame.RPCServerError
		if errors.As(err, &rpcErr) && rpcErr != nil {
			payload["error_code"] = rpcErr.Envelope.ErrorCode()
			payload["error_rpc"] = rpcErr.Name.String()
			message += fmt.Sprintf("；错误 RPC %s，代码 %d（原始错误见明细）", rpcErr.Name, rpcErr.Envelope.ErrorCode())
		}
	}
	raw, _ := json.Marshal(payload)
	r.emit(Event{Kind: "fml_membership_sync", Category: automation.CategoryUnion, Domain: "union.membership",
		Action: "changed", Label: "公会身份确认", Message: message, PayloadJSON: string(raw), Level: level})
}
