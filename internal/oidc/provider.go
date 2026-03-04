package oidc

import (
	"context"
	"fmt"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Provider wraps go-oidc and oauth2 for OIDC authentication.
type Provider struct {
	verifier     *gooidc.IDTokenVerifier
	oauth2Cfg    oauth2.Config
	oidcProvider *gooidc.Provider
}

// Claims holds the claims extracted from an OIDC ID token.
type Claims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

// NewProvider creates a new OIDC provider from configuration.
func NewProvider(ctx context.Context, issuerURL, clientID, clientSecret, scopes, callbackURL string) (*Provider, error) {
	oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed for %s: %w", issuerURL, err)
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     oidcProvider.Endpoint(),
		RedirectURL:  callbackURL,
		Scopes:       parseScopes(scopes),
	}

	verifier := oidcProvider.Verifier(&gooidc.Config{ClientID: clientID})

	return &Provider{
		verifier:     verifier,
		oauth2Cfg:    oauth2Cfg,
		oidcProvider: oidcProvider,
	}, nil
}

// AuthURL returns the URL to redirect the user to for OIDC authentication.
func (p *Provider) AuthURL(state, nonce string) string {
	return p.oauth2Cfg.AuthCodeURL(state, gooidc.Nonce(nonce))
}

// Exchange trades an authorization code for an ID token and returns its claims.
func (p *Provider) Exchange(ctx context.Context, code, nonce string) (*Claims, error) {
	token, err := p.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}

	if idToken.Nonce != nonce {
		return nil, fmt.Errorf("nonce mismatch")
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("id_token missing sub claim")
	}

	return &claims, nil
}

func parseScopes(scopes string) []string {
	var result []string
	for _, s := range splitAndTrim(scopes) {
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return []string{gooidc.ScopeOpenID, "email", "profile"}
	}
	return result
}

func splitAndTrim(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ','
	})
}
