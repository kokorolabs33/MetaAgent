// Package executor implements the DAG execution engine for task orchestration.
//
// The DAGExecutor takes an execution plan (DAG of subtasks) and runs subtasks
// in dependency order. It uses the A2A protocol to communicate with agents,
// handles retries, human-in-the-loop input via A2A input-required state,
// blocked propagation, replanning, and cancellation.
package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/a2a"
	"taskhub/internal/audit"
	"taskhub/internal/events"
	"taskhub/internal/models"
	"taskhub/internal/orchestrator"
	"taskhub/internal/policy"
	"taskhub/internal/webhook"
)

// Sentinel errors.
var (
	ErrTaskCanceled = errors.New("task was canceled")
)

// DAGExecutor manages the lifecycle of task execution.
type DAGExecutor struct {
	DB            *pgxpool.Pool
	Broker        *events.Broker
	EventStore    *events.Store
	Audit         *audit.Logger
	Orchestrator  *orchestrator.Orchestrator
	A2AClient     *a2a.Client
	PolicyEngine  *policy.Engine
	WebhookSender *webhook.Sender

	cancels sync.Map // task_id → context.CancelFunc

	agentRunningCount sync.Map // agent_id (string) → *int32 (atomic counter)

	maxConcurrent      int // global max concurrent subtasks (default 10)
	maxConcurrentAgent int // per-agent max concurrent subtasks (default 3)
}

// getMaxConcurrent returns the global concurrency limit, defaulting to 10.
func (e *DAGExecutor) getMaxConcurrent() int {
	if e.maxConcurrent > 0 {
		return e.maxConcurrent
	}
	return 10
}

// getMaxConcurrentAgent returns the per-agent concurrency limit, defaulting to 3.
func (e *DAGExecutor) getMaxConcurrentAgent() int {
	if e.maxConcurrentAgent > 0 {
		return e.maxConcurrentAgent
	}
	return 3
}

// incrAgentRunning increments the running count for an agent.
// If the count transitions from 0->1, publishes agent.status_changed with activity_status="working".
func (e *DAGExecutor) incrAgentRunning(agentID, agentName string) {
	val, _ := e.agentRunningCount.LoadOrStore(agentID, new(int32))
	counter := val.(*int32)
	newVal := atomic.AddInt32(counter, 1)
	if newVal == 1 {
		e.publishAgentStatus(agentID, agentName, "working")
	}
}

// decrAgentRunning decrements the running count for an agent.
// If the count transitions from 1->0, publishes agent.status_changed with activity_status="idle".
func (e *DAGExecutor) decrAgentRunning(agentID, agentName string) {
	val, ok := e.agentRunningCount.Load(agentID)
	if !ok {
		return
	}
	counter := val.(*int32)
	newVal := atomic.AddInt32(counter, -1)
	if newVal == 0 {
		e.publishAgentStatus(agentID, agentName, "idle")
	}
}

// publishAgentStatus publishes an agent.status_changed event to the global "agents" Broker topic.
// These events are NOT persisted to the event store (per D-06 — derive at runtime only).
func (e *DAGExecutor) publishAgentStatus(agentID, agentName, activityStatus string) {
	dataJSON, _ := json.Marshal(map[string]any{
		"agent_id":        agentID,
		"agent_name":      agentName,
		"activity_status": activityStatus,
	})
	evt := &models.Event{
		ID:        uuid.NewString(),
		Type:      "agent.status_changed",
		ActorType: "system",
		Data:      dataJSON,
		CreatedAt: time.Now(),
	}
	e.Broker.PublishGlobal("agents", evt)
}

