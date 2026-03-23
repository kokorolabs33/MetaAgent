// Package executor implements the DAG execution engine for task orchestration.
//
// The DAGExecutor takes an execution plan (DAG of subtasks) and runs subtasks
// in dependency order. It handles polling external agents, retries, human-in-the-loop
// input via signal channels, blocked propagation, replanning, budget enforcement,
// and cancellation.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/adapter"
	"taskhub/internal/audit"
	"taskhub/internal/events"
	"taskhub/internal/models"
	"taskhub/internal/orchestrator"
)

// Sentinel errors.
var (
	ErrSubtaskNotWaiting = errors.New("subtask is not waiting for input")
	ErrBudgetExceeded    = errors.New("monthly budget exceeded")
	ErrTaskCanceled      = errors.New("task was canceled")
)

// Polling backoff intervals (exponential with cap).
var pollIntervals = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

// DAGExecutor manages the lifecycle of task execution.
type DAGExecutor struct {
	DB           *pgxpool.Pool
	Broker       *events.Broker
	EventStore   *events.Store
	Audit        *audit.Logger
	Orchestrator *orchestrator.Orchestrator
	Adapters     map[string]adapter.AgentAdapter // adapter_type → adapter

	signals sync.Map // subtask_id → chan adapter.UserInput
	cancels sync.Map // task_id → context.CancelFunc

	maxConcurrent      int // global max concurrent subtasks (default 10)
	maxConcurrentAgent int // per-agent max concurrent subtasks (default 3)
}

// MaxConcurrent returns the global concurrency limit, defaulting to 10.
func (e *DAGExecutor) getMaxConcurrent() int {
	if e.maxConcurrent > 0 {
		return e.maxConcurrent
	}
	return 10
}

// MaxConcurrentAgent returns the per-agent concurrency limit, defaulting to 3.
func (e *DAGExecutor) getMaxConcurrentAgent() int {
	if e.maxConcurrentAgent > 0 {
		return e.maxConcurrentAgent
	}
	return 3
}

// Execute is the main entry point for task execution.
// It plans the task via the orchestrator, creates subtask records, and runs the DAG loop.
func (e *DAGExecutor) Execute(ctx context.Context, task models.Task) error {
	// 1. Update task status to "planning"
	if err := e.updateTaskStatus(ctx, task.ID, "planning", ""); err != nil {
		return fmt.Errorf("update task to planning: %w", err)
	}
	e.publishEvent(ctx, task.ID, "", "task.planning", "system", "", nil)

	// 2. Load org for budget checks
	var orgBudget *float64
	var orgAlertThreshold float64
	err := e.DB.QueryRow(ctx,
		`SELECT budget_usd_monthly, budget_alert_threshold FROM organizations WHERE id = $1`,
		task.OrgID).Scan(&orgBudget, &orgAlertThreshold)
	if err != nil {
		return fmt.Errorf("load org: %w", err)
	}

	// 3. Check budget before LLM call
	if err := e.checkBudget(ctx, task.OrgID, orgBudget); err != nil {
		_ = e.updateTaskStatus(ctx, task.ID, "failed", err.Error())
		e.publishEvent(ctx, task.ID, "", "task.failed", "system", "", map[string]any{"error": err.Error()})
		return err
	}

	// 4. Load agents for this org
	agents, err := e.loadOrgAgents(ctx, task.OrgID)
	if err != nil {
		return fmt.Errorf("load agents: %w", err)
	}
	if len(agents) == 0 {
		errMsg := "no agents available for this organization"
		_ = e.updateTaskStatus(ctx, task.ID, "failed", errMsg)
		e.publishEvent(ctx, task.ID, "", "task.failed", "system", "", map[string]any{"error": errMsg})
		return errors.New(errMsg)
	}

	// 5. Call orchestrator to create plan
	plan, err := e.Orchestrator.Plan(ctx, task, agents)
	if err != nil {
		errMsg := fmt.Sprintf("planning failed: %v", err)
		_ = e.updateTaskStatus(ctx, task.ID, "failed", errMsg)
		e.publishEvent(ctx, task.ID, "", "task.failed", "system", "", map[string]any{"error": errMsg})
		return fmt.Errorf("orchestrator plan: %w", err)
	}

	// 6. Store plan in task record
	planJSON, _ := json.Marshal(plan)
	_, err = e.DB.Exec(ctx,
		`UPDATE tasks SET plan = $1, status = 'running' WHERE id = $2`,
		planJSON, task.ID)
	if err != nil {
		return fmt.Errorf("store plan: %w", err)
	}
	e.publishEvent(ctx, task.ID, "", "task.planned", "system", "", map[string]any{"summary": plan.Summary})

	// 7. Create subtask records from plan
	subtasks, err := e.createSubtasks(ctx, task.ID, plan, agents)
	if err != nil {
		return fmt.Errorf("create subtasks: %w", err)
	}

	// Publish subtask.created events
	for i := range subtasks {
		e.publishEvent(ctx, task.ID, subtasks[i].ID, "subtask.created", "system", "", map[string]any{
			"agent_id":    subtasks[i].AgentID,
			"instruction": subtasks[i].Instruction,
			"depends_on":  subtasks[i].DependsOn,
		})
	}

	e.publishEvent(ctx, task.ID, "", "task.running", "system", "", nil)

	// 8. Run DAG loop
	return e.runDAGLoop(ctx, task, subtasks, agents)
}

