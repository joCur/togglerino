package model

import (
	"encoding/json"
	"time"
)

type Webhook struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Secret     string    `json:"secret"`
	EventTypes []string  `json:"event_types"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID           string          `json:"id"`
	WebhookID    string          `json:"webhook_id"`
	EventType    string          `json:"event_type"`
	Payload      json.RawMessage `json:"payload"`
	StatusCode   *int            `json:"status_code,omitempty"`
	ResponseBody *string         `json:"response_body,omitempty"`
	Error        *string         `json:"error,omitempty"`
	Attempt      int             `json:"attempt"`
	Success      bool            `json:"success"`
	DurationMs   *int            `json:"duration_ms,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}
