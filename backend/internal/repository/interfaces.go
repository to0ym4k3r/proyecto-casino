package repository

import (
	"context"
	"time"

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
	// UpdateProfile persists only the mutable profile fields (username,
	// first_name, last_name). Email and credentials are immutable and are never
	// touched here (design D6).
	UpdateProfile(ctx context.Context, userID uuid.UUID, username, firstName, lastName string) error
	Delete(ctx context.Context, id uuid.UUID) error
	// DeletePermanent physically removes a user row. It is used only by the
	// register rollback compensation, where the account was created seconds ago
	// and has no history to preserve; unlike Delete (soft), it frees the
	// email/username UNIQUE identities so a retry can re-register.
	DeletePermanent(ctx context.Context, id uuid.UUID) error
}

// WalletRepository defines data access operations for wallets.
type WalletRepository interface {
	Create(ctx context.Context, wallet *domain.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error)
	UpdateBalance(ctx context.Context, id uuid.UUID, amount int64, version int) error
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

// GameSessionRepository defines data access operations for game sessions.
type 	GameSessionRepository interface {
	Create(ctx context.Context, session *domain.GameSession) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.GameSession, error)
	GetActiveSession(ctx context.Context, userID, gameID uuid.UUID) (*domain.GameSession, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.GameSessionStatus) error
	FindAbandoned(ctx context.Context, olderThan time.Duration) ([]domain.GameSession, error)
}

// KycRepository defines data access for KYC submissions and documents. A user
// has at most one submission (kyc_submissions.user_id is UNIQUE, design D7);
// documents may be uploaded before any submission exists, so CreateDocument
// never touches the submission row.
type KycRepository interface {
	Submit(ctx context.Context, sub *domain.KycSubmission) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.KycSubmission, error)
	CreateDocument(ctx context.Context, doc *domain.KycDocument) error
}

// SolanaTxRepository defines data access for on-chain Solana transaction records.
type SolanaTxRepository interface {
	Create(ctx context.Context, tx *domain.SolanaTx) error
	ListByWalletID(ctx context.Context, walletID uuid.UUID, txType string, after, before *time.Time, limit, offset int) ([]domain.SolanaTx, int, error)
	GetByTxHash(ctx context.Context, txHash string) (*domain.SolanaTx, error)
}
