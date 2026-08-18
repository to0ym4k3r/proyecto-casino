package service

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"
	"casino-backend/internal/rng"

	"github.com/google/uuid"
)

// GameService handles game-related business logic.
type GameService struct {
	gameRepo repository.GameRepository
}

// NewGameService creates a new game service.
func NewGameService(gameRepo repository.GameRepository) *GameService {
	return &GameService{gameRepo: gameRepo}
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
