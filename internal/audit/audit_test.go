package audit

import (
	"encoding/json"
	"testing"
)

func TestEntry_JSONSerialization(t *testing.T) {
	e := Entry{
		TaskID:       "task-1",
		SubtaskID:    "sub-1",
		AgentID:      "agent-1",
		ActorType:    "agent",
		ActorID:      "actor-1",
		Action:       "llm_call",
		ResourceType: "subtask",
		ResourceID:   "sub-1",
		Details:      map[string]string{"prompt": "hello"},
		Model:        "claude-3-opus",
		InputTokens:  100,
		OutputTokens: 200,
		CostUSD:      0.015,
		Endpoint:     "https://api.anthropic.com/v1/messages",
		LatencyMs:    1500,
		StatusCode:   200,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Entry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.TaskID != e.TaskID {
		t.Errorf("TaskID = %q, want %q", got.TaskID, e.TaskID)
	}
	if got.SubtaskID != e.SubtaskID {
		t.Errorf("SubtaskID = %q, want %q", got.SubtaskID, e.SubtaskID)
	}
	if got.AgentID != e.AgentID {
		t.Errorf("AgentID = %q, want %q", got.AgentID, e.AgentID)
	}
	if got.ActorType != e.ActorType {
		t.Errorf("ActorType = %q, want %q", got.ActorType, e.ActorType)
	}
	if got.ActorID != e.ActorID {
		t.Errorf("ActorID = %q, want %q", got.ActorID, e.ActorID)
	}
	if got.Action != e.Action {
		t.Errorf("Action = %q, want %q", got.Action, e.Action)
	}
	if got.ResourceType != e.ResourceType {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, e.ResourceType)
	}
	if got.ResourceID != e.ResourceID {
		t.Errorf("ResourceID = %q, want %q", got.ResourceID, e.ResourceID)
	}
	if got.Model != e.Model {
		t.Errorf("Model = %q, want %q", got.Model, e.Model)
	}
	if got.InputTokens != e.InputTokens {
		t.Errorf("InputTokens = %d, want %d", got.InputTokens, e.InputTokens)
	}
	if got.OutputTokens != e.OutputTokens {
		t.Errorf("OutputTokens = %d, want %d", got.OutputTokens, e.OutputTokens)
	}
	if got.CostUSD != e.CostUSD {
		t.Errorf("CostUSD = %f, want %f", got.CostUSD, e.CostUSD)
	}
	if got.Endpoint != e.Endpoint {
		t.Errorf("Endpoint = %q, want %q", got.Endpoint, e.Endpoint)
	}
	if got.LatencyMs != e.LatencyMs {
		t.Errorf("LatencyMs = %d, want %d", got.LatencyMs, e.LatencyMs)
	}
	if got.StatusCode != e.StatusCode {
		t.Errorf("StatusCode = %d, want %d", got.StatusCode, e.StatusCode)
	}
}

func TestEntry_DetailsNil(t *testing.T) {
	e := Entry{
		TaskID:    "task-1",
		ActorType: "system",
		ActorID:   "system",
		Action:    "task_created",
		Details:   nil,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Details should be null when nil
	if got["Details"] != nil {
		t.Errorf("expected nil Details, got %v", got["Details"])
	}
}

func TestEntry_DetailsComplex(t *testing.T) {
	details := map[string]any{
		"prompt": "analyze this",
		"tokens": float64(42),
		"nested": map[string]any{
			"key": "value",
		},
	}

	e := Entry{
		Details: details,
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Entry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Details should round-trip correctly
	detailsMap, ok := got.Details.(map[string]any)
	if !ok {
		t.Fatalf("Details is not map[string]any: %T", got.Details)
	}
	if detailsMap["prompt"] != "analyze this" {
		t.Errorf("Details.prompt = %v, want 'analyze this'", detailsMap["prompt"])
	}
}

func TestEntry_ZeroValues(t *testing.T) {
	e := Entry{}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal zero entry: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON for zero entry")
	}

	var got Entry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal zero entry: %v", err)
	}
	if got.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", got.InputTokens)
	}
	if got.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", got.CostUSD)
	}
}
