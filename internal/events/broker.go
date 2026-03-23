package events

import (
	"sync"

	"taskhub/internal/models"
)

// Broker provides in-memory pub/sub for real-time event fanout.
// The event store handles persistence; the broker handles live delivery.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan *models.Event // task_id → channels
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string][]chan *models.Event),
	}
}

// Subscribe returns a channel that receives events for a given task.
// Call Unsubscribe when done.
func (b *Broker) Subscribe(taskID string) chan *models.Event {
	ch := make(chan *models.Event, 64)
	b.mu.Lock()
	b.subscribers[taskID] = append(b.subscribers[taskID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel from subscriptions and closes it.
func (b *Broker) Unsubscribe(taskID string, ch chan *models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			b.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
	if len(b.subscribers[taskID]) == 0 {
		delete(b.subscribers, taskID)
	}
}

// Publish sends an event to all subscribers of a task.
// If a subscriber's channel is full, the event is dropped (they can catch up from DB).
func (b *Broker) Publish(event *models.Event) {
	b.mu.RLock()
	subs := b.subscribers[event.TaskID]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Subscriber too slow, drop event (they can catch up from DB)
		}
	}
}
