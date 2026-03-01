package togglerino

import (
	"sync"
	"testing"
)

func TestEventEmitter_Subscribe_And_Emit(t *testing.T) {
	em := newEventEmitter()
	called := false
	em.on(eventReady, func(payload any) {
		called = true
	})
	em.emit(eventReady, nil)
	if !called {
		t.Fatal("listener was not called")
	}
}

func TestEventEmitter_Unsubscribe(t *testing.T) {
	em := newEventEmitter()
	called := false
	unsub := em.on(eventReady, func(payload any) {
		called = true
	})
	unsub()
	em.emit(eventReady, nil)
	if called {
		t.Fatal("listener was called after unsubscribe")
	}
}

func TestEventEmitter_Multiple_Listeners(t *testing.T) {
	em := newEventEmitter()
	var count int
	em.on(eventReady, func(payload any) { count++ })
	em.on(eventReady, func(payload any) { count++ })
	em.emit(eventReady, nil)
	if count != 2 {
		t.Fatalf("expected 2 calls, got %d", count)
	}
}

func TestEventEmitter_Panicking_Listener_Does_Not_Break_Others(t *testing.T) {
	em := newEventEmitter()
	secondCalled := false
	em.on(eventReady, func(payload any) {
		panic("boom")
	})
	em.on(eventReady, func(payload any) {
		secondCalled = true
	})
	em.emit(eventReady, nil)
	if !secondCalled {
		t.Fatal("second listener was not called after first panicked")
	}
}

func TestEventEmitter_Clear(t *testing.T) {
	em := newEventEmitter()
	called := false
	em.on(eventReady, func(payload any) { called = true })
	em.clear()
	em.emit(eventReady, nil)
	if called {
		t.Fatal("listener was called after clear")
	}
}

func TestEventEmitter_Concurrent_Access(t *testing.T) {
	em := newEventEmitter()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub := em.on(eventReady, func(payload any) {})
			em.emit(eventReady, nil)
			unsub()
		}()
	}
	wg.Wait()
}
