package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type InviteStore struct {
	pool *pgxpool.Pool
}

func NewInviteStore(pool *pgxpool.Pool) *InviteStore {
	return &InviteStore{pool: pool}
}

// Create inserts a new invite and populates its ID and CreatedAt fields.
func (s *InviteStore) Create(ctx context.Context, invite *model.Invite) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO invites (email, role, token, expires_at, invited_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		invite.Email, invite.Role, invite.Token, invite.ExpiresAt, invite.InvitedBy,
	).Scan(&invite.ID, &invite.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating invite: %w", err)
	}
	return nil
}

// FindByToken looks up an invite by its token string.
func (s *InviteStore) FindByToken(ctx context.Context, token string) (*model.Invite, error) {
	var invite model.Invite
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, role, token, expires_at, accepted_at, invited_by, created_at
		 FROM invites WHERE token = $1`,
		token,
	).Scan(&invite.ID, &invite.Email, &invite.Role, &invite.Token, &invite.ExpiresAt, &invite.AcceptedAt, &invite.InvitedBy, &invite.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding invite by token: %w", err)
	}
	return &invite, nil
}

// MarkAccepted sets the accepted_at timestamp to now for the given invite.
func (s *InviteStore) MarkAccepted(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE invites SET accepted_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("marking invite accepted: %w", err)
	}
	return nil
}

// ListPending returns all invites that have not yet been accepted.
func (s *InviteStore) ListPending(ctx context.Context) ([]model.Invite, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, role, token, expires_at, accepted_at, invited_by, created_at
		 FROM invites WHERE accepted_at IS NULL ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing pending invites: %w", err)
	}
	defer rows.Close()

	var invites []model.Invite
	for rows.Next() {
		var inv model.Invite
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.InvitedBy, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning invite: %w", err)
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating invites: %w", err)
	}
	return invites, nil
}
