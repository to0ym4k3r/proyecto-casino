package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DepositAddress represents a user's derived Solana deposit addresses.
type DepositAddress struct {
	UserID    uuid.UUID `json:"user_id"`
	UserIndex int       `json:"user_index"`
	SolAddr   string    `json:"sol_address"`
	USDCAta   string    `json:"usdc_ata"`
}

// DepositAddressRepository defines data access for deposit addresses.
type DepositAddressRepository interface {
	Create(ctx context.Context, da *DepositAddress) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*DepositAddress, error)
	AllocateIndex(ctx context.Context, userID uuid.UUID) (int, error)
}

// PostgresDepositAddressRepo implements DepositAddressRepository with PostgreSQL.
type PostgresDepositAddressRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresDepositAddressRepo creates a new PostgreSQL deposit address repository.
func NewPostgresDepositAddressRepo(pool *pgxpool.Pool) *PostgresDepositAddressRepo {
	return &PostgresDepositAddressRepo{pool: pool}
}

// Create inserts a new deposit address record.
func (r *PostgresDepositAddressRepo) Create(ctx context.Context, da *DepositAddress) error {
	query := `
		INSERT INTO deposit_addresses (user_id, user_index, sol_address, usdc_ata)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO NOTHING`

	_, err := r.pool.Exec(ctx, query, da.UserID, da.UserIndex, da.SolAddr, da.USDCAta)
	if err != nil {
		return fmt.Errorf("insert deposit address: %w", err)
	}
	return nil
}

// GetByUserID retrieves the deposit address for a user.
func (r *PostgresDepositAddressRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*DepositAddress, error) {
	query := `
		SELECT user_id, user_index, sol_address, usdc_ata
		FROM deposit_addresses
		WHERE user_id = $1`

	da := &DepositAddress{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&da.UserID,
		&da.UserIndex,
		&da.SolAddr,
		&da.USDCAta,
	)
	if err != nil {
		return nil, fmt.Errorf("get deposit address by user id: %w", err)
	}
	return da, nil
}

// AllocateIndex atomically allocates the next user index from the sequence.
// Returns the allocated index. The DB sequence guarantees uniqueness.
func (r *PostgresDepositAddressRepo) AllocateIndex(ctx context.Context, userID uuid.UUID) (int, error) {
	var index int
	query := `SELECT nextval('user_deposit_index_seq')`
	err := r.pool.QueryRow(ctx, query).Scan(&index)
	if err != nil {
		return 0, fmt.Errorf("allocate deposit index: %w", err)
	}
	return index, nil
}
