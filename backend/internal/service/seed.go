package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateServerSeed creates a cryptographically secure 32-byte random seed
// encoded as a 64-character hex string. This seed is stored in the database
// and its SHA-256 hash is returned to the client before play begins.
func GenerateServerSeed() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("service: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// HashSeed returns the SHA-256 hash of the given seed, encoded as a
// 64-character hex string. This hash is sent to the client as a commitment
// before the game begins; the pre-image (server_seed) is never revealed
// until the session ends.
func HashSeed(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(h[:])
}
