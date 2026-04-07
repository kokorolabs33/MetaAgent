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

	"taskhub/internal/audit"
	"taskhub/internal/ctxutil"
	"taskhub/internal/events"
	"taskhub/internal/executor"
	"taskhub/internal/models"
	"taskhub/internal/orchestrator"
)

// ConversationHandler provides HTTP handlers for conversations.
type ConversationHandler struct {
	DB           *pgxpool.Pool
	Executor     *executor.DAGExecutor
	EventStore   *events.Store
	Broker       *events.Broker
	Orchestrator *orchestrator.Orchestrator
	Audit        *audit.Logger
}

// List handles GET /api/conversations.
// Returns conversations sorted by updated_at DESC, with agent_count and task_count.
func (h *ConversationHandler) List(w http.ResponseWriter, r *http.Request) {
	user := ctxutil.UserFromCtx(r.Context())

	rows, err := h.DB.Query(r.Context(),
		`SELECT c.id, c.title,
			COUNT(DISTINCT t.id) AS task_count,
			COUNT(DISTINCT s.agent_id) AS agent_count,
			COALESCE((
				SELECT t2.status FROM tasks t2
				WHERE t2.conversation_id = c.id
				ORDER BY t2.created_at DESC LIMIT 1
			), '') AS latest_status,
			c.updated_at
		 FROM conversations c
		 LEFT JOIN tasks t ON t.conversation_id = c.id
		 LEFT JOIN subtasks s ON s.task_id = t.id
		 WHERE c.created_by = $1
		 GROUP BY c.id
		 ORDER BY c.updated_at DESC`, user.ID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]models.ConversationListItem, 0)
	for rows.Next() {
		var item models.ConversationListItem
		var updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Title, &item.TaskCount, &item.AgentCount, &item.LatestStatus, &updatedAt); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		item.UpdatedAt = updatedAt.Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	jsonOK(w, items)
}

// createConversationRequest is the expected body for POST /api/conversations.
type createConversationRequest struct {
	Title string `json:"title"`
}

// Create handles POST /api/conversations.
func (h *ConversationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createConversationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user := ctxutil.UserFromCtx(r.Context())
	now := time.Now().UTC()

	conv := models.Conversation{
		ID:        uuid.New().String(),
		Title:     strings.TrimSpace(req.Title),
		CreatedBy: user.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO conversations (id, title, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		conv.ID, conv.Title, conv.CreatedBy, conv.CreatedAt, conv.UpdatedAt)
	if err != nil {
		jsonError(w, "could not create conversation", http.StatusInternalServerError)
		return
	}

	jsonCreated(w, conv)
}

// Get handles GET /api/conversations/{id}.
func (h *ConversationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var conv models.Conversation
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, title, created_by, created_at, updated_at
		 FROM conversations WHERE id = $1`, id).
		Scan(&conv.ID, &conv.Title, &conv.CreatedBy, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		jsonError(w, "conversation not found", http.StatusNotFound)
		return
	}

	jsonOK(w, conv)
}

// updateConversationRequest is the expected body for PUT /api/conversations/{id}.
type updateConversationRequest struct {
	Title string `json:"title"`
}

// Update handles PUT /api/conversations/{id}.
func (h *ConversationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateConversationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	tag, err := h.DB.Exec(r.Context(),
		`UPDATE conversations SET title = $1, updated_at = $2 WHERE id = $3`,
		title, now, id)
	if err != nil {
		jsonError(w, "could not update conversation", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "conversation not found", http.StatusNotFound)
		return
	}

	var conv models.Conversation
	err = h.DB.QueryRow(r.Context(),
		`SELECT id, title, created_by, created_at, updated_at
		 FROM conversations WHERE id = $1`, id).
		Scan(&conv.ID, &conv.Title, &conv.CreatedBy, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		jsonError(w, "could not read updated conversation", http.StatusInternalServerError)
		return
	}

	jsonOK(w, conv)
}

// Delete handles DELETE /api/conversations/{id}.
// Cascade deletes tasks, messages, and events.
func (h *ConversationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Delete in dependency order: events, subtasks (via tasks), messages, tasks, conversation.
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonError(w, "could not start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Delete events for this conversation
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM events WHERE conversation_id = $1`, id); err != nil {
		jsonError(w, "could not delete events", http.StatusInternalServerError)
		return
	}

	// Delete subtasks for tasks in this conversation
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM subtasks WHERE task_id IN (SELECT id FROM tasks WHERE conversation_id = $1)`, id); err != nil {
		jsonError(w, "could not delete subtasks", http.StatusInternalServerError)
		return
	}

	// Delete messages
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM messages WHERE conversation_id = $1`, id); err != nil {
		jsonError(w, "could not delete messages", http.StatusInternalServerError)
		return
	}

	// Delete tasks
	if _, err := tx.Exec(r.Context(),
		`DELETE FROM tasks WHERE conversation_id = $1`, id); err != nil {
		jsonError(w, "could not delete tasks", http.StatusInternalServerError)
		return
	}

	// Delete conversation
	tag, err := tx.Exec(r.Context(),
		`DELETE FROM conversations WHERE id = $1`, id)
	if err != nil {
		jsonError(w, "could not delete conversation", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "conversation not found", http.StatusNotFound)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		jsonError(w, "could not commit delete", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"status": "deleted"})
}

