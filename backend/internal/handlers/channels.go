package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"taskhub/internal/models"
	"taskhub/internal/sse"
)

type ChannelHandler struct {
	DB     *sql.DB
	Broker *sse.Broker
}

type ChannelDetail struct {
	Channel  models.Channel        `json:"channel"`
	Messages []models.Message      `json:"messages"`
	Agents   []models.ChannelAgent `json:"agents"`
}

func (h *ChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var ch models.Channel
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, task_id, document, status, created_at FROM channels WHERE id = $1`, id,
	).Scan(&ch.ID, &ch.TaskID, &ch.Document, &ch.Status, &ch.CreatedAt)
	if err == sql.ErrNoRows {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, channel_id, sender_id, sender_name, content, type, created_at FROM messages WHERE channel_id = $1 ORDER BY created_at`, id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	messages := []models.Message{}
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.SenderID, &m.SenderName, &m.Content, &m.Type, &m.CreatedAt); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	agentRows, err := h.DB.QueryContext(r.Context(),
		`SELECT channel_id, agent_id, status FROM channel_agents WHERE channel_id = $1`, id)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer agentRows.Close()

	agents := []models.ChannelAgent{}
	for agentRows.Next() {
		var ca models.ChannelAgent
		if err := agentRows.Scan(&ca.ChannelID, &ca.AgentID, &ca.Status); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		agents = append(agents, ca)
	}
	if err := agentRows.Err(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, ChannelDetail{Channel: ch, Messages: messages, Agents: agents})
}

func (h *ChannelHandler) Stream(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	ch := h.Broker.Subscribe(id)
	defer h.Broker.Unsubscribe(id, ch)

	// Send current snapshot as catch-up events for clients connecting mid-task
	h.sendCatchup(w, flusher, r, id)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("data: " + event + "\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *ChannelHandler) sendCatchup(w http.ResponseWriter, flusher http.Flusher, r *http.Request, channelID string) {
	// Fetch current channel
	var ch models.Channel
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, task_id, document, status, created_at FROM channels WHERE id = $1`, channelID,
	).Scan(&ch.ID, &ch.TaskID, &ch.Document, &ch.Status, &ch.CreatedAt)
	if err != nil {
		return
	}

	// Send channel state
	sendSSE(w, flusher, "channel_created", map[string]any{"channel_id": channelID})
	if ch.Document != "" {
		sendSSE(w, flusher, "document_updated", map[string]any{"document": ch.Document})
	}

	// Fetch and send channel agents
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT ca.agent_id, a.name, ca.status FROM channel_agents ca JOIN agents a ON a.id = ca.agent_id WHERE ca.channel_id = $1`, channelID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var agentID, agentName, status string
		if err := rows.Scan(&agentID, &agentName, &status); err != nil {
			continue
		}
		sendSSE(w, flusher, "agent_joined", map[string]any{"agent_id": agentID, "agent_name": agentName})
		if status == "working" {
			sendSSE(w, flusher, "agent_working", map[string]any{"agent_id": agentID, "agent_name": agentName})
		} else if status == "done" {
			sendSSE(w, flusher, "agent_done", map[string]any{"agent_id": agentID})
		}
	}

	// Fetch and send messages
	msgRows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, channel_id, sender_id, sender_name, content, type, created_at FROM messages WHERE channel_id = $1 ORDER BY created_at`, channelID)
	if err != nil {
		return
	}
	defer msgRows.Close()
	for msgRows.Next() {
		var msg models.Message
		if err := msgRows.Scan(&msg.ID, &msg.ChannelID, &msg.SenderID, &msg.SenderName, &msg.Content, &msg.Type, &msg.CreatedAt); err != nil {
			continue
		}
		sendSSE(w, flusher, "message", map[string]any{"message": msg})
	}
}

// sendSSE writes a single SSE event and flushes
func sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data any) {
	payload := map[string]any{"type": eventType, "data": data}
	msg, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("data: " + string(msg) + "\n\n"))
	flusher.Flush()
}
