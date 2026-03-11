package model

import (
	"encoding/json"
	"time"
)

// ScheduleStatus represents the lifecycle state of a scheduled flag change.
type ScheduleStatus string

const (
	ScheduleStatusPending   ScheduleStatus = "pending"
	ScheduleStatusExecuted  ScheduleStatus = "executed"
	ScheduleStatusCancelled ScheduleStatus = "cancelled"
	ScheduleStatusFailed    ScheduleStatus = "failed"
)

// ScheduledFlagChange represents a future flag environment config change.
type ScheduledFlagChange struct {
	ID             string          `json:"id"`
	FlagID         string          `json:"flag_id"`
	EnvironmentID  string          `json:"environment_id"`
	ScheduledAt    time.Time       `json:"scheduled_at"`
	Status         ScheduleStatus  `json:"status"`
	ConfigSnapshot json.RawMessage `json:"config_snapshot"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	ExecutedAt     *time.Time      `json:"executed_at,omitempty"`
	CancelledAt    *time.Time      `json:"cancelled_at,omitempty"`
	CancelReason   *string         `json:"cancel_reason,omitempty"`
}

// ConfigSnapshotPayload is the config state to be applied at the scheduled time.
type ConfigSnapshotPayload struct {
	Enabled        bool            `json:"enabled"`
	DefaultVariant string          `json:"default_variant"`
	Variants       json.RawMessage `json:"variants"`
	TargetingRules json.RawMessage `json:"targeting_rules"`
}
