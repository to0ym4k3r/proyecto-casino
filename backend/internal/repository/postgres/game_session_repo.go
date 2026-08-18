package postgres

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresGameSessionRepo implements the GameSessionRepository interface with PostgreSQL.
type PostgresGameSessionRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresGameSessionRepo creates a new PostgreSQL game session repository.
func NewPostgresGameSessionRepo(pool *pgxpool.Pool) *PostgresGameSessionRepo {
	return &PostgresGameSessionRepo{pool: pool}
}

// Create inserts a new game session into the database.
func (r *PostgresGameSessionRepo) Create(ctx context.Context, session *domain.GameSession) error {
	query := `
		INSERT INTO game_sessions (id, user_id, game_id, server_seed, server_seed_hash, status, balance_snapshot, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.GameID,
		session.ServerSeed,
		session.ServerSeedHash,
		session.Status,
		session.BalanceSnapshot,
		session.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("insert game session: %w", err)
	}
	return nil
}

// GetByID retrieves a game session by its UUID.
func (r *PostgresGameSessionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.GameSession, error) {
	query := `
		SELECT id, user_id, game_id, server_seed, server_seed_hash, status, balance_snapshot, started_at, ended_at
		FROM game_sessions
		WHERE id = $1`

	session := &domain.GameSession{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.UserID,
		&session.GameID,
		&session.ServerSeed,
		&session.ServerSeedHash,
		&session.Status,
		&session.BalanceSnapshot,
		&session.StartedAt,
		&session.EndedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get game session by id: %w", err)
	}
	return session, nil
}

// UpdateStatus changes a session's status. When setting to completed or
// abandoned, it also sets ended_at to the current timestamp.
func (r *PostgresGameSessionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.GameSessionStatus) error {
	var query string
	if status == domain.SessionStatusCompleted || status == domain.SessionStatusAbandoned {
		query = `
			UPDATE game_sessions
			SET status = $2, ended_at = NOW()
			WHERE id = $1`
	} else {
		query = `
			UPDATE game_sessions
			SET status = $2
			WHERE id = $1`
	}

	tag, err := r.pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update game session status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("game session not found")
	}
	return nil
}

// FindAbandoned returns all active sessions that started more than the given
// duration ago. These are candidates for cleanup.
func (r *PostgresGameSessionRepo) FindAbandoned(ctx context.Context, olderThan time.Duration) ([]domain.GameSession, error) {
	query := `
		SELECT id, user_id, game_id, server_seed, server_seed_hash, status, balance_snapshot, started_at, ended_at
		FROM game_sessions
		WHERE status = 'active' AND started_at < NOW() - $1::INTERVAL`

	rows, err := r.pool.Query(ctx, query, fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("find abandoned sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.GameSession
	for rows.Next() {
		var s domain.GameSession
		if err := rows.Scan(
			&s.ID,
			&s.UserID,
			&s.GameID,
			&s.ServerSeed,
			&s.ServerSeedHash,
			&s.Status,
			&s.BalanceSnapshot,
			&s.StartedAt,
			&s.EndedAt,
		); err != nil {
			return nil, fmt.Errorf("scan game session: %w", err)
		}
		sessions = append(sessions, s)
	}

	return sessions, nil
}
