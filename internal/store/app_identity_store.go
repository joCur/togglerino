package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type AppIdentityStore struct {
	pool *pgxpool.Pool
}

func NewAppIdentityStore(pool *pgxpool.Pool) *AppIdentityStore {
	return &AppIdentityStore{pool: pool}
}

func (s *AppIdentityStore) Set(ctx context.Context, userID, projectID, appUserID string) (*model.AppIdentity, error) {
	var identity model.AppIdentity
	err := s.pool.QueryRow(ctx,
		`INSERT INTO user_app_identities (user_id, project_id, app_user_id)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, project_id)
		 DO UPDATE SET app_user_id = EXCLUDED.app_user_id, updated_at = NOW()
		 RETURNING user_id, project_id, app_user_id, created_at, updated_at`,
		userID, projectID, appUserID,
	).Scan(&identity.UserID, &identity.ProjectID, &identity.AppUserID, &identity.CreatedAt, &identity.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrDuplicateAppUserID
		}
		return nil, fmt.Errorf("setting app identity: %w", err)
	}
	return &identity, nil
}

func (s *AppIdentityStore) Get(ctx context.Context, userID, projectID string) (*model.AppIdentity, error) {
	var identity model.AppIdentity
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, project_id, app_user_id, created_at, updated_at
		 FROM user_app_identities
		 WHERE user_id = $1 AND project_id = $2`,
		userID, projectID,
	).Scan(&identity.UserID, &identity.ProjectID, &identity.AppUserID, &identity.CreatedAt, &identity.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting app identity: %w", err)
	}
	return &identity, nil
}

func (s *AppIdentityStore) GetByProjectKey(ctx context.Context, userID, projectKey string) (*model.AppIdentity, error) {
	var identity model.AppIdentity
	err := s.pool.QueryRow(ctx,
		`SELECT uai.user_id, uai.project_id, uai.app_user_id, uai.created_at, uai.updated_at
		 FROM user_app_identities uai
		 JOIN projects p ON p.id = uai.project_id
		 WHERE uai.user_id = $1 AND p.key = $2`,
		userID, projectKey,
	).Scan(&identity.UserID, &identity.ProjectID, &identity.AppUserID, &identity.CreatedAt, &identity.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting app identity by project key: %w", err)
	}
	return &identity, nil
}

func (s *AppIdentityStore) Delete(ctx context.Context, userID, projectID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM user_app_identities WHERE user_id = $1 AND project_id = $2`,
		userID, projectID,
	)
	if err != nil {
		return fmt.Errorf("deleting app identity: %w", err)
	}
	return nil
}
