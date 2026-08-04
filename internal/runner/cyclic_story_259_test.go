package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestIsCyclicStoryOrderNotReadyError(t *testing.T) {
	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCActCyclicStoryRecvOrderRwd,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":259,"args":[]}`)},
	}
	if !isCyclicStoryOrderNotReadyError(clientproto.RPCActCyclicStoryRecvOrderRwd.String(), rpcErr) {
		t.Fatal("expected match for envelope code 259")
	}
	wrapped := errors.New(`actCyclicStory.recvOrderRwd: rpc actCyclicStory.recvOrderRwd: server: {"code":259,"args":[]}`)
	if !isCyclicStoryOrderNotReadyError(clientproto.RPCActCyclicStoryRecvOrderRwd.String(), wrapped) {
		t.Fatal("expected match for wrapped string code 259")
	}
	if isCyclicStoryOrderNotReadyError(clientproto.RPCActCyclicStoryRecv.String(), rpcErr) {
		t.Fatal("must not match milestone recv")
	}
	other := &babigame.RPCServerError{
		Name:     clientproto.RPCActCyclicStoryRecvOrderRwd,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":301,"args":[]}`)},
	}
	if isCyclicStoryOrderNotReadyError(clientproto.RPCActCyclicStoryRecvOrderRwd.String(), other) {
		t.Fatal("must not match other codes")
	}
}

func TestHandleOperationErrorCyclicStory259SoftDefer(t *testing.T) {
	r := newOperationEventTestRunner()
	raw, err := os.ReadFile("../state/testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	r.state.ApplyV(raw)
	now := time.UnixMilli(1783696000000)
	futureValid := now.UnixMilli() + 10*time.Minute.Milliseconds()
	r.state.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"23":{"0":{"9101":{"14":{"106":{"0":{"0":{"0":1,"1":23001,"2":1783695000000,"3":%d}}}}}}}}`,
		futureValid,
	)))

	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCActCyclicStoryRecvOrderRwd,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":259,"args":[]}`)},
	}
	op := &automation.PlannedOp{
		Kind:        clientproto.RPCActCyclicStoryRecvOrderRwd.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryActivity,
		Domain:      "activity.actCyclicStory",
		Action:      "claim_order",
		OperationID: "actCyclicStory.recvOrderRwd:9101:0:1",
		BatchID:     9101,
		SlotID:      0,
		TaskID:      1,
		FlowerID:    23001,
	}
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              rpcErr,
		finishedAt:       now,
	})
	if got != nil {
		t.Fatalf("259 must soft-defer (nil), got %v", got)
	}
	cd, cooling := r.operationCoolingDown(op, now.Add(time.Second))
	if !cooling {
		t.Fatal("259 must place the order claim on cooldown")
	}
	wantUntil := time.UnixMilli(futureValid)
	if cd.Until.Before(wantUntil) {
		t.Fatalf("cooldown until=%v, want >= validTime %v", cd.Until, wantUntil)
	}
}
