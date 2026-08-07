package postgres

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRegistrationRepo implements repository.RegistrationRepository with
// PostgreSQL. A registration is all-or-nothing: the user, wallet, email
// verification token and initial refresh token are written inside a single
// transaction so a failure can never leave an orphaned user or wallet.
type PostgresRegistrationRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRegistrationRepo creates a new PostgreSQL registration repository.
func NewPostgresRegistrationRepo(pool *pgxpool.Pool) *PostgresRegistrationRepo {
	return &PostgresRegistrationRepo{pool: pool}
}

// CreateUserWithWallet persists the full registration atomically in one
// transaction. Wallet.Balance is expressed in cents: the SQL cast divides by
// 100 so the NUMERIC(15,2) column stays intact (cents in, dollars in DB).
func (r *PostgresRegistrationRepo) CreateUserWithWallet(ctx context.Context, in domain.RegistrationInput) error {
	return r.withTx(ctx, func(t pgx.Tx) error {
		userQuery := `
			INSERT INTO users (id, username, email, password_hash, first_name, last_name, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
		if _, err := t.Exec(ctx, userQuery,
			in.User.ID,
			in.User.Username,
			in.User.Email,
			in.User.PasswordHash,
			in.User.FirstName,
			in.User.LastName,
			in.User.IsActive,
			in.User.CreatedAt,
			in.User.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		walletQuery := `
			INSERT INTO wallets (id, user_id, balance, currency, version, created_at, updated_at)
			VALUES ($1, $2, $3::numeric / 100, $4, $5, $6, $7)`
		if _, err := t.Exec(ctx, walletQuery,
			in.Wallet.ID,
			in.Wallet.UserID,
			in.Wallet.Balance,
			in.Wallet.Currency,
			in.Wallet.Version,
			in.Wallet.CreatedAt,
			in.Wallet.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert wallet: %w", err)
		}

		verificationQuery := `
			INSERT INTO email_verifications (id, user_id, token_hash, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5)`
		if _, err := t.Exec(ctx, verificationQuery,
			in.Verification.ID,
			in.Verification.UserID,
			in.Verification.TokenHash,
			in.Verification.ExpiresAt,
			in.Verification.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert email verification: %w", err)
		}

		if err := insertRefreshToken(ctx, t, in.RefreshToken); err != nil {
			return err
		}
		return nil
	})
}

func (r *PostgresRegistrationRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

var _ repository.RegistrationRepository = (*PostgresRegistrationRepo)(nil)
