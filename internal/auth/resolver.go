package auth

import (
	"context"
	"fmt"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// BuildRoleResolver returns a RoleResolver that checks for an explicit project
// membership first, then falls back to the organization's base project role.
func BuildRoleResolver(members *store.ProjectMemberStore, projects *store.ProjectStore, orgSettings *store.OrgSettingsStore) RoleResolver {
	return func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		// 1. Look up project by key.
		project, err := projects.FindByKey(ctx, projectKey)
		if err != nil {
			return "", fmt.Errorf("project not found: %w", err)
		}

		// 2. Check for explicit project membership.
		role, err := members.GetRole(ctx, project.ID, userID)
		if err == nil {
			return role, nil
		}

		// 3. Fall back to org base project role.
		baseRole, err := orgSettings.GetBaseProjectRole(ctx)
		if err != nil {
			return "", fmt.Errorf("get base project role: %w", err)
		}

		if baseRole == "none" {
			return "", fmt.Errorf("no access to project %q", projectKey)
		}

		return model.ProjectRole(baseRole), nil
	}
}
