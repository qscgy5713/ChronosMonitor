package retention

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

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

func TestCleanOnceDeletesTasksOlderThanRetentionWindow(t *testing.T) {
	taskStore := newTestStore(t)
	now := time.Now().UTC()

	mustCreate(t, taskStore, models.TaskRun{
		RunID: "old", TaskName: "a", Status: models.StatusRunning,
		StartedAt: now.Add(-100 * 24 * time.Hour),
	})
	if err := taskStore.Finish("old", models.StatusSuccess, "", now.Add(-100*24*time.Hour).Add(time.Minute)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	c := New(taskStore, 30, time.Hour) // retain 30 days
	c.cleanOnce()

	if _, err := taskStore.Get("old"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get(old) error = %v, want ErrNotFound after cleanOnce()", err)
	}
}

func TestCleanOnceKeepsRecentAndRunningTasks(t *testing.T) {
	taskStore := newTestStore(t)
	now := time.Now().UTC()

	mustCreate(t, taskStore, models.TaskRun{
		RunID: "recent", TaskName: "a", Status: models.StatusRunning,
		StartedAt: now.Add(-time.Hour),
	})
	if err := taskStore.Finish("recent", models.StatusFailed, "boom", now.Add(-time.Hour).Add(time.Minute)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	mustCreate(t, taskStore, models.TaskRun{
		RunID: "old-running", TaskName: "b", Status: models.StatusRunning,
		StartedAt: now.Add(-200 * 24 * time.Hour),
	})

	c := New(taskStore, 30, time.Hour)
	c.cleanOnce()

	if _, err := taskStore.Get("recent"); err != nil {
		t.Errorf("Get(recent) error = %v, want it to survive (finished recently)", err)
	}
	if _, err := taskStore.Get("old-running"); err != nil {
		t.Errorf("Get(old-running) error = %v, want it to survive (still running)", err)
	}
}

func TestRunCleansImmediatelyAtStartupAndStopsOnContextCancel(t *testing.T) {
	taskStore := newTestStore(t)
	now := time.Now().UTC()

	mustCreate(t, taskStore, models.TaskRun{
		RunID: "old", TaskName: "a", Status: models.StatusRunning,
		StartedAt: now.Add(-100 * 24 * time.Hour),
	})
	if err := taskStore.Finish("old", models.StatusSuccess, "", now.Add(-100*24*time.Hour).Add(time.Minute)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	c := New(taskStore, 30, time.Hour) // long interval: only the immediate startup run should fire in this test
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // let the immediate startup cleanup happen

	if _, err := taskStore.Get("old"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected Run() to clean up immediately at startup; Get(old) error = %v", err)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func mustCreate(t *testing.T, s *store.TaskStore, task models.TaskRun) {
	t.Helper()
	if err := s.Create(task); err != nil {
		t.Fatalf("Create(%+v) error = %v", task, err)
	}
}
