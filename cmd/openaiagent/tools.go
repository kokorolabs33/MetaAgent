package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ToolDefinition describes a tool that can be called by the LLM.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
	Execute     func(ctx context.Context, args json.RawMessage) (string, error)
}

// toolSets maps role IDs to their available tools.
var toolSets = map[string][]ToolDefinition{}

func init() {
	webSearch := makeWebSearchTool()
	codeAnalysis := makeStubTool("code_analysis",
		"Analyze code structure, dependencies, and patterns",
		`{"type":"object","properties":{"query":{"type":"string","description":"What to analyze about the code"}},"required":["query"],"additionalProperties":false}`,
	)
	competitorSearch := makeStubTool("competitor_search",
		"Search for competitor information and market positioning",
		`{"type":"object","properties":{"query":{"type":"string","description":"Competitor or market query"}},"required":["query"],"additionalProperties":false}`,
	)

	// Per D-01, D-11, D-12: hardcoded role-to-toolset mapping.
	// All roles get web_search. Role-specific extras return "not yet implemented" per D-13.
	toolSets["engineering"] = []ToolDefinition{webSearch, codeAnalysis}
	toolSets["finance"] = []ToolDefinition{webSearch}
	toolSets["legal"] = []ToolDefinition{webSearch}
	toolSets["marketing"] = []ToolDefinition{webSearch, competitorSearch}
}

// GetToolsForRole returns the tool definitions for a given role ID.
// Returns nil if no tools are configured for the role.
func GetToolsForRole(roleID string) []ToolDefinition {
	return toolSets[roleID]
}

// makeWebSearchTool creates the Tavily web search tool.
// Per D-03: Tavily API as sole search provider, hand-rolled Go HTTP client.
// Per D-04: TAVILY_API_KEY env var required; if not set, tool is unavailable.
func makeWebSearchTool() ToolDefinition {
	return ToolDefinition{
		Name:        "web_search",
		Description: "Search the web for current, real-time information. Use this when you need up-to-date data, facts, statistics, or information that may have changed after your training cutoff.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"The search query"}},"required":["query"],"additionalProperties":false}`),
		Execute:     executeWebSearch,
	}
}

// makeStubTool creates a tool that returns "not yet implemented" per D-13.
func makeStubTool(name, description, paramsJSON string) ToolDefinition {
	return ToolDefinition{
		Name:        name,
		Description: description,
		Parameters:  json.RawMessage(paramsJSON),
		Execute: func(_ context.Context, _ json.RawMessage) (string, error) {
			return fmt.Sprintf("Tool '%s' is not yet implemented. Please proceed without this tool and note that %s is planned for a future update.", name, name), nil
		},
	}
}

// tavilyRequest is the Tavily Search API request body.
type tavilyRequest struct {
	APIKey        string `json:"api_key"`
	Query         string `json:"query"`
	SearchDepth   string `json:"search_depth"`
	IncludeAnswer bool   `json:"include_answer"`
	MaxResults    int    `json:"max_results"`
}

// tavilyResponse is the Tavily Search API response.
type tavilyResponse struct {
	Answer  string `json:"answer"`
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

// executeWebSearch calls the Tavily Search API.
// Per D-05: Tavily returns pre-summarized LLM-optimized responses -- pass directly.
// Per T-07-01: validate query is non-empty.
// Per T-07-02: API key read from env only, never logged or returned.
func executeWebSearch(ctx context.Context, args json.RawMessage) (string, error) {
	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		return "Web search is unavailable: TAVILY_API_KEY is not configured.", nil
	}

	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse web_search args: %w", err)
	}
	if params.Query == "" {
		return "Error: search query is empty.", nil
	}

	reqBody, err := json.Marshal(tavilyRequest{
		APIKey:        apiKey,
		Query:         params.Query,
		SearchDepth:   "basic",
		IncludeAnswer: true,
		MaxResults:    5,
	})
	if err != nil {
		return "", fmt.Errorf("marshal tavily request: %w", err)
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create tavily request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("tavily search failed: %v", err)
		return "Web search failed: connection error. Proceeding without search results.", nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Web search failed: could not read response.", nil
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("tavily search returned %d: %s", resp.StatusCode, string(body))
		return fmt.Sprintf("Web search failed (HTTP %d). Proceeding without search results.", resp.StatusCode), nil
	}

	var tavilyResp tavilyResponse
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return "Web search failed: could not parse results.", nil
	}

	// Format results for LLM consumption.
	var result strings.Builder
	if tavilyResp.Answer != "" {
		result.WriteString("Search Summary:\n")
		result.WriteString(tavilyResp.Answer)
		result.WriteString("\n\nSources:\n")
	}
	for i, r := range tavilyResp.Results {
		fmt.Fprintf(&result, "%d. %s\n   URL: %s\n   %s\n\n", i+1, r.Title, r.URL, r.Content)
	}

	return result.String(), nil
}
