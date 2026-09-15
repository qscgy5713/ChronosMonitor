package store

import (
	"errors"
	"testing"
	"time"

	"chronosmonitor/internal/models"
)

func newTestScheduleStore(t *testing.T) *ScheduleStore {
	t.Helper()
	return NewScheduleStore(newTestConn(t))
}

func TestUpsertCreatesNewSchedule(t *testing.T) {
	s := newTestScheduleStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	if err := s.Upsert("daily-report", 86400, 300, now); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, err := s.Get("daily-report")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ExpectedIntervalSeconds != 86400 || got.GracePeriodSeconds != 300 {
		t.Errorf("got = %+v, want interval=86400 grace=300", got)
	}
	if got.Status != models.ScheduleStatusOK {
		t.Errorf("Status = %q, want ok for a freshly registered schedule", got.Status)
	}
	if got.LastSeenAt != nil {
		t.Errorf("LastSeenAt = %v, want nil (never touched yet)", got.LastSeenAt)
	}
}

func TestUpsertUpdatingExistingScheduleDoesNotResetLastSeenOrStatus(t *testing.T) {
	s := newTestScheduleStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	if err := s.Upsert("daily-report", 86400, 0, now); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	touchedAt := now.Add(time.Minute)
	if err := s.Touch("daily-report", touchedAt); err != nil {
		t.Fatalf("Touch() error = %v", err)
	}

	// Re-register with a different interval, as if the operator changed it.
	if err := s.Upsert("daily-report", 43200, 600, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}

	got, err := s.Get("daily-report")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ExpectedIntervalSeconds != 43200 || got.GracePeriodSeconds != 600 {
		t.Errorf("got = %+v, want the updated interval=43200 grace=600", got)
	}
	if got.LastSeenAt == nil || !got.LastSeenAt.Equal(touchedAt) {
		t.Errorf("LastSeenAt = %v, want it preserved from before the re-registration (%v)", got.LastSeenAt, touchedAt)
	}
}

func TestTouchUpdatesLastSeenAndResetsStatusToOK(t *testing.T) {
	s := newTestScheduleStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	if err := s.Upsert("daily-report", 60, 0, now); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	// Simulate a missed episode.
	if _, err := s.DetectMissed(now.Add(time.Hour)); err != nil {
		t.Fatalf("DetectMissed() error = %v", err)
	}
	if got, _ := s.Get("daily-report"); got.Status != models.ScheduleStatusMissed {
		t.Fatalf("precondition failed: schedule should be 'missed' before Touch(), got %q", got.Status)
	}

	touchedAt := now.Add(2 * time.Hour)
	if err := s.Touch("daily-report", touchedAt); err != nil {
		t.Fatalf("Touch() error = %v", err)
	}

	got, err := s.Get("daily-report")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != models.ScheduleStatusOK {
		t.Errorf("Status = %q, want ok after Touch() clears a missed episode", got.Status)
	}
	if got.LastSeenAt == nil || !got.LastSeenAt.Equal(touchedAt) {
		t.Errorf("LastSeenAt = %v, want %v", got.LastSeenAt, touchedAt)
	}
}

func TestTouchOnUnregisteredTaskNameIsNoOp(t *testing.T) {
	s := newTestScheduleStore(t)

	if err := s.Touch("no-such-schedule", time.Now()); err != nil {
		t.Errorf("Touch() on an unregistered task_name error = %v, want nil (best-effort no-op)", err)
	}
}

