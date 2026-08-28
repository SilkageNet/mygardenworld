package runner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestPearlHireSuccessMessageIncludesExpectedPearls(t *testing.T) {
	st := state.New()
	st.ApplyV(json.RawMessage(`{
		"28":{"5":[{"0":2001,"1":"劳工甲","4":12}]},
		"115":{"0":{"1":{"1":1,"2":2001,"3":9999999999999,"5":0,"6":5,"7":0,"8":0,"9":100}}}
	}`))

	op := &automation.PlannedOp{
		Kind:      clientproto.RPCPearlPlaceHire.String(),
		Domain:    "basic.pearl.hire",
		TargetID:  1,
		TargetUID: 2001,
	}
	got := pearlHireSuccessMessage(op, st)
	if !strings.Contains(got, "雇佣劳工成功") {
		t.Fatalf("message=%q, want success prefix", got)
	}
	if !strings.Contains(got, "槽位=1") || !strings.Contains(got, "劳工=劳工甲(Lv.12)") {
		t.Fatalf("message=%q, want slot and worker label", got)
	}
	if !strings.Contains(got, "产出=5/次") || !strings.Contains(got, "预计获取珍珠=200") {
		t.Fatalf("message=%q, want production and expected pearls", got)
	}
	if got := operationEventLabel(op); got != "雇佣劳工" {
		t.Fatalf("operationEventLabel=%q", got)
	}
}

func TestPearlClaimSuccessMessageIncludesPearlGain(t *testing.T) {
	st := state.New()
	st.ApplyV(json.RawMessage(`{"7":{"2":{"2":{"1006":800}}}}`))
	got := pearlClaimSuccessMessage(240, true, st)
	if !strings.Contains(got, "获取珍珠=+560") || !strings.Contains(got, "当前=800") {
		t.Fatalf("message=%q", got)
	}
}

func TestPearlHireFailureMessageUsesHireLabel(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCPearlPlaceHire.String(),
		Domain:    "basic.pearl.hire",
		TargetID:  2,
		TargetUID: 3001,
	}
	if got := opDesc(op); got != "雇佣劳工" {
		t.Fatalf("opDesc=%q", got)
	}
	if got := operationTargetSuffix(op); got != " (槽位=2 劳工=3001)" {
		t.Fatalf("suffix=%q", got)
	}
}

func TestPearlHireSyncOpKindDescIsChinese(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{clientproto.RPCOpptGetDetailOppts.String(), "同步候选人详情"},
		{clientproto.RPCPearlGetRecommendList.String(), "同步推荐候选人"},
		{clientproto.RPCPearlGetHireStateByUids.String(), "同步候选人雇佣状态"},
		{clientproto.RPCFrdEnter.String(), "同步好友列表"},
		{clientproto.RPCPearlPlaceHire.String(), "雇佣劳工"},
	}
	for _, tc := range tests {
		if got := opKindDesc(tc.kind); got != tc.want {
			t.Errorf("opKindDesc(%s)=%q, want %q", tc.kind, got, tc.want)
		}
	}
	if got := operationEventLabel(&automation.PlannedOp{
		Kind: clientproto.RPCOpptGetDetailOppts.String(), Domain: "basic.pearl.hire",
	}); got != "同步候选人" {
		t.Fatalf("detail label=%q", got)
	}
	if got := operationEventLabel(&automation.PlannedOp{
		Kind: clientproto.RPCFrdEnter.String(), Domain: "basic.pearl.hire",
	}); got != "同步好友候选人" {
		t.Fatalf("friend label=%q", got)
	}
}
