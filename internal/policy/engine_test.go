package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMatchesWhen_NilCondition(t *testing.T) {
	// nil When clause should always match
	if !matchesWhen(nil, "any task text") {
		t.Error("matchesWhen(nil, ...) = false, want true")
	}
}

func TestMatchesWhen_Always(t *testing.T) {
	when := &WhenClause{Always: true}
	if !matchesWhen(when, "any task text") {
		t.Error("matchesWhen({Always:true}, ...) = false, want true")
	}
}

func TestMatchesWhen_TaskContains(t *testing.T) {
	tests := []struct {
		name     string
		keywords []string
		taskText string
		want     bool
	}{
		{
			name:     "matching keyword",
			keywords: []string{"security"},
			taskText: "run a security audit on the codebase",
			want:     true,
		},
		{
			name:     "case insensitive match",
			keywords: []string{"SECURITY"},
			taskText: "run a security audit",
			want:     true,
		},
		{
			name:     "no match",
			keywords: []string{"deploy"},
			taskText: "run a security audit",
			want:     false,
		},
		{
			name:     "multiple keywords first matches",
			keywords: []string{"security", "compliance"},
			taskText: "security review needed",
			want:     true,
		},
		{
			name:     "multiple keywords second matches",
			keywords: []string{"deploy", "compliance"},
			taskText: "compliance check required",
			want:     true,
		},
		{
			name:     "multiple keywords none match",
			keywords: []string{"deploy", "migrate"},
			taskText: "security audit review",
			want:     false,
		},
		{
			name:     "empty keywords list",
			keywords: []string{},
			taskText: "any task",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			when := &WhenClause{TaskContains: tt.keywords}
			got := matchesWhen(when, tt.taskText)
			if got != tt.want {
				t.Errorf("matchesWhen(%+v, %q) = %v, want %v", when, tt.taskText, got, tt.want)
			}
		})
	}
}

func TestMatchesWhen_EmptyClause(t *testing.T) {
	// WhenClause with no fields set (Always=false, no TaskContains) should not match
	when := &WhenClause{}
	if matchesWhen(when, "some task") {
		t.Error("matchesWhen({}, ...) = true, want false")
	}
}

func TestFormatForPrompt_Empty(t *testing.T) {
	result := &EvalResult{}
	got := result.FormatForPrompt()
	if got != "" {
		t.Errorf("FormatForPrompt() = %q, want empty string", got)
	}
}

func TestFormatForPrompt_WithConstraints(t *testing.T) {
	result := &EvalResult{
		Constraints: []Constraint{
			{
				PolicyName:  "security-policy",
				Description: "Must include agent(s) with skills: code-review",
			},
		},
		MaxSubtasks:    5,
		MaxTimeMinutes: 30,
	}

	got := result.FormatForPrompt()

	if got == "" {
		t.Fatal("FormatForPrompt() returned empty string, want non-empty")
	}

	// Check that the output contains expected elements
	checks := []string{
		"[Policy Constraints]",
		"Must include agent(s) with skills: code-review",
		"security-policy",
		"Maximum 5 subtasks",
		"Maximum 30 minutes execution time",
	}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("FormatForPrompt() missing %q in output:\n%s", check, got)
		}
	}
}

func TestFormatForPrompt_OnlyMaxSubtasks(t *testing.T) {
	result := &EvalResult{
		MaxSubtasks: 3,
	}

	got := result.FormatForPrompt()
	if got == "" {
		t.Fatal("FormatForPrompt() returned empty, want non-empty")
	}
	if !strings.Contains(got, "Maximum 3 subtasks") {
		t.Errorf("FormatForPrompt() missing max subtasks in output:\n%s", got)
	}
}

func TestFormatForPrompt_OnlyMaxTime(t *testing.T) {
	result := &EvalResult{
		MaxTimeMinutes: 60,
	}

	got := result.FormatForPrompt()
	if got == "" {
		t.Fatal("FormatForPrompt() returned empty, want non-empty")
	}
	if !strings.Contains(got, "Maximum 60 minutes execution time") {
		t.Errorf("FormatForPrompt() missing max time in output:\n%s", got)
	}
}

func TestFormatForPrompt_OnlyConstraints(t *testing.T) {
	result := &EvalResult{
		Constraints: []Constraint{
			{PolicyName: "p1", Description: "desc1"},
			{PolicyName: "p2", Description: "desc2"},
		},
	}

	got := result.FormatForPrompt()
	if got == "" {
		t.Fatal("FormatForPrompt() returned empty, want non-empty")
	}
	if !strings.Contains(got, "desc1") || !strings.Contains(got, "desc2") {
		t.Errorf("FormatForPrompt() missing constraint descriptions:\n%s", got)
	}
	if !strings.Contains(got, "p1") || !strings.Contains(got, "p2") {
		t.Errorf("FormatForPrompt() missing policy names:\n%s", got)
	}
}

func TestFormatForPrompt_IgnoresApprovalThreshold(t *testing.T) {
	// RequireApprovalAboveSubtasks is stored but not included in prompt output
	result := &EvalResult{
		RequireApprovalAboveSubtasks: 3,
	}

	got := result.FormatForPrompt()
	// No constraints, no max subtasks, no max time -> empty
	if got != "" {
		t.Errorf("FormatForPrompt() with only approval threshold = %q, want empty", got)
	}
}

