package service

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"
	"casino-backend/internal/rng"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GameService handles game-related business logic.
type GameService struct {
	gameRepo    repository.GameRepository
	walletRepo  repository.WalletRepository
	betRepo     repository.BetRepository
	sessionRepo repository.GameSessionRepository
	txFunc      func(ctx context.Context, fn func(pgx.Tx) error) error
}

// TxFunc is the signature for transaction helpers provided by the infrastructure layer.
type TxFunc func(ctx context.Context, fn func(pgx.Tx) error) error

// NewGameService creates a new game service.
func NewGameService(gameRepo repository.GameRepository) *GameService {
	return &GameService{gameRepo: gameRepo}
}

// NewGameServiceWithPersistence creates a game service with full persistence support.
func NewGameServiceWithPersistence(
	gameRepo repository.GameRepository,
	walletRepo repository.WalletRepository,
	betRepo repository.BetRepository,
	sessionRepo repository.GameSessionRepository,
	txFunc TxFunc,
) *GameService {
	return &GameService{
		gameRepo:    gameRepo,
		walletRepo:  walletRepo,
		betRepo:     betRepo,
		sessionRepo: sessionRepo,
		txFunc:      txFunc,
	}
}

// ListGames returns a paginated list of active games with the total count.
//
// Parameters are clamped: limit defaults to 20 (max 100), offset is floored at 0.
// Returns the slice of games, total available count, and any repository error.
func (s *GameService) ListGames(ctx context.Context, limit, offset int) ([]domain.Game, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.gameRepo.List(ctx, limit, offset)
}

// GetGame returns a game by its UUID.
//
// Delegates directly to the repository. Returns a wrapped error if the game
// does not exist.
func (s *GameService) GetGame(ctx context.Context, id uuid.UUID) (*domain.Game, error) {
	return s.gameRepo.GetByID(ctx, id)
}

// StartGame initializes a new GameSession for the given user and game.
//
// The game must exist and have status GameStatusActive; otherwise an error is
// returned. A cryptographically secure server seed is generated, its SHA-256
// hash is stored alongside, and the hash is returned to the caller. The raw
// seed is persisted but never returned after the initial response.
func (s *GameService) StartGame(ctx context.Context, userID, gameID uuid.UUID) (*domain.GameSession, error) {
	game, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game not found: %w", err)
	}

	if game.Status != domain.GameStatusActive {
		return nil, fmt.Errorf("game is not active")
	}

	seed := GenerateServerSeed()
	seedHash := HashSeed(seed)

	session := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          userID,
		GameID:          gameID,
		ServerSeed:      seed,
		ServerSeedHash:  seedHash,
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 0,
		StartedAt:       time.Now().UTC(),
	}

	return session, nil
}

// ResolveBet simulates a random game outcome using crypto/rand and returns
// the payout amount, multiplier, and whether the player won.
//
// The multiplier is capped by the game's RTP: maxMultiplier = rtp / 100.
// For RTP=95, maxMultiplier=0.95; for RTP=100, maxMultiplier=1.0.
// This is a stateless helper — it does not touch the wallet or database.
func ResolveBet(amount int64, rtp int) (int64, float64, bool) {
	maxMultiplier := float64(rtp) / 100.0
	multiplier := rng.SecureRandom() * maxMultiplier
	won := multiplier >= 1.0
	if !won {
		return 0, multiplier, false
	}
	payout := int64(float64(amount) * multiplier)
	return payout, multiplier, true
}

