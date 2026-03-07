package model

import (
	"encoding/json"
	"time"
)

// AppIdentity maps a dashboard user to their application user ID within a project.
type AppIdentity struct {
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	AppUserID string    `json:"app_user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FlagOverride is a personal flag value override set by a dashboard user.
type FlagOverride struct {
	ID             string          `json:"id"`
	UserID         string          `json:"user_id"`
	FlagID         string          `json:"flag_id"`
	FlagKey        string          `json:"flag_key,omitempty"`
	EnvironmentID  string          `json:"environment_id"`
	EnvironmentKey string          `json:"environment_key,omitempty"`
	ProjectKey     string          `json:"project_key,omitempty"`
	Value          json.RawMessage `json:"value"`
	ExpiresAt      *time.Time      `json:"expires_at"`
	CreatedAt      time.Time       `json:"created_at"`
}
