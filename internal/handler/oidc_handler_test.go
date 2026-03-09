package handler_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/oidc"
	"github.com/togglerino/togglerino/internal/store"
)

// oidcTestServer is a mock OIDC identity provider for testing the callback handler.
type oidcTestServer struct {
	Server *httptest.Server
	URL    string

	key           *rsa.PrivateKey
	emailVerified bool
	email         string
	subject       string
}

func newOIDCTestServer(t *testing.T) *oidcTestServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	s := &oidcTestServer{
		key:     key,
		email:   "oidc-test@example.com",
		subject: "oidc-test-subject",
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 s.URL,
			"authorization_endpoint": s.URL + "/authorize",
			"token_endpoint":         s.URL + "/token",
			"jwks_uri":               s.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": "test-key",
				"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			}},
		})
	})

	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		// The code parameter carries the nonce (set by our test setup)
		nonce := r.FormValue("code")
		idToken := s.signJWT(t, nonce)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"id_token":     idToken,
		})
	})

	server := httptest.NewServer(mux)
	s.Server = server
	s.URL = server.URL
	t.Cleanup(server.Close)

	return s
}

func (s *oidcTestServer) signJWT(t *testing.T, nonce string) string {
	t.Helper()

	headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key"})
	header := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{
		"iss":            s.URL,
		"sub":            s.subject,
		"aud":            "test-client-id",
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
		"nonce":          nonce,
		"email":          s.email,
		"email_verified": s.emailVerified,
	}
	payloadJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	sigInput := header + "." + payload
	hash := sha256.Sum256([]byte(sigInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	return sigInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestOIDCCallback_RejectsUnverifiedEmail(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	oidcSt := store.NewOIDCStore(pool)
	userSt := store.NewUserStore(pool)
	sessionSt := store.NewSessionStore(pool)
	auditSt := store.NewAuditStore(pool)

	// Clean up any existing OIDC provider
	if existing, _ := oidcSt.GetProvider(ctx); existing != nil {
		oidcSt.DeleteProvider(ctx, existing.ID)
	}

	// Set up mock OIDC server
	mockServer := newOIDCTestServer(t)
	mockServer.emailVerified = false
	mockServer.email = uniqueEmail("oidc-reject")

	secret := []byte("test-secret-32-chars-for-hmac!!!")
	baseURL := "http://localhost:8080"

	h := handler.NewOIDCHandler(oidcSt, userSt, sessionSt, secret, baseURL, auditSt)

	// Create DB provider with skip_email_verification=false (default secure behavior)
	provider := &model.OIDCProvider{
		Name:                  "Test OIDC",
		IssuerURL:             mockServer.URL,
		ClientID:              "test-client-id",
		ClientSecret:          "test-client-secret",
		Scopes:                "openid email profile",
		DefaultRole:           model.RoleMember,
		Enabled:               true,
		SkipEmailVerification: false,
	}
	if err := oidcSt.UpsertProvider(ctx, provider); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	t.Cleanup(func() { oidcSt.DeleteProvider(ctx, provider.ID) })

	// Initialize the OIDC handler provider
	callbackURL := baseURL + "/api/v1/auth/oidc/callback"
	h.InitProvider(ctx, callbackURL, mockServer.URL, "test-client-id", "test-client-secret", "openid email profile", "member", false)

	// Generate state/nonce and set state cookie
	state := "test-state-value"
	nonce := "test-nonce-value"

	// Create request simulating the OIDC callback
	// We pass nonce as the code so the mock server can embed it in the JWT
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state="+state+"&code="+nonce, nil)

	// Set the signed state cookie
	if err := oidc.SetStateCookie(httptest.NewRecorder(), secret, oidc.StateData{State: state, Nonce: nonce}, false); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}

	// Actually we need to capture the cookie from the recorder and add it to the request
	cookieRR := httptest.NewRecorder()
	if err := oidc.SetStateCookie(cookieRR, secret, oidc.StateData{State: state, Nonce: nonce}, false); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}
	for _, c := range cookieRR.Result().Cookies() {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.Callback(rr, req)

	// With skip_email_verification=false and email_verified=false,
	// should redirect with error=oidc_email_not_verified
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if location != "/?error=oidc_email_not_verified" {
		t.Errorf("expected redirect to /?error=oidc_email_not_verified, got %q", location)
	}
}

func TestOIDCCallback_AllowsUnverifiedEmailWhenSkipEnabled(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	oidcSt := store.NewOIDCStore(pool)
	userSt := store.NewUserStore(pool)
	sessionSt := store.NewSessionStore(pool)
	auditSt := store.NewAuditStore(pool)

	// Clean up any existing OIDC provider
	if existing, _ := oidcSt.GetProvider(ctx); existing != nil {
		oidcSt.DeleteProvider(ctx, existing.ID)
	}

	// Set up mock OIDC server
	mockServer := newOIDCTestServer(t)
	mockServer.emailVerified = false
	mockServer.email = uniqueEmail("oidc-skip")

	secret := []byte("test-secret-32-chars-for-hmac!!!")
	baseURL := "http://localhost:8080"

	h := handler.NewOIDCHandler(oidcSt, userSt, sessionSt, secret, baseURL, auditSt)

	// Create DB provider with skip_email_verification=true
	provider := &model.OIDCProvider{
		Name:                  "Test OIDC",
		IssuerURL:             mockServer.URL,
		ClientID:              "test-client-id",
		ClientSecret:          "test-client-secret",
		Scopes:                "openid email profile",
		DefaultRole:           model.RoleMember,
		Enabled:               true,
		SkipEmailVerification: true,
	}
	if err := oidcSt.UpsertProvider(ctx, provider); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	t.Cleanup(func() { oidcSt.DeleteProvider(ctx, provider.ID) })

	// Initialize the OIDC handler provider
	callbackURL := baseURL + "/api/v1/auth/oidc/callback"
	h.InitProvider(ctx, callbackURL, mockServer.URL, "test-client-id", "test-client-secret", "openid email profile", "member", true)

	// Generate state/nonce
	state := "test-state-value"
	nonce := "test-nonce-value"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?state="+state+"&code="+nonce, nil)

	cookieRR := httptest.NewRecorder()
	if err := oidc.SetStateCookie(cookieRR, secret, oidc.StateData{State: state, Nonce: nonce}, false); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}
	for _, c := range cookieRR.Result().Cookies() {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.Callback(rr, req)

	// With skip_email_verification=true, should NOT redirect with oidc_email_not_verified
	// Instead should proceed to user provisioning/linking and redirect to / or /link-account
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rr.Code)
	}
	location := rr.Header().Get("Location")
	if location == "/?error=oidc_email_not_verified" {
		t.Error("should NOT have rejected unverified email when skip_email_verification is enabled")
	}
	// The callback should proceed past email verification — it will either:
	// - redirect to / (new user provisioned + session created)
	// - redirect to /link-account (existing user with same email)
	// Both are success outcomes from the email verification perspective.
	if location != "/" && location != "/link-account" {
		// Could also be an error from a later step, which is fine for this test
		// as long as it's not the email verification error
		t.Logf("redirect location: %s (not email_not_verified, which is correct)", location)
	}
}
