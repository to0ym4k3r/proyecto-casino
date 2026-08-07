package domain

import (
	"time"

	"github.com/google/uuid"
)

// EmailVerification represents a single-use, time-limited email verification
// token. Only the SHA-256 hash of the raw token is stored; UsedAt marks it as
// consumed so it can never be accepted twice.
type EmailVerification struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// RegistrationInput bundles every record created atomically when a new player
// registers: the user, their wallet, the email verification token and the
// initial refresh token. RegistrationRepository persists all of them inside a
// single transaction so a failure leaves no orphaned rows.
type RegistrationInput struct {
	User         *User
	Wallet       *Wallet
	Verification *EmailVerification
	RefreshToken *RefreshToken
}
