package a2a

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthChecker_DefaultInterval(t *testing.T) {
	hc := &HealthChecker{}

	if hc.Interval != 0 {
		t.Errorf("initial Interval = %v, want 0", hc.Interval)
	}

	// Start sets the default interval if not configured.
	// We can't actually call Start without a DB, but we can test
	// the interval defaulting logic by checking the struct.
}

func TestHealthChecker_CustomInterval(t *testing.T) {
	hc := &HealthChecker{
		Interval: 30 * time.Second,
	}

	if hc.Interval != 30*time.Second {
		t.Errorf("Interval = %v, want 30s", hc.Interval)
	}
}

func TestHealthChecker_CheckOne_UnreachableEndpoint(t *testing.T) {
	resolver := NewResolver()
	agg := NewAggregator(nil)

	hc := &HealthChecker{
		Resolver:   resolver,
		Aggregator: agg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use an endpoint that definitely won't respond
	online, hash := hc.checkOne(ctx, "http://127.0.0.1:1")

	if online {
		t.Error("checkOne should return false for unreachable endpoint")
	}
	if hash != "" {
		t.Errorf("checkOne should return empty hash for unreachable endpoint, got %q", hash)
	}
}

func TestHealthChecker_CheckOne_InvalidAgentCard(t *testing.T) {
	// Create a test server that returns invalid JSON at the well-known path
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	resolver := &Resolver{
		HTTPClient: ts.Client(),
	}
	agg := NewAggregator(nil)

	hc := &HealthChecker{
		Resolver:   resolver,
		Aggregator: agg,
	}

	ctx := context.Background()
	online, hash := hc.checkOne(ctx, ts.URL)

	if online {
		t.Error("checkOne should return false for invalid agent card")
	}
	if hash != "" {
		t.Errorf("hash should be empty for invalid card, got %q", hash)
	}
}

func TestHealthChecker_CheckOne_ValidAgentCard(t *testing.T) {
	// Create a test server that returns a valid agent card
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name": "Test Agent",
			"description": "A test agent",
			"url": "http://test.example.com",
			"version": "1.0.0",
			"skills": [
				{"id": "s1", "name": "Skill 1", "description": "Test skill"}
			]
		}`))
	}))
	defer ts.Close()

	resolver := &Resolver{
		HTTPClient: ts.Client(),
	}
	agg := NewAggregator(nil)

	hc := &HealthChecker{
		Resolver:   resolver,
		Aggregator: agg,
	}

	ctx := context.Background()
	online, hash := hc.checkOne(ctx, ts.URL)

	if !online {
		t.Error("checkOne should return true for valid agent card")
	}
	if hash == "" {
		t.Error("hash should be non-empty for valid card")
	}
}

func TestHealthChecker_CheckOne_ConsistentHash(t *testing.T) {
	// Verify that the same agent card produces the same hash
	cardJSON := `{
		"name": "Stable Agent",
		"description": "Returns the same card every time",
		"url": "http://stable.example.com",
		"version": "1.0.0",
		"skills": [{"id": "s1", "name": "Skill", "description": "desc"}]
	}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cardJSON))
	}))
	defer ts.Close()

	resolver := &Resolver{
		HTTPClient: ts.Client(),
	}
	agg := NewAggregator(nil)

	hc := &HealthChecker{
		Resolver:   resolver,
		Aggregator: agg,
	}

	ctx := context.Background()
	_, hash1 := hc.checkOne(ctx, ts.URL)
	_, hash2 := hc.checkOne(ctx, ts.URL)

	if hash1 != hash2 {
		t.Errorf("same card should produce same hash: %q != %q", hash1, hash2)
	}
}

func TestNewResolver(t *testing.T) {
	r := NewResolver()
	if r == nil {
		t.Fatal("NewResolver returned nil")
	}
	if r.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}
