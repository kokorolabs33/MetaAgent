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

// mentionRe matches @mentions in message content.
var mentionRe = regexp.MustCompile(`@(\S+)`)

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

	// Extract @mentions from content.
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	mentions := make([]string, 0, len(matches))
	for _, m := range matches {
		mentions = append(mentions, m[1])
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

// routeToAgents checks if any mentioned agent names have active subtasks
// and sends a follow-up A2A message via the executor.
func (h *MessageHandler) routeToAgents(ctx context.Context, taskID string, mentions []string, content string) {
	// Query subtasks that are running or input_required along with their agent names.
	rows, err := h.DB.Query(ctx,
		`SELECT s.id, s.agent_id, a.name
		 FROM subtasks s
		 JOIN agents a ON a.id = s.agent_id
		 WHERE s.task_id = $1 AND s.status IN ('running', 'input_required')`, taskID)
	if err != nil {
		return
	}
	defer rows.Close()

	type activeSubtask struct {
		subtaskID string
		agentID   string
		agentName string
	}

	var active []activeSubtask
	for rows.Next() {
		var as activeSubtask
		if err := rows.Scan(&as.subtaskID, &as.agentID, &as.agentName); err != nil {
			continue
		}
		active = append(active, as)
	}
	if rows.Err() != nil {
		return
	}

	// Match mentions against active agent names.
	// Supports partial match: @Finance matches "Finance Review Agent"
	// because the regex only captures the first word after @.
	for _, as := range active {
		agentLower := strings.ToLower(as.agentName)
		for _, m := range mentions {
			mentionLower := strings.ToLower(m)
			// Exact match or agent name starts with mention
			if agentLower == mentionLower || strings.HasPrefix(agentLower, mentionLower+" ") || strings.Contains(agentLower, mentionLower) {
				go func(sub activeSubtask) {
					if err := h.Executor.SendFollowUp(ctx, taskID, sub.subtaskID, sub.agentID, fmt.Sprintf("User message: %s", content)); err != nil {
						log.Printf("routeToAgents: follow-up to agent %s failed: %v", sub.agentName, err)
					}
				}(as)
				break
			}
		}
	}
}
