package security

import (
	"strings"
	"testing"

	"casino-backend/internal/config"
)

// TestConfigValidatesJWTSecret verifies GOV-3: JWT_SECRET is mandatory and must
// be at least 32 bytes. cmd/server fails fast on a missing or short secret,
// while cmd/migrate keeps working because Load() never returns an error and the
// production-default secret was removed.
func TestConfigValidatesJWTSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		wantOK bool
	}{
		{"empty secret is rejected (no default applied)", "", false},
		{"short secret is rejected", "short-secret", false},
		{"31-byte secret is rejected (boundary below 32)", strings.Repeat("a", 31), false},
		{"32-byte secret is accepted (boundary at 32)", strings.Repeat("a", 32), true},
		{"64-byte secret is accepted", strings.Repeat("b", 64), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", tt.secret)
			cfg := config.Load()
			err := cfg.ValidateJWTSecret()
			if tt.wantOK && err != nil {
				t.Fatalf("expected valid secret, got error: %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Fatalf("expected error for JWT_SECRET of %d bytes, got nil", len(tt.secret))
			}
		})
	}

	t.Run("missing secret gets no default and fails validation", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "")
		cfg := config.Load()
		if cfg.JWT.Secret != "" {
			t.Fatalf("expected no default JWT_SECRET, got %q", cfg.JWT.Secret)
		}
		if err := cfg.ValidateJWTSecret(); err == nil {
			t.Fatal("expected error when JWT_SECRET is empty")
		}
	})

	t.Run("validation error names the JWT_SECRET variable", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "too-short")
		err := config.Load().ValidateJWTSecret()
		if err == nil {
			t.Fatal("expected error for short secret")
		}
		if !strings.Contains(err.Error(), "JWT_SECRET") {
			t.Fatalf("error should mention JWT_SECRET, got: %v", err)
		}
	})
}