// GetMessages handles GET /api/conversations/{id}/messages.
func (h *ConversationHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")

	rows, err := h.DB.Query(r.Context(),
		`SELECT id, COALESCE(task_id, ''), conversation_id, sender_type, COALESCE(sender_id, ''), sender_name,
			content, mentions, metadata, created_at
		 FROM messages
		 WHERE conversation_id = $1
		 ORDER BY created_at`, convID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	messages := make([]models.Message, 0)
	for rows.Next() {
		var m models.Message
		var metadata []byte
		if err := rows.Scan(
			&m.ID, &m.TaskID, &m.ConversationID, &m.SenderType, &m.SenderID, &m.SenderName,
			&m.Content, &m.Mentions, &metadata, &m.CreatedAt,
		); err != nil {
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

// sendConversationMessageRequest is the expected body for POST /api/conversations/{id}/messages.
type sendConversationMessageRequest struct {
	Content string `json:"content"`
}

// SendMessage handles POST /api/conversations/{id}/messages.
// Saves the user message, then asynchronously calls the orchestrator to detect intent.
func (h *ConversationHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	user := ctxutil.UserFromCtx(r.Context())

	var req sendConversationMessageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		jsonError(w, "content is required", http.StatusBadRequest)
		return
	}

	// Verify conversation exists
	var exists bool
	if err := h.DB.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM conversations WHERE id = $1)`, convID).Scan(&exists); err != nil || !exists {
		jsonError(w, "conversation not found", http.StatusNotFound)
		return
	}

	// Extract mentions (agent IDs) from <@id|name> format
	mentionRe := regexp.MustCompile(`<@([^|]+)\|[^>]+>`)
	mentionMatches := mentionRe.FindAllStringSubmatch(content, -1)
	mentions := make([]string, 0, len(mentionMatches))
	for _, m := range mentionMatches {
		mentions = append(mentions, m[1])
	}

	now := time.Now().UTC()
	msg := models.Message{
		ID:             uuid.New().String(),
		ConversationID: convID,
		SenderType:     "user",
		SenderID:       user.ID,
		SenderName:     user.Name,
		Content:        content,
		Mentions:       mentions,
		CreatedAt:      now,
	}

	// Insert user message
	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO messages (id, conversation_id, sender_type, sender_id, sender_name, content, mentions, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		msg.ID, msg.ConversationID, msg.SenderType, msg.SenderID, msg.SenderName, msg.Content, msg.Mentions, msg.CreatedAt)
	if err != nil {
		jsonError(w, "could not save message", http.StatusInternalServerError)
		return
	}

	// Publish user message event
	evt, _ := h.EventStore.SaveWithConversation(r.Context(), "", convID, "", "message", "user", user.ID, map[string]any{
		"message_id":  msg.ID,
		"sender_name": msg.SenderName,
		"content":     msg.Content,
	})
	if evt != nil {
		h.Broker.Publish(evt)
	}

	// Touch conversation updated_at
	_, _ = h.DB.Exec(r.Context(),
		`UPDATE conversations SET updated_at = $1 WHERE id = $2`, now, convID)

	// Handle orchestrator in background goroutine
	go func() { //nolint:contextcheck // context.Background is intentional as orchestrator outlives the HTTP request
		ctx := context.Background()
		h.handleOrchestrator(ctx, convID, user, content)
	}()

	jsonCreated(w, msg)
}

// handleOrchestrator loads history, calls intent detection, and acts on the result.
func (h *ConversationHandler) handleOrchestrator(ctx context.Context, convID string, user *models.User, content string) {
	// Check for @mentions — route directly to agents if possible
	mentionRe := regexp.MustCompile(`<@([^|]+)\|[^>]+>`)
	matches := mentionRe.FindAllStringSubmatch(content, -1)
	if len(matches) > 0 {
		agentIDs := make([]string, 0, len(matches))
		for _, m := range matches {
			agentIDs = append(agentIDs, m[1]) // m[1] is the agent ID
		}

		// Find tasks in this conversation that have running/input_required subtasks
		routed := h.routeMentionsToAgentsByID(ctx, convID, agentIDs, content)
		if routed {
			return // message was routed to an agent, no need for orchestrator
		}
	}

	// Load conversation history (last 50 messages)
	history, err := h.loadConversationHistory(ctx, convID)
	if err != nil {
		log.Printf("conversation: load history: %v", err)
		h.saveSystemMessage(ctx, convID, "Sorry, I encountered an error processing your request.")
		return
	}

	// Load active agents
	agents, err := h.loadActiveAgents(ctx)
	if err != nil {
		log.Printf("conversation: load agents: %v", err)
		h.saveSystemMessage(ctx, convID, "Sorry, I could not load available agents.")
		return
	}

	// Detect intent
	intent, err := h.Orchestrator.DetectIntent(ctx, history, content, agents)
	if err != nil {
		log.Printf("conversation: detect intent: %v", err)
		h.saveSystemMessage(ctx, convID, "Sorry, I encountered an error processing your request.")
		return
	}

	if intent.Type == "chat" {
		// Save orchestrator's conversational response as system message
		h.saveSystemMessage(ctx, convID, intent.Content)
	} else {
		// Create task within this conversation and execute
		h.createAndExecuteTask(ctx, convID, intent.Title, intent.Description, user.ID)
	}
}

// loadConversationHistory returns recent messages formatted for the orchestrator.
func (h *ConversationHandler) loadConversationHistory(ctx context.Context, convID string) ([]orchestrator.ChatMessage, error) {
	rows, err := h.DB.Query(ctx,
		`SELECT sender_type, sender_name, content
		 FROM messages
		 WHERE conversation_id = $1
		 ORDER BY created_at DESC
		 LIMIT 50`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []orchestrator.ChatMessage
	for rows.Next() {
		var cm orchestrator.ChatMessage
		if err := rows.Scan(&cm.Role, &cm.Name, &cm.Content); err != nil {
			return nil, err
		}
		msgs = append(msgs, cm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse so oldest first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}

// loadActiveAgents returns all online agents.
func (h *ConversationHandler) loadActiveAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := h.DB.Query(ctx,
		`SELECT id, name, description, capabilities
		 FROM agents
		 WHERE status = 'active'
		 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Capabilities); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// saveSystemMessage inserts a system message into the conversation and publishes an SSE event.
func (h *ConversationHandler) saveSystemMessage(ctx context.Context, convID, content string) {
	msgID := uuid.New().String()
	now := time.Now().UTC()

	_, err := h.DB.Exec(ctx,
		`INSERT INTO messages (id, task_id, conversation_id, sender_type, sender_id, sender_name, content, mentions, created_at)
		 VALUES ($1, '', $2, 'system', '', 'System', $3, '{}', $4)`,
		msgID, convID, content, now)
	if err != nil {
		log.Printf("conversation: insert system message: %v", err)
		return
	}

	evt, _ := h.EventStore.SaveWithConversation(ctx, "", convID, "", "message", "system", "", map[string]any{
		"message_id":  msgID,
		"sender_type": "system",
		"sender_name": "System",
		"content":     content,
	})
	if evt != nil {
		h.Broker.Publish(evt)
	}
}

// createAndExecuteTask creates a task within a conversation and starts execution.
func (h *ConversationHandler) createAndExecuteTask(ctx context.Context, convID, title, description, userID string) {
	now := time.Now().UTC()
	task := models.Task{
		ID:             uuid.New().String(),
		ConversationID: convID,
		Title:          title,
		Description:    description,
		Status:         "pending",
		CreatedBy:      userID,
		ReplanCount:    0,
		CreatedAt:      now,
	}

	_, err := h.DB.Exec(ctx,
		`INSERT INTO tasks (id, conversation_id, title, description, status, created_by, replan_count, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		task.ID, task.ConversationID, task.Title, task.Description, task.Status, task.CreatedBy, task.ReplanCount, task.CreatedAt)
	if err != nil {
		log.Printf("conversation: create task: %v", err)
		h.saveSystemMessage(ctx, convID, "Sorry, I could not create the task.")
		return
	}

	// Publish task_created event to conversation stream
	evt, _ := h.EventStore.SaveWithConversation(ctx, task.ID, convID, "", "task_created", "system", "", map[string]any{
		"task_id":     task.ID,
		"title":       task.Title,
		"description": task.Description,
	})
	if evt != nil {
		h.Broker.Publish(evt)
	}

	// Inform the conversation
	h.saveSystemMessage(ctx, convID, fmt.Sprintf("Creating task: **%s**\n\n%s", task.Title, task.Description))

	// Execute the task (existing DAG executor flow)
	if err := h.Executor.Execute(ctx, task); err != nil {
		log.Printf("conversation: execute task %s: %v", task.ID, err)
	}
}

// ListTasks handles GET /api/conversations/{id}/tasks.
func (h *ConversationHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")

	rows, err := h.DB.Query(r.Context(),
		`SELECT id, title, description, status, created_by,
			metadata, plan, result, error, replan_count, created_at, completed_at
		 FROM tasks
		 WHERE conversation_id = $1
		 ORDER BY created_at`, convID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := make([]models.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows.Scan)
		if err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		t.ConversationID = convID
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	jsonOK(w, tasks)
}

// Stream handles GET /api/conversations/{id}/events.
// Same pattern as task-level SSE but filters by conversation_id.
func (h *ConversationHandler) Stream(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Determine replay strategy based on Last-Event-ID
	lastEventID := r.Header.Get("Last-Event-ID")

	var replayEvents []models.Event
	var err error

	if lastEventID != "" {
		lastEvt, lookupErr := h.EventStore.GetByID(r.Context(), lastEventID)
		if lookupErr != nil {
			replayEvents, err = h.EventStore.ListByConversation(r.Context(), convID)
		} else {
			replayEvents, err = h.EventStore.ListByConversationAfter(r.Context(), convID, lastEvt.CreatedAt, lastEvt.ID)
		}
	} else {
		replayEvents, err = h.EventStore.ListByConversation(r.Context(), convID)
	}

	if err != nil {
		log.Printf("stream: replay events for conversation %s: %v", convID, err)
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"could not load events\"}\n\n")
		flusher.Flush()
		return
	}

	// Write replayed events
	for i := range replayEvents {
		writeSSEEvent(w, &replayEvents[i])
	}
	flusher.Flush()

	// Subscribe to live conversation events
	ch := h.Broker.SubscribeConversation(convID)
	defer h.Broker.UnsubscribeConversation(convID, ch)

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

// routeMentionsToAgentsByID routes messages to agents by their IDs extracted from <@id|name> mentions.
func (h *ConversationHandler) routeMentionsToAgentsByID(ctx context.Context, convID string, agentIDs []string, content string) bool {
	for _, agentID := range agentIDs {
		// Find active subtask for this agent in this conversation
		var subtaskID, taskID string
		err := h.DB.QueryRow(ctx,
			`SELECT s.id, s.task_id
			 FROM subtasks s
			 JOIN tasks t ON t.id = s.task_id
			 WHERE t.conversation_id = $1 AND s.agent_id = $2
			 ORDER BY s.created_at DESC
			 LIMIT 1`, convID, agentID).
			Scan(&subtaskID, &taskID)
		if err != nil {
			continue
		}

		// Route via executor
		go func(sid, tid, aid string) {
			bgCtx := context.Background()
			if err := h.Executor.SendFollowUp(bgCtx, tid, sid, aid, fmt.Sprintf("User message: %s", content)); err != nil {
				log.Printf("conversation: follow-up to agent %s failed: %v", aid, err)
			}
		}(subtaskID, taskID, agentID)
		return true
	}

	return false
}
