// Package retention periodically deletes finished task runs older than a
// configured window, so the database doesn't grow forever. Running tasks are
// never deleted, no matter how old — only ones that have actually completed
// (success/failed/timeout).
package retention

import (
	"context"
	"log"
	"time"

	"chronosmonitor/internal/store"
)

type Cleaner struct {
	store    *store.TaskStore
	retain   time.Duration
	interval time.Duration
}

func New(s *store.TaskStore, retentionDays int, interval time.Duration) *Cleaner {
	return &Cleaner{
		store:    s,
		retain:   time.Duration(retentionDays) * 24 * time.Hour,
		interval: interval,
	}
}

// Run blocks, cleaning up immediately and then on each tick, until ctx is
// cancelled. Cleaning immediately (rather than waiting for the first tick)
// matters here because the sweep interval defaults to an hour — without it,
// enabling retention on an existing large table would leave stale data
// sitting around for up to an hour before the first cleanup.
func (c *Cleaner) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.cleanOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanOnce()
		}
	}
}

func (c *Cleaner) cleanOnce() {
	cutoff := time.Now().UTC().Add(-c.retain)

	n, err := c.store.DeleteOlderThan(cutoff)
	if err != nil {
		log.Printf("retention cleanup failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("retention cleanup: deleted %d finished task run(s) older than %s", n, cutoff.Format(time.RFC3339))
	}
}
