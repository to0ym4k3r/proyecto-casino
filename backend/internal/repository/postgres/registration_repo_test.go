package postgres

import (
	"errors"
	"fmt"
	"testing"

	"casino-backend/internal/repository"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestMapUniqueViolation pins the 23505 → ErrUserAlreadyExists translation the
// service layer depends on: a duplicate email/username racing the insert must
// surface as the repository's sentinel (mapped to 409 at the HTTP boundary),
// never as an opaque 500. This is a pure function — no database needed.
func TestMapUniqueViolation(t *testing.T) {
	t.Run("nil passes through", func(t *testing.T) {
		if err := mapUniqueViolation(nil); err != nil {
			t.Fatalf("mapUniqueViolation(nil) = %v, want nil", err)
		}
	})

	t.Run("bare 23505 maps to ErrUserAlreadyExists", func(t *testing.T) {
		err := mapUniqueViolation(&pgconn.PgError{Code: "23505"})
		if !errors.Is(err, repository.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want repository.ErrUserAlreadyExists", err)
		}
	})

	t.Run("23505 wrapped in fmt.Errorf maps too", func(t *testing.T) {
		inner := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
		err := mapUniqueViolation(fmt.Errorf("insert user: %w", inner))
		if !errors.Is(err, repository.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want repository.ErrUserAlreadyExists", err)
		}
	})

	t.Run("non-unique errors pass through unchanged", func(t *testing.T) {
		original := &pgconn.PgError{Code: "23503"} // FK violation
		err := mapUniqueViolation(original)
		if !errors.Is(err, repository.ErrUserAlreadyExists) {
			if err != original {
				t.Fatalf("err = %v, want the original error unchanged", err)
			}
		} else {
			t.Fatal("FK violation must not map to ErrUserAlreadyExists")
		}
	})

	t.Run("non-PgError passes through unchanged", func(t *testing.T) {
		original := errors.New("connection reset")
		if err := mapUniqueViolation(original); err != original {
			t.Fatalf("err = %v, want the original error unchanged", err)
		}
	})
}
