package a2a

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthChecker periodically probes all active agents, updates their online
// status and skill hash, and invalidates the aggregated card when skills drift.
type HealthChecker struct {
	DB         *pgxpool.Pool
	Resolver   *Resolver
	Aggregator *Aggregator
	Interval   time.Duration
}

// Start launches the health check loop in a background goroutine.
// It blocks until ctx is canceled.
func (h *HealthChecker) Start(ctx context.Context) {
	if h.Interval <= 0 {
		h.Interval = 60 * time.Second
	}

	ticker := time.NewTicker(h.Interval)
	defer ticker.Stop()

	// Run an initial check immediately.
	h.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("health: shutting down")
			return
		case <-ticker.C:
			h.checkAll(ctx)
		}
	}
}

// agentRow holds the fields we need from the agents table for a health check.
type agentRow struct {
	ID        string
	Endpoint  string
	SkillHash string
}

// checkAll queries all active agents and checks each one.
func (h *HealthChecker) checkAll(ctx context.Context) {
	rows, err := h.DB.Query(ctx,
		`SELECT id, endpoint, skill_hash FROM agents WHERE status = 'active'`)
	if err != nil {
		log.Printf("health: query agents: %v", err)
		return
	}
	defer rows.Close()

	var agents []agentRow
	for rows.Next() {
		var a agentRow
		if err := rows.Scan(&a.ID, &a.Endpoint, &a.SkillHash); err != nil {
			log.Printf("health: scan agent: %v", err)
			continue
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		log.Printf("health: iterate agents: %v", err)
		return
	}

	driftDetected := false

	for _, agent := range agents {
		if agent.Endpoint == "" {
			continue
		}

		online, newHash := h.checkOne(ctx, agent.Endpoint)

		_, err := h.DB.Exec(ctx,
			`UPDATE agents
			 SET is_online = $1, last_health_check = NOW(), skill_hash = $2
			 WHERE id = $3`,
			online, newHash, agent.ID)
		if err != nil {
			log.Printf("health: update agent %s: %v", agent.ID, err)
		}

		if online && newHash != agent.SkillHash && agent.SkillHash != "" {
			log.Printf("health: skill drift detected for agent %s (old=%s new=%s)",
				agent.ID, agent.SkillHash, newHash)
			driftDetected = true
		}
	}

	if driftDetected {
		h.Aggregator.Invalidate()
	}
}

// checkOne probes a single agent endpoint via Discover() and returns
// whether it is online and the SHA-256 hash of its skills JSON.
func (h *HealthChecker) checkOne(ctx context.Context, endpoint string) (online bool, skillHash string) {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	discovered, err := h.Resolver.Discover(checkCtx, endpoint)
	if err != nil {
		return false, ""
	}

	skillsJSON, err := json.Marshal(discovered.Skills)
	if err != nil {
		log.Printf("health: marshal skills for %s: %v", endpoint, err)
		return true, ""
	}

	hash := sha256.Sum256(skillsJSON)
	return true, fmt.Sprintf("%x", hash)
}