func TestGetUnknownScheduleReturnsErrNotFound(t *testing.T) {
	s := newTestScheduleStore(t)

	if _, err := s.Get("does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestListReturnsSchedulesOrderedByTaskName(t *testing.T) {
	s := newTestScheduleStore(t)
	now := time.Now().UTC()

	if err := s.Upsert("zebra-job", 60, 0, now); err != nil {
		t.Fatalf("Upsert(zebra-job) error = %v", err)
	}
	if err := s.Upsert("alpha-job", 60, 0, now); err != nil {
		t.Fatalf("Upsert(alpha-job) error = %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 || list[0].TaskName != "alpha-job" || list[1].TaskName != "zebra-job" {
		t.Fatalf("List() = %+v, want [alpha-job, zebra-job]", list)
	}
}

func TestDeleteRemovesSchedule(t *testing.T) {
	s := newTestScheduleStore(t)

	if err := s.Upsert("daily-report", 60, 0, time.Now()); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := s.Delete("daily-report"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := s.Get("daily-report"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
}

func TestDeleteUnknownScheduleReturnsErrNotFound(t *testing.T) {
	s := newTestScheduleStore(t)

	if err := s.Delete("does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestDetectMissedFlagsOverdueSchedules(t *testing.T) {
	s := newTestScheduleStore(t)
	now := time.Now().UTC()

	// Registered long ago, never touched: overdue relative to registration time.
	if err := s.Upsert("never-seen", 60, 0, now.Add(-time.Hour)); err != nil {
		t.Fatalf("Upsert(never-seen) error = %v", err)
	}

	// Touched recently: within its window, not overdue.
	if err := s.Upsert("healthy", 60, 0, now.Add(-time.Hour)); err != nil {
		t.Fatalf("Upsert(healthy) error = %v", err)
	}
	if err := s.Touch("healthy", now.Add(-10*time.Second)); err != nil {
		t.Fatalf("Touch(healthy) error = %v", err)
	}

	// Touched a while ago, but still within interval + grace.
	if err := s.Upsert("within-grace", 60, 30, now.Add(-time.Hour)); err != nil {
		t.Fatalf("Upsert(within-grace) error = %v", err)
	}
	if err := s.Touch("within-grace", now.Add(-80*time.Second)); err != nil { // 80s < 60+30
		t.Fatalf("Touch(within-grace) error = %v", err)
	}

	// Touched just past interval + grace: overdue.
	if err := s.Upsert("past-grace", 60, 30, now.Add(-time.Hour)); err != nil {
		t.Fatalf("Upsert(past-grace) error = %v", err)
	}
	if err := s.Touch("past-grace", now.Add(-100*time.Second)); err != nil { // 100s > 60+30
		t.Fatalf("Touch(past-grace) error = %v", err)
	}

	missed, err := s.DetectMissed(now)
	if err != nil {
		t.Fatalf("DetectMissed() error = %v", err)
	}

	gotNames := map[string]bool{}
	for _, m := range missed {
		gotNames[m.TaskName] = true
	}
	wantMissed := []string{"never-seen", "past-grace"}
	for _, name := range wantMissed {
		if !gotNames[name] {
			t.Errorf("DetectMissed() did not flag %q, want it flagged", name)
		}
	}
	wantHealthy := []string{"healthy", "within-grace"}
	for _, name := range wantHealthy {
		if gotNames[name] {
			t.Errorf("DetectMissed() flagged %q, want it left alone", name)
		}
	}

	// Persisted status should reflect the flagging.
	got, err := s.Get("never-seen")
	if err != nil {
		t.Fatalf("Get(never-seen) error = %v", err)
	}
	if got.Status != models.ScheduleStatusMissed {
		t.Errorf("never-seen status = %q, want missed", got.Status)
	}
}

func TestDetectMissedDoesNotReflagAlreadyMissedSchedules(t *testing.T) {
	s := newTestScheduleStore(t)
	now := time.Now().UTC()

	if err := s.Upsert("never-seen", 60, 0, now.Add(-time.Hour)); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	first, err := s.DetectMissed(now)
	if err != nil {
		t.Fatalf("first DetectMissed() error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first DetectMissed() = %+v, want exactly 1", first)
	}

	second, err := s.DetectMissed(now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second DetectMissed() error = %v", err)
	}
	if len(second) != 0 {
		t.Errorf("second DetectMissed() = %+v, want no-op on an already-missed schedule", second)
	}
}
