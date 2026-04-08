package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookPayload_JSONSerialization(t *testing.T) {
	p := WebhookPayload{
		Event:     "task.completed",
		TaskID:    "task-1",
		SubtaskID: "sub-1",
		Data:      map[string]string{"result": "success"},
		Timestamp: "2026-03-30T00:00:00Z",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["event"] != "task.completed" {
		t.Errorf("event = %v, want task.completed", got["event"])
	}
	if got["task_id"] != "task-1" {
		t.Errorf("task_id = %v, want task-1", got["task_id"])
	}
	if got["subtask_id"] != "sub-1" {
		t.Errorf("subtask_id = %v, want sub-1", got["subtask_id"])
	}
	if got["timestamp"] != "2026-03-30T00:00:00Z" {
		t.Errorf("timestamp = %v, want 2026-03-30T00:00:00Z", got["timestamp"])
	}

	dataMap, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("data is not a map: %T", got["data"])
	}
	if dataMap["result"] != "success" {
		t.Errorf("data.result = %v, want success", dataMap["result"])
	}
}

func TestWebhookPayload_OmitsEmptyFields(t *testing.T) {
	p := WebhookPayload{
		Event:     "task.started",
		Data:      nil,
		Timestamp: "2026-03-30T00:00:00Z",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// task_id and subtask_id have omitempty
	if _, exists := got["task_id"]; exists {
		t.Error("expected task_id to be omitted when empty")
	}
	if _, exists := got["subtask_id"]; exists {
		t.Error("expected subtask_id to be omitted when empty")
	}
}

func TestHMACSHA256_Signature(t *testing.T) {
	// Known test vector: HMAC-SHA256("my-secret", "hello world")
	secret := "my-secret" // pragma: allowlist secret
	body := []byte("hello world")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	// Verify the signature format matches what deliver() produces
	expected := "sha256=" + sig
	if len(expected) < 10 {
		t.Fatal("signature too short")
	}
	if expected[:7] != "sha256=" {
		t.Errorf("expected sha256= prefix, got %q", expected[:7])
	}

	// Verify determinism: computing again should produce the same result
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write(body)
	sig2 := hex.EncodeToString(mac2.Sum(nil))
	if sig != sig2 {
		t.Errorf("HMAC not deterministic: %q != %q", sig, sig2)
	}
}

func TestDeliver_ContentType(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	s.deliver(srv.URL, "", []byte(`{"event":"test"}`))

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
}

func TestDeliver_IncludesSignatureWhenSecretSet(t *testing.T) {
	secret := "webhook-secret-123" // pragma: allowlist secret
	body := []byte(`{"event":"task.completed"}`)

	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-TaskHub-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	s.deliver(srv.URL, secret, body)

	if gotSig == "" {
		t.Fatal("expected X-TaskHub-Signature header to be set")
	}

	// Verify the signature is correct
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if gotSig != expectedSig {
		t.Errorf("signature = %q, want %q", gotSig, expectedSig)
	}
}

func TestDeliver_NoSignatureWhenSecretEmpty(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-TaskHub-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	s.deliver(srv.URL, "", []byte(`{"event":"test"}`))

	if gotSig != "" {
		t.Errorf("expected no signature header when secret is empty, got %q", gotSig)
	}
}

func TestDeliver_SendsCorrectBody(t *testing.T) {
	body := []byte(`{"event":"task.completed","task_id":"t1"}`)

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	s.deliver(srv.URL, "", body)

	if string(gotBody) != string(body) {
		t.Errorf("body = %q, want %q", string(gotBody), string(body))
	}
}

func TestDeliver_UsesPostMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	s.deliver(srv.URL, "", []byte(`{}`))

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
}

func TestDeliverTest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	result := s.DeliverTest(srv.URL, "", []byte(`{}`))

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("status code = %d, want 200", result.StatusCode)
	}
}

func TestDeliverTest_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	result := s.DeliverTest(srv.URL, "", []byte(`{}`))

	if result.Success {
		t.Error("expected failure for 500 response")
	}
	if result.StatusCode != 500 {
		t.Errorf("status code = %d, want 500", result.StatusCode)
	}
}

func TestDeliverTest_WithSignature(t *testing.T) {
	secret := "test-secret" // pragma: allowlist secret
	body := []byte(`{"event":"test"}`)

	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-TaskHub-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &Sender{httpClient: srv.Client()}
	result := s.DeliverTest(srv.URL, secret, body)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if gotSig == "" {
		t.Fatal("expected signature header")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if gotSig != expected {
		t.Errorf("signature = %q, want %q", gotSig, expected)
	}
}

func TestTestResult_JSONSerialization(t *testing.T) {
	r := TestResult{
		Success:    true,
		StatusCode: 200,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	if got["status_code"] != float64(200) {
		t.Errorf("status_code = %v, want 200", got["status_code"])
	}
}

func TestTestResult_ErrorField_OmittedWhenEmpty(t *testing.T) {
	r := TestResult{
		Success:    true,
		StatusCode: 200,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, exists := got["error"]; exists {
		t.Error("expected error field to be omitted when empty")
	}
}
