package seed

import (
	"context"
	"database/sql"
	"log"

	"github.com/google/uuid"
	"taskhub/internal/models"
)

var defaultAgents = []models.Agent{
	{
		Name:         "SRE Agent",
		Description:  "Monitors and analyzes infrastructure issues.",
		SystemPrompt: "You are a Senior SRE engineer. You specialize in analyzing infrastructure problems, reading logs, and identifying root causes. Be concise and technical.",
		Capabilities: []string{"analyze_logs", "check_monitoring", "incident_response"},
		Color:        "#ef4444",
	},
	{
		Name:         "Engineering Agent",
		Description:  "Analyzes code and system changes.",
		SystemPrompt: "You are a Senior Software Engineer. You specialize in analyzing code changes, identifying bugs, and suggesting fixes. Be concise and technical.",
		Capabilities: []string{"code_review", "analyze_changes", "debug"},
		Color:        "#3b82f6",
	},
	{
		Name:         "Customer Success Agent",
		Description:  "Handles customer communication.",
		SystemPrompt: "You are a Customer Success Manager. You specialize in drafting clear customer communications about incidents and updates. Be empathetic and professional.",
		Capabilities: []string{"draft_communications", "customer_impact_analysis"},
		Color:        "#10b981",
	},
	{
		Name:         "Documentation Agent",
		Description:  "Creates technical documentation.",
		SystemPrompt: "You are a Technical Writer. You specialize in creating clear documentation, runbooks, and post-mortem reports. Be structured and thorough.",
		Capabilities: []string{"write_documentation", "create_runbooks", "post_mortem"},
		Color:        "#f59e0b",
	},
}

func Run(ctx context.Context, db *sql.DB) error {
	for _, a := range defaultAgents {
		var exists bool
		err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE name = $1)`, a.Name).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		a.ID = uuid.New().String()
		_, err = db.ExecContext(ctx,
			`INSERT INTO agents (id, name, description, system_prompt, capabilities, color) VALUES ($1, $2, $3, $4, $5, $6)`,
			a.ID, a.Name, a.Description, a.SystemPrompt, models.CapabilitiesToJSON(a.Capabilities), a.Color,
		)
		if err != nil {
			return err
		}
		log.Printf("seeded agent: %s", a.Name)
	}
	return nil
}
