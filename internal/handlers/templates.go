package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/models"
)

// TemplateHandler provides CRUD operations for workflow templates.
type TemplateHandler struct {
	DB *pgxpool.Pool
}

// templateWithStats embeds WorkflowTemplate with usage statistics from template_executions.
type templateWithStats struct {
	models.WorkflowTemplate
	UsageCount     int     `json:"usage_count"`
	SuccessRate    float64 `json:"success_rate"`
	AvgDurationSec float64 `json:"avg_duration_sec"`
}

// List handles GET /api/templates.
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT wt.id, wt.name, wt.description, wt.version, wt.steps, wt.variables,
		        wt.source_task_id, wt.is_active, wt.created_at, wt.updated_at,
		        COUNT(te.id) AS usage_count,
		        COALESCE(
		          ROUND(100.0 * SUM(CASE WHEN te.outcome = 'completed' THEN 1 ELSE 0 END) / NULLIF(COUNT(te.id), 0)),
		          0
		        ) AS success_rate,
		        COALESCE(AVG(te.duration_seconds), 0) AS avg_duration_sec
		 FROM workflow_templates wt
		 LEFT JOIN template_executions te ON te.template_id = wt.id
		 GROUP BY wt.id, wt.name, wt.description, wt.version, wt.steps, wt.variables,
		          wt.source_task_id, wt.is_active, wt.created_at, wt.updated_at
		 ORDER BY wt.updated_at DESC`)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	templates := make([]templateWithStats, 0)
	for rows.Next() {
		var ts templateWithStats
		var steps, variables []byte
		var sourceTaskID *string
		if err := rows.Scan(&ts.ID, &ts.Name, &ts.Description, &ts.Version,
			&steps, &variables, &sourceTaskID, &ts.IsActive, &ts.CreatedAt, &ts.UpdatedAt,
			&ts.UsageCount, &ts.SuccessRate, &ts.AvgDurationSec); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		if steps != nil {
			ts.Steps = json.RawMessage(steps)
		}
		if variables != nil {
			ts.Variables = json.RawMessage(variables)
		}
		if sourceTaskID != nil {
			ts.SourceTaskID = *sourceTaskID
		}
		templates = append(templates, ts)
	}
	if rows.Err() != nil {
		jsonError(w, "rows error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, templates)
}

type createTemplateRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Steps       json.RawMessage `json:"steps"`
	Variables   json.RawMessage `json:"variables"`
}

// Create handles POST /api/templates.
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Steps == nil {
		req.Steps = json.RawMessage(`[]`)
	}
	if req.Variables == nil {
		req.Variables = json.RawMessage(`[]`)
	}

	now := time.Now().UTC()
	t := models.WorkflowTemplate{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Version:     1,
		Steps:       req.Steps,
		Variables:   req.Variables,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO workflow_templates (id, name, description, version, steps, variables, is_active, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		t.ID, t.Name, t.Description, t.Version, t.Steps, t.Variables, t.IsActive, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		jsonError(w, "could not create template", http.StatusInternalServerError)
		return
	}

	// Save version 1
	_, _ = h.DB.Exec(r.Context(),
		`INSERT INTO template_versions (id, template_id, version, steps, source, changes, created_at)
		 VALUES ($1,$2,1,$3,'manual_save','[]',$4)`,
		uuid.New().String(), t.ID, t.Steps, now)

	jsonCreated(w, t)
}

// Get handles GET /api/templates/{id}.
func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var t models.WorkflowTemplate
	var steps, variables []byte
	var sourceTaskID *string
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, description, version, steps, variables, source_task_id, is_active, created_at, updated_at
		 FROM workflow_templates WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Description, &t.Version,
			&steps, &variables, &sourceTaskID, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		jsonError(w, "template not found", http.StatusNotFound)
		return
	}
	if steps != nil {
		t.Steps = json.RawMessage(steps)
	}
	if variables != nil {
		t.Variables = json.RawMessage(variables)
	}
	if sourceTaskID != nil {
		t.SourceTaskID = *sourceTaskID
	}

	jsonOK(w, t)
}

// Update handles PUT /api/templates/{id}.
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        *string          `json:"name"`
		Description *string          `json:"description"`
		Steps       *json.RawMessage `json:"steps"`
		Variables   *json.RawMessage `json:"variables"`
		IsActive    *bool            `json:"is_active"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	stepsChanged := false

	if req.Name != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE workflow_templates SET name = $1, updated_at = $2 WHERE id = $3`, *req.Name, now, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.Description != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE workflow_templates SET description = $1, updated_at = $2 WHERE id = $3`, *req.Description, now, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.Steps != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE workflow_templates SET steps = $1, updated_at = $2 WHERE id = $3`, *req.Steps, now, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
		stepsChanged = true
	}
	if req.Variables != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE workflow_templates SET variables = $1, updated_at = $2 WHERE id = $3`, *req.Variables, now, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.IsActive != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE workflow_templates SET is_active = $1, updated_at = $2 WHERE id = $3`, *req.IsActive, now, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}

	// If steps changed, bump version and save snapshot
	if stepsChanged {
		if _, err := h.DB.Exec(r.Context(), `UPDATE workflow_templates SET version = version + 1, updated_at = $1 WHERE id = $2`, now, id); err != nil {
			jsonError(w, "version bump failed", http.StatusInternalServerError)
			return
		}
		var newVersion int
		if err := h.DB.QueryRow(r.Context(), `SELECT version FROM workflow_templates WHERE id = $1`, id).Scan(&newVersion); err != nil {
			jsonError(w, "read version failed", http.StatusInternalServerError)
			return
		}
		_, _ = h.DB.Exec(r.Context(),
			`INSERT INTO template_versions (id, template_id, version, steps, source, changes, created_at)
			 VALUES ($1,$2,$3,$4,'user_edit','["steps updated"]',$5)`,
			uuid.New().String(), id, newVersion, *req.Steps, now)
	}

	h.Get(w, r)
}

// Delete handles DELETE /api/templates/{id}.
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM workflow_templates WHERE id = $1`, id)
	if err != nil {
		jsonError(w, "could not delete", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateFromTask handles POST /api/templates/from-task/{task_id}.
// Extracts the DAG structure from a completed task and saves as a template.
func (h *TemplateHandler) CreateFromTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "task_id")

	// Get the task's plan
	var planJSON []byte
	var taskTitle string
	var taskStatus string
	err := h.DB.QueryRow(r.Context(),
		`SELECT title, status, plan FROM tasks WHERE id = $1`, taskID).
		Scan(&taskTitle, &taskStatus, &planJSON)
	if err != nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
	if taskStatus != "completed" {
		jsonError(w, "can only create templates from completed tasks", http.StatusBadRequest)
		return
	}
	if planJSON == nil {
		jsonError(w, "task has no plan", http.StatusBadRequest)
		return
	}

	// Parse the plan to extract step structure
	var plan models.ExecutionPlan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		jsonError(w, "could not parse task plan", http.StatusInternalServerError)
		return
	}

	// Convert plan subtasks into template steps.
	// Template steps use instruction templates instead of specific agent IDs.
	type templateStep struct {
		ID          string   `json:"id"`
		Instruction string   `json:"instruction_template"`
		DependsOn   []string `json:"depends_on,omitempty"`
	}

	steps := make([]templateStep, 0, len(plan.SubTasks))
	for _, st := range plan.SubTasks {
		steps = append(steps, templateStep{
			ID:          st.ID,
			Instruction: st.Instruction,
			DependsOn:   st.DependsOn,
		})
	}

	stepsJSON, _ := json.Marshal(steps)

	// Optional: allow custom name from request body
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "Template from: " + taskTitle
	}

	now := time.Now().UTC()
	t := models.WorkflowTemplate{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		Version:      1,
		Steps:        stepsJSON,
		Variables:    json.RawMessage(`[]`),
		SourceTaskID: taskID,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO workflow_templates (id, name, description, version, steps, variables, source_task_id, is_active, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		t.ID, t.Name, t.Description, t.Version, t.Steps, t.Variables, t.SourceTaskID, t.IsActive, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		jsonError(w, "could not create template", http.StatusInternalServerError)
		return
	}

	// Save version 1
	_, _ = h.DB.Exec(r.Context(),
		`INSERT INTO template_versions (id, template_id, version, steps, source, changes, created_at)
		 VALUES ($1,$2,1,$3,'manual_save','["created from task"]',$4)`,
		uuid.New().String(), t.ID, t.Steps, now)

	jsonCreated(w, t)
}

