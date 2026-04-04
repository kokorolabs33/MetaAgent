package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Sender dispatches webhook notifications for subscribed events.
type Sender struct {
	DB         *pgxpool.Pool
	httpClient *http.Client
}

// NewSender creates a webhook sender backed by the given database pool.
func NewSender(db *pgxpool.Pool) *Sender {
	return &Sender{
		DB:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// WebhookPayload is the JSON body sent to webhook endpoints.
type WebhookPayload struct {
	Event     string `json:"event"`
	TaskID    string `json:"task_id,omitempty"`
	SubtaskID string `json:"subtask_id,omitempty"`
	Data      any    `json:"data"`
	Timestamp string `json:"timestamp"`
}

// Send delivers a webhook notification for the given event to all matching active webhooks.
func (s *Sender) Send(ctx context.Context, eventType, taskID, subtaskID string, data any) {
	rows, err := s.DB.Query(ctx,
		`SELECT url, secret FROM webhook_configs WHERE is_active = true AND $1 = ANY(events)`,
		eventType)
	if err != nil {
		log.Printf("webhook: query configs for %s: %v", eventType, err)
		return
	}
	defer rows.Close()

	payload := WebhookPayload{
		Event:     eventType,
		TaskID:    taskID,
		SubtaskID: subtaskID,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: marshal payload: %v", err)
		return
	}

	for rows.Next() {
		var url, secret string
		if err := rows.Scan(&url, &secret); err != nil {
			continue
		}
		go s.deliver(url, secret, body)
	}
}

func (s *Sender) deliver(url, secret string, body []byte) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: create request to %s: %v", url, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-TaskHub-Signature", "sha256="+sig)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("webhook: deliver to %s: %v", url, err)
		return
	}
	resp.Body.Close()
}

// TestResult is the response from a synchronous test delivery.
type TestResult struct {
	Success    bool   `json:"success"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// DeliverTest sends a webhook payload synchronously and returns the result.
func (s *Sender) DeliverTest(url, secret string, body []byte) TestResult {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")

	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-TaskHub-Signature", "sha256="+sig)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return TestResult{Success: false, Error: err.Error()}
	}
	resp.Body.Close()
	return TestResult{Success: resp.StatusCode >= 200 && resp.StatusCode < 300, StatusCode: resp.StatusCode}
}
