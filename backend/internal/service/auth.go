package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/mailer"
	"casino-backend/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials is returned when the supplied email/password combination
	// does not match any user in the database.
	ErrInvalidCredentials = errors.New("invalid email or password")
	// ErrUserAlreadyExists is returned when a registration attempt uses an email
	// or username that is already taken.
	ErrUserAlreadyExists = errors.New("user with this email or username already exists")
	// ErrInvalidRefreshToken is returned when a refresh token is malformed, expired,
	// revoked, or does not contain a "refresh" type claim.
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	// ErrInvalidDOB is returned when DateOfBirth cannot be parsed as a date-only
	// YYYY-MM-DD value.
	ErrInvalidDOB = errors.New("invalid date of birth")
	// ErrUnderage is returned when a registration's date of birth implies the
	// player is under 18 years of age.
	ErrUnderage = errors.New("date of birth implies player is under 18")
	// ErrEmailNotVerified is returned when a login is attempted before the
	// player's email address has been verified.
	ErrEmailNotVerified = errors.New("email verification required")
	// ErrInvalidVerificationToken is returned when the presented email
	// verification token is unknown, expired, or already consumed.
	ErrInvalidVerificationToken = errors.New("invalid or expired verification token")
	// ErrVerificationEmailFailed is returned when the verification email could
	// not be dispatched and the just-created account was rolled back.
	ErrVerificationEmailFailed = errors.New("verification email dispatch failed")
)

const (
	bcryptCost    = 12
	ageOfMajority = 18
)

// AuthService handles authentication, registration, email verification, and
// JWT token management.
type AuthService struct {
	userRepo         repository.UserRepository
	registrationRepo repository.RegistrationRepository
	refreshRepo      repository.RefreshTokenRepository
	verificationRepo repository.EmailVerificationRepository
	mailer           Mailer
	cfg              *config.JWTConfig
	verifyTTL        time.Duration
}

// NewAuthService creates a new authentication service.
//
// registrationRepo atomically persists the user, wallet, verification record
// and initial refresh token; refreshRepo stores only SHA-256 hashes of refresh
// JWTs; verificationRepo consumes single-use email verification tokens; mailer
// dispatches the verification email (a dispatch failure rolls the account back);
// verifyTTL bounds how long an issued verification token remains valid.
func NewAuthService(
	userRepo repository.UserRepository,
	registrationRepo repository.RegistrationRepository,
	refreshRepo repository.RefreshTokenRepository,
	verificationRepo repository.EmailVerificationRepository,
	mailer Mailer,
	cfg *config.JWTConfig,
	verifyTTL time.Duration,
) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		registrationRepo: registrationRepo,
		refreshRepo:      refreshRepo,
		verificationRepo: verificationRepo,
		mailer:           mailer,
		cfg:              cfg,
		verifyTTL:        verifyTTL,
	}
}

// Register creates a new user account with a bcrypt-hashed password (cost 12).
//
// It normalizes the email to lowercase, parses and age-gates the date of birth
// (18+), checks for duplicate email and username, then atomically creates the
// user, wallet, single-use email verification record and an initial refresh
// token. The verification token is dispatched via the mailer; a dispatch
// failure rolls the account back. On success the newly
// created User (with generated UUID, timestamps, and IsActive=true) is returned.
func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, ErrInvalidCredentials
	}

	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, ErrInvalidDOB
	}
	if ageInYears(dob) < ageOfMajority {
		return nil, ErrUnderage
	}

	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("check email uniqueness: %w", err)
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	existing, err = s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		return nil, fmt.Errorf("check username uniqueness: %w", err)
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		DateOfBirth:  dob,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	wallet := &domain.Wallet{
		ID:        uuid.New(),
		UserID:    user.ID,
		Balance:   0,
		Currency:  "USD",
		CreatedAt: now,
		UpdatedAt: now,
	}

	rawVerificationToken, err := mailer.NewVerifyToken()
	if err != nil {
		return nil, fmt.Errorf("generate verification token: %w", err)
	}
	verification := &domain.EmailVerification{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(rawVerificationToken),
		ExpiresAt: now.Add(s.verifyTTL),
		CreatedAt: now,
	}

	// The initial refresh token exists so the hash can be family-revoked later;
	// it is NOT returned to the client — the player must log in after verifying.
	refreshJWT, err := s.generateRefreshToken(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generate initial refresh token: %w", err)
	}
	refreshRow := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(refreshJWT),
		ExpiresAt: now.Add(s.cfg.RefreshExpiry),
		CreatedAt: now,
	}

	input := domain.RegistrationInput{
		User:         user,
		Wallet:       wallet,
		Verification: verification,
		RefreshToken: refreshRow,
	}
	if err := s.registrationRepo.CreateUserWithWallet(ctx, input); err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Best-effort dispatch (design D3): a failed email must not fail the
	// registration — the token stays valid for the TTL and a resend flow can
	// reuse it in a later change.
	if err := s.mailer.SendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		log.Printf("auth: verification email dispatch failed for %s: %v", email, err)
		// Compensate: physically roll back the just-created account so a failed
		// dispatch never leaves a bricked (unverifiable) account behind. A soft
		// delete would keep the row occupying the UNIQUE email/username and turn
		// every retry into a permanent 409; DeletePermanent frees the identities.
		if delErr := s.userRepo.DeletePermanent(ctx, user.ID); delErr != nil {
			log.Printf("auth: compensating user delete for %s failed: %v", email, delErr)
		}
		return nil, ErrVerificationEmailFailed
	}

	return user, nil
}

