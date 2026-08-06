package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
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
	// or does not contain a "refresh" type claim.
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
)

// AuthService handles authentication, registration, and JWT token management.
type AuthService struct {
	userRepo repository.UserRepository
	cfg      *config.JWTConfig
}

// NewAuthService creates a new authentication service.
func NewAuthService(userRepo repository.UserRepository, cfg *config.JWTConfig) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// Register creates a new user account with a bcrypt-hashed password (cost 12).
//
// It checks for duplicate email and username before inserting. Returns
// ErrUserAlreadyExists if either is already taken. On success the newly
// created User (with generated UUID, timestamps, and IsActive=true) is returned.
func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.User, error) {
	existing, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	existing, _ = s.userRepo.GetByUsername(ctx, req.Username)
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:           uuid.New(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user by email and password, returning a JWT token pair.
//
// It looks up the user by email and compares the bcrypt hash. Returns
// ErrInvalidCredentials on mismatch or if the email is not found.
// On success a TokenPair containing signed access and refresh tokens is returned.
func (s *AuthService) Login(ctx context.Context, req domain.LoginRequest) (*domain.TokenPair, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	tokens, err := s.generateTokenPair(user.ID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, nil
}

// Refresh validates a refresh token and returns a fresh token pair.
//
// The token is parsed and validated for signature (HMAC), expiry, and that the
// "type" claim equals "refresh". The subject claim is extracted as the user ID.
// Returns ErrInvalidRefreshToken if any validation step fails.
// On success a new access + refresh pair is issued (token rotation).
func (s *AuthService) Refresh(ctx context.Context, refreshTokenStr string) (*domain.TokenPair, error) {
	token, err := jwt.Parse(refreshTokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidRefreshToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidRefreshToken
	}

	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return nil, ErrInvalidRefreshToken
	}

	sub, _ := claims["sub"].(string)
	userID, err := uuid.Parse(sub)
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}

	tokens, err := s.generateTokenPair(userID)
	if err != nil {
		return nil, fmt.Errorf("generate tokens: %w", err)
	}

	return tokens, nil
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

// generateTokenPair creates signed access and refresh JWTs for the given user.
//
// The access token uses the configured AccessExpiry; the refresh token uses
// RefreshExpiry. Both are signed with HMAC-SHA256 using the service secret.
// This is an internal helper — not safe for concurrent use without external locking.
func (s *AuthService) generateTokenPair(userID uuid.UUID) (*domain.TokenPair, error) {
	accessClaims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "access",
		"exp":  time.Now().Add(s.cfg.AccessExpiry).Unix(),
		"iat":  time.Now().Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessStr, err := accessToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshClaims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "refresh",
		"exp":  time.Now().Add(s.cfg.RefreshExpiry).Unix(),
		"iat":  time.Now().Unix(),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshStr, err := refreshToken.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessStr,
		RefreshToken: refreshStr,
	}, nil
}
