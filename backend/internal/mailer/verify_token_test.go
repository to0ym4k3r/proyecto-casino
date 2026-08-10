package mailer

import (
	"strings"
	"testing"
)

// TestNewVerifyToken generates the shared single-use verification token used
// by the auth flow: it must be non-empty, URL-safe (no padding, no reserved
// chars) and unique per call so replay of a different session can never hit
// the same hash.
func TestNewVerifyToken(t *testing.T) {
	tok, err := NewVerifyToken()
	if err != nil {
		t.Fatalf("NewVerifyToken() error = %v", err)
	}
	if tok == "" {
		t.Fatal("NewVerifyToken() returned empty token")
	}
	if strings.ContainsAny(tok, "+/=") {
		t.Errorf("token %q contains reserved/padded base64 chars", tok)
	}
	if len(tok) < 32 {
		t.Errorf("token too short: %d chars, want >= 32", len(tok))
	}

	other, err := NewVerifyToken()
	if err != nil {
		t.Fatalf("second NewVerifyToken() error = %v", err)
	}
	if other == tok {
		t.Fatal("NewVerifyToken() returned the same token twice")
	}
}

// TestVerifyLink pins the frozen contract: the URL must reach POST /api/v1/auth/verify.
func TestVerifyLink(t *testing.T) {
	if got := verifyLink("http://localhost:8080", "abc"); got != "http://localhost:8080/api/v1/auth/verify?token=abc" {
		t.Errorf("verifyLink() = %q, want the frozen /api/v1/auth/verify contract", got)
	}
}
