package override

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockDeleter struct {
	calls   atomic.Int64
	deleted int64
	err     error
}

func (m *mockDeleter) DeleteExpired(_ context.Context) (int64, error) {
	m.calls.Add(1)
	return m.deleted, m.err
}

type mockPurger struct {
	calls atomic.Int64
}

func (m *mockPurger) PurgeExpiredOverrides() {
	m.calls.Add(1)
}

func TestNewCleaner(t *testing.T) {
	c := NewCleaner(&mockDeleter{}, nil, 15*time.Minute)
	if c == nil {
		t.Fatal("expected non-nil cleaner")
	}
	if c.interval != 15*time.Minute {
		t.Fatalf("expected 15m interval, got %v", c.interval)
	}
}

func TestCleaner_Run_CallsDeleteExpired(t *testing.T) {
	mock := &mockDeleter{deleted: 3}
	c := NewCleaner(mock, nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Wait for at least 2 ticks
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done

	calls := mock.calls.Load()
	if calls < 2 {
		t.Fatalf("expected at least 2 calls to DeleteExpired, got %d", calls)
	}
}

func TestCleaner_Run_StopsOnContextCancel(t *testing.T) {
	mock := &mockDeleter{}
	c := NewCleaner(mock, nil, 10*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// success — Run exited
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestCleaner_Run_CallsPurgerAfterDelete(t *testing.T) {
	deleter := &mockDeleter{deleted: 1}
	purger := &mockPurger{}
	c := NewCleaner(deleter, purger, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Wait for at least 1 tick
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	deleteCalls := deleter.calls.Load()
	purgeCalls := purger.calls.Load()
	if deleteCalls < 1 {
		t.Fatalf("expected at least 1 call to DeleteExpired, got %d", deleteCalls)
	}
	if purgeCalls < 1 {
		t.Fatalf("expected at least 1 call to PurgeExpiredOverrides, got %d", purgeCalls)
	}
	if purgeCalls != deleteCalls {
		t.Fatalf("expected purger calls (%d) to equal deleter calls (%d)", purgeCalls, deleteCalls)
	}
}
