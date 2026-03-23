package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	DB *pgxpool.Pool
}

type Entry struct {
	OrgID        string
	TaskID       string
	SubtaskID    string
	AgentID      string
	ActorType    string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Details      any
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Endpoint     string
	LatencyMs    int
	StatusCode   int
}

// Log inserts an audit log entry into the database.
func (l *Logger) Log(ctx context.Context, e Entry) error {
	detailsJSON, err := json.Marshal(e.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	_, err = l.DB.Exec(ctx,
		`INSERT INTO audit_logs (id, org_id, task_id, subtask_id, agent_id, actor_type, actor_id,
		 action, resource_type, resource_id, details, model, input_tokens, output_tokens, cost_usd,
		 endpoint_called, latency_ms, status_code)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6, $7,
		 $8, $9, NULLIF($10,''), $11, NULLIF($12,''), $13, $14, $15,
		 NULLIF($16,''), NULLIF($17,0), NULLIF($18,0))`,
		uuid.New().String(), e.OrgID, e.TaskID, e.SubtaskID, e.AgentID,
		e.ActorType, e.ActorID, e.Action, e.ResourceType, e.ResourceID,
		detailsJSON, e.Model, e.InputTokens, e.OutputTokens, e.CostUSD,
		e.Endpoint, e.LatencyMs, e.StatusCode)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// GetTaskCost returns total cost, input tokens, and output tokens for a task.
func (l *Logger) GetTaskCost(ctx context.Context, taskID string) (float64, int, int, error) {
	var totalCost float64
	var totalInput, totalOutput int
	err := l.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost_usd),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0)
		 FROM audit_logs WHERE task_id = $1`, taskID).
		Scan(&totalCost, &totalInput, &totalOutput)
	return totalCost, totalInput, totalOutput, err
}

// GetOrgMonthlySpend returns the current month's total spend for an organization.
func (l *Logger) GetOrgMonthlySpend(ctx context.Context, orgID string) (float64, error) {
	var spent float64
	err := l.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM audit_logs
		 WHERE org_id = $1 AND created_at >= date_trunc('month', NOW())`,
		orgID).Scan(&spent)
	return spent, err
}
