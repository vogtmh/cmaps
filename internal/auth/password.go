package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// Local-user password helpers (stdlib only, no bcrypt dependency to keep the
// vendored module set small for the offline server). Format matches entraapi:
// hash = hex(sha256(salt + password)).

// HashPassword generates a random salt and returns (hash, salt).
func HashPassword(password string) (hash, salt string) {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	salt = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:]), salt
}

// CheckPassword verifies a plaintext password against a stored hash+salt using
// a constant-time comparison.
func CheckPassword(password, hash, salt string) bool {
	h := sha256.Sum256([]byte(salt + password))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h[:])), []byte(hash)) == 1
}