// Execute is the main entry point for task execution.
// It plans the task via the orchestrator, creates subtask records, and runs the DAG loop.
func (e *DAGExecutor) Execute(ctx context.Context, task models.Task) error {
	// 1. Update task status to "planning"
	if err := e.updateTaskStatus(ctx, task.ID, "planning", ""); err != nil {
		return fmt.Errorf("update task to planning: %w", err)
	}
	e.publishEvent(ctx, task.ID, "", "task.planning", "system", "", nil)
	e.publishSystemMessage(ctx, task.ID, "Planning task: analyzing and decomposing into subtasks...")

	// 2. Load all active agents
	agents, err := e.loadAgents(ctx)
	if err != nil {
		return fmt.Errorf("load agents: %w", err)
	}
	if len(agents) == 0 {
		errMsg := "no active agents available"
		_ = e.updateTaskStatus(ctx, task.ID, "failed", errMsg)
		e.publishEvent(ctx, task.ID, "", "task.failed", "system", "", map[string]any{"error": errMsg})
		return errors.New(errMsg)
	}

	// 4. Evaluate policies
	var policyConstraints string
	var appliedPolicies []string
	var policyResult *policy.EvalResult
	if e.PolicyEngine != nil {
		evalResult, evalErr := e.PolicyEngine.Evaluate(ctx, task.Title, task.Description)
		if evalErr != nil {
			log.Printf("executor: policy evaluation failed: %v", evalErr)
			// Non-fatal — proceed without policy constraints
		} else {
			policyResult = evalResult
			policyConstraints = evalResult.FormatForPrompt()
			appliedPolicies = evalResult.AppliedPolicies
		}
	}

	// Publish policy.applied event for observability
	if len(appliedPolicies) > 0 {
		e.publishEvent(ctx, task.ID, "", "policy.applied", "system", "", map[string]any{
			"policies": appliedPolicies,
		})
	}

	// 4b. Load template skeleton if task references a template
	var templateSkeleton string
	if task.TemplateID != "" {
		var stepsJSON []byte
		var tmplVersion int
		tmplErr := e.DB.QueryRow(ctx,
			`SELECT version, steps FROM workflow_templates WHERE id = $1 AND is_active = true`,
			task.TemplateID).Scan(&tmplVersion, &stepsJSON)
		if tmplErr == nil {
			templateSkeleton = fmt.Sprintf("\n[Template Skeleton (v%d)]\nUse this as a guide. You may adjust instructions and add auxiliary steps, but keep mandatory steps:\n%s", tmplVersion, string(stepsJSON))
			// Update task with template version used
			_, _ = e.DB.Exec(ctx, `UPDATE tasks SET template_version = $1 WHERE id = $2`, tmplVersion, task.ID)

			// Publish template.matched event for observability
			e.publishEvent(ctx, task.ID, "", "template.matched", "system", "", map[string]any{
				"template_id": task.TemplateID,
				"version":     tmplVersion,
			})
		} else {
			log.Printf("executor: template %s not found or inactive: %v", task.TemplateID, tmplErr)
		}
	}

	// 5. Call orchestrator to create plan
	plan, err := e.Orchestrator.Plan(ctx, task, agents, policyConstraints, templateSkeleton)
	if err != nil {
		errMsg := fmt.Sprintf("planning failed: %v", err)
		_ = e.updateTaskStatus(ctx, task.ID, "failed", errMsg)
		e.publishEvent(ctx, task.ID, "", "task.failed", "system", "", map[string]any{"error": errMsg})
		e.recordTemplateExecution(ctx, task.ID, task.TemplateID, task.TemplateVersion, "failed")
		return fmt.Errorf("orchestrator plan: %w", err)
	}

	// 6. Store plan in task record
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	// Check if subtask count requires approval before proceeding
	if policyResult != nil && policyResult.RequireApprovalAboveSubtasks > 0 && len(plan.SubTasks) > policyResult.RequireApprovalAboveSubtasks {
		_, err = e.DB.Exec(ctx,
			`UPDATE tasks SET plan = $1, status = 'approval_required' WHERE id = $2`,
			planJSON, task.ID)
		if err != nil {
			return fmt.Errorf("store plan for approval: %w", err)
		}

		// Store applied policies
		if len(appliedPolicies) > 0 {
			policyJSON, marshalErr := json.Marshal(appliedPolicies)
			if marshalErr == nil {
				_, _ = e.DB.Exec(ctx, `UPDATE tasks SET policy_applied = $1 WHERE id = $2`, policyJSON, task.ID)
			}
		}

		e.publishEvent(ctx, task.ID, "", "approval.requested", "system", "", map[string]any{
			"subtask_count": len(plan.SubTasks),
			"threshold":     policyResult.RequireApprovalAboveSubtasks,
		})
		e.publishSystemMessage(ctx, task.ID,
			fmt.Sprintf("Plan has %d subtasks (threshold: %d). Awaiting approval.", len(plan.SubTasks), policyResult.RequireApprovalAboveSubtasks))

		// Don't proceed with execution — wait for approval via the approve endpoint
		return nil
	}

	_, err = e.DB.Exec(ctx,
		`UPDATE tasks SET plan = $1, status = 'running' WHERE id = $2`,
		planJSON, task.ID)
	if err != nil {
		return fmt.Errorf("store plan: %w", err)
	}
	e.publishEvent(ctx, task.ID, "", "task.planned", "system", "", map[string]any{"summary": plan.Summary})
	e.publishSystemMessage(ctx, task.ID, fmt.Sprintf("Plan ready: %s (%d subtasks)", plan.Summary, len(plan.SubTasks)))

	// Store applied policies on the task
	if len(appliedPolicies) > 0 {
		policyJSON, marshalErr := json.Marshal(appliedPolicies)
		if marshalErr == nil {
			_, _ = e.DB.Exec(ctx, `UPDATE tasks SET policy_applied = $1 WHERE id = $2`, policyJSON, task.ID)
		}
	}

	// 7–8. Create subtask records and run DAG loop
	return e.executePlan(ctx, task, plan, agents)
}

// executePlan creates subtask records from a plan and runs the DAG loop.
// Shared by Execute (normal flow) and ResumeApproved (after approval).
func (e *DAGExecutor) executePlan(ctx context.Context, task models.Task, plan *models.ExecutionPlan, agents []models.Agent) error {
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

	return e.runDAGLoop(ctx, task, subtasks, agents)
}

