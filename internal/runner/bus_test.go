package runner

import (
	"fmt"
	"testing"
)

func TestBusSubscribeReceivesRecentEvents(t *testing.T) {
	bus := NewBus()
	for i := 0; i < recentEventLimit+2; i++ {
		bus.Publish(Event{Kind: fmt.Sprintf("event-%02d", i)})
	}

	ch, cancel := bus.Subscribe(1)
	defer cancel()

	for i := 2; i < recentEventLimit+2; i++ {
		event := <-ch
		want := fmt.Sprintf("event-%02d", i)
		if event.Kind != want {
			t.Fatalf("recent event %d = %q, want %q", i-2, event.Kind, want)
		}
	}
}

func TestBusSubscribeContinuesWithLiveEventsAfterRecentEvents(t *testing.T) {
	bus := NewBus()
	bus.Publish(Event{Kind: "recent"})

	ch, cancel := bus.Subscribe(1)
	defer cancel()

	if event := <-ch; event.Kind != "recent" {
		t.Fatalf("first event = %q, want recent", event.Kind)
	}

	bus.Publish(Event{Kind: "live"})
	if event := <-ch; event.Kind != "live" {
		t.Fatalf("second event = %q, want live", event.Kind)
	}
}

func TestBusSubscribeLiveSkipsRecentEvents(t *testing.T) {
	bus := NewBus()
	bus.Publish(Event{Kind: "recent"})

	ch, cancel := bus.SubscribeLive(1)
	defer cancel()

	select {
	case event := <-ch:
		t.Fatalf("unexpected replayed event: %q", event.Kind)
	default:
	}

	bus.Publish(Event{Kind: "live"})
	if event := <-ch; event.Kind != "live" {
		t.Fatalf("live event = %q, want live", event.Kind)
	}
}

func TestBusPublishTransientIsLiveOnly(t *testing.T) {
	bus := NewBus()
	live, cancelLive := bus.SubscribeLive(1)
	defer cancelLive()
	bus.PublishTransient(Event{Kind: "refresh"})
	if event := <-live; event.Kind != "refresh" {
		t.Fatalf("live event kind=%q, want refresh", event.Kind)
	}

	replayed, cancelReplayed := bus.Subscribe(1)
	defer cancelReplayed()
	bus.Publish(Event{Kind: "persisted"})
	if event := <-replayed; event.Kind != "persisted" {
		t.Fatalf("replayed event kind=%q, want persisted", event.Kind)
	}
}
