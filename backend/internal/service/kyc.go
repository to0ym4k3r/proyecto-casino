package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
)

var (
	// ErrKycAlreadyExists is returned when a user submits KYC identity fields
	// a second time: kyc_submissions.user_id is UNIQUE (design D7), so the
	// repository surfaces the duplicate as 409.
	ErrKycAlreadyExists = errors.New("kyc submission already exists")
	// ErrKycNotFound is returned when no submission exists for the user.
	ErrKycNotFound = errors.New("kyc submission not found")
	// ErrInvalidKycSubmission is returned when the identity payload fails
	// validation; full_name is required.
	ErrInvalidKycSubmission = errors.New("full_name is required")
)

// KycService handles identity verification: submitting identity fields,
// reading back the submission status, and persisting document metadata. Admin
// review lands in a later change (casino-admin), so every submission stays
// pending.
type KycService struct {
	kycRepo repository.KycRepository
}

// NewKycService creates a new KYC service.
func NewKycService(kycRepo repository.KycRepository) *KycService {
	return &KycService{kycRepo: kycRepo}
}

// Submit validates and persists a KYC identity submission for the given user.
// The status is always pending (admin review is out of scope); the returned
// submission is what the repository persisted.
func (s *KycService) Submit(ctx context.Context, userID uuid.UUID, in domain.KycSubmission) (*domain.KycSubmission, error) {
	fullName := strings.TrimSpace(in.FullName)
	if fullName == "" {
		return nil, ErrInvalidKycSubmission
	}

	sub := &domain.KycSubmission{
		ID:             uuid.New(),
		UserID:         userID,
		FullName:       fullName,
		DateOfBirth:    in.DateOfBirth,
		Nationality:    strings.TrimSpace(in.Nationality),
		DocumentType:   strings.TrimSpace(in.DocumentType),
		DocumentNumber: strings.TrimSpace(in.DocumentNumber),
		Status:         domain.KycStatusPending,
		SubmittedAt:    time.Now().UTC(),
	}

	if err := s.kycRepo.Submit(ctx, sub); err != nil {
		if errors.Is(err, repository.ErrKycAlreadyExists) {
			return nil, ErrKycAlreadyExists
		}
		return nil, fmt.Errorf("submit kyc: %w", err)
	}
	return sub, nil
}

// Status returns the user's current submission, or ErrKycNotFound when none
// exists yet.
func (s *KycService) Status(ctx context.Context, userID uuid.UUID) (*domain.KycSubmission, error) {
	sub, err := s.kycRepo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrKycNotFound) {
			return nil, ErrKycNotFound
		}
		return nil, fmt.Errorf("get kyc status: %w", err)
	}
	return sub, nil
}

// SaveDocument persists the metadata row of a stored KYC document. The file
// itself is written through the storage layer before this is called; the row
// carries the generated stored name so the user filename is never a key.
func (s *KycService) SaveDocument(ctx context.Context, doc *domain.KycDocument) error {
	if err := s.kycRepo.CreateDocument(ctx, doc); err != nil {
		return fmt.Errorf("save kyc document: %w", err)
	}
	return nil
}
