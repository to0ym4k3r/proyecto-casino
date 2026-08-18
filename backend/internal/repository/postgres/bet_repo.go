package postgres

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBetRepo implements the BetRepository interface with PostgreSQL.
type PostgresBetRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresBetRepo creates a new PostgreSQL bet repository.
func NewPostgresBetRepo(pool *pgxpool.Pool) *PostgresBetRepo {
	return &PostgresBetRepo{pool: pool}
}

// Create inserts a new bet into the database.
func (r *PostgresBetRepo) Create(ctx context.Context, bet *domain.Bet) error {
	query := `
		INSERT INTO bets (id, user_id, game_id, session_id, amount, payout, multiplier, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, query,
		bet.ID,
		bet.UserID,
		bet.GameID,
		bet.SessionID,
		bet.Amount,
		bet.Payout,
		bet.Multiplier,
		bet.Status,
		bet.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert bet: %w", err)
	}
	return nil
}

// GetByID retrieves a bet by its UUID.
func (r *PostgresBetRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Bet, error) {
	query := `
		SELECT id, user_id, game_id, session_id, amount, payout, multiplier, status, created_at, resolved_at
		FROM bets
		WHERE id = $1`

	bet := &domain.Bet{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&bet.ID,
		&bet.UserID,
		&bet.GameID,
		&bet.SessionID,
		&bet.Amount,
		&bet.Payout,
		&bet.Multiplier,
		&bet.Status,
		&bet.CreatedAt,
		&bet.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get bet by id: %w", err)
	}
	return bet, nil
}

// GetByUserID returns a paginated list of bets for a given user.
func (r *PostgresBetRepo) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Bet, error) {
	query := `
		SELECT id, user_id, game_id, session_id, amount, payout, multiplier, status, created_at, resolved_at
		FROM bets
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bets by user: %w", err)
	}
	defer rows.Close()

	var bets []domain.Bet
	for rows.Next() {
		var bet domain.Bet
		if err := rows.Scan(
			&bet.ID,
			&bet.UserID,
			&bet.GameID,
			&bet.SessionID,
			&bet.Amount,
			&bet.Payout,
			&bet.Multiplier,
			&bet.Status,
			&bet.CreatedAt,
			&bet.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bet: %w", err)
		}
		bets = append(bets, bet)
	}

	return bets, nil
}

// Update modifies an existing bet record.
func (r *PostgresBetRepo) Update(ctx context.Context, bet *domain.Bet) error {
	query := `
		UPDATE bets
		SET session_id = $2, amount = $3, payout = $4, multiplier = $5, status = $6, resolved_at = $7
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query,
		bet.ID,
		bet.SessionID,
		bet.Amount,
		bet.Payout,
		bet.Multiplier,
		bet.Status,
		bet.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("update bet: %w", err)
	}
	return nil
}