// createSubtasks creates DB records from the execution plan.
// It maps plan temp IDs to real UUIDs and resolves depends_on references.
func (e *DAGExecutor) createSubtasks(ctx context.Context, taskID string, plan *models.ExecutionPlan, agents []models.Agent) ([]models.SubTask, error) {
	// Build ID mapping: plan temp ID → real UUID
	idMap := make(map[string]string, len(plan.SubTasks))
	for _, ps := range plan.SubTasks {
		idMap[ps.ID] = uuid.New().String()
	}

	// Build agent name→ID lookup for fallback matching
	agentNameMap := make(map[string]string, len(agents))
	for _, a := range agents {
		agentNameMap[a.Name] = a.ID
	}

	subtasks := make([]models.SubTask, 0, len(plan.SubTasks))
	now := time.Now()

	for _, ps := range plan.SubTasks {
		realID := idMap[ps.ID]

		// Resolve agent ID: use agent_id from plan, fallback to name lookup
		agentID := ps.AgentID
		if agentID == "" {
			if id, ok := agentNameMap[ps.AgentName]; ok {
				agentID = id
			}
		}
		if agentID == "" {
			return nil, fmt.Errorf("subtask %s: could not resolve agent (id=%q, name=%q)", ps.ID, ps.AgentID, ps.AgentName)
		}

		// Resolve depends_on: map plan IDs → real UUIDs
		deps := make([]string, 0)
		for _, depID := range ps.DependsOn {
			realDepID, ok := idMap[depID]
			if !ok {
				return nil, fmt.Errorf("subtask %s depends on unknown subtask %s", ps.ID, depID)
			}
			deps = append(deps, realDepID)
		}

		st := models.SubTask{
			ID:          realID,
			TaskID:      taskID,
			AgentID:     agentID,
			Instruction: ps.Instruction,
			DependsOn:   deps,
			Status:      "pending",
			Attempt:     0,
			MaxAttempts: 3,
			CreatedAt:   now,
		}

		// Insert into DB
		_, err := e.DB.Exec(ctx,
			`INSERT INTO subtasks (id, task_id, agent_id, instruction, depends_on, status, attempt, max_attempts, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			st.ID, st.TaskID, st.AgentID, st.Instruction, st.DependsOn, st.Status, st.Attempt, st.MaxAttempts, st.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert subtask %s: %w", st.ID, err)
		}

		subtasks = append(subtasks, st)
	}

	return subtasks, nil
}

// runDAGLoop is the core DAG execution loop.
// It finds ready subtasks, starts them in goroutines, and waits for completion.
func (e *DAGExecutor) runDAGLoop(ctx context.Context, task models.Task, subtasks []models.SubTask, agents []models.Agent) error {
	// Create a cancellable context for this task
	taskCtx, taskCancel := context.WithCancel(ctx)
	defer taskCancel()
	e.cancels.Store(task.ID, taskCancel)
	defer e.cancels.Delete(task.ID)

	// Channel to notify the DAG loop when any subtask changes status
	statusChangeCh := make(chan string, len(subtasks))

	// Track running goroutines
	var wg sync.WaitGroup
	runningCount := 0
	agentRunning := make(map[string]int) // agent_id → count

	// Build agent lookup
	agentMap := make(map[string]models.Agent, len(agents))
	for _, a := range agents {
		agentMap[a.ID] = a
	}

	for {
		// Reload subtask statuses from DB
		var err error
		subtasks, err = e.loadSubtasks(taskCtx, task.ID)
		if err != nil {
			return fmt.Errorf("reload subtasks: %w", err)
		}

		// Check if all are terminal
		allDone := true
		anyFailed := false
		var failedSubtask *models.SubTask
		pendingCount := 0
		runningCount = 0

		for i := range subtasks {
			switch subtasks[i].Status {
			case "completed", "canceled":
				// Terminal success states
			case "failed":
				anyFailed = true
				if failedSubtask == nil {
					failedSubtask = &subtasks[i]
				}
				allDone = false
			case "blocked":
				// Blocked is terminal for this loop iteration
			case "running", "waiting_for_input":
				allDone = false
				if subtasks[i].Status == "running" {
					runningCount++
				}
			case "pending":
				allDone = false
				pendingCount++
			}
		}

		// All completed (no pending, no running, no failed)
		if allDone && !anyFailed {
			now := time.Now()
			_, _ = e.DB.Exec(taskCtx, `UPDATE tasks SET status = 'completed', completed_at = $1 WHERE id = $2`, now, task.ID)
			e.publishEvent(taskCtx, task.ID, "", "task.completed", "system", "", nil)
			wg.Wait()
			return nil
		}

		// If we have a failed subtask and no running/pending work, try replan or fail
		if anyFailed && runningCount == 0 && pendingCount == 0 {
			// Try replan
			replanned, err := e.tryReplan(taskCtx, task, *failedSubtask, agents, subtasks)
			if err != nil || !replanned {
				// No more replans or replan failed — task failed
				errMsg := fmt.Sprintf("subtask %s failed: %s", failedSubtask.ID, failedSubtask.Error)
				_ = e.updateTaskStatus(taskCtx, task.ID, "failed", errMsg)
				e.publishEvent(taskCtx, task.ID, "", "task.failed", "system", "", map[string]any{"error": errMsg})
				wg.Wait()
				return fmt.Errorf("task failed: %s", errMsg)
			}
			// Replan succeeded — reload subtasks and continue loop
			continue
		}

		// Find ready subtasks
		ready := findReadySubtasks(subtasks)

		// Start ready subtasks with concurrency limits
		for _, st := range ready {
			// Global concurrency limit
			if runningCount >= e.getMaxConcurrent() {
				break
			}
			// Per-agent concurrency limit
			if agentRunning[st.AgentID] >= e.getMaxConcurrentAgent() {
				continue
			}

			agent, ok := agentMap[st.AgentID]
			if !ok {
				log.Printf("executor: agent %s not found for subtask %s", st.AgentID, st.ID)
				continue
			}

			// Mark as running
			now := time.Now()
			_, err := e.DB.Exec(taskCtx,
				`UPDATE subtasks SET status = 'running', started_at = $1, attempt = attempt + 1 WHERE id = $2`,
				now, st.ID)
			if err != nil {
				log.Printf("executor: update subtask %s to running: %v", st.ID, err)
				continue
			}

			runningCount++
			agentRunning[st.AgentID]++

			e.publishEvent(taskCtx, task.ID, st.ID, "subtask.running", "system", "", map[string]any{
				"agent_id": st.AgentID,
				"attempt":  st.Attempt + 1,
			})

			// Launch goroutine
			stCopy := st
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.runSubtask(taskCtx, task, stCopy, agent, subtasks, agents, statusChangeCh)
			}()
		}

		// Wait for a status change or context cancellation
		select {
		case <-taskCtx.Done():
			// Task was canceled
			_ = e.updateTaskStatus(ctx, task.ID, "canceled", "task canceled")
			e.publishEvent(ctx, task.ID, "", "task.canceled", "system", "", nil)
			wg.Wait()
			return ErrTaskCanceled
		case <-statusChangeCh:
			// A subtask changed status, re-evaluate the DAG
			continue
		}
	}
}

// runSubtask handles the lifecycle of a single subtask:
// submit → poll loop → handle terminal status.
func (e *DAGExecutor) runSubtask(
	ctx context.Context,
	task models.Task,
	st models.SubTask,
	agent models.Agent,
	allSubtasks []models.SubTask,
	agents []models.Agent,
	statusChangeCh chan<- string,
) {
	// Always notify the DAG loop when we're done
	defer func() {
		select {
		case statusChangeCh <- st.ID:
		default:
		}
	}()

	// Check budget before submit
	if err := e.checkBudgetForOrg(ctx, task.OrgID); err != nil {
		e.failSubtask(ctx, task.ID, st.ID, err.Error(), allSubtasks)
		return
	}

	// Build input with upstream outputs
	inputMap := buildSubtaskInput(st, allSubtasks, agents)
	inputJSON, _ := json.Marshal(inputMap)

	// Store input in DB
	_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET input = $1 WHERE id = $2`, inputJSON, st.ID)

	// Get the adapter for this agent type
	adp, ok := e.Adapters[agent.AdapterType]
	if !ok {
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("no adapter for type %q", agent.AdapterType), allSubtasks)
		return
	}

	// Submit to agent
	handle, err := adp.Submit(ctx, agent, adapter.SubTaskInput{
		TaskID:      st.TaskID,
		Instruction: st.Instruction,
		Input:       inputJSON,
	})
	if err != nil {
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("submit failed: %v", err), allSubtasks)
		return
	}

	// Store poll_job_id in DB (critical for crash recovery)
	_, err = e.DB.Exec(ctx,
		`UPDATE subtasks SET poll_job_id = $1, poll_endpoint = $2 WHERE id = $3`,
		handle.JobID, handle.StatusEndpoint, st.ID)
	if err != nil {
		log.Printf("executor: store poll_job_id for subtask %s: %v", st.ID, err)
	}

	// Audit: agent call submitted
	_ = e.Audit.Log(ctx, audit.Entry{
		OrgID:        task.OrgID,
		TaskID:       task.ID,
		SubtaskID:    st.ID,
		AgentID:      agent.ID,
		ActorType:    "system",
		ActorID:      "executor",
		Action:       "agent.submit",
		ResourceType: "subtask",
		ResourceID:   st.ID,
		Endpoint:     agent.Endpoint,
	})

	// Polling loop with exponential backoff
	e.pollSubtask(ctx, task, st, agent, handle, allSubtasks, agents, statusChangeCh)
}