// ResumeApproved resumes a task that was approved after approval_required state.
// It loads the stored plan from the database and proceeds with subtask creation and DAG execution.
func (e *DAGExecutor) ResumeApproved(ctx context.Context, taskID string) error {
	// Load the task (includes the stored plan JSON)
	task, err := e.loadTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}

	if task.Plan == nil || len(task.Plan) == 0 {
		return fmt.Errorf("task %s has no stored plan", taskID)
	}

	var plan models.ExecutionPlan
	if err := json.Unmarshal(task.Plan, &plan); err != nil {
		return fmt.Errorf("parse stored plan: %w", err)
	}

	// Update status to running
	if err := e.updateTaskStatus(ctx, taskID, "running", ""); err != nil {
		return fmt.Errorf("update task to running: %w", err)
	}

	e.publishEvent(ctx, taskID, "", "approval.resolved", "user", "", map[string]any{"action": "approved"})
	e.publishSystemMessage(ctx, taskID, "Plan approved. Starting execution...")

	// Load agents for agent ID resolution
	agents, err := e.loadAgents(ctx)
	if err != nil {
		return fmt.Errorf("load agents: %w", err)
	}

	return e.executePlan(ctx, *task, &plan, agents)
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

		// Compute per-agent running counts from freshly loaded subtasks
		agentRunning := make(map[string]int)
		for i := range subtasks {
			if subtasks[i].Status == "running" {
				agentRunning[subtasks[i].AgentID]++
			}
		}

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
			case "running", "input_required":
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

			// Aggregate all subtask outputs into task result
			taskResult := e.aggregateResults(taskCtx, task.ID, subtasks)
			if _, err := e.DB.Exec(taskCtx,
				`UPDATE tasks SET status = 'completed', completed_at = $1, result = $2 WHERE id = $3`,
				now, taskResult, task.ID); err != nil {
				log.Printf("executor: mark task %s completed: %v", task.ID, err)
			}
			e.publishEvent(taskCtx, task.ID, "", "task.completed", "system", "", map[string]any{"result": taskResult})
			e.publishSystemMessage(taskCtx, task.ID, "All subtasks completed. Task finished.")
			e.recordTemplateExecution(taskCtx, task.ID, task.TemplateID, task.TemplateVersion, "completed")
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
				e.recordTemplateExecution(taskCtx, task.ID, task.TemplateID, task.TemplateVersion, "failed")
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
			e.incrAgentRunning(st.AgentID, agent.Name)

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
			e.recordTemplateExecution(ctx, task.ID, task.TemplateID, task.TemplateVersion, "canceled")
			wg.Wait()
			return ErrTaskCanceled
		case <-statusChangeCh:
			// A subtask changed status, re-evaluate the DAG
			continue
		}
	}
}