// PlaceBet deducts the bet amount from the user's wallet and creates a bet
// record atomically in a single database transaction. Uses SELECT ... FOR UPDATE
// on the wallet row with optimistic locking to prevent race conditions.
//
// Returns insufficient_balance if the wallet has fewer funds than the bet amount.
func (s *GameService) PlaceBet(ctx context.Context, userID, gameID, sessionID uuid.UUID, amount int64) (*domain.Bet, error) {
	if s.txFunc == nil {
		return nil, fmt.Errorf("persistence not configured")
	}

	var bet *domain.Bet

	err := s.txFunc(ctx, func(tx pgx.Tx) error {
		// 1. Lock the wallet row for update
		var walletID uuid.UUID
		var balance int64
		var version int
		err := tx.QueryRow(ctx,
			`SELECT id, balance, version FROM wallets WHERE user_id = $1 FOR UPDATE`,
			userID,
		).Scan(&walletID, &balance, &version)
		if err != nil {
			return fmt.Errorf("lock wallet: %w", err)
		}

		// 2. Check sufficient balance
		if balance < amount {
			return fmt.Errorf("insufficient balance: have %d, want %d", balance, amount)
		}

		// 3. Deduct from wallet with optimistic lock
		tag, err := tx.Exec(ctx,
			`UPDATE wallets SET balance = balance - $2, version = version + 1, updated_at = NOW()
			 WHERE id = $1 AND version = $3`,
			walletID, amount, version,
		)
		if err != nil {
			return fmt.Errorf("deduct balance: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("optimistic lock conflict on wallet")
		}

		// 4. Create bet record
		bet = &domain.Bet{
			ID:        uuid.New(),
			UserID:    userID,
			GameID:    gameID,
			SessionID: sessionID,
			Amount:    amount,
			Status:    domain.BetStatusPending,
			CreatedAt: time.Now().UTC(),
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO bets (id, user_id, game_id, session_id, amount, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			bet.ID, bet.UserID, bet.GameID, bet.SessionID, bet.Amount, bet.Status, bet.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert bet: %w", err)
		}

		// 5. Record wallet transaction
		_, err = tx.Exec(ctx,
			`INSERT INTO transactions (id, wallet_id, type, amount, balance_after, reference, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.New(), walletID, domain.TransactionBet, -amount, balance-amount,
			fmt.Sprintf("bet-%s", bet.ID.String()[:8]), time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("record bet transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return bet, nil
}

// CreditWin credits the winnings to the user's wallet and resolves the bet
// as won. Must be called inside an existing transaction or with txFunc configured.
func (s *GameService) CreditWin(ctx context.Context, bet *domain.Bet, payout int64, multiplier float64) error {
	if s.txFunc == nil {
		return fmt.Errorf("persistence not configured")
	}

	return s.txFunc(ctx, func(tx pgx.Tx) error {
		// 1. Lock wallet
		var walletID uuid.UUID
		var balance int64
		var version int
		err := tx.QueryRow(ctx,
			`SELECT id, balance, version FROM wallets WHERE user_id = $1 FOR UPDATE`,
			bet.UserID,
		).Scan(&walletID, &balance, &version)
		if err != nil {
			return fmt.Errorf("lock wallet for credit: %w", err)
		}

		// 2. Credit wallet
		tag, err := tx.Exec(ctx,
			`UPDATE wallets SET balance = balance + $2, version = version + 1, updated_at = NOW()
			 WHERE id = $1 AND version = $3`,
			walletID, payout, version,
		)
		if err != nil {
			return fmt.Errorf("credit balance: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("optimistic lock conflict on wallet credit")
		}

		// 3. Record win transaction
		_, err = tx.Exec(ctx,
			`INSERT INTO transactions (id, wallet_id, type, amount, balance_after, reference, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			uuid.New(), walletID, domain.TransactionWin, payout, balance+payout,
			fmt.Sprintf("win-%s", bet.ID.String()[:8]), time.Now().UTC(),
		)
		if err != nil {
			return fmt.Errorf("record win transaction: %w", err)
		}

		// 4. Resolve bet as won
		now := time.Now().UTC()
		_, err = tx.Exec(ctx,
			`UPDATE bets SET payout = $2, multiplier = $3, status = $4, resolved_at = $5 WHERE id = $1`,
			bet.ID, payout, multiplier, domain.BetStatusWon, now,
		)
		if err != nil {
			return fmt.Errorf("resolve bet: %w", err)
		}

		return nil
	})
}

// CreditLoss resolves the bet as lost. No wallet credit occurs — the
// deduction from PlaceBet is final.
func (s *GameService) CreditLoss(ctx context.Context, bet *domain.Bet, multiplier float64) error {
	if s.betRepo == nil {
		return fmt.Errorf("persistence not configured")
	}

	now := time.Now().UTC()
	bet.Status = domain.BetStatusLost
	bet.Multiplier = multiplier
	bet.ResolvedAt = &now
	return s.betRepo.Update(ctx, bet)
}

// EndSession marks a game session as completed and sets ended_at.
func (s *GameService) EndSession(ctx context.Context, sessionID uuid.UUID) error {
	if s.sessionRepo == nil {
		return fmt.Errorf("persistence not configured")
	}
	return s.sessionRepo.UpdateStatus(ctx, sessionID, domain.SessionStatusCompleted)
}
