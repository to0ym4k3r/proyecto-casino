package service

import (
	"context"
	"fmt"
	"math/rand"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

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

// StartGame initializes a new in-memory GameSession for the given user and game.
//
// The game must exist and have status GameStatusActive; otherwise an error is
// returned. The session starts with zero balance and "active" status. Note: the
// session is not persisted — it is returned for immediate use by the handler.
func (s *GameService) StartGame(ctx context.Context, userID, gameID uuid.UUID) (*domain.GameSession, error) {
	game, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		return nil, fmt.Errorf("game not found: %w", err)
	}

	if game.Status != domain.GameStatusActive {
		return nil, fmt.Errorf("game is not active")
	}

	session := &domain.GameSession{
		ID:        uuid.New(),
		UserID:    userID,
		GameID:    gameID,
		Status:    "active",
		Balance:   0,
		CreatedAt: game.CreatedAt,
	}

	return session, nil
}

// ResolveBet simulates a random game outcome and returns the payout amount.
//
// A random multiplier between 0 and 3 is generated. If the multiplier is >= 1.0
// the player wins and receives amount * multiplier; otherwise they lose (0, false).
// This is a stateless helper — it does not touch the wallet or database.
func ResolveBet(amount float64) (float64, bool) {
	multiplier := rand.Float64() * 3.0
	won := multiplier >= 1.0
	if !won {
		return 0, false
	}
	return amount * multiplier, true
}
