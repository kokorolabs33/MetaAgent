package sse

import (
	"encoding/json"
	"sync"
)

// Broker manages SSE subscriptions keyed by channel ID.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

func NewBroker() *Broker {
	return &Broker{subscribers: make(map[string][]chan string)}
}

// Subscribe creates a new subscriber channel for the given channel ID.
func (b *Broker) Subscribe(channelID string) chan string {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.subscribers[channelID] = append(b.subscribers[channelID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber without closing its channel.
// The channel gets garbage collected when the subscriber exits.
func (b *Broker) Unsubscribe(channelID string, sub chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[channelID]
	for i, s := range subs {
		if s == sub {
			b.subscribers[channelID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Publish sends a typed SSE event to all subscribers of a channel.
func (b *Broker) Publish(channelID string, eventType string, data any) {
	payload := map[string]any{"type": eventType, "data": data}
	msg, _ := json.Marshal(payload)

	b.mu.RLock()
	subs := make([]chan string, len(b.subscribers[channelID]))
	copy(subs, b.subscribers[channelID])
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub <- string(msg):
		default:
			// drop if buffer full — non-blocking
		}
	}
}
