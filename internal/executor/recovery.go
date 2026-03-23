package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"taskhub/internal/models"
)

// Recover scans the database for in-progress tasks and resumes their execution.
// Called once on server startup. It handles three recovery scenarios:
//
//  1. Subtasks with status='running' and a2a_task_id IS NOT NULL
//     → Query the agent via A2A GetTask to check current state
//
//  2. Subtasks with status='running' and a2a_task_id IS NULL
//     → Server crashed between SendMessage and storing task_id; mark as failed
//     → The retry logic will re-submit if attempts remain
//
//  3. Tasks with status='running'
//     → Create fresh cancel context and re-enter DAG loop
func (e *DAGExecutor) Recover(ctx context.Context) {
	// Find all tasks that were running when the server stopped
	rows, err := e.DB.Query(ctx,
		`SELECT DISTINCT t.id FROM tasks t WHERE t.status = 'running'`)
	if err != nil {
		log.Printf("recovery: query running tasks: %v", err)
		return
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.Printf("recovery: scan task id: %v", err)
			continue
		}
		taskIDs = append(taskIDs, id)
	}
	if err := rows.Err(); err != nil {
		log.Printf("recovery: rows error: %v", err)
		return
	}

	if len(taskIDs) == 0 {
		return
	}

	log.Printf("recovery: found %d running tasks to resume", len(taskIDs))

	for _, taskID := range taskIDs {
		// Mark orphaned running subtasks (no a2a_task_id) as failed
		result, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'failed', error = 'server restarted before A2A task ID stored'
			 WHERE task_id = $1 AND status = 'running' AND (a2a_task_id IS NULL OR a2a_task_id = '')`, taskID)
		if err != nil {
			log.Printf("recovery: mark orphaned subtasks for task %s: %v", taskID, err)
		} else if result.RowsAffected() > 0 {
			log.Printf("recovery: marked %d orphaned subtasks as failed for task %s", result.RowsAffected(), taskID)
		}

		// Resume the task in a new goroutine
		go func(tid string) {
			log.Printf("recovery: resuming task %s", tid)
			if err := e.resumeTask(ctx, tid); err != nil {
				log.Printf("recovery: task %s failed to resume: %v", tid, err)
			}
		}(taskID)
	}

	log.Printf("recovery: initiated resume for %d tasks", len(taskIDs))
}

// resumeTask loads a task and its subtasks from the database, checks running subtasks
// via A2A GetTask, and re-enters the DAG loop.
func (e *DAGExecutor) resumeTask(parentCtx context.Context, taskID string) error {
	// Load task from DB
	task, err := e.loadTask(parentCtx, taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}

	// Load all subtasks
	subtasks, err := e.loadSubtasks(parentCtx, taskID)
	if err != nil {
		return fmt.Errorf("load subtasks: %w", err)
	}

	// Load agents for this org
	agents, err := e.loadOrgAgents(parentCtx, task.OrgID)
	if err != nil {
		return fmt.Errorf("load agents: %w", err)
	}

	// Build agent lookup
	agentMap := make(map[string]models.Agent, len(agents))
	for _, a := range agents {
		agentMap[a.ID] = a
	}

	// Create cancellable context for this task
	taskCtx, taskCancel := context.WithCancel(parentCtx)
	e.cancels.Store(taskID, taskCancel)

	var wg sync.WaitGroup

	// Check running subtasks that have an a2a_task_id via A2A GetTask
	for _, st := range subtasks {
		if st.Status != "running" || st.A2ATaskID == "" {
			continue
		}

		agent, ok := agentMap[st.AgentID]
		if !ok {
			log.Printf("recovery: agent %s not found for subtask %s, marking failed", st.AgentID, st.ID)
			_, _ = e.DB.Exec(taskCtx,
				`UPDATE subtasks SET status = 'failed', error = 'agent not found during recovery' WHERE id = $1`, st.ID)
			continue
		}

		stCopy := st
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("recovery: checking A2A task %s for subtask %s", stCopy.A2ATaskID, stCopy.ID)

			result, err := e.A2AClient.GetTask(taskCtx, agent.Endpoint, stCopy.A2ATaskID)
			if err != nil {
				log.Printf("recovery: GetTask failed for subtask %s: %v", stCopy.ID, err)
				// Cannot reach agent — mark as failed for retry
				_, _ = e.DB.Exec(taskCtx,
					`UPDATE subtasks SET status = 'failed', error = $1 WHERE id = $2`,
					fmt.Sprintf("recovery: agent unreachable: %v", err), stCopy.ID)
				return
			}

			switch result.State {
			case "completed":
				now := time.Now()
				output := result.Artifacts
				if output == nil {
					output = json.RawMessage(`null`)
				}
				_, _ = e.DB.Exec(taskCtx,
					`UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2 WHERE id = $3`,
					output, now, stCopy.ID)
				log.Printf("recovery: subtask %s completed during downtime", stCopy.ID)

			case "failed":
				_, _ = e.DB.Exec(taskCtx,
					`UPDATE subtasks SET status = 'failed', error = $1 WHERE id = $2`,
					result.Error, stCopy.ID)
				log.Printf("recovery: subtask %s failed during downtime: %s", stCopy.ID, result.Error)

			case "input-required":
				_, _ = e.DB.Exec(taskCtx,
					`UPDATE subtasks SET status = 'input_required' WHERE id = $1`, stCopy.ID)
				log.Printf("recovery: subtask %s is waiting for input", stCopy.ID)

			default:
				// Still working or unknown state — leave as running, DAG loop will handle
				log.Printf("recovery: subtask %s still in state %q", stCopy.ID, result.State)
			}
		}()
	}

	// Wait for all recovery checks to complete
	wg.Wait()

	// Re-enter the DAG loop to pick up any remaining work
	err = e.runDAGLoop(taskCtx, *task, subtasks, agents)

	// Clean up
	taskCancel()
	e.cancels.Delete(taskID)

	return err
}

// loadTask loads a single task from the database.
func (e *DAGExecutor) loadTask(ctx context.Context, taskID string) (*models.Task, error) {
	var t models.Task
	var completedAt *time.Time
	var metadata, plan, result []byte
	var taskError *string
	err := e.DB.QueryRow(ctx,
		`SELECT id, org_id, title, COALESCE(description,''), status, created_by,
		        metadata, plan, result, error, replan_count, created_at, completed_at
		 FROM tasks WHERE id = $1`, taskID).
		Scan(&t.ID, &t.OrgID, &t.Title, &t.Description, &t.Status, &t.CreatedBy,
			&metadata, &plan, &result, &taskError, &t.ReplanCount, &t.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	t.CompletedAt = completedAt
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

	// Ensure Metadata is not nil (for JSON serialization)
	if t.Metadata == nil {
		t.Metadata = json.RawMessage("{}")
	}

	return &t, nil
}
