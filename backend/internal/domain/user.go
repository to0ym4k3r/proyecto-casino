package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered casino user.
type User struct {
	ID              uuid.UUID  `json:"id"`
	Username        string     `json:"username" validate:"required,min=3,max=50"`
	Email           string     `json:"email" validate:"required,email"`
	PasswordHash    string     `json:"-"`
	FirstName       string     `json:"first_name" validate:"required,max=100"`
	LastName        string     `json:"last_name" validate:"required,max=100"`
	DateOfBirth     time.Time  `json:"date_of_birth" validate:"required"`
	IsActive        bool       `json:"is_active"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

// RegisterRequest is the payload for user registration. DateOfBirth is the
// player's birth date as "2006-01-02"; it is parsed server-side and age-gated
// at >= 18 before any row is created.
type RegisterRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=8,max=72"`
	FirstName   string `json:"first_name" validate:"required,max=100"`
	LastName    string `json:"last_name" validate:"required,max=100"`
	DateOfBirth string `json:"date_of_birth" validate:"required"`
}

// LoginRequest is the payload for user login.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// TokenPair contains access and refresh JWT tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshRequest is the payload for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// VerifyEmailRequest is the payload for email verification. Token is the raw
// single-use verification token from the dispatched email.
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// UpdateProfileRequest is the payload for updating the authenticated user's
// profile. Email is deliberately NOT part of the request: it is immutable in
// this change (a later change re-verifies it). Omitted fields stay unchanged.
type UpdateProfileRequest struct {
	Username  string `json:"username" validate:"omitempty,min=3,max=50"`
	FirstName string `json:"first_name" validate:"omitempty,max=100"`
	LastName  string `json:"last_name" validate:"omitempty,max=100"`
}
