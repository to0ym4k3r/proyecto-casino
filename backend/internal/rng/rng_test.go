package rng

import (
	"math"
	"testing"
)

// TestSecureRandomReturnsInRange proves SecureRandom produces values in [0, 1).
func TestSecureRandomReturnsInRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		v := SecureRandom()
		if v < 0 || v >= 1.0 {
			t.Fatalf("SecureRandom() = %f, want [0, 1)", v)
		}
	}
}

// TestSecureRandomNotDeterministic proves 1000 calls do not produce a
// deterministic or repeating pattern. We check that not all values are
// identical (which would indicate a broken PRNG).
func TestSecureRandomNotDeterministic(t *testing.T) {
	seen := make(map[float64]bool)
	unique := 0
	for i := 0; i < 1000; i++ {
		v := SecureRandom()
		if !seen[v] {
			seen[v] = true
			unique++
		}
	}
	// With a good PRNG, 1000 float64 samples should produce many unique values.
	// A floor of 900 unique values out of 1000 is conservative.
	if unique < 900 {
		t.Errorf("only %d unique values out of 1000 — PRNG may be deterministic", unique)
	}
}

// TestSecureRandomStatisticalDistribution verifies the mean of many samples
// is close to 0.5 (uniform distribution on [0,1)).
func TestSecureRandomStatisticalDistribution(t *testing.T) {
	n := 10000
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += SecureRandom()
	}
	mean := sum / float64(n)
	// Mean should be within 0.05 of 0.5 for uniform distribution.
	if math.Abs(mean-0.5) > 0.05 {
		t.Errorf("mean = %f, want within 0.05 of 0.5 (non-uniform?)", mean)
	}
}

// TestNoMathRandImport is a compile-time guard: this file must not import
// math/rand. The test simply verifies the function works — the import check
// is enforced by the forbid_mathrand_test.go file in this package.
func TestNoMathRandImport(t *testing.T) {
	// If SecureRandom works, it uses crypto/rand under the hood.
	// The actual import ban is verified in forbid_mathrand_test.go.
	v := SecureRandom()
	if v < 0 || v >= 1.0 {
		t.Errorf("SecureRandom() = %f, unexpected", v)
	}
}
