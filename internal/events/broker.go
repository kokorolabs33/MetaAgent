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

// Publish sends an event to all subscribers of a task, and also to conversation-level
// subscribers if the event has a ConversationID.
// If a subscriber's channel is full, the event is dropped (they can catch up from DB).
func (b *Broker) Publish(event *models.Event) {
	b.mu.RLock()
	taskSubs := b.subscribers[event.TaskID]
	var convSubs []chan *models.Event
	if event.ConversationID != "" {
		convSubs = b.subscribers["conv:"+event.ConversationID]
	}
	b.mu.RUnlock()

	for _, ch := range taskSubs {
		select {
		case ch <- event:
		default:
		}
	}
	for _, ch := range convSubs {
		select {
		case ch <- event:
		default:
		}
	}
}

// SubscribeConversation returns a channel that receives events for a given conversation.
// Call UnsubscribeConversation when done.
func (b *Broker) SubscribeConversation(conversationID string) chan *models.Event {
	return b.Subscribe("conv:" + conversationID)
}

// UnsubscribeConversation removes a channel from conversation subscriptions and closes it.
func (b *Broker) UnsubscribeConversation(conversationID string, ch chan *models.Event) {
	b.Unsubscribe("conv:"+conversationID, ch)
}

// SubscribeGlobal returns a channel that receives events for a global topic
// (e.g. "agents"). Unlike Subscribe, the topic key is used directly — not
// prefixed with a task ID. Call UnsubscribeGlobal when done.
func (b *Broker) SubscribeGlobal(topic string) chan *models.Event {
	return b.Subscribe(topic)
}

// UnsubscribeGlobal removes a channel from a global topic and closes it.
func (b *Broker) UnsubscribeGlobal(topic string, ch chan *models.Event) {
	b.Unsubscribe(topic, ch)
}

// PublishGlobal sends an event to all subscribers of a global topic.
// Unlike Publish, it does not route to task or conversation subscribers.
// If a subscriber's channel is full, the event is dropped.
func (b *Broker) PublishGlobal(topic string, event *models.Event) {
	b.mu.RLock()
	subs := b.subscribers[topic]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}
