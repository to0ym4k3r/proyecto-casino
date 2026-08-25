package postgres

import (
	"testing"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
)

// TestPostgresWalletRepoImplementsInterface proves the Postgres wallet
// repository satisfies the repository interface at compile time.
func TestPostgresWalletRepoImplementsInterface(t *testing.T) {
	var _ repository.WalletRepository = (*PostgresWalletRepo)(nil)
}

// TestWalletRepoBonusBalanceRoundTrip pins the wallets.bonus_balance wiring:
// the column must be written on Create and hydrated by both lookups, and a
// real-balance UpdateBalance must leave bonus_balance untouched. Without this
// contract the whole bonus engine credits money that silently disappears on
// the next wallet read.
//
// Requires TEST_DATABASE_URL; skipped otherwise (same policy as the rest of
// the integration suite).
func TestWalletRepoBonusBalanceRoundTrip(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresWalletRepo(pool)
	ctx := t.Context()

	wallet := &domain.Wallet{
		ID:           uuid.New(),
		UserID:       seedTestUser(t, pool),
		Balance:      10000,
		BonusBalance: 2500,
		Currency:     "USD",
		Version:      1,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := repo.Create(ctx, wallet); err != nil {
		t.Fatalf("Create with BonusBalance=%d: %v", wallet.BonusBalance, err)
	}

	got, err := repo.GetByID(ctx, wallet.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.BonusBalance != 2500 {
		t.Errorf("GetByID BonusBalance = %d, want 2500", got.BonusBalance)
	}

	gotByUser, err := repo.GetByUserID(ctx, wallet.UserID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if gotByUser.BonusBalance != 2500 {
		t.Errorf("GetByUserID BonusBalance = %d, want 2500", gotByUser.BonusBalance)
	}
	if gotByUser.Balance != 10000 {
		t.Errorf("GetByUserID Balance = %d, want 10000", gotByUser.Balance)
	}

	// A real-money balance update must never clobber the bonus column.
	if err := repo.UpdateBalance(ctx, wallet.ID, -4000, 1); err != nil {
		t.Fatalf("UpdateBalance: %v", err)
	}
	after, err := repo.GetByID(ctx, wallet.ID)
	if err != nil {
		t.Fatalf("GetByID after UpdateBalance: %v", err)
	}
	if after.Balance != 6000 {
		t.Errorf("Balance after update = %d, want 6000", after.Balance)
	}
	if after.BonusBalance != 2500 {
		t.Errorf("BonusBalance after balance-only update = %d, want 2500 (must be preserved)", after.BonusBalance)
	}
}

// TestRegistrationRepoPersistsBonusBalance proves a freshly registered wallet
// carries its BonusBalance through CreateUserWithWallet instead of relying on
// the DB default being invisible to the INSERT column list.
func TestRegistrationRepoPersistsBonusBalance(t *testing.T) {
	pool := testPool(t)
	regRepo := NewPostgresRegistrationRepo(pool)
	walletRepo := NewPostgresWalletRepo(pool)
	ctx := t.Context()

	now := time.Now().UTC()
	userID := uuid.New()
	in := domain.RegistrationInput{
		User: &domain.User{
			ID:           userID,
			Username:     "bonus_rt_" + uuid.New().String()[:8],
			Email:        "bonus_rt_" + uuid.New().String()[:8] + "@example.com",
			PasswordHash: "hash",
			IsActive:     true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Wallet: &domain.Wallet{
			ID:           uuid.New(),
			UserID:       userID,
			Balance:      0,
			BonusBalance: 750,
			Currency:     "USD",
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Verification: &domain.EmailVerification{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "tok_" + uuid.New().String(),
			ExpiresAt: now.Add(time.Hour),
			CreatedAt: now,
		},
		RefreshToken: &domain.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: "rt_" + uuid.New().String(),
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
		},
	}

	if err := regRepo.CreateUserWithWallet(ctx, in); err != nil {
		t.Fatalf("CreateUserWithWallet: %v", err)
	}

	got, err := walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID after registration: %v", err)
	}
	if got.BonusBalance != 750 {
		t.Errorf("registered wallet BonusBalance = %d, want 750", got.BonusBalance)
	}
}