// VerifyEmail consumes the single-use verification token and marks the
// player's email as verified. It is idempotent only in the sense that the
// repo owns the single-use semantics; a replayed, expired or unknown token
// yields ErrInvalidVerificationToken.
func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) error {
	if err := s.verificationRepo.Verify(ctx, hashToken(rawToken)); err != nil {
		if errors.Is(err, repository.ErrInvalidVerificationToken) {
			return ErrInvalidVerificationToken
		}
		return fmt.Errorf("verify email: %w", err)
	}
	return nil
}

// Login authenticates a user by email and password, returning a JWT token pair.
//
// It looks up the user by email (case-insensitive, stored lowercase) and
// compares the bcrypt hash. Unverified accounts are rejected with
// ErrEmailNotVerified. On success a TokenPair is returned and the refresh
// token hash is persisted so rotation/replay protection applies.
func (s *AuthService) Login(ctx context.Context, req domain.LoginRequest) (*domain.TokenPair, error) {
	user, err := s.userRepo.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}

	tokens, err := s.generateTokenPair(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	if err := s.persistRefreshToken(ctx, user.ID, tokens.RefreshToken); err != nil {
		return nil, fmt.Errorf("persist refresh token: %w", err)
	}

	return tokens, nil
}

// Refresh validates a refresh token and returns a fresh token pair (rotation).
//
// The token is parsed and validated for signature (HMAC), expiry, and that the
// "type" claim equals "refresh"; the stored SHA-256 hash must exist and belong
// to the subject in the claim. If the token was already replaced or revoked,
// the whole family is revoked (replay detection) and ErrInvalidRefreshToken is
// returned.
func (s *AuthService) Refresh(ctx context.Context, refreshTokenStr string) (*domain.TokenPair, error) {
	userID, err := s.parseRefreshClaims(refreshTokenStr)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	row, err := s.refreshRepo.GetByHash(ctx, hashToken(refreshTokenStr))
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("lookup refresh token: %w", err)
	}

	if row.UserID != userID {
		return nil, ErrInvalidRefreshToken
	}

	// Replay detection MUST precede the revoked check: a rotated token is
	// marked revoked AND replaced, and replaying it is the credential-theft
	// signal that requires the whole family to die.
	if row.ReplacedBy != nil {
		_ = s.refreshRepo.RevokeFamily(ctx, row.ID)
		return nil, ErrInvalidRefreshToken
	}
	if row.RevokedAt != nil {
		return nil, ErrInvalidRefreshToken
	}
	if !row.ExpiresAt.After(time.Now()) {
		return nil, ErrInvalidRefreshToken
	}

	newRefreshJWT, err := s.generateRefreshToken(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	replacement := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    row.UserID,
		TokenHash: hashToken(newRefreshJWT),
		ExpiresAt: time.Now().UTC().Add(s.cfg.RefreshExpiry),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.refreshRepo.Rotate(ctx, row.ID, replacement); err != nil {
		if errors.Is(err, repository.ErrReplayDetected) {
			// Lost the rotation race: another request already replaced this
			// token — fail closed and kill the family.
			_ = s.refreshRepo.RevokeFamily(ctx, row.ID)
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}

	accessToken, err := s.generateAccessToken(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshJWT,
	}, nil
}

// Logout revokes the refresh token family the presented token belongs to.
// Presenting an unknown token yields ErrInvalidRefreshToken; the operation is
// otherwise idempotent (revoking an already-revoked family succeeds).
func (s *AuthService) Logout(ctx context.Context, refreshTokenStr string) error {
	row, err := s.refreshRepo.GetByHash(ctx, hashToken(refreshTokenStr))
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return ErrInvalidRefreshToken
		}
		return fmt.Errorf("lookup refresh token: %w", err)
	}

	if err := s.refreshRepo.RevokeFamily(ctx, row.ID); err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}

