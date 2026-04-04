// Package policy evaluates active policies against tasks to produce constraints
// that guide the orchestrator's task decomposition.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Constraint is a single constraint derived from policy evaluation.
type Constraint struct {
	PolicyName  string `json:"policy_name"`
	Description string `json:"description"`
}

// EvalResult is the result of evaluating all policies against a task.
type EvalResult struct {
	Constraints                  []Constraint `json:"constraints"`
	MaxSubtasks                  int          `json:"max_subtasks,omitempty"`
	MaxTimeMinutes               int          `json:"max_time_minutes,omitempty"`
	RequireApprovalAboveSubtasks int          `json:"require_approval_above_subtasks,omitempty"`
	AppliedPolicies              []string     `json:"applied_policies"`
}

// PolicyRule is the structure of rules stored in the policies.rules JSONB column.
type PolicyRule struct {
	When                         *WhenClause `json:"when,omitempty"`
	Require                      *Require    `json:"require,omitempty"`
	RestrictAgentsTo             *Restrict   `json:"restrict_agents_to,omitempty"`
	MaxSubtasks                  int         `json:"max_subtasks,omitempty"`
	MaxTimeMinutes               int         `json:"max_execution_time_minutes,omitempty"`
	MaxReplanAttempts            int         `json:"max_replan_attempts,omitempty"`
	RequireApprovalAboveSubtasks int         `json:"require_approval_above_subtasks,omitempty"`
}

// WhenClause defines conditions for policy activation.
type WhenClause struct {
	TaskContains []string `json:"task_contains,omitempty"`
	Always       bool     `json:"always,omitempty"`
}

// Require specifies agent capabilities that must be present.
type Require struct {
	AgentSkills []string `json:"agent_skills,omitempty"`
}

// Restrict limits which agents may be used.
type Restrict struct {
	Tags []string `json:"tags,omitempty"`
}

// Engine evaluates policies against tasks.
type Engine struct {
	DB *pgxpool.Pool
}

// NewEngine creates a policy engine.
func NewEngine(db *pgxpool.Pool) *Engine {
	return &Engine{DB: db}
}

// Evaluate loads all active policies and evaluates them against the task text.
func (e *Engine) Evaluate(ctx context.Context, taskTitle, taskDescription string) (*EvalResult, error) {
	rows, err := e.DB.Query(ctx,
		`SELECT name, rules FROM policies WHERE is_active = true ORDER BY priority DESC`)
	if err != nil {
		return nil, fmt.Errorf("query policies: %w", err)
	}
	defer rows.Close()

	result := &EvalResult{}
	taskText := strings.ToLower(taskTitle + " " + taskDescription)

	for rows.Next() {
		var name string
		var rulesJSON []byte
		if err := rows.Scan(&name, &rulesJSON); err != nil {
			continue
		}

		var rule PolicyRule
		if err := json.Unmarshal(rulesJSON, &rule); err != nil {
			continue
		}

		if !matchesWhen(rule.When, taskText) {
			continue
		}

		result.AppliedPolicies = append(result.AppliedPolicies, name)

		if rule.Require != nil && len(rule.Require.AgentSkills) > 0 {
			result.Constraints = append(result.Constraints, Constraint{
				PolicyName:  name,
				Description: fmt.Sprintf("Must include agent(s) with skills: %s", strings.Join(rule.Require.AgentSkills, ", ")),
			})
		}

		if rule.RestrictAgentsTo != nil && len(rule.RestrictAgentsTo.Tags) > 0 {
			result.Constraints = append(result.Constraints, Constraint{
				PolicyName:  name,
				Description: fmt.Sprintf("Only use agents with tags: %s", strings.Join(rule.RestrictAgentsTo.Tags, ", ")),
			})
		}

		if rule.MaxSubtasks > 0 && (result.MaxSubtasks == 0 || rule.MaxSubtasks < result.MaxSubtasks) {
			result.MaxSubtasks = rule.MaxSubtasks
		}

		if rule.MaxTimeMinutes > 0 && (result.MaxTimeMinutes == 0 || rule.MaxTimeMinutes < result.MaxTimeMinutes) {
			result.MaxTimeMinutes = rule.MaxTimeMinutes
		}

		if rule.RequireApprovalAboveSubtasks > 0 &&
			(result.RequireApprovalAboveSubtasks == 0 || rule.RequireApprovalAboveSubtasks < result.RequireApprovalAboveSubtasks) {
			result.RequireApprovalAboveSubtasks = rule.RequireApprovalAboveSubtasks
		}
	}

	return result, nil
}

// FormatForPrompt converts evaluation results into a constraint string for the LLM prompt.
func (r *EvalResult) FormatForPrompt() string {
	if len(r.Constraints) == 0 && r.MaxSubtasks == 0 && r.MaxTimeMinutes == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n[Policy Constraints]\n")
	for _, c := range r.Constraints {
		fmt.Fprintf(&sb, "- %s (policy: %s)\n", c.Description, c.PolicyName)
	}
	if r.MaxSubtasks > 0 {
		fmt.Fprintf(&sb, "- Maximum %d subtasks\n", r.MaxSubtasks)
	}
	if r.MaxTimeMinutes > 0 {
		fmt.Fprintf(&sb, "- Maximum %d minutes execution time\n", r.MaxTimeMinutes)
	}
	return sb.String()
}

func matchesWhen(when *WhenClause, taskText string) bool {
	if when == nil {
		return true // no condition = always match
	}
	if when.Always {
		return true
	}
	for _, keyword := range when.TaskContains {
		if strings.Contains(taskText, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
