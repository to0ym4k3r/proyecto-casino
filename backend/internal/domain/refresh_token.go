package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a persisted refresh token. Only the SHA-256 hash of
// the JWT is stored (leak-safe). Rotation invalidates the previous token by
// setting ReplacedBy; RevokeFamily revokes the whole chain in both directions.
type RefreshToken struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	TokenHash  string     `json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ReplacedBy *uuid.UUID `json:"replaced_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
