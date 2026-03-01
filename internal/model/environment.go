package model

import "time"

type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type SDKKey struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	EnvironmentID  string    `json:"environment_id"`
	Name           string    `json:"name"`
	Revoked        bool      `json:"revoked"`
	CreatedAt      time.Time `json:"created_at"`
	ProjectID      string    `json:"project_id"`
	ProjectKey     string    `json:"project_key"`
	EnvironmentKey string    `json:"environment_key"`
}
