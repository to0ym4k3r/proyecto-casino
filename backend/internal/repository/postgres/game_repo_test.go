package postgres

import (
	"context"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
)

func TestGameRepoCreateAndGetByID(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameRepo(pool)
	ctx := context.Background()

	game := &domain.Game{
		ID:          uuid.New(),
		Name:        "Test Slots",
		Description: "A test slot game",
		Category:    "slots",
		MinBet:      100,
		MaxBet:      50000,
		RTP:         96,
		Status:      domain.GameStatusActive,
		CoverImage:  "https://example.com/slots.png",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := repo.Create(ctx, game); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Test Slots" {
		t.Errorf("Name = %s, want Test Slots", got.Name)
	}
	if got.Category != "slots" {
		t.Errorf("Category = %s, want slots", got.Category)
	}
	if got.RTP != 96 {
		t.Errorf("RTP = %d, want 96", got.RTP)
	}
	if got.MinBet != 100 {
		t.Errorf("MinBet = %d, want 100", got.MinBet)
	}
	if got.MaxBet != 50000 {
		t.Errorf("MaxBet = %d, want 50000", got.MaxBet)
	}
}

func TestGameRepoList(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameRepo(pool)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		game := &domain.Game{
			ID:        uuid.New(),
			Name:      "Game " + time.Now().Format("150405.000"),
			Category:  "slots",
			MinBet:    100,
			MaxBet:    50000,
			RTP:       95,
			Status:    domain.GameStatusActive,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := repo.Create(ctx, game); err != nil {
			t.Fatalf("Create game %d: %v", i, err)
		}
	}

	games, total, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 3 {
		t.Errorf("total = %d, want >= 3", total)
	}
	if len(games) < 3 {
		t.Errorf("len(games) = %d, want >= 3", len(games))
	}
}

func TestGameRepoUpdate(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameRepo(pool)
	ctx := context.Background()

	game := &domain.Game{
		ID:        uuid.New(),
		Name:      "Original Name",
		Category:  "poker",
		MinBet:    500,
		MaxBet:    100000,
		RTP:       97,
		Status:    domain.GameStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, game); err != nil {
		t.Fatalf("Create: %v", err)
	}

	game.Name = "Updated Name"
	game.RTP = 99
	game.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, game); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Updated Name" {
		t.Errorf("Name = %s, want Updated Name", got.Name)
	}
	if got.RTP != 99 {
		t.Errorf("RTP = %d, want 99", got.RTP)
	}
}

func TestGameRepoDelete(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameRepo(pool)
	ctx := context.Background()

	game := &domain.Game{
		ID:        uuid.New(),
		Name:      "Doomed Game",
		Category:  "roulette",
		MinBet:    100,
		MaxBet:    10000,
		RTP:       95,
		Status:    domain.GameStatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, game); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, game.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByID(ctx, game.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestGameRepoDeleteNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresGameRepo(pool)
	ctx := context.Background()

	err := repo.Delete(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for nonexistent game, got nil")
	}
}
