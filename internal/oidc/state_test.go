package oidc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStateCookieRoundTrip(t *testing.T) {
	secret := []byte("test-secret-key-for-hmac-signing")

	original := StateData{State: "abc123", Nonce: "nonce456"}

	w := httptest.NewRecorder()
	if err := SetStateCookie(w, secret, original, false); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}

	r := &http.Request{Header: http.Header{"Cookie": w.Header()["Set-Cookie"]}}
	got, err := GetStateCookie(r, secret)
	if err != nil {
		t.Fatalf("GetStateCookie: %v", err)
	}

	if got.State != original.State || got.Nonce != original.Nonce {
		t.Errorf("got %+v, want %+v", got, original)
	}
}

func TestStateCookieTamperedSignature(t *testing.T) {
	secret := []byte("test-secret-key-for-hmac-signing")
	wrongSecret := []byte("wrong-secret-key-for-hmac-check")

	original := StateData{State: "abc123", Nonce: "nonce456"}

	w := httptest.NewRecorder()
	if err := SetStateCookie(w, secret, original, false); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}

	r := &http.Request{Header: http.Header{"Cookie": w.Header()["Set-Cookie"]}}
	_, err := GetStateCookie(r, wrongSecret)
	if err == nil {
		t.Fatal("expected error for tampered cookie, got nil")
	}
}

func TestPendingLinkCookieRoundTrip(t *testing.T) {
	secret := []byte("test-secret-key-for-hmac-signing")

	original := PendingLink{ProviderID: "prov-1", Subject: "sub-123", Email: "user@example.com"}

	w := httptest.NewRecorder()
	if err := SetPendingLinkCookie(w, secret, original, false); err != nil {
		t.Fatalf("SetPendingLinkCookie: %v", err)
	}

	r := &http.Request{Header: http.Header{"Cookie": w.Header()["Set-Cookie"]}}
	got, err := GetPendingLinkCookie(r, secret)
	if err != nil {
		t.Fatalf("GetPendingLinkCookie: %v", err)
	}

	if got.ProviderID != original.ProviderID || got.Subject != original.Subject || got.Email != original.Email {
		t.Errorf("got %+v, want %+v", got, original)
	}
}

func TestGenerateRandom(t *testing.T) {
	a, err := GenerateRandom(32)
	if err != nil {
		t.Fatalf("GenerateRandom: %v", err)
	}
	b, err := GenerateRandom(32)
	if err != nil {
		t.Fatalf("GenerateRandom: %v", err)
	}
	if len(a) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(a))
	}
	if a == b {
		t.Error("two random values should not be equal")
	}
}
