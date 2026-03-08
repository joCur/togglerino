package auth

import (
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func TestRoleCache(t *testing.T) {
	cache := NewRoleCache()

	roles := []model.RoleDefinition{
		{Name: "admin", Permissions: []string{"flags:read", "flags:write", "project:settings"}, IsBuiltIn: true},
		{Name: "viewer", Permissions: []string{"flags:read"}, IsBuiltIn: true},
	}
	cache.Load(roles)

	if !cache.HasPermission("admin", model.PermFlagsRead) {
		t.Error("admin should have flags:read")
	}
	if !cache.HasPermission("admin", model.PermProjectSettings) {
		t.Error("admin should have project:settings")
	}
	if cache.HasPermission("viewer", model.PermFlagsWrite) {
		t.Error("viewer should not have flags:write")
	}
	if cache.HasPermission("unknown", model.PermFlagsRead) {
		t.Error("unknown role should not have any permissions")
	}
}

func TestRoleCache_Reload(t *testing.T) {
	cache := NewRoleCache()

	cache.Load([]model.RoleDefinition{
		{Name: "custom", Permissions: []string{"flags:read"}},
	})
	if !cache.HasPermission("custom", model.PermFlagsRead) {
		t.Error("custom should have flags:read after first load")
	}

	// Reload with different permissions
	cache.Load([]model.RoleDefinition{
		{Name: "custom", Permissions: []string{"flags:write"}},
	})
	if cache.HasPermission("custom", model.PermFlagsRead) {
		t.Error("custom should NOT have flags:read after reload")
	}
	if !cache.HasPermission("custom", model.PermFlagsWrite) {
		t.Error("custom should have flags:write after reload")
	}
}
