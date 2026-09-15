package notifier

import (
	"context"
	"sync"
	"testing"
	"time"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/models"
)

type fakeNotifier struct {
	mu    sync.Mutex
	calls []models.TaskRun
}

func (f *fakeNotifier) Notify(task models.TaskRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, task)
	return nil
}

func (f *fakeNotifier) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestHandle_OnlyForwardsFailedAndTimeout(t *testing.T) {
	fake := &fakeNotifier{}
	d := NewDispatcher(broker.New(), fake, 0, time.Minute)

	events := []string{
		broker.EventStarted,
		broker.EventHeartbeat,
		broker.EventSucceeded,
		broker.EventFailed,
		broker.EventTimeout,
	}
	for _, evtType := range events {
		d.handle(broker.Event{Type: evtType, Task: models.TaskRun{RunID: evtType}})
	}

	if fake.callCount() != 2 {
		t.Fatalf("Notify called %d times, want 2 (failed + timeout only)", fake.callCount())
	}
	if fake.calls[0].RunID != broker.EventFailed || fake.calls[1].RunID != broker.EventTimeout {
		t.Errorf("notified tasks = %+v, want [failed, timeout]", fake.calls)
	}
}

func TestHandle_RateLimitSuppressesBurstsWithinWindow(t *testing.T) {
	fake := &fakeNotifier{}
	d := NewDispatcher(broker.New(), fake, 2, time.Minute)
	now := time.Now()
	d.now = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		d.handle(broker.Event{Type: broker.EventFailed, Task: models.TaskRun{RunID: "run"}})
	}

	if fake.callCount() != 2 {
		t.Fatalf("Notify called %d times, want exactly 2 (rate limit = 2/window)", fake.callCount())
	}
}

func TestHandle_RateLimitResetsAfterWindowElapses(t *testing.T) {
	fake := &fakeNotifier{}
	d := NewDispatcher(broker.New(), fake, 1, time.Minute)
	now := time.Now()
	d.now = func() time.Time { return now }

	d.handle(broker.Event{Type: broker.EventFailed, Task: models.TaskRun{RunID: "1"}})
	d.handle(broker.Event{Type: broker.EventFailed, Task: models.TaskRun{RunID: "2"}}) // suppressed

	now = now.Add(time.Minute + time.Second) // advance past the window
	d.handle(broker.Event{Type: broker.EventFailed, Task: models.TaskRun{RunID: "3"}})

	if fake.callCount() != 2 {
		t.Fatalf("Notify called %d times, want 2 (one per window)", fake.callCount())
	}
	if fake.calls[1].RunID != "3" {
		t.Errorf("second notified run = %q, want %q", fake.calls[1].RunID, "3")
	}
}

func TestHandle_ZeroLimitMeansUnlimited(t *testing.T) {
	fake := &fakeNotifier{}
	d := NewDispatcher(broker.New(), fake, 0, time.Minute)

	for i := 0; i < 50; i++ {
		d.handle(broker.Event{Type: broker.EventFailed, Task: models.TaskRun{RunID: "run"}})
	}

	if fake.callCount() != 50 {
		t.Fatalf("Notify called %d times, want 50 (limit=0 means unlimited)", fake.callCount())
	}
}

func TestRun_SubscribesAndStopsOnContextCancel(t *testing.T) {
	hub := broker.New()
	fake := &fakeNotifier{}
	d := NewDispatcher(hub, fake, 0, time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()

	// Give Run a moment to subscribe before publishing.
	time.Sleep(20 * time.Millisecond)
	hub.Publish(broker.Event{Type: broker.EventFailed, Task: models.TaskRun{RunID: "abc"}})
	time.Sleep(20 * time.Millisecond)

	if fake.callCount() != 1 {
		t.Fatalf("Notify called %d times via Run(), want 1", fake.callCount())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}
