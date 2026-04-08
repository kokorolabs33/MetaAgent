package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON_ValidBody(t *testing.T) {
	body := `{"title":"Test Task","description":"A test","template_id":"tmpl-1"}`
	r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req createTaskRequest
	err := decodeJSON(w, r, &req)

	if err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	if req.Title != "Test Task" {
		t.Errorf("Title = %q, want %q", req.Title, "Test Task")
	}
	if req.Description != "A test" {
		t.Errorf("Description = %q, want %q", req.Description, "A test")
	}
	if req.TemplateID != "tmpl-1" {
		t.Errorf("TemplateID = %q, want %q", req.TemplateID, "tmpl-1")
	}
}

func TestDecodeJSON_InvalidBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	var req createTaskRequest
	err := decodeJSON(w, r, &req)

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeJSON_EmptyBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(""))
	w := httptest.NewRecorder()

	var req createTaskRequest
	err := decodeJSON(w, r, &req)

	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestCreateTaskRequest_WithTemplateVariables(t *testing.T) {
	body := `{
		"title": "Deploy",
		"description": "Deploy to production",
		"template_id": "tmpl-123",
		"template_variables": {"env": "prod", "version": "1.2.3"}
	}`
	r := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req createTaskRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}

	if req.TemplateID != "tmpl-123" {
		t.Errorf("TemplateID = %q, want %q", req.TemplateID, "tmpl-123")
	}
	if len(req.TemplateVariables) != 2 {
		t.Errorf("TemplateVariables length = %d, want 2", len(req.TemplateVariables))
	}
	if req.TemplateVariables["env"] != "prod" {
		t.Errorf("TemplateVariables[env] = %q, want %q", req.TemplateVariables["env"], "prod")
	}
}

func TestCreatePolicyRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		check   func(t *testing.T, req createPolicyRequest)
	}{
		{
			name: "valid with all fields",
			body: `{"name":"Security Policy","rules":{"when":{"always":true}},"priority":10}`,
			check: func(t *testing.T, req createPolicyRequest) {
				if req.Name != "Security Policy" {
					t.Errorf("Name = %q, want %q", req.Name, "Security Policy")
				}
				if req.Priority != 10 {
					t.Errorf("Priority = %d, want 10", req.Priority)
				}
				if req.Rules == nil {
					t.Error("Rules should not be nil")
				}
			},
		},
		{
			name: "valid with name only",
			body: `{"name":"Minimal Policy"}`,
			check: func(t *testing.T, req createPolicyRequest) {
				if req.Name != "Minimal Policy" {
					t.Errorf("Name = %q, want %q", req.Name, "Minimal Policy")
				}
			},
		},
		{
			name: "empty name parses but should fail validation",
			body: `{"name":"","rules":{}}`,
			check: func(t *testing.T, req createPolicyRequest) {
				if req.Name != "" {
					t.Errorf("Name = %q, want empty", req.Name)
				}
			},
		},
		{
			name:    "invalid JSON",
			body:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/policies", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var req createPolicyRequest
			err := decodeJSON(w, r, &req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeJSON: %v", err)
			}
			tt.check(t, req)
		})
	}
}

func TestCreateTemplateRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		check   func(t *testing.T, req createTemplateRequest)
	}{
		{
			name: "valid with all fields",
			body: `{
				"name": "CI Pipeline",
				"description": "Standard CI workflow",
				"steps": [{"id":"s1","instruction":"build"}],
				"variables": [{"name":"branch","type":"string"}]
			}`,
			check: func(t *testing.T, req createTemplateRequest) {
				if req.Name != "CI Pipeline" {
					t.Errorf("Name = %q, want %q", req.Name, "CI Pipeline")
				}
				if req.Description != "Standard CI workflow" {
					t.Errorf("Description = %q, want %q", req.Description, "Standard CI workflow")
				}
				if req.Steps == nil {
					t.Error("Steps should not be nil")
				}
				if req.Variables == nil {
					t.Error("Variables should not be nil")
				}
			},
		},
		{
			name: "valid with name only",
			body: `{"name":"Simple Template"}`,
			check: func(t *testing.T, req createTemplateRequest) {
				if req.Name != "Simple Template" {
					t.Errorf("Name = %q, want %q", req.Name, "Simple Template")
				}
				// Steps and Variables will be nil; handler sets defaults
			},
		},
		{
			name: "empty name parses but should fail validation",
			body: `{"name":""}`,
			check: func(t *testing.T, req createTemplateRequest) {
				if req.Name != "" {
					t.Errorf("Name = %q, want empty", req.Name)
				}
			},
		},
		{
			name:    "invalid JSON",
			body:    `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var req createTemplateRequest
			err := decodeJSON(w, r, &req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeJSON: %v", err)
			}
			tt.check(t, req)
		})
	}
}

func TestUpdateA2AConfigRequest_Parse(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, req updateA2AConfigRequest)
	}{
		{
			name: "all fields",
			body: `{"enabled":true,"name_override":"Custom","description_override":"Custom Desc"}`,
			check: func(t *testing.T, req updateA2AConfigRequest) {
				if req.Enabled == nil || !*req.Enabled {
					t.Error("Enabled should be true")
				}
				if req.NameOverride == nil || *req.NameOverride != "Custom" {
					t.Error("NameOverride should be 'Custom'")
				}
				if req.DescriptionOverride == nil || *req.DescriptionOverride != "Custom Desc" {
					t.Error("DescriptionOverride should be 'Custom Desc'")
				}
			},
		},
		{
			name: "only enabled",
			body: `{"enabled":false}`,
			check: func(t *testing.T, req updateA2AConfigRequest) {
				if req.Enabled == nil || *req.Enabled {
					t.Error("Enabled should be false")
				}
				if req.NameOverride != nil {
					t.Error("NameOverride should be nil")
				}
				if req.DescriptionOverride != nil {
					t.Error("DescriptionOverride should be nil")
				}
			},
		},
		{
			name: "empty object",
			body: `{}`,
			check: func(t *testing.T, req updateA2AConfigRequest) {
				if req.Enabled != nil {
					t.Error("Enabled should be nil for empty request")
				}
				if req.NameOverride != nil {
					t.Error("NameOverride should be nil for empty request")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/a2a-config", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var req updateA2AConfigRequest
			if err := decodeJSON(w, r, &req); err != nil {
				t.Fatalf("decodeJSON: %v", err)
			}
			tt.check(t, req)
		})
	}
}

func TestJsonOK(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	jsonOK(rec, data)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var decoded map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded["status"] != "ok" {
		t.Errorf("status = %q, want ok", decoded["status"])
	}
}

func TestJsonCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"id": "new-1"}

	jsonCreated(rec, data)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}
}

func TestJsonError(t *testing.T) {
	rec := httptest.NewRecorder()

	jsonError(rec, "something went wrong", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var decoded map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded["error"] != "something went wrong" {
		t.Errorf("error = %q, want %q", decoded["error"], "something went wrong")
	}
}

func TestUpdatePolicyRequest_Parse(t *testing.T) {
	body := `{"name":"Updated","priority":20,"is_active":false}`
	r := httptest.NewRequest(http.MethodPut, "/policies/1", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req updatePolicyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}

	if req.Name == nil || *req.Name != "Updated" {
		t.Error("Name should be 'Updated'")
	}
	if req.Priority == nil || *req.Priority != 20 {
		t.Error("Priority should be 20")
	}
	if req.IsActive == nil || *req.IsActive {
		t.Error("IsActive should be false")
	}
}

func TestUpdatePolicyRequest_PartialUpdate(t *testing.T) {
	body := `{"is_active":true}`
	r := httptest.NewRequest(http.MethodPut, "/policies/1", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req updatePolicyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}

	if req.Name != nil {
		t.Error("Name should be nil for partial update")
	}
	if req.Rules != nil {
		t.Error("Rules should be nil for partial update")
	}
	if req.Priority != nil {
		t.Error("Priority should be nil for partial update")
	}
	if req.IsActive == nil || !*req.IsActive {
		t.Error("IsActive should be true")
	}
}
