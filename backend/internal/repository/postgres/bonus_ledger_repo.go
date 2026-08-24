package postgres

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBonusLedgerRepo implements BonusLedgerRepository with PostgreSQL.
type PostgresBonusLedgerRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresBonusLedgerRepo creates a new PostgreSQL bonus ledger repository.
func NewPostgresBonusLedgerRepo(pool *pgxpool.Pool) *PostgresBonusLedgerRepo {
	return &PostgresBonusLedgerRepo{pool: pool}
}

// Create inserts a new bonus ledger entry (immutable audit trail) inside the
// caller's transaction.
func (r *PostgresBonusLedgerRepo) Create(ctx context.Context, tx pgx.Tx, entry *domain.BonusLedgerEntry) error {
	query := `
		INSERT INTO bonus_ledger (id, grant_id, transaction_type, amount_cents, reference_type, reference_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := tx.Exec(ctx, query,
		entry.ID,
		entry.GrantID,
		entry.TransactionType,
		entry.AmountCents,
		entry.ReferenceType,
		entry.ReferenceID,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert bonus ledger entry: %w", err)
	}
	return nil
}

// ListByGrant returns all ledger entries for a grant, ordered chronologically.
func (r *PostgresBonusLedgerRepo) ListByGrant(ctx context.Context, grantID uuid.UUID) ([]domain.BonusLedgerEntry, error) {
	query := `
		SELECT id, grant_id, transaction_type, amount_cents, reference_type, reference_id, created_at
		FROM bonus_ledger
		WHERE grant_id = $1
		ORDER BY created_at ASC`

	rows, err := r.pool.Query(ctx, query, grantID)
	if err != nil {
		return nil, fmt.Errorf("list bonus ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.BonusLedgerEntry
	for rows.Next() {
		var e domain.BonusLedgerEntry
		if err := rows.Scan(
			&e.ID,
			&e.GrantID,
			&e.TransactionType,
			&e.AmountCents,
			&e.ReferenceType,
			&e.ReferenceID,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bonus ledger entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}
