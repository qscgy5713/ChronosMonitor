package broker

import (
	"testing"
	"time"

	"chronosmonitor/internal/models"
)

func TestPublishDeliversToSubscriber(t *testing.T) {
	hub := New()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	hub.Publish(Event{Type: EventStarted, Task: models.TaskRun{RunID: "abc"}})

	select {
	case evt := <-ch:
		if evt.Type != EventStarted || evt.Task.RunID != "abc" {
			t.Errorf("received %+v, want Type=%q Task.RunID=abc", evt, EventStarted)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestPublishFansOutToAllSubscribers(t *testing.T) {
	hub := New()
	ch1, unsub1 := hub.Subscribe()
	defer unsub1()
	ch2, unsub2 := hub.Subscribe()
	defer unsub2()

	hub.Publish(Event{Type: EventSucceeded, Task: models.TaskRun{RunID: "abc"}})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case evt := <-ch:
			if evt.Task.RunID != "abc" {
				t.Errorf("subscriber %d got %+v, want RunID=abc", i, evt)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out waiting for event", i)
		}
	}
}

func TestUnsubscribeClosesChannelAndStopsDelivery(t *testing.T) {
	hub := New()
	ch, unsubscribe := hub.Subscribe()
	unsubscribe()

	hub.Publish(Event{Type: EventStarted})

	select {
	case evt, ok := <-ch:
		if ok {
			t.Fatalf("expected closed channel after unsubscribe, got event %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for channel to report closed")
	}
}

func TestUnsubscribeIsSafeToCallTwice(t *testing.T) {
	hub := New()
	_, unsubscribe := hub.Subscribe()

	unsubscribe()
	unsubscribe() // must not panic (e.g. double close of the channel)
}

func TestSlowSubscriberDoesNotBlockPublish(t *testing.T) {
	hub := New()
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	// The subscriber never reads, so once its buffered channel fills up,
	// Publish must drop events rather than block the caller.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			hub.Publish(Event{Type: EventHeartbeat})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a slow subscriber instead of dropping events")
	}

	// Drain whatever made it into the buffer to confirm the hub is still
	// functional afterwards.
	select {
	case <-ch:
	default:
		t.Fatal("expected at least one buffered event to have been delivered")
	}
}
