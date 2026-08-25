package postgres

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWalletRepo implements the WalletRepository interface with PostgreSQL.
type PostgresWalletRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresWalletRepo creates a new PostgreSQL wallet repository.
func NewPostgresWalletRepo(pool *pgxpool.Pool) *PostgresWalletRepo {
	return &PostgresWalletRepo{pool: pool}
}

// Create inserts a new wallet into the database.
func (r *PostgresWalletRepo) Create(ctx context.Context, wallet *domain.Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, balance, bonus_balance, currency, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		wallet.ID,
		wallet.UserID,
		wallet.Balance,
		wallet.BonusBalance,
		wallet.Currency,
		wallet.Version,
		wallet.CreatedAt,
		wallet.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert wallet: %w", err)
	}
	return nil
}

// GetByID retrieves a wallet by its UUID.
func (r *PostgresWalletRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
	query := `
		SELECT id, user_id, balance, bonus_balance, currency, version, created_at, updated_at
		FROM wallets
		WHERE id = $1`

	wallet := &domain.Wallet{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Balance,
		&wallet.BonusBalance,
		&wallet.Currency,
		&wallet.Version,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get wallet by id: %w", err)
	}
	return wallet, nil
}

// GetByUserID retrieves the wallet belonging to a specific user.
func (r *PostgresWalletRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	query := `
		SELECT id, user_id, balance, bonus_balance, currency, version, created_at, updated_at
		FROM wallets
		WHERE user_id = $1`

	wallet := &domain.Wallet{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Balance,
		&wallet.BonusBalance,
		&wallet.Currency,
		&wallet.Version,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get wallet by user id: %w", err)
	}
	return wallet, nil
}

// UpdateBalance atomically updates a wallet's balance using optimistic locking.
func (r *PostgresWalletRepo) UpdateBalance(ctx context.Context, id uuid.UUID, amount int64, version int) error {
	query := `
		UPDATE wallets
		SET balance = balance + $2, version = version + 1, updated_at = NOW()
		WHERE id = $1 AND version = $3`

	tag, err := r.pool.Exec(ctx, query, id, amount, version)
	if err != nil {
		return fmt.Errorf("update wallet balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("wallet not found or version conflict")
	}
	return nil
}

// CreateTransaction inserts a wallet transaction record within a transaction.
func (r *PostgresWalletRepo) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	return r.withTx(ctx, func(t pgx.Tx) error {
		query := `
			INSERT INTO transactions (id, wallet_id, type, amount, balance_after, reference, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err := t.Exec(ctx, query,
			tx.ID,
			tx.WalletID,
			tx.Type,
			tx.Amount,
			tx.Balance,
			tx.Reference,
			tx.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert transaction: %w", err)
		}
		return nil
	})
}

func (r *PostgresWalletRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
