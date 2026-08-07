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
)
