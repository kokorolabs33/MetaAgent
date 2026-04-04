package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditLogHandler provides HTTP handlers for querying the events log.
type AuditLogHandler struct {
	DB *pgxpool.Pool
}

// List handles GET /api/audit-logs.
// Supports filtering: ?task_id=, ?agent_id=, ?type=, ?page=, ?per_page=
func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	taskID := r.URL.Query().Get("task_id")
	agentID := r.URL.Query().Get("agent_id")
	eventType := r.URL.Query().Get("type")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	var conditions []string
	var args []any
	argN := 1

	if taskID != "" {
		conditions = append(conditions, fmt.Sprintf("task_id = $%d", argN))
		args = append(args, taskID)
		argN++
	}
	if agentID != "" {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argN))
		args = append(args, agentID)
		argN++
	}
	if eventType != "" {
		conditions = append(conditions, fmt.Sprintf("type ILIKE $%d", argN))
		args = append(args, "%"+eventType+"%")
		argN++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	err := h.DB.QueryRow(ctx, "SELECT COUNT(*) FROM events "+where, args...).Scan(&total)
	if err != nil {
		jsonError(w, "count query failed", http.StatusInternalServerError)
		return
	}

	query := fmt.Sprintf(
		`SELECT id, task_id, COALESCE(subtask_id, ''), type, actor_type, actor_id, data, created_at
		 FROM events %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argN, argN+1)
	args = append(args, perPage, offset)

	rows, err := h.DB.Query(ctx, query, args...)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type auditEntry struct {
		ID        string          `json:"id"`
		TaskID    string          `json:"task_id"`
		SubtaskID string          `json:"subtask_id,omitempty"`
		Type      string          `json:"type"`
		ActorType string          `json:"actor_type"`
		ActorID   string          `json:"actor_id,omitempty"`
		Data      json.RawMessage `json:"data"`
		CreatedAt string          `json:"created_at"`
	}

	entries := make([]auditEntry, 0)
	for rows.Next() {
		var e auditEntry
		var data []byte
		if err := rows.Scan(&e.ID, &e.TaskID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &data, &e.CreatedAt); err != nil {
			continue
		}
		if data != nil {
			e.Data = json.RawMessage(data)
		}
		entries = append(entries, e)
	}

	jsonOK(w, map[string]any{
		"items":    entries,
		"total":    total,
		"page":     page,
		"per_page": perPage,
		"pages":    (total + perPage - 1) / perPage,
	})
}
