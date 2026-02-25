package model

import (
	"encoding/json"
	"time"
)

type AuditEntry struct {
	ID         string          `json:"id"`
	ProjectID  *string         `json:"project_id,omitempty"`
	UserID     *string         `json:"user_id,omitempty"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	OldValue   json.RawMessage `json:"old_value,omitempty"`
	NewValue   json.RawMessage `json:"new_value,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
