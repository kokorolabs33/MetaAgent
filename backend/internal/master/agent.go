package master

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"taskhub/internal/models"
	oai "taskhub/internal/openai"
	"taskhub/internal/sse"
)

const masterSystemPrompt = `You are the Master Agent for TaskHub, an enterprise AI coordination platform.
Your job is to:
1. Analyze the given task
2. Decompose it into subtasks
3. Assign each subtask to the most appropriate team agent based on their capabilities
4. Return a JSON plan

Always respond with valid JSON in this format:
{
  "summary": "brief task summary",
  "subtasks": [
    {
      "agent_id": "agent-id-here",
      "agent_name": "SRE Agent",
      "instruction": "specific instruction for this agent",
      "order": 1
    }
  ]
}`

type SubTask struct {
	AgentID     string `json:"agent_id"`
	AgentName   string `json:"agent_name"`
	Instruction string `json:"instruction"`
	Order       int    `json:"order"`
}

type Plan struct {
	Summary  string    `json:"summary"`
	SubTasks []SubTask `json:"subtasks"`
}

type Agent struct {
	DB     *sql.DB
	OpenAI *oai.Client
	Broker *sse.Broker
}

// Run executes the full master agent pipeline for a task.
// Must be called in a goroutine — it blocks until the task completes.
func (a *Agent) Run(taskID string, description string) {
	ctx := context.Background()

	// Update task to running
	if _, err := a.DB.ExecContext(ctx, `UPDATE tasks SET status='running' WHERE id=$1`, taskID); err != nil {
		log.Printf("master: update task status: %v", err)
		return
	}

	// Load available agents
	agents, err := a.loadAgents(ctx)
	if err != nil {
		log.Printf("master: load agents: %v", err)
		a.failTask(ctx, taskID)
		return
	}

	// Build agent capabilities summary for the LLM prompt
	var agentDesc strings.Builder
	for _, ag := range agents {
		agentDesc.WriteString(fmt.Sprintf("- ID: %s | Name: %s | Capabilities: %v\n",
			ag.ID, ag.Name, ag.Capabilities))
	}

	userMsg := fmt.Sprintf("Task: %s\n\nAvailable agents:\n%s", description, agentDesc.String())

	// Step 1: Ask master LLM to decompose the task
	planJSON, err := a.OpenAI.Chat(ctx, masterSystemPrompt, userMsg)
	if err != nil {
		log.Printf("master: llm decompose: %v", err)
		a.failTask(ctx, taskID)
		return
	}

	// Strip markdown code fences if the LLM wraps JSON in ```json ... ```
	planJSON = strings.TrimSpace(planJSON)
	if strings.HasPrefix(planJSON, "```") {
		lines := strings.Split(planJSON, "\n")
		if len(lines) > 2 {
			planJSON = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var plan Plan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		log.Printf("master: parse plan JSON: %v (raw: %s)", err, planJSON)
		a.failTask(ctx, taskID)
		return
	}

	// Step 2: Create a Channel for this task
	channelID := uuid.New().String()
	initDoc := fmt.Sprintf("# Task\n%s\n\n## Plan\n%s\n\n## Subtasks\n", description, plan.Summary)
	for _, st := range plan.SubTasks {
		initDoc += fmt.Sprintf("%d. **%s**: %s\n", st.Order, st.AgentName, st.Instruction)
	}

	if _, err := a.DB.ExecContext(ctx,
		`INSERT INTO channels (id, task_id, document) VALUES ($1,$2,$3)`,
		channelID, taskID, initDoc,
	); err != nil {
		log.Printf("master: create channel: %v", err)
		a.failTask(ctx, taskID)
		return
	}

	// Publish initial SSE events
	a.Broker.Publish(channelID, "task_started", map[string]any{"task_id": taskID})
	a.Broker.Publish(channelID, "channel_created", map[string]any{"channel_id": channelID})
	a.Broker.Publish(channelID, "document_updated", map[string]any{"document": initDoc})

	// Step 3: Add selected agents to the channel
	agentMap := make(map[string]models.Agent)
	for _, ag := range agents {
		agentMap[ag.ID] = ag
	}
	for _, st := range plan.SubTasks {
		if _, ok := agentMap[st.AgentID]; !ok {
			continue
		}
		if _, err := a.DB.ExecContext(ctx,
			`INSERT INTO channel_agents (channel_id, agent_id, status) VALUES ($1,$2,'idle') ON CONFLICT DO NOTHING`,
			channelID, st.AgentID,
		); err != nil {
			log.Printf("master: add agent to channel: %v", err)
		}
		a.Broker.Publish(channelID, "agent_joined", map[string]any{
			"agent_id": st.AgentID, "agent_name": st.AgentName,
		})
	}

	// Post master system message summarizing the plan
	a.postMessage(ctx, channelID, "master", "Master Agent", plan.Summary, "system")

	// Step 4: Execute subtasks sequentially
	currentDoc := initDoc
	for _, st := range plan.SubTasks {
		ag, ok := agentMap[st.AgentID]
		if !ok {
			log.Printf("master: agent %s not found, skipping subtask", st.AgentID)
			continue
		}

		// Mark agent as working
		if _, err := a.DB.ExecContext(ctx,
			`UPDATE channel_agents SET status='working' WHERE channel_id=$1 AND agent_id=$2`,
			channelID, ag.ID,
		); err != nil {
			log.Printf("master: update agent status working: %v", err)
		}
		a.Broker.Publish(channelID, "agent_working", map[string]any{
			"agent_id": ag.ID, "agent_name": ag.Name,
		})

		// Build the agent's prompt: their system prompt + current doc + specific instruction
		agentUserMsg := fmt.Sprintf("Current shared context:\n%s\n\nYour instruction: %s", currentDoc, st.Instruction)

		result, err := a.OpenAI.Chat(ctx, ag.SystemPrompt, agentUserMsg)
		if err != nil {
			log.Printf("master: agent %s LLM call failed: %v", ag.Name, err)
			result = fmt.Sprintf("[Agent %s encountered an error: %v]", ag.Name, err)
		}

		// Append result to the shared document
		currentDoc += fmt.Sprintf("\n\n---\n## %s Result\n%s", ag.Name, result)
		if _, err := a.DB.ExecContext(ctx,
			`UPDATE channels SET document=$1 WHERE id=$2`, currentDoc, channelID,
		); err != nil {
			log.Printf("master: update document: %v", err)
		}

		// Post agent message and fire SSE events
		msg := a.postMessage(ctx, channelID, ag.ID, ag.Name, result, "result")
		if msg != nil {
			a.Broker.Publish(channelID, "message", map[string]any{"message": msg})
		}
		a.Broker.Publish(channelID, "document_updated", map[string]any{"document": currentDoc})

		// Mark agent done
		if _, err := a.DB.ExecContext(ctx,
			`UPDATE channel_agents SET status='done' WHERE channel_id=$1 AND agent_id=$2`,
			channelID, ag.ID,
		); err != nil {
			log.Printf("master: update agent status done: %v", err)
		}
		a.Broker.Publish(channelID, "agent_done", map[string]any{"agent_id": ag.ID})
	}

	// Step 5: Mark task completed
	now := time.Now()
	if _, err := a.DB.ExecContext(ctx,
		`UPDATE tasks SET status='completed', completed_at=$1 WHERE id=$2`, now, taskID,
	); err != nil {
		log.Printf("master: complete task: %v", err)
	}
	if _, err := a.DB.ExecContext(ctx,
		`UPDATE channels SET status='archived' WHERE id=$1`, channelID,
	); err != nil {
		log.Printf("master: archive channel: %v", err)
	}

	var task models.Task
	if err := a.DB.QueryRowContext(ctx,
		`SELECT id, title, description, status, created_at, completed_at FROM tasks WHERE id=$1`, taskID,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.CreatedAt, &task.CompletedAt); err != nil {
		log.Printf("master: fetch completed task: %v", err)
	}

	a.Broker.Publish(channelID, "task_completed", map[string]any{"task": task})
	log.Printf("master: task %s completed successfully", taskID)
}

func (a *Agent) loadAgents(ctx context.Context) ([]models.Agent, error) {
	rows, err := a.DB.QueryContext(ctx,
		`SELECT id, name, description, system_prompt, capabilities, color, created_at FROM agents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var ag models.Agent
		var caps string
		if err := rows.Scan(&ag.ID, &ag.Name, &ag.Description, &ag.SystemPrompt, &caps, &ag.Color, &ag.CreatedAt); err != nil {
			return nil, err
		}
		ag.Capabilities = models.JSONToCapabilities(caps)
		agents = append(agents, ag)
	}
	return agents, rows.Err()
}

func (a *Agent) postMessage(ctx context.Context, channelID, senderID, senderName, content, msgType string) *models.Message {
	msg := &models.Message{
		ID:         uuid.New().String(),
		ChannelID:  channelID,
		SenderID:   senderID,
		SenderName: senderName,
		Content:    content,
		Type:       msgType,
		CreatedAt:  time.Now(),
	}
	if _, err := a.DB.ExecContext(ctx,
		`INSERT INTO messages (id, channel_id, sender_id, sender_name, content, type) VALUES ($1,$2,$3,$4,$5,$6)`,
		msg.ID, msg.ChannelID, msg.SenderID, msg.SenderName, msg.Content, msg.Type,
	); err != nil {
		log.Printf("master: insert message: %v", err)
		return nil
	}
	return msg
}

func (a *Agent) failTask(ctx context.Context, taskID string) {
	if _, err := a.DB.ExecContext(ctx, `UPDATE tasks SET status='failed' WHERE id=$1`, taskID); err != nil {
		log.Printf("master: fail task: %v", err)
	}
}
