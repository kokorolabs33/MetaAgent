package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTask_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	task := Task{
		ID:              "task-123",
		Title:           "Review code",
		Description:     "Review the frontend code",
		Status:          "pending",
		CreatedBy:       "user-1",
		TemplateID:      "tmpl-456",
		TemplateVersion: 2,
		ReplanCount:     1,
		CreatedAt:       now,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify key JSON field names
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "title", "description", "status", "created_by",
		"template_id", "template_version", "replan_count", "created_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}

	// Round-trip
	var decoded Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != task.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, task.ID)
	}
	if decoded.TemplateID != task.TemplateID {
		t.Errorf("TemplateID = %q, want %q", decoded.TemplateID, task.TemplateID)
	}
	if decoded.TemplateVersion != task.TemplateVersion {
		t.Errorf("TemplateVersion = %d, want %d", decoded.TemplateVersion, task.TemplateVersion)
	}
	if decoded.ReplanCount != task.ReplanCount {
		t.Errorf("ReplanCount = %d, want %d", decoded.ReplanCount, task.ReplanCount)
	}
}

func TestTask_JSONOmitEmpty(t *testing.T) {
	task := Task{
		ID:        "task-1",
		Title:     "Test",
		Status:    "pending",
		CreatedBy: "user",
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	// These fields should be omitted when empty
	omitFields := []string{"metadata", "plan", "result", "error", "completed_at"}
	for _, field := range omitFields {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when empty, but present", field)
		}
	}
}

func TestTask_WithCompletedAt(t *testing.T) {
	now := time.Now().UTC()
	task := Task{
		ID:          "task-1",
		Title:       "Done",
		Status:      "completed",
		CreatedBy:   "user",
		CreatedAt:   now,
		CompletedAt: &now,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	if _, ok := raw["completed_at"]; !ok {
		t.Error("completed_at should be present when set")
	}
}

func TestAgent_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	healthCheck := now.Add(-5 * time.Minute)
	agent := Agent{
		ID:              "agent-123",
		Name:            "Test Agent",
		Version:         "1.0.0",
		Description:     "A test agent",
		Endpoint:        "http://localhost:8080",
		AgentCardURL:    "http://localhost:8080/.well-known/agent-card.json",
		Capabilities:    []string{"code-review", "testing"},
		Status:          "active",
		IsOnline:        true,
		LastHealthCheck: &healthCheck,
		SkillHash:       "abc123",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Verify key JSON field names
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "name", "version", "description", "endpoint",
		"agent_card_url", "capabilities", "status",
		"is_online", "last_health_check", "skill_hash",
		"created_at", "updated_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}

	// Round-trip
	var decoded Agent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.IsOnline != true {
		t.Error("IsOnline should be true")
	}
	if decoded.SkillHash != "abc123" {
		t.Errorf("SkillHash = %q, want %q", decoded.SkillHash, "abc123")
	}
	if decoded.LastHealthCheck == nil {
		t.Error("LastHealthCheck should not be nil")
	}
}

func TestAgent_JSONOmitEmpty(t *testing.T) {
	agent := Agent{
		ID:        "agent-1",
		Name:      "Agent",
		Status:    "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	omitFields := []string{"agent_card", "card_fetched_at", "last_health_check", "skills"}
	for _, field := range omitFields {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when empty, but present", field)
		}
	}
}

func TestWorkflowTemplate_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tmpl := WorkflowTemplate{
		ID:           "tmpl-123",
		Name:         "Code Review Pipeline",
		Description:  "Standard code review workflow",
		Version:      3,
		Steps:        json.RawMessage(`[{"id":"s1","instruction":"review code"}]`),
		Variables:    json.RawMessage(`[{"name":"repo","type":"string"}]`),
		SourceTaskID: "task-orig",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "name", "description", "version", "steps",
		"variables", "source_task_id", "is_active",
		"created_at", "updated_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}

	// Round-trip
	var decoded WorkflowTemplate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Version != 3 {
		t.Errorf("Version = %d, want 3", decoded.Version)
	}
	if decoded.SourceTaskID != "task-orig" {
		t.Errorf("SourceTaskID = %q, want %q", decoded.SourceTaskID, "task-orig")
	}
	if !decoded.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestPolicy_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	policy := Policy{
		ID:        "pol-123",
		Name:      "Security Policy",
		Rules:     json.RawMessage(`{"when":{"always":true},"max_subtasks":5}`),
		Priority:  10,
		IsActive:  true,
		CreatedAt: now,
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "name", "rules", "priority", "is_active", "created_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}

	// Round-trip
	var decoded Policy
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Priority != 10 {
		t.Errorf("Priority = %d, want 10", decoded.Priority)
	}
	if !decoded.IsActive {
		t.Error("IsActive should be true")
	}
}

func TestA2AServerConfig_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	nameOverride := "Custom Name"
	descOverride := "Custom Description"

	config := A2AServerConfig{
		ID:                  1,
		Enabled:             true,
		NameOverride:        &nameOverride,
		DescriptionOverride: &descOverride,
		SecurityScheme:      json.RawMessage(`{"type":"bearer"}`),
		AggregatedCard:      json.RawMessage(`{"name":"test"}`),
		CardUpdatedAt:       &now,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "enabled", "name_override", "description_override",
		"security_scheme", "aggregated_card", "card_updated_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}

	// Round-trip
	var decoded A2AServerConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !decoded.Enabled {
		t.Error("Enabled should be true")
	}
	if decoded.NameOverride == nil || *decoded.NameOverride != "Custom Name" {
		t.Error("NameOverride should be 'Custom Name'")
	}
}

func TestA2AServerConfig_JSONOmitEmpty(t *testing.T) {
	config := A2AServerConfig{
		ID:      1,
		Enabled: false,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	omitFields := []string{"name_override", "description_override", "card_updated_at"}
	for _, field := range omitFields {
		if _, ok := raw[field]; ok {
			t.Errorf("field %q should be omitted when nil, but present", field)
		}
	}
}

func TestSubTask_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	st := SubTask{
		ID:          "st-1",
		TaskID:      "task-1",
		AgentID:     "agent-1",
		Instruction: "Do something",
		DependsOn:   []string{"st-0"},
		Status:      "pending",
		A2ATaskID:   "a2a-task-1",
		Attempt:     1,
		MaxAttempts: 3,
		CreatedAt:   now,
	}

	data, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "task_id", "agent_id", "instruction", "depends_on",
		"status", "a2a_task_id", "attempt", "max_attempts", "created_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
}

func TestExecutionPlan_JSONMarshal(t *testing.T) {
	plan := ExecutionPlan{
		Summary: "Test plan",
		SubTasks: []PlanSubTask{
			{
				ID:          "s1",
				AgentID:     "agent-1",
				AgentName:   "Agent One",
				Instruction: "Do first thing",
				DependsOn:   []string{},
			},
			{
				ID:          "s2",
				AgentID:     "agent-2",
				AgentName:   "Agent Two",
				Instruction: "Do second thing",
				DependsOn:   []string{"s1"},
			},
		},
	}

	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ExecutionPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Summary != "Test plan" {
		t.Errorf("Summary = %q, want %q", decoded.Summary, "Test plan")
	}
	if len(decoded.SubTasks) != 2 {
		t.Fatalf("SubTasks length = %d, want 2", len(decoded.SubTasks))
	}
	if decoded.SubTasks[1].DependsOn[0] != "s1" {
		t.Errorf("SubTasks[1].DependsOn = %v, want [s1]", decoded.SubTasks[1].DependsOn)
	}
}

func TestEvent_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	event := Event{
		ID:        "evt-1",
		TaskID:    "task-1",
		SubtaskID: "st-1",
		Type:      "agent_working",
		ActorType: "agent",
		ActorID:   "agent-1",
		Data:      json.RawMessage(`{"progress":50}`),
		CreatedAt: now,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "task_id", "type", "actor_type", "data", "created_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
}

func TestMessage_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	msg := Message{
		ID:         "msg-1",
		TaskID:     "task-1",
		SenderType: "agent",
		SenderID:   "agent-1",
		SenderName: "Code Reviewer",
		Content:    "Found 3 issues",
		Mentions:   []string{"agent-2"},
		CreatedAt:  now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.SenderName != "Code Reviewer" {
		t.Errorf("SenderName = %q, want %q", decoded.SenderName, "Code Reviewer")
	}
	if len(decoded.Mentions) != 1 || decoded.Mentions[0] != "agent-2" {
		t.Errorf("Mentions = %v, want [agent-2]", decoded.Mentions)
	}
}

func TestTemplateVersion_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tv := TemplateVersion{
		ID:                "tv-1",
		TemplateID:        "tmpl-1",
		Version:           2,
		Steps:             json.RawMessage(`[{"id":"s1"}]`),
		Source:            "user_edit",
		Changes:           json.RawMessage(`["updated steps"]`),
		BasedOnExecutions: 5,
		CreatedAt:         now,
	}

	data, err := json.Marshal(tv)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "template_id", "version", "steps", "source",
		"changes", "based_on_executions", "created_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
}

func TestTemplateExecution_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	te := TemplateExecution{
		ID:                "te-1",
		TemplateID:        "tmpl-1",
		TemplateVersion:   2,
		TaskID:            "task-1",
		ActualSteps:       json.RawMessage(`[{"id":"s1","status":"completed"}]`),
		HITLInterventions: 1,
		ReplanCount:       0,
		Outcome:           "success",
		DurationSeconds:   120,
		CreatedAt:         now,
	}

	data, err := json.Marshal(te)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	requiredFields := []string{
		"id", "template_id", "template_version", "task_id",
		"actual_steps", "hitl_interventions", "replan_count",
		"outcome", "duration_seconds", "created_at",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field %q", field)
		}
	}
}

