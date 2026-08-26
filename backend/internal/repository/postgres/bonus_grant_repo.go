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

// PostgresBonusGrantRepo implements BonusGrantRepository with PostgreSQL.
type PostgresBonusGrantRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresBonusGrantRepo creates a new PostgreSQL bonus grant repository.
func NewPostgresBonusGrantRepo(pool *pgxpool.Pool) *PostgresBonusGrantRepo {
	return &PostgresBonusGrantRepo{pool: pool}
}

// Create inserts a new bonus grant inside the caller's transaction. The
// caller is responsible for ensuring no active grant already exists for the
// same user+definition (partial unique index enforces this at the DB level);
// a 23505 violation surfaces as repository.ErrActiveBonusGrantExists.
func (r *PostgresBonusGrantRepo) Create(ctx context.Context, tx pgx.Tx, grant *domain.BonusGrant) error {
	query := `
		INSERT INTO bonus_grants (id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := tx.Exec(ctx, query,
		grant.ID,
		grant.UserID,
		grant.BonusDefinitionID,
		grant.BonusAmountCents,
		grant.WageringRequiredCents,
		grant.TotalWageredCents,
		grant.Status,
		grant.GrantedBy,
		grant.AdminID,
		grant.ExpiresAt,
		grant.CreatedAt,
		grant.UpdatedAt,
	)
	if err != nil {
		return mapBonusGrantUniqueViolation(fmt.Errorf("insert bonus grant: %w", err))
	}
	return nil
}

// mapBonusGrantUniqueViolation maps a PostgreSQL 23505 unique violation from
// bonus_grants inserts to repository.ErrActiveBonusGrantExists.
func mapBonusGrantUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return repository.ErrActiveBonusGrantExists
	}
	return err
}

// GetByID retrieves a bonus grant by its UUID.
func (r *PostgresBonusGrantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.BonusGrant, error) {
	query := `
		SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at
		FROM bonus_grants
		WHERE id = $1`

	grant := &domain.BonusGrant{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&grant.ID,
		&grant.UserID,
		&grant.BonusDefinitionID,
		&grant.BonusAmountCents,
		&grant.WageringRequiredCents,
		&grant.TotalWageredCents,
		&grant.Status,
		&grant.GrantedBy,
		&grant.AdminID,
		&grant.ExpiresAt,
		&grant.CreatedAt,
		&grant.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("get bonus grant by id: %w", repository.ErrBonusGrantNotFound)
		}
		return nil, fmt.Errorf("get bonus grant by id: %w", err)
	}
	return grant, nil
}

// GetActiveByUserAndDefinition returns the active grant for a user+definition
// pair, locking the row with SELECT FOR UPDATE for concurrent claim protection
// until the caller's transaction commits or rolls back.
// Returns ErrBonusGrantNotFound if no active grant exists.
func (r *PostgresBonusGrantRepo) GetActiveByUserAndDefinition(ctx context.Context, tx pgx.Tx, userID, defID uuid.UUID) (*domain.BonusGrant, error) {
	query := `
		SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at
		FROM bonus_grants
		WHERE user_id = $1 AND bonus_definition_id = $2 AND status = 'active'
		FOR UPDATE`

	grant := &domain.BonusGrant{}
	err := tx.QueryRow(ctx, query, userID, defID).Scan(
		&grant.ID,
		&grant.UserID,
		&grant.BonusDefinitionID,
		&grant.BonusAmountCents,
		&grant.WageringRequiredCents,
		&grant.TotalWageredCents,
		&grant.Status,
		&grant.GrantedBy,
		&grant.AdminID,
		&grant.ExpiresAt,
		&grant.CreatedAt,
		&grant.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("get active grant: %w", repository.ErrBonusGrantNotFound)
		}
		return nil, fmt.Errorf("get active grant: %w", err)
	}
	return grant, nil
}

// ListActiveByUser returns all active grants for a user, ordered by creation time.
func (r *PostgresBonusGrantRepo) ListActiveByUser(ctx context.Context, userID uuid.UUID) ([]domain.BonusGrant, error) {
	query := `
		SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at
		FROM bonus_grants
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC`

	return r.listGrants(ctx, query, userID)
}

// ListActiveByUserTx returns all active grants for a user within the caller's
// transaction, ordered by creation time. Used by game integration to query
// grants inside the same transaction that places the bet.
func (r *PostgresBonusGrantRepo) ListActiveByUserTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]domain.BonusGrant, error) {
	query := `
		SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at
		FROM bonus_grants
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC`

	rows, err := tx.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list active grants tx: %w", err)
	}
	defer rows.Close()

	var grants []domain.BonusGrant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active grant tx: %w", err)
		}
		grants = append(grants, g)
	}

	return grants, rows.Err()
}

// ListByUser returns grants for a user filtered by status. Empty status returns all.
func (r *PostgresBonusGrantRepo) ListByUser(ctx context.Context, userID uuid.UUID, status string) ([]domain.BonusGrant, error) {
	var query string
	if status == "" {
		query = `
			SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
				wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
				expires_at, created_at, updated_at
			FROM bonus_grants
			WHERE user_id = $1
			ORDER BY created_at DESC`
		return r.listGrants(ctx, query, userID)
	}

	query = `
		SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at
		FROM bonus_grants
		WHERE user_id = $1 AND status = $2
		ORDER BY created_at DESC`
	return r.listGrants(ctx, query, userID, status)
}

// ListAll returns a filtered, paginated list of grants for admin use.
// status and userID are optional filters; pass empty/nil to skip.
func (r *PostgresBonusGrantRepo) ListAll(ctx context.Context, status string, userID *uuid.UUID, limit, offset int) ([]domain.BonusGrant, int, error) {
	// Build WHERE clause dynamically.
	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if userID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *userID)
		argIdx++
	}

	// Count query.
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM bonus_grants %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count bonus grants: %w", err)
	}

	// Data query with pagination.
	dataQuery := fmt.Sprintf(`
		SELECT id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at
		FROM bonus_grants %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	grants, err := r.listGrants(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	return grants, total, nil
}

// UpdateStatus changes the status of a bonus grant (e.g. active → expired)
// inside the caller's transaction.
func (r *PostgresBonusGrantRepo) UpdateStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string) error {
	query := `
		UPDATE bonus_grants
		SET status = $2, updated_at = NOW()
		WHERE id = $1`

	tag, err := tx.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("update bonus grant status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update bonus grant status: %w", repository.ErrBonusGrantNotFound)
	}
	return nil
}

