package domain

import (
	"time"

	"github.com/google/uuid"
)

// GameStatus represents the state of a game.
type GameStatus string

const (
	GameStatusActive  GameStatus = "active"
	GameStatusInactive GameStatus = "inactive"
	GameStatusMaintenance GameStatus = "maintenance"
)

// Game represents a casino game available to players.
type Game struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name" validate:"required,max=200"`
	Description string     `json:"description" validate:"max=1000"`
	Category    string     `json:"category" validate:"required,max=100"`
	MinBet      float64    `json:"min_bet" validate:"required,gt=0"`
	MaxBet      float64    `json:"max_bet" validate:"required,gt=0"`
	RTP         float64    `json:"rtp" validate:"required,gte=0,lte=100"`
	Status      GameStatus `json:"status"`
	CoverImage  string     `json:"cover_image" validate:"max=500"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// GameSession represents an active game session for a user.
type GameSession struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	GameID    uuid.UUID  `json:"game_id"`
	Status    string     `json:"status"`
	Balance   float64    `json:"balance"`
	CreatedAt time.Time  `json:"created_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}
