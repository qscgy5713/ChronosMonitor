package sweeper

import (
	"context"
	"log"
	"time"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/store"
)

// Sweeper periodically scans for tasks that have exceeded their TTL without
// a heartbeat or completion, and marks them as timed out.
type Sweeper struct {
	store    *store.TaskStore
	hub      *broker.Hub
	interval time.Duration
}

func New(store *store.TaskStore, hub *broker.Hub, interval time.Duration) *Sweeper {
	return &Sweeper{store: store, hub: hub, interval: interval}
}

// Run blocks, sweeping on each tick until ctx is cancelled.
func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepOnce()
		}
	}
}

func (s *Sweeper) sweepOnce() {
	tasks, err := s.store.SweepTimeouts(time.Now().UTC())
	if err != nil {
		log.Printf("ttl sweep failed: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	runIDs := make([]string, len(tasks))
	for i, t := range tasks {
		runIDs[i] = t.RunID
		s.hub.Publish(broker.Event{Type: broker.EventTimeout, Task: t})
	}
	log.Printf("ttl sweep: marked %d task(s) as timeout: %v", len(runIDs), runIDs)
}
