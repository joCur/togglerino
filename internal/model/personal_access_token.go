package model

import "time"

// PersonalAccessToken represents a user's API token for programmatic access.
type PersonalAccessToken struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id,omitempty"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// PersonalAccessTokenWithValue includes the plaintext token (only returned on creation).
type PersonalAccessTokenWithValue struct {
	PersonalAccessToken
	Token string `json:"token"`
}