// runSubtask handles the lifecycle of a single subtask via A2A SendMessage.
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

	// Build input with upstream outputs
	inputMap := buildSubtaskInput(st, allSubtasks, agents)
	inputJSON, err := json.Marshal(inputMap)
	if err != nil {
		e.decrAgentRunning(st.AgentID, agent.Name)
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("marshal subtask input: %v", err), allSubtasks)
		return
	}

	// Store input in DB
	if _, err := e.DB.Exec(ctx, `UPDATE subtasks SET input = $1 WHERE id = $2`, inputJSON, st.ID); err != nil {
		log.Printf("executor: store input for subtask %s: %v", st.ID, err)
	}

	// Build A2A message parts
	parts := []a2a.MessagePart{a2a.TextPart(st.Instruction)}

	// If there are upstream outputs, include them as a data part
	upstreamData := make(map[string]any)
	for key, val := range inputMap {
		if strings.HasPrefix(key, "upstream_") {
			upstreamData[key] = val
		}
	}
	if len(upstreamData) > 0 {
		parts = append(parts, a2a.DataPart(upstreamData))
	}

	// Audit: agent call submitted
	_ = e.Audit.Log(ctx, audit.Entry{
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

	// System message: agent started (truncate instruction for readability)
	instrSummary := st.Instruction
	if len(instrSummary) > 100 {
		instrSummary = instrSummary[:97] + "..."
	}
	e.publishSystemMessage(ctx, task.ID, fmt.Sprintf("%s started working on: %s", agent.Name, instrSummary))

	// Send A2A message to agent
	result, err := e.A2AClient.SendMessage(ctx, agent.Endpoint, task.ID, "", parts)
	if err != nil {
		e.decrAgentRunning(st.AgentID, agent.Name)
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("a2a send failed: %v", err), allSubtasks)
		return
	}

	// Store A2A task ID for future follow-up
	if result.TaskID != "" {
		if _, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET a2a_task_id = $1 WHERE id = $2`,
			result.TaskID, st.ID); err != nil {
			log.Printf("executor: store a2a_task_id for subtask %s: %v", st.ID, err)
		}
	}

	// Poll async agents until they reach a terminal state
	result = e.pollUntilTerminal(ctx, agent.Endpoint, result, task.ID, st.ID, agent.ID)

	// Handle result state
	e.handleA2AResult(ctx, task, st, agent, result, allSubtasks, statusChangeCh)
}

// handleA2AResult processes the result from an A2A SendMessage call.
func (e *DAGExecutor) handleA2AResult(
	ctx context.Context,
	task models.Task,
	st models.SubTask,
	agent models.Agent,
	result *a2a.SendResult,
	allSubtasks []models.SubTask,
	statusChangeCh chan<- string,
) {
	switch result.State {
	case "completed":
		now := time.Now()
		output := result.Artifacts
		if output == nil {
			output = json.RawMessage(`null`)
		}
		_, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2 WHERE id = $3`,
			output, now, st.ID)
		if err != nil {
			log.Printf("executor: update subtask %s to completed: %v", st.ID, err)
		}
		e.decrAgentRunning(st.AgentID, agent.Name)

		e.publishEvent(ctx, task.ID, st.ID, "subtask.completed", "agent", agent.ID, map[string]any{
			"output": output,
		})

		// Publish agent's output as a chat message
		if len(result.Artifacts) > 0 {
			outputStr := string(result.Artifacts)
			if len(outputStr) > 2 && outputStr[0] == '"' && outputStr[len(outputStr)-1] == '"' {
				var unquoted string
				if json.Unmarshal(result.Artifacts, &unquoted) == nil {
					outputStr = unquoted
				}
			}

			// Try to detect structured artifacts in the output
			metadata := detectArtifactMetadata(outputStr)
			if metadata != nil {
				e.publishMessageWithMetadata(ctx, task.ID, agent.ID, agent.Name, outputStr, metadata)
			} else {
				e.publishMessage(ctx, task.ID, agent.ID, agent.Name, outputStr)
			}
		}
		e.publishSystemMessage(ctx, task.ID, fmt.Sprintf("%s completed the task", agent.Name))

		// Audit: agent call completed
		_ = e.Audit.Log(ctx, audit.Entry{
			TaskID:       task.ID,
			SubtaskID:    st.ID,
			AgentID:      agent.ID,
			ActorType:    "agent",
			ActorID:      agent.ID,
			Action:       "agent.completed",
			ResourceType: "subtask",
			ResourceID:   st.ID,
		})

	case "failed":
		// Check retry
		var attempt int
		var maxAttempts int
		_ = e.DB.QueryRow(ctx,
			`SELECT attempt, max_attempts FROM subtasks WHERE id = $1`, st.ID).
			Scan(&attempt, &maxAttempts)

		if attempt < maxAttempts {
			log.Printf("executor: subtask %s failed (attempt %d/%d), retrying: %s", st.ID, attempt, maxAttempts, result.Error)
			if _, err := e.DB.Exec(ctx,
				`UPDATE subtasks SET status = 'pending', error = $1, a2a_task_id = '' WHERE id = $2`,
				result.Error, st.ID); err != nil {
				log.Printf("executor: retry reset for subtask %s: %v", st.ID, err)
			}
			e.decrAgentRunning(st.AgentID, agent.Name)
			e.publishEvent(ctx, task.ID, st.ID, "subtask.failed", "agent", agent.ID, map[string]any{
				"error":   result.Error,
				"attempt": attempt,
				"retried": true,
			})
			return
		}

		// Final failure
		e.decrAgentRunning(st.AgentID, agent.Name)
		e.failSubtask(ctx, task.ID, st.ID, result.Error, allSubtasks)

	case "input-required":
		// Update DB status to input_required
		if _, err := e.DB.Exec(ctx, `UPDATE subtasks SET status = 'input_required' WHERE id = $1`, st.ID); err != nil {
			log.Printf("executor: set input_required for subtask %s: %v", st.ID, err)
		}
		e.decrAgentRunning(st.AgentID, agent.Name)

		// Publish event
		e.publishEvent(ctx, task.ID, st.ID, "subtask.input_required", "agent", agent.ID, map[string]any{
			"message": result.Message,
		})

		// Post message to chat
		chatMsg := fmt.Sprintf("@user %s needs your input", agent.Name)
		if result.Message != "" {
			chatMsg = fmt.Sprintf("@user %s: %s", agent.Name, result.Message)
		}
		e.publishMessage(ctx, task.ID, agent.ID, agent.Name, chatMsg)

		// Notify DAG loop that we're in input_required state
		select {
		case statusChangeCh <- st.ID:
		default:
		}

	default:
		// Unknown state — treat as failure
		log.Printf("executor: subtask %s returned unexpected state %q", st.ID, result.State)
		e.decrAgentRunning(st.AgentID, agent.Name)
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("unexpected agent state: %s", result.State), allSubtasks)
	}
}

// pollUntilTerminal polls an async A2A agent until the task reaches a terminal
// or actionable state (completed, failed, input-required, canceled). If the
// result is already terminal it returns immediately. The poll uses a 2-second
// interval with a 5-minute timeout.
func (e *DAGExecutor) pollUntilTerminal(ctx context.Context, agentURL string, result *a2a.SendResult, parentTaskID, subtaskID, agentID string) *a2a.SendResult {
	if result.State != "working" && result.State != "submitted" {
		return result
	}

	const pollInterval = 2 * time.Second
	const pollTimeout = 5 * time.Minute

	deadline := time.Now().Add(pollTimeout)
	taskID := result.TaskID

	log.Printf("executor: polling agent %s for task %s (state=%s)", agentURL, taskID, result.State)

	var lastToolProgress string

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return result // canceled
		case <-time.After(pollInterval):
		}

		pollResult, err := e.A2AClient.GetTask(ctx, agentURL, taskID)
		if err != nil {
			log.Printf("executor: poll task %s: %v", taskID, err)
			continue // transient error, keep polling
		}

		result = pollResult

		// Check for tool call progress markers during polling (D-07: real-time tool visibility)
		if result.State == "working" && result.Message != "" && result.Message != lastToolProgress {
			e.checkToolProgress(ctx, parentTaskID, subtaskID, agentID, result)
			lastToolProgress = result.Message
		}

		if result.State != "working" && result.State != "submitted" {
			log.Printf("executor: task %s reached state %q", taskID, result.State)
			return result
		}
	}

	// Timed out — synthesize a failure result
	log.Printf("executor: task %s timed out after %v", taskID, pollTimeout)
	result.State = "failed"
	result.Error = fmt.Sprintf("agent task timed out after %v", pollTimeout)
	return result
}

