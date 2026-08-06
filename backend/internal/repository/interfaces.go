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

// BetRepository defines data access operations for bets.
type BetRepository interface {
	Create(ctx context.Context, bet *domain.Bet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Bet, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Bet, error)
	Update(ctx context.Context, bet *domain.Bet) error
}
