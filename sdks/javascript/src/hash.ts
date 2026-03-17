import { createHash } from 'node:crypto'

/**
 * Returns a deterministic bucket (0-99) for a given flag key and user ID.
 * Uses SHA-256 for distribution.
 *
 * MUST match the Go backend: binary.BigEndian.Uint64(sha256(flagKey+userID)[:8]) % 100
 */
export function consistentHash(flagKey: string, userId: string): number {
  const hash = createHash('sha256').update(flagKey + userId).digest()
  // Read first 8 bytes as big-endian uint64 — matches Go backend
  const value = hash.readBigUInt64BE(0)
  return Number(value % 100n)
}
