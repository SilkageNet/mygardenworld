package runner

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestWebSocketSessionStartSentinelAndReconnectBackoff(t *testing.T) {
	err := fmt.Errorf("connect account: %w: ws connect: gateway unavailable", errWebSocketSessionStart)
	if !errors.Is(err, errWebSocketSessionStart) {
		t.Fatalf("wrapped error did not preserve websocket startup sentinel: %v", err)
	}

	wait := reconnectInitialWait
	wants := []time.Duration{4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second}
	for _, want := range wants {
		wait = nextReconnectWait(wait)
		if wait != want {
			t.Fatalf("nextReconnectWait=%s, want %s", wait, want)
		}
	}
}
