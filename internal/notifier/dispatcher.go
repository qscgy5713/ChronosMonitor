package notifier

import (
	"context"
	"log"
	"sync"
	"time"

	"chronosmonitor/internal/broker"
)

// Dispatcher subscribes to task lifecycle events and forwards the
// alert-worthy ones (failed, timeout, schedule-missed) to a Notifier. It
// applies a simple rolling-window rate limit so a burst of simultaneous
// failures (e.g. a shared dependency going down) can't flood the
// notification channel.
type Dispatcher struct {
	hub      *broker.Hub
	notifier Notifier
	limit    int // alerts allowed per window; <= 0 means unlimited
	window   time.Duration
	now      func() time.Time

	mu          sync.Mutex
	windowStart time.Time
	windowCount int
}

func NewDispatcher(hub *broker.Hub, n Notifier, limitPerWindow int, window time.Duration) *Dispatcher {
	return &Dispatcher{
		hub:      hub,
		notifier: n,
		limit:    limitPerWindow,
		window:   window,
		now:      time.Now,
	}
}

// Run blocks, dispatching alerts until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	ch, unsubscribe := d.hub.Subscribe()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			d.handle(evt)
		}
	}
}

func (d *Dispatcher) handle(evt broker.Event) {
	switch evt.Type {
	case broker.EventFailed, broker.EventTimeout, broker.EventScheduleMissed:
	default:
		return
	}

	if !d.allow() {
		log.Printf("alert suppressed (rate limit): task=%s run=%s", evt.Task.TaskName, evt.Task.RunID)
		return
	}

	if err := d.notifier.Notify(evt.Task); err != nil {
		log.Printf("failed to send alert for run=%s: %v", evt.Task.RunID, err)
	}
}

func (d *Dispatcher) allow() bool {
	if d.limit <= 0 {
		return true
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.now()
	if now.Sub(d.windowStart) >= d.window {
		d.windowStart = now
		d.windowCount = 0
	}

	if d.windowCount >= d.limit {
		return false
	}
	d.windowCount++
	return true
}
