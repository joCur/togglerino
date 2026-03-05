package model

import "time"

// ProjectMember represents a user's membership in a project with a specific role.
type ProjectMember struct {
	ProjectID string      `json:"project_id"`
	UserID    string      `json:"user_id"`
	Role      ProjectRole `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ProjectMemberWithUser includes user details alongside the membership.
type ProjectMemberWithUser struct {
	ProjectID   string      `json:"project_id"`
	UserID      string      `json:"user_id"`
	Role        ProjectRole `json:"role"`
	Email       string      `json:"email"`
	DisplayName *string     `json:"display_name,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// UserProjectAssignment represents a project a user has access to with their role.
type UserProjectAssignment struct {
	ProjectID   string      `json:"project_id"`
	ProjectKey  string      `json:"project_key"`
	ProjectName string      `json:"project_name"`
	Role        ProjectRole `json:"role"`
}
