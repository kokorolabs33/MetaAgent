package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"taskhub/internal/models"
)

type TaskHandler struct {
	DB *sql.DB
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, title, description, status, created_at, completed_at FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.CompletedAt); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, tasks)
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t models.Task
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id, title, description, status, created_at, completed_at FROM tasks WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.CompletedAt)
	if err == sql.ErrNoRows {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, t)
}

func (h *TaskHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	var channelID string
	err := h.DB.QueryRowContext(r.Context(),
		`SELECT id FROM channels WHERE task_id = $1 ORDER BY created_at DESC LIMIT 1`, taskID,
	).Scan(&channelID)
	if err == sql.ErrNoRows {
		jsonError(w, "no channel yet", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"channel_id": channelID})
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Description == "" {
		jsonError(w, "description required", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		req.Title = req.Description
		if len([]rune(req.Title)) > 80 {
			req.Title = string([]rune(req.Title)[:80]) + "..."
		}
	}

	t := models.Task{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO tasks (id, title, description, status) VALUES ($1,$2,$3,$4)`,
		t.ID, t.Title, t.Description, t.Status,
	)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Master agent will be triggered by main.go after this handler returns
	// For now just return the task — the Master field will be wired in a later task
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, t)
}
