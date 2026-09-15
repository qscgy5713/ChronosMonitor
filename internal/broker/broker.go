package broker

import (
	"sync"

	"chronosmonitor/internal/models"
)

const (
	EventStarted   = "task.started"
	EventHeartbeat = "task.heartbeat"
	EventSucceeded = "task.succeeded"
	EventFailed    = "task.failed"
	EventTimeout   = "task.timeout"
)

type Event struct {
	Type string         `json:"type"`
	Task models.TaskRun `json:"task"`
}

// Hub is an in-memory pub/sub broker that fans out task lifecycle events to
// any number of subscribers (e.g. SSE connections).
type Hub struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

func New() *Hub {
	return &Hub{subs: make(map[chan Event]struct{})}
}

// Subscribe registers a new listener and returns its channel plus an
// unsubscribe function that must be called when the listener is done.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)

	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}

	return ch, unsubscribe
}

// Publish broadcasts an event to all current subscribers. Slow subscribers
// that can't keep up have the event dropped rather than blocking the caller.
func (h *Hub) Publish(evt Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}
