package auth_test

import (
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !auth.VerifyPassword(hash, "mysecretpassword") {
		t.Error("VerifyPassword returned false for correct password")
	}

	if auth.VerifyPassword(hash, "wrongpassword") {
		t.Error("VerifyPassword returned true for wrong password")
	}
}
