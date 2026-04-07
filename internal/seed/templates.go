package seed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedTemplates inserts default workflow templates for local-mode demos.
// Safe to call multiple times (idempotent via ON CONFLICT DO NOTHING).
func SeedTemplates(ctx context.Context, pool *pgxpool.Pool) error {
	type tmpl struct {
		id, name, description string
		version               int
		steps, variables      string
		active                bool
	}

	templates := []tmpl{
		{
			id:          "tmpl-code-review",
			name:        "Code Review Pipeline",
			description: "Automated multi-agent code review covering security, performance, and synthesis",
			version:     3,
			active:      true,
			steps: `[
				{"id":"s1","instruction_template":"Review {{repo_url}} for security vulnerabilities","depends_on":[]},
				{"id":"s2","instruction_template":"Analyze performance implications of changes in {{repo_url}}","depends_on":[]},
				{"id":"s3","instruction_template":"Synthesize security and performance findings into final review","depends_on":["s1","s2"]}
			]`,
			variables: `[{"name":"repo_url","type":"string","description":"Repository URL to review"}]`,
		},
		{
			id:          "tmpl-market-research",
			name:        "Market Research Report",
			description: "Competitive analysis and market sizing with executive summary output",
			version:     2,
			active:      true,
			steps: `[
				{"id":"s1","instruction_template":"Research market size and growth trends for {{industry}}","depends_on":[]},
				{"id":"s2","instruction_template":"Identify top 5 competitors in {{industry}} and analyze positioning","depends_on":[]},
				{"id":"s3","instruction_template":"Compile findings into executive summary with recommendations","depends_on":["s1","s2"]}
			]`,
			variables: `[{"name":"industry","type":"string","description":"Target industry or market segment"}]`,
		},
		{
			id:          "tmpl-bug-triage",
			name:        "Bug Triage Workflow",
			description: "Reproduce, classify, and root-cause analysis for reported bugs",
			version:     1,
			active:      true,
			steps: `[
				{"id":"s1","instruction_template":"Reproduce and classify severity of bug: {{bug_description}}","depends_on":[]},
				{"id":"s2","instruction_template":"Identify root cause and affected components for: {{bug_description}}","depends_on":["s1"]}
			]`,
			variables: `[{"name":"bug_description","type":"string","description":"Bug report description"}]`,
		},
		{
			id:          "tmpl-content-creation",
			name:        "Content Creation Pipeline",
			description: "End-to-end content pipeline from research through draft to SEO-optimized review",
			version:     4,
			active:      true,
			steps: `[
				{"id":"s1","instruction_template":"Research topic and outline key points for: {{topic}}","depends_on":[]},
				{"id":"s2","instruction_template":"Draft {{content_type}} on {{topic}} targeting {{audience}}","depends_on":["s1"]},
				{"id":"s3","instruction_template":"Review draft for accuracy, tone, and SEO optimization","depends_on":["s2"]}
			]`,
			variables: `[
				{"name":"topic","type":"string","description":"Content topic"},
				{"name":"content_type","type":"string","description":"Article, blog post, whitepaper, etc."},
				{"name":"audience","type":"string","description":"Target audience"}
			]`,
		},
		{
			id:          "tmpl-security-audit",
			name:        "Security Audit Checklist",
			description: "OWASP-based security audit with auth review and remediation report",
			version:     2,
			active:      true,
			steps: `[
				{"id":"s1","instruction_template":"Scan {{target}} for known vulnerabilities (OWASP Top 10)","depends_on":[]},
				{"id":"s2","instruction_template":"Review authentication and authorization controls for {{target}}","depends_on":[]},
				{"id":"s3","instruction_template":"Compile audit report with risk ratings and remediation steps","depends_on":["s1","s2"]}
			]`,
			variables: `[{"name":"target","type":"string","description":"Application or system to audit"}]`,
		},
		{
			id:          "tmpl-onboarding",
			name:        "Onboarding Checklist",
			description: "New employee onboarding workflow with system access and orientation scheduling",
			version:     1,
			active:      false,
			steps: `[
				{"id":"s1","instruction_template":"Prepare system access and accounts for {{employee_name}}","depends_on":[]},
				{"id":"s2","instruction_template":"Schedule orientation meetings for {{employee_name}} in {{department}}","depends_on":["s1"]}
			]`,
			variables: `[
				{"name":"employee_name","type":"string","description":"New employee name"},
				{"name":"department","type":"string","description":"Department"}
			]`,
		},
	}

	for _, t := range templates {
		_, err := pool.Exec(ctx,
			`INSERT INTO workflow_templates (id, name, description, version, steps, variables, is_active)
			 VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7)
			 ON CONFLICT (name) DO NOTHING`,
			t.id, t.name, t.description, t.version, t.steps, t.variables, t.active)
		if err != nil {
			return fmt.Errorf("seed template %q: %w", t.name, err)
		}
	}

	return nil
}
