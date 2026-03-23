package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/adapter"
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
// Creates a message, parses @mentions, and signals waiting subtasks if applicable.
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

	// Check if any mention matches an agent with a waiting subtask.
	// For each mentioned name, look up whether there's a waiting_for_input subtask
	// assigned to an agent with that name.
	if len(mentions) > 0 {
		h.signalWaitingAgents(r.Context(), taskID, mentions, content)
	}

	jsonCreated(w, msg)
}

// signalWaitingAgents checks if any mentioned agent names have waiting subtasks
// and delivers the user input via the executor's Signal method.
func (h *MessageHandler) signalWaitingAgents(ctx context.Context, taskID string, mentions []string, content string) {
	// Query subtasks waiting for input along with their agent names.
	rows, err := h.DB.Query(ctx,
		`SELECT s.id, s.agent_id, a.name
		 FROM subtasks s
		 JOIN agents a ON a.id = s.agent_id
		 WHERE s.task_id = $1 AND s.status = 'waiting_for_input'`, taskID)
	if err != nil {
		return
	}
	defer rows.Close()

	type waitingSubtask struct {
		subtaskID string
		agentID   string
		agentName string
	}

	var waiting []waitingSubtask
	for rows.Next() {
		var ws waitingSubtask
		if err := rows.Scan(&ws.subtaskID, &ws.agentID, &ws.agentName); err != nil {
			continue
		}
		waiting = append(waiting, ws)
	}
	if rows.Err() != nil {
		return
	}

	// Match mentions against waiting agent names (case-insensitive).
	mentionSet := make(map[string]bool, len(mentions))
	for _, m := range mentions {
		mentionSet[strings.ToLower(m)] = true
	}

	for _, ws := range waiting {
		if mentionSet[strings.ToLower(ws.agentName)] {
			_ = h.Executor.Signal(ctx, taskID, adapter.UserInput{
				SubtaskID: ws.subtaskID,
				Message:   fmt.Sprintf("User message: %s", content),
			})
		}
	}
}
