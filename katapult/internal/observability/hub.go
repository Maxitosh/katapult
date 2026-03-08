package observability

import (
	"sync"

	"github.com/google/uuid"
)

// @cpt-algo:cpt-katapult-algo-observability-emit-progress:p1
// @cpt-dod:cpt-katapult-dod-observability-realtime-progress:p1

const subscriberBufferSize = 16

// ProgressPublisher is the interface used by the orchestrator to push progress events.
type ProgressPublisher interface {
	Publish(transferID uuid.UUID, progress EnrichedProgress)
	CloseTransfer(transferID uuid.UUID)
}

// ProgressSubscriber is the interface used by the HTTP server to subscribe to progress events.
type ProgressSubscriber interface {
	Subscribe(transferID uuid.UUID) (<-chan EnrichedProgress, func())
}

type subscriber struct {
	ch     chan EnrichedProgress
	closed bool
}

// ProgressHub is an in-memory pub/sub hub for transfer progress events.
type ProgressHub struct {
	mu          sync.Mutex
	subscribers map[uuid.UUID]map[*subscriber]struct{}
}

// NewProgressHub creates a new progress event hub.
func NewProgressHub() *ProgressHub {
	return &ProgressHub{
		subscribers: make(map[uuid.UUID]map[*subscriber]struct{}),
	}
}

// Subscribe registers a new subscriber for the given transfer ID.
// Returns a read-only channel and an unsubscribe cleanup function.
func (h *ProgressHub) Subscribe(transferID uuid.UUID) (<-chan EnrichedProgress, func()) {
	sub := &subscriber{ch: make(chan EnrichedProgress, subscriberBufferSize)}

	h.mu.Lock()
	subs, ok := h.subscribers[transferID]
	if !ok {
		subs = make(map[*subscriber]struct{})
		h.subscribers[transferID] = subs
	}
	subs[sub] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if sub.closed {
			return
		}
		sub.closed = true
		if subs, ok := h.subscribers[transferID]; ok {
			delete(subs, sub)
			if len(subs) == 0 {
				delete(h.subscribers, transferID)
			}
		}
		close(sub.ch)
	}

	return sub.ch, unsubscribe
}

// Publish sends a progress event to all subscribers for the given transfer ID.
// Non-blocking: drops events for slow consumers.
func (h *ProgressHub) Publish(transferID uuid.UUID, progress EnrichedProgress) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subs, ok := h.subscribers[transferID]
	if !ok {
		return
	}
	for sub := range subs {
		select {
		case sub.ch <- progress:
		default:
			// drop if subscriber is slow
		}
	}
}

// CloseTransfer closes all subscriber channels and removes the transfer entry.
func (h *ProgressHub) CloseTransfer(transferID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	subs, ok := h.subscribers[transferID]
	if !ok {
		return
	}
	delete(h.subscribers, transferID)

	for sub := range subs {
		if !sub.closed {
			sub.closed = true
			close(sub.ch)
		}
	}
}
