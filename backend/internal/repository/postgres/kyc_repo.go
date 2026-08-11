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

// PostgresKycRepo implements the repository.KycRepository interface with
// PostgreSQL. A user has at most one submission (user_id UNIQUE, design D7);
// documents are recorded independently so uploads may precede a submission.
type PostgresKycRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresKycRepo creates a new PostgreSQL KYC repository.
func NewPostgresKycRepo(pool *pgxpool.Pool) *PostgresKycRepo {
	return &PostgresKycRepo{pool: pool}
}

// Submit inserts a KYC identity submission. Because kyc_submissions.user_id is
// UNIQUE (design D7), a second submission violates the constraint and surfaces
// as repository.ErrKycAlreadyExists (mapped from PostgreSQL error 23505).
func (r *PostgresKycRepo) Submit(ctx context.Context, sub *domain.KycSubmission) error {
	query := `
		INSERT INTO kyc_submissions (id, user_id, full_name, date_of_birth, nationality, document_type, document_number, status, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, query,
		sub.ID,
		sub.UserID,
		sub.FullName,
		sub.DateOfBirth,
		sub.Nationality,
		sub.DocumentType,
		sub.DocumentNumber,
		sub.Status,
		sub.SubmittedAt,
	)
	if err != nil {
		return mapKycUniqueViolation(fmt.Errorf("insert kyc submission: %w", err))
	}
	return nil
}

// GetByUserID returns the user's submission, or repository.ErrKycNotFound when
// no submission exists yet (PostgreSQL error 02000 — no rows).
func (r *PostgresKycRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.KycSubmission, error) {
	query := `
		SELECT id, user_id, full_name, date_of_birth, nationality, document_type, document_number, status, submitted_at, reviewed_at, reviewed_by, admin_notes
		FROM kyc_submissions
		WHERE user_id = $1`

	sub := &domain.KycSubmission{}
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&sub.ID,
		&sub.UserID,
		&sub.FullName,
		&sub.DateOfBirth,
		&sub.Nationality,
		&sub.DocumentType,
		&sub.DocumentNumber,
		&sub.Status,
		&sub.SubmittedAt,
		&sub.ReviewedAt,
		&sub.ReviewedBy,
		&sub.AdminNotes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrKycNotFound
		}
		return nil, fmt.Errorf("get kyc submission by user: %w", err)
	}
	return sub, nil
}

// CreateDocument inserts a KYC document metadata row. The file itself lives in
// the storage layer under a generated key (never the user filename); this row
// records the stored name, content type and size.
func (r *PostgresKycRepo) CreateDocument(ctx context.Context, doc *domain.KycDocument) error {
	query := `
		INSERT INTO kyc_documents (id, user_id, submission_id, file_name, stored_name, content_type, size_bytes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		doc.ID,
		doc.UserID,
		doc.SubmissionID,
		doc.FileName,
		doc.StoredName,
		doc.ContentType,
		doc.SizeBytes,
		doc.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert kyc document: %w", err)
	}
	return nil
}

// mapKycUniqueViolation converts a PostgreSQL 23505 unique violation into the
// repository's ErrKycAlreadyExists sentinel. It mirrors mapUniqueViolation
// (registration repo) but targets the KYC sentinel, so the pinned registration
// mapping (ErrUserAlreadyExists) is untouched.
func mapKycUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return repository.ErrKycAlreadyExists
	}
	return err
}

var _ repository.KycRepository = (*PostgresKycRepo)(nil)
