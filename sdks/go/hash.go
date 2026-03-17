package togglerino

import (
	"crypto/sha256"
	"encoding/binary"
)

// consistentHash returns a deterministic bucket (0-99) for a given flag key
// and user ID. Uses SHA-256 for distribution. Must match the backend's
// ConsistentHash in internal/evaluation/hash.go exactly.
func consistentHash(flagKey, userID string) int {
	h := sha256.Sum256([]byte(flagKey + userID))
	// Take first 8 bytes as big-endian uint64.
	n := binary.BigEndian.Uint64(h[:8])
	return int(n % 100)
}
