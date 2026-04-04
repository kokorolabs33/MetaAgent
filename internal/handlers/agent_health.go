package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentHealthHandler provides HTTP handlers for agent health metrics.
type AgentHealthHandler struct {
	DB *pgxpool.Pool
}

// GetHealth handles GET /api/agents/{id}/health.
// Returns health metrics and task stats for a single agent.
func (h *AgentHealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	var name, status, skillHash string
	var isOnline bool
	var lastCheck *time.Time
	var endpoint string

	err := h.DB.QueryRow(ctx,
		`SELECT name, status, endpoint, is_online, last_health_check, skill_hash
		 FROM agents WHERE id = $1`, id).
		Scan(&name, &status, &endpoint, &isOnline, &lastCheck, &skillHash)
	if err != nil {
		jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	var totalSubtasks, completedSubtasks, failedSubtasks int
	err = h.DB.QueryRow(ctx,
		`SELECT COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)
		 FROM subtasks WHERE agent_id = $1`, id).
		Scan(&totalSubtasks, &completedSubtasks, &failedSubtasks)
	if err != nil {
		jsonError(w, "failed to query subtask stats", http.StatusInternalServerError)
		return
	}

	var avgDuration float64
	err = h.DB.QueryRow(ctx,
		`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0)
		 FROM subtasks
		 WHERE agent_id = $1
		   AND status = 'completed'
		   AND started_at IS NOT NULL
		   AND completed_at IS NOT NULL`, id).
		Scan(&avgDuration)
	if err != nil {
		jsonError(w, "failed to query duration stats", http.StatusInternalServerError)
		return
	}

	successRate := 0.0
	if totalSubtasks > 0 {
		successRate = float64(completedSubtasks) / float64(totalSubtasks)
	}

	jsonOK(w, map[string]any{
		"id":                id,
		"name":              name,
		"status":            status,
		"endpoint":          endpoint,
		"is_online":         isOnline,
		"last_health_check": lastCheck,
		"skill_hash":        skillHash,
		"total_subtasks":    totalSubtasks,
		"completed":         completedSubtasks,
		"failed":            failedSubtasks,
		"success_rate":      successRate,
		"avg_duration_sec":  avgDuration,
	})
}

// GetOverview handles GET /api/agents/health/overview.
// Returns health metrics for all active agents.
func (h *AgentHealthHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.DB.Query(ctx,
		`SELECT a.id, a.name, a.status, a.is_online, a.last_health_check,
			COUNT(s.id) AS total_tasks,
			COALESCE(SUM(CASE WHEN s.status = 'completed' THEN 1 ELSE 0 END), 0) AS completed,
			COALESCE(SUM(CASE WHEN s.status = 'failed' THEN 1 ELSE 0 END), 0) AS failed,
			COALESCE(AVG(EXTRACT(EPOCH FROM (s.completed_at - s.started_at)))
				FILTER (WHERE s.status = 'completed' AND s.started_at IS NOT NULL AND s.completed_at IS NOT NULL), 0) AS avg_dur
		 FROM agents a
		 LEFT JOIN subtasks s ON s.agent_id = a.id
		 WHERE a.status = 'active'
		 GROUP BY a.id, a.name, a.status, a.is_online, a.last_health_check
		 ORDER BY total_tasks DESC`)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	agents := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, status string
		var isOnline bool
		var lastCheck *time.Time
		var total, completed, failed int
		var avgDur float64

		if err := rows.Scan(&id, &name, &status, &isOnline, &lastCheck,
			&total, &completed, &failed, &avgDur); err != nil {
			continue
		}

		successRate := 0.0
		if total > 0 {
			successRate = float64(completed) / float64(total)
		}

		agents = append(agents, map[string]any{
			"id":                id,
			"name":              name,
			"status":            status,
			"is_online":         isOnline,
			"last_health_check": lastCheck,
			"total_subtasks":    total,
			"completed":         completed,
			"failed":            failed,
			"success_rate":      successRate,
			"avg_duration_sec":  avgDur,
		})
	}

	jsonOK(w, agents)
}
