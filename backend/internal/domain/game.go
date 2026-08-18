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
// MinBet and MaxBet are in integer cents (e.g. 100 = $1.00).
// RTP is an integer percentage (e.g. 96 = 96%).
type Game struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name" validate:"required,max=200"`
	Description string     `json:"description" validate:"max=1000"`
	Category    string     `json:"category" validate:"required,max=100"`
	MinBet      int64      `json:"min_bet" validate:"required,gt=0"`
	MaxBet      int64      `json:"max_bet" validate:"required,gt=0"`
	RTP         int        `json:"rtp" validate:"required,gte=0,lte=100"`
	Status      GameStatus `json:"status"`
	CoverImage  string     `json:"cover_image" validate:"max=500"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// GameSessionStatus represents the lifecycle state of a game session.
type GameSessionStatus string

const (
	SessionStatusActive    GameSessionStatus = "active"
	SessionStatusCompleted GameSessionStatus = "completed"
	SessionStatusAbandoned GameSessionStatus = "abandoned"
)

// Valid returns true when the status is one of the defined constants.
func (s GameSessionStatus) Valid() bool {
	switch s {
	case SessionStatusActive, SessionStatusCompleted, SessionStatusAbandoned:
		return true
	}
	return false
}

// GameSession represents an active game session for a user.
// BalanceSnapshot is in integer cents.
type GameSession struct {
	ID             uuid.UUID         `json:"id"`
	UserID         uuid.UUID         `json:"user_id"`
	GameID         uuid.UUID         `json:"game_id"`
	ServerSeed     string            `json:"server_seed"`
	ServerSeedHash string            `json:"server_seed_hash"`
	Status         GameSessionStatus `json:"status"`
	BalanceSnapshot int64            `json:"balance_snapshot"`
	StartedAt      time.Time         `json:"started_at"`
	EndedAt        *time.Time        `json:"ended_at,omitempty"`
}
