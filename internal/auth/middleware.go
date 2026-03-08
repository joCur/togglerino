package auth

import (
	"context"
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"
const projectContextKey contextKey = "project"

func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userContextKey).(*model.User)
	return u
}

// ContextWithUser returns a new context with the given user set.
// Exported for use in tests and other packages.
func ContextWithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// ProjectFromContext returns the project stored by RequireProjectPermission middleware.
func ProjectFromContext(ctx context.Context) *model.Project {
	p, _ := ctx.Value(projectContextKey).(*model.Project)
	return p
}

// ContextWithProject returns a new context with the given project set.
func ContextWithProject(ctx context.Context, project *model.Project) context.Context {
	return context.WithValue(ctx, projectContextKey, project)
}

// RoleResolver resolves a user's effective project role for a given project key.
type RoleResolver func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error)

// RequireOrgPermission returns middleware that checks whether the authenticated
// user's organization role grants the given permission.
func RequireOrgPermission(perm model.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || !user.Role.HasOrgPermission(perm) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireProjectPermission returns middleware that checks whether the
// authenticated user has the given permission for the project identified by the
// "key" path value. Org admins bypass the check entirely. The resolved project
// is stored in the request context and can be retrieved via ProjectFromContext.
func RequireProjectPermission(perm model.Permission, resolve RoleResolver, roleCache *RoleCache, projects ...*store.ProjectStore) func(http.Handler) http.Handler {
	var projectStore *store.ProjectStore
	if len(projects) > 0 {
		projectStore = projects[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Look up the project and store it in context if a store was provided.
			if projectStore != nil {
				projectKey := r.PathValue("key")
				project, err := projectStore.FindByKey(r.Context(), projectKey)
				if err != nil {
					http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
					return
				}
				ctx := context.WithValue(r.Context(), projectContextKey, project)
				r = r.WithContext(ctx)
			}

			// Org admins have full access to all projects.
			if user.Role == model.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			projectKey := r.PathValue("key")
			role, err := resolve(r.Context(), projectKey, user.ID)
			if err != nil {
				// Hide project existence from unauthorized users.
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
				return
			}

			if !roleCache.HasPermission(string(role), perm) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SessionAuth middleware checks for a valid session cookie and loads the user.
func SessionAuth(sessions *store.SessionStore, users *store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessions.FindByID(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := users.FindByID(r.Context(), session.UserID)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole middleware checks that the authenticated user has the required role.
func RequireRole(role model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || user.Role != role {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}
