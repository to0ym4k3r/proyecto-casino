package postgres

import (
	"context"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
)

func TestGameSessionRepoCreateAndGetByID(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameSessionRepo(pool)
	ctx := context.Background()

	userID := seedTestUser(t, pool)
	gameID := seedTestGame(t, pool)

	session := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          userID,
		GameID:          gameID,
		ServerSeed:      "testseed123",
		ServerSeedHash:  "testhash456",
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 0,
		StartedAt:       time.Now().UTC(),
	}

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("ID = %v, want %v", got.ID, session.ID)
	}
	if got.ServerSeed != "testseed123" {
		t.Errorf("ServerSeed = %s, want testseed123", got.ServerSeed)
	}
	if got.Status != domain.SessionStatusActive {
		t.Errorf("Status = %s, want %s", got.Status, domain.SessionStatusActive)
	}
}

func TestGameSessionRepoUpdateStatus(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameSessionRepo(pool)
	ctx := context.Background()

	userID := seedTestUser(t, pool)
	gameID := seedTestGame(t, pool)

	session := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          userID,
		GameID:          gameID,
		ServerSeed:      "seed",
		ServerSeedHash:  "hash",
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 0,
		StartedAt:       time.Now().UTC(),
	}
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateStatus(ctx, session.ID, domain.SessionStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, err := repo.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != domain.SessionStatusCompleted {
		t.Errorf("Status = %s, want %s", got.Status, domain.SessionStatusCompleted)
	}
	if got.EndedAt == nil {
		t.Error("EndedAt is nil, want a timestamp")
	}
}

func TestGameSessionRepoFindAbandoned(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameSessionRepo(pool)
	ctx := context.Background()

	userID := seedTestUser(t, pool)
	gameID := seedTestGame(t, pool)

	// Create an old session (25 hours ago)
	oldSession := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          userID,
		GameID:          gameID,
		ServerSeed:      "oldseed",
		ServerSeedHash:  "oldhash",
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 0,
		StartedAt:       time.Now().UTC().Add(-25 * time.Hour),
	}
	if err := repo.Create(ctx, oldSession); err != nil {
		t.Fatalf("Create old session: %v", err)
	}

	// Create a recent session (1 hour ago)
	recentSession := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          userID,
		GameID:          gameID,
		ServerSeed:      "recentseed",
		ServerSeedHash:  "recenthash",
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 0,
		StartedAt:       time.Now().UTC().Add(-1 * time.Hour),
	}
	if err := repo.Create(ctx, recentSession); err != nil {
		t.Fatalf("Create recent session: %v", err)
	}

	abandoned, err := repo.FindAbandoned(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("FindAbandoned: %v", err)
	}

	// Should find the old session but not the recent one
	found := false
	for _, s := range abandoned {
		if s.ID == oldSession.ID {
			found = true
		}
		if s.ID == recentSession.ID {
			t.Error("recent session should not be abandoned")
		}
	}
	if !found {
		t.Error("old session not found in abandoned list")
	}
}
