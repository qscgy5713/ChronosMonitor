package sweeper

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/config"
	"chronosmonitor/internal/db"
	"chronosmonitor/internal/models"
	"chronosmonitor/internal/store"
)

func newTestStore(t *testing.T) *store.TaskStore {
	t.Helper()
	conn, err := db.Open(config.Config{
		DBDriver: "sqlite",
		DBPath:   filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return store.NewTaskStore(conn)
}

func TestRunStopsPromptlyOnContextCancel(t *testing.T) {
	s := New(newTestStore(t), broker.New(), 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	// Let a few ticks fire before stopping, to exercise sweepOnce too.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func TestSweepOncePublishesTimeoutEvents(t *testing.T) {
	taskStore := newTestStore(t)
	hub := broker.New()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	now := time.Now().UTC()
	ttl := int64(5)
	if err := taskStore.Create(models.TaskRun{
		RunID:      "expired",
		TaskName:   "t",
		Status:     models.StatusRunning,
		StartedAt:  now.Add(-10 * time.Second),
		TTLSeconds: &ttl,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	s := New(taskStore, hub, 10*time.Millisecond)
	s.sweepOnce()

	select {
	case evt := <-ch:
		if evt.Type != broker.EventTimeout || evt.Task.RunID != "expired" {
			t.Errorf("event = %+v, want Type=%q Task.RunID=expired", evt, broker.EventTimeout)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a task.timeout event")
	}
}
