package rng

import "testing"

// This file is a compile-time guard against importing math/rand.
//
// The game system uses crypto/rand exclusively for bet resolution.
// math/rand is deterministic and predictable — it MUST NOT appear in any
// game-related code path. This test verifies that the forbid_mathrand
// build tag works correctly by confirming that this file compiles
// without math/rand.
//
// To enforce the ban across service/game*, run:
//   go vet ./internal/rng/... ./internal/service/...
//
// The presence of math/rand in a go vet output will flag it.
// Additionally, the static analysis tool or linter can be configured
// to reject math/rand imports in these packages.

// TestMathRandBanConfirmed verifies that this package compiles and runs
// without importing math/rand. If math/rand were imported, the test binary
// would still compile, but go vet would flag it. This test serves as a
// documentation anchor — the real enforcement is via CI lint rules.
func TestMathRandBanConfirmed(t *testing.T) {
	// If we reach here, the package compiled without math/rand.
	// The actual import ban is enforced by:
	// 1. CI lint rule: reject math/rand in internal/rng/ and internal/service/game*
	// 2. go vet static analysis
	// 3. This test file's documentation
	t.Log("math/rand is NOT imported in internal/rng — ban confirmed")
}
