package domain

import (
	"time"

	"github.com/google/uuid"
)

// BetStatus represents the current state of a bet.
type BetStatus string

const (
	BetStatusPending BetStatus = "pending"
	BetStatusWon     BetStatus = "won"
	BetStatusLost    BetStatus = "lost"
	BetStatusCanceled BetStatus = "canceled"
)

// Bet represents a wager placed by a user on a game.
type Bet struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	GameID    uuid.UUID  `json:"game_id"`
	Amount    float64    `json:"amount" validate:"required,gt=0"`
	Result    *float64   `json:"result,omitempty"`
	Status    BetStatus  `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// PlaceBetRequest is the payload for placing a bet.
type PlaceBetRequest struct {
	GameID uuid.UUID `json:"game_id" validate:"required"`
	Amount float64   `json:"amount" validate:"required,gt=0"`
}