// ValidateToken parses and validates a JWT access token, returning the user ID.
//
// It verifies the HMAC signature, checks expiry, and ensures the "type" claim
// is "access". Returns uuid.Nil with an error if the token is invalid, expired,
// or is not an access token.
func (s *AuthService) ValidateToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid claims")
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "access" {
		return uuid.Nil, fmt.Errorf("not an access token")
	}

	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid subject: %w", err)
	}

	return userID, nil
}

// parseRefreshClaims validates a refresh JWT and returns its subject user ID.
func (s *AuthService) parseRefreshClaims(refreshTokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(refreshTokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid claims")
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return uuid.Nil, fmt.Errorf("not a refresh token")
	}

	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid subject")
	}
	return userID, nil
}

// persistRefreshToken stores the SHA-256 hash of a refresh JWT for the user.
func (s *AuthService) persistRefreshToken(ctx context.Context, userID uuid.UUID, refreshJWT string) error {
	now := time.Now().UTC()
	row := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hashToken(refreshJWT),
		ExpiresAt: now.Add(s.cfg.RefreshExpiry),
		CreatedAt: now,
	}
	return s.refreshRepo.Save(ctx, row)
}

// generateAccessToken signs an access JWT with the configured expiry.
func (s *AuthService) generateAccessToken(userID uuid.UUID) (string, error) {
	accessClaims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "access",
		"exp":  time.Now().Add(s.cfg.AccessExpiry).Unix(),
		"iat":  time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return accessStr, nil
}

// generateRefreshToken signs a refresh JWT with the configured expiry. The
// random "nonce" claim guarantees every issued token is distinct — without it,
// two tokens signed in the same second (e.g. login followed immediately by a
// refresh) would be byte-identical, silently defeating rotation and replay
// detection.
func (s *AuthService) generateRefreshToken(userID uuid.UUID) (string, error) {
	nonce, err := randomNonce()
	if err != nil {
		return "", fmt.Errorf("generate refresh nonce: %w", err)
	}
	refreshClaims := jwt.MapClaims{
		"sub":   userID.String(),
		"type":  "refresh",
		"exp":   time.Now().Add(s.cfg.RefreshExpiry).Unix(),
		"iat":   time.Now().Unix(),
		"nonce": nonce,
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("sign refresh token: %w", err)
	}
	return refreshStr, nil
}

// generateTokenPair creates signed access and refresh JWTs for the given user.
func (s *AuthService) generateTokenPair(userID uuid.UUID) (*domain.TokenPair, error) {
	accessStr, err := s.generateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	refreshStr, err := s.generateRefreshToken(userID)
	if err != nil {
		return nil, err
	}
	return &domain.TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
	}, nil
}

// hashToken returns the hex SHA-256 digest of a raw token. Only digests are
// ever persisted — the raw refresh JWT and verification token are never stored.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// randomNonce returns a cryptographically random URL-safe string used as a
// per-token claim so refresh JWTs are always distinct.
func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ageInYears returns the calendar age of a birth date at the current time.
func ageInYears(dob time.Time) int {
	now := time.Now()
	years := now.Year() - dob.Year()
	if now.Month() < dob.Month() || (now.Month() == dob.Month() && now.Day() < dob.Day()) {
		years--
	}
	return years
}
