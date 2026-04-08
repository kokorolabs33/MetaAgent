package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"taskhub/internal/models"
)

func TestBuildAgentDescription(t *testing.T) {
	tests := []struct {
		name   string
		agents []models.Agent
		checks []string
	}{
		{
			name:   "empty list",
			agents: []models.Agent{},
			checks: nil,
		},
		{
			name: "single agent with capabilities",
			agents: []models.Agent{
				{
					ID:           "agent-1",
					Name:         "Code Reviewer",
					Description:  "Reviews code for best practices",
					Capabilities: []string{"code-review", "testing"},
				},
			},
			checks: []string{
				"ID: agent-1",
				"Name: Code Reviewer",
				"Capabilities: code-review, testing",
				"Description: Reviews code for best practices",
			},
		},
		{
			name: "agent without capabilities",
			agents: []models.Agent{
				{
					ID:           "agent-2",
					Name:         "Writer",
					Description:  "Writes documentation",
					Capabilities: nil,
				},
			},
			checks: []string{
				"ID: agent-2",
				"Capabilities: none",
			},
		},
		{
			name: "agent with empty capabilities",
			agents: []models.Agent{
				{
					ID:           "agent-3",
					Name:         "Helper",
					Description:  "Helps out",
					Capabilities: []string{},
				},
			},
			checks: []string{
				"Capabilities: none",
			},
		},
		{
			name: "multiple agents",
			agents: []models.Agent{
				{
					ID:           "a1",
					Name:         "Agent One",
					Description:  "First",
					Capabilities: []string{"cap1"},
				},
				{
					ID:           "a2",
					Name:         "Agent Two",
					Description:  "Second",
					Capabilities: []string{"cap2"},
				},
			},
			checks: []string{
				"ID: a1",
				"ID: a2",
				"Agent One",
				"Agent Two",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAgentDescription(tt.agents)

			if len(tt.agents) == 0 {
				if got != "" {
					t.Errorf("buildAgentDescription([]) = %q, want empty", got)
				}
				return
			}

			for _, check := range tt.checks {
				if !strings.Contains(got, check) {
					t.Errorf("buildAgentDescription() missing %q in:\n%s", check, got)
				}
			}
		})
	}
}

func TestStripMarkdownFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fences",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			name:  "json fence",
			input: "```json\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "plain fence",
			input: "```\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "fence with whitespace",
			input: "  ```json\n{\"key\":\"value\"}\n```  ",
			want:  `{"key":"value"}`,
		},
		{
			name:  "multiline content in fence",
			input: "```json\n{\n  \"key\": \"value\",\n  \"other\": 42\n}\n```",
			want:  "{\n  \"key\": \"value\",\n  \"other\": 42\n}",
		},
		{
			name:  "already clean",
			input: `  {"clean": true}  `,
			want:  `{"clean": true}`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only backticks",
			input: "```\n```",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownFences(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownFences(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIntentResultJSON_Chat(t *testing.T) {
	raw := `{"type":"chat","content":"Here are the details you asked about..."}`
	var result IntentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Type != "chat" {
		t.Errorf("Type = %q, want %q", result.Type, "chat")
	}
	if result.Content != "Here are the details you asked about..." {
		t.Errorf("Content = %q, want %q", result.Content, "Here are the details you asked about...")
	}
	if result.Title != "" {
		t.Errorf("Title = %q, want empty", result.Title)
	}
	if result.Description != "" {
		t.Errorf("Description = %q, want empty", result.Description)
	}
}

func TestIntentResultJSON_Task(t *testing.T) {
	raw := `{"type":"task","title":"Review Q4 Deal","description":"Analyze the Q4 deal pipeline with finance and legal review"}`
	var result IntentResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Type != "task" {
		t.Errorf("Type = %q, want %q", result.Type, "task")
	}
	if result.Title != "Review Q4 Deal" {
		t.Errorf("Title = %q, want %q", result.Title, "Review Q4 Deal")
	}
	if result.Description != "Analyze the Q4 deal pipeline with finance and legal review" {
		t.Errorf("Description = %q, want %q", result.Description, "Analyze the Q4 deal pipeline with finance and legal review")
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty", result.Content)
	}
}

func TestIntentResultJSON_Roundtrip(t *testing.T) {
	original := IntentResult{
		Type:        "task",
		Title:       "Deploy Service",
		Description: "Deploy the microservice to production",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded IntentResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != original.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, original.Title)
	}
	if decoded.Description != original.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, original.Description)
	}
}

func TestChatMessageFormatting(t *testing.T) {
	messages := []ChatMessage{
		{Role: "user", Name: "Alice", Content: "Review the Q4 pipeline"},
		{Role: "system", Name: "System", Content: "Creating task: Q4 Review"},
		{Role: "agent", Name: "Finance Agent", Content: "Analysis complete"},
	}

	// Verify fields are accessible and correct
	if messages[0].Role != "user" {
		t.Errorf("messages[0].Role = %q, want %q", messages[0].Role, "user")
	}
	if messages[1].Name != "System" {
		t.Errorf("messages[1].Name = %q, want %q", messages[1].Name, "System")
	}
	if messages[2].Content != "Analysis complete" {
		t.Errorf("messages[2].Content = %q, want %q", messages[2].Content, "Analysis complete")
	}

	// Build formatted history string (same format as DetectIntent)
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString("[" + msg.Role + "] " + msg.Name + ": " + msg.Content + "\n")
	}

	formatted := sb.String()
	if !strings.Contains(formatted, "[user] Alice: Review the Q4 pipeline") {
		t.Errorf("formatted history missing user message: %s", formatted)
	}
	if !strings.Contains(formatted, "[system] System: Creating task: Q4 Review") {
		t.Errorf("formatted history missing system message: %s", formatted)
	}
	if !strings.Contains(formatted, "[agent] Finance Agent: Analysis complete") {
		t.Errorf("formatted history missing agent message: %s", formatted)
	}
}

func TestBuildAgentDescription_Format(t *testing.T) {
	agents := []models.Agent{
		{
			ID:           "uuid-123",
			Name:         "TestAgent",
			Description:  "A test agent",
			Capabilities: []string{"analyze"},
		},
	}

	got := buildAgentDescription(agents)

	// Should start with "- "
	if !strings.HasPrefix(got, "- ") {
		t.Errorf("should start with '- ', got: %q", got)
	}

	// Should contain pipe separators
	if strings.Count(got, "|") != 3 {
		t.Errorf("expected 3 pipe separators, got %d in: %q", strings.Count(got, "|"), got)
	}

	// Should end with newline
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("should end with newline, got: %q", got)
	}
}
