package missedrun

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/config"
	"chronosmonitor/internal/db"
	"chronosmonitor/internal/store"
)

func newTestStore(t *testing.T) *store.ScheduleStore {
	t.Helper()
	conn, err := db.Open(config.Config{
		DBDriver: "sqlite",
		DBPath:   filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return store.NewScheduleStore(conn)
}

func TestCheckOncePublishesScheduleMissedEvent(t *testing.T) {
	scheduleStore := newTestStore(t)
	hub := broker.New()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	now := time.Now().UTC()
	if err := scheduleStore.Upsert("invoice-sync", 60, 0, now.Add(-time.Hour)); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	d := New(scheduleStore, hub, time.Hour)
	d.checkOnce()

	select {
	case evt := <-ch:
		if evt.Type != broker.EventScheduleMissed {
			t.Errorf("event type = %q, want %q", evt.Type, broker.EventScheduleMissed)
		}
		if evt.Task.TaskName != "invoice-sync" {
			t.Errorf("event task_name = %q, want invoice-sync", evt.Task.TaskName)
		}
		if evt.Task.RunID != "" {
			t.Errorf("event RunID = %q, want empty (synthetic alert, no real run)", evt.Task.RunID)
		}
		if evt.Task.ErrorMessage == "" {
			t.Error("event ErrorMessage is empty, want a description of the missed schedule")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a schedule.missed event")
	}
}

func TestCheckOnceDoesNotPublishForHealthySchedules(t *testing.T) {
	scheduleStore := newTestStore(t)
	hub := broker.New()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	now := time.Now().UTC()
	if err := scheduleStore.Upsert("invoice-sync", 3600, 0, now); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := scheduleStore.Touch("invoice-sync", now); err != nil {
		t.Fatalf("Touch() error = %v", err)
	}

	d := New(scheduleStore, hub, time.Hour)
	d.checkOnce()

	select {
	case evt := <-ch:
		t.Fatalf("received unexpected event %+v for a schedule that isn't overdue", evt)
	case <-time.After(100 * time.Millisecond):
		// expected: no event
	}
}

func TestRunStopsPromptlyOnContextCancel(t *testing.T) {
	d := New(newTestStore(t), broker.New(), 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond) // let a few ticks fire
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
