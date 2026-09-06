package babigame

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRPCGuardRejectsBeforeBuildingOrSending(t *testing.T) {
	blocked := errors.New("account paused")
	c := NewClient(&Session{})
	c.BeforeRPC = func(_ context.Context, name string) error {
		if name != "farm.harvest" {
			t.Fatalf("name=%q", name)
		}
		return blocked
	}
	_, _, err := c.rpc(context.Background(), "farm.harvest", nil, "", time.Second, true)
	if !errors.Is(err, blocked) || c.seq.Load() != 0 || len(c.pending) != 0 {
		t.Fatalf("guard did not reject before transport: %v", err)
	}
}

func TestResponseObserverPrecedesCallerAndIncludesUnmatchedErrors(t *testing.T) {
	c := NewClient(&Session{})
	result := make(chan rpcResult, 1)
	c.pending["key"] = pendingRPC{name: "fmlRace.delTask", result: result}
	var names []string
	c.OnRPCResponse = func(name string, d WSResponseD) {
		if len(result) != 0 {
			t.Fatal("caller resumed before guard observed the error")
		}
		if d.ErrorCode() != 97777 {
			t.Fatalf("code=%d", d.ErrorCode())
		}
		names = append(names, name)
	}
	c.dispatchText([]byte(`{"e":"response","d":{"k":"key","m":{"code":97777}}}`))
	if (<-result).d.ErrorCode() != 97777 {
		t.Fatal("error was lost")
	}
	c.dispatchText([]byte(`{"e":"response","d":{"k":"late","m":{"code":97777}}}`))
	if len(names) != 2 || names[0] != "fmlRace.delTask" || names[1] != "" {
		t.Fatalf("observed names: %v", names)
	}
}