// IncrementWagered atomically increments total_wagered_cents and transitions
// the grant to 'wagered' status when the wagering requirement is met, inside
// the caller's transaction.
func (r *PostgresBonusGrantRepo) IncrementWagered(ctx context.Context, tx pgx.Tx, id uuid.UUID, amount int64) error {
	query := `
		UPDATE bonus_grants
		SET total_wagered_cents = total_wagered_cents + $2,
			status = CASE WHEN total_wagered_cents + $2 >= wagering_required_cents THEN 'wagered' ELSE status END,
			updated_at = NOW()
		WHERE id = $1 AND status = 'active'`

	tag, err := tx.Exec(ctx, query, id, amount)
	if err != nil {
		return fmt.Errorf("increment wagered: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("increment wagered: %w", repository.ErrBonusGrantNotFound)
	}
	return nil
}

// ExpireStale marks all grants past their expires_at as expired within the
// caller's transaction and returns them. It does NOT zero wallets.bonus_balance
// nor write ledger entries: the caller MUST do both in the SAME transaction
// before committing. The returned grants have status='expired' after the UPDATE.
func (r *PostgresBonusGrantRepo) ExpireStale(ctx context.Context, tx pgx.Tx) ([]domain.BonusGrant, error) {
	query := `
		UPDATE bonus_grants
		SET status = 'expired', updated_at = NOW()
		WHERE status = 'active' AND expires_at < NOW()
		RETURNING id, user_id, bonus_definition_id, bonus_amount_cents,
			wagering_required_cents, total_wagered_cents, status, granted_by, admin_id,
			expires_at, created_at, updated_at`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("expire stale grants: %w", err)
	}
	defer rows.Close()

	var grants []domain.BonusGrant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expired grant: %w", err)
		}
		grants = append(grants, g)
	}

	return grants, rows.Err()
}

// scanGrant scans a single bonus grant row. Extracted for reuse.
func scanGrant(rows pgx.Rows) (domain.BonusGrant, error) {
	var g domain.BonusGrant
	err := rows.Scan(
		&g.ID,
		&g.UserID,
		&g.BonusDefinitionID,
		&g.BonusAmountCents,
		&g.WageringRequiredCents,
		&g.TotalWageredCents,
		&g.Status,
		&g.GrantedBy,
		&g.AdminID,
		&g.ExpiresAt,
		&g.CreatedAt,
		&g.UpdatedAt,
	)
	return g, err
}

// listGrants is a shared helper for queries that return multiple bonus grants.
func (r *PostgresBonusGrantRepo) listGrants(ctx context.Context, query string, args ...interface{}) ([]domain.BonusGrant, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list bonus grants: %w", err)
	}
	defer rows.Close()

	var grants []domain.BonusGrant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("scan bonus grant: %w", err)
		}
		grants = append(grants, g)
	}

	return grants, rows.Err()
}
