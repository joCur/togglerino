package handler

import (
	"context"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/webhook"
)

func webhookActorFromContext(ctx context.Context) *webhook.Actor {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	return &webhook.Actor{ID: user.ID, Email: user.Email}
}
