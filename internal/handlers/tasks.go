package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/audit"
	"taskhub/internal/ctxutil"
	"taskhub/internal/events"
	"taskhub/internal/executor"
	"taskhub/internal/models"
)

// TaskHandler provides HTTP handlers for task CRUD and lifecycle operations.
type TaskHandler struct {
	DB         *pgxpool.Pool
	Executor   *executor.DAGExecutor
	EventStore *events.Store
	Audit      *audit.Logger
}

// createTaskRequest is the expected body for POST /tasks.
type createTaskRequest struct {
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	TemplateID        string            `json:"template_id"`
	TemplateVariables map[string]string `json:"template_variables"`
}

// Create handles POST /tasks.
// It validates the request, creates a task record, spawns the executor in a
// background goroutine, and returns 201 with the new task.
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	user := ctxutil.UserFromCtx(r.Context())

	now := time.Now().UTC()
	task := models.Task{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		Status:      "pending",
		CreatedBy:   user.ID,
		TemplateID:  req.TemplateID,
		ReplanCount: 0,
		CreatedAt:   now,
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO tasks (id, title, description, status, created_by, template_id, replan_count, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		task.ID, task.Title, task.Description, task.Status, task.CreatedBy, task.TemplateID, task.ReplanCount, task.CreatedAt)
	if err != nil {
		jsonError(w, "could not create task", http.StatusInternalServerError)
		return
	}

	// Spawn executor in a background goroutine.
	// Use context.Background() because execution outlives the HTTP request.
	go func() { //nolint:contextcheck // context.Background is intentional as execution outlives the HTTP request
		if err := h.Executor.Execute(context.Background(), task); err != nil {
			log.Printf("executor: task %s failed: %v", task.ID, err)
		}
	}()

	jsonCreated(w, task)
}

// List handles GET /tasks.
// Returns tasks for the current org, optionally filtered by ?status= and ?q=,
// with pagination via ?page= and ?per_page=.
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("q")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	// Build query dynamically.
	var conditions []string
	var args []any
	argN := 1

	if statusFilter != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, statusFilter)
		argN++
	}
	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argN, argN))
		args = append(args, "%"+search+"%")
		argN++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Get total count.
	var total int
	countQuery := "SELECT COUNT(*) FROM tasks " + where
	if err := h.DB.QueryRow(r.Context(), countQuery, args...).Scan(&total); err != nil {
		jsonError(w, "count query failed", http.StatusInternalServerError)
		return
	}

	// Get paginated results.
	query := fmt.Sprintf(
		`SELECT id, title, description, status, created_by,
			metadata, plan, result, error, replan_count, created_at, completed_at
		 FROM tasks %s
		 ORDER BY created_at DESC
		 LIMIT $%d OFFSET $%d`,
		where, argN, argN+1)
	args = append(args, perPage, offset)

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := make([]models.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows.Scan)
		if err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + perPage - 1) / perPage
	}

	jsonOK(w, map[string]any{
		"items":    tasks,
		"total":    total,
		"page":     page,
		"per_page": perPage,
		"pages":    totalPages,
	})
}

// Get handles GET /tasks/{id}.
// Returns the task with its subtasks (TaskWithSubtasks). 404 if not found.
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	task, err := scanTask(
		h.DB.QueryRow(r.Context(),
			`SELECT id, title, description, status, created_by,
				metadata, plan, result, error, replan_count, created_at, completed_at
			 FROM tasks
			 WHERE id = $1`, id).Scan,
	)
	if err != nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	subtasks, err := h.querySubtasks(r.Context(), id)
	if err != nil {
		jsonError(w, "could not load subtasks", http.StatusInternalServerError)
		return
	}

	result := models.TaskWithSubtasks{
		Task:     task,
		SubTasks: subtasks,
	}
	jsonOK(w, result)
}

