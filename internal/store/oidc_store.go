package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type OIDCStore struct {
	pool *pgxpool.Pool
}

func NewOIDCStore(pool *pgxpool.Pool) *OIDCStore {
	return &OIDCStore{pool: pool}
}

// GetProvider returns the first (only) OIDC provider, or nil if none configured.
func (s *OIDCStore) GetProvider(ctx context.Context) (*model.OIDCProvider, error) {
	var p model.OIDCProvider
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, issuer_url, client_id, client_secret, scopes, default_role, enabled, created_at, updated_at
		 FROM oidc_providers ORDER BY created_at LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.IssuerURL, &p.ClientID, &p.ClientSecret, &p.Scopes, &p.DefaultRole, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("getting oidc provider: %w", err)
	}
	return &p, nil
}

// UpsertProvider creates or updates the OIDC provider config.
// For single-provider mode, this deletes any existing provider and inserts the new one.
func (s *OIDCStore) UpsertProvider(ctx context.Context, p *model.OIDCProvider) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM oidc_providers`)
	if err != nil {
		return fmt.Errorf("clearing oidc providers: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`INSERT INTO oidc_providers (name, issuer_url, client_id, client_secret, scopes, default_role, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.IssuerURL, p.ClientID, p.ClientSecret, p.Scopes, p.DefaultRole, p.Enabled,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting oidc provider: %w", err)
	}
	return nil
}

// DeleteProvider removes an OIDC provider by ID.
func (s *OIDCStore) DeleteProvider(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM oidc_providers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting oidc provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("oidc provider not found")
	}
	return nil
}

// FindIdentity looks up an OIDC identity by provider and subject.
func (s *OIDCStore) FindIdentity(ctx context.Context, providerID, subject string) (*model.OIDCIdentity, error) {
	var ident model.OIDCIdentity
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, provider_id, subject, email, created_at
		 FROM oidc_identities WHERE provider_id = $1 AND subject = $2`,
		providerID, subject,
	).Scan(&ident.ID, &ident.UserID, &ident.ProviderID, &ident.Subject, &ident.Email, &ident.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("finding oidc identity: %w", err)
	}
	return &ident, nil
}

// CreateIdentity links an OIDC subject to a user.
func (s *OIDCStore) CreateIdentity(ctx context.Context, ident *model.OIDCIdentity) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO oidc_identities (user_id, provider_id, subject, email)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		ident.UserID, ident.ProviderID, ident.Subject, ident.Email,
	).Scan(&ident.ID, &ident.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating oidc identity: %w", err)
	}
	return nil
}

// FindIdentitiesByUser returns all OIDC identities linked to a user.
func (s *OIDCStore) FindIdentitiesByUser(ctx context.Context, userID string) ([]model.OIDCIdentity, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, provider_id, subject, email, created_at
		 FROM oidc_identities WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing oidc identities: %w", err)
	}
	defer rows.Close()

	var identities []model.OIDCIdentity
	for rows.Next() {
		var i model.OIDCIdentity
		if err := rows.Scan(&i.ID, &i.UserID, &i.ProviderID, &i.Subject, &i.Email, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning oidc identity: %w", err)
		}
		identities = append(identities, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating oidc identities: %w", err)
	}
	return identities, nil
}
