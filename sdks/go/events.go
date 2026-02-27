package togglerino

import (
	"sync"
	"time"
)

type eventType string

const (
	eventReady         eventType = "ready"
	eventChange        eventType = "change"
	eventDeleted       eventType = "deleted"
	eventError         eventType = "error"
	eventReconnecting  eventType = "reconnecting"
	eventReconnected   eventType = "reconnected"
	eventContextChange eventType = "context_change"
)

type listener struct {
	id int
	fn func(any)
}

type eventEmitter struct {
	mu        sync.RWMutex
	listeners map[eventType][]listener
	nextID    int
}

func newEventEmitter() *eventEmitter {
	return &eventEmitter{
		listeners: make(map[eventType][]listener),
	}
}

func (e *eventEmitter) on(event eventType, fn func(any)) func() {
	e.mu.Lock()
	id := e.nextID
	e.nextID++
	e.listeners[event] = append(e.listeners[event], listener{id: id, fn: fn})
	e.mu.Unlock()

	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		ls := e.listeners[event]
		for i, l := range ls {
			if l.id == id {
				e.listeners[event] = append(ls[:i], ls[i+1:]...)
				break
			}
		}
	}
}

func (e *eventEmitter) emit(event eventType, payload any) {
	e.mu.RLock()
	ls := make([]listener, len(e.listeners[event]))
	copy(ls, e.listeners[event])
	e.mu.RUnlock()

	for _, l := range ls {
		func() {
			defer func() { recover() }()
			l.fn(payload)
		}()
	}
}

func (e *eventEmitter) clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = make(map[eventType][]listener)
}

// reconnectingPayload is the internal payload for reconnecting events.
type reconnectingPayload struct {
	Attempt int
	Delay   time.Duration
}
