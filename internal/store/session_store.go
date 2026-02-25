package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type SessionStore struct {
	pool *pgxpool.Pool
}

func NewSessionStore(pool *pgxpool.Pool) *SessionStore {
	return &SessionStore{pool: pool}
}

func (s *SessionStore) Create(ctx context.Context, userID string, duration time.Duration) (*model.Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("generating session id: %w", err)
	}

	expiresAt := time.Now().Add(duration)
	var session model.Session
	err = s.pool.QueryRow(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)
		 RETURNING id, user_id, expires_at, created_at`,
		id, userID, expiresAt,
	).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return &session, nil
}

func (s *SessionStore) FindByID(ctx context.Context, id string) (*model.Session, error) {
	var session model.Session
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, expires_at, created_at FROM sessions
		 WHERE id = $1 AND expires_at > NOW()`,
		id,
	).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding session: %w", err)
	}
	return &session, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return fmt.Errorf("deleting expired sessions: %w", err)
	}
	return nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
