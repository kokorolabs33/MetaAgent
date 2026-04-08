package webhook

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// ParsedPayload is the normalized result of parsing a provider-specific webhook payload.
type ParsedPayload struct {
	Title       string
	Description string
	Source      string            // "github", "slack", "generic"
	Metadata    map[string]string // repo, author, url, etc.
}

// SanitizeForLLM wraps external webhook content in delimiters to prevent
// prompt injection (D-04). Control characters are stripped except newlines,
// and the content is truncated to the given max length.
func SanitizeForLLM(content string, maxLen int) string {
	// Strip control characters except newlines and tabs
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, content)

	if len(cleaned) > maxLen {
		cleaned = cleaned[:maxLen] + "..."
	}

	return "[EXTERNAL WEBHOOK CONTENT START]\n" + cleaned + "\n[EXTERNAL WEBHOOK CONTENT END]"
}

func truncateTitle(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

// ParseGitHubPayload parses a GitHub webhook payload based on the event type.
// Returns nil, nil for event types that should be acknowledged but not create tasks (D-05).
func ParseGitHubPayload(eventType string, body []byte) (*ParsedPayload, error) {
	switch eventType {
	case "push":
		return parseGitHubPush(body)
	case "pull_request":
		return parseGitHubPR(body)
	default:
		// Acknowledge but don't create a task (D-05)
		return nil, nil
	}
}

func parseGitHubPush(body []byte) (*ParsedPayload, error) {
	var payload struct {
		Repository struct {
			FullName   string `json:"full_name"`
			CompareURL string `json:"compare"`
		} `json:"repository"`
		Pusher struct {
			Name string `json:"name"`
		} `json:"pusher"`
		HeadCommit struct {
			Message string `json:"message"`
			URL     string `json:"url"`
			Author  struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"head_commit"`
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse github push: %w", err)
	}

	commitMsg := firstLine(payload.HeadCommit.Message)
	title := truncateTitle(
		fmt.Sprintf("[%s] Push: %s", payload.Repository.FullName, commitMsg),
		200,
	)

	desc := fmt.Sprintf("Push to %s by %s\nRef: %s\nCommit: %s\n\n%s",
		payload.Repository.FullName,
		payload.Pusher.Name,
		payload.Ref,
		payload.HeadCommit.URL,
		payload.HeadCommit.Message,
	)

	return &ParsedPayload{
		Title:       title,
		Description: SanitizeForLLM(desc, 5000),
		Source:      "github",
		Metadata: map[string]string{
			"repo":   payload.Repository.FullName,
			"author": payload.Pusher.Name,
			"url":    payload.HeadCommit.URL,
			"ref":    payload.Ref,
		},
	}, nil
}

func parseGitHubPR(body []byte) (*ParsedPayload, error) {
	var payload struct {
		Action      string `json:"action"`
		Number      int    `json:"number"`
		PullRequest struct {
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse github pr: %w", err)
	}

	title := truncateTitle(
		fmt.Sprintf("[%s] PR #%d: %s", payload.Repository.FullName, payload.Number, payload.PullRequest.Title),
		200,
	)

	desc := fmt.Sprintf("Pull Request %s by %s\n%s\n\n%s",
		payload.Action,
		payload.PullRequest.User.Login,
		payload.PullRequest.HTMLURL,
		payload.PullRequest.Body,
	)

	return &ParsedPayload{
		Title:       title,
		Description: SanitizeForLLM(desc, 5000),
		Source:      "github",
		Metadata: map[string]string{
			"repo":   payload.Repository.FullName,
			"author": payload.PullRequest.User.Login,
			"url":    payload.PullRequest.HTMLURL,
			"action": payload.Action,
		},
	}, nil
}

// ParseSlackPayload parses a Slack webhook payload. It handles both slash commands
// (form-encoded) and event callbacks (JSON). Returns nil, nil for unsupported event types.
func ParseSlackPayload(body []byte) (*ParsedPayload, error) {
	// Try JSON first (event callback)
	var jsonPayload struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Event     struct {
			Type string `json:"type"`
			Text string `json:"text"`
			User string `json:"user"`
		} `json:"event"`
	}
	if err := json.Unmarshal(body, &jsonPayload); err == nil && jsonPayload.Type != "" {
		// URL verification handshake
		if jsonPayload.Type == "url_verification" {
			return &ParsedPayload{
				Title:  "__slack_challenge__",
				Source: "slack",
				Metadata: map[string]string{
					"challenge": jsonPayload.Challenge,
				},
			}, nil
		}

		// Event callback
		if jsonPayload.Type == "event_callback" {
			evtType := jsonPayload.Event.Type
			if evtType == "message" || evtType == "app_mention" {
				title := truncateTitle(
					fmt.Sprintf("Slack: message from %s", jsonPayload.Event.User),
					200,
				)
				return &ParsedPayload{
					Title:       title,
					Description: SanitizeForLLM(jsonPayload.Event.Text, 5000),
					Source:      "slack",
					Metadata: map[string]string{
						"user":       jsonPayload.Event.User,
						"event_type": evtType,
					},
				}, nil
			}
			// Unsupported event type
			return nil, nil
		}
	}

	// Try form-encoded (slash command)
	values, err := url.ParseQuery(string(body))
	if err == nil && values.Get("command") != "" {
		command := values.Get("command")
		text := values.Get("text")
		userName := values.Get("user_name")
		channelName := values.Get("channel_name")

		titleText := text
		if len(titleText) > 60 {
			titleText = titleText[:60]
		}
		title := truncateTitle(
			fmt.Sprintf("Slack: %s %s", command, titleText),
			200,
		)

		desc := fmt.Sprintf("Slash command from %s in #%s\nCommand: %s\n\n%s",
			userName, channelName, command, text,
		)

		return &ParsedPayload{
			Title:       title,
			Description: SanitizeForLLM(desc, 5000),
			Source:      "slack",
			Metadata: map[string]string{
				"command":  command,
				"user":     userName,
				"channel":  channelName,
				"raw_text": text,
			},
		}, nil
	}

	return nil, fmt.Errorf("unrecognized slack payload format")
}

// ParseGenericPayload parses a generic JSON webhook payload.
// Looks for optional "title" and "description" fields.
func ParseGenericPayload(body []byte) (*ParsedPayload, error) {
	var payload struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		// Not valid JSON -- use raw body as description
		return &ParsedPayload{
			Title:       "Webhook task",
			Description: SanitizeForLLM(string(body), 5000),
			Source:      "generic",
			Metadata:    map[string]string{},
		}, nil
	}

	title := payload.Title
	if title == "" {
		title = "Webhook task"
	}
	title = truncateTitle(title, 200)

	desc := payload.Description
	if desc == "" {
		desc = string(body)
	}

	return &ParsedPayload{
		Title:       title,
		Description: SanitizeForLLM(desc, 5000),
		Source:      "generic",
		Metadata:    map[string]string{},
	}, nil
}
