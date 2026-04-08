package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMapStatusToA2AState(t *testing.T) {
	tests := []struct {
		internal string
		want     string
	}{
		{"pending", "submitted"},
		{"planning", "submitted"},
		{"running", "working"},
		{"completed", "completed"},
		{"failed", "failed"},
		{"canceled", "canceled"},
		{"unknown_status", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.internal, func(t *testing.T) {
			got := mapStatusToA2AState(tt.internal)
			if got != tt.want {
				t.Errorf("mapStatusToA2AState(%q) = %q, want %q", tt.internal, got, tt.want)
			}
		})
	}
}

func TestHandleJSONRPC_InvalidJSON(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	s.HandleJSONRPC(rec, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error code = %d, want -32700", resp.Error.Code)
	}
}

func TestHandleJSONRPC_UnknownMethod(t *testing.T) {
	s := &Server{}

	body := `{"jsonrpc":"2.0","method":"unknown/method","id":"1","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.HandleJSONRPC(rec, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code = %d, want -32601", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "unknown/method") {
		t.Errorf("error message = %q, should mention the method", resp.Error.Message)
	}
}

func TestHandleJSONRPC_NonPost(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodGet, "/a2a", nil)
	rec := httptest.NewRecorder()

	s.HandleJSONRPC(rec, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for non-POST request")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("error code = %d, want -32600", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "POST") {
		t.Errorf("error message = %q, should mention POST", resp.Error.Message)
	}
}

func TestHandleJSONRPC_MissingVersion(t *testing.T) {
	s := &Server{}

	body := `{"jsonrpc":"1.0","method":"tasks/get","id":"1","params":{"id":"abc"}}`
	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	rec := httptest.NewRecorder()

	s.HandleJSONRPC(rec, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for wrong jsonrpc version")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("error code = %d, want -32600", resp.Error.Code)
	}
}

func TestWriteRPCResult(t *testing.T) {
	rec := httptest.NewRecorder()

	result := map[string]string{"status": "ok"}
	writeRPCResult(rec, "42", result)

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID != "42" {
		t.Errorf("id = %q, want 42", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("result is nil")
	}

	// Verify result contents
	var parsed map[string]string
	if err := json.Unmarshal(resp.Result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if parsed["status"] != "ok" {
		t.Errorf("result.status = %q, want ok", parsed["status"])
	}
}

func TestWriteRPCError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeRPCError(rec, "99", -32600, "invalid request")

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", resp.JSONRPC)
	}
	if resp.ID != "99" {
		t.Errorf("id = %q, want 99", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("error.code = %d, want -32600", resp.Error.Code)
	}
	if resp.Error.Message != "invalid request" {
		t.Errorf("error.message = %q, want %q", resp.Error.Message, "invalid request")
	}
	if resp.Result != nil {
		t.Errorf("result should be nil for error response, got %v", resp.Result)
	}
}

func TestWriteRPCError_EmptyID(t *testing.T) {
	rec := httptest.NewRecorder()

	writeRPCError(rec, "", -32700, "parse error")

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID != "" {
		t.Errorf("id = %q, want empty for parse errors", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error.code = %d, want -32700", resp.Error.Code)
	}
}

func TestA2AServerTask_JSON(t *testing.T) {
	task := a2aServerTask{
		ID: "task-123",
		Status: a2aStatus{
			State: "working",
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded a2aServerTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != "task-123" {
		t.Errorf("ID = %q, want task-123", decoded.ID)
	}
	if decoded.Status.State != "working" {
		t.Errorf("Status.State = %q, want working", decoded.Status.State)
	}
}

func TestA2AServerTask_WithArtifacts(t *testing.T) {
	task := a2aServerTask{
		ID: "task-456",
		Status: a2aStatus{
			State: "completed",
		},
		Artifacts: []artifact{
			{
				ArtifactID: "result",
				Parts:      []MessagePart{TextPart("some result")},
			},
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded a2aServerTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(decoded.Artifacts) != 1 {
		t.Fatalf("Artifacts length = %d, want 1", len(decoded.Artifacts))
	}
	if decoded.Artifacts[0].ArtifactID != "result" {
		t.Errorf("ArtifactID = %q, want result", decoded.Artifacts[0].ArtifactID)
	}
}

func TestHandleJSONRPC_EmptyBody(t *testing.T) {
	s := &Server{}

	req := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(""))
	rec := httptest.NewRecorder()

	s.HandleJSONRPC(rec, req)

	var resp jsonRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for empty body")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error code = %d, want -32700 (parse error)", resp.Error.Code)
	}
}
