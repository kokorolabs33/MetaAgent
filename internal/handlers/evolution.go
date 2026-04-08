package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EvolutionHandler provides analysis of template executions and generates evolution proposals.
type EvolutionHandler struct {
	DB *pgxpool.Pool
}

// EvolutionProposal is a suggested change to a template based on execution data.
type EvolutionProposal struct {
	Type        string `json:"type"` // "add_step", "remove_step", "modify_step", "reorder"
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

// EvolutionAnalysis is the response from the analysis endpoint.
type EvolutionAnalysis struct {
	TemplateID     string              `json:"template_id"`
	TemplateName   string              `json:"template_name"`
	CurrentVersion int                 `json:"current_version"`
	ExecutionCount int                 `json:"execution_count"`
	SuccessRate    float64             `json:"success_rate"`
	AvgDuration    int                 `json:"avg_duration_seconds"`
	AvgHITL        float64             `json:"avg_hitl_interventions"`
	Proposals      []EvolutionProposal `json:"proposals"`
}

// Analyze handles POST /api/templates/{id}/analyze.
// It analyzes recent template executions and generates evolution proposals via LLM.
func (h *EvolutionHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Load template
	var name string
	var version int
	var stepsJSON []byte
	err := h.DB.QueryRow(r.Context(),
		`SELECT name, version, steps FROM workflow_templates WHERE id = $1`, id).
		Scan(&name, &version, &stepsJSON)
	if err != nil {
		jsonError(w, "template not found", http.StatusNotFound)
		return
	}

	// Load recent executions (last 10)
	rows, err := h.DB.Query(r.Context(),
		`SELECT outcome, duration_seconds, hitl_interventions, replan_count
		 FROM template_executions
		 WHERE template_id = $1
		 ORDER BY created_at DESC LIMIT 10`, id)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var totalExecs, completedExecs, totalDuration, totalHITL, totalReplans int
	for rows.Next() {
		var outcome string
		var dur, hitl, replans int
		if err := rows.Scan(&outcome, &dur, &hitl, &replans); err != nil {
			continue
		}
		totalExecs++
		if outcome == "completed" {
			completedExecs++
		}
		totalDuration += dur
		totalHITL += hitl
		totalReplans += replans
	}

	analysis := EvolutionAnalysis{
		TemplateID:     id,
		TemplateName:   name,
		CurrentVersion: version,
		ExecutionCount: totalExecs,
		Proposals:      []EvolutionProposal{},
	}

	if totalExecs > 0 {
		analysis.SuccessRate = float64(completedExecs) / float64(totalExecs)
		analysis.AvgDuration = totalDuration / totalExecs
		analysis.AvgHITL = float64(totalHITL) / float64(totalExecs)
	}

	// Only generate proposals if we have enough execution data
	if totalExecs >= 3 {
		proposals, err := generateProposals(r.Context(), name, string(stepsJSON), totalExecs, analysis.SuccessRate, analysis.AvgHITL, totalReplans)
		if err == nil {
			analysis.Proposals = proposals
		}
	}

	jsonOK(w, analysis)
}

// generateProposals uses an LLM to suggest template improvements based on execution stats.
func generateProposals(ctx context.Context, templateName, stepsJSON string, execCount int, successRate, avgHITL float64, totalReplans int) ([]EvolutionProposal, error) {
	prompt := fmt.Sprintf(`Analyze this workflow template and suggest improvements based on execution data.

Template: %s
Steps: %s

Execution stats (last %d runs):
- Success rate: %.0f%%
- Average HITL interventions per run: %.1f
- Total replans: %d

Based on this data, suggest 0-3 concrete improvements. Return ONLY a JSON array:
[{"type":"add_step|remove_step|modify_step|reorder","description":"what to change","reason":"why"}]

If no improvements are needed, return [].`,
		templateName, stepsJSON, execCount, successRate*100, avgHITL, totalReplans)

	cmd := exec.CommandContext(ctx, "claude", "--print", "--output-format", "text", prompt)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("claude CLI: %w", err)
	}

	result := strings.TrimSpace(string(out))
	// Strip markdown fences if present
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var proposals []EvolutionProposal
	if err := json.Unmarshal([]byte(result), &proposals); err != nil {
		return nil, fmt.Errorf("parse proposals: %w", err)
	}
	return proposals, nil
}
