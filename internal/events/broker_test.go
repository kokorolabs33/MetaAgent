package events

import (
	"testing"
	"time"

	"taskhub/internal/models"
)

func TestSubscribe_Publish_Receive(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe("task-1")
	defer b.Unsubscribe("task-1", ch)

	evt := &models.Event{
		ID:     "evt-1",
		TaskID: "task-1",
		Type:   "task.started",
	}
	b.Publish(evt)

	select {
	case got := <-ch:
		if got.ID != evt.ID {
			t.Errorf("event ID = %q, want %q", got.ID, evt.ID)
		}
		if got.Type != evt.Type {
			t.Errorf("event Type = %q, want %q", got.Type, evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestUnsubscribe_RemovesChannel(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe("task-1")
	b.Unsubscribe("task-1", ch)

	// Channel should be closed after unsubscribe
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}

	// Subscriber list should be empty (key deleted)
	b.mu.RLock()
	_, exists := b.subscribers["task-1"]
	b.mu.RUnlock()
	if exists {
		t.Error("expected subscriber list to be cleaned up after last unsubscribe")
	}
}

func TestPublish_NoSubscribers_NoPanic(t *testing.T) {
	b := NewBroker()

	// Should not panic
	evt := &models.Event{
		ID:     "evt-1",
		TaskID: "no-subscribers",
		Type:   "test",
	}
	b.Publish(evt)
}

func TestSubscribeConversation_Publish_FanOut(t *testing.T) {
	b := NewBroker()
	ch := b.SubscribeConversation("conv-1")
	defer b.UnsubscribeConversation("conv-1", ch)

	evt := &models.Event{
		ID:             "evt-1",
		TaskID:         "task-1",
		ConversationID: "conv-1",
		Type:           "message",
	}
	b.Publish(evt)

	select {
	case got := <-ch:
		if got.ID != evt.ID {
			t.Errorf("event ID = %q, want %q", got.ID, evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for conversation event")
	}
}

func TestUnsubscribeConversation(t *testing.T) {
	b := NewBroker()
	ch := b.SubscribeConversation("conv-1")
	b.UnsubscribeConversation("conv-1", ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}

	// Internal key should be cleaned up
	b.mu.RLock()
	_, exists := b.subscribers["conv:conv-1"]
	b.mu.RUnlock()
	if exists {
		t.Error("expected conversation subscriber list to be cleaned up")
	}
}

func TestMultipleSubscribers_SameTopic(t *testing.T) {
	b := NewBroker()
	ch1 := b.Subscribe("task-1")
	ch2 := b.Subscribe("task-1")
	defer b.Unsubscribe("task-1", ch1)
	defer b.Unsubscribe("task-1", ch2)

	evt := &models.Event{
		ID:     "evt-1",
		TaskID: "task-1",
		Type:   "test",
	}
	b.Publish(evt)

	// Both subscribers should receive the event
	for i, ch := range []chan *models.Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.ID != evt.ID {
				t.Errorf("subscriber %d: event ID = %q, want %q", i, got.ID, evt.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestPublish_FansOutToBothTaskAndConversation(t *testing.T) {
	b := NewBroker()
	taskCh := b.Subscribe("task-1")
	convCh := b.SubscribeConversation("conv-1")
	defer b.Unsubscribe("task-1", taskCh)
	defer b.UnsubscribeConversation("conv-1", convCh)

	evt := &models.Event{
		ID:             "evt-1",
		TaskID:         "task-1",
		ConversationID: "conv-1",
		Type:           "message",
	}
	b.Publish(evt)

	// Task subscriber should receive
	select {
	case got := <-taskCh:
		if got.ID != evt.ID {
			t.Errorf("task subscriber: event ID = %q, want %q", got.ID, evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("task subscriber: timed out")
	}

	// Conversation subscriber should also receive
	select {
	case got := <-convCh:
		if got.ID != evt.ID {
			t.Errorf("conv subscriber: event ID = %q, want %q", got.ID, evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("conv subscriber: timed out")
	}
}

func TestPublish_ConversationOnly_NoTaskFanout(t *testing.T) {
	b := NewBroker()
	// Subscribe to a different task
	taskCh := b.Subscribe("task-other")
	convCh := b.SubscribeConversation("conv-1")
	defer b.Unsubscribe("task-other", taskCh)
	defer b.UnsubscribeConversation("conv-1", convCh)

	evt := &models.Event{
		ID:             "evt-1",
		TaskID:         "task-1",
		ConversationID: "conv-1",
		Type:           "message",
	}
	b.Publish(evt)

	// Task subscriber for a different task should NOT receive
	select {
	case <-taskCh:
		t.Error("task subscriber for different task should not receive event")
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	// Conversation subscriber should receive
	select {
	case got := <-convCh:
		if got.ID != evt.ID {
			t.Errorf("conv subscriber: event ID = %q, want %q", got.ID, evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("conv subscriber: timed out")
	}
}

func TestUnsubscribe_OneOfMultiple(t *testing.T) {
	b := NewBroker()
	ch1 := b.Subscribe("task-1")
	ch2 := b.Subscribe("task-1")

	// Unsubscribe only ch1
	b.Unsubscribe("task-1", ch1)

	// ch1 should be closed
	_, ok := <-ch1
	if ok {
		t.Error("expected ch1 to be closed")
	}

	// ch2 should still work
	evt := &models.Event{ID: "evt-1", TaskID: "task-1", Type: "test"}
	b.Publish(evt)

	select {
	case got := <-ch2:
		if got.ID != evt.ID {
			t.Errorf("ch2: event ID = %q, want %q", got.ID, evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("ch2: timed out")
	}

	b.Unsubscribe("task-1", ch2)
}

func TestPublish_DropsWhenBufferFull(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe("task-1")
	defer b.Unsubscribe("task-1", ch)

	// Fill the buffer (capacity is 64)
	for i := range 64 {
		b.Publish(&models.Event{
			ID:     "evt-fill",
			TaskID: "task-1",
			Type:   "fill-" + string(rune('0'+i%10)),
		})
	}

	// This should be dropped (buffer full), not block
	b.Publish(&models.Event{
		ID:     "evt-dropped",
		TaskID: "task-1",
		Type:   "overflow",
	})

	// Drain the buffer - should get exactly 64 events
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Errorf("expected 64 buffered events, got %d", count)
	}
}
