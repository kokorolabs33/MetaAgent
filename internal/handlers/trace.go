package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TraceHandler provides HTTP handlers for task event timelines.
type TraceHandler struct {
	DB *pgxpool.Pool
}

// timelineEvent is the JSON shape returned by GetTimeline.
type timelineEvent struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	SubtaskID string          `json:"subtask_id,omitempty"`
	Type      string          `json:"type"`
	ActorType string          `json:"actor_type"`
	ActorID   string          `json:"actor_id,omitempty"`
	Data      json.RawMessage `json:"data"`
	CreatedAt string          `json:"created_at"`
}

// GetTimeline handles GET /api/tasks/{id}/timeline.
// Returns all events for a task, formatted for timeline display.
func (h *TraceHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	rows, err := h.DB.Query(r.Context(),
		`SELECT id, task_id, subtask_id, type, actor_type, actor_id, data, created_at
		 FROM events
		 WHERE task_id = $1
		 ORDER BY created_at ASC`, taskID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	events := make([]timelineEvent, 0)
	for rows.Next() {
		var e timelineEvent
		var data []byte
		var createdAt time.Time
		if err := rows.Scan(&e.ID, &e.TaskID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &data, &createdAt); err != nil {
			continue
		}
		if data != nil {
			e.Data = json.RawMessage(data)
		}
		e.CreatedAt = createdAt.Format(time.RFC3339Nano)
		events = append(events, e)
	}

	jsonOK(w, events)
}
