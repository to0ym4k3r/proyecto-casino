// Package rng provides cryptographically secure random number generation
// for game bet resolution. It MUST use crypto/rand exclusively — math/rand
// is forbidden in this package and all game-related code paths.
package rng

import (
	"crypto/rand"
	"math/big"
)

// SecureRandom returns a cryptographically secure random float64 in [0, 1).
// It uses crypto/rand via math/big for unbiased sampling.
func SecureRandom() float64 {
	// Generate a random integer in [0, 2^53) for uniform float64 in [0, 1).
	// Using 2^53 because that's the granularity of float64 mantissa.
	max := big.NewInt(1<<53 - 1)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// crypto/rand.Reader should never fail; if it does, the system has
		// no entropy source and we panic rather than fall back to math/rand.
		panic("rng: crypto/rand failed: " + err.Error())
	}
	return float64(n.Int64()) / float64(1<<53-1)
}