// pollSubtask runs the polling loop for a submitted subtask.
func (e *DAGExecutor) pollSubtask(
	ctx context.Context,
	task models.Task,
	st models.SubTask,
	agent models.Agent,
	handle adapter.JobHandle,
	allSubtasks []models.SubTask,
	agents []models.Agent,
	statusChangeCh chan<- string,
) {
	adp := e.Adapters[agent.AdapterType]
	intervalIdx := 0
	var lastProgress *float64

	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Wait before polling
		interval := pollIntervals[intervalIdx]
		if intervalIdx < len(pollIntervals)-1 {
			intervalIdx++
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		// Poll agent
		status, err := adp.Poll(ctx, agent, handle)
		if err != nil {
			log.Printf("executor: poll subtask %s: %v", st.ID, err)
			// Network errors during poll are transient — continue polling
			continue
		}

		switch status.Status {
		case "running":
			// Publish progress if changed
			if status.Progress != nil && (lastProgress == nil || *status.Progress != *lastProgress) {
				lastProgress = status.Progress
				e.publishEvent(ctx, task.ID, st.ID, "subtask.progress", "agent", agent.ID, map[string]any{
					"progress": *status.Progress,
				})
			}

			// Publish any new messages from the agent to the group chat
			for _, msg := range status.Messages {
				e.publishMessage(ctx, task.ID, agent.ID, agent.Name, msg.Content)
			}

		case "completed":
			now := time.Now()
			_, err := e.DB.Exec(ctx,
				`UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2 WHERE id = $3`,
				status.Result, now, st.ID)
			if err != nil {
				log.Printf("executor: update subtask %s to completed: %v", st.ID, err)
			}

			e.publishEvent(ctx, task.ID, st.ID, "subtask.completed", "agent", agent.ID, map[string]any{
				"output": status.Result,
			})

			// Audit: agent call completed
			_ = e.Audit.Log(ctx, audit.Entry{
				OrgID:        task.OrgID,
				TaskID:       task.ID,
				SubtaskID:    st.ID,
				AgentID:      agent.ID,
				ActorType:    "agent",
				ActorID:      agent.ID,
				Action:       "agent.completed",
				ResourceType: "subtask",
				ResourceID:   st.ID,
			})
			return

		case "failed":
			// Check retry
			var attempt int
			var maxAttempts int
			_ = e.DB.QueryRow(ctx,
				`SELECT attempt, max_attempts FROM subtasks WHERE id = $1`, st.ID).
				Scan(&attempt, &maxAttempts)

			if attempt < maxAttempts {
				// Retry: reset to pending, the DAG loop will re-pick it up
				log.Printf("executor: subtask %s failed (attempt %d/%d), retrying: %s", st.ID, attempt, maxAttempts, status.Error)
				_, _ = e.DB.Exec(ctx,
					`UPDATE subtasks SET status = 'pending', error = $1, poll_job_id = NULL, poll_endpoint = NULL WHERE id = $2`,
					status.Error, st.ID)
				e.publishEvent(ctx, task.ID, st.ID, "subtask.failed", "agent", agent.ID, map[string]any{
					"error":   status.Error,
					"attempt": attempt,
					"retried": true,
				})
				return
			}

			// Final failure
			e.failSubtask(ctx, task.ID, st.ID, status.Error, allSubtasks)
			return

		case "needs_input":
			// CRITICAL: Register signal channel BEFORE updating DB status
			inputCh := make(chan adapter.UserInput, 1)
			e.signals.Store(st.ID, inputCh)

			// Update DB status
			_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET status = 'waiting_for_input' WHERE id = $1`, st.ID)

			// Publish event
			var inputReqData map[string]any
			if status.InputRequest != nil {
				inputReqData = map[string]any{
					"message": status.InputRequest.Message,
					"options": status.InputRequest.Options,
				}
			}
			e.publishEvent(ctx, task.ID, st.ID, "subtask.waiting_for_input", "agent", agent.ID, inputReqData)

			// Post message to chat: "@user agent needs input"
			chatMsg := fmt.Sprintf("@user %s needs your input", agent.Name)
			if status.InputRequest != nil && status.InputRequest.Message != "" {
				chatMsg = fmt.Sprintf("@user %s: %s", agent.Name, status.InputRequest.Message)
			}
			e.publishMessage(ctx, task.ID, agent.ID, agent.Name, chatMsg)

			// Notify DAG loop that we're waiting
			select {
			case statusChangeCh <- st.ID:
			default:
			}

			// Block waiting for user input or cancellation
			select {
			case <-ctx.Done():
				e.signals.Delete(st.ID)
				return
			case userInput := <-inputCh:
				e.signals.Delete(st.ID)

				// Send input to agent
				if err := adp.SendInput(ctx, agent, handle, userInput); err != nil {
					log.Printf("executor: send input to subtask %s: %v", st.ID, err)
					e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("send input failed: %v", err), allSubtasks)
					return
				}

				// Resume polling
				_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET status = 'running' WHERE id = $1`, st.ID)
				e.publishEvent(ctx, task.ID, st.ID, "subtask.input_provided", "user", userInput.SubtaskID, map[string]any{
					"message": userInput.Message,
				})
				e.publishEvent(ctx, task.ID, st.ID, "subtask.running", "system", "", map[string]any{
					"resumed_after_input": true,
				})

				// Reset backoff
				intervalIdx = 0
			}
		}
	}
}