func TestPageRequest_Defaults(t *testing.T) {
	tests := []struct {
		name      string
		cursor    string
		limit     int
		wantLimit int
	}{
		{"zero limit defaults to 20", "", 0, 20},
		{"negative limit defaults to 20", "", -1, 20},
		{"over 100 defaults to 20", "", 101, 20},
		{"valid limit preserved", "", 50, 50},
		{"limit 1 is valid", "", 1, 1},
		{"limit 100 is valid", "", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := NewPageRequest(tt.cursor, tt.limit)
			if pr.Limit != tt.wantLimit {
				t.Errorf("NewPageRequest(%q, %d).Limit = %d, want %d",
					tt.cursor, tt.limit, pr.Limit, tt.wantLimit)
			}
		})
	}
}

func TestTaskWithSubtasks_JSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	tws := TaskWithSubtasks{
		Task: Task{
			ID:        "task-1",
			Title:     "Test",
			Status:    "running",
			CreatedBy: "user",
			CreatedAt: now,
		},
		SubTasks: []SubTask{
			{
				ID:          "st-1",
				TaskID:      "task-1",
				AgentID:     "agent-1",
				Instruction: "Do work",
				DependsOn:   []string{},
				Status:      "pending",
				Attempt:     0,
				MaxAttempts: 3,
				CreatedAt:   now,
			},
		},
	}

	data, err := json.Marshal(tws)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	if _, ok := raw["subtasks"]; !ok {
		t.Error("missing subtasks field in TaskWithSubtasks JSON")
	}
	if _, ok := raw["id"]; !ok {
		t.Error("missing embedded Task.id field")
	}
}
