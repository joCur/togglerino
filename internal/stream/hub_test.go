package stream

import (
	"sync"
	"testing"
	"time"
)

func TestSubscribeAndReceiveBroadcast(t *testing.T) {
	hub := NewHub()

	ch := hub.Subscribe("proj1", "staging")
	defer hub.Unsubscribe("proj1", "staging", ch)

	event := Event{FlagKey: "dark-mode", Value: true, Variant: "on"}
	hub.Broadcast("proj1", "staging", event)

	select {
	case received := <-ch:
		if received.FlagKey != event.FlagKey {
			t.Errorf("expected FlagKey %q, got %q", event.FlagKey, received.FlagKey)
		}
		if received.Value != event.Value {
			t.Errorf("expected Value %v, got %v", event.Value, received.Value)
		}
		if received.Variant != event.Variant {
			t.Errorf("expected Variant %q, got %q", event.Variant, received.Variant)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}
}

func TestUnsubscribeRemovesChannel(t *testing.T) {
	hub := NewHub()

	ch := hub.Subscribe("proj1", "prod")
	if count := hub.SubscriberCount("proj1", "prod"); count != 1 {
		t.Fatalf("expected 1 subscriber, got %d", count)
	}

	hub.Unsubscribe("proj1", "prod", ch)

	if count := hub.SubscriberCount("proj1", "prod"); count != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", count)
	}

	// Verify the channel is closed
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after unsubscribe")
	}
}

func TestBroadcastToEmptyScope(t *testing.T) {
	hub := NewHub()

	// Should not panic when broadcasting to a scope with no subscribers
	hub.Broadcast("nonexistent", "env", Event{FlagKey: "test", Value: false, Variant: ""})
}

func TestMultipleSubscribersReceiveSameEvent(t *testing.T) {
	hub := NewHub()

	const numSubscribers = 5
	channels := make([]chan Event, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		channels[i] = hub.Subscribe("proj1", "dev")
	}
	defer func() {
		for _, ch := range channels {
			hub.Unsubscribe("proj1", "dev", ch)
		}
	}()

	if count := hub.SubscriberCount("proj1", "dev"); count != numSubscribers {
		t.Fatalf("expected %d subscribers, got %d", numSubscribers, count)
	}

	event := Event{FlagKey: "feature-x", Value: "blue", Variant: "variant-b"}
	hub.Broadcast("proj1", "dev", event)

	for i, ch := range channels {
		select {
		case received := <-ch:
			if received.FlagKey != event.FlagKey {
				t.Errorf("subscriber %d: expected FlagKey %q, got %q", i, event.FlagKey, received.FlagKey)
			}
			if received.Value != event.Value {
				t.Errorf("subscriber %d: expected Value %v, got %v", i, event.Value, received.Value)
			}
			if received.Variant != event.Variant {
				t.Errorf("subscriber %d: expected Variant %q, got %q", i, event.Variant, received.Variant)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestBroadcastDropsEventWhenChannelFull(t *testing.T) {
	hub := NewHub()

	ch := hub.Subscribe("proj1", "staging")
	defer hub.Unsubscribe("proj1", "staging", ch)

	// Fill the channel buffer (capacity 16)
	for i := 0; i < 16; i++ {
		hub.Broadcast("proj1", "staging", Event{FlagKey: "flag", Value: i, Variant: ""})
	}

	// This broadcast should be dropped (non-blocking) because the channel is full
	hub.Broadcast("proj1", "staging", Event{FlagKey: "dropped", Value: true, Variant: ""})

	// Drain the channel and verify we got 16 events, none with FlagKey "dropped"
	for i := 0; i < 16; i++ {
		select {
		case e := <-ch:
			if e.FlagKey == "dropped" {
				t.Fatal("expected the overflow event to be dropped, but it was received")
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out draining event %d", i)
		}
	}

	// Channel should now be empty
	select {
	case e := <-ch:
		t.Fatalf("expected channel to be empty, but got event: %+v", e)
	default:
		// good
	}
}

func TestScopesAreIsolated(t *testing.T) {
	hub := NewHub()

	ch1 := hub.Subscribe("proj1", "staging")
	ch2 := hub.Subscribe("proj2", "prod")
	defer hub.Unsubscribe("proj1", "staging", ch1)
	defer hub.Unsubscribe("proj2", "prod", ch2)

	// Broadcast only to proj1:staging
	hub.Broadcast("proj1", "staging", Event{FlagKey: "flag-a", Value: true, Variant: ""})

	// ch1 should receive the event
	select {
	case <-ch1:
		// good
	case <-time.After(time.Second):
		t.Fatal("ch1 timed out waiting for event")
	}

	// ch2 should NOT receive anything
	select {
	case e := <-ch2:
		t.Fatalf("ch2 should not have received an event, got: %+v", e)
	case <-time.After(50 * time.Millisecond):
		// good
	}
}

func TestConcurrentSubscribeUnsubscribeBroadcast(t *testing.T) {
	hub := NewHub()

	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // subscribers, unsubscribers, broadcasters

	// Concurrent subscribers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ch := hub.Subscribe("proj1", "env1")
				hub.Unsubscribe("proj1", "env1", ch)
			}
		}()
	}

	// Concurrent broadcasters
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				hub.Broadcast("proj1", "env1", Event{FlagKey: "flag", Value: j, Variant: ""})
			}
		}()
	}

	// Concurrent subscriber count readers
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				hub.SubscriberCount("proj1", "env1")
			}
		}()
	}

	wg.Wait()

	// After all goroutines finish, subscriber count should be 0
	if count := hub.SubscriberCount("proj1", "env1"); count != 0 {
		t.Errorf("expected 0 subscribers after concurrent test, got %d", count)
	}
}
