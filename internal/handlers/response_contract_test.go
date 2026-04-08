package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"taskhub/internal/models"
)

// ---------------------------------------------------------------------------
// Frontend-Backend Response Contract Tests
//
// These tests verify that Go model JSON serialization produces the exact field
// names and shapes expected by the TypeScript interfaces in web/lib/types.ts.
// Every required field listed here corresponds to a non-optional property in
// the matching TypeScript interface.
//
// If a Go struct tag is renamed or a field is removed, these tests catch the
// break before it reaches the frontend.
// ---------------------------------------------------------------------------

// assertFields is a helper that marshals a value and checks that all listed
// JSON keys are present in the resulting object.
func assertFields(t *testing.T, label string, v any, required []string) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: json.Marshal: %v", label, err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("%s: json.Unmarshal: %v", label, err)
	}

	for _, field := range required {
		if _, ok := parsed[field]; !ok {
			t.Errorf("%s: missing required JSON field %q", label, field)
		}
	}
	return parsed
}

// assertFieldAbsent verifies a field is NOT present (omitempty removed it).
func assertFieldAbsent(t *testing.T, label string, parsed map[string]any, field string) {
	t.Helper()
	if _, ok := parsed[field]; ok {
		t.Errorf("%s: field %q should be omitted when empty, but is present", label, field)
	}
}

// ---------------------------------------------------------------------------
// Agent  ↔  web/lib/types.ts Agent
// ---------------------------------------------------------------------------

