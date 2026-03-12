package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type PATStore struct {
	pool *pgxpool.Pool
}

func NewPATStore(pool *pgxpool.Pool) *PATStore {
	return &PATStore{pool: pool}
}

func (s *PATStore) Create(ctx context.Context, userID, name string, expiresAt *time.Time) (*model.PersonalAccessTokenWithValue, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating random token: %w", err)
	}
	token := "pat_" + hex.EncodeToString(b)
	prefix := token[:12]

	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	var pat model.PersonalAccessToken
	err := s.pool.QueryRow(ctx,
		`INSERT INTO personal_access_tokens (user_id, name, token_hash, token_prefix, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, name, token_prefix, expires_at, last_used_at, created_at`,
		userID, name, hash, prefix, expiresAt,
	).Scan(&pat.ID, &pat.UserID, &pat.Name, &pat.TokenPrefix, &pat.ExpiresAt, &pat.LastUsedAt, &pat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating personal access token: %w", err)
	}

	return &model.PersonalAccessTokenWithValue{
		PersonalAccessToken: pat,
		Token:               token,
	}, nil
}

func (s *PATStore) FindByHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error) {
	var pat model.PersonalAccessToken
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, token_prefix, expires_at, last_used_at, created_at
		 FROM personal_access_tokens
		 WHERE token_hash = $1`,
		hash,
	).Scan(&pat.ID, &pat.UserID, &pat.Name, &pat.TokenPrefix, &pat.ExpiresAt, &pat.LastUsedAt, &pat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding personal access token: %w", err)
	}
	return &pat, nil
}

func (s *PATStore) ListByUser(ctx context.Context, userID string) ([]model.PersonalAccessToken, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, token_prefix, expires_at, last_used_at, created_at
		 FROM personal_access_tokens
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing personal access tokens: %w", err)
	}
	defer rows.Close()

	var tokens []model.PersonalAccessToken
	for rows.Next() {
		var pat model.PersonalAccessToken
		if err := rows.Scan(&pat.ID, &pat.UserID, &pat.Name, &pat.TokenPrefix, &pat.ExpiresAt, &pat.LastUsedAt, &pat.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning personal access token: %w", err)
		}
		tokens = append(tokens, pat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating personal access tokens: %w", err)
	}
	return tokens, nil
}

func (s *PATStore) Delete(ctx context.Context, id, userID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM personal_access_tokens WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting personal access token: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("token not found or not owned by user")
	}
	return nil
}

func (s *PATStore) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE personal_access_tokens SET last_used_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("updating last_used_at: %w", err)
	}
	return nil
}
