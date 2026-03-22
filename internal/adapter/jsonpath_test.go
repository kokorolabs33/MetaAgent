package adapter

import "testing"

func TestExtractScalar(t *testing.T) {
	data := []byte(`{"id": "job-123", "state": "running", "nested": {"value": 42}}`)

	val, err := ExtractScalar(data, "$.id")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != "job-123" {
		t.Errorf("got %v", val)
	}

	val, err = ExtractScalar(data, "$.nested.value")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != float64(42) {
		t.Errorf("got %v (type %T)", val, val)
	}
}

func TestExtractScalarMissing(t *testing.T) {
	data := []byte(`{"id": "job-123"}`)
	val, err := ExtractScalar(data, "$.nonexistent")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestExtractScalarRequired(t *testing.T) {
	data := []byte(`{"id": "job-123"}`)
	_, err := ExtractScalarRequired(data, "$.missing")
	if err == nil {
		t.Fatal("expected error for missing required path")
	}
}

func TestExtractStringSlice(t *testing.T) {
	data := []byte(`{"tags": ["a", "b", "c"]}`)
	strs, err := ExtractStringSlice(data, "$.tags")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(strs) != 3 || strs[0] != "a" {
		t.Errorf("got %v", strs)
	}
}
