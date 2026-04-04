package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"taskhub/internal/a2a"
)

func TestPollUntilTerminal_AlreadyCompleted(t *testing.T) {
	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "completed", TaskID: "t1"}

	got := e.pollUntilTerminal(context.Background(), "http://unused", result)

	if got.State != "completed" {
		t.Errorf("state = %q, want completed", got.State)
	}
}

func TestPollUntilTerminal_AlreadyFailed(t *testing.T) {
	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "failed", TaskID: "t1", Error: "something broke"}

	got := e.pollUntilTerminal(context.Background(), "http://unused", result)

	if got.State != "failed" {
		t.Errorf("state = %q, want failed", got.State)
	}
	if got.Error != "something broke" {
		t.Errorf("error = %q, want 'something broke'", got.Error)
	}
}

func TestPollUntilTerminal_AlreadyInputRequired(t *testing.T) {
	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "input-required", TaskID: "t1"}

	got := e.pollUntilTerminal(context.Background(), "http://unused", result)

	if got.State != "input-required" {
		t.Errorf("state = %q, want input-required", got.State)
	}
}

func TestPollUntilTerminal_AlreadyCanceled(t *testing.T) {
	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "canceled", TaskID: "t1"}

	got := e.pollUntilTerminal(context.Background(), "http://unused", result)

	if got.State != "canceled" {
		t.Errorf("state = %q, want canceled", got.State)
	}
}

func TestPollUntilTerminal_WaitForCompletion(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		state := "working"
		if count >= 2 {
			state = "completed"
		}

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result": map[string]any{
				"id":     "task-1",
				"status": map[string]string{"state": state},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "working", TaskID: "task-1"}

	got := e.pollUntilTerminal(context.Background(), srv.URL, result)

	if got.State != "completed" {
		t.Errorf("state = %q, want completed", got.State)
	}
	if callCount.Load() < 2 {
		t.Errorf("expected at least 2 poll calls, got %d", callCount.Load())
	}
}

func TestPollUntilTerminal_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "working", TaskID: "t1"}

	got := e.pollUntilTerminal(ctx, "http://unused", result)

	// Should return the original result since context is canceled
	if got.State != "working" {
		t.Errorf("state = %q, want working (canceled context)", got.State)
	}
}

func TestPollUntilTerminal_SubmittedState(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result": map[string]any{
				"id":     "task-1",
				"status": map[string]string{"state": "completed"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "submitted", TaskID: "task-1"}

	got := e.pollUntilTerminal(context.Background(), srv.URL, result)

	if got.State != "completed" {
		t.Errorf("state = %q, want completed", got.State)
	}
}

func TestPollUntilTerminal_TransientErrors(t *testing.T) {
	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			// First call: return a server error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Second call: return completed
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result": map[string]any{
				"id":     "task-1",
				"status": map[string]string{"state": "completed"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := &DAGExecutor{A2AClient: a2a.NewClient()}
	result := &a2a.SendResult{State: "working", TaskID: "task-1"}

	got := e.pollUntilTerminal(context.Background(), srv.URL, result)

	if got.State != "completed" {
		t.Errorf("state = %q, want completed (after transient error)", got.State)
	}
}

func TestGetMaxConcurrent_Default(t *testing.T) {
	e := &DAGExecutor{}
	if got := e.getMaxConcurrent(); got != 10 {
		t.Errorf("getMaxConcurrent() = %d, want 10", got)
	}
}

func TestGetMaxConcurrent_Custom(t *testing.T) {
	e := &DAGExecutor{maxConcurrent: 5}
	if got := e.getMaxConcurrent(); got != 5 {
		t.Errorf("getMaxConcurrent() = %d, want 5", got)
	}
}

func TestGetMaxConcurrentAgent_Default(t *testing.T) {
	e := &DAGExecutor{}
	if got := e.getMaxConcurrentAgent(); got != 3 {
		t.Errorf("getMaxConcurrentAgent() = %d, want 3", got)
	}
}

func TestGetMaxConcurrentAgent_Custom(t *testing.T) {
	e := &DAGExecutor{maxConcurrentAgent: 7}
	if got := e.getMaxConcurrentAgent(); got != 7 {
		t.Errorf("getMaxConcurrentAgent() = %d, want 7", got)
	}
}