func TestEvalResult_ConstraintFields(t *testing.T) {
	result := &EvalResult{
		Constraints: []Constraint{
			{PolicyName: "test", Description: "test constraint"},
		},
		MaxSubtasks:                  5,
		MaxTimeMinutes:               30,
		RequireApprovalAboveSubtasks: 3,
		AppliedPolicies:              []string{"test-policy"},
	}

	if result.MaxSubtasks != 5 {
		t.Errorf("MaxSubtasks = %d, want 5", result.MaxSubtasks)
	}
	if result.MaxTimeMinutes != 30 {
		t.Errorf("MaxTimeMinutes = %d, want 30", result.MaxTimeMinutes)
	}
	if result.RequireApprovalAboveSubtasks != 3 {
		t.Errorf("RequireApprovalAboveSubtasks = %d, want 3", result.RequireApprovalAboveSubtasks)
	}
	if len(result.AppliedPolicies) != 1 || result.AppliedPolicies[0] != "test-policy" {
		t.Errorf("AppliedPolicies = %v, want [test-policy]", result.AppliedPolicies)
	}
}

func TestPolicyRule_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		checkFn func(t *testing.T, rule PolicyRule)
	}{
		{
			name:  "always rule with max subtasks",
			input: `{"when":{"always":true},"max_subtasks":5}`,
			checkFn: func(t *testing.T, rule PolicyRule) {
				if rule.When == nil || !rule.When.Always {
					t.Error("When.Always should be true")
				}
				if rule.MaxSubtasks != 5 {
					t.Errorf("MaxSubtasks = %d, want 5", rule.MaxSubtasks)
				}
			},
		},
		{
			name:  "task contains rule",
			input: `{"when":{"task_contains":["security","audit"]}}`,
			checkFn: func(t *testing.T, rule PolicyRule) {
				if rule.When == nil {
					t.Fatal("When is nil")
				}
				if len(rule.When.TaskContains) != 2 {
					t.Errorf("TaskContains length = %d, want 2", len(rule.When.TaskContains))
				}
			},
		},
		{
			name:  "require agent skills",
			input: `{"require":{"agent_skills":["code-review"]}}`,
			checkFn: func(t *testing.T, rule PolicyRule) {
				if rule.Require == nil {
					t.Fatal("Require is nil")
				}
				if len(rule.Require.AgentSkills) != 1 || rule.Require.AgentSkills[0] != "code-review" {
					t.Errorf("AgentSkills = %v, want [code-review]", rule.Require.AgentSkills)
				}
			},
		},
		{
			name:  "restrict agents to tags",
			input: `{"restrict_agents_to":{"tags":["backend"]}}`,
			checkFn: func(t *testing.T, rule PolicyRule) {
				if rule.RestrictAgentsTo == nil {
					t.Fatal("RestrictAgentsTo is nil")
				}
				if len(rule.RestrictAgentsTo.Tags) != 1 || rule.RestrictAgentsTo.Tags[0] != "backend" {
					t.Errorf("Tags = %v, want [backend]", rule.RestrictAgentsTo.Tags)
				}
			},
		},
		{
			name:  "approval threshold",
			input: `{"require_approval_above_subtasks":3}`,
			checkFn: func(t *testing.T, rule PolicyRule) {
				if rule.RequireApprovalAboveSubtasks != 3 {
					t.Errorf("RequireApprovalAboveSubtasks = %d, want 3",
						rule.RequireApprovalAboveSubtasks)
				}
			},
		},
		{
			name:  "max execution time",
			input: `{"max_execution_time_minutes":120}`,
			checkFn: func(t *testing.T, rule PolicyRule) {
				if rule.MaxTimeMinutes != 120 {
					t.Errorf("MaxTimeMinutes = %d, want 120", rule.MaxTimeMinutes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rule PolicyRule
			if err := json.Unmarshal([]byte(tt.input), &rule); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			tt.checkFn(t, rule)
		})
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine(nil)
	if e == nil {
		t.Fatal("NewEngine(nil) returned nil")
	}
	if e.DB != nil {
		t.Error("NewEngine(nil).DB is non-nil, want nil")
	}
}

func TestEvalResult_JSONMarshal(t *testing.T) {
	result := &EvalResult{
		Constraints: []Constraint{
			{PolicyName: "p1", Description: "d1"},
		},
		MaxSubtasks:                  5,
		MaxTimeMinutes:               30,
		RequireApprovalAboveSubtasks: 3,
		AppliedPolicies:              []string{"p1"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded EvalResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.MaxSubtasks != 5 {
		t.Errorf("MaxSubtasks = %d, want 5", decoded.MaxSubtasks)
	}
	if decoded.RequireApprovalAboveSubtasks != 3 {
		t.Errorf("RequireApprovalAboveSubtasks = %d, want 3", decoded.RequireApprovalAboveSubtasks)
	}
	if len(decoded.AppliedPolicies) != 1 {
		t.Errorf("AppliedPolicies length = %d, want 1", len(decoded.AppliedPolicies))
	}
}
