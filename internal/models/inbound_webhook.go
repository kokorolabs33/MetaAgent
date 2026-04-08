package models

import "time"

// InboundWebhook represents a configured inbound webhook endpoint that external
// systems (GitHub, Slack, etc.) can POST to in order to create TaskHub tasks.
type InboundWebhook struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`        // github, slack, generic
	Secret         string    `json:"secret"`          // pragma: allowlist secret
	PreviousSecret string    `json:"previous_secret"` // pragma: allowlist secret
	IsActive       bool      `json:"is_active"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// WebhookDelivery records a processed webhook delivery for idempotency protection.
type WebhookDelivery struct {
	DeliveryID string    `json:"delivery_id"`
	WebhookID  string    `json:"webhook_id"`
	CreatedAt  time.Time `json:"created_at"`
}
