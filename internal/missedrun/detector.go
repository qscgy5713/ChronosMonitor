// Package missedrun periodically checks registered schedules for tasks that
// should have started by now but haven't — the counterpart to TTL timeout
// detection, which only catches a task that started but got stuck.
package missedrun

import (
	"context"
	"fmt"
	"log"
	"time"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/models"
	"chronosmonitor/internal/store"
)

type Detector struct {
	store    *store.ScheduleStore
	hub      *broker.Hub
	interval time.Duration
}

func New(s *store.ScheduleStore, hub *broker.Hub, interval time.Duration) *Detector {
	return &Detector{store: s, hub: hub, interval: interval}
}

// Run blocks, checking on each tick until ctx is cancelled.
func (d *Detector) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.checkOnce()
		}
	}
}

func (d *Detector) checkOnce() {
	missed, err := d.store.DetectMissed(time.Now().UTC())
	if err != nil {
		log.Printf("missed-run detection failed: %v", err)
		return
	}
	if len(missed) == 0 {
		return
	}

	names := make([]string, len(missed))
	for i, sched := range missed {
		names[i] = sched.TaskName
		d.hub.Publish(broker.Event{Type: broker.EventScheduleMissed, Task: toAlertTask(sched)})
	}
	log.Printf("missed-run detection: %d task(s) missed their expected schedule: %v", len(missed), names)
}

// toAlertTask adapts a Schedule into the models.TaskRun shape the existing
// alert notifier already knows how to format. There's no real "run" here —
// that's the whole point, one never started — so this is a synthetic record
// built only to travel through the existing failed/timeout alert pipeline
// without duplicating it. It's never written to task_runs.
func toAlertTask(sched models.Schedule) models.TaskRun {
	lastSeen := "never"
	if sched.LastSeenAt != nil {
		lastSeen = sched.LastSeenAt.Format(time.RFC3339)
	}
	return models.TaskRun{
		TaskName: sched.TaskName,
		Status:   models.StatusMissed,
		ErrorMessage: fmt.Sprintf(
			"expected at least once every %ds (+%ds grace); last seen: %s",
			sched.ExpectedIntervalSeconds, sched.GracePeriodSeconds, lastSeen,
		),
	}
}
