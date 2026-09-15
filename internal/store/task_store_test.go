package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"chronosmonitor/internal/config"
	"chronosmonitor/internal/db"
	"chronosmonitor/internal/models"
)

func newTestConn(t *testing.T) *db.Conn {
	t.Helper()

	conn, err := db.Open(config.Config{
		DBDriver: "sqlite",
		DBPath:   filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return conn
}

func newTestStore(t *testing.T) *TaskStore {
	t.Helper()
	return NewTaskStore(newTestConn(t))
}

func int64ptr(v int64) *int64 { return &v }

func TestCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	startedAt := time.Now().UTC().Truncate(time.Microsecond)

	err := s.Create(models.TaskRun{
		RunID:     "run-1",
		TaskName:  "daily-report",
		Source:    "laravel-cron",
		Status:    models.StatusRunning,
		StartedAt: startedAt,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.Get("run-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.TaskName != "daily-report" || got.Source != "laravel-cron" {
		t.Errorf("Get() = %+v, want task_name/source preserved", got)
	}
	if got.Status != models.StatusRunning {
		t.Errorf("Get().Status = %q, want %q", got.Status, models.StatusRunning)
	}
	if got.LastHeartbeatAt == nil || !got.LastHeartbeatAt.Equal(startedAt) {
		t.Errorf("Get().LastHeartbeatAt = %v, want it initialized to StartedAt (%v)", got.LastHeartbeatAt, startedAt)
	}
	if got.TTLSeconds != nil {
		t.Errorf("Get().TTLSeconds = %v, want nil when not set on start", got.TTLSeconds)
	}
}

func TestGetUnknownRunReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Get("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestHeartbeatUpdatesLastHeartbeatAt(t *testing.T) {
	s := newTestStore(t)
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	mustCreate(t, s, models.TaskRun{RunID: "run-1", TaskName: "t", Status: models.StatusRunning, StartedAt: startedAt})

	beat := startedAt.Add(5 * time.Second)
	if err := s.Heartbeat("run-1", beat); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}

	got, err := s.Get("run-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.LastHeartbeatAt == nil || !got.LastHeartbeatAt.Equal(beat) {
		t.Errorf("LastHeartbeatAt = %v, want %v", got.LastHeartbeatAt, beat)
	}
	if got.Status != models.StatusRunning {
		t.Errorf("Status = %q, want still running", got.Status)
	}
}

func TestHeartbeatUnknownRunReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)

	if err := s.Heartbeat("does-not-exist", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Heartbeat() error = %v, want ErrNotFound", err)
	}
}

func TestHeartbeatOnFinishedTaskReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	mustCreate(t, s, models.TaskRun{RunID: "run-1", TaskName: "t", Status: models.StatusRunning, StartedAt: startedAt})

	if err := s.Finish("run-1", models.StatusSuccess, "", startedAt.Add(time.Second)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	// A worker sending a stray heartbeat after the task already finished
	// should not be able to resurrect it as "running".
	if err := s.Heartbeat("run-1", startedAt.Add(2*time.Second)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Heartbeat() on finished task error = %v, want ErrNotFound", err)
	}
}

func TestFinishSuccessComputesDuration(t *testing.T) {
	s := newTestStore(t)
	startedAt := time.Now().UTC().Truncate(time.Microsecond)
	mustCreate(t, s, models.TaskRun{RunID: "run-1", TaskName: "t", Status: models.StatusRunning, StartedAt: startedAt})

	finishedAt := startedAt.Add(750 * time.Millisecond)
	if err := s.Finish("run-1", models.StatusSuccess, "", finishedAt); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	got, err := s.Get("run-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != models.StatusSuccess {
		t.Errorf("Status = %q, want success", got.Status)
	}
	if got.DurationMs == nil || *got.DurationMs != 750 {
		t.Errorf("DurationMs = %v, want 750", got.DurationMs)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Errorf("FinishedAt = %v, want %v", got.FinishedAt, finishedAt)
	}
}

