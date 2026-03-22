package adapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PaesslerAG/jsonpath"
)

// ExtractScalar extracts a single value from JSON data using a JSONPath expression.
// Returns nil if path not found (for optional fields).
// Returns error if the JSON is malformed.
func ExtractScalar(data []byte, path string) (any, error) {
	if path == "" {
		return nil, nil
	}
	var obj any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	result, err := jsonpath.Get(path, obj)
	if err != nil {
		// Path not found -> nil (not error) for optional fields
		return nil, nil //nolint:nilerr // missing path is not an error for optional extraction
	}
	return result, nil
}

// ExtractScalarRequired extracts a single value, returning error if not found.
func ExtractScalarRequired(data []byte, path string) (any, error) {
	result, err := ExtractScalar(data, path)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("required path %q not found", path)
	}
	return result, nil
}

// ExtractStringSlice extracts an array of strings from JSON data.
// Returns nil if path not found.
func ExtractStringSlice(data []byte, path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	result, err := ExtractScalar(data, path)
	if err != nil || result == nil {
		return nil, err
	}

	// jsonpath may return []any for array paths
	switch v := result.(type) {
	case []any:
		strs := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				strs = append(strs, s)
			} else {
				strs = append(strs, fmt.Sprintf("%v", item))
			}
		}
		return strs, nil
	case string:
		return []string{v}, nil
	default:
		return []string{fmt.Sprintf("%v", v)}, nil
	}
}

// Needed to satisfy jsonpath.Get context requirement in some versions.
var _ = context.Background
