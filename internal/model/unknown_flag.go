package model

import "time"

type UnknownFlag struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	EnvironmentID   string    `json:"environment_id"`
	FlagKey         string    `json:"flag_key"`
	RequestCount    int64     `json:"request_count"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	EnvironmentKey  string    `json:"environment_key"`
	EnvironmentName string    `json:"environment_name"`
}
