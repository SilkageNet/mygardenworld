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

func TestRPCLifetimeWrapsGuardAndReleasesOnFailure(t *testing.T) {
	c := NewClient(&Session{})
	active, released := false, false
	c.BeginRPC = func(ctx context.Context) (context.Context, func(), error) {
		active = true
		workCtx, cancel := context.WithCancel(ctx)
		cancel()
		return workCtx, func() { active = false; released = true }, nil
	}
	c.BeforeRPC = func(ctx context.Context, _ string) error {
		if !active || !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("guard not using tracked context")
		}
		return ctx.Err()
	}
	_, _, err := c.rpc(context.Background(), "shopCultivate.buy", nil, "", time.Second, true)
	if !errors.Is(err, context.Canceled) || active || !released || c.seq.Load() != 0 {
		t.Fatal("lifetime not released before return", err)
	}
}

func TestClientCannotConnectAfterLifetimeWasClosed(t *testing.T) {
	c := NewClient(&Session{})
	closed := 0
	c.OnClosed = func() { closed++ }
	_ = c.Close()
	_ = c.Close()
	if closed != 1 {
		t.Fatalf("closed callback count=%d", closed)
	}
	if err := c.Connect(context.Background()); err == nil || err.Error() != "client closed" {
		t.Fatal("closed client reached dial", err)
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
	c.dispatchText([]byte(`{"e":"response","d":{"k":"numeric","m":97777}}`))
	if len(names) != 3 || names[0] != "fmlRace.delTask" || names[1] != "" || names[2] != "" {
		t.Fatalf("observed names: %v", names)
	}
}