// checkToolProgress parses tool call progress markers from an A2A status message
// and publishes corresponding SSE events (tool.call_started / tool.call_completed).
// Per T-07-07: markers use a fixed prefix format that cannot be spoofed by regular text.
func (e *DAGExecutor) checkToolProgress(ctx context.Context, taskID, subtaskID, agentID string, result *a2a.SendResult) {
	if result.State != "working" || result.Message == "" {
		return
	}
	msg := result.Message

	if strings.HasPrefix(msg, "tool_call_started:") {
		parts := strings.SplitN(msg[len("tool_call_started:"):], ":", 2)
		if len(parts) == 2 {
			toolName := parts[0]
			toolArgs := parts[1]
			e.publishEvent(ctx, taskID, subtaskID, "tool.call_started", "agent", agentID, map[string]any{
				"tool_name": toolName,
				"args":      toolArgs,
			})
		}
	} else if strings.HasPrefix(msg, "tool_call_completed:") {
		parts := strings.SplitN(msg[len("tool_call_completed:"):], ":", 2)
		if len(parts) == 2 {
			toolName := parts[0]
			summary := parts[1]
			e.publishEvent(ctx, taskID, subtaskID, "tool.call_completed", "agent", agentID, map[string]any{
				"tool_name": toolName,
				"summary":   summary,
			})
		}
	}
}

