package a2a

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CardInterface describes a supported protocol binding for the agent.
type CardInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
}

// AggregatedCard is the merged AgentCard built from all connected agents.
type AggregatedCard struct {
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	URL                 string          `json:"url"`
	Version             string          `json:"version"`
	SupportedInterfaces []CardInterface `json:"supportedInterfaces"`
	Capabilities        CardCapability  `json:"capabilities"`
	Skills              []CardSkill     `json:"skills"`
	DefaultInputModes   []string        `json:"defaultInputModes"`
	DefaultOutputModes  []string        `json:"defaultOutputModes"`
}

// Aggregator builds and caches an AggregatedCard from all active, online agents.
type Aggregator struct {
	DB *pgxpool.Pool

	mu       sync.RWMutex
	card     *AggregatedCard
	cardJSON []byte
	etag     string
	builtAt  time.Time
}

// NewAggregator creates a new Aggregator backed by the given database pool.
func NewAggregator(db *pgxpool.Pool) *Aggregator {
	return &Aggregator{DB: db}
}

// cacheTTL controls how long a cached card remains valid before rebuild.
const cacheTTL = 5 * time.Minute

// GetCard returns the cached aggregated card JSON and its ETag.
// If the cache is empty or expired, it rebuilds automatically.
func (a *Aggregator) GetCard(ctx context.Context, baseURL string) ([]byte, string, error) {
	a.mu.RLock()
	if a.cardJSON != nil && time.Since(a.builtAt) < cacheTTL {
		data, tag := a.cardJSON, a.etag
		a.mu.RUnlock()
		return data, tag, nil
	}
	a.mu.RUnlock()

	return a.Rebuild(ctx, baseURL)
}

// Rebuild forces a rebuild of the aggregated card from the database.
func (a *Aggregator) Rebuild(ctx context.Context, baseURL string) ([]byte, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	card, err := a.buildCard(ctx, baseURL)
	if err != nil {
		return nil, "", fmt.Errorf("build aggregated card: %w", err)
	}

	data, err := json.Marshal(card)
	if err != nil {
		return nil, "", fmt.Errorf("marshal aggregated card: %w", err)
	}

	hash := sha256.Sum256(data)
	etag := fmt.Sprintf(`"%x"`, hash[:8])

	a.card = card
	a.cardJSON = data
	a.etag = etag
	a.builtAt = time.Now()

	// Persist the aggregated card to a2a_server_config for external consumption.
	if _, err := a.DB.Exec(ctx,
		`UPDATE a2a_server_config SET aggregated_card = $1, card_updated_at = NOW() WHERE id = 1`,
		data); err != nil {
		log.Printf("aggregator: persist card to DB: %v", err)
	}

	return data, etag, nil
}

// Invalidate clears the cached card so the next GetCard call triggers a rebuild.
func (a *Aggregator) Invalidate() {
	a.mu.Lock()
	a.cardJSON = nil
	a.card = nil
	a.etag = ""
	a.builtAt = time.Time{}
	a.mu.Unlock()
}

// buildCard queries all active+online agents, deduplicates their skills,
// reads name/description overrides, and returns the assembled card.
func (a *Aggregator) buildCard(ctx context.Context, baseURL string) (*AggregatedCard, error) {
	// Collect skills from all active+online agents.
	rows, err := a.DB.Query(ctx,
		`SELECT skills FROM agents WHERE status = 'active' AND skills IS NOT NULL AND skills != '[]'::jsonb`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	var allSkills []CardSkill

	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan skills: %w", err)
		}
		if raw == nil {
			continue
		}

		var skills []CardSkill
		if err := json.Unmarshal(raw, &skills); err != nil {
			log.Printf("aggregator: unmarshal skills: %v", err)
			continue
		}

		for _, s := range skills {
			if _, dup := seen[s.ID]; dup {
				continue
			}
			seen[s.ID] = struct{}{}
			allSkills = append(allSkills, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}

	// Read optional name/description overrides from a2a_server_config.
	var nameOverride, descOverride *string
	err = a.DB.QueryRow(ctx,
		`SELECT name_override, description_override FROM a2a_server_config WHERE id = 1`,
	).Scan(&nameOverride, &descOverride)
	if err != nil {
		log.Printf("aggregator: read config overrides: %v", err)
	}

	name := "TaskHub Meta-Agent"
	if nameOverride != nil && *nameOverride != "" {
		name = *nameOverride
	}
	description := "An orchestrating agent that decomposes tasks and delegates to specialized sub-agents."
	if descOverride != nil && *descOverride != "" {
		description = *descOverride
	}

	if allSkills == nil {
		allSkills = []CardSkill{}
	}

	return &AggregatedCard{
		Name:        name,
		Description: description,
		URL:         strings.TrimRight(baseURL, "/") + "/a2a",
		Version:     "1.0.0",
		SupportedInterfaces: []CardInterface{
			{
				URL:             strings.TrimRight(baseURL, "/") + "/a2a",
				ProtocolBinding: "jsonrpc-over-http",
			},
		},
		Capabilities: CardCapability{
			Streaming:              false,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		Skills:             allSkills,
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}, nil
}
