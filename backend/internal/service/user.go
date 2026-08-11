package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
)

var (
	// ErrProfileNotFound is returned when the authenticated user's profile row
	// no longer exists (e.g. the account was deleted after the token was issued).
	ErrProfileNotFound = errors.New("profile not found")
	// ErrInvalidUsername is returned when a requested username falls outside
	// the 3-50 character range.
	ErrInvalidUsername = errors.New("username must be between 3 and 50 characters")
	// ErrInvalidProfileName is returned when first_name or last_name exceeds
	// the 100 character limit.
	ErrInvalidProfileName = errors.New("first_name and last_name must be at most 100 characters")
)

// UserService handles the authenticated user's own profile: reading it and
// updating the mutable profile fields. Email is immutable in this change
// (design D6); credentials are never touched here.
type UserService struct {
	userRepo repository.UserRepository
}

// NewUserService creates a new user profile service.
func NewUserService(userRepo repository.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

// GetProfile returns the authenticated user's profile.
//
// It reuses UserRepository.GetByID, which selects every public profile field;
// the password hash never crosses the HTTP boundary (json:"-" on
// domain.User.PasswordHash).
func (s *UserService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return user, nil
}

// UpdateProfile validates the requested changes and persists the mutable
// profile fields (username, first_name, last_name). Email is immutable and the
// request type deliberately cannot carry it. Omitted fields stay unchanged.
//
// The merged field values are written through repository.UpdateProfile
// (parameterized SQL); a fresh read returns the persisted profile.
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, req domain.UpdateProfileRequest) (*domain.User, error) {
	username := strings.TrimSpace(req.Username)
	firstName := strings.TrimSpace(req.FirstName)
	lastName := strings.TrimSpace(req.LastName)

	if username != "" && (len(username) < 3 || len(username) > 50) {
		return nil, ErrInvalidUsername
	}
	if len(firstName) > 100 || len(lastName) > 100 {
		return nil, ErrInvalidProfileName
	}

	current, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("load profile for update: %w", err)
	}

	// Omitted (or whitespace-only) fields stay unchanged.
	if username == "" {
		username = current.Username
	}
	if firstName == "" {
		firstName = current.FirstName
	}
	if lastName == "" {
		lastName = current.LastName
	}

	if err := s.userRepo.UpdateProfile(ctx, userID, username, firstName, lastName); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}

	updated, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("reload profile after update: %w", err)
	}
	return updated, nil
}
