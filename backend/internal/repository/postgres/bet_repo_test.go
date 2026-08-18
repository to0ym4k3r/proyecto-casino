package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func seedTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, first_name, last_name)
		VALUES ($1, 'testuser_'+$1::text, 'test_'+$1::text+'@example.com', 'hash', 'Test', 'User')`,
		userID)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return userID
}

func seedTestGame(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	gameID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO games (id, name, description, category, min_bet, max_bet, rtp, status)
		VALUES ($1, 'Test Game', 'A test game', 'slots', 100, 50000, 95, 'active')`,
		gameID)
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	return gameID
}

func TestBetRepoCreateAndGetByID(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresBetRepo(pool)
	ctx := context.Background()

	userID := seedTestUser(t, pool)
	gameID := seedTestGame(t, pool)

	bet := &domain.Bet{
		ID:        uuid.New(),
		UserID:    userID,
		GameID:    gameID,
		Amount:    500,
		Status:    domain.BetStatusPending,
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, bet); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, bet.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != bet.ID {
		t.Errorf("ID = %v, want %v", got.ID, bet.ID)
	}
	if got.Amount != 500 {
		t.Errorf("Amount = %d, want 500", got.Amount)
	}
	if got.Status != domain.BetStatusPending {
		t.Errorf("Status = %s, want %s", got.Status, domain.BetStatusPending)
	}
}

func TestBetRepoGetByUserIDPagination(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresBetRepo(pool)
	ctx := context.Background()

	userID := seedTestUser(t, pool)
	gameID := seedTestGame(t, pool)

	// Create 5 bets
	for i := 0; i < 5; i++ {
		bet := &domain.Bet{
			ID:        uuid.New(),
			UserID:    userID,
			GameID:    gameID,
			Amount:    int64((i + 1) * 100),
			Status:    domain.BetStatusPending,
			CreatedAt: time.Now().UTC(),
		}
		if err := repo.Create(ctx, bet); err != nil {
			t.Fatalf("Create bet %d: %v", i, err)
		}
	}

	// Get page 1: limit=2, offset=0
	page1, err := repo.GetByUserID(ctx, userID, 2, 0)
	if err != nil {
		t.Fatalf("GetByUserID page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("page 1 length = %d, want 2", len(page1))
	}

	// Get page 2: limit=2, offset=2
	page2, err := repo.GetByUserID(ctx, userID, 2, 2)
	if err != nil {
		t.Fatalf("GetByUserID page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page 2 length = %d, want 2", len(page2))
	}

	// Get page 3: limit=2, offset=4
	page3, err := repo.GetByUserID(ctx, userID, 2, 4)
	if err != nil {
		t.Fatalf("GetByUserID page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("page 3 length = %d, want 1", len(page3))
	}
}
