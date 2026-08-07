package postgres

import (
	"context"
	"errors"
	"fmt"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRefreshTokenRepo implements repository.RefreshTokenRepository with
// PostgreSQL. Families are modelled as a replaced_by chain (no root_id column):
// RevokeFamily walks the chain in both directions with a recursive CTE, bounded
// by user_id so a token can never revoke another user's family.
type PostgresRefreshTokenRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRefreshTokenRepo creates a new PostgreSQL refresh token repository.
func NewPostgresRefreshTokenRepo(pool *pgxpool.Pool) *PostgresRefreshTokenRepo {
	return &PostgresRefreshTokenRepo{pool: pool}
}

// execer is satisfied by both *pgxpool.Pool and pgx.Tx, letting Save and Rotate
// share the same INSERT statement.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Save persists a new refresh token row.
func (r *PostgresRefreshTokenRepo) Save(ctx context.Context, rt *domain.RefreshToken) error {
	return insertRefreshToken(ctx, r.pool, rt)
}

// GetByHash looks up a refresh token by its SHA-256 hash. The row is returned
// even when revoked or rotated: the caller inspects RevokedAt/ReplacedBy to
// decide whether this is a replay.
func (r *PostgresRefreshTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by, created_at
		FROM refresh_tokens
		WHERE token_hash = $1`

	rt := &domain.RefreshToken{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.TokenHash,
		&rt.ExpiresAt,
		&rt.RevokedAt,
		&rt.ReplacedBy,
		&rt.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, repository.ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token by hash: %w", err)
	}
	return rt, nil
}

// Rotate invalidates the presented token and persists its replacement in one
// transaction. The conditional UPDATE only matches a live token (not yet
// revoked, not yet replaced): when zero rows match the token was already
// rotated (replay), the transaction rolls back and ErrReplayDetected is
// returned so the caller can revoke the whole family.
func (r *PostgresRefreshTokenRepo) Rotate(ctx context.Context, oldID uuid.UUID, replacement *domain.RefreshToken) error {
	return r.withTx(ctx, func(t pgx.Tx) error {
		// The replacement row must exist before the old token's replaced_by
		// FK can point at it (immediate constraint).
		if err := insertRefreshToken(ctx, t, replacement); err != nil {
			return err
		}

		query := `
			UPDATE refresh_tokens
			SET replaced_by = $2, revoked_at = NOW()
			WHERE id = $1 AND replaced_by IS NULL AND revoked_at IS NULL
			RETURNING user_id`

		var userID uuid.UUID
		err := t.QueryRow(ctx, query, oldID, replacement.ID).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.ErrReplayDetected
		}
		if err != nil {
			return fmt.Errorf("rotate refresh token: %w", err)
		}
		return nil
	})
}

// RevokeFamily revokes the presented token and its entire family: every
// ancestor (tokens that replaced it) and every descendant (tokens it replaced)
// reachable through the replaced_by chain, bounded to the same user_id.
//
// NOTE: the design's three-branch UNION CTE (anchor UNION rec1 UNION rec2)
// fails on PostgreSQL with SQLSTATE 42P19 (recursive reference must not appear
// within its non-recursive term), so the two directions are expressed as one
// recursive branch joined with an OR — semantically identical.
func (r *PostgresRefreshTokenRepo) RevokeFamily(ctx context.Context, tokenID uuid.UUID) error {
	query := `
		WITH RECURSIVE family AS (
		    SELECT id, user_id, replaced_by FROM refresh_tokens WHERE id = $1
		    UNION
		    SELECT t.id, t.user_id, t.replaced_by
		    FROM refresh_tokens t
		    JOIN family f ON (t.replaced_by = f.id OR t.id = f.replaced_by) AND t.user_id = f.user_id
		)
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE id IN (SELECT id FROM family) AND revoked_at IS NULL`

	_, err := r.pool.Exec(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}

func insertRefreshToken(ctx context.Context, q execer, rt *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := q.Exec(ctx, query,
		rt.ID,
		rt.UserID,
		rt.TokenHash,
		rt.ExpiresAt,
		rt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (r *PostgresRefreshTokenRepo) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
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

var _ repository.RefreshTokenRepository = (*PostgresRefreshTokenRepo)(nil)
