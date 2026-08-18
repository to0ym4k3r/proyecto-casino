package postgres

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSolanaTxRepo implements Solana transaction persistence with PostgreSQL.
type PostgresSolanaTxRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresSolanaTxRepo creates a new PostgreSQL Solana transaction repository.
func NewPostgresSolanaTxRepo(pool *pgxpool.Pool) *PostgresSolanaTxRepo {
	return &PostgresSolanaTxRepo{pool: pool}
}

// Create inserts a new Solana transaction record.
func (r *PostgresSolanaTxRepo) Create(ctx context.Context, tx *domain.SolanaTx) error {
	query := `
		INSERT INTO solana_txs (id, wallet_id, tx_hash, type, amount, from_address, to_address, confirmations, network_fee, usd_value_at_time, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.pool.Exec(ctx, query,
		tx.ID, tx.WalletID, tx.TxHash, tx.Type, tx.Amount,
		tx.FromAddress, tx.ToAddress, tx.Confirmations,
		tx.NetworkFee, tx.UsdValue, tx.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert solana tx: %w", err)
	}
	return nil
}

// ListByWalletID returns Solana transactions for a wallet with filter, limit/offset pagination.
func (r *PostgresSolanaTxRepo) ListByWalletID(
	ctx context.Context,
	walletID uuid.UUID,
	txType string,
	after, before *time.Time,
	limit, offset int,
) ([]domain.SolanaTx, int, error) {
	// Build dynamic query with optional filters
	baseQuery := `FROM solana_txs WHERE wallet_id = $1`
	args := []any{walletID}
	argIdx := 2

	if txType != "" {
		baseQuery += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, txType)
		argIdx++
	}
	if after != nil {
		baseQuery += fmt.Sprintf(" AND created_at > $%d", argIdx)
		args = append(args, *after)
		argIdx++
	}
	if before != nil {
		baseQuery += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, *before)
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count solana txs: %w", err)
	}

	// Fetch page
	selectQuery := `
		SELECT id, wallet_id, tx_hash, type, amount, from_address, to_address,
		       confirmations, network_fee, usd_value_at_time, created_at
		` + baseQuery + `
		ORDER BY created_at DESC
		LIMIT $` + fmt.Sprintf("%d", argIdx) + ` OFFSET $` + fmt.Sprintf("%d", argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list solana txs: %w", err)
	}
	defer rows.Close()

	var result []domain.SolanaTx
	for rows.Next() {
		var tx domain.SolanaTx
		if err := rows.Scan(
			&tx.ID, &tx.WalletID, &tx.TxHash, &tx.Type, &tx.Amount,
			&tx.FromAddress, &tx.ToAddress, &tx.Confirmations,
			&tx.NetworkFee, &tx.UsdValue, &tx.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan solana tx: %w", err)
		}
		result = append(result, tx)
	}
	return result, total, nil
}

// GetByTxHash retrieves a Solana transaction by its on-chain hash.
func (r *PostgresSolanaTxRepo) GetByTxHash(ctx context.Context, txHash string) (*domain.SolanaTx, error) {
	query := `
		SELECT id, wallet_id, tx_hash, type, amount, from_address, to_address,
		       confirmations, network_fee, usd_value_at_time, created_at
		FROM solana_txs WHERE tx_hash = $1`

	tx := &domain.SolanaTx{}
	err := r.pool.QueryRow(ctx, query, txHash).Scan(
		&tx.ID, &tx.WalletID, &tx.TxHash, &tx.Type, &tx.Amount,
		&tx.FromAddress, &tx.ToAddress, &tx.Confirmations,
		&tx.NetworkFee, &tx.UsdValue, &tx.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get solana tx by hash: %w", err)
	}
	return tx, nil
}
