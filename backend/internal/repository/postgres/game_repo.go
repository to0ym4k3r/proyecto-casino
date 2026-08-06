package postgres

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresGameRepo implements the GameRepository interface with PostgreSQL.
type PostgresGameRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresGameRepo creates a new PostgreSQL game repository.
func NewPostgresGameRepo(pool *pgxpool.Pool) *PostgresGameRepo {
	return &PostgresGameRepo{pool: pool}
}

// Create inserts a new game into the database.
func (r *PostgresGameRepo) Create(ctx context.Context, game *domain.Game) error {
	query := `
		INSERT INTO games (id, name, description, category, min_bet, max_bet, rtp, status, cover_image, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.pool.Exec(ctx, query,
		game.ID,
		game.Name,
		game.Description,
		game.Category,
		game.MinBet,
		game.MaxBet,
		game.RTP,
		game.Status,
		game.CoverImage,
		game.CreatedAt,
		game.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert game: %w", err)
	}
	return nil
}

// GetByID retrieves a game by its UUID.
func (r *PostgresGameRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	query := `
		SELECT id, name, description, category, min_bet, max_bet, rtp, status, cover_image, created_at, updated_at
		FROM games
		WHERE id = $1`

	game := &domain.Game{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&game.ID,
		&game.Name,
		&game.Description,
		&game.Category,
		&game.MinBet,
		&game.MaxBet,
		&game.RTP,
		&game.Status,
		&game.CoverImage,
		&game.CreatedAt,
		&game.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get game by id: %w", err)
	}
	return game, nil
}

// List returns a paginated list of games with total count.
func (r *PostgresGameRepo) List(ctx context.Context, limit, offset int) ([]domain.Game, int, error) {
	countQuery := `SELECT COUNT(*) FROM games`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count games: %w", err)
	}

	query := `
		SELECT id, name, description, category, min_bet, max_bet, rtp, status, cover_image, created_at, updated_at
		FROM games
		ORDER BY name ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()

	var games []domain.Game
	for rows.Next() {
		var game domain.Game
		if err := rows.Scan(
			&game.ID,
			&game.Name,
			&game.Description,
			&game.Category,
			&game.MinBet,
			&game.MaxBet,
			&game.RTP,
			&game.Status,
			&game.CoverImage,
			&game.CreatedAt,
			&game.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, game)
	}

	return games, total, nil
}

// Update modifies an existing game record.
func (r *PostgresGameRepo) Update(ctx context.Context, game *domain.Game) error {
	query := `
		UPDATE games
		SET name = $2, description = $3, category = $4, min_bet = $5, max_bet = $6,
		    rtp = $7, status = $8, cover_image = $9, updated_at = $10
		WHERE id = $1`

	_, err := r.pool.Exec(ctx, query,
		game.ID,
		game.Name,
		game.Description,
		game.Category,
		game.MinBet,
		game.MaxBet,
		game.RTP,
		game.Status,
		game.CoverImage,
		game.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update game: %w", err)
	}
	return nil
}

// Delete removes a game by its UUID.
func (r *PostgresGameRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM games WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete game: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("game not found")
	}
	return nil
}
