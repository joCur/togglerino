package model

import "time"

type Segment struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"project_id"`
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Conditions  []Condition `json:"conditions"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