// Cancel handles POST /tasks/{id}/cancel.
// Verifies the task belongs to the org and delegates to the executor.
func (h *TaskHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var exists bool
	err := h.DB.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`, id).Scan(&exists)
	if err != nil || !exists {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if err := h.Executor.Cancel(r.Context(), id); err != nil {
		jsonError(w, "could not cancel task", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]string{"status": "canceled"})
}

// GetCost handles GET /tasks/{id}/cost.
// Returns the aggregated cost summary from audit logs.
func (h *TaskHandler) GetCost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	totalCost, totalInput, totalOutput, err := h.Audit.GetTaskCost(r.Context(), id)
	if err != nil {
		jsonError(w, "could not get cost", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]any{
		"total_cost_usd":      totalCost,
		"total_input_tokens":  totalInput,
		"total_output_tokens": totalOutput,
	})
}

// Approve handles POST /tasks/{id}/approve.
// It approves or rejects a task that is in the "approval_required" state.
func (h *TaskHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Action string `json:"action"` // "approve" or "reject"
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Action != "approve" && req.Action != "reject" {
		jsonError(w, "action must be 'approve' or 'reject'", http.StatusBadRequest)
		return
	}

	// Verify task exists and is in approval_required state
	var status string
	err := h.DB.QueryRow(r.Context(),
		`SELECT status FROM tasks WHERE id = $1`, id).Scan(&status)
	if err != nil {
		jsonError(w, "task not found", http.StatusNotFound)
		return
	}
	if status != "approval_required" {
		jsonError(w, "task is not awaiting approval", http.StatusBadRequest)
		return
	}

	if req.Action == "reject" {
		_, _ = h.DB.Exec(r.Context(), `UPDATE tasks SET status = 'canceled' WHERE id = $1`, id)
		jsonOK(w, map[string]string{"status": "rejected"})
		return
	}

	// Approve: resume execution from stored plan
	go func() { //nolint:contextcheck // context.Background is intentional as execution outlives the HTTP request
		ctx := context.Background()
		if err := h.Executor.ResumeApproved(ctx, id); err != nil {
			log.Printf("approve: resume task %s: %v", id, err)
		}
	}()

	jsonOK(w, map[string]string{"status": "approved"})
}

// ListSubtasks handles GET /tasks/{id}/subtasks.
// Returns all subtasks for a given task.
func (h *TaskHandler) ListSubtasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	subtasks, err := h.querySubtasks(r.Context(), id)
	if err != nil {
		jsonError(w, "could not load subtasks", http.StatusInternalServerError)
		return
	}

	jsonOK(w, subtasks)
}

// querySubtasks loads all subtasks for a task from the database.
func (h *TaskHandler) querySubtasks(ctx context.Context, taskID string) ([]models.SubTask, error) {
	rows, err := h.DB.Query(ctx,
		`SELECT id, task_id, agent_id, instruction, depends_on, status,
			input, output, error, a2a_task_id,
			attempt, max_attempts, created_at, started_at, completed_at
		 FROM subtasks
		 WHERE task_id = $1
		 ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subtasks := make([]models.SubTask, 0)
	for rows.Next() {
		st, err := scanSubtask(rows.Scan)
		if err != nil {
			return nil, err
		}
		subtasks = append(subtasks, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subtasks, nil
}

// scanTask scans a task row, handling nullable JSONB and timestamp columns.
func scanTask(scan func(dest ...any) error) (models.Task, error) {
	var t models.Task
	var metadata, plan, result []byte
	var taskError *string

	err := scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedBy,
		&metadata, &plan, &result, &taskError, &t.ReplanCount, &t.CreatedAt, &t.CompletedAt,
	)
	if err != nil {
		return t, err
	}

	if metadata != nil {
		t.Metadata = json.RawMessage(metadata)
	}
	if plan != nil {
		t.Plan = json.RawMessage(plan)
	}
	if result != nil {
		t.Result = json.RawMessage(result)
	}
	if taskError != nil {
		t.Error = *taskError
	}

	return t, nil
}

// scanSubtask scans a subtask row, handling TEXT[], JSONB, and nullable timestamps.
func scanSubtask(scan func(dest ...any) error) (models.SubTask, error) {
	var st models.SubTask
	var input, output []byte
	var stError, a2aTaskID *string

	err := scan(
		&st.ID, &st.TaskID, &st.AgentID, &st.Instruction, &st.DependsOn, &st.Status,
		&input, &output, &stError, &a2aTaskID,
		&st.Attempt, &st.MaxAttempts, &st.CreatedAt, &st.StartedAt, &st.CompletedAt,
	)
	if err != nil {
		return st, err
	}

	if input != nil {
		st.Input = json.RawMessage(input)
	}
	if output != nil {
		st.Output = json.RawMessage(output)
	}
	if stError != nil {
		st.Error = *stError
	}
	if a2aTaskID != nil {
		st.A2ATaskID = *a2aTaskID
	}
	if st.DependsOn == nil {
		st.DependsOn = []string{}
	}

	return st, nil
}
