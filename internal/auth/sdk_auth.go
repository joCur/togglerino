package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type sdkContextKey string

const sdkKeyContextKey sdkContextKey = "sdk_key"

// SDKKeyFromContext returns the SDK key from the request context.
func SDKKeyFromContext(ctx context.Context) *model.SDKKey {
	k, _ := ctx.Value(sdkKeyContextKey).(*model.SDKKey)
	return k
}

// SDKAuth middleware reads the Authorization: Bearer <sdk_key> header,
// looks up the SDK key, and injects it into the context.
func SDKAuth(sdkKeys *store.SDKKeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			key := strings.TrimPrefix(authHeader, "Bearer ")
			sdkKey, err := sdkKeys.FindByKey(r.Context(), key)
			if err != nil {
				http.Error(w, `{"error":"invalid SDK key"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), sdkKeyContextKey, sdkKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
