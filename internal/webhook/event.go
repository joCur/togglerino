package webhook

import (
	"encoding/json"
	"time"
)

type Actor struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Event struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	ProjectID string          `json:"project_id"`
	Actor     *Actor          `json:"actor,omitempty"`
	Entity    json.RawMessage `json:"entity,omitempty"`
}

const (
	EventFlagCreated        = "flag.created"
	EventFlagUpdated        = "flag.updated"
	EventFlagDeleted        = "flag.deleted"
	EventFlagArchived       = "flag.archived"
	EventFlagConfigUpdated  = "flag_config.updated"
	EventSegmentCreated     = "segment.created"
	EventSegmentUpdated     = "segment.updated"
	EventSegmentDeleted     = "segment.deleted"
	EventEnvironmentCreated = "environment.created"
	EventWebhookTest        = "webhook.test"
)

var ValidEventTypes = map[string]bool{
	EventFlagCreated:        true,
	EventFlagUpdated:        true,
	EventFlagDeleted:        true,
	EventFlagArchived:       true,
	EventFlagConfigUpdated:  true,
	EventSegmentCreated:     true,
	EventSegmentUpdated:     true,
	EventSegmentDeleted:     true,
	EventEnvironmentCreated: true,
	EventWebhookTest:        true,
}
