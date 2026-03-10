package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func Sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
