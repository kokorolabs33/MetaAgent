package adapter

import "testing"

func TestRenderTemplateSimple(t *testing.T) {
	vars := map[string]any{"instruction": "analyze this", "job_id": "j-123"}
	result := RenderTemplate("Task: {{instruction}}", vars)
	if result != "Task: analyze this" {
		t.Errorf("got %q", result)
	}
}

func TestRenderTemplateNested(t *testing.T) {
	vars := map[string]any{
		"input": map[string]any{
			"context": "some context",
			"nested":  map[string]any{"field": "deep value"},
		},
	}
	result := RenderTemplate("{{input.context}}", vars)
	if result != "some context" {
		t.Errorf("got %q", result)
	}
	result = RenderTemplate("{{input.nested.field}}", vars)
	if result != "deep value" {
		t.Errorf("got %q", result)
	}
}

func TestRenderTemplateMissing(t *testing.T) {
	vars := map[string]any{}
	result := RenderTemplate("{{missing}}", vars)
	if result != "" {
		t.Errorf("expected empty string for missing var, got %q", result)
	}
}

func TestRenderTemplateMap(t *testing.T) {
	tmpl := map[string]any{
		"prompt":  "{{instruction}}",
		"context": "{{input.ctx}}",
	}
	vars := map[string]any{
		"instruction": "do stuff",
		"input":       map[string]any{"ctx": "bg info"},
	}
	result := RenderTemplate(tmpl, vars).(map[string]any)
	if result["prompt"] != "do stuff" {
		t.Errorf("prompt: got %q", result["prompt"])
	}
	if result["context"] != "bg info" {
		t.Errorf("context: got %q", result["context"])
	}
}
