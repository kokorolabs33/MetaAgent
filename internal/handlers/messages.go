package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
	"taskhub/internal/events"
	"taskhub/internal/executor"
	"taskhub/internal/models"
)

// MessageHandler provides HTTP handlers for group chat messages.
type MessageHandler struct {
	DB         *pgxpool.Pool
	Executor   *executor.DAGExecutor
	EventStore *events.Store
	Broker     *events.Broker
}

// mentionRe matches <@agent_id|Display Name> mentions in message content.
var mentionRe = regexp.MustCompile(`<@([^|]+)\|[^>]+>`)

// sendMessageRequest is the expected body for POST /tasks/{id}/messages.
type sendMessageRequest struct {
	Content string `json:"content"`
}

// List handles GET /tasks/{id}/messages.
// Returns all messages for a task, ordered by created_at.
func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	rows, err := h.DB.Query(r.Context(),
		`SELECT id, task_id, sender_type, COALESCE(sender_id, ''), sender_name,
			content, mentions, metadata, created_at
		 FROM messages
		 WHERE task_id = $1
		 ORDER BY created_at`, taskID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	messages := make([]models.Message, 0)
	for rows.Next() {
		var m models.Message
		var metadata []byte

		err := rows.Scan(
			&m.ID, &m.TaskID, &m.SenderType, &m.SenderID, &m.SenderName,
			&m.Content, &m.Mentions, &metadata, &m.CreatedAt,
		)
		if err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}

		if metadata != nil {
			m.Metadata = json.RawMessage(metadata)
		}
		if m.Mentions == nil {
			m.Mentions = []string{}
		}

		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	jsonOK(w, messages)
}

// Send handles POST /tasks/{id}/messages.
// Creates a message, parses @mentions, and routes to agents via A2A follow-up if applicable.
func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	var req sendMessageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		jsonError(w, "content is required", http.StatusBadRequest)
		return
	}

	user := ctxutil.UserFromCtx(r.Context())

	// Extract agent IDs from <@id|name> mentions.
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	mentions := make([]string, 0, len(matches))
	for _, m := range matches {
		mentions = append(mentions, m[1]) // m[1] is the agent ID
	}

	now := time.Now().UTC()
	msg := models.Message{
		ID:         uuid.New().String(),
		TaskID:     taskID,
		SenderType: "user",
		SenderID:   user.ID,
		SenderName: user.Name,
		Content:    content,
		Mentions:   mentions,
		CreatedAt:  now,
	}

	// Insert message into DB.
	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO messages (id, task_id, sender_type, sender_id, sender_name, content, mentions, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		msg.ID, msg.TaskID, msg.SenderType, msg.SenderID, msg.SenderName, msg.Content, msg.Mentions, msg.CreatedAt)
	if err != nil {
		jsonError(w, "could not save message", http.StatusInternalServerError)
		return
	}

	// Publish message event.
	evt, _ := h.EventStore.Save(r.Context(), taskID, "", "message", "user", user.ID, map[string]any{
		"message_id":  msg.ID,
		"sender_name": msg.SenderName,
		"content":     msg.Content,
		"mentions":    msg.Mentions,
	})
	if evt != nil {
		h.Broker.Publish(evt)
	}

	// Check if any mention matches an agent with a running or input_required subtask.
	if len(mentions) > 0 {
		h.routeToAgents(r.Context(), taskID, mentions, content)
	}

	jsonCreated(w, msg)
}

// routeToAgents routes messages to agents by their IDs extracted from <@id|name> mentions.
func (h *MessageHandler) routeToAgents(ctx context.Context, taskID string, mentions []string, content string) {
	for _, agentID := range mentions {
		// Find active subtask for this agent in this task
		var subtaskID string
		err := h.DB.QueryRow(ctx,
			`SELECT s.id
			 FROM subtasks s
			 WHERE s.task_id = $1 AND s.agent_id = $2 AND s.status IN ('running', 'input_required')
			 ORDER BY s.created_at DESC
			 LIMIT 1`, taskID, agentID).
			Scan(&subtaskID)
		if err != nil {
			continue
		}

		go func(sid, aid string) {
			// Use background context — the HTTP request context will be canceled
			// after the response is sent, but the A2A follow-up call takes time.
			bgCtx := context.Background()
			if err := h.Executor.SendFollowUp(bgCtx, taskID, sid, aid, fmt.Sprintf("User message: %s", content)); err != nil {
				log.Printf("routeToAgents: follow-up to agent %s failed: %v", aid, err)
			}
		}(subtaskID, agentID)
	}
}
