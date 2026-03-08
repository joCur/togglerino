package auth

import (
	"context"

	"github.com/togglerino/togglerino/internal/model"
)

// ContextWithSDKKey injects an SDK key into the context.
// Exported for use in handler tests; not intended for production code.
func ContextWithSDKKey(ctx context.Context, key *model.SDKKey) context.Context {
	return context.WithValue(ctx, sdkKeyContextKey, key)
}
