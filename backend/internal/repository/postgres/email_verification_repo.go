package postgres

import (
	"context"
	"errors"
	"fmt"

	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEmailVerificationRepo implements repository.EmailVerificationRepository
// with PostgreSQL. Verification is single-use and time-limited: the conditional
// UPDATE consumes the token (used_at) only when it is still unused and
// unexpired, and marks the user verified in the same transaction.
type PostgresEmailVerificationRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresEmailVerificationRepo creates a new PostgreSQL email verification
// repository.
func NewPostgresEmailVerificationRepo(pool *pgxpool.Pool) *PostgresEmailVerificationRepo {
	return &PostgresEmailVerificationRepo{pool: pool}
}

// Verify consumes a verification token and marks its user as email-verified in
// one transaction. It returns ErrInvalidVerificationToken when the token is
// unknown, already used, or expired, without touching the user row.
func (r *PostgresEmailVerificationRepo) Verify(ctx context.Context, tokenHash string) error {
	return r.withTx(ctx, func(t pgx.Tx) error {
		consumeQuery := `
			UPDATE email_verifications
			SET used_at = NOW()
			WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
			RETURNING user_id`

		var userID uuid.UUID
		err := t.QueryRow(ctx, consumeQuery, tokenHash).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.ErrInvalidVerificationToken
		}
		if err != nil {
			return fmt.Errorf("consume verification token: %w", err)
		}

		markQuery := `
			UPDATE users
			SET email_verified_at = NOW(), updated_at = NOW()
			WHERE id = $1`
		tag, err := t.Exec(ctx, markQuery, userID)
		if err != nil {
			return fmt.Errorf("mark email verified: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return repository.ErrInvalidVerificationToken
		}
		return nil
	})
}

func (r *PostgresEmailVerificationRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

var _ repository.EmailVerificationRepository = (*PostgresEmailVerificationRepo)(nil)
