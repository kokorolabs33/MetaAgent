package adapter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var templateVarRegex = regexp.MustCompile(`\{\{(\w+(?:\.\w+)*)\}\}`)

// RenderTemplate substitutes {{var}} and {{var.nested}} in a template value.
// vars is a flat map like {"instruction": "...", "job_id": "...", "input": {...}}.
// Missing variables are replaced with empty string.
func RenderTemplate(tmpl any, vars map[string]any) any {
	switch v := tmpl.(type) {
	case string:
		return renderString(v, vars)
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			result[key] = RenderTemplate(val, vars)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = RenderTemplate(val, vars)
		}
		return result
	default:
		return v
	}
}

func renderString(s string, vars map[string]any) string {
	return templateVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract path: "{{input.context}}" -> "input.context"
		path := match[2 : len(match)-2]
		val := resolveVarPath(path, vars)
		if val == nil {
			return ""
		}
		switch v := val.(type) {
		case string:
			return v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprintf("%v", v)
			}
			return string(b)
		}
	})
}

// resolveVarPath resolves "input.nested.field" against the vars map.
func resolveVarPath(path string, vars map[string]any) any {
	parts := strings.Split(path, ".")
	var current any = vars
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}
