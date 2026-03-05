package model

// Permission represents a granular action that can be checked against a role.
type Permission string

// Organization-level permissions.
const (
	PermOrgUsersManage   Permission = "org:users:manage"
	PermOrgOIDCManage    Permission = "org:oidc:manage"
	PermOrgProjectsCreate Permission = "org:projects:create"
	PermOrgProjectsDelete Permission = "org:projects:delete"
)

// Project-level permissions.
const (
	PermFlagsRead        Permission = "flags:read"
	PermFlagsWrite       Permission = "flags:write"
	PermEnvironmentsRead Permission = "environments:read"
	PermEnvironmentsWrite Permission = "environments:write"
	PermSDKKeysManage    Permission = "sdk_keys:manage"
	PermSegmentsWrite    Permission = "segments:write"
	PermTemplatesManage  Permission = "templates:manage"
	PermProjectSettings  Permission = "project:settings"
)

// ProjectRole represents a user's role within a specific project.
type ProjectRole string

const (
	ProjectRoleAdmin  ProjectRole = "admin"
	ProjectRoleEditor ProjectRole = "editor"
	ProjectRoleViewer ProjectRole = "viewer"
)

// ValidProjectRole returns true if s is a valid project role.
func ValidProjectRole(s string) bool {
	switch ProjectRole(s) {
	case ProjectRoleAdmin, ProjectRoleEditor, ProjectRoleViewer:
		return true
	}
	return false
}

// projectRolePermissions maps each project role to its allowed permissions.
var projectRolePermissions = map[ProjectRole]map[Permission]bool{
	ProjectRoleAdmin: {
		PermFlagsRead:         true,
		PermFlagsWrite:        true,
		PermEnvironmentsRead:  true,
		PermEnvironmentsWrite: true,
		PermSDKKeysManage:     true,
		PermSegmentsWrite:     true,
		PermTemplatesManage:   true,
		PermProjectSettings:   true,
	},
	ProjectRoleEditor: {
		PermFlagsRead:         true,
		PermFlagsWrite:        true,
		PermEnvironmentsRead:  true,
		PermEnvironmentsWrite: true,
		PermSDKKeysManage:     true,
		PermSegmentsWrite:     true,
		PermTemplatesManage:   true,
	},
	ProjectRoleViewer: {
		PermFlagsRead:        true,
		PermEnvironmentsRead: true,
	},
}

// HasPermission returns true if the project role grants the given permission.
func (r ProjectRole) HasPermission(p Permission) bool {
	perms, ok := projectRolePermissions[r]
	if !ok {
		return false
	}
	return perms[p]
}

// orgRolePermissions maps each organization role to its allowed permissions.
var orgRolePermissions = map[Role]map[Permission]bool{
	RoleAdmin: {
		PermOrgUsersManage:    true,
		PermOrgOIDCManage:     true,
		PermOrgProjectsCreate: true,
		PermOrgProjectsDelete: true,
	},
	RoleMember: {},
}

// HasOrgPermission returns true if the organization role grants the given permission.
func (r Role) HasOrgPermission(p Permission) bool {
	perms, ok := orgRolePermissions[r]
	if !ok {
		return false
	}
	return perms[p]
}
