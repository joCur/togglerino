package model

import "testing"

func TestValidProjectRole(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"admin", true},
		{"editor", true},
		{"viewer", true},
		{"member", false},
		{"", false},
		{"Admin", false},
		{"owner", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ValidProjectRole(tt.input); got != tt.want {
				t.Errorf("ValidProjectRole(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestProjectRoleHasPermission(t *testing.T) {
	allProjectPerms := []Permission{
		PermFlagsRead,
		PermFlagsWrite,
		PermEnvironmentsRead,
		PermEnvironmentsWrite,
		PermSDKKeysManage,
		PermSegmentsWrite,
		PermTemplatesManage,
		PermProjectSettings,
	}

	// admin: all project permissions
	for _, p := range allProjectPerms {
		if !ProjectRoleAdmin.HasPermission(p) {
			t.Errorf("ProjectRoleAdmin should have permission %s", p)
		}
	}

	// editor: all except project:settings
	editorYes := []Permission{
		PermFlagsRead,
		PermFlagsWrite,
		PermEnvironmentsRead,
		PermEnvironmentsWrite,
		PermSDKKeysManage,
		PermSegmentsWrite,
		PermTemplatesManage,
	}
	editorNo := []Permission{
		PermProjectSettings,
	}
	for _, p := range editorYes {
		if !ProjectRoleEditor.HasPermission(p) {
			t.Errorf("ProjectRoleEditor should have permission %s", p)
		}
	}
	for _, p := range editorNo {
		if ProjectRoleEditor.HasPermission(p) {
			t.Errorf("ProjectRoleEditor should NOT have permission %s", p)
		}
	}

	// viewer: only flags:read and environments:read
	viewerYes := []Permission{
		PermFlagsRead,
		PermEnvironmentsRead,
	}
	viewerNo := []Permission{
		PermFlagsWrite,
		PermEnvironmentsWrite,
		PermSDKKeysManage,
		PermSegmentsWrite,
		PermTemplatesManage,
		PermProjectSettings,
	}
	for _, p := range viewerYes {
		if !ProjectRoleViewer.HasPermission(p) {
			t.Errorf("ProjectRoleViewer should have permission %s", p)
		}
	}
	for _, p := range viewerNo {
		if ProjectRoleViewer.HasPermission(p) {
			t.Errorf("ProjectRoleViewer should NOT have permission %s", p)
		}
	}

	// unknown role has no permissions
	unknown := ProjectRole("unknown")
	for _, p := range allProjectPerms {
		if unknown.HasPermission(p) {
			t.Errorf("unknown ProjectRole should NOT have permission %s", p)
		}
	}
}

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