// SendFollowUp sends a follow-up message to an agent for an existing subtask.
// Used for @mention routing when a user sends a message to an agent.
func (e *DAGExecutor) SendFollowUp(ctx context.Context, taskID, subtaskID, agentID, content string) error {
	// Load the subtask to get the a2a_task_id
	var a2aTaskID string
	var agentEndpoint string
	err := e.DB.QueryRow(ctx,
		`SELECT s.a2a_task_id, a.endpoint
		 FROM subtasks s JOIN agents a ON a.id = s.agent_id
		 WHERE s.id = $1`, subtaskID).
		Scan(&a2aTaskID, &agentEndpoint)
	if err != nil {
		return fmt.Errorf("load subtask: %w", err)
	}

	parts := []a2a.MessagePart{a2a.TextPart(content)}

	result, err := e.A2AClient.SendMessage(ctx, agentEndpoint, taskID, a2aTaskID, parts)
	if err != nil {
		return fmt.Errorf("a2a follow-up failed: %w", err)
	}

	// Update A2A task ID if it changed
	if result.TaskID != "" && result.TaskID != a2aTaskID {
		if _, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET a2a_task_id = $1 WHERE id = $2`,
			result.TaskID, subtaskID); err != nil {
			log.Printf("executor: update a2a_task_id for subtask %s: %v", subtaskID, err)
		}
	}

	// Poll async agents until they reach a terminal state
	result = e.pollUntilTerminal(ctx, agentEndpoint, result, taskID, subtaskID, agentID)

	// Handle the result
	switch result.State {
	case "completed":
		now := time.Now()
		output := result.Artifacts
		if output == nil {
			output = json.RawMessage(`null`)
		}
		if _, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2 WHERE id = $3`,
			output, now, subtaskID); err != nil {
			log.Printf("executor: mark follow-up subtask %s completed: %v", subtaskID, err)
		}

		// Publish subtask.completed event so frontend updates DAG + banner
		e.publishEvent(ctx, taskID, subtaskID, "subtask.completed", "agent", agentID, map[string]any{
			"output": output,
		})

		// Publish completion to chat
		var agentName string
		_ = e.DB.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, agentID).Scan(&agentName)
		if agentName == "" {
			agentName = agentID
		}
		if len(result.Artifacts) > 0 {
			outputStr := string(result.Artifacts)
			if len(outputStr) > 2 && outputStr[0] == '"' && outputStr[len(outputStr)-1] == '"' {
				var unquoted string
				if json.Unmarshal(result.Artifacts, &unquoted) == nil {
					outputStr = unquoted
				}
			}

			// Try to detect structured artifacts in the output
			metadata := detectArtifactMetadata(outputStr)
			if metadata != nil {
				e.publishMessageWithMetadata(ctx, taskID, agentID, agentName, outputStr, metadata)
			} else {
				e.publishMessage(ctx, taskID, agentID, agentName, outputStr)
			}
		}
		e.publishSystemMessage(ctx, taskID, fmt.Sprintf("%s completed the task", agentName))

	case "input-required":
		if _, err := e.DB.Exec(ctx, `UPDATE subtasks SET status = 'input_required' WHERE id = $1`, subtaskID); err != nil {
			log.Printf("executor: set input_required for follow-up subtask %s: %v", subtaskID, err)
		}

	case "failed":
		if _, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'failed', error = $1 WHERE id = $2`,
			result.Error, subtaskID); err != nil {
			log.Printf("executor: mark follow-up subtask %s failed: %v", subtaskID, err)
		}
	}

	return nil
}

// SendAdvisory sends an advisory message to an agent without modifying subtask status.
// Used for @mention routing when a user sends an advisory message during task execution.
// NOTE: No ctx parameter — creates its own background context per D-16.
func (e *DAGExecutor) SendAdvisory(taskID, subtaskID, agentID, content string) {
	// Create isolated context with 60s timeout (D-14, D-16)
	advCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Look up agent name for typing indicators and error messages
	var agentName string
	_ = e.DB.QueryRow(advCtx, `SELECT name FROM agents WHERE id = $1`, agentID).Scan(&agentName)
	if agentName == "" {
		agentName = agentID
	}

	// Publish transient typing indicator via Broker-only (D-09)
	e.publishTransientEvent(taskID, "agent.typing", map[string]any{
		"agent_id":   agentID,
		"agent_name": agentName,
	})

	// Always clear typing indicator on exit (Pitfall 5 prevention)
	defer e.publishTransientEvent(taskID, "agent.typing_stopped", map[string]any{
		"agent_id": agentID,
	})

	// Build advisory context (D-04): user message + subtask description + previous output
	var instruction string
	var prevOutput string
	_ = e.DB.QueryRow(advCtx,
		`SELECT instruction, COALESCE(output::text, '') FROM subtasks WHERE id = $1`,
		subtaskID).Scan(&instruction, &prevOutput)

	advisoryContent := fmt.Sprintf("Advisory message from user: %s\n\nYour current task: %s", content, instruction)
	if prevOutput != "" && prevOutput != "null" {
		if len(prevOutput) > 2000 {
			prevOutput = prevOutput[:2000] + "... (truncated)"
		}
		advisoryContent += fmt.Sprintf("\n\nYour previous output: %s", prevOutput)
	}

	// Load agent endpoint (we intentionally do NOT use the subtask's a2a_task_id —
	// sending to the same A2A task would cause the main executor's poll loop to see
	// the advisory completion as a subtask completion, triggering "completed the task")
	var agentEndpoint string
	err := e.DB.QueryRow(advCtx,
		`SELECT a.endpoint FROM subtasks s JOIN agents a ON a.id = s.agent_id WHERE s.id = $1`,
		subtaskID).Scan(&agentEndpoint)
	if err != nil {
		log.Printf("advisory: load subtask %s: %v", subtaskID, err)
		e.publishAdvisoryError(advCtx, taskID, agentID, agentName, "could not reach the agent")
		return
	}

	parts := []a2a.MessagePart{a2a.TextPart(advisoryContent)}

	// Send A2A message with a NEW advisory-specific contextID and empty taskID.
	// This creates a separate A2A task on the agent, isolated from the running subtask.
	advContextID := fmt.Sprintf("advisory-%s-%s-%d", taskID, subtaskID, time.Now().UnixMilli())
	result, err := e.A2AClient.SendMessage(advCtx, agentEndpoint, advContextID, "", parts)
	if err != nil {
		// Retry once (D-14)
		result, err = e.A2AClient.SendMessage(advCtx, agentEndpoint, advContextID, "", parts)
		if err != nil {
			log.Printf("advisory: a2a send to agent %s failed after retry: %v", agentID, err)
			e.publishAdvisoryError(advCtx, taskID, agentID, agentName, "did not respond to the advisory message")
			return
		}
	}

	// Poll if async
	if result.State == "working" || result.State == "submitted" {
		result = e.pollUntilTerminal(advCtx, agentEndpoint, result, taskID, subtaskID, agentID)
	}

	// Publish advisory response as chat message (NOT subtask status change)
	if result.State == "completed" && len(result.Artifacts) > 0 {
		e.publishAdvisoryMessage(advCtx, taskID, agentID, agentName, result.Artifacts, subtaskID)
	} else if result.State == "failed" {
		e.publishAdvisoryError(advCtx, taskID, agentID, agentName, "encountered an error processing the advisory")
	}
}

// publishAdvisoryMessage inserts an advisory response as a chat message with advisory metadata.
func (e *DAGExecutor) publishAdvisoryMessage(ctx context.Context, taskID, senderID, senderName string, artifacts json.RawMessage, subtaskID string) {
	// Extract text content from artifacts
	content := string(artifacts)
	if len(content) > 2 && content[0] == '"' && content[len(content)-1] == '"' {
		var unquoted string
		if json.Unmarshal(artifacts, &unquoted) == nil {
			content = unquoted
		}
	}
	if len(content) > 5000 {
		content = content[:5000] + "... (truncated)"
	}

	msgID := uuid.New().String()
	now := time.Now()

	// Look up conversation_id for the task
	var conversationID string
	_ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)

	metadata, _ := json.Marshal(map[string]any{
		"advisory":             true,
		"advisory_for_subtask": subtaskID,
	})

	_, err := e.DB.Exec(ctx,
		`INSERT INTO messages (id, task_id, conversation_id, sender_type, sender_id, sender_name, content, mentions, metadata, created_at)
		 VALUES ($1, $2, $3, 'agent', $4, $5, $6, '{}', $7, $8)`,
		msgID, taskID, conversationID, senderID, senderName, content, metadata, now)
	if err != nil {
		log.Printf("executor: insert advisory message: %v", err)
		return
	}

	e.publishEvent(ctx, taskID, "", "message", "agent", senderID, map[string]any{
		"message_id":  msgID,
		"sender_name": senderName,
		"sender_type": "agent",
		"content":     content,
		"metadata":    map[string]any{"advisory": true, "advisory_for_subtask": subtaskID},
	})
}

// publishAdvisoryError publishes an advisory error as an inline system message.
func (e *DAGExecutor) publishAdvisoryError(ctx context.Context, taskID, agentID, agentName, reason string) {
	errorContent := fmt.Sprintf("%s %s", agentName, reason)
	e.publishSystemMessage(ctx, taskID, errorContent)
}

// publishTransientEvent publishes an event via Broker only (NOT EventStore.Save).
// Used for ephemeral events like typing indicators that should not be replayed on reconnect.
func (e *DAGExecutor) publishTransientEvent(taskID, eventType string, data map[string]any) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("executor: marshal transient event data: %v", err)
		return
	}

	// Look up conversation_id for the task so event routes to conversation subscribers too
	var conversationID string
	if taskID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)
	}

	evt := &models.Event{
		ID:             uuid.NewString(),
		TaskID:         taskID,
		ConversationID: conversationID,
		Type:           eventType,
		ActorType:      "system",
		Data:           dataJSON,
		CreatedAt:      time.Now(),
	}

	// Broker-only — do NOT call EventStore.Save
	e.Broker.Publish(evt)
}

// Cancel cancels a running task by calling its cancel function.
func (e *DAGExecutor) Cancel(ctx context.Context, taskID string) error {
	cancelFn, ok := e.cancels.Load(taskID)
	if ok {
		cancelFn.(context.CancelFunc)()
	}

	_ = e.updateTaskStatus(ctx, taskID, "canceled", "canceled by user")

	// Cancel all running/pending/input_required subtasks
	if _, err := e.DB.Exec(ctx,
		`UPDATE subtasks SET status = 'canceled' WHERE task_id = $1 AND status IN ('pending', 'running', 'input_required')`,
		taskID); err != nil {
		log.Printf("executor: cancel subtasks for task %s: %v", taskID, err)
	}

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

// propagateBlocked marks all pending subtasks downstream of a failed subtask as blocked.
// Running subtasks are not blocked — they continue until they naturally complete or fail.
func (e *DAGExecutor) propagateBlocked(ctx context.Context, failedSubtaskID string, subtasks []models.SubTask) {
	// Find direct dependents that are pending
	var blocked []string
	for _, st := range subtasks {
		if st.Status == "pending" {
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
		if _, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'blocked', error = 'upstream dependency failed' WHERE id = $1 AND status IN ('pending')`,
			blockedID); err != nil {
			log.Printf("executor: propagate blocked for subtask %s: %v", blockedID, err)
		}
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
	if _, err := e.DB.Exec(ctx,
		`UPDATE subtasks SET status = 'failed', error = $1 WHERE id = $2`,
		errMsg, subtaskID); err != nil {
		log.Printf("executor: mark subtask %s failed: %v", subtaskID, err)
	}

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
		        input, output, COALESCE(error, ''), COALESCE(a2a_task_id, ''),
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
			&st.Input, &st.Output, &st.Error, &st.A2ATaskID,
			&st.Attempt, &st.MaxAttempts, &st.CreatedAt, &st.StartedAt, &st.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("scan subtask: %w", err)
		}
		subtasks = append(subtasks, st)
	}
	return subtasks, rows.Err()
}

