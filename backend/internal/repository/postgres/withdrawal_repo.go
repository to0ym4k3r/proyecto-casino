package postgres

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWithdrawalRepo implements the WithdrawalRepository interface with PostgreSQL.
type PostgresWithdrawalRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresWithdrawalRepo creates a new PostgreSQL withdrawal repository.
func NewPostgresWithdrawalRepo(pool *pgxpool.Pool) *PostgresWithdrawalRepo {
	return &PostgresWithdrawalRepo{pool: pool}
}

// Create inserts a new withdrawal record.
func (r *PostgresWithdrawalRepo) Create(ctx context.Context, w *domain.Withdrawal) error {
	query := `
		INSERT INTO withdrawals (id, user_id, amount, currency, to_address, tx_hash, status, created_at, resolved_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, query,
		w.ID, w.UserID, w.Amount, w.Currency, w.ToAddress,
		w.TxHash, w.Status, w.CreatedAt, w.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}
	return nil
}

// GetByID retrieves a withdrawal by its ID.
func (r *PostgresWithdrawalRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
	query := `
		SELECT id, user_id, amount, currency, to_address, tx_hash, status, created_at, resolved_at
		FROM withdrawals WHERE id = $1`

	w := &domain.Withdrawal{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&w.ID, &w.UserID, &w.Amount, &w.Currency, &w.ToAddress,
		&w.TxHash, &w.Status, &w.CreatedAt, &w.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get withdrawal by id: %w", err)
	}
	return w, nil
}

// ListByUserID returns withdrawals for a user with limit/offset pagination.
func (r *PostgresWithdrawalRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Withdrawal, error) {
	query := `
		SELECT id, user_id, amount, currency, to_address, tx_hash, status, created_at, resolved_at
		FROM withdrawals WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	defer rows.Close()

	var result []domain.Withdrawal
	for rows.Next() {
		var w domain.Withdrawal
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.Amount, &w.Currency, &w.ToAddress,
			&w.TxHash, &w.Status, &w.CreatedAt, &w.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}
		result = append(result, w)
	}
	return result, nil
}

// GetDailyTotal returns the sum of withdrawal amounts for a user since the given time.
func (r *PostgresWithdrawalRepo) GetDailyTotal(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM withdrawals
		WHERE user_id = $1 AND status != $2 AND created_at >= $3`

	var total int64
	err := r.pool.QueryRow(ctx, query, userID, domain.WithdrawalStatusRejected, since).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get daily total: %w", err)
	}
	return total, nil
}

// UpdateStatus updates the status and optional tx_hash of a withdrawal.
func (r *PostgresWithdrawalRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WithdrawalStatus, txHash string) error {
	query := `
		UPDATE withdrawals
		SET status = $2, tx_hash = CASE WHEN $3 = '' THEN tx_hash ELSE $3 END,
		    resolved_at = CASE WHEN $2 IN ($4, $5) THEN NOW() ELSE resolved_at END
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id, status, txHash,
		domain.WithdrawalStatusCompleted, domain.WithdrawalStatusRejected)
	if err != nil {
		return fmt.Errorf("update withdrawal status: %w", err)
	}
	return nil
}
