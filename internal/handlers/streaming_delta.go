package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/events"
	"taskhub/internal/models"
)

// StreamingDeltaHandler receives streaming token deltas from agents and relays
// them as ephemeral events via the SSE broker. Deltas are NOT persisted to the
// event store -- only the final assembled message is stored by the executor.
type StreamingDeltaHandler struct {
	DB     *pgxpool.Pool
	Broker *events.Broker
}

// deltaRequest is the JSON body for POST /api/internal/streaming-delta.
type deltaRequest struct {
	TaskID    string `json:"task_id"`
	SubtaskID string `json:"subtask_id"`
	AgentID   string `json:"agent_id"`
	DeltaText string `json:"delta_text"`
	Done      bool   `json:"done"`
}

// HandleDelta receives a streaming delta from an agent and publishes it as an
// ephemeral "agent.streaming_delta" event via the broker.
//
// POST /api/internal/streaming-delta
func (h *StreamingDeltaHandler) HandleDelta(w http.ResponseWriter, r *http.Request) {
	var req deltaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.TaskID == "" {
		jsonError(w, "task_id is required", http.StatusBadRequest)
		return
	}
	if req.AgentID == "" {
		jsonError(w, "agent_id is required", http.StatusBadRequest)
		return
	}

	// Look up conversation_id for the task so event routes to conversation
	// subscribers too (same pattern as executor.publishTransientEvent).
	var conversationID string
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := h.DB.QueryRow(ctx,
		`SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`,
		req.TaskID).Scan(&conversationID)
	if err != nil {
		log.Printf("streaming-delta: lookup conversation for task %s: %v", req.TaskID, err)
		// Continue anyway -- the event will still route by task_id.
	}

	// Build event data.
	dataJSON, err := json.Marshal(map[string]any{
		"delta_text": req.DeltaText,
		"done":       req.Done,
		"agent_id":   req.AgentID,
	})
	if err != nil {
		log.Printf("streaming-delta: marshal event data: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	evt := &models.Event{
		ID:             uuid.NewString(),
		TaskID:         req.TaskID,
		ConversationID: conversationID,
		SubtaskID:      req.SubtaskID,
		Type:           "agent.streaming_delta",
		ActorType:      "agent",
		ActorID:        req.AgentID,
		Data:           dataJSON,
		CreatedAt:      time.Now(),
	}

	// Publish via broker only -- per D-02, do NOT save to EventStore.
	h.Broker.Publish(evt)

	w.WriteHeader(http.StatusNoContent)
}
