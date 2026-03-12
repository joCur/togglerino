package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

// mockPATFinder implements PATFinder for testing.
type mockPATFinder struct {
	tokens map[string]*model.PersonalAccessToken
}

func (m *mockPATFinder) FindByHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error) {
	pat, ok := m.tokens[hash]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return pat, nil
}

func (m *mockPATFinder) UpdateLastUsed(ctx context.Context, id string) error {
	return nil
}

// mockUserFinder implements UserFinder for testing.
type mockUserFinder struct {
	users map[string]*model.User
}

func (m *mockUserFinder) FindByID(ctx context.Context, id string) (*model.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return user, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func TestSessionOrPATAuth_ValidPAT(t *testing.T) {
	const rawToken = "pat_testtoken123"
	hash := hashToken(rawToken)

	userID := "user-1"
	pat := &model.PersonalAccessToken{
		ID:        "pat-1",
		UserID:    userID,
		ExpiresAt: nil, // no expiry
	}
	user := &model.User{ID: userID, Email: "test@example.com"}

	pats := &mockPATFinder{tokens: map[string]*model.PersonalAccessToken{hash: pat}}
	users := &mockUserFinder{users: map[string]*model.User{userID: user}}

	mw := SessionOrPATAuth(nil, users, pats)

	var gotUser *model.User
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotUser == nil || gotUser.ID != userID {
		t.Fatalf("expected user %q in context, got %v", userID, gotUser)
	}
}

func TestSessionOrPATAuth_ExpiredPAT(t *testing.T) {
	const rawToken = "pat_expiredtoken"
	hash := hashToken(rawToken)

	past := time.Now().Add(-time.Hour)
	pat := &model.PersonalAccessToken{
		ID:        "pat-2",
		UserID:    "user-2",
		ExpiresAt: &past,
	}

	pats := &mockPATFinder{tokens: map[string]*model.PersonalAccessToken{hash: pat}}
	users := &mockUserFinder{users: map[string]*model.User{}}

	mw := SessionOrPATAuth(nil, users, pats)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired PAT, got %d", rr.Code)
	}
}

func TestSessionOrPATAuth_FallsBackToSession(t *testing.T) {
	sessionUser := &model.User{ID: "session-user", Email: "session@example.com"}

	// Fake session middleware that always sets a test user.
	fakeSessionAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := ContextWithUser(r.Context(), sessionUser)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	pats := &mockPATFinder{tokens: map[string]*model.PersonalAccessToken{}}
	users := &mockUserFinder{users: map[string]*model.User{}}

	mw := SessionOrPATAuth(fakeSessionAuth, users, pats)

	var gotUser *model.User
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// No Authorization header → should fall back to session auth.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from session fallback, got %d", rr.Code)
	}
	if gotUser == nil || gotUser.ID != sessionUser.ID {
		t.Fatalf("expected session user %q in context, got %v", sessionUser.ID, gotUser)
	}
}
