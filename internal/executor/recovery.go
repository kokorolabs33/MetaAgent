package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"taskhub/internal/adapter"
	"taskhub/internal/models"
)

// Recover scans the database for in-progress tasks and resumes their execution.
// Called once on server startup. It handles four recovery scenarios:
//
//  1. Subtasks with status='running' and poll_job_id IS NOT NULL
//     → Resume polling using the EXISTING poll_job_id (never re-Submit)
//
//  2. Subtasks with status='running' and poll_job_id IS NULL
//     → Server crashed between Submit and storing job_id; mark as failed
//     → The retry logic will re-submit if attempts remain
//
//  3. Subtasks with status='waiting_for_input'
//     → Re-register signal channels so Signal() works
//
//  4. Tasks with status='running'
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
		// Mark orphaned running subtasks (no poll_job_id) as failed
		result, err := e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'failed', error = 'server restarted before job submission completed'
			 WHERE task_id = $1 AND status = 'running' AND (poll_job_id IS NULL OR poll_job_id = '')`, taskID)
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

// resumeTask loads a task and its subtasks from the database, sets up cancel/signal
// infrastructure, resumes polling for any in-progress subtasks, and re-enters the DAG loop.
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

	// Channel to notify the DAG loop when subtasks change status
	statusChangeCh := make(chan string, len(subtasks))

	var wg sync.WaitGroup

	// Resume running subtasks that have a poll_job_id (scenario 1)
	for _, st := range subtasks {
		if st.Status == "running" && st.PollJobID != "" {
			agent, ok := agentMap[st.AgentID]
			if !ok {
				log.Printf("recovery: agent %s not found for subtask %s, marking failed", st.AgentID, st.ID)
				e.DB.Exec(taskCtx,
					`UPDATE subtasks SET status = 'failed', error = 'agent not found during recovery' WHERE id = $1`, st.ID)
				continue
			}

			handle := adapter.JobHandle{
				JobID:          st.PollJobID,
				StatusEndpoint: st.PollEndpoint,
			}

			stCopy := st
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Printf("recovery: resuming poll for subtask %s (job %s)", stCopy.ID, handle.JobID)
				e.pollSubtask(taskCtx, *task, stCopy, agent, handle, subtasks, agents, statusChangeCh)
			}()
		}
	}

	// Re-register signal channels for waiting_for_input subtasks (scenario 3)
	for _, st := range subtasks {
		if st.Status == "waiting_for_input" {
			agent, ok := agentMap[st.AgentID]
			if !ok {
				log.Printf("recovery: agent %s not found for waiting subtask %s", st.AgentID, st.ID)
				continue
			}

			handle := adapter.JobHandle{
				JobID:          st.PollJobID,
				StatusEndpoint: st.PollEndpoint,
			}

			// Re-register signal channel
			inputCh := make(chan adapter.UserInput, 1)
			e.signals.Store(st.ID, inputCh)

			stCopy := st
			adp := e.Adapters[agent.AdapterType]

			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Printf("recovery: re-waiting for input on subtask %s", stCopy.ID)

				select {
				case <-taskCtx.Done():
					e.signals.Delete(stCopy.ID)
					return
				case userInput := <-inputCh:
					e.signals.Delete(stCopy.ID)

					// Send input to agent
					if adp != nil {
						if err := adp.SendInput(taskCtx, agent, handle, userInput); err != nil {
							log.Printf("recovery: send input to subtask %s: %v", stCopy.ID, err)
							e.failSubtask(taskCtx, taskID, stCopy.ID, fmt.Sprintf("send input failed: %v", err), subtasks)
							select {
							case statusChangeCh <- stCopy.ID:
							default:
							}
							return
						}
					}

					// Resume to running status
					e.DB.Exec(taskCtx, `UPDATE subtasks SET status = 'running' WHERE id = $1`, stCopy.ID)
					e.publishEvent(taskCtx, taskID, stCopy.ID, "subtask.input_provided", "user", "", map[string]any{
						"message": userInput.Message,
					})

					// Resume polling
					if adp != nil {
						e.pollSubtask(taskCtx, *task, stCopy, agent, handle, subtasks, agents, statusChangeCh)
					}
				}
			}()
		}
	}

	// Re-enter the DAG loop to pick up any remaining work
	// This runs in the current goroutine (already in a goroutine from Recover)
	err = e.runDAGLoop(taskCtx, *task, subtasks, agents)

	// Clean up
	taskCancel()
	e.cancels.Delete(taskID)
	wg.Wait()

	return err
}

// loadTask loads a single task from the database.
func (e *DAGExecutor) loadTask(ctx context.Context, taskID string) (*models.Task, error) {
	var t models.Task
	var completedAt *time.Time
	err := e.DB.QueryRow(ctx,
		`SELECT id, org_id, title, COALESCE(description,''), status, created_by,
		        metadata, plan, result, COALESCE(error,''), replan_count, created_at, completed_at
		 FROM tasks WHERE id = $1`, taskID).
		Scan(&t.ID, &t.OrgID, &t.Title, &t.Description, &t.Status, &t.CreatedBy,
			&t.Metadata, &t.Plan, &t.Result, &t.Error, &t.ReplanCount, &t.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	t.CompletedAt = completedAt

	// Ensure Metadata is not nil (for JSON serialization)
	if t.Metadata == nil {
		t.Metadata = json.RawMessage("{}")
	}

	return &t, nil
}
