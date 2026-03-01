package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type SDKKeyStore struct {
	pool *pgxpool.Pool
}

func NewSDKKeyStore(pool *pgxpool.Pool) *SDKKeyStore {
	return &SDKKeyStore{pool: pool}
}

// Create generates a new SDK key for an environment.
// Key format: "sdk_" + 32 random hex characters (using crypto/rand).
func (s *SDKKeyStore) Create(ctx context.Context, environmentID, name string) (*model.SDKKey, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating random key: %w", err)
	}
	key := "sdk_" + hex.EncodeToString(b)

	var k model.SDKKey
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sdk_keys (key, environment_id, name) VALUES ($1, $2, $3)
		 RETURNING id, key, environment_id, name, revoked, created_at`,
		key, environmentID, name,
	).Scan(&k.ID, &k.Key, &k.EnvironmentID, &k.Name, &k.Revoked, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating SDK key: %w", err)
	}
	return &k, nil
}

// ListByEnvironment returns all SDK keys for an environment.
func (s *SDKKeyStore) ListByEnvironment(ctx context.Context, environmentID string) ([]model.SDKKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, environment_id, name, revoked, created_at FROM sdk_keys WHERE environment_id = $1 ORDER BY created_at DESC`,
		environmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing SDK keys: %w", err)
	}
	defer rows.Close()

	var keys []model.SDKKey
	for rows.Next() {
		var k model.SDKKey
		if err := rows.Scan(&k.ID, &k.Key, &k.EnvironmentID, &k.Name, &k.Revoked, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning SDK key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating SDK keys: %w", err)
	}
	return keys, nil
}

// FindByKey looks up an SDK key by its key string. Returns error if not found or revoked.
// Joins environments and projects to resolve the project and environment keys
// so handlers can verify the SDK key is authorized for the requested scope.
func (s *SDKKeyStore) FindByKey(ctx context.Context, key string) (*model.SDKKey, error) {
	var k model.SDKKey
	err := s.pool.QueryRow(ctx,
		`SELECT sk.id, sk.key, sk.environment_id, sk.name, sk.revoked, sk.created_at, p.id, p.key, e.key
		 FROM sdk_keys sk
		 JOIN environments e ON e.id = sk.environment_id
		 JOIN projects p ON p.id = e.project_id
		 WHERE sk.key = $1 AND sk.revoked = FALSE`,
		key,
	).Scan(&k.ID, &k.Key, &k.EnvironmentID, &k.Name, &k.Revoked, &k.CreatedAt, &k.ProjectID, &k.ProjectKey, &k.EnvironmentKey)
	if err != nil {
		return nil, fmt.Errorf("finding SDK key: %w", err)
	}
	return &k, nil
}

// Revoke marks an SDK key as revoked.
func (s *SDKKeyStore) Revoke(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE sdk_keys SET revoked = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking SDK key: %w", err)
	}
	return nil
}
