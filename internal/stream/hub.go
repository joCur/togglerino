package stream

import "sync"

// Event represents a flag change event sent to SSE clients.
type Event struct {
	FlagKey string `json:"flag_key"`
	Value   any    `json:"value"`
	Variant string `json:"variant"`
}

// Hub manages SSE subscriptions per project/environment.
type Hub struct {
	mu sync.RWMutex
	// Key: "projectKey:envKey", Value: set of subscriber channels
	subscribers map[string]map[chan Event]struct{}
}

// NewHub creates a new Hub ready for use.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan Event]struct{}),
	}
}

// Subscribe creates a new channel for receiving events for a project/environment.
// Returns the channel. Caller must call Unsubscribe when done.
func (h *Hub) Subscribe(projectKey, envKey string) chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := projectKey + ":" + envKey
	if h.subscribers[key] == nil {
		h.subscribers[key] = make(map[chan Event]struct{})
	}

	ch := make(chan Event, 16) // buffered to avoid blocking broadcasts
	h.subscribers[key][ch] = struct{}{}
	return ch
}

// Unsubscribe removes a channel from the subscriber set and closes it.
func (h *Hub) Unsubscribe(projectKey, envKey string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := projectKey + ":" + envKey
	if subs, ok := h.subscribers[key]; ok {
		delete(subs, ch)
		close(ch)
		if len(subs) == 0 {
			delete(h.subscribers, key)
		}
	}
}

// Broadcast sends an event to all subscribers for a project/environment.
// Non-blocking: if a subscriber's channel is full, the event is dropped for that subscriber.
func (h *Hub) Broadcast(projectKey, envKey string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := projectKey + ":" + envKey
	if subs, ok := h.subscribers[key]; ok {
		for ch := range subs {
			select {
			case ch <- event:
			default:
				// Drop event if subscriber is too slow
			}
		}
	}
}

// SubscriberCount returns the number of subscribers for a project/environment (for testing/monitoring).
func (h *Hub) SubscriberCount(projectKey, envKey string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := projectKey + ":" + envKey
	return len(h.subscribers[key])
}

// Close closes all subscriber channels and clears the subscribers map.
// It should be called during graceful shutdown to notify all connected SSE clients.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for key, subs := range h.subscribers {
		for ch := range subs {
			close(ch)
		}
		delete(h.subscribers, key)
	}
}