func TestFinishFailedStoresErrorMessage(t *testing.T) {
	s := newTestStore(t)
	startedAt := time.Now().UTC()
	mustCreate(t, s, models.TaskRun{RunID: "run-1", TaskName: "t", Status: models.StatusRunning, StartedAt: startedAt})

	if err := s.Finish("run-1", models.StatusFailed, "connection refused", startedAt.Add(time.Second)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	got, err := s.Get("run-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != models.StatusFailed {
		t.Errorf("Status = %q, want failed", got.Status)
	}
	if got.ErrorMessage != "connection refused" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "connection refused")
	}
}

func TestFinishUnknownRunReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)

	if err := s.Finish("does-not-exist", models.StatusSuccess, "", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Finish() error = %v, want ErrNotFound", err)
	}
}

func TestListFiltersByStatusAndOrdersByStartedAtDesc(t *testing.T) {
	s := newTestStore(t)
	base := time.Now().UTC().Truncate(time.Microsecond)

	mustCreate(t, s, models.TaskRun{RunID: "older", TaskName: "a", Status: models.StatusRunning, StartedAt: base})
	mustCreate(t, s, models.TaskRun{RunID: "newer", TaskName: "b", Status: models.StatusRunning, StartedAt: base.Add(time.Second)})
	if err := s.Finish("newer", models.StatusFailed, "boom", base.Add(2*time.Second)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	all, err := s.List("")
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List(\"\") returned %d tasks, want 2", len(all))
	}
	if all[0].RunID != "newer" || all[1].RunID != "older" {
		t.Errorf("List(\"\") order = [%s, %s], want [newer, older]", all[0].RunID, all[1].RunID)
	}

	failed, err := s.List("failed")
	if err != nil {
		t.Fatalf("List(\"failed\") error = %v", err)
	}
	if len(failed) != 1 || failed[0].RunID != "newer" {
		t.Fatalf("List(\"failed\") = %+v, want only [newer]", failed)
	}
}

func TestSweepTimeoutsMarksExpiredRunningTasks(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Expired: started 10s ago, TTL 5s, no heartbeat since start.
	mustCreate(t, s, models.TaskRun{
		RunID: "expired", TaskName: "a", Status: models.StatusRunning,
		StartedAt: now.Add(-10 * time.Second), TTLSeconds: int64ptr(5),
	})

	// Not expired: same start time, but TTL is longer than the elapsed time.
	mustCreate(t, s, models.TaskRun{
		RunID: "not-expired", TaskName: "b", Status: models.StatusRunning,
		StartedAt: now.Add(-10 * time.Second), TTLSeconds: int64ptr(60),
	})

	// Kept alive: started 10s ago but heartbeated 1s ago, well within its 5s TTL.
	mustCreate(t, s, models.TaskRun{
		RunID: "kept-alive", TaskName: "c", Status: models.StatusRunning,
		StartedAt: now.Add(-10 * time.Second), TTLSeconds: int64ptr(5),
	})
	if err := s.Heartbeat("kept-alive", now.Add(-1*time.Second)); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}

	// No TTL declared: must never be swept regardless of age.
	mustCreate(t, s, models.TaskRun{
		RunID: "no-ttl", TaskName: "d", Status: models.StatusRunning,
		StartedAt: now.Add(-1000 * time.Second),
	})

	timedOut, err := s.SweepTimeouts(now)
	if err != nil {
		t.Fatalf("SweepTimeouts() error = %v", err)
	}
	if len(timedOut) != 1 || timedOut[0].RunID != "expired" {
		t.Fatalf("SweepTimeouts() = %+v, want only [expired]", timedOut)
	}

	got, err := s.Get("expired")
	if err != nil {
		t.Fatalf("Get(expired) error = %v", err)
	}
	if got.Status != models.StatusTimeout {
		t.Errorf("expired task status = %q, want timeout", got.Status)
	}
	if got.DurationMs == nil || *got.DurationMs != 10000 {
		t.Errorf("expired task DurationMs = %v, want 10000", got.DurationMs)
	}

	for _, runID := range []string{"not-expired", "kept-alive", "no-ttl"} {
		got, err := s.Get(runID)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", runID, err)
		}
		if got.Status != models.StatusRunning {
			t.Errorf("%s status = %q, want still running", runID, got.Status)
		}
	}
}

func TestSweepTimeoutsIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	mustCreate(t, s, models.TaskRun{
		RunID: "expired", TaskName: "a", Status: models.StatusRunning,
		StartedAt: now.Add(-10 * time.Second), TTLSeconds: int64ptr(5),
	})

	if _, err := s.SweepTimeouts(now); err != nil {
		t.Fatalf("first SweepTimeouts() error = %v", err)
	}
	second, err := s.SweepTimeouts(now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second SweepTimeouts() error = %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second SweepTimeouts() = %+v, want no-op on an already-timed-out task", second)
	}
}

func TestDeleteOlderThanRemovesOnlyOldFinishedTasks(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	// Old and finished: should be deleted.
	mustCreate(t, s, models.TaskRun{
		RunID: "old-success", TaskName: "a", Status: models.StatusRunning,
		StartedAt: now.Add(-100 * 24 * time.Hour),
	})
	if err := s.Finish("old-success", models.StatusSuccess, "", now.Add(-100*24*time.Hour).Add(time.Minute)); err != nil {
		t.Fatalf("Finish(old-success) error = %v", err)
	}

	// Recent and finished: should survive.
	mustCreate(t, s, models.TaskRun{
		RunID: "recent-failed", TaskName: "b", Status: models.StatusRunning,
		StartedAt: now.Add(-time.Hour),
	})
	if err := s.Finish("recent-failed", models.StatusFailed, "boom", now.Add(-time.Hour).Add(time.Minute)); err != nil {
		t.Fatalf("Finish(recent-failed) error = %v", err)
	}

	// Old but still running: must never be deleted, regardless of age.
	mustCreate(t, s, models.TaskRun{
		RunID: "old-running", TaskName: "c", Status: models.StatusRunning,
		StartedAt: now.Add(-200 * 24 * time.Hour),
	})

	cutoff := now.Add(-30 * 24 * time.Hour)
	deleted, err := s.DeleteOlderThan(cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteOlderThan() deleted %d row(s), want 1", deleted)
	}

	if _, err := s.Get("old-success"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old-success: err = %v, want ErrNotFound (should have been deleted)", err)
	}
	if _, err := s.Get("recent-failed"); err != nil {
		t.Errorf("recent-failed was deleted, want it to survive: %v", err)
	}
	if _, err := s.Get("old-running"); err != nil {
		t.Errorf("old-running (still running) was deleted, want it to survive regardless of age: %v", err)
	}
}

func TestDeleteOlderThanIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()

	mustCreate(t, s, models.TaskRun{RunID: "old", TaskName: "a", Status: models.StatusRunning, StartedAt: now.Add(-100 * 24 * time.Hour)})
	if err := s.Finish("old", models.StatusSuccess, "", now.Add(-100*24*time.Hour).Add(time.Minute)); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}

	cutoff := now.Add(-30 * 24 * time.Hour)
	if _, err := s.DeleteOlderThan(cutoff); err != nil {
		t.Fatalf("first DeleteOlderThan() error = %v", err)
	}
	second, err := s.DeleteOlderThan(cutoff)
	if err != nil {
		t.Fatalf("second DeleteOlderThan() error = %v", err)
	}
	if second != 0 {
		t.Errorf("second DeleteOlderThan() deleted %d row(s), want 0 (nothing left to delete)", second)
	}
}

func mustCreate(t *testing.T, s *TaskStore, task models.TaskRun) {
	t.Helper()
	if err := s.Create(task); err != nil {
		t.Fatalf("Create(%+v) error = %v", task, err)
	}
}
