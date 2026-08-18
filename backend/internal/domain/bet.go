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
// Amount and Payout are in integer cents (e.g. 500 = $5.00).
type Bet struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	GameID     uuid.UUID  `json:"game_id"`
	SessionID  uuid.UUID  `json:"session_id"`
	Amount     int64      `json:"amount" validate:"required,gt=0"`
	Payout     int64      `json:"payout"`
	Multiplier float64    `json:"multiplier"`
	Status     BetStatus  `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// PlaceBetRequest is the payload for placing a bet.
// Amount is in integer cents (e.g. 500 = $5.00).
type PlaceBetRequest struct {
	GameID uuid.UUID `json:"game_id" validate:"required"`
	Amount int64     `json:"amount" validate:"required,gt=0"`
}
