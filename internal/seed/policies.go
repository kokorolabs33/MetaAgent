package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedPolicies inserts default governance policies for local-mode demos.
// Safe to call multiple times (idempotent via ON CONFLICT DO NOTHING).
func SeedPolicies(ctx context.Context, pool *pgxpool.Pool) error {
	type pol struct {
		id, name string
		rules    string
		priority int
		active   bool
	}

	policies := []pol{
		{
			id:       "pol-security-review",
			name:     "Security Review Required",
			priority: 100,
			active:   true,
			rules:    `{"when":{"task_contains":["security","vulnerability","auth","password","encryption"]},"require":{"agent_skills":["security_analysis"]},"max_execution_time_minutes":30}`,
		},
		{
			id:       "pol-budget-approval",
			name:     "Budget Approval Threshold",
			priority: 90,
			active:   true,
			rules:    `{"when":{"task_contains":["budget","purchase","expense","procurement"]},"require":{"agent_skills":["financial_analysis"]},"require_approval_above_subtasks":3,"max_execution_time_minutes":60}`,
		},
		{
			id:       "pol-data-access",
			name:     "Data Access Policy",
			priority: 80,
			active:   true,
			rules:    `{"when":{"task_contains":["database","PII","customer data","GDPR"]},"require":{"agent_skills":["data_governance"]},"restrict":{"max_concurrent_subtasks":2}}`,
		},
		{
			id:       "pol-compliance-check",
			name:     "Compliance Check",
			priority: 70,
			active:   true,
			rules:    `{"when":{"task_contains":["compliance","regulation","legal","contract"]},"require":{"agent_skills":["legal_analysis"]},"max_execution_time_minutes":45}`,
		},
		{
			id:       "pol-rate-limiting",
			name:     "Rate Limiting",
			priority: 50,
			active:   false,
			rules:    `{"when":{"always":true},"restrict":{"max_concurrent_subtasks":5,"max_subtasks_per_task":10}}`,
		},
	}

	for _, p := range policies {
		_, err := pool.Exec(ctx,
			`INSERT INTO policies (id, name, rules, priority, is_active)
			 VALUES ($1, $2, $3::jsonb, $4, $5)
			 ON CONFLICT (name) DO NOTHING`,
			p.id, p.name, p.rules, p.priority, p.active)
		if err != nil {
			return fmt.Errorf("seed policy %q: %w", p.name, err)
		}
	}

	return nil
}