// Rollback handles POST /api/templates/{id}/rollback/{version}.
func (h *TemplateHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	version := chi.URLParam(r, "version")

	// Get the version's steps
	var steps []byte
	err := h.DB.QueryRow(r.Context(),
		`SELECT steps FROM template_versions WHERE template_id = $1 AND version = $2`, id, version).
		Scan(&steps)
	if err != nil {
		jsonError(w, "version not found", http.StatusNotFound)
		return
	}

	// Update template with rolled-back steps and bump version
	now := time.Now().UTC()
	if _, err := h.DB.Exec(r.Context(),
		`UPDATE workflow_templates SET steps = $1, version = version + 1, updated_at = $2 WHERE id = $3`,
		steps, now, id); err != nil {
		jsonError(w, "rollback failed", http.StatusInternalServerError)
		return
	}

	// Get new version number
	var newVersion int
	if err := h.DB.QueryRow(r.Context(), `SELECT version FROM workflow_templates WHERE id = $1`, id).Scan(&newVersion); err != nil {
		jsonError(w, "read version failed", http.StatusInternalServerError)
		return
	}

	// Save as new version
	_, _ = h.DB.Exec(r.Context(),
		`INSERT INTO template_versions (id, template_id, version, steps, source, changes, created_at)
		 VALUES ($1,$2,$3,$4,'user_edit',$5,$6)`,
		uuid.New().String(), id, newVersion, steps,
		json.RawMessage(fmt.Sprintf(`["rolled back to version %s"]`, version)), now)

	h.Get(w, r)
}

// ListExecutions handles GET /api/templates/{id}/executions.
func (h *TemplateHandler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	rows, err := h.DB.Query(r.Context(),
		`SELECT id, template_id, template_version, task_id, actual_steps,
			hitl_interventions, replan_count, outcome, duration_seconds, created_at
		 FROM template_executions WHERE template_id = $1 ORDER BY created_at DESC`, id)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	execs := make([]models.TemplateExecution, 0)
	for rows.Next() {
		var e models.TemplateExecution
		var actualSteps []byte
		if err := rows.Scan(&e.ID, &e.TemplateID, &e.TemplateVersion, &e.TaskID,
			&actualSteps, &e.HITLInterventions, &e.ReplanCount, &e.Outcome,
			&e.DurationSeconds, &e.CreatedAt); err != nil {
			continue
		}
		if actualSteps != nil {
			e.ActualSteps = json.RawMessage(actualSteps)
		}
		execs = append(execs, e)
	}
	jsonOK(w, execs)
}
