package mailer

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// verificationTokenBytes is the entropy of a single-use email verification
// token (32 bytes -> 43 URL-safe characters).
const verificationTokenBytes = 32

// NewVerifyToken returns a cryptographically random, URL-safe single-use token
// for the email verification flow. Only its SHA-256 hash is ever persisted
// (see service.hashToken); the raw value is dispatched to the player and never
// stored.
func NewVerifyToken() (string, error) {
	b := make([]byte, verificationTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate verification token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
