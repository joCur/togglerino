package auth

import (
	"sync"

	"github.com/togglerino/togglerino/internal/model"
)

// RoleCache provides fast in-memory permission lookups for project roles.
type RoleCache struct {
	mu    sync.RWMutex
	perms map[string]map[model.Permission]bool // role name -> permission set
}

func NewRoleCache() *RoleCache {
	return &RoleCache{
		perms: make(map[string]map[model.Permission]bool),
	}
}

// Load replaces all cached roles with the given definitions.
func (c *RoleCache) Load(roles []model.RoleDefinition) {
	m := make(map[string]map[model.Permission]bool, len(roles))
	for _, r := range roles {
		perms := make(map[model.Permission]bool, len(r.Permissions))
		for _, p := range r.Permissions {
			perms[model.Permission(p)] = true
		}
		m[r.Name] = perms
	}
	c.mu.Lock()
	c.perms = m
	c.mu.Unlock()
}

// HasPermission returns true if the named role grants the given permission.
func (c *RoleCache) HasPermission(roleName string, perm model.Permission) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	perms, ok := c.perms[roleName]
	if !ok {
		return false
	}
	return perms[perm]
}
