package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// StateData holds OIDC state stored in a cookie during the auth flow.
type StateData struct {
	State string `json:"s"`
	Nonce string `json:"n"`
}

// PendingLink holds OIDC claims for account linking (stored in cookie after callback).
type PendingLink struct {
	ProviderID string `json:"p"`
	Subject    string `json:"s"`
	Email      string `json:"e"`
}

// GenerateRandom returns a random hex string of the given byte length.
func GenerateRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetStateCookie stores OIDC state+nonce in a signed, HttpOnly cookie.
func SetStateCookie(w http.ResponseWriter, secret []byte, data StateData) error {
	return setSignedCookie(w, "oidc_state", secret, data, 10*time.Minute)
}

// GetStateCookie reads and verifies the OIDC state cookie.
func GetStateCookie(r *http.Request, secret []byte) (*StateData, error) {
	var data StateData
	if err := getSignedCookie(r, "oidc_state", secret, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ClearStateCookie removes the OIDC state cookie.
func ClearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// SetPendingLinkCookie stores pending link data in a signed, HttpOnly cookie.
func SetPendingLinkCookie(w http.ResponseWriter, secret []byte, data PendingLink) error {
	return setSignedCookie(w, "oidc_pending", secret, data, 5*time.Minute)
}

// GetPendingLinkCookie reads and verifies the pending link cookie.
func GetPendingLinkCookie(r *http.Request, secret []byte) (*PendingLink, error) {
	var data PendingLink
	if err := getSignedCookie(r, "oidc_pending", secret, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ClearPendingLinkCookie removes the pending link cookie.
func ClearPendingLinkCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_pending",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func setSignedCookie(w http.ResponseWriter, name string, secret []byte, data any, maxAge time.Duration) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling cookie data: %w", err)
	}

	sig := sign(secret, payload)
	value := base64.URLEncoding.EncodeToString(payload) + "." + base64.URLEncoding.EncodeToString(sig)

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	})
	return nil
}

func getSignedCookie(r *http.Request, name string, secret []byte, dest any) error {
	cookie, err := r.Cookie(name)
	if err != nil {
		return fmt.Errorf("cookie %s not found: %w", name, err)
	}

	dot := -1
	for i := len(cookie.Value) - 1; i >= 0; i-- {
		if cookie.Value[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return fmt.Errorf("invalid cookie format")
	}

	payloadB64 := cookie.Value[:dot]
	sigB64 := cookie.Value[dot+1:]

	payload, err := base64.URLEncoding.DecodeString(payloadB64)
	if err != nil {
		return fmt.Errorf("decoding payload: %w", err)
	}

	sig, err := base64.URLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	if !hmac.Equal(sig, sign(secret, payload)) {
		return fmt.Errorf("invalid cookie signature")
	}

	if err := json.Unmarshal(payload, dest); err != nil {
		return fmt.Errorf("unmarshaling cookie data: %w", err)
	}

	return nil
}

func sign(secret, data []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return mac.Sum(nil)
}
