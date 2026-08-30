package runner

import "sync"

// Bus is the in-process pub/sub used to broadcast events from runners to
// any number of workspace subscribers. Subscribers register a
// buffered channel; if a subscriber is too slow, events are dropped on the
// floor for that subscriber (we never block a runner on a stuck client).
type Bus struct {
	mu           sync.RWMutex
	subscribers  map[int]chan Event
	nextID       int
	recentEvents []Event
}

const recentEventLimit = 500

// NewBus returns an empty pub/sub.
func NewBus() *Bus {
	return &Bus{subscribers: make(map[int]chan Event)}
}

// Subscribe returns a buffered channel that first receives in-memory recent
// events, then every newly published event, plus a cancel func that
// unsubscribes and closes the channel.
func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	return b.subscribe(buffer, true)
}

// SubscribeLive returns only newly published events. Workspace sessions use
// this variant because persisted event_log replay is the source of truth
// for history.
func (b *Bus) SubscribeLive(buffer int) (<-chan Event, func()) {
	return b.subscribe(buffer, false)
}

func (b *Bus) subscribe(buffer int, replayRecent bool) (<-chan Event, func()) {
	if buffer <= 0 {
		buffer = 64
	}
	if buffer < recentEventLimit {
		buffer = recentEventLimit
	}
	ch := make(chan Event, buffer)
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	if replayRecent {
		for _, event := range b.recentEvents {
			ch <- event
		}
	}
	b.subscribers[id] = ch
	b.mu.Unlock()
	cancel := func() {
		b.mu.Lock()
		if c, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(c)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Publish fans out e to every subscriber. Slow subscribers drop the event.
func (b *Bus) Publish(e Event) {
	b.publish(e, true)
}

// PublishTransient fans out an internal live signal without retaining it for
// subscribers that request recent user-facing events later.
func (b *Bus) PublishTransient(e Event) {
	b.publish(e, false)
}

func (b *Bus) publish(e Event, remember bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if remember {
		b.recentEvents = append(b.recentEvents, e)
		if len(b.recentEvents) > recentEventLimit {
			copy(b.recentEvents, b.recentEvents[len(b.recentEvents)-recentEventLimit:])
			b.recentEvents = b.recentEvents[:recentEventLimit]
		}
	}
	for _, ch := range b.subscribers {
		select {
		case ch <- e:
		default:
			// drop
		}
	}
}
