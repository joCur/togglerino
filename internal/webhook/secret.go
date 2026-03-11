package webhook

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating secret: %w", err)
	}
	return "whsec_" + hex.EncodeToString(b), nil
}

func MaskSecret(secret string) string {
	if len(secret) <= 10 {
		return "****"
	}
	return secret[:10] + "****"
}
