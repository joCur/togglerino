package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func createTestProvider(t *testing.T, s *store.OIDCStore) *model.OIDCProvider {
	t.Helper()
	p := &model.OIDCProvider{
		Name:         "Test IDP",
		IssuerURL:    "https://idp.example.com",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Scopes:       "openid email profile",
		DefaultRole:  model.RoleMember,
		Enabled:      true,
	}
	if err := s.UpsertProvider(context.Background(), p); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	return p
}

func TestOIDCStore_UpsertProvider_Insert(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	p := createTestProvider(t, s)

	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	got, err := s.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got.Name != "Test IDP" {
		t.Errorf("name: got %q, want %q", got.Name, "Test IDP")
	}
	if got.IssuerURL != "https://idp.example.com" {
		t.Errorf("issuer_url: got %q, want %q", got.IssuerURL, "https://idp.example.com")
	}

	// Clean up
	s.DeleteProvider(ctx, p.ID)
}

func TestOIDCStore_UpsertProvider_Update_PreservesIdentities(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	// Create provider and a linked identity
	p := createTestProvider(t, s)

	email := uniqueEmail("oidc-cascade")
	user, err := us.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("Create user: %v", err)
	}

	ident := &model.OIDCIdentity{
		UserID:     user.ID,
		ProviderID: p.ID,
		Subject:    "sub-12345",
		Email:      email,
	}
	if err := s.CreateIdentity(ctx, ident); err != nil {
		t.Fatalf("CreateIdentity: %v", err)
	}

	// Update provider config — identity must survive
	updated := &model.OIDCProvider{
		Name:         "Updated IDP",
		IssuerURL:    "https://idp2.example.com",
		ClientID:     "new-client-id",
		ClientSecret: "new-secret",
		Scopes:       "openid",
		DefaultRole:  model.RoleAdmin,
		Enabled:      false,
	}
	if err := s.UpsertProvider(ctx, updated); err != nil {
		t.Fatalf("UpsertProvider (update): %v", err)
	}

	// Provider ID must be preserved (same row updated)
	if updated.ID != p.ID {
		t.Errorf("provider ID changed: got %q, want %q", updated.ID, p.ID)
	}

	// Verify identity still exists
	found, err := s.FindIdentity(ctx, p.ID, "sub-12345")
	if err != nil {
		t.Fatalf("FindIdentity after update: %v", err)
	}
	if found == nil {
		t.Fatal("identity was destroyed by provider update — cascade bug")
	}
	if found.ID != ident.ID {
		t.Errorf("identity ID: got %q, want %q", found.ID, ident.ID)
	}

	// Verify provider fields were updated
	got, err := s.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got.Name != "Updated IDP" {
		t.Errorf("name: got %q, want %q", got.Name, "Updated IDP")
	}
	if got.Enabled != false {
		t.Error("expected provider to be disabled")
	}

	// Clean up
	s.DeleteProvider(ctx, updated.ID)
	us.Delete(ctx, user.ID)
}

func TestOIDCStore_CreateIdentity_And_FindIdentity(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	p := createTestProvider(t, s)

	email := uniqueEmail("oidc-ident")
	user, err := us.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("Create user: %v", err)
	}

	ident := &model.OIDCIdentity{
		UserID:     user.ID,
		ProviderID: p.ID,
		Subject:    "sub-find-test",
		Email:      email,
	}
	if err := s.CreateIdentity(ctx, ident); err != nil {
		t.Fatalf("CreateIdentity: %v", err)
	}
	if ident.ID == "" {
		t.Error("expected non-empty identity ID")
	}

	// Find by provider + subject
	found, err := s.FindIdentity(ctx, p.ID, "sub-find-test")
	if err != nil {
		t.Fatalf("FindIdentity: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find identity")
	}
	if found.UserID != user.ID {
		t.Errorf("user_id: got %q, want %q", found.UserID, user.ID)
	}

	// Not found for different subject
	notFound, err := s.FindIdentity(ctx, p.ID, "sub-nonexistent")
	if err != nil {
		t.Fatalf("FindIdentity (miss): %v", err)
	}
	if notFound != nil {
		t.Error("expected nil for nonexistent subject")
	}

	// Clean up
	s.DeleteProvider(ctx, p.ID)
	us.Delete(ctx, user.ID)
}

