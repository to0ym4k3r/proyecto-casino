package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered casino user.
type User struct {
	ID           uuid.UUID  `json:"id"`
	Username     string     `json:"username" validate:"required,min=3,max=50"`
	Email        string     `json:"email" validate:"required,email"`
	PasswordHash string     `json:"-"`
	FirstName    string     `json:"first_name" validate:"required,max=100"`
	LastName     string     `json:"last_name" validate:"required,max=100"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

// RegisterRequest is the payload for user registration.
type RegisterRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=50"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8,max=72"`
	FirstName string `json:"first_name" validate:"required,max=100"`
	LastName  string `json:"last_name" validate:"required,max=100"`
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
