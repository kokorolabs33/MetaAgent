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
	LocalOrgID     = "local-org"
	LocalOrgName   = "My Workspace"
	LocalOrgSlug   = "local"
)

// LocalSeed creates the default user + org for local mode.
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

	// Upsert org
	_, err = pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		LocalOrgID, LocalOrgName, LocalOrgSlug)
	if err != nil {
		return fmt.Errorf("seed org: %w", err)
	}

	// Upsert membership (owner)
	_, err = pool.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role)
		 VALUES ($1, $2, 'owner')
		 ON CONFLICT (org_id, user_id) DO NOTHING`,
		LocalOrgID, LocalUserID)
	if err != nil {
		return fmt.Errorf("seed membership: %w", err)
	}

	return nil
}

// LocalSeedAndLog runs LocalSeed and logs the result.
func LocalSeedAndLog(ctx context.Context, pool *pgxpool.Pool) {
	if err := LocalSeed(ctx, pool); err != nil {
		log.Printf("local seed: %v", err)
		return
	}
	log.Println("Local mode: default workspace ready")
}
