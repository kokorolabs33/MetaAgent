package handlers

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"taskhub/internal/models"
)

type ChannelHandler struct {
	DB *sql.DB
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
