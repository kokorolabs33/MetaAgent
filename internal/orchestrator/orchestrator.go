// Package orchestrator decomposes tasks into subtask DAGs using an LLM.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"taskhub/internal/models"
)

// Orchestrator decomposes tasks into subtask DAGs using an LLM.
type Orchestrator struct {
	// Future: use Anthropic Go SDK. MVP: use claude CLI.
}

const planPrompt = `You are a task decomposition engine. Given a task and available agents, break the task into subtasks.

RULES:
- Each subtask is assigned to exactly one agent by agent_id
- Subtasks can depend on other subtasks (DAG - no cycles)
- Use depends_on with subtask IDs to define execution order
- Subtasks with no dependencies can run in parallel
- Give each subtask a unique short ID like "s1", "s2", etc.
- When a task involves multiple departments or perspectives, add a final synthesis subtask that depends on all previous subtasks to create a consolidated summary/recommendation
- Return ONLY valid JSON, no markdown or explanation

Available agents:
%s

Respond with ONLY this JSON structure:
{"summary":"brief plan summary","subtasks":[{"id":"s1","agent_id":"<agent-uuid>","agent_name":"<agent-name>","instruction":"specific instruction for this agent","depends_on":[]}]}`

const replanPrompt = `A subtask in the execution plan has failed. You need to create replacement subtasks.

RULES:
- Only replace the failed subtask and its downstream dependents that were blocked
- Already-completed subtasks must NOT be regenerated
- Use new unique subtask IDs that don't collide with existing ones
- Preserve the same JSON structure
- Return ONLY valid JSON, no markdown or explanation

Original task: %s
Current plan state: %s
Failed subtask: %s (agent: %s, instruction: %s)
Error: %s

Respond with ONLY this JSON structure:
{"summary":"replan summary","subtasks":[{"id":"s1_retry","agent_id":"<agent-uuid>","agent_name":"<agent-name>","instruction":"specific instruction","depends_on":[]}]}`

// Plan calls the LLM to decompose a task into a DAG of subtasks.
// policyConstraints is an optional string appended to the user message to guide
// decomposition according to active policy rules (may be empty).
// templateSkeleton is an optional template structure that guides the decomposition
// when a task is created from a workflow template (may be empty).
func (o *Orchestrator) Plan(ctx context.Context, task models.Task, agents []models.Agent, policyConstraints, templateSkeleton string) (*models.ExecutionPlan, error) {
	agentDesc := buildAgentDescription(agents)
	systemPrompt := fmt.Sprintf(planPrompt, agentDesc)
	userMsg := fmt.Sprintf("Task: %s\n\nDescription: %s", task.Title, task.Description)
	if policyConstraints != "" {
		userMsg += "\n" + policyConstraints
	}
	if templateSkeleton != "" {
		userMsg += "\n" + templateSkeleton
	}

	response, err := callLLM(ctx, systemPrompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("plan llm call: %w", err)
	}

	var plan models.ExecutionPlan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse plan response: %w (response: %.200s)", err, response)
	}

	if len(plan.SubTasks) == 0 {
		return nil, fmt.Errorf("plan returned no subtasks")
	}

	return &plan, nil
}

// Replan calls the LLM to create replacement subtasks after a failure.
// It receives the original task, the failed subtask, and the available agents.
// The LLM returns a partial plan replacing the failed subtask and its blocked dependents.
func (o *Orchestrator) Replan(ctx context.Context, task models.Task, failed models.SubTask, agents []models.Agent) (*models.ExecutionPlan, error) {
	planJSON, _ := json.Marshal(task.Plan)
	userMsg := fmt.Sprintf(replanPrompt,
		task.Title,
		string(planJSON),
		failed.ID,
		failed.AgentID,
		failed.Instruction,
		failed.Error,
	)

	agentDesc := buildAgentDescription(agents)
	systemPrompt := fmt.Sprintf("You are a task replanning engine. Available agents:\n%s\nRespond with ONLY valid JSON.", agentDesc)

	response, err := callLLM(ctx, systemPrompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("replan llm call: %w", err)
	}

	var plan models.ExecutionPlan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse replan response: %w (response: %.200s)", err, response)
	}

	if len(plan.SubTasks) == 0 {
		return nil, fmt.Errorf("replan returned no subtasks")
	}

	return &plan, nil
}

// buildAgentDescription formats agent info for the LLM prompt.
func buildAgentDescription(agents []models.Agent) string {
	var sb strings.Builder
	for _, a := range agents {
		caps := "none"
		if len(a.Capabilities) > 0 {
			caps = strings.Join(a.Capabilities, ", ")
		}
		fmt.Fprintf(&sb, "- ID: %s | Name: %s | Capabilities: %s | Description: %s\n",
			a.ID, a.Name, caps, a.Description)
	}
	return sb.String()
}

// callLLM uses the claude CLI for MVP. Replace with Anthropic Go SDK later.
func callLLM(ctx context.Context, systemPrompt, userMsg string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "--print", "--system-prompt", systemPrompt, "--output-format", "text", userMsg)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude cli exited %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude cli: %w", err)
	}

	result := stripMarkdownFences(strings.TrimSpace(string(out)))
	return result, nil
}

// IntentResult is the output of DetectIntent.
type IntentResult struct {
	Type        string `json:"type"`        // "chat" or "task"
	Content     string `json:"content"`     // response text (for chat)
	Title       string `json:"title"`       // task title (for task)
	Description string `json:"description"` // task description (for task)
}

// ChatMessage represents a single message in conversation history for intent detection.
type ChatMessage struct {
	Role    string // "user", "agent", "system"
	Name    string
	Content string
}

const intentPrompt = `You are TaskHub's orchestrator. Analyze the user's message and decide:
1. If the message describes a task that requires multiple agents/departments to collaborate, respond with a task decomposition.
2. If the message is a question, clarification, follow-up discussion, or doesn't need agent collaboration, respond conversationally.

Available agents:
%s

Respond with ONLY this JSON (no markdown fences):
For conversational response: {"type":"chat","content":"your response"}
For task creation: {"type":"task","title":"brief task title","description":"detailed task description"}`

// DetectIntent analyzes a user message in context and decides whether it should
// trigger a task creation or a conversational response.
func (o *Orchestrator) DetectIntent(ctx context.Context, history []ChatMessage, userMessage string, agents []models.Agent) (*IntentResult, error) {
	agentDesc := buildAgentDescription(agents)
	systemPrompt := fmt.Sprintf(intentPrompt, agentDesc)

	// Build conversation history context
	var sb strings.Builder
	for _, msg := range history {
		fmt.Fprintf(&sb, "[%s] %s: %s\n", msg.Role, msg.Name, msg.Content)
	}
	sb.WriteString("\n[user] " + userMessage)

	response, err := callLLM(ctx, systemPrompt, sb.String())
	if err != nil {
		return nil, fmt.Errorf("detect intent llm call: %w", err)
	}

	var result IntentResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse intent response: %w (response: %.200s)", err, response)
	}

	if result.Type != "chat" && result.Type != "task" {
		return nil, fmt.Errorf("unexpected intent type: %q", result.Type)
	}

	return &result, nil
}

// stripMarkdownFences removes markdown code fences from LLM output.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	// Handle ```json ... ``` or ``` ... ```
	if strings.HasPrefix(s, "```") {
		// Remove the opening fence line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove the closing fence
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
