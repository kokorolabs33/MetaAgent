package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalyticsHandler provides HTTP handlers for aggregate analytics.
type AnalyticsHandler struct {
	DB *pgxpool.Pool
}

// GetDashboard handles GET /api/analytics/dashboard.
// Returns aggregate statistics for the dashboard view.
func (h *AnalyticsHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var totalTasks, completedTasks, failedTasks, runningTasks, totalAgents, onlineAgents int
	var avgDurationSec float64

	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&totalTasks)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE status = 'completed'`).Scan(&completedTasks)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE status = 'failed'`).Scan(&failedTasks)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE status IN ('running', 'planning')`).Scan(&runningTasks)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE status = 'active'`).Scan(&totalAgents)
	_ = h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE status = 'active' AND is_online = true`).Scan(&onlineAgents)
	_ = h.DB.QueryRow(ctx,
		`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - created_at))), 0)
		 FROM tasks WHERE status = 'completed' AND completed_at IS NOT NULL`).Scan(&avgDurationSec)

	// Task status distribution (for pie chart)
	statusRows, err := h.DB.Query(ctx,
		`SELECT status, COUNT(*) FROM tasks GROUP BY status ORDER BY COUNT(*) DESC`)
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

	// Tasks per day (last 7 days, for bar chart)
	dailyRows, err := h.DB.Query(ctx,
		`SELECT DATE(created_at)::text AS day, COUNT(*) AS count
		 FROM tasks
		 WHERE created_at > NOW() - INTERVAL '7 days'
		 GROUP BY DATE(created_at)
		 ORDER BY day`)
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

	// Agent usage (subtask count per agent)
	agentRows, err := h.DB.Query(ctx,
		`SELECT a.name, COUNT(s.id) AS task_count,
			SUM(CASE WHEN s.status = 'completed' THEN 1 ELSE 0 END) AS completed,
			SUM(CASE WHEN s.status = 'failed' THEN 1 ELSE 0 END) AS failed
		 FROM agents a
		 LEFT JOIN subtasks s ON s.agent_id = a.id
		 WHERE a.status = 'active'
		 GROUP BY a.id, a.name
		 ORDER BY task_count DESC
		 LIMIT 10`)
	agentUsage := make([]map[string]any, 0)
	if err == nil {
		defer agentRows.Close()
		for agentRows.Next() {
			var name string
			var taskCount, completed, failed int
			if err := agentRows.Scan(&name, &taskCount, &completed, &failed); err != nil {
				continue
			}
			agentUsage = append(agentUsage, map[string]any{
				"name": name, "task_count": taskCount,
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

func safeDiv(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
