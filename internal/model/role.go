package model

import "time"

// RoleDefinition represents a project-level role with its permissions.
type RoleDefinition struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	IsBuiltIn   bool      `json:"is_built_in"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AllProjectPermissions is the canonical list of valid project-level permissions.
var AllProjectPermissions = []Permission{
	PermFlagsRead,
	PermFlagsWrite,
	PermEnvironmentsRead,
	PermEnvironmentsWrite,
	PermSDKKeysManage,
	PermSegmentsWrite,
	PermTemplatesManage,
	PermProjectSettings,
}

// ValidPermission returns true if p is a known project-level permission.
func ValidPermission(p string) bool {
	for _, perm := range AllProjectPermissions {
		if string(perm) == p {
			return true
		}
	}
	return false
}
