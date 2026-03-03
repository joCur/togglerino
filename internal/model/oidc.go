package model

import "time"

type OIDCProvider struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	IssuerURL    string    `json:"issuer_url"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"`
	Scopes       string    `json:"scopes"`
	DefaultRole  Role      `json:"default_role"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OIDCIdentity struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ProviderID string    `json:"provider_id"`
	Subject    string    `json:"subject"`
	Email      string    `json:"email,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
