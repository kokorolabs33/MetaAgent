package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalyticsHandler provides HTTP handlers for aggregate analytics.
type AnalyticsHandler struct {
	DB *pgxpool.Pool
}

// timeCondition returns a SQL condition string for the given range param.
// Valid values: "7d", "30d", "all" (or empty). Returns empty string for "all"/empty.
func timeCondition(rangeParam string) string {
	switch rangeParam {
	case "7d":
		return "created_at > NOW() - INTERVAL '7 days'"
	case "30d":
		return "created_at > NOW() - INTERVAL '30 days'"
	default:
		return ""
	}
}

// validRange validates the range query parameter against an allowlist.
func validRange(r string) string {
	switch r {
	case "7d", "30d", "all":
		return r
	default:
		return "all"
	}
}

// validStatus validates the status query parameter against an allowlist.
func validStatus(s string) string {
	switch s {
	case "completed", "failed", "all":
		return s
	default:
		return "all"
	}
}

// GetDashboard handles GET /api/analytics/dashboard.
// Accepts optional query params: range (7d|30d|all) and status (completed|failed|all).
// Returns aggregate statistics for the dashboard view.
func (h *AnalyticsHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rangeParam := validRange(r.URL.Query().Get("range"))
	statusParam := validStatus(r.URL.Query().Get("status"))

	// Build task WHERE clause from filters
	var taskConds []string
	if tc := timeCondition(rangeParam); tc != "" {
		taskConds = append(taskConds, tc)
	}
	if statusParam != "all" {
		taskConds = append(taskConds, fmt.Sprintf("status = '%s'", statusParam))
	}
	taskWhere := ""
	if len(taskConds) > 0 {
		taskWhere = "WHERE " + strings.Join(taskConds, " AND ")
	}

	var totalTasks, completedTasks, failedTasks, runningTasks, totalAgents, onlineAgents int
	var avgDurationSec float64

	_ = h.DB.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, taskWhere)).Scan(&totalTasks)

	// For specific stat queries, add status condition on top of taskWhere
	completedWhere := addCondition(taskWhere, "status = 'completed'")
	failedWhere := addCondition(taskWhere, "status = 'failed'")
	runningWhere := addCondition(taskWhere, "status IN ('running', 'planning')")

	_ = h.DB.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, completedWhere)).Scan(&completedTasks)
	_ = h.DB.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, failedWhere)).Scan(&failedTasks)
	_ = h.DB.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, runningWhere)).Scan(&runningTasks)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE status = 'active'`).Scan(&totalAgents)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE status = 'active' AND is_online = true`).Scan(&onlineAgents)

	avgWhere := addCondition(taskWhere, "status = 'completed' AND completed_at IS NOT NULL")
	_ = h.DB.QueryRow(ctx, fmt.Sprintf(
		`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - created_at))), 0)
		 FROM tasks %s`, avgWhere)).Scan(&avgDurationSec)

	// Task status distribution (for pie chart)
	statusRows, err := h.DB.Query(ctx, fmt.Sprintf(
		`SELECT status, COUNT(*) FROM tasks %s GROUP BY status ORDER BY COUNT(*) DESC`, taskWhere))
	statusDist := make([]map[string]any, 0)
	if err == nil {
		defer statusRows.Close()
		for statusRows.Next() {
			var status string
			var count int
			if err := statusRows.Scan(&status, &count); err != nil {
				continue
			}
			statusDist = append(statusDist, map[string]any{"status": status, "count": count})
		}
	}

	// Tasks per day (for bar chart) -- always shows recent days regardless of "all" range
	dailyInterval := "7 days"
	if rangeParam == "30d" {
		dailyInterval = "30 days"
	}
	dailyWhere := fmt.Sprintf("WHERE created_at > NOW() - INTERVAL '%s'", dailyInterval)
	if statusParam != "all" {
		dailyWhere += fmt.Sprintf(" AND status = '%s'", statusParam)
	}
	dailyRows, err := h.DB.Query(ctx, fmt.Sprintf(
		`SELECT DATE(created_at)::text AS day, COUNT(*) AS count
		 FROM tasks
		 %s
		 GROUP BY DATE(created_at)
		 ORDER BY day`, dailyWhere))
	dailyTasks := make([]map[string]any, 0)
	if err == nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var day string
			var count int
			if err := dailyRows.Scan(&day, &count); err != nil {
				continue
			}
			dailyTasks = append(dailyTasks, map[string]any{"date": day, "count": count})
		}
	}

	// Agent usage (subtask count per agent) with time/status filters on subtasks
	var subtaskConds []string
	if tc := timeCondition(rangeParam); tc != "" {
		subtaskConds = append(subtaskConds, "s."+tc)
	}
	if statusParam != "all" {
		subtaskConds = append(subtaskConds, fmt.Sprintf("s.status = '%s'", statusParam))
	}
	subtaskExtra := ""
	if len(subtaskConds) > 0 {
		subtaskExtra = "AND " + strings.Join(subtaskConds, " AND ")
	}

	agentRows, err := h.DB.Query(ctx, fmt.Sprintf(
		`SELECT a.id, a.name, COUNT(s.id) AS task_count,
			SUM(CASE WHEN s.status = 'completed' THEN 1 ELSE 0 END) AS completed,
			SUM(CASE WHEN s.status = 'failed' THEN 1 ELSE 0 END) AS failed
		 FROM agents a
		 LEFT JOIN subtasks s ON s.agent_id = a.id %s
		 WHERE a.status = 'active'
		 GROUP BY a.id, a.name
		 ORDER BY task_count DESC
		 LIMIT 10`, subtaskExtra))
	agentUsage := make([]map[string]any, 0)
	if err == nil {
		defer agentRows.Close()
		for agentRows.Next() {
			var agentID, name string
			var taskCount, completed, failed int
			if err := agentRows.Scan(&agentID, &name, &taskCount, &completed, &failed); err != nil {
				continue
			}
			agentUsage = append(agentUsage, map[string]any{
				"id": agentID, "name": name, "task_count": taskCount,
				"completed": completed, "failed": failed,
			})
		}
	}

	jsonOK(w, map[string]any{
		"total_tasks":         totalTasks,
		"completed_tasks":     completedTasks,
		"failed_tasks":        failedTasks,
		"running_tasks":       runningTasks,
		"success_rate":        safeDiv(completedTasks, totalTasks),
		"total_agents":        totalAgents,
		"online_agents":       onlineAgents,
		"avg_duration_sec":    avgDurationSec,
		"status_distribution": statusDist,
		"daily_tasks":         dailyTasks,
		"agent_usage":         agentUsage,
	})
}