func TestOIDCStore_FindIdentitiesByUser(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	p := createTestProvider(t, s)

	email := uniqueEmail("oidc-byuser")
	user, err := us.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("Create user: %v", err)
	}

	if err := s.CreateIdentity(ctx, &model.OIDCIdentity{
		UserID: user.ID, ProviderID: p.ID, Subject: "sub-a", Email: email,
	}); err != nil {
		t.Fatalf("CreateIdentity: %v", err)
	}

	identities, err := s.FindIdentitiesByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindIdentitiesByUser: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected 1 identity, got %d", len(identities))
	}
	if identities[0].Subject != "sub-a" {
		t.Errorf("subject: got %q, want %q", identities[0].Subject, "sub-a")
	}

	// Clean up
	s.DeleteProvider(ctx, p.ID)
	us.Delete(ctx, user.ID)
}

func TestOIDCStore_DeleteProvider(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	p := createTestProvider(t, s)

	if err := s.DeleteProvider(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProvider: %v", err)
	}

	got, err := s.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider after delete: %v", err)
	}
	if got != nil {
		t.Error("expected nil provider after delete")
	}
}

func TestOIDCStore_ProvisionUser(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	p := createTestProvider(t, s)

	email := uniqueEmail("oidc-provision")
	userID, err := s.ProvisionUser(ctx, email, "Jane Doe", model.RoleMember, p.ID, "sub-prov-123")
	if err != nil {
		t.Fatalf("ProvisionUser: %v", err)
	}
	if userID == "" {
		t.Fatal("expected non-empty user ID")
	}

	// Verify user was created
	user, err := us.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if user.Email != email {
		t.Errorf("email: got %q, want %q", user.Email, email)
	}
	if user.DisplayName == nil || *user.DisplayName != "Jane Doe" {
		t.Errorf("display_name: got %v, want %q", user.DisplayName, "Jane Doe")
	}
	if user.PasswordHash != "" {
		t.Errorf("expected empty password_hash for OIDC-provisioned user, got %q", user.PasswordHash)
	}

	// Verify identity was created
	ident, err := s.FindIdentity(ctx, p.ID, "sub-prov-123")
	if err != nil {
		t.Fatalf("FindIdentity: %v", err)
	}
	if ident == nil {
		t.Fatal("expected identity to be created")
	}
	if ident.UserID != userID {
		t.Errorf("identity user_id: got %q, want %q", ident.UserID, userID)
	}
	if ident.Email != email {
		t.Errorf("identity email: got %q, want %q", ident.Email, email)
	}

	// Clean up
	s.DeleteProvider(ctx, p.ID)
	us.Delete(ctx, userID)
}

func TestOIDCStore_SkipEmailVerification(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	// Upsert with skip_email_verification = true
	p := &model.OIDCProvider{
		Name:                  "Test Provider",
		IssuerURL:             "https://example.com",
		ClientID:              "client-id",
		ClientSecret:          "client-secret",
		Scopes:                "openid email",
		DefaultRole:           model.RoleMember,
		Enabled:               true,
		SkipEmailVerification: true,
	}
	if err := s.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	got, err := s.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if !got.SkipEmailVerification {
		t.Error("expected SkipEmailVerification=true, got false")
	}

	// Update to false
	p.SkipEmailVerification = false
	if err := s.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("UpsertProvider (update): %v", err)
	}

	got, err = s.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider (after update): %v", err)
	}
	if got.SkipEmailVerification {
		t.Error("expected SkipEmailVerification=false, got true")
	}

	// Clean up
	s.DeleteProvider(ctx, got.ID)
}

func TestOIDCStore_GetProvider_Empty(t *testing.T) {
	pool := testPool(t)
	s := store.NewOIDCStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := s.GetProvider(ctx)
	if existing != nil {
		s.DeleteProvider(ctx, existing.ID)
	}

	got, err := s.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got != nil {
		t.Error("expected nil provider when none configured")
	}
}
