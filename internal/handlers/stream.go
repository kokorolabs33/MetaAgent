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
// It subscribes to live events FIRST, then replays historical events from the
// database, deduplicating any events that arrived on the live channel during
// the replay window. This eliminates the race condition where events published
// between replay and subscribe would be lost. Supports Last-Event-ID for
// reconnection catchup.
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

	// 1. Subscribe to live events FIRST to avoid race condition.
	//    The channel buffers up to 64 events (per broker.go) while we replay.
	ch := h.Broker.Subscribe(taskID)
	defer h.Broker.Unsubscribe(taskID, ch)

	// 2. Replay historical events from the database.
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

	// 3. Write replayed events, tracking IDs for dedup against live channel.
	seen := make(map[string]struct{}, len(replayEvents))
	for i := range replayEvents {
		writeSSEEvent(w, &replayEvents[i])
		seen[replayEvents[i].ID] = struct{}{}
	}
	flusher.Flush()

	// 4. Stream live events with dedup. Events that were both persisted (replayed)
	//    and published to the live channel during the replay window are skipped.
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
			if _, dup := seen[evt.ID]; dup {
				delete(seen, evt.ID) // Free memory after dedup hit
				continue
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
