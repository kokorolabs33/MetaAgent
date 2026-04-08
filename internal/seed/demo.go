package seed

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedDemo inserts realistic completed task data to populate analytics,
// audit, template, and dashboard pages for demos.
// Safe to call multiple times (idempotent via ON CONFLICT DO NOTHING).
// Runs inside a single transaction -- all-or-nothing.
func SeedDemo(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// ── Tasks ───────────────────────────────────────────────────
	type demoTask struct {
		id, title, description, status string
		dayOffset                      int
		durationSec                    int
		errMsg                         *string
	}

	agentErr := "Agent unreachable after 3 retries"
	tasks := []demoTask{
		{"demo-task-001", "Quarterly Security Audit for Auth Service", "Comprehensive security review of the authentication service including OWASP Top 10 scanning and authorization controls audit", "completed", 13, 180, nil},
		{"demo-task-002", "Market Analysis: AI Developer Tools 2026", "Research market size, growth trends, and competitive landscape for AI-powered developer tools", "completed", 11, 240, nil},
		{"demo-task-003", "Code Review: Payment Gateway Integration", "Security and performance review of the new Stripe payment gateway integration", "completed", 9, 120, nil},
		{"demo-task-004", "Bug Triage: Session Timeout Issue", "Investigate and root-cause the session timeout bug reported by enterprise customers", "completed", 7, 90, nil},
		{"demo-task-005", "Content Strategy: Developer Blog Launch", "Plan and draft initial content for the company developer blog targeting DevOps engineers", "completed", 5, 300, nil},
		{"demo-task-006", "Compliance Review: GDPR Data Processing", "Review data processing agreements and ensure GDPR compliance for EU customer data", "failed", 4, 0, &agentErr},
		{"demo-task-007", "Infrastructure Cost Optimization", "Analyze cloud infrastructure spend and recommend cost optimization strategies", "completed", 2, 200, nil},
		{"demo-task-008", "API Documentation Update", "Update OpenAPI specs and developer docs for v2.3 release", "completed", 1, 150, nil},
	}

	taskCount := 0
	for _, t := range tasks {
		var completedAt interface{}
		if t.status == "completed" {
			completedAt = fmt.Sprintf("NOW() - INTERVAL '%d days' + INTERVAL '%d seconds'", t.dayOffset, t.durationSec)
		}

		var query string
		if t.status == "completed" {
			query = fmt.Sprintf(
				`INSERT INTO tasks (id, title, description, status, created_by, source, created_at, completed_at)
				 VALUES ($1, $2, $3, $4, $5, 'web',
				         NOW() - INTERVAL '%d days',
				         NOW() - INTERVAL '%d days' + INTERVAL '%d seconds')
				 ON CONFLICT (id) DO NOTHING`,
				t.dayOffset, t.dayOffset, t.durationSec)
		} else {
			query = fmt.Sprintf(
				`INSERT INTO tasks (id, title, description, status, created_by, source, error, created_at)
				 VALUES ($1, $2, $3, $4, $5, 'web', $6,
				         NOW() - INTERVAL '%d days')
				 ON CONFLICT (id) DO NOTHING`,
				t.dayOffset)
		}

		var execErr error
		if t.status == "completed" {
			_, execErr = tx.Exec(ctx, query, t.id, t.title, t.description, t.status, LocalUserID)
		} else {
			_, execErr = tx.Exec(ctx, query, t.id, t.title, t.description, t.status, LocalUserID, t.errMsg)
		}
		if execErr != nil {
			return fmt.Errorf("seed task %s: %w", t.id, execErr)
		}
		taskCount++
		_ = completedAt // used in query template above
	}

	// ── Subtasks ────────────────────────────────────────────────
	// Subtasks reference agents by name via subquery.
	// If agent does not exist, the INSERT is skipped (WHERE clause filters NULL).
	type demoSubtask struct {
		id, taskID, instruction, agentName, status string
		dayOffset, offsetSec                       int
		output                                     string
		dependsOn                                  string // postgres array literal
		errMsg                                     *string
	}

	subtasks := []demoSubtask{
		// Task 001: Security Audit (2 subtasks)
		{"demo-sub-001a", "demo-task-001", "Scan authentication service for OWASP Top 10 vulnerabilities", "Engineering Department", "completed", 13, 30, `{"text":"Found 2 medium-severity issues: XSS in error messages, weak CORS policy"}`, "{}", nil},
		{"demo-sub-001b", "demo-task-001", "Review authorization controls and session management", "Engineering Department", "completed", 13, 90, `{"text":"Authorization controls solid. Recommend adding token rotation for refresh tokens"}`, "{demo-sub-001a}", nil},
		{"demo-sub-001c", "demo-task-001", "Compile audit report with risk ratings and remediation steps", "Engineering Department", "completed", 13, 150, `{"text":"Audit report generated: 2 medium, 0 critical findings. Remediation timeline: 2 weeks"}`, "{demo-sub-001a,demo-sub-001b}", nil},

		// Task 002: Market Analysis (3 subtasks)
		{"demo-sub-002a", "demo-task-002", "Research market size and growth trends for AI developer tools", "Marketing Department", "completed", 11, 40, `{"text":"AI dev tools market: $12.8B in 2026, 34% CAGR. Key segments: code generation, testing, documentation"}`, "{}", nil},
		{"demo-sub-002b", "demo-task-002", "Identify top 5 competitors and analyze positioning", "Marketing Department", "completed", 11, 100, `{"text":"Top 5: GitHub Copilot, Cursor, Codeium, Tabnine, Amazon Q. Copilot leads with 45% market share"}`, "{}", nil},
		{"demo-sub-002c", "demo-task-002", "Compile findings into executive summary with recommendations", "Marketing Department", "completed", 11, 200, `{"text":"Executive summary: differentiate on multi-agent orchestration. Target enterprise segment with A2A protocol advantage"}`, "{demo-sub-002a,demo-sub-002b}", nil},

		// Task 003: Code Review (2 subtasks)
		{"demo-sub-003a", "demo-task-003", "Review payment gateway code for security vulnerabilities", "Engineering Department", "completed", 9, 25, `{"text":"No critical security issues. Minor: API key should use vault, not env var"}`, "{}", nil},
		{"demo-sub-003b", "demo-task-003", "Analyze performance implications of Stripe integration", "Engineering Department", "completed", 9, 80, `{"text":"Performance acceptable. Stripe SDK adds 120ms p99 latency. Recommend connection pooling"}`, "{demo-sub-003a}", nil},

		// Task 004: Bug Triage (2 subtasks)
		{"demo-sub-004a", "demo-task-004", "Reproduce and classify severity of session timeout bug", "Engineering Department", "completed", 7, 20, `{"text":"Reproduced: sessions expire after 15min instead of 24h. Severity: High. Affects all enterprise SSO users"}`, "{}", nil},
		{"demo-sub-004b", "demo-task-004", "Identify root cause and affected components", "Engineering Department", "completed", 7, 60, `{"text":"Root cause: token refresh middleware skips SSO sessions. Fix: check auth_provider before skipping refresh"}`, "{demo-sub-004a}", nil},

		// Task 005: Content Strategy (3 subtasks)
		{"demo-sub-005a", "demo-task-005", "Research topic and outline key points for developer blog", "Marketing Department", "completed", 5, 50, `{"text":"Outlined 8 initial posts: A2A protocol intro, multi-agent patterns, orchestration deep-dive, performance tuning..."}`, "{}", nil},
		{"demo-sub-005b", "demo-task-005", "Draft blog post on A2A protocol for DevOps engineers", "Marketing Department", "completed", 5, 180, `{"text":"Draft complete: 'A2A Protocol: The Missing Piece in Your DevOps Automation' — 2,400 words with diagrams"}`, "{demo-sub-005a}", nil},
		{"demo-sub-005c", "demo-task-005", "Review draft for accuracy, tone, and SEO optimization", "Marketing Department", "completed", 5, 260, `{"text":"Review complete: added 3 internal links, improved meta description, keyword density at 1.2%"}`, "{demo-sub-005b}", nil},

		// Task 006: Compliance Review - FAILED (2 subtasks, one failed)
		{"demo-sub-006a", "demo-task-006", "Review GDPR data processing agreements", "Legal Department", "completed", 4, 30, `{"text":"DPA review complete: 3 clauses need updates for new EU AI Act requirements"}`, "{}", nil},
		{"demo-sub-006b", "demo-task-006", "Assess data flow compliance for EU customer data", "Legal Department", "failed", 4, 60, "", "{demo-sub-006a}", &agentErr},

		// Task 007: Cost Optimization (2 subtasks)
		{"demo-sub-007a", "demo-task-007", "Analyze current cloud infrastructure spend by service", "Finance Department", "completed", 2, 40, `{"text":"Monthly spend: $24,300. Top 3: RDS ($8,200), EC2 ($6,100), Lambda ($3,400). 23% idle resources detected"}`, "{}", nil},
		{"demo-sub-007b", "demo-task-007", "Recommend cost optimization strategies with ROI projections", "Finance Department", "completed", 2, 150, `{"text":"Recommended: reserved instances (-35%), right-sizing (-18%), spot for batch (-60%). Projected savings: $7,200/mo"}`, "{demo-sub-007a}", nil},

		// Task 008: API Docs (2 subtasks)
		{"demo-sub-008a", "demo-task-008", "Update OpenAPI specifications for v2.3 endpoints", "Engineering Department", "completed", 1, 30, `{"text":"Updated 12 endpoint specs: 3 new endpoints, 5 modified request schemas, 4 new response types"}`, "{}", nil},
		{"demo-sub-008b", "demo-task-008", "Write developer guide sections for new API features", "Engineering Department", "completed", 1, 100, `{"text":"Added guides: webhook configuration, batch operations, pagination cursors. Total: 3,200 words"}`, "{demo-sub-008a}", nil},
	}

	subtaskCount := 0
	for _, s := range subtasks {
		query := fmt.Sprintf(
			`INSERT INTO subtasks (id, task_id, instruction, agent_id, status, output, depends_on, error,
			                       created_at, started_at, completed_at)
			 SELECT $1, $2, $3, a.id, $4,
			        CASE WHEN $5 = '' THEN NULL ELSE $5::jsonb END,
			        $6::text[], $7,
			        NOW() - INTERVAL '%d days' + INTERVAL '%d seconds',
			        NOW() - INTERVAL '%d days' + INTERVAL '%d seconds',
			        CASE WHEN $4 = 'completed' OR $4 = 'failed'
			             THEN NOW() - INTERVAL '%d days' + INTERVAL '%d seconds'
			             ELSE NULL END
			 FROM agents a WHERE a.name = $8
			 ON CONFLICT (id) DO NOTHING`,
			s.dayOffset, s.offsetSec,
			s.dayOffset, s.offsetSec+5,
			s.dayOffset, s.offsetSec+30)

		_, err := tx.Exec(ctx, query,
			s.id, s.taskID, s.instruction, s.status,
			s.output, s.dependsOn, s.errMsg, s.agentName)
		if err != nil {
			return fmt.Errorf("seed subtask %s: %w", s.id, err)
		}
		subtaskCount++
	}

	// ── Events ──────────────────────────────────────────────────
	type demoEvent struct {
		id, taskID, eventType, actorType, actorID string
		dayOffset, offsetSec                      int
		subtaskID                                 *string
	}

	var eventDefs []demoEvent
	eventCounter := 0

	for i, t := range tasks {
		base := fmt.Sprintf("demo-evt-%03d", i+1)

		// Standard lifecycle events for each task
		eventDefs = append(eventDefs,
			demoEvent{base + "a", t.id, "task.planning", "system", "master-agent", t.dayOffset, 0, nil},
			demoEvent{base + "b", t.id, "task.planned", "system", "master-agent", t.dayOffset, 5, nil},
			demoEvent{base + "c", t.id, "task.running", "system", "executor", t.dayOffset, 10, nil},
		)

		if t.status == "completed" {
			eventDefs = append(eventDefs,
				demoEvent{base + "z", t.id, "task.completed", "system", "executor", t.dayOffset, t.durationSec, nil},
			)
		} else {
			eventDefs = append(eventDefs,
				demoEvent{base + "z", t.id, "task.failed", "system", "executor", t.dayOffset, 60, nil},
			)
		}
	}

	// Add subtask-level events for the first few tasks for richer data
	subEvents := []demoEvent{
		{"demo-evt-sub-001a", "demo-task-001", "subtask.created", "system", "executor", 13, 10, strPtr("demo-sub-001a")},
		{"demo-evt-sub-001b", "demo-task-001", "agent.working", "agent", "engineering", 13, 15, strPtr("demo-sub-001a")},
		{"demo-evt-sub-001c", "demo-task-001", "message", "agent", "engineering", 13, 25, strPtr("demo-sub-001a")},
		{"demo-evt-sub-001d", "demo-task-001", "subtask.created", "system", "executor", 13, 85, strPtr("demo-sub-001b")},
		{"demo-evt-sub-001e", "demo-task-001", "agent.working", "agent", "engineering", 13, 88, strPtr("demo-sub-001b")},
		{"demo-evt-sub-002a", "demo-task-002", "subtask.created", "system", "executor", 11, 10, strPtr("demo-sub-002a")},
		{"demo-evt-sub-002b", "demo-task-002", "agent.working", "agent", "marketing", 11, 15, strPtr("demo-sub-002a")},
		{"demo-evt-sub-002c", "demo-task-002", "message", "agent", "marketing", 11, 35, strPtr("demo-sub-002a")},
		{"demo-evt-sub-003a", "demo-task-003", "subtask.created", "system", "executor", 9, 10, strPtr("demo-sub-003a")},
		{"demo-evt-sub-003b", "demo-task-003", "agent.working", "agent", "engineering", 9, 12, strPtr("demo-sub-003a")},
		{"demo-evt-sub-005a", "demo-task-005", "subtask.created", "system", "executor", 5, 10, strPtr("demo-sub-005a")},
		{"demo-evt-sub-005b", "demo-task-005", "agent.working", "agent", "marketing", 5, 15, strPtr("demo-sub-005a")},
		{"demo-evt-sub-005c", "demo-task-005", "message", "agent", "marketing", 5, 45, strPtr("demo-sub-005a")},
		{"demo-evt-sub-006a", "demo-task-006", "subtask.created", "system", "executor", 4, 10, strPtr("demo-sub-006a")},
		{"demo-evt-sub-006b", "demo-task-006", "agent.working", "agent", "legal", 4, 15, strPtr("demo-sub-006a")},
	}
	eventDefs = append(eventDefs, subEvents...)

	eventCount := 0
	for _, e := range eventDefs {
		var query string
		if e.subtaskID != nil {
			query = fmt.Sprintf(
				`INSERT INTO events (id, task_id, subtask_id, type, actor_type, actor_id, data, created_at)
				 SELECT $1, $2, CASE WHEN EXISTS (SELECT 1 FROM subtasks WHERE id = $3) THEN $3 ELSE NULL END,
				        $4, $5, $6, '{}'::jsonb,
				        NOW() - INTERVAL '%d days' + INTERVAL '%d seconds'
				 ON CONFLICT (id) DO NOTHING`,
				e.dayOffset, e.offsetSec)
			_, err = tx.Exec(ctx, query, e.id, e.taskID, *e.subtaskID, e.eventType, e.actorType, e.actorID)
		} else {
			query = fmt.Sprintf(
				`INSERT INTO events (id, task_id, type, actor_type, actor_id, data, created_at)
				 VALUES ($1, $2, $3, $4, $5, '{}'::jsonb,
				         NOW() - INTERVAL '%d days' + INTERVAL '%d seconds')
				 ON CONFLICT (id) DO NOTHING`,
				e.dayOffset, e.offsetSec)
			_, err = tx.Exec(ctx, query, e.id, e.taskID, e.eventType, e.actorType, e.actorID)
		}
		if err != nil {
			return fmt.Errorf("seed event %s: %w", e.id, err)
		}
		eventCount++
		_ = eventCounter
	}

	// ── Template Executions ─────────────────────────────────────
	type tmplExec struct {
		id, templateID, taskID, outcome string
		version, duration               int
	}

	tmplExecs := []tmplExec{
		{"demo-texec-001", "tmpl-security-audit", "demo-task-001", "completed", 2, 180},
		{"demo-texec-002", "tmpl-market-research", "demo-task-002", "completed", 2, 240},
		{"demo-texec-003", "tmpl-code-review", "demo-task-003", "completed", 3, 120},
		{"demo-texec-005", "tmpl-content-creation", "demo-task-005", "completed", 4, 300},
	}

	tmplExecCount := 0
	for _, te := range tmplExecs {
		_, err := tx.Exec(ctx,
			`INSERT INTO template_executions (id, template_id, template_version, task_id, outcome, duration_seconds)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (id) DO NOTHING`,
			te.id, te.templateID, te.version, te.taskID, te.outcome, te.duration)
		if err != nil {
			return fmt.Errorf("seed template execution %s: %w", te.id, err)
		}
		tmplExecCount++
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	log.Printf("seed-demo: seeded %d tasks, %d subtasks, %d events, %d template executions",
		taskCount, subtaskCount, eventCount, tmplExecCount)
	return nil
}

func strPtr(s string) *string {
	return &s
}
