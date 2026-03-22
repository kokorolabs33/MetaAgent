// Package main implements a mock agent service for testing the TaskHub agent protocol.
// It supports various behavior keywords in the instruction field to simulate different
// agent scenarios (echo, slow, fail, progress, ask for input, etc.).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// job represents a task tracked by the mock agent.
type job struct {
	mu            sync.Mutex
	ID            string   `json:"id"`
	Status        string   `json:"status"`
	Instruction   string   `json:"instruction"`
	Result        string   `json:"result,omitempty"`
	Error         string   `json:"error,omitempty"`
	Progress      float64  `json:"progress"`
	Messages      []string `json:"messages,omitempty"`
	InputReceived string   `json:"input_received,omitempty"`
	PollCount     int      `json:"poll_count"`
	Attempts      int      `json:"attempts"`
}

var jobs sync.Map

func main() {
	port := flag.Int("port", 9090, "listen port")
	flag.Parse()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/tasks", submitTask)
	r.Get("/tasks/{id}/status", getStatus)
	r.Post("/tasks/{id}/input", sendInput)
	r.Get("/health", health)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Mock agent listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

// submitRequest is the expected body for POST /tasks.
type submitRequest struct {
	TaskID      string `json:"task_id"`
	Instruction string `json:"instruction"`
}

func submitTask(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	j := &job{
		ID:          uuid.New().String(),
		Status:      "running",
		Instruction: req.Instruction,
	}

	// Apply immediate-completion keywords.
	instruction := req.Instruction
	switch {
	case strings.HasPrefix(instruction, "echo:"):
		j.Status = "completed"
		j.Result = strings.TrimPrefix(instruction, "echo:")
		j.Progress = 1.0

	case strings.HasPrefix(instruction, "fail:"):
		j.Status = "failed"
		j.Error = strings.TrimPrefix(instruction, "fail:")

	case strings.HasPrefix(instruction, "ask:"):
		j.Status = "needs_input"
		msg := strings.TrimPrefix(instruction, "ask:")
		j.Messages = []string{msg}
	}

	jobs.Store(j.ID, j)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"job_id": j.ID})
}

// statusResponse is the shape returned by GET /tasks/{id}/status.
type statusResponse struct {
	Status   string   `json:"status"`
	Result   string   `json:"result,omitempty"`
	Error    string   `json:"error,omitempty"`
	Progress float64  `json:"progress"`
	Messages []string `json:"messages,omitempty"`
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	val, ok := jobs.Load(id)
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	j := val.(*job)
	j.mu.Lock()
	defer j.mu.Unlock()

	// Advance state based on instruction keyword.
	instruction := j.Instruction
	switch {
	case strings.HasPrefix(instruction, "slow:"):
		n := parseIntSuffix(instruction, "slow:")
		j.PollCount++
		if j.PollCount >= n {
			j.Status = "completed"
			j.Result = fmt.Sprintf("completed after %d polls", n)
			j.Progress = 1.0
		} else {
			j.Progress = float64(j.PollCount) / float64(n)
		}

	case strings.HasPrefix(instruction, "fail-then-succeed:"):
		n := parseIntSuffix(instruction, "fail-then-succeed:")
		j.Attempts++
		if j.Attempts <= n {
			j.Status = "failed"
			j.Error = fmt.Sprintf("failure %d of %d", j.Attempts, n)
		} else {
			j.Status = "completed"
			j.Result = "succeeded after retries"
			j.Error = ""
			j.Progress = 1.0
		}

	case strings.HasPrefix(instruction, "progress:"):
		j.PollCount++
		steps := []float64{0, 0.25, 0.5, 0.75, 1.0}
		idx := j.PollCount
		if idx >= len(steps) {
			idx = len(steps) - 1
		}
		j.Progress = steps[idx]
		if j.Progress >= 1.0 {
			j.Status = "completed"
			j.Result = "progress complete"
		}

	case strings.HasPrefix(instruction, "echo:"),
		strings.HasPrefix(instruction, "fail:"),
		strings.HasPrefix(instruction, "ask:"):
		// Already handled at submit time; no state change on poll.

	default:
		// Default: complete after 2 polls.
		j.PollCount++
		if j.PollCount >= 2 {
			j.Status = "completed"
			j.Result = "done"
			j.Progress = 1.0
		} else {
			j.Progress = float64(j.PollCount) / 2.0
		}
	}

	resp := statusResponse{
		Status:   j.Status,
		Result:   j.Result,
		Error:    j.Error,
		Progress: j.Progress,
		Messages: j.Messages,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// inputRequest is the expected body for POST /tasks/{id}/input.
type inputRequest struct {
	Input string `json:"input"`
}

func sendInput(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	val, ok := jobs.Load(id)
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	j := val.(*job)

	var req inputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	j.InputReceived = req.Input
	j.Status = "completed"
	j.Result = fmt.Sprintf("received input: %s", req.Input)
	j.Progress = 1.0

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// parseIntSuffix extracts the integer after a keyword prefix (e.g., "slow:3" → 3).
// Returns 2 as default if parsing fails.
func parseIntSuffix(s, prefix string) int {
	v, err := strconv.Atoi(strings.TrimPrefix(s, prefix))
	if err != nil || v <= 0 {
		return 2
	}
	return v
}