// loadAgents loads all active agents.
func (e *DAGExecutor) loadAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := e.DB.Query(ctx,
		`SELECT id, name, COALESCE(version,''), COALESCE(description,''), endpoint,
		        COALESCE(agent_card_url,''), COALESCE(agent_card,'{}'), card_fetched_at,
		        COALESCE(capabilities, '[]'), COALESCE(skills,'[]'),
		        COALESCE(status,'active'), created_at, updated_at
		 FROM agents WHERE status = 'active'`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		var capsJSON, skillsJSON, agentCard []byte
		err := rows.Scan(
			&a.ID, &a.Name, &a.Version, &a.Description, &a.Endpoint,
			&a.AgentCardURL, &agentCard, &a.CardFetchedAt,
			&capsJSON, &skillsJSON,
			&a.Status, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if agentCard != nil {
			a.AgentCard = json.RawMessage(agentCard)
		}
		if skillsJSON != nil {
			a.Skills = json.RawMessage(skillsJSON)
		}
		if err := json.Unmarshal(capsJSON, &a.Capabilities); err != nil {
			a.Capabilities = []string{}
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

// recordTemplateExecution records execution metrics when a template-based task reaches a terminal state.
func (e *DAGExecutor) recordTemplateExecution(ctx context.Context, taskID, templateID string, templateVersion int, outcome string) {
	if templateID == "" {
		return
	}

	var duration int
	var createdAt time.Time
	var replanCount int
	err := e.DB.QueryRow(ctx,
		`SELECT created_at, replan_count FROM tasks WHERE id = $1`, taskID).
		Scan(&createdAt, &replanCount)
	if err != nil {
		log.Printf("executor: record template execution: load task %s: %v", taskID, err)
		return
	}
	if !createdAt.IsZero() {
		duration = int(time.Since(createdAt).Seconds())
	}

	var hitlCount int
	_ = e.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM messages WHERE task_id = $1 AND sender_type = 'user'`, taskID).
		Scan(&hitlCount)

	_, err = e.DB.Exec(ctx,
		`INSERT INTO template_executions (id, template_id, template_version, task_id, actual_steps, hitl_interventions, replan_count, outcome, duration_seconds, created_at)
		 VALUES ($1, $2, $3, $4, '[]', $5, $6, $7, $8, NOW())`,
		uuid.New().String(), templateID, templateVersion, taskID,
		hitlCount, replanCount, outcome, duration)
	if err != nil {
		log.Printf("executor: insert template execution for task %s: %v", taskID, err)
	}
}

// publishEvent persists an event and broadcasts it to SSE subscribers.
// It looks up the task's conversation_id to also publish to conversation-level streams.
func (e *DAGExecutor) publishEvent(ctx context.Context, taskID, subtaskID, eventType, actorType, actorID string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}

	// Look up conversation_id for the task (if task exists)
	var conversationID string
	if taskID != "" {
		_ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)
	}

	evt, err := e.EventStore.SaveWithConversation(ctx, taskID, conversationID, subtaskID, eventType, actorType, actorID, data)
	if err != nil {
		log.Printf("executor: save event %s: %v", eventType, err)
		return
	}
	e.Broker.Publish(evt)
	if e.WebhookSender != nil {
		go e.WebhookSender.Send(ctx, eventType, taskID, subtaskID, data)
	}
}

// aggregateResults collects all subtask outputs into a combined task result.
func (e *DAGExecutor) aggregateResults(ctx context.Context, taskID string, subtasks []models.SubTask) json.RawMessage {
	result := make(map[string]json.RawMessage)

	// Load agent names for readable keys
	for _, st := range subtasks {
		if st.Status != "completed" || len(st.Output) == 0 {
			continue
		}
		var agentName string
		_ = e.DB.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, st.AgentID).Scan(&agentName)
		if agentName == "" {
			agentName = st.AgentID
		}
		key := agentName
		// If multiple subtasks from same agent, append index
		if _, exists := result[key]; exists {
			key = fmt.Sprintf("%s_%s", agentName, st.ID[:8])
		}
		result[key] = st.Output
	}

	b, _ := json.Marshal(result)
	return b
}

// publishSystemMessage inserts a system message into the group chat.
func (e *DAGExecutor) publishSystemMessage(ctx context.Context, taskID, content string) {
	msgID := uuid.New().String()
	now := time.Now()

	// Look up conversation_id for the task
	var conversationID string
	if taskID != "" {
		_ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)
	}

	_, err := e.DB.Exec(ctx,
		`INSERT INTO messages (id, task_id, conversation_id, sender_type, sender_id, sender_name, content, mentions, created_at)
		 VALUES ($1, $2, $3, 'system', '', 'System', $4, '{}', $5)`,
		msgID, taskID, conversationID, content, now)
	if err != nil {
		log.Printf("executor: insert system message: %v", err)
		return
	}

	e.publishEvent(ctx, taskID, "", "message", "system", "", map[string]any{
		"message_id":  msgID,
		"sender_type": "system",
		"sender_name": "System",
		"content":     content,
	})
}

// publishMessage inserts a message into the messages table and publishes it as an event.
func (e *DAGExecutor) publishMessage(ctx context.Context, taskID, senderID, senderName, content string) {
	msgID := uuid.New().String()
	now := time.Now()

	// Look up conversation_id for the task
	var conversationID string
	if taskID != "" {
		_ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)
	}

	_, err := e.DB.Exec(ctx,
		`INSERT INTO messages (id, task_id, conversation_id, sender_type, sender_id, sender_name, content, mentions, created_at)
		 VALUES ($1, $2, $3, 'agent', $4, $5, $6, '{}', $7)`,
		msgID, taskID, conversationID, senderID, senderName, content, now)
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

// publishMessageWithMetadata inserts a message with metadata into the messages table and publishes it as an event.
func (e *DAGExecutor) publishMessageWithMetadata(ctx context.Context, taskID, senderID, senderName, content string, metadata map[string]any) {
	msgID := uuid.New().String()
	now := time.Now()

	var conversationID string
	if taskID != "" {
		_ = e.DB.QueryRow(ctx, `SELECT COALESCE(conversation_id, '') FROM tasks WHERE id = $1`, taskID).Scan(&conversationID)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	_, err = e.DB.Exec(ctx,
		`INSERT INTO messages (id, task_id, conversation_id, sender_type, sender_id, sender_name, content, mentions, metadata, created_at)
		 VALUES ($1, $2, $3, 'agent', $4, $5, $6, '{}', $7, $8)`,
		msgID, taskID, conversationID, senderID, senderName, content, metadataJSON, now)
	if err != nil {
		log.Printf("executor: insert message with metadata: %v", err)
		return
	}

	e.publishEvent(ctx, taskID, "", "message", "agent", senderID, map[string]any{
		"message_id":  msgID,
		"sender_name": senderName,
		"content":     content,
		"metadata":    metadata,
	})
}

// detectArtifactMetadata inspects agent output for structured artifact markers.
// Returns metadata map with "artifacts" key if structured data is found, nil otherwise.
func detectArtifactMetadata(output string) map[string]any {
	var raw map[string]any
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return nil
	}

	artifacts, ok := raw["artifacts"]
	if !ok {
		return nil
	}

	artifactList, ok := artifacts.([]any)
	if !ok || len(artifactList) == 0 {
		return nil
	}

	valid := make([]any, 0, len(artifactList))
	for _, a := range artifactList {
		aMap, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if _, hasType := aMap["type"]; hasType {
			valid = append(valid, aMap)
		}
	}

	if len(valid) == 0 {
		return nil
	}

	return map[string]any{"artifacts": valid}
}
