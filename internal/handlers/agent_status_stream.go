package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/events"
	"taskhub/internal/models"
)

// AgentStatusStreamHandler provides SSE streaming for agent status events.
type AgentStatusStreamHandler struct {
	Broker *events.Broker
	DB     *pgxpool.Pool
}

// Stream handles GET /api/agents/stream.
// On connect, it replays current agent statuses as a snapshot,
// then streams live agent.status_changed events via SSE.
// Follows subscribe-before-replay pattern from Phase 1 D-01/D-02.
func (h *AgentStatusStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe BEFORE replay (subscribe-before-replay pattern, per D-03).
	ch := h.Broker.SubscribeGlobal("agents")
	defer h.Broker.UnsubscribeGlobal("agents", ch)

	// Replay: query current agent statuses from DB and send as snapshot events.
	// This gives new subscribers an immediate view of all agent states.
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, is_online, last_health_check FROM agents WHERE status = 'active'`)
	if err != nil {
		log.Printf("agent-status-stream: query agents: %v", err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"could not load agents\"}\n\n")
		flusher.Flush()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var agent models.Agent
		if err := rows.Scan(&agent.ID, &agent.Name, &agent.IsOnline, &agent.LastHealthCheck); err != nil {
			log.Printf("agent-status-stream: scan agent: %v", err)
			continue
		}
		// Derive initial activity_status from is_online.
		activityStatus := "offline"
		if agent.IsOnline {
			activityStatus = "online"
		}
		// Check staleness: if last_health_check is nil or older than 5 minutes, mark "unknown".
		if agent.LastHealthCheck == nil {
			activityStatus = "unknown"
		} else if time.Since(*agent.LastHealthCheck) > 5*time.Minute {
			activityStatus = "unknown"
		}

		dataJSON, _ := json.Marshal(map[string]any{
			"agent_id":        agent.ID,
			"agent_name":      agent.Name,
			"activity_status": activityStatus,
			"is_online":       agent.IsOnline,
			"snapshot":        true,
		})
		evt := &models.Event{
			ID:        uuid.NewString(),
			Type:      "agent.status_changed",
			ActorType: "system",
			Data:      dataJSON,
			CreatedAt: time.Now(),
		}
		writeSSEEvent(w, evt)
	}
	flusher.Flush()

	// Stream live events until client disconnects.
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			writeSSEEvent(w, evt)
			flusher.Flush()
		}
	}
}
