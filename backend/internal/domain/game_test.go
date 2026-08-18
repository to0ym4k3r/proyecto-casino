package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// GameSession — struct fields, status enum, server seed contract
// ---------------------------------------------------------------------------

// TestGameSessionHasServerSeedFields proves GameSession carries the two seed
// columns that migration 005 adds: server_seed and server_seed_hash.
func TestGameSessionHasServerSeedFields(t *testing.T) {
	s := GameSession{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		GameID:         uuid.New(),
		ServerSeed:     "a1b2c3d4e5f6",
		ServerSeedHash: "sha256hash",
		Status:         SessionStatusActive,
	}
	if s.ServerSeed != "a1b2c3d4e5f6" {
		t.Errorf("ServerSeed = %q, want %q", s.ServerSeed, "a1b2c3d4e5f6")
	}
	if s.ServerSeedHash != "sha256hash" {
		t.Errorf("ServerSeedHash = %q, want %q", s.ServerSeedHash, "sha256hash")
	}
}

// TestGameSessionStatusEnum proves the three allowed lifecycle states.
func TestGameSessionStatusEnum(t *testing.T) {
	tests := []struct {
		status GameSessionStatus
		want   string
	}{
		{SessionStatusActive, "active"},
		{SessionStatusCompleted, "completed"},
		{SessionStatusAbandoned, "abandoned"},
	}
	for _, tt := range tests {
		if got := string(tt.status); got != tt.want {
			t.Errorf("SessionStatus %s = %q, want %q", tt.status, got, tt.want)
		}
	}
	if GameSessionStatus("").Valid() {
		t.Error("empty status must not be valid")
	}
	if GameSessionStatus("garbage").Valid() {
		t.Error("unknown status must not be valid")
	}
}

// TestGameSessionJSONContract proves the JSON wire format matches the API
// contract: snake_case keys, server_seed and server_seed_hash present.
func TestGameSessionJSONContract(t *testing.T) {
	s := GameSession{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		GameID:         uuid.New(),
		ServerSeed:     "seed123",
		ServerSeedHash: "hash456",
		Status:         SessionStatusActive,
		BalanceSnapshot: 5000,
		StartedAt:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	for _, key := range []string{`"server_seed"`, `"server_seed_hash"`, `"status":"active"`, `"balance_snapshot":5000`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled session missing %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// Bet — int64 cents, session_id, multiplier, payout
// ---------------------------------------------------------------------------

// TestBetUsesInt64Cents proves Amount and Payout are int64 (integer cents),
// matching the BIGINT columns in the database.
func TestBetUsesInt64Cents(t *testing.T) {
	b := Bet{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		GameID:    uuid.New(),
		SessionID: uuid.New(),
		Amount:    5000, // $50.00
		Payout:    10000,
		Multiplier: 2.0,
		Status:    BetStatusWon,
		CreatedAt: time.Now(),
	}
	if b.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", b.Amount)
	}
	if b.Payout != 10000 {
		t.Errorf("Payout = %d, want 10000", b.Payout)
	}
	if b.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f, want 2.0", b.Multiplier)
	}
}

// TestBetJSONContract proves the JSON wire format matches the API contract.
func TestBetJSONContract(t *testing.T) {
	b := Bet{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		GameID:    uuid.New(),
		SessionID: uuid.New(),
		Amount:    500,
		Payout:    1000,
		Multiplier: 2.0,
		Status:    BetStatusWon,
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal bet: %v", err)
	}
	for _, key := range []string{`"amount":500`, `"payout":1000`, `"multiplier":2`, `"session_id"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled bet missing %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// Game — int64 cents for min/max bet, int for RTP
// ---------------------------------------------------------------------------

// TestGameUsesInt64CentsForBetLimits proves MinBet and MaxBet are int64
// (integer cents), matching the BIGINT columns after migration 005.
func TestGameUsesInt64CentsForBetLimits(t *testing.T) {
	g := Game{
		ID:      uuid.New(),
		Name:    "Slots",
		MinBet:  100,  // $1.00
		MaxBet:  50000, // $500.00
		RTP:     96,
	}
	if g.MinBet != 100 {
		t.Errorf("MinBet = %d, want 100", g.MinBet)
	}
	if g.MaxBet != 50000 {
		t.Errorf("MaxBet = %d, want 50000", g.MaxBet)
	}
	if g.RTP != 96 {
		t.Errorf("RTP = %d, want 96", g.RTP)
	}
}