// Signal delivers user input to a subtask that is waiting for input.
func (e *DAGExecutor) Signal(ctx context.Context, taskID string, input adapter.UserInput) error {
	ch, ok := e.signals.Load(input.SubtaskID)
	if !ok {
		return ErrSubtaskNotWaiting
	}
	ch.(chan adapter.UserInput) <- input
	return nil
}

// Cancel cancels a running task by calling its cancel function.
func (e *DAGExecutor) Cancel(ctx context.Context, taskID string) error {
	cancelFn, ok := e.cancels.Load(taskID)
	if ok {
		cancelFn.(context.CancelFunc)()
	}

	_ = e.updateTaskStatus(ctx, taskID, "canceled", "canceled by user")

	// Cancel all running/pending subtasks
	_, _ = e.DB.Exec(ctx,
		`UPDATE subtasks SET status = 'canceled' WHERE task_id = $1 AND status IN ('pending', 'running', 'waiting_for_input')`,
		taskID)

	e.publishEvent(ctx, taskID, "", "task.canceled", "user", "", nil)
	return nil
}

// findReadySubtasks returns subtasks that are pending and have all dependencies completed.
func findReadySubtasks(subtasks []models.SubTask) []models.SubTask {
	// Build status map
	statusMap := make(map[string]string, len(subtasks))
	for _, st := range subtasks {
		statusMap[st.ID] = st.Status
	}

	var ready []models.SubTask
	for _, st := range subtasks {
		if st.Status != "pending" {
			continue
		}
		allDepsCompleted := true
		for _, depID := range st.DependsOn {
			if statusMap[depID] != "completed" {
				allDepsCompleted = false
				break
			}
		}
		if allDepsCompleted {
			ready = append(ready, st)
		}
	}
	return ready
}

