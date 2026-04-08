package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

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
	w.Header().Set("X-Accel-Buffering", "no")

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

// maxMultiStreamIDs caps how many task IDs a single multiplexed SSE request
// may subscribe to. Sized to comfortably exceed the dashboard UI's 12-per-page
// grid (per CONTEXT.md D-05) while bounding per-request memory (see T-04-02).
const maxMultiStreamIDs = 50

// MultiStream handles GET /api/tasks/stream?ids=a,b,c.
//
// It subscribes to each task's broker channel and fans all events out on a
// single HTTP response writer, so the browser never opens more than one
// task-stream connection regardless of how many cards the dashboard is
// showing. Combined with /api/agents/stream that keeps the dashboard under
// the 6-connections-per-domain browser cap.
//
// No replay: the dashboard's initial LIST API call provides the snapshot;
// this stream only delivers live events after connect.
func (h *StreamHandler) MultiStream(w http.ResponseWriter, r *http.Request) {
	idsParam := strings.TrimSpace(r.URL.Query().Get("ids"))
	if idsParam == "" {
		jsonError(w, "ids query param required", http.StatusBadRequest)
		return
	}

	// Parse, trim, deduplicate.
	raw := strings.Split(idsParam, ",")
	seen := make(map[string]struct{}, len(raw))
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		jsonError(w, "ids query param required", http.StatusBadRequest)
		return
	}
	if len(ids) > maxMultiStreamIDs {
		jsonError(w, "too many ids (max 50)", http.StatusBadRequest)
		return
	}

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to every task BEFORE any other work (subscribe-before-replay
	// pattern from Phase 1 D-01). No replay is needed for the dashboard —
	// the LIST API is the snapshot.
	type sub struct {
		taskID string
		ch     chan *models.Event
	}
	subs := make([]sub, 0, len(ids))
	for _, id := range ids {
		subs = append(subs, sub{taskID: id, ch: h.Broker.Subscribe(id)})
	}
	defer func() {
		for _, s := range subs {
			h.Broker.Unsubscribe(s.taskID, s.ch)
		}
	}()

	// Fan-in: one goroutine per subscription drains its channel into a
	// merged channel. Only the main loop writes to w, so SSE frames are
	// never interleaved by concurrent goroutines (net/http requires serial
	// writes).
	ctx := r.Context()
	merged := make(chan *models.Event, 64)
	var wg sync.WaitGroup
	for _, s := range subs {
		wg.Add(1)
		go func(s sub) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-s.ch:
					if !ok {
						return
					}
					select {
					case merged <- evt:
					case <-ctx.Done():
						return
					}
				}
			}
		}(s)
	}
	// Close merged once all drainers exit (after ctx done + Unsubscribe
	// closes their source channels).
	go func() {
		wg.Wait()
		close(merged)
	}()

	// Flush headers so the EventSource opens immediately.
	flusher.Flush()

	// Single writer loop.
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-merged:
			if !ok {
				return
			}
			writeSSEEvent(w, evt)
			flusher.Flush()
		}
	}
}
