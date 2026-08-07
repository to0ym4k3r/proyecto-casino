package repository

import (
	"context"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
)

// UserRepository defines data access operations for users.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// WalletRepository defines data access operations for wallets.
type WalletRepository interface {
	Create(ctx context.Context, wallet *domain.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error)
	UpdateBalance(ctx context.Context, id uuid.UUID, amount float64, version int) error
	CreateTransaction(ctx context.Context, tx *domain.Transaction) error
}

// GameRepository defines data access operations for games.
type GameRepository interface {
	Create(ctx context.Context, game *domain.Game) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Game, error)
	List(ctx context.Context, limit, offset int) ([]domain.Game, int, error)
	Update(ctx context.Context, game *domain.Game) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// RefreshTokenRepository defines persistence for rotated refresh tokens.
//
// Only the SHA-256 hash of a refresh JWT is ever stored. Rotate invalidates the
// presented token (marking it replaced_by the new one) and persists the
// replacement in one transaction; RevokeFamily revokes the entire family chain
// in both directions, bounded to the token's user.
type RefreshTokenRepository interface {
	Save(ctx context.Context, rt *domain.RefreshToken) error
	GetByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	Rotate(ctx context.Context, oldID uuid.UUID, replacement *domain.RefreshToken) error
	RevokeFamily(ctx context.Context, tokenID uuid.UUID) error
}

// RegistrationRepository creates the user, wallet, email verification and
// initial refresh token atomically in a single transaction.
type RegistrationRepository interface {
	CreateUserWithWallet(ctx context.Context, in domain.RegistrationInput) error
}

// EmailVerificationRepository validates single-use email verification tokens.
type EmailVerificationRepository interface {
	Verify(ctx context.Context, tokenHash string) error
}

// BetRepository defines data access operations for bets.
type BetRepository interface {
	Create(ctx context.Context, bet *domain.Bet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Bet, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Bet, error)
	Update(ctx context.Context, bet *domain.Bet) error
}
