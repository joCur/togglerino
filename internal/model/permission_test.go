package model

import "testing"

func TestRoleHasOrgPermission(t *testing.T) {
	allOrgPerms := []Permission{
		PermOrgUsersManage,
		PermOrgOIDCManage,
		PermOrgProjectsCreate,
		PermOrgProjectsDelete,
	}

	// admin: all org permissions
	for _, p := range allOrgPerms {
		if !RoleAdmin.HasOrgPermission(p) {
			t.Errorf("RoleAdmin should have org permission %s", p)
		}
	}

	// member: no org permissions
	for _, p := range allOrgPerms {
		if RoleMember.HasOrgPermission(p) {
			t.Errorf("RoleMember should NOT have org permission %s", p)
		}
	}

	// unknown role has no org permissions
	unknown := Role("unknown")
	for _, p := range allOrgPerms {
		if unknown.HasOrgPermission(p) {
			t.Errorf("unknown Role should NOT have org permission %s", p)
		}
	}
}