func TestAgentResponseContract(t *testing.T) {
	now := time.Now().UTC()
	agent := models.Agent{
		ID:           "agent-1",
		Name:         "Code Reviewer",
		Version:      "1.0.0",
		Description:  "Reviews code",
		Endpoint:     "http://localhost:9001",
		AgentCardURL: "http://localhost:9001/.well-known/agent-card.json",
		Capabilities: []string{"streaming"},
		Status:       "active",
		IsOnline:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	parsed := assertFields(t, "Agent", agent, []string{
		"id", "name", "version", "description", "endpoint",
		"agent_card_url", "capabilities", "status", "is_online",
		"created_at", "updated_at",
	})

	// capabilities must serialize as an array
	caps, ok := parsed["capabilities"].([]any)
	if !ok {
		t.Fatalf("Agent.capabilities should be []any, got %T", parsed["capabilities"])
	}
	if len(caps) != 1 || caps[0] != "streaming" {
		t.Errorf("Agent.capabilities = %v, want [streaming]", caps)
	}

	// is_online must be a boolean
	if _, ok := parsed["is_online"].(bool); !ok {
		t.Errorf("Agent.is_online should be bool, got %T", parsed["is_online"])
	}
}

func TestAgentResponseContract_NilCapabilities(t *testing.T) {
	agent := models.Agent{
		ID:        "agent-2",
		Name:      "Minimal",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Go serializes nil []string as JSON null.
	// Frontend MUST use (agent.capabilities ?? []) to handle this.
	val := parsed["capabilities"]
	if val != nil {
		if _, ok := val.([]any); !ok {
			t.Errorf("Agent.capabilities should be null or array, got %T", val)
		}
	}
	// null is acceptable — the frontend handles it with ?? []
}

func TestAgentResponseContract_OmitEmpty(t *testing.T) {
	agent := models.Agent{
		ID:        "agent-3",
		Name:      "Bare",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, _ := json.Marshal(agent)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	// Fields with omitempty should be absent when zero
	for _, field := range []string{"agent_card", "card_fetched_at", "last_health_check", "skills"} {
		assertFieldAbsent(t, "Agent", parsed, field)
	}
}

// ---------------------------------------------------------------------------
// Task  ↔  web/lib/types.ts Task
// ---------------------------------------------------------------------------

func TestTaskResponseContract(t *testing.T) {
	now := time.Now().UTC()
	task := models.Task{
		ID:          "task-1",
		Title:       "Review frontend",
		Description: "Review the React components",
		Status:      "pending",
		CreatedBy:   "user-1",
		ReplanCount: 0,
		CreatedAt:   now,
	}

	assertFields(t, "Task", task, []string{
		"id", "title", "description", "status", "created_by",
		"replan_count", "created_at",
	})
}

func TestTaskResponseContract_AllStatuses(t *testing.T) {
	// Verify every status value the frontend union type expects is valid.
	validStatuses := []string{
		"pending", "planning", "running", "completed",
		"failed", "canceled", "approval_required",
	}
	for _, status := range validStatuses {
		task := models.Task{
			ID:        "task-s",
			Title:     "Status test",
			Status:    status,
			CreatedBy: "user-1",
			CreatedAt: time.Now().UTC(),
		}
		data, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("Marshal with status %q: %v", status, err)
		}
		var parsed map[string]any
		json.Unmarshal(data, &parsed) //nolint:errcheck
		if parsed["status"] != status {
			t.Errorf("status round-trip: got %q, want %q", parsed["status"], status)
		}
	}
}

func TestTaskResponseContract_OmitEmpty(t *testing.T) {
	task := models.Task{
		ID:        "task-2",
		Title:     "Minimal",
		Status:    "pending",
		CreatedBy: "user-1",
		CreatedAt: time.Now().UTC(),
	}

	data, _ := json.Marshal(task)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	for _, field := range []string{"metadata", "plan", "result", "error", "completed_at", "template_id"} {
		assertFieldAbsent(t, "Task", parsed, field)
	}
}

func TestTaskResponseContract_WithOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	task := models.Task{
		ID:              "task-3",
		Title:           "Full task",
		Description:     "All fields",
		Status:          "completed",
		CreatedBy:       "user-1",
		Metadata:        json.RawMessage(`{"key":"val"}`),
		Plan:            json.RawMessage(`{"summary":"test"}`),
		Result:          json.RawMessage(`{"output":"done"}`),
		Error:           "something",
		TemplateID:      "tmpl-1",
		TemplateVersion: 2,
		ReplanCount:     1,
		CreatedAt:       now,
		CompletedAt:     &now,
	}

	assertFields(t, "Task(full)", task, []string{
		"id", "title", "description", "status", "created_by",
		"metadata", "plan", "result", "error",
		"template_id", "template_version", "replan_count",
		"created_at", "completed_at",
	})
}

// ---------------------------------------------------------------------------
// SubTask  ↔  web/lib/types.ts SubTask
// ---------------------------------------------------------------------------

func TestSubTaskResponseContract(t *testing.T) {
	now := time.Now().UTC()
	st := models.SubTask{
		ID:          "st-1",
		TaskID:      "task-1",
		AgentID:     "agent-1",
		Instruction: "Analyze the code",
		DependsOn:   []string{"st-0"},
		Status:      "pending",
		Attempt:     0,
		MaxAttempts: 3,
		CreatedAt:   now,
	}

	parsed := assertFields(t, "SubTask", st, []string{
		"id", "task_id", "agent_id", "instruction", "depends_on",
		"status", "attempt", "max_attempts", "created_at",
	})

	// depends_on must be an array
	deps, ok := parsed["depends_on"].([]any)
	if !ok {
		t.Fatalf("SubTask.depends_on should be []any, got %T", parsed["depends_on"])
	}
	if len(deps) != 1 {
		t.Errorf("SubTask.depends_on length = %d, want 1", len(deps))
	}
}

func TestSubTaskResponseContract_NilDependsOn(t *testing.T) {
	st := models.SubTask{
		ID:          "st-2",
		TaskID:      "task-1",
		AgentID:     "agent-1",
		Instruction: "No deps",
		DependsOn:   nil, // Go nil slice
		Status:      "pending",
		Attempt:     0,
		MaxAttempts: 3,
		CreatedAt:   time.Now().UTC(),
	}

	data, _ := json.Marshal(st)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	// nil []string serializes as JSON null.
	// Frontend must use (st.depends_on ?? []) to handle this safely.
	val := parsed["depends_on"]
	if val != nil {
		if _, ok := val.([]any); !ok {
			t.Errorf("SubTask.depends_on should be null or array, got %T", val)
		}
	}
}

func TestSubTaskResponseContract_OmitEmpty(t *testing.T) {
	st := models.SubTask{
		ID:          "st-3",
		TaskID:      "task-1",
		AgentID:     "agent-1",
		Instruction: "Minimal",
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
	}

	data, _ := json.Marshal(st)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	for _, field := range []string{"input", "output", "error", "a2a_task_id", "started_at", "completed_at"} {
		assertFieldAbsent(t, "SubTask", parsed, field)
	}
}

func TestSubTaskResponseContract_AllStatuses(t *testing.T) {
	validStatuses := []string{
		"pending", "running", "completed", "failed",
		"input_required", "canceled", "blocked",
	}
	for _, status := range validStatuses {
		st := models.SubTask{
			ID:        "st-s",
			TaskID:    "task-1",
			AgentID:   "agent-1",
			Status:    status,
			CreatedAt: time.Now().UTC(),
		}
		data, _ := json.Marshal(st)
		var parsed map[string]any
		json.Unmarshal(data, &parsed) //nolint:errcheck
		if parsed["status"] != status {
			t.Errorf("SubTask status round-trip: got %q, want %q", parsed["status"], status)
		}
	}
}

// ---------------------------------------------------------------------------
// TaskWithSubtasks  ↔  web/lib/types.ts TaskWithSubtasks
// ---------------------------------------------------------------------------

func TestTaskWithSubtasksResponseContract(t *testing.T) {
	now := time.Now().UTC()
	tws := models.TaskWithSubtasks{
		Task: models.Task{
			ID:        "task-1",
			Title:     "Parent task",
			Status:    "running",
			CreatedBy: "user-1",
			CreatedAt: now,
		},
		SubTasks: []models.SubTask{
			{
				ID:        "st-1",
				TaskID:    "task-1",
				AgentID:   "agent-1",
				Status:    "completed",
				DependsOn: []string{},
				CreatedAt: now,
			},
		},
	}

	parsed := assertFields(t, "TaskWithSubtasks", tws, []string{
		"id", "title", "status", "created_by", "created_at", "subtasks",
	})

	// subtasks must be an array
	subtasks, ok := parsed["subtasks"].([]any)
	if !ok {
		t.Fatalf("TaskWithSubtasks.subtasks should be []any, got %T", parsed["subtasks"])
	}
	if len(subtasks) != 1 {
		t.Errorf("subtasks length = %d, want 1", len(subtasks))
	}
}

func TestTaskWithSubtasksResponseContract_EmptySubtasks(t *testing.T) {
	tws := models.TaskWithSubtasks{
		Task: models.Task{
			ID:        "task-2",
			Title:     "No subtasks yet",
			Status:    "pending",
			CreatedBy: "user-1",
			CreatedAt: time.Now().UTC(),
		},
		SubTasks: []models.SubTask{}, // empty, not nil
	}

	data, _ := json.Marshal(tws)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	subtasks, ok := parsed["subtasks"].([]any)
	if !ok {
		t.Fatalf("subtasks should be []any even when empty, got %T", parsed["subtasks"])
	}
	if len(subtasks) != 0 {
		t.Errorf("subtasks length = %d, want 0", len(subtasks))
	}
}

// ---------------------------------------------------------------------------
// Message  ↔  web/lib/types.ts Message
// ---------------------------------------------------------------------------

func TestMessageResponseContract(t *testing.T) {
	now := time.Now().UTC()
	msg := models.Message{
		ID:         "msg-1",
		TaskID:     "task-1",
		SenderType: "agent",
		SenderID:   "agent-1",
		SenderName: "Code Reviewer",
		Content:    "Found 3 issues",
		Mentions:   []string{"agent-2"},
		CreatedAt:  now,
	}

	parsed := assertFields(t, "Message", msg, []string{
		"id", "task_id", "sender_type", "sender_name",
		"content", "mentions", "created_at",
	})

	// mentions must be an array
	mentions, ok := parsed["mentions"].([]any)
	if !ok {
		t.Fatalf("Message.mentions should be []any, got %T", parsed["mentions"])
	}
	if len(mentions) != 1 {
		t.Errorf("Message.mentions length = %d, want 1", len(mentions))
	}
}

func TestMessageResponseContract_NilMentions(t *testing.T) {
	msg := models.Message{
		ID:         "msg-2",
		TaskID:     "task-1",
		SenderType: "user",
		SenderName: "Alice",
		Content:    "Hello",
		Mentions:   nil, // Go nil slice
		CreatedAt:  time.Now().UTC(),
	}

	data, _ := json.Marshal(msg)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	// Frontend must use (msg.mentions ?? []) to handle JSON null.
	val := parsed["mentions"]
	if val != nil {
		if _, ok := val.([]any); !ok {
			t.Errorf("Message.mentions should be null or array, got %T", val)
		}
	}
}

func TestMessageResponseContract_SenderTypes(t *testing.T) {
	// Verify all valid sender_type values
	for _, senderType := range []string{"agent", "user", "system"} {
		msg := models.Message{
			ID:         "msg-st",
			TaskID:     "task-1",
			SenderType: senderType,
			SenderName: "Test",
			Content:    "Hello",
			CreatedAt:  time.Now().UTC(),
		}
		data, _ := json.Marshal(msg)
		var parsed map[string]any
		json.Unmarshal(data, &parsed) //nolint:errcheck
		if parsed["sender_type"] != senderType {
			t.Errorf("sender_type round-trip: got %q, want %q", parsed["sender_type"], senderType)
		}
	}
}

// ---------------------------------------------------------------------------
// Event  ↔  web/lib/types.ts TaskEvent / TimelineEvent
// ---------------------------------------------------------------------------

func TestEventResponseContract(t *testing.T) {
	now := time.Now().UTC()
	evt := models.Event{
		ID:        "evt-1",
		TaskID:    "task-1",
		SubtaskID: "st-1",
		Type:      "subtask.completed",
		ActorType: "agent",
		ActorID:   "agent-1",
		Data:      json.RawMessage(`{"output":"test"}`),
		CreatedAt: now,
	}

	parsed := assertFields(t, "Event", evt, []string{
		"id", "task_id", "type", "actor_type", "data", "created_at",
	})

	// data must be an object (not a string)
	if _, ok := parsed["data"].(map[string]any); !ok {
		t.Errorf("Event.data should be object, got %T", parsed["data"])
	}
}

func TestEventResponseContract_ActorTypes(t *testing.T) {
	for _, actorType := range []string{"system", "agent", "user"} {
		evt := models.Event{
			ID:        "evt-at",
			TaskID:    "task-1",
			Type:      "test",
			ActorType: actorType,
			Data:      json.RawMessage(`{}`),
			CreatedAt: time.Now().UTC(),
		}
		data, _ := json.Marshal(evt)
		var parsed map[string]any
		json.Unmarshal(data, &parsed) //nolint:errcheck
		if parsed["actor_type"] != actorType {
			t.Errorf("actor_type round-trip: got %q, want %q", parsed["actor_type"], actorType)
		}
	}
}

// ---------------------------------------------------------------------------
// WorkflowTemplate  ↔  web/lib/types.ts WorkflowTemplate
// ---------------------------------------------------------------------------

func TestWorkflowTemplateResponseContract(t *testing.T) {
	now := time.Now().UTC()
	tmpl := models.WorkflowTemplate{
		ID:          "tmpl-1",
		Name:        "CI Pipeline",
		Description: "Standard CI workflow",
		Version:     3,
		Steps:       json.RawMessage(`[{"id":"s1","instruction":"build"}]`),
		Variables:   json.RawMessage(`[{"name":"branch","type":"string"}]`),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	parsed := assertFields(t, "WorkflowTemplate", tmpl, []string{
		"id", "name", "description", "version", "steps", "variables",
		"is_active", "created_at", "updated_at",
	})

	// steps and variables must be arrays
	if _, ok := parsed["steps"].([]any); !ok {
		t.Errorf("WorkflowTemplate.steps should be array, got %T", parsed["steps"])
	}
	if _, ok := parsed["variables"].([]any); !ok {
		t.Errorf("WorkflowTemplate.variables should be array, got %T", parsed["variables"])
	}
}

func TestWorkflowTemplateResponseContract_NilStepsAndVariables(t *testing.T) {
	tmpl := models.WorkflowTemplate{
		ID:        "tmpl-2",
		Name:      "Minimal",
		Version:   1,
		Steps:     nil,
		Variables: nil,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, _ := json.Marshal(tmpl)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	// Steps and Variables are json.RawMessage without omitempty, so they
	// serialize as null when nil. Frontend uses (tmpl.steps ?? []).
	// The key should still be present.
	if _, ok := parsed["steps"]; !ok {
		t.Error("WorkflowTemplate.steps key should be present even when nil")
	}
	if _, ok := parsed["variables"]; !ok {
		t.Error("WorkflowTemplate.variables key should be present even when nil")
	}
}

// ---------------------------------------------------------------------------
// Policy  ↔  web/lib/types.ts Policy
// ---------------------------------------------------------------------------

func TestPolicyResponseContract(t *testing.T) {
	now := time.Now().UTC()
	pol := models.Policy{
		ID:        "pol-1",
		Name:      "Security Policy",
		Rules:     json.RawMessage(`{"when":{"always":true},"max_subtasks":5}`),
		Priority:  10,
		IsActive:  true,
		CreatedAt: now,
	}

	parsed := assertFields(t, "Policy", pol, []string{
		"id", "name", "rules", "priority", "is_active", "created_at",
	})

	// rules must be an object
	if _, ok := parsed["rules"].(map[string]any); !ok {
		t.Errorf("Policy.rules should be object, got %T", parsed["rules"])
	}
	// priority must be numeric
	if _, ok := parsed["priority"].(float64); !ok {
		t.Errorf("Policy.priority should be number, got %T", parsed["priority"])
	}
}

// ---------------------------------------------------------------------------
// A2AServerConfig  ↔  web/lib/api.ts A2AConfig
// ---------------------------------------------------------------------------

func TestA2AServerConfigResponseContract(t *testing.T) {
	cfg := models.A2AServerConfig{
		ID:             1,
		Enabled:        true,
		SecurityScheme: json.RawMessage(`{"type":"bearer"}`),
		AggregatedCard: json.RawMessage(`{"name":"test"}`),
	}

	parsed := assertFields(t, "A2AServerConfig", cfg, []string{
		"id", "enabled", "security_scheme", "aggregated_card",
	})

	if _, ok := parsed["enabled"].(bool); !ok {
		t.Errorf("A2AServerConfig.enabled should be bool, got %T", parsed["enabled"])
	}
}

func TestA2AServerConfigResponseContract_WithOverrides(t *testing.T) {
	name := "Custom Name"
	desc := "Custom Description"
	now := time.Now().UTC()

	cfg := models.A2AServerConfig{
		ID:                  1,
		Enabled:             true,
		NameOverride:        &name,
		DescriptionOverride: &desc,
		SecurityScheme:      json.RawMessage(`{}`),
		AggregatedCard:      json.RawMessage(`{}`),
		CardUpdatedAt:       &now,
	}

	assertFields(t, "A2AServerConfig(overrides)", cfg, []string{
		"id", "enabled", "name_override", "description_override",
		"security_scheme", "aggregated_card", "card_updated_at",
	})
}

func TestA2AServerConfigResponseContract_OmitEmpty(t *testing.T) {
	cfg := models.A2AServerConfig{
		ID:      1,
		Enabled: false,
	}

	data, _ := json.Marshal(cfg)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	for _, field := range []string{"name_override", "description_override", "card_updated_at"} {
		assertFieldAbsent(t, "A2AServerConfig", parsed, field)
	}
}

// ---------------------------------------------------------------------------
// User  ↔  web/lib/types.ts User
// ---------------------------------------------------------------------------

func TestUserResponseContract(t *testing.T) {
	user := models.User{
		ID:        "user-1",
		Email:     "alice@example.com",
		Name:      "Alice",
		AvatarURL: "https://example.com/avatar.png",
		CreatedAt: time.Now().UTC(),
	}

	assertFields(t, "User", user, []string{
		"id", "email", "name", "avatar_url", "created_at",
	})
}

// ---------------------------------------------------------------------------
// PageResponse  ↔  web/lib/types.ts PageResponse
// ---------------------------------------------------------------------------

func TestPageResponseContract(t *testing.T) {
	next := "cursor-abc"
	resp := models.PageResponse[models.Agent]{
		Items:      []models.Agent{{ID: "a-1", Name: "Agent", Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}},
		NextCursor: &next,
		HasMore:    true,
	}

	parsed := assertFields(t, "PageResponse", resp, []string{
		"items", "next_cursor", "has_more",
	})

	items, ok := parsed["items"].([]any)
	if !ok {
		t.Fatalf("PageResponse.items should be []any, got %T", parsed["items"])
	}
	if len(items) != 1 {
		t.Errorf("PageResponse.items length = %d, want 1", len(items))
	}
}

func TestPageResponseContract_EmptyItems(t *testing.T) {
	resp := models.PageResponse[models.Task]{
		Items:   []models.Task{},
		HasMore: false,
	}

	data, _ := json.Marshal(resp)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	items, ok := parsed["items"].([]any)
	if !ok {
		t.Fatalf("PageResponse.items should be []any even when empty, got %T", parsed["items"])
	}
	if len(items) != 0 {
		t.Errorf("PageResponse.items length = %d, want 0", len(items))
	}
}

// ---------------------------------------------------------------------------
// PaginatedTasks (handler response)  ↔  web/lib/api.ts PaginatedTasks
//
// The task List handler returns map[string]any directly, so we test the
// shape that the handler builds.
// ---------------------------------------------------------------------------

func TestPaginatedTasksResponseContract(t *testing.T) {
	// This mirrors what TaskHandler.List builds
	resp := map[string]any{
		"items":    []models.Task{},
		"total":    0,
		"page":     1,
		"per_page": 20,
		"pages":    0,
	}

	assertFields(t, "PaginatedTasks", resp, []string{
		"items", "total", "page", "per_page", "pages",
	})
}

// ---------------------------------------------------------------------------
// Handler response shape: healthcheck
// ---------------------------------------------------------------------------

func TestHealthcheckResponseContract(t *testing.T) {
	resp := healthcheckResponse{
		Status:    200,
		LatencyMs: 42,
	}

	parsed := assertFields(t, "HealthcheckResponse", resp, []string{
		"status", "latency_ms",
	})

	if _, ok := parsed["status"].(float64); !ok {
		t.Errorf("status should be number, got %T", parsed["status"])
	}
	if _, ok := parsed["latency_ms"].(float64); !ok {
		t.Errorf("latency_ms should be number, got %T", parsed["latency_ms"])
	}
}

// ---------------------------------------------------------------------------
// TemplateVersion & TemplateExecution — used by template detail/executions
// ---------------------------------------------------------------------------

func TestTemplateVersionResponseContract(t *testing.T) {
	tv := models.TemplateVersion{
		ID:                "tv-1",
		TemplateID:        "tmpl-1",
		Version:           2,
		Steps:             json.RawMessage(`[{"id":"s1"}]`),
		Source:            "evolution",
		Changes:           json.RawMessage(`["added retry logic"]`),
		BasedOnExecutions: 10,
		CreatedAt:         time.Now().UTC(),
	}

	assertFields(t, "TemplateVersion", tv, []string{
		"id", "template_id", "version", "steps", "source",
		"changes", "based_on_executions", "created_at",
	})
}

func TestTemplateExecutionResponseContract(t *testing.T) {
	te := models.TemplateExecution{
		ID:                "te-1",
		TemplateID:        "tmpl-1",
		TemplateVersion:   2,
		TaskID:            "task-1",
		ActualSteps:       json.RawMessage(`[{"id":"s1","status":"completed"}]`),
		HITLInterventions: 1,
		ReplanCount:       0,
		Outcome:           "success",
		DurationSeconds:   120,
		CreatedAt:         time.Now().UTC(),
	}

	assertFields(t, "TemplateExecution", te, []string{
		"id", "template_id", "template_version", "task_id",
		"actual_steps", "hitl_interventions", "replan_count",
		"outcome", "duration_seconds", "created_at",
	})
}

// ---------------------------------------------------------------------------
// ExecutionPlan — used internally but also returned in task.plan
// ---------------------------------------------------------------------------

func TestExecutionPlanResponseContract(t *testing.T) {
	plan := models.ExecutionPlan{
		Summary: "Review and deploy",
		SubTasks: []models.PlanSubTask{
			{
				ID:          "ps-1",
				AgentID:     "agent-1",
				AgentName:   "Reviewer",
				Instruction: "Review the code",
				DependsOn:   []string{},
			},
			{
				ID:          "ps-2",
				AgentID:     "agent-2",
				AgentName:   "Deployer",
				Instruction: "Deploy to staging",
				DependsOn:   []string{"ps-1"},
			},
		},
	}

	parsed := assertFields(t, "ExecutionPlan", plan, []string{
		"summary", "subtasks",
	})

	subtasks, ok := parsed["subtasks"].([]any)
	if !ok {
		t.Fatalf("ExecutionPlan.subtasks should be []any, got %T", parsed["subtasks"])
	}
	if len(subtasks) != 2 {
		t.Errorf("ExecutionPlan.subtasks length = %d, want 2", len(subtasks))
	}

	// Verify PlanSubTask fields
	first, ok := subtasks[0].(map[string]any)
	if !ok {
		t.Fatalf("PlanSubTask should be object, got %T", subtasks[0])
	}
	for _, field := range []string{"id", "agent_id", "agent_name", "instruction", "depends_on"} {
		if _, exists := first[field]; !exists {
			t.Errorf("PlanSubTask missing field %q", field)
		}
	}
}

// ---------------------------------------------------------------------------
// Null Array Serialization — Cross-cutting concern
//
// Go marshals nil slices as JSON null, not []. The frontend must use ?? []
// for any array field that can be nil. This test documents which fields are
// affected and ensures the serialization behavior doesn't change silently.
// ---------------------------------------------------------------------------

func TestNullArraySerialization(t *testing.T) {
	tests := []struct {
		name  string
		value any
		field string
	}{
		{
			"Agent.Capabilities(nil)",
			models.Agent{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
			"capabilities",
		},
		{
			"SubTask.DependsOn(nil)",
			models.SubTask{CreatedAt: time.Now().UTC()},
			"depends_on",
		},
		{
			"Message.Mentions(nil)",
			models.Message{CreatedAt: time.Now().UTC()},
			"mentions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var parsed map[string]any
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			val, exists := parsed[tt.field]
			if !exists {
				// Field with omitempty omitted — frontend handles undefined too
				t.Logf("%s: field omitted (omitempty) — frontend must handle undefined", tt.field)
				return
			}
			if val == nil {
				// null — frontend must use ?? []
				t.Logf("%s: serialized as null — frontend must use ?? []", tt.field)
			} else if _, ok := val.([]any); ok {
				// Already an array — safe
				t.Logf("%s: serialized as array — safe", tt.field)
			} else {
				t.Errorf("%s: unexpected type %T", tt.field, val)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JSON Null vs Omitted — Verify omitempty behavior for optional pointer fields
//
// This documents which fields are omitted vs set to null, ensuring the
// frontend type definitions (using ?) match the actual serialization.
// ---------------------------------------------------------------------------

func TestOptionalFieldOmission(t *testing.T) {
	// Build a minimal Task with all optional fields unset
	task := models.Task{
		ID:        "t-1",
		Title:     "Test",
		Status:    "pending",
		CreatedBy: "user-1",
		CreatedAt: time.Now().UTC(),
	}

	data, _ := json.Marshal(task)
	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw) //nolint:errcheck

	// These optional fields in the TS interface use ? — they should be omitted
	optionalOmitted := []string{
		"metadata", "plan", "result", "error",
		"completed_at", "template_id",
	}
	for _, field := range optionalOmitted {
		if _, exists := raw[field]; exists {
			t.Errorf("Task.%s should be omitted when zero (omitempty), but present in JSON", field)
		}
	}

	// These always-present fields should exist
	alwaysPresent := []string{"id", "title", "status", "created_by", "replan_count", "created_at"}
	for _, field := range alwaysPresent {
		if _, exists := raw[field]; !exists {
			t.Errorf("Task.%s should always be present in JSON", field)
		}
	}
}

// ---------------------------------------------------------------------------
// Round-trip: marshal → unmarshal preserves all fields
// ---------------------------------------------------------------------------

func TestAgentRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	healthCheck := now.Add(-5 * time.Minute)

	original := models.Agent{
		ID:              "agent-rt",
		Name:            "Round Trip Agent",
		Version:         "2.1.0",
		Description:     "Tests round-trip",
		Endpoint:        "http://localhost:9001",
		AgentCardURL:    "http://localhost:9001/.well-known/agent-card.json",
		Capabilities:    []string{"streaming", "pushNotifications"},
		Status:          "active",
		IsOnline:        true,
		LastHealthCheck: &healthCheck,
		SkillHash:       "sha256-abc",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded models.Agent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version: got %q, want %q", decoded.Version, original.Version)
	}
	if decoded.IsOnline != original.IsOnline {
		t.Errorf("IsOnline: got %v, want %v", decoded.IsOnline, original.IsOnline)
	}
	if len(decoded.Capabilities) != 2 {
		t.Errorf("Capabilities length: got %d, want 2", len(decoded.Capabilities))
	}
	if decoded.SkillHash != original.SkillHash {
		t.Errorf("SkillHash: got %q, want %q", decoded.SkillHash, original.SkillHash)
	}
	if decoded.LastHealthCheck == nil {
		t.Error("LastHealthCheck should not be nil after round-trip")
	} else if !decoded.LastHealthCheck.Truncate(time.Millisecond).Equal(healthCheck) {
		t.Errorf("LastHealthCheck: got %v, want %v", decoded.LastHealthCheck, healthCheck)
	}
}

func TestTaskRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	completedAt := now.Add(5 * time.Minute)

	original := models.Task{
		ID:              "task-rt",
		Title:           "Round Trip Task",
		Description:     "Tests round-trip fidelity",
		Status:          "completed",
		CreatedBy:       "user-1",
		Metadata:        json.RawMessage(`{"priority":"high"}`),
		Plan:            json.RawMessage(`{"summary":"test plan"}`),
		Result:          json.RawMessage(`{"output":"success"}`),
		TemplateID:      "tmpl-rt",
		TemplateVersion: 5,
		ReplanCount:     2,
		CreatedAt:       now,
		CompletedAt:     &completedAt,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded models.Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.TemplateID != original.TemplateID {
		t.Errorf("TemplateID: got %q, want %q", decoded.TemplateID, original.TemplateID)
	}
	if decoded.TemplateVersion != original.TemplateVersion {
		t.Errorf("TemplateVersion: got %d, want %d", decoded.TemplateVersion, original.TemplateVersion)
	}
	if decoded.ReplanCount != original.ReplanCount {
		t.Errorf("ReplanCount: got %d, want %d", decoded.ReplanCount, original.ReplanCount)
	}
	if decoded.CompletedAt == nil {
		t.Error("CompletedAt should not be nil after round-trip")
	}
}

func TestSubTaskRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	startedAt := now.Add(time.Minute)

	original := models.SubTask{
		ID:          "st-rt",
		TaskID:      "task-rt",
		AgentID:     "agent-rt",
		Instruction: "Do the thing",
		DependsOn:   []string{"st-0", "st-1"},
		Status:      "running",
		Input:       json.RawMessage(`{"file":"main.go"}`),
		A2ATaskID:   "a2a-123",
		Attempt:     2,
		MaxAttempts: 5,
		CreatedAt:   now,
		StartedAt:   &startedAt,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded models.SubTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(decoded.DependsOn) != 2 {
		t.Errorf("DependsOn length: got %d, want 2", len(decoded.DependsOn))
	}
	if decoded.A2ATaskID != original.A2ATaskID {
		t.Errorf("A2ATaskID: got %q, want %q", decoded.A2ATaskID, original.A2ATaskID)
	}
	if decoded.Attempt != original.Attempt {
		t.Errorf("Attempt: got %d, want %d", decoded.Attempt, original.Attempt)
	}
	if decoded.StartedAt == nil {
		t.Error("StartedAt should not be nil after round-trip")
	}
}

// ---------------------------------------------------------------------------
// Conversation  ↔  web/lib/types.ts Conversation
// ---------------------------------------------------------------------------

func TestConversationResponseContract(t *testing.T) {
	now := time.Now().UTC()
	conv := models.Conversation{
		ID:        "conv-1",
		Title:     "Test Conversation",
		CreatedBy: "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	assertFields(t, "Conversation", conv, []string{
		"id", "title", "created_by", "created_at", "updated_at",
	})
}

// ---------------------------------------------------------------------------
// ConversationListItem  ↔  web/lib/types.ts ConversationListItem
// ---------------------------------------------------------------------------

func TestConversationListItemResponseContract(t *testing.T) {
	item := models.ConversationListItem{
		ID:           "conv-1",
		Title:        "Deal Review",
		AgentCount:   3,
		TaskCount:    2,
		LatestStatus: "running",
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	parsed := assertFields(t, "ConversationListItem", item, []string{
		"id", "title", "agent_count", "task_count", "latest_status", "updated_at",
	})

	// Numeric fields must serialize as numbers
	if _, ok := parsed["agent_count"].(float64); !ok {
		t.Errorf("agent_count should be number, got %T", parsed["agent_count"])
	}
	if _, ok := parsed["task_count"].(float64); !ok {
		t.Errorf("task_count should be number, got %T", parsed["task_count"])
	}
}

func TestConversationListItemResponseContract_EmptyStatus(t *testing.T) {
	item := models.ConversationListItem{
		ID:           "conv-2",
		Title:        "New",
		AgentCount:   0,
		TaskCount:    0,
		LatestStatus: "", // no tasks yet
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	parsed := assertFields(t, "ConversationListItem(empty)", item, []string{
		"id", "title", "agent_count", "task_count", "latest_status", "updated_at",
	})

	if parsed["latest_status"] != "" {
		t.Errorf("latest_status should be empty string, got %q", parsed["latest_status"])
	}
}

// ---------------------------------------------------------------------------
// WebhookConfig (handler-local struct)  ↔  web/lib/types.ts WebhookConfig
// ---------------------------------------------------------------------------

func TestWebhookConfigResponseContract(t *testing.T) {
	cfg := webhookConfig{
		ID:        "wh-1",
		Name:      "Deploy Notifier",
		URL:       "https://example.com/webhook",
		Events:    []string{"task.completed", "task.failed"},
		IsActive:  true,
		Secret:    "***",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	parsed := assertFields(t, "WebhookConfig", cfg, []string{
		"id", "name", "url", "events", "is_active", "secret", "created_at",
	})

	// events must be array
	events, ok := parsed["events"].([]any)
	if !ok {
		t.Fatalf("WebhookConfig.events should be []any, got %T", parsed["events"])
	}
	if len(events) != 2 {
		t.Errorf("WebhookConfig.events length = %d, want 2", len(events))
	}
}

// ---------------------------------------------------------------------------
// TimelineEvent (handler-local struct)  ↔  web/lib/types.ts TimelineEvent
// ---------------------------------------------------------------------------

func TestTimelineEventResponseContract(t *testing.T) {
	evt := timelineEvent{
		ID:        "te-1",
		TaskID:    "task-1",
		SubtaskID: "st-1",
		Type:      "subtask.started",
		ActorType: "system",
		ActorID:   "system",
		Data:      json.RawMessage(`{"status":"running"}`),
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}

	parsed := assertFields(t, "TimelineEvent", evt, []string{
		"id", "task_id", "type", "actor_type", "data", "created_at",
	})

	// data must be an object
	if _, ok := parsed["data"].(map[string]any); !ok {
		t.Errorf("TimelineEvent.data should be object, got %T", parsed["data"])
	}
}

// ---------------------------------------------------------------------------
// ConversationID field on Task, Message, Event
// ---------------------------------------------------------------------------

func TestTaskConversationIDContract(t *testing.T) {
	now := time.Now().UTC()
	task := models.Task{
		ID:             "task-conv",
		ConversationID: "conv-1",
		Title:          "Task in conversation",
		Status:         "pending",
		CreatedBy:      "user-1",
		CreatedAt:      now,
	}

	parsed := assertFields(t, "Task(with conversation)", task, []string{
		"id", "conversation_id", "title", "status",
	})

	if parsed["conversation_id"] != "conv-1" {
		t.Errorf("conversation_id = %v, want %q", parsed["conversation_id"], "conv-1")
	}
}

func TestTaskConversationIDOmitted(t *testing.T) {
	task := models.Task{
		ID:        "task-no-conv",
		Title:     "Standalone task",
		Status:    "pending",
		CreatedBy: "user-1",
		CreatedAt: time.Now().UTC(),
	}

	data, _ := json.Marshal(task)
	var parsed map[string]any
	json.Unmarshal(data, &parsed) //nolint:errcheck

	// conversation_id has omitempty — should be absent for standalone tasks
	assertFieldAbsent(t, "Task(standalone)", parsed, "conversation_id")
}

func TestMessageConversationIDContract(t *testing.T) {
	msg := models.Message{
		ID:             "msg-conv",
		ConversationID: "conv-1",
		SenderType:     "user",
		SenderName:     "Alice",
		Content:        "Hello",
		CreatedAt:      time.Now().UTC(),
	}

	parsed := assertFields(t, "Message(with conversation)", msg, []string{
		"id", "conversation_id", "sender_type", "content",
	})

	if parsed["conversation_id"] != "conv-1" {
		t.Errorf("conversation_id = %v, want %q", parsed["conversation_id"], "conv-1")
	}
}

func TestEventConversationIDContract(t *testing.T) {
	evt := models.Event{
		ID:             "evt-conv",
		TaskID:         "task-1",
		ConversationID: "conv-1",
		Type:           "message",
		ActorType:      "user",
		Data:           json.RawMessage(`{}`),
		CreatedAt:      time.Now().UTC(),
	}

	parsed := assertFields(t, "Event(with conversation)", evt, []string{
		"id", "task_id", "conversation_id", "type", "actor_type",
	})

	if parsed["conversation_id"] != "conv-1" {
		t.Errorf("conversation_id = %v, want %q", parsed["conversation_id"], "conv-1")
	}
}

// ---------------------------------------------------------------------------
// DashboardData (handler response)  ↔  web/lib/types.ts DashboardData
// ---------------------------------------------------------------------------

func TestDashboardDataResponseContract(t *testing.T) {
	// This mirrors what AnalyticsHandler.GetDashboard builds
	resp := map[string]any{
		"total_tasks":         10,
		"completed_tasks":     7,
		"failed_tasks":        1,
		"running_tasks":       2,
		"success_rate":        0.7,
		"total_agents":        5,
		"online_agents":       3,
		"avg_duration_sec":    45.2,
		"status_distribution": []map[string]any{},
		"daily_tasks":         []map[string]any{},
		"agent_usage":         []map[string]any{},
	}

	assertFields(t, "DashboardData", resp, []string{
		"total_tasks", "completed_tasks", "failed_tasks", "running_tasks",
		"success_rate", "total_agents", "online_agents", "avg_duration_sec",
		"status_distribution", "daily_tasks", "agent_usage",
	})
}
