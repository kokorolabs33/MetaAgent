package a2a

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestJSONRPCRequest_JSON(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tasks/send",
		ID:      "1",
		Params:  json.RawMessage(`{"message":{"role":"user","parts":[{"text":"hello"}]}}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded JSONRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Method != "tasks/send" {
		t.Errorf("Method = %q, want tasks/send", decoded.Method)
	}
}

func TestA2ATask_JSON(t *testing.T) {
	task := A2ATask{
		ID:        "task-1",
		ContextID: "ctx-1",
		Status:    A2AStatus{State: "completed"},
		Artifacts: []Artifact{
			{ArtifactID: "a1", Parts: []MessagePart{TextPart("result")}},
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded A2ATask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != "task-1" {
		t.Errorf("ID = %q, want task-1", decoded.ID)
	}
	if decoded.ContextID != "ctx-1" {
		t.Errorf("ContextID = %q, want ctx-1", decoded.ContextID)
	}
	if decoded.Status.State != "completed" {
		t.Errorf("Status.State = %q, want completed", decoded.Status.State)
	}
	if len(decoded.Artifacts) != 1 {
		t.Fatalf("Artifacts length = %d, want 1", len(decoded.Artifacts))
	}
	if decoded.Artifacts[0].Parts[0].Text != "result" {
		t.Errorf("Artifact text = %q, want result", decoded.Artifacts[0].Parts[0].Text)
	}
}

func TestWriteRPCResult_Exported(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteRPCResult(rec, "42", map[string]string{"status": "ok"})

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}

	var resp JSONRPCResponse
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
}

func TestWriteRPCError_Exported(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteRPCError(rec, "99", -32600, "invalid request")

	var resp JSONRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
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
}

func TestSendMessageParams_JSON(t *testing.T) {
	params := SendMessageParams{
		Message: A2AMessage{
			Role:  "user",
			Parts: []MessagePart{TextPart("hello")},
		},
		ContextID: "ctx-1",
		TaskID:    "task-1",
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded SendMessageParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Message.Role != "user" {
		t.Errorf("Role = %q, want user", decoded.Message.Role)
	}
	if decoded.ContextID != "ctx-1" {
		t.Errorf("ContextID = %q, want ctx-1", decoded.ContextID)
	}
}

func TestTaskIDParams_JSON(t *testing.T) {
	params := TaskIDParams{ID: "task-1"}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded TaskIDParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ID != "task-1" {
		t.Errorf("ID = %q, want task-1", decoded.ID)
	}
}
