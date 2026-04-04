package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"taskhub/internal/events"
	"taskhub/internal/models"
)

// StreamHandler provides the SSE streaming endpoint for task events.
type StreamHandler struct {
	EventStore *events.Store
	Broker     *events.Broker
}

// Stream handles GET /tasks/{id}/events.
// It replays historical events from the database then streams live events via SSE.
// Supports Last-Event-ID for reconnection catchup.
func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	// Set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Determine replay strategy based on Last-Event-ID.
	lastEventID := r.Header.Get("Last-Event-ID")

	var replayEvents []models.Event
	var err error

	if lastEventID != "" {
		// Look up the event to get its created_at for cursor-based catchup.
		lastEvt, lookupErr := h.EventStore.GetByID(r.Context(), lastEventID)
		if lookupErr != nil {
			// If the event ID is not found, replay everything.
			replayEvents, err = h.EventStore.ListByTask(r.Context(), taskID)
		} else {
			replayEvents, err = h.EventStore.ListByTaskAfter(r.Context(), taskID, lastEvt.CreatedAt, lastEvt.ID)
		}
	} else {
		// No Last-Event-ID: replay all events from DB.
		replayEvents, err = h.EventStore.ListByTask(r.Context(), taskID)
	}

	if err != nil {
		log.Printf("stream: replay events for task %s: %v", taskID, err)
		// Write error as SSE event and return.
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"could not load events\"}\n\n")
		flusher.Flush()
		return
	}

	// Write replayed events.
	for i := range replayEvents {
		writeSSEEvent(w, &replayEvents[i])
	}
	flusher.Flush()

	// Subscribe to live events.
	ch := h.Broker.Subscribe(taskID)
	defer h.Broker.Unsubscribe(taskID, ch)

	// Stream live events until the client disconnects.
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				// Channel was closed (unsubscribed).
				return
			}
			writeSSEEvent(w, evt)
			flusher.Flush()
		}
	}
}

// writeSSEEvent writes a single event in SSE format:
//
//	id: <event_id>
//	data: <json>
//
// (blank line terminates the message)
func writeSSEEvent(w http.ResponseWriter, evt *models.Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		log.Printf("stream: marshal event %s: %v", evt.ID, err)
		return
	}
	fmt.Fprintf(w, "id: %s\ndata: %s\n\n", evt.ID, data)
}
