package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"taskhub/internal/events"
	"taskhub/internal/models"
)

func TestMultiStream_EmptyIDs_Returns400(t *testing.T) {
	h := &StreamHandler{Broker: events.NewBroker()}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/stream", nil)
	rec := httptest.NewRecorder()

	h.MultiStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ids query param required") {
		t.Fatalf("expected error message about ids, got %q", rec.Body.String())
	}
}

func TestMultiStream_WhitespaceOnlyIDs_Returns400(t *testing.T) {
	h := &StreamHandler{Broker: events.NewBroker()}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/stream?ids=%20%2C%20", nil)
	rec := httptest.NewRecorder()

	h.MultiStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ids query param required") {
		t.Fatalf("expected error message about ids, got %q", rec.Body.String())
	}
}

func TestMultiStream_TooManyIDs_Returns400(t *testing.T) {
	h := &StreamHandler{Broker: events.NewBroker()}
	parts := make([]string, 51)
	for i := range parts {
		parts[i] = "t-" + string(rune('a'+i%26)) + "-" + strings.Repeat("x", i%3+1)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/stream?ids="+strings.Join(parts, ","), nil)
	rec := httptest.NewRecorder()

	h.MultiStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "too many ids") {
		t.Fatalf("expected 'too many ids' error, got %q", rec.Body.String())
	}
}

func TestMultiStream_ValidIDs_StreamsPublishedEvents(t *testing.T) {
	broker := events.NewBroker()
	h := &StreamHandler{Broker: broker}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/stream?ids=task-1,task-2", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.MultiStream(rec, req)
		close(done)
	}()

	// Give the handler a moment to subscribe before publishing.
	time.Sleep(50 * time.Millisecond)

	evt := &models.Event{
		ID:        "evt-1",
		TaskID:    "task-1",
		Type:      "subtask.completed",
		ActorType: "system",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now(),
	}
	broker.Publish(evt)

	// Wait for the event to land in the recorder.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.Body.String(), `"task_id":"task-1"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("expected Cache-Control no-cache, got %q", rec.Header().Get("Cache-Control"))
	}
	if !strings.Contains(rec.Body.String(), `"task_id":"task-1"`) {
		t.Fatalf("expected event body to include task_id, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"type":"subtask.completed"`) {
		t.Fatalf("expected type in body, got %q", rec.Body.String())
	}
}

func TestMultiStream_DuplicateIDs_AreDeduplicated(t *testing.T) {
	broker := events.NewBroker()
	h := &StreamHandler{Broker: broker}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/stream?ids=same,same,same", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.MultiStream(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish one event; if Subscribe was called 3 times, the broker would
	// deliver it 3 times (via 3 channels). writeSSEEvent prints one frame
	// per receive, so the body would contain 3 copies of "id":"evt-x".
	broker.Publish(&models.Event{
		ID:        "evt-x",
		TaskID:    "same",
		Type:      "task.running",
		ActorType: "system",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now(),
	})

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	count := strings.Count(rec.Body.String(), `"id":"evt-x"`)
	if count != 1 {
		t.Fatalf("expected 1 event frame from deduplicated subscription, got %d (body=%q)", count, rec.Body.String())
	}
}
