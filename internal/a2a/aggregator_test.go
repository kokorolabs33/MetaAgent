package a2a

import (
	"encoding/json"
	"testing"
)

func TestAggregator_Invalidate(t *testing.T) {
	agg := &Aggregator{}

	// Manually set cache fields to simulate a cached state
	agg.cardJSON = []byte(`{"name":"test"}`)
	agg.etag = `"abc123"`
	agg.card = &AggregatedCard{Name: "test"}

	agg.Invalidate()

	if agg.cardJSON != nil {
		t.Errorf("after Invalidate(), cardJSON = %v, want nil", agg.cardJSON)
	}
	if agg.card != nil {
		t.Error("after Invalidate(), card should be nil")
	}
	if agg.etag != "" {
		t.Errorf("after Invalidate(), etag = %q, want empty", agg.etag)
	}
	if !agg.builtAt.IsZero() {
		t.Errorf("after Invalidate(), builtAt = %v, want zero", agg.builtAt)
	}
}

func TestAggregator_InvalidateIdempotent(t *testing.T) {
	agg := &Aggregator{}

	// Calling Invalidate on an already-empty aggregator should not panic
	agg.Invalidate()
	agg.Invalidate()

	if agg.cardJSON != nil {
		t.Error("double Invalidate should leave cardJSON nil")
	}
}

func TestNewAggregator(t *testing.T) {
	agg := NewAggregator(nil)
	if agg == nil {
		t.Fatal("NewAggregator returned nil")
	}
	if agg.DB != nil {
		t.Error("NewAggregator(nil).DB should be nil")
	}
	if agg.cardJSON != nil {
		t.Error("initial cardJSON should be nil")
	}
}

func TestCardInterface_JSON(t *testing.T) {
	ci := CardInterface{
		URL:             "https://example.com/a2a",
		ProtocolBinding: "jsonrpc-over-http",
	}

	data, err := json.Marshal(ci)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded CardInterface
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.URL != ci.URL {
		t.Errorf("URL = %q, want %q", decoded.URL, ci.URL)
	}
	if decoded.ProtocolBinding != ci.ProtocolBinding {
		t.Errorf("ProtocolBinding = %q, want %q", decoded.ProtocolBinding, ci.ProtocolBinding)
	}
}

func TestAggregatedCard_JSON(t *testing.T) {
	card := AggregatedCard{
		Name:        "TaskHub Meta-Agent",
		Description: "An orchestrating agent",
		URL:         "https://example.com/a2a",
		Version:     "1.0.0",
		SupportedInterfaces: []CardInterface{
			{
				URL:             "https://example.com/a2a",
				ProtocolBinding: "jsonrpc-over-http",
			},
		},
		Capabilities: CardCapability{
			Streaming:              false,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		Skills: []CardSkill{
			{ID: "s1", Name: "Code Review", Description: "Reviews code"},
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
	}

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded AggregatedCard
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Name != card.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, card.Name)
	}
	if decoded.Version != card.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, card.Version)
	}
	if len(decoded.SupportedInterfaces) != 1 {
		t.Fatalf("SupportedInterfaces length = %d, want 1", len(decoded.SupportedInterfaces))
	}
	if decoded.SupportedInterfaces[0].ProtocolBinding != "jsonrpc-over-http" {
		t.Errorf("ProtocolBinding = %q, want %q",
			decoded.SupportedInterfaces[0].ProtocolBinding, "jsonrpc-over-http")
	}
	if len(decoded.Skills) != 1 {
		t.Fatalf("Skills length = %d, want 1", len(decoded.Skills))
	}
	if decoded.Skills[0].ID != "s1" {
		t.Errorf("Skills[0].ID = %q, want %q", decoded.Skills[0].ID, "s1")
	}
	if len(decoded.DefaultInputModes) != 1 || decoded.DefaultInputModes[0] != "text" {
		t.Errorf("DefaultInputModes = %v, want [text]", decoded.DefaultInputModes)
	}
}

func TestCardCapability_JSON(t *testing.T) {
	cap := CardCapability{
		Streaming:              true,
		PushNotifications:      true,
		StateTransitionHistory: false,
	}

	data, err := json.Marshal(cap)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded CardCapability
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !decoded.Streaming {
		t.Error("Streaming should be true")
	}
	if !decoded.PushNotifications {
		t.Error("PushNotifications should be true")
	}
	if decoded.StateTransitionHistory {
		t.Error("StateTransitionHistory should be false")
	}
}

func TestCardSkill_JSON(t *testing.T) {
	skill := CardSkill{
		ID:          "code-review",
		Name:        "Code Review",
		Description: "Reviews code for best practices",
	}

	data, err := json.Marshal(skill)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded CardSkill
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != skill.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, skill.ID)
	}
	if decoded.Name != skill.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, skill.Name)
	}
	if decoded.Description != skill.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, skill.Description)
	}
}
