package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"taskhub/internal/models"
)

func TestConversationJSON(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	conv := models.Conversation{
		ID:        "conv-1",
		Title:     "Test Conversation",
		CreatedBy: "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(conv)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.Conversation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != "conv-1" {
		t.Errorf("ID = %q, want %q", decoded.ID, "conv-1")
	}
	if decoded.Title != "Test Conversation" {
		t.Errorf("Title = %q, want %q", decoded.Title, "Test Conversation")
	}
	if decoded.CreatedBy != "user-1" {
		t.Errorf("CreatedBy = %q, want %q", decoded.CreatedBy, "user-1")
	}
	if !decoded.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, now)
	}
	if !decoded.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", decoded.UpdatedAt, now)
	}
}

func TestConversationListItemJSON(t *testing.T) {
	item := models.ConversationListItem{
		ID:           "conv-2",
		Title:        "Deal Review",
		AgentCount:   3,
		TaskCount:    2,
		LatestStatus: "completed",
		UpdatedAt:    "2025-06-15T10:30:00Z",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.ConversationListItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != "conv-2" {
		t.Errorf("ID = %q, want %q", decoded.ID, "conv-2")
	}
	if decoded.Title != "Deal Review" {
		t.Errorf("Title = %q, want %q", decoded.Title, "Deal Review")
	}
	if decoded.AgentCount != 3 {
		t.Errorf("AgentCount = %d, want 3", decoded.AgentCount)
	}
	if decoded.TaskCount != 2 {
		t.Errorf("TaskCount = %d, want 2", decoded.TaskCount)
	}
	if decoded.LatestStatus != "completed" {
		t.Errorf("LatestStatus = %q, want %q", decoded.LatestStatus, "completed")
	}
	if decoded.UpdatedAt != "2025-06-15T10:30:00Z" {
		t.Errorf("UpdatedAt = %q, want %q", decoded.UpdatedAt, "2025-06-15T10:30:00Z")
	}

	// Verify JSON field names use snake_case
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	expectedFields := []string{"id", "title", "agent_count", "task_count", "latest_status", "updated_at"}
	for _, f := range expectedFields {
		if _, ok := raw[f]; !ok {
			t.Errorf("missing JSON field %q", f)
		}
	}
}

func TestConversationListItemJSON_EmptyStatus(t *testing.T) {
	item := models.ConversationListItem{
		ID:           "conv-3",
		Title:        "New Conversation",
		AgentCount:   0,
		TaskCount:    0,
		LatestStatus: "",
		UpdatedAt:    "2025-06-15T10:30:00Z",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.ConversationListItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.LatestStatus != "" {
		t.Errorf("LatestStatus = %q, want empty", decoded.LatestStatus)
	}
	if decoded.AgentCount != 0 {
		t.Errorf("AgentCount = %d, want 0", decoded.AgentCount)
	}
}

func TestCreateConversationRequest_Decode(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		check   func(t *testing.T, req createConversationRequest)
	}{
		{
			name: "with title",
			body: `{"title":"My Conversation"}`,
			check: func(t *testing.T, req createConversationRequest) {
				if req.Title != "My Conversation" {
					t.Errorf("Title = %q, want %q", req.Title, "My Conversation")
				}
			},
		},
		{
			name: "empty title",
			body: `{"title":""}`,
			check: func(t *testing.T, req createConversationRequest) {
				if req.Title != "" {
					t.Errorf("Title = %q, want empty", req.Title)
				}
			},
		},
		{
			name: "empty object",
			body: `{}`,
			check: func(t *testing.T, req createConversationRequest) {
				if req.Title != "" {
					t.Errorf("Title = %q, want empty", req.Title)
				}
			},
		},
		{
			name:    "invalid JSON",
			body:    "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/conversations", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var req createConversationRequest
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

func TestUpdateConversationRequest_Decode(t *testing.T) {
	body := `{"title":"Updated Title"}`
	r := httptest.NewRequest(http.MethodPut, "/conversations/conv-1", strings.NewReader(body))
	w := httptest.NewRecorder()

	var req updateConversationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}

	if req.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", req.Title, "Updated Title")
	}
}

func TestSendConversationMessageRequest_Decode(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
		check   func(t *testing.T, req sendConversationMessageRequest)
	}{
		{
			name: "valid message",
			body: `{"content":"Hello agents!"}`,
			check: func(t *testing.T, req sendConversationMessageRequest) {
				if req.Content != "Hello agents!" {
					t.Errorf("Content = %q, want %q", req.Content, "Hello agents!")
				}
			},
		},
		{
			name: "empty content",
			body: `{"content":""}`,
			check: func(t *testing.T, req sendConversationMessageRequest) {
				if req.Content != "" {
					t.Errorf("Content = %q, want empty", req.Content)
				}
			},
		},
		{
			name:    "invalid JSON",
			body:    "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/conversations/conv-1/messages", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			var req sendConversationMessageRequest
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

func TestMessageWithConversationIDJSON(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	msg := models.Message{
		ID:             "msg-1",
		TaskID:         "",
		ConversationID: "conv-1",
		SenderType:     "user",
		SenderID:       "user-1",
		SenderName:     "Alice",
		Content:        "Review this deal",
		Mentions:       []string{},
		CreatedAt:      now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ConversationID != "conv-1" {
		t.Errorf("ConversationID = %q, want %q", decoded.ConversationID, "conv-1")
	}
	if decoded.TaskID != "" {
		t.Errorf("TaskID = %q, want empty", decoded.TaskID)
	}

	// Verify JSON has conversation_id field
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["conversation_id"]; !ok {
		t.Error("missing JSON field conversation_id")
	}
}

func TestEventWithConversationIDJSON(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	evt := models.Event{
		ID:             "evt-1",
		TaskID:         "task-1",
		ConversationID: "conv-1",
		Type:           "message",
		ActorType:      "user",
		ActorID:        "user-1",
		Data:           json.RawMessage(`{"content":"hello"}`),
		CreatedAt:      now,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ConversationID != "conv-1" {
		t.Errorf("ConversationID = %q, want %q", decoded.ConversationID, "conv-1")
	}
}

func TestTaskWithConversationIDJSON(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	task := models.Task{
		ID:             "task-1",
		ConversationID: "conv-1",
		Title:          "Review Q4 deal",
		Description:    "Full review of the Q4 deal pipeline",
		Status:         "pending",
		CreatedBy:      "user-1",
		CreatedAt:      now,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded models.Task
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ConversationID != "conv-1" {
		t.Errorf("ConversationID = %q, want %q", decoded.ConversationID, "conv-1")
	}

	// Verify JSON field name
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["conversation_id"]; !ok {
		t.Error("missing JSON field conversation_id")
	}
}