// propagateBlocked marks all subtasks downstream of a failed subtask as blocked.
func (e *DAGExecutor) propagateBlocked(ctx context.Context, failedSubtaskID string, subtasks []models.SubTask) {
	// Find direct dependents
	var blocked []string
	for _, st := range subtasks {
		if st.Status == "pending" || st.Status == "running" {
			for _, depID := range st.DependsOn {
				if depID == failedSubtaskID {
					blocked = append(blocked, st.ID)
					break
				}
			}
		}
	}

	// Mark each as blocked and recursively propagate
	for _, blockedID := range blocked {
		_, _ = e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'blocked', error = 'upstream dependency failed' WHERE id = $1 AND status IN ('pending')`,
			blockedID)
		e.publishEvent(ctx, "", blockedID, "subtask.blocked", "system", "", map[string]any{
			"blocked_by": failedSubtaskID,
		})
		// Recursive: propagate to dependents of this blocked subtask
		e.propagateBlocked(ctx, blockedID, subtasks)
	}
}

// buildSubtaskInput builds the input map for a subtask, injecting upstream outputs.
func buildSubtaskInput(st models.SubTask, allSubtasks []models.SubTask, agents []models.Agent) map[string]any {
	input := map[string]any{
		"instruction": st.Instruction,
	}

	// Build agent ID → name lookup
	agentNames := make(map[string]string, len(agents))
	for _, a := range agents {
		agentNames[a.ID] = a.Name
	}

	// Inject upstream outputs keyed by agent name
	for _, depID := range st.DependsOn {
		for _, dep := range allSubtasks {
			if dep.ID == depID && dep.Output != nil {
				agentName := agentNames[dep.AgentID]
				if agentName == "" {
					agentName = dep.AgentID
				}
				var output any
				if err := json.Unmarshal(dep.Output, &output); err == nil {
					input["upstream_"+agentName] = output
				}
			}
		}
	}

	return input
}

// checkBudget verifies that the org hasn't exceeded its monthly budget.
func (e *DAGExecutor) checkBudget(ctx context.Context, orgID string, budget *float64) error {
	if budget == nil || *budget <= 0 {
		return nil // no budget set
	}

	spent, err := e.Audit.GetOrgMonthlySpend(ctx, orgID)
	if err != nil {
		return fmt.Errorf("check budget: %w", err) // fail closed
	}

	if spent >= *budget {
		return ErrBudgetExceeded
	}

	// Alert if approaching threshold (80% default)
	if spent >= *budget*0.8 {
		e.publishEvent(ctx, "", "", "budget.alert", "system", "", map[string]any{
			"org_id": orgID,
			"spent":  spent,
			"budget": *budget,
		})
	}

	return nil
}

// checkBudgetForOrg loads the org budget and checks it.
func (e *DAGExecutor) checkBudgetForOrg(ctx context.Context, orgID string) error {
	var budget *float64
	err := e.DB.QueryRow(ctx,
		`SELECT budget_usd_monthly FROM organizations WHERE id = $1`, orgID).
		Scan(&budget)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("load org budget: %w", err)
	}
	return e.checkBudget(ctx, orgID, budget)
}

// tryReplan attempts to replan a failed subtask.
// Returns true if replanning was successful and new subtasks were created.
func (e *DAGExecutor) tryReplan(
	ctx context.Context,
	task models.Task,
	failed models.SubTask,
	agents []models.Agent,
	currentSubtasks []models.SubTask,
) (bool, error) {
	// Check replan count
	var replanCount int
	err := e.DB.QueryRow(ctx,
		`SELECT replan_count FROM tasks WHERE id = $1`, task.ID).
		Scan(&replanCount)
	if err != nil {
		return false, fmt.Errorf("check replan count: %w", err)
	}

	if replanCount >= 2 {
		log.Printf("executor: task %s exhausted replans (%d/2)", task.ID, replanCount)
		return false, nil
	}

	// Increment replan count atomically
	_, err = e.DB.Exec(ctx,
		`UPDATE tasks SET replan_count = replan_count + 1 WHERE id = $1`, task.ID)
	if err != nil {
		return false, fmt.Errorf("increment replan count: %w", err)
	}

	// Call orchestrator for replan
	newPlan, err := e.Orchestrator.Replan(ctx, task, failed, agents)
	if err != nil {
		log.Printf("executor: replan failed for task %s: %v", task.ID, err)
		return false, err
	}

	// Remove blocked subtasks that will be replaced
	for _, st := range currentSubtasks {
		if st.Status == "blocked" {
			_, _ = e.DB.Exec(ctx, `DELETE FROM subtasks WHERE id = $1`, st.ID)
		}
	}

	// Also remove the failed subtask itself
	_, _ = e.DB.Exec(ctx, `DELETE FROM subtasks WHERE id = $1`, failed.ID)

	// Create new subtasks from replan
	_, err = e.createSubtasks(ctx, task.ID, newPlan, agents)
	if err != nil {
		log.Printf("executor: create replan subtasks failed for task %s: %v", task.ID, err)
		return false, err
	}

	e.publishEvent(ctx, task.ID, "", "task.replanned", "system", "", map[string]any{
		"replan_count":      replanCount + 1,
		"failed_subtask":    failed.ID,
		"new_subtask_count": len(newPlan.SubTasks),
	})

	return true, nil
}

// failSubtask marks a subtask as failed and propagates blocked status.
func (e *DAGExecutor) failSubtask(ctx context.Context, taskID, subtaskID, errMsg string, allSubtasks []models.SubTask) {
	_, _ = e.DB.Exec(ctx,
		`UPDATE subtasks SET status = 'failed', error = $1 WHERE id = $2`,
		errMsg, subtaskID)

	e.publishEvent(ctx, taskID, subtaskID, "subtask.failed", "system", "", map[string]any{
		"error":   errMsg,
		"retried": false,
	})

	// Propagate blocked status to downstream subtasks
	e.propagateBlocked(ctx, subtaskID, allSubtasks)
}

// loadSubtasks loads all subtasks for a task from the database.
func (e *DAGExecutor) loadSubtasks(ctx context.Context, taskID string) ([]models.SubTask, error) {
	rows, err := e.DB.Query(ctx,
		`SELECT id, task_id, agent_id, instruction, COALESCE(depends_on, '{}'), status,
		        input, output, COALESCE(error, ''), COALESCE(poll_job_id, ''), COALESCE(poll_endpoint, ''),
		        attempt, max_attempts, created_at, started_at, completed_at
		 FROM subtasks WHERE task_id = $1 ORDER BY created_at`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query subtasks: %w", err)
	}
	defer rows.Close()

	var subtasks []models.SubTask
	for rows.Next() {
		var st models.SubTask
		err := rows.Scan(
			&st.ID, &st.TaskID, &st.AgentID, &st.Instruction, &st.DependsOn, &st.Status,
			&st.Input, &st.Output, &st.Error, &st.PollJobID, &st.PollEndpoint,
			&st.Attempt, &st.MaxAttempts, &st.CreatedAt, &st.StartedAt, &st.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("scan subtask: %w", err)
		}
		subtasks = append(subtasks, st)
	}
	return subtasks, rows.Err()
}

// loadOrgAgents loads all active agents for an organization.
func (e *DAGExecutor) loadOrgAgents(ctx context.Context, orgID string) ([]models.Agent, error) {
	rows, err := e.DB.Query(ctx,
		`SELECT id, org_id, name, COALESCE(version,''), COALESCE(description,''), endpoint,
		        adapter_type, adapter_config, COALESCE(auth_type,'none'), auth_config,
		        COALESCE(capabilities, '{}'), input_schema, output_schema, config,
		        COALESCE(status,'active'), created_at, updated_at
		 FROM agents WHERE org_id = $1 AND status = 'active'`, orgID)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		err := rows.Scan(
			&a.ID, &a.OrgID, &a.Name, &a.Version, &a.Description, &a.Endpoint,
			&a.AdapterType, &a.AdapterConfig, &a.AuthType, &a.AuthConfig,
			&a.Capabilities, &a.InputSchema, &a.OutputSchema, &a.Config,
			&a.Status, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// updateTaskStatus updates the task status and error in the database.
func (e *DAGExecutor) updateTaskStatus(ctx context.Context, taskID, status, errMsg string) error {
	_, err := e.DB.Exec(ctx,
		`UPDATE tasks SET status = $1, error = NULLIF($2, '') WHERE id = $3`,
		status, errMsg, taskID)
	return err
}

// publishEvent persists an event and broadcasts it to SSE subscribers.
func (e *DAGExecutor) publishEvent(ctx context.Context, taskID, subtaskID, eventType, actorType, actorID string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	evt, err := e.EventStore.Save(ctx, taskID, subtaskID, eventType, actorType, actorID, data)
	if err != nil {
		log.Printf("executor: save event %s: %v", eventType, err)
		return
	}
	e.Broker.Publish(evt)
}

// publishMessage inserts a message into the messages table and publishes it as an event.
func (e *DAGExecutor) publishMessage(ctx context.Context, taskID, senderID, senderName, content string) {
	msgID := uuid.New().String()
	now := time.Now()

	_, err := e.DB.Exec(ctx,
		`INSERT INTO messages (id, task_id, sender_type, sender_id, sender_name, content, mentions, created_at)
		 VALUES ($1, $2, 'agent', $3, $4, $5, '{}', $6)`,
		msgID, taskID, senderID, senderName, content, now)
	if err != nil {
		log.Printf("executor: insert message: %v", err)
		return
	}

	e.publishEvent(ctx, taskID, "", "message", "agent", senderID, map[string]any{
		"message_id":  msgID,
		"sender_name": senderName,
		"content":     content,
	})
}
