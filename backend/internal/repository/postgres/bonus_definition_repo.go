package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBonusDefinitionRepo implements BonusDefinitionRepository with PostgreSQL.
type PostgresBonusDefinitionRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresBonusDefinitionRepo creates a new PostgreSQL bonus definition repository.
func NewPostgresBonusDefinitionRepo(pool *pgxpool.Pool) *PostgresBonusDefinitionRepo {
	return &PostgresBonusDefinitionRepo{pool: pool}
}

// Create inserts a new bonus definition into the database.
func (r *PostgresBonusDefinitionRepo) Create(ctx context.Context, def *domain.BonusDefinition) error {
	configJSON, err := json.Marshal(def.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	query := `
		INSERT INTO bonus_definitions (id, code, name, description, wagering_multiplier, expiry_days,
			max_bonus_amount_cents, min_deposit_cents, is_active, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err = r.pool.Exec(ctx, query,
		def.ID,
		def.Code,
		def.Name,
		def.Description,
		def.WageringMultiplier,
		def.ExpiryDays,
		def.MaxBonusAmountCents,
		def.MinDepositCents,
		def.IsActive,
		configJSON,
		def.CreatedAt,
		def.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert bonus definition: %w", err)
	}
	return nil
}

// GetByID retrieves a bonus definition by its UUID.
func (r *PostgresBonusDefinitionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.BonusDefinition, error) {
	query := `
		SELECT id, code, name, description, wagering_multiplier, expiry_days,
			max_bonus_amount_cents, min_deposit_cents, is_active, config, created_at, updated_at
		FROM bonus_definitions
		WHERE id = $1`

	def := &domain.BonusDefinition{}
	var configJSON []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&def.ID,
		&def.Code,
		&def.Name,
		&def.Description,
		&def.WageringMultiplier,
		&def.ExpiryDays,
		&def.MaxBonusAmountCents,
		&def.MinDepositCents,
		&def.IsActive,
		&configJSON,
		&def.CreatedAt,
		&def.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("get bonus definition by id: %w", repository.ErrBonusDefinitionNotFound)
		}
		return nil, fmt.Errorf("get bonus definition by id: %w", err)
	}
	if err := json.Unmarshal(configJSON, &def.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return def, nil
}

// GetByCode retrieves a bonus definition by its unique code.
func (r *PostgresBonusDefinitionRepo) GetByCode(ctx context.Context, code string) (*domain.BonusDefinition, error) {
	query := `
		SELECT id, code, name, description, wagering_multiplier, expiry_days,
			max_bonus_amount_cents, min_deposit_cents, is_active, config, created_at, updated_at
		FROM bonus_definitions
		WHERE code = $1`

	def := &domain.BonusDefinition{}
	var configJSON []byte
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&def.ID,
		&def.Code,
		&def.Name,
		&def.Description,
		&def.WageringMultiplier,
		&def.ExpiryDays,
		&def.MaxBonusAmountCents,
		&def.MinDepositCents,
		&def.IsActive,
		&configJSON,
		&def.CreatedAt,
		&def.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("get bonus definition by code: %w", repository.ErrBonusDefinitionNotFound)
		}
		return nil, fmt.Errorf("get bonus definition by code: %w", err)
	}
	if err := json.Unmarshal(configJSON, &def.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return def, nil
}

// ListActive returns all active bonus definitions ordered by code.
func (r *PostgresBonusDefinitionRepo) ListActive(ctx context.Context) ([]domain.BonusDefinition, error) {
	query := `
		SELECT id, code, name, description, wagering_multiplier, expiry_days,
			max_bonus_amount_cents, min_deposit_cents, is_active, config, created_at, updated_at
		FROM bonus_definitions
		WHERE is_active = true
		ORDER BY code`

	return r.list(ctx, query)
}

// ListAll returns all bonus definitions ordered by code.
func (r *PostgresBonusDefinitionRepo) ListAll(ctx context.Context) ([]domain.BonusDefinition, error) {
	query := `
		SELECT id, code, name, description, wagering_multiplier, expiry_days,
			max_bonus_amount_cents, min_deposit_cents, is_active, config, created_at, updated_at
		FROM bonus_definitions
		ORDER BY code`

	return r.list(ctx, query)
}

// Update modifies an existing bonus definition.
func (r *PostgresBonusDefinitionRepo) Update(ctx context.Context, def *domain.BonusDefinition) error {
	configJSON, err := json.Marshal(def.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	query := `
		UPDATE bonus_definitions
		SET code = $2, name = $3, description = $4, wagering_multiplier = $5, expiry_days = $6,
			max_bonus_amount_cents = $7, min_deposit_cents = $8, is_active = $9, config = $10, updated_at = NOW()
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query,
		def.ID,
		def.Code,
		def.Name,
		def.Description,
		def.WageringMultiplier,
		def.ExpiryDays,
		def.MaxBonusAmountCents,
		def.MinDepositCents,
		def.IsActive,
		configJSON,
	)
	if err != nil {
		return fmt.Errorf("update bonus definition: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update bonus definition: %w", repository.ErrBonusDefinitionNotFound)
	}
	return nil
}

// list is a shared helper for queries that return multiple bonus definitions.
func (r *PostgresBonusDefinitionRepo) list(ctx context.Context, query string) ([]domain.BonusDefinition, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list bonus definitions: %w", err)
	}
	defer rows.Close()

	var defs []domain.BonusDefinition
	for rows.Next() {
		var def domain.BonusDefinition
		var configJSON []byte
		if err := rows.Scan(
			&def.ID,
			&def.Code,
			&def.Name,
			&def.Description,
			&def.WageringMultiplier,
			&def.ExpiryDays,
			&def.MaxBonusAmountCents,
			&def.MinDepositCents,
			&def.IsActive,
			&configJSON,
			&def.CreatedAt,
			&def.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bonus definition: %w", err)
		}
		if err := json.Unmarshal(configJSON, &def.Config); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		defs = append(defs, def)
	}

	return defs, rows.Err()
}
