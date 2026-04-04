// Package seed provides data seeding for local mode.
package seed

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	LocalUserID    = "local-user"
	LocalUserEmail = "local@taskhub.local"
	LocalUserName  = "Local User"
)

// LocalSeed creates the default user for local mode.
// No session needed — auth middleware is bypassed in local mode.
// Safe to call multiple times (idempotent).
func LocalSeed(ctx context.Context, pool *pgxpool.Pool) error {
	// Upsert user
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, avatar_url, auth_provider, auth_provider_id)
		 VALUES ($1, $2, $3, '', 'local', 'local')
		 ON CONFLICT (id) DO NOTHING`,
		LocalUserID, LocalUserEmail, LocalUserName)
	if err != nil {
		return fmt.Errorf("seed user: %w", err)
	}

	return nil
}

// SeedAgents creates demo agents for local mode.
// These agents are pre-registered so the platform is immediately usable.
// Safe to call multiple times (idempotent via ON CONFLICT).
func SeedAgents(ctx context.Context, pool *pgxpool.Pool) error {
	agents := []struct {
		id, name, desc, endpoint string
		caps                     string // JSON array
	}{
		{
			id:       "agent-engineering",
			name:     "Engineering",
			desc:     "Handles technical architecture, code reviews, and implementation planning",
			endpoint: "http://engineering-agent:9001",
			caps:     `["code_review","architecture","implementation"]`,
		},
		{
			id:       "agent-finance",
			name:     "Finance",
			desc:     "Handles budgets, cost analysis, financial projections, and ROI calculations",
			endpoint: "http://finance-agent:9002",
			caps:     `["budgeting","cost_analysis","financial_planning"]`,
		},
		{
			id:       "agent-legal",
			name:     "Legal",
			desc:     "Handles compliance, contracts, risk assessment, and regulatory review",
			endpoint: "http://legal-agent:9003",
			caps:     `["compliance","contracts","risk_assessment"]`,
		},
		{
			id:       "agent-marketing",
			name:     "Marketing",
			desc:     "Handles market research, campaign strategy, messaging, and competitive analysis",
			endpoint: "http://marketing-agent:9004",
			caps:     `["market_research","campaign_strategy","competitive_analysis"]`,
		},
	}

	for _, a := range agents {
		_, err := pool.Exec(ctx,
			`INSERT INTO agents (id, name, description, endpoint, capabilities, status, is_online, version)
			 VALUES ($1, $2, $3, $4, $5::jsonb, 'active', false, '1.0.0')
			 ON CONFLICT (id) DO UPDATE SET
			   name = EXCLUDED.name,
			   description = EXCLUDED.description,
			   endpoint = EXCLUDED.endpoint,
			   capabilities = EXCLUDED.capabilities`,
			a.id, a.name, a.desc, a.endpoint, a.caps)
		if err != nil {
			return fmt.Errorf("seed agent %s: %w", a.name, err)
		}
	}

	return nil
}

// LocalSeedAndLog runs LocalSeed and logs the result.
func LocalSeedAndLog(ctx context.Context, pool *pgxpool.Pool) {
	if err := LocalSeed(ctx, pool); err != nil {
		log.Printf("local seed: %v", err)
		return
	}
	if err := SeedAgents(ctx, pool); err != nil {
		log.Printf("agent seed: %v", err)
		return
	}
	log.Println("Local mode: default workspace ready (4 demo agents seeded)")
}
