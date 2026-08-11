package postgres

import (
	"context"
	"errors"
	"fmt"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresUserRepo implements the UserRepository interface with PostgreSQL.
type PostgresUserRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresUserRepo creates a new PostgreSQL user repository.
func NewPostgresUserRepo(pool *pgxpool.Pool) *PostgresUserRepo {
	return &PostgresUserRepo{pool: pool}
}

// Create inserts a new user into the database.
func (r *PostgresUserRepo) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash, first_name, last_name, date_of_birth, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.DateOfBirth,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by their UUID.
func (r *PostgresUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, first_name, last_name, date_of_birth, is_active, email_verified_at, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.DateOfBirth,
		&user.IsActive,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// GetByEmail retrieves a user by their email address.
func (r *PostgresUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, first_name, last_name, date_of_birth, is_active, email_verified_at, created_at, updated_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.DateOfBirth,
		&user.IsActive,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// GetByUsername retrieves a user by their username.
func (r *PostgresUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, first_name, last_name, date_of_birth, is_active, email_verified_at, created_at, updated_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL`

	user := &domain.User{}
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.DateOfBirth,
		&user.IsActive,
		&user.EmailVerifiedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return user, nil
}

// Update modifies an existing user record.
func (r *PostgresUserRepo) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET username = $2, email = $3, first_name = $4, last_name = $5, is_active = $6, updated_at = $7
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.pool.Exec(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.FirstName,
		user.LastName,
		user.IsActive,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// UpdateProfile persists the mutable profile fields for a user. It
// intentionally excludes email and credentials: email is immutable in this
// change (design D6) and password/verification state is owned by the auth
// flows. Uses parameterized SQL only — values are never concatenated.
func (r *PostgresUserRepo) UpdateProfile(ctx context.Context, userID uuid.UUID, username, firstName, lastName string) error {
	query := `
		UPDATE users
		SET username = $2, first_name = $3, last_name = $4, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, query, userID, username, firstName, lastName)
	if err != nil {
		return fmt.Errorf("update user profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrUserNotFound
	}
	return nil
}

// Delete soft-deletes a user by setting deleted_at.
func (r *PostgresUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// DeletePermanent physically deletes the user row. Safe for the register
// rollback: the account was just created in the same request, has no bets or
// KYC rows, and wallet/refresh/verification rows cascade on delete.
func (r *PostgresUserRepo) DeletePermanent(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("permanently delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrUserNotFound
	}
	return nil
}
