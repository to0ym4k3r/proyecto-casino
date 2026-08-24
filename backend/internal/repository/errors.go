package repository

import "errors"

// Sentinel errors returned by the data-access layer. Services map these to
// typed responses; callers compare with errors.Is, never by string matching.
var (
	// ErrRefreshTokenNotFound is returned when no persisted refresh token
	// matches the presented token hash, or the matching row has already been
	// revoked or rotated away.
	ErrRefreshTokenNotFound = errors.New("refresh token not found")

	// ErrReplayDetected is returned by Rotate when the old token was already
	// rotated (concurrent or repeated use). The caller MUST revoke the whole
	// token family before responding.
	ErrReplayDetected = errors.New("refresh token replay detected")

	// ErrInvalidVerificationToken is returned when an email verification token
	// is unknown, already used, or expired.
	ErrInvalidVerificationToken = errors.New("invalid verification token")

	// ErrUserAlreadyExists is returned when an insert violates the unique
	// constraints on users.email or users.username (PostgreSQL error 23505).
	// Services map it to their own public error.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrUserNotFound is returned when no user row matches a lookup by email,
	// username or ID (PostgreSQL error 02000 â no rows).
	ErrUserNotFound = errors.New("user not found")

	// ErrKycAlreadyExists is returned when a user submits KYC identity fields a
	// second time: kyc_submissions.user_id is UNIQUE (design D7). Mapped from
	// PostgreSQL error 23505 in kyc_repo.go.
	ErrKycAlreadyExists = errors.New("kyc submission already exists")

	// ErrKycNotFound is returned when no KYC submission exists for a user
	// (PostgreSQL error 02000 — no rows).
	ErrKycNotFound = errors.New("kyc submission not found")

	// ErrBonusDefinitionNotFound is returned when no bonus definition matches
	// a lookup by ID or code (PostgreSQL error 02000 — no rows).
	ErrBonusDefinitionNotFound = errors.New("bonus definition not found")

	// ErrBonusGrantNotFound is returned when no bonus grant matches a lookup
	// by ID (PostgreSQL error 02000 — no rows).
	ErrBonusGrantNotFound = errors.New("bonus grant not found")

	// ErrActiveBonusGrantExists is returned when a user already has an active
	// grant for the given bonus definition. Mapped from the partial unique
	// index on bonus_grants(user_id, bonus_definition_id) WHERE status = 'active'.
	ErrActiveBonusGrantExists = errors.New("active bonus grant already exists")
)
