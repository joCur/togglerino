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
// These constants match the three built-in roles seeded by migration 016.
// Custom roles are stored in the roles table and resolved at runtime via RoleCache.
type ProjectRole string

const (
	ProjectRoleAdmin  ProjectRole = "admin"
	ProjectRoleEditor ProjectRole = "editor"
	ProjectRoleViewer ProjectRole = "viewer"
)

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