// GetAgentTasks handles GET /api/analytics/agents/{id}/tasks.
// Returns a list of subtasks assigned to a specific agent, filtered by time range and status.
func (h *AnalyticsHandler) GetAgentTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentID := chi.URLParam(r, "id")
	rangeParam := validRange(r.URL.Query().Get("range"))
	statusParam := validStatus(r.URL.Query().Get("status"))

	var conditions []string
	var args []any
	argN := 1

	conditions = append(conditions, fmt.Sprintf("s.agent_id = $%d", argN))
	args = append(args, agentID)
	argN++

	if tc := timeCondition(rangeParam); tc != "" {
		conditions = append(conditions, "s."+tc)
	}
	if statusParam != "" && statusParam != "all" {
		conditions = append(conditions, fmt.Sprintf("s.status = $%d", argN))
		args = append(args, statusParam)
		argN++ //nolint:ineffassign // kept for consistency with builder pattern
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	rows, err := h.DB.Query(ctx, fmt.Sprintf(
		`SELECT s.id, t.title, s.status,
		        COALESCE(EXTRACT(EPOCH FROM (s.completed_at - s.started_at)), 0) AS duration_sec,
		        s.created_at
		 FROM subtasks s
		 JOIN tasks t ON t.id = s.task_id
		 %s
		 ORDER BY s.created_at DESC
		 LIMIT 50`, where), args...)
	if err != nil {
		http.Error(w, "query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type agentTask struct {
		ID          string    `json:"id"`
		TaskTitle   string    `json:"task_title"`
		Status      string    `json:"status"`
		DurationSec float64   `json:"duration_sec"`
		CreatedAt   time.Time `json:"created_at"`
	}

	tasks := make([]agentTask, 0)
	for rows.Next() {
		var t agentTask
		if err := rows.Scan(&t.ID, &t.TaskTitle, &t.Status, &t.DurationSec, &t.CreatedAt); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}

	jsonOK(w, tasks)
}

// addCondition appends a SQL condition to an existing WHERE clause (or creates one).
func addCondition(existing, cond string) string {
	if existing == "" {
		return "WHERE " + cond
	}
	return existing + " AND " + cond
}

func safeDiv(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
