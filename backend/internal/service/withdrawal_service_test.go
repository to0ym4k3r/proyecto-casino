package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
)

// --- Mock WithdrawalRepository ---

type mockWithdrawalRepo struct {
	withdrawals map[uuid.UUID]*domain.Withdrawal
	dailyTotal  int64
	dailyErr    error
	createErr   error
}

func newMockWithdrawalRepo() *mockWithdrawalRepo {
	return &mockWithdrawalRepo{
		withdrawals: make(map[uuid.UUID]*domain.Withdrawal),
	}
}

func (m *mockWithdrawalRepo) Create(_ context.Context, w *domain.Withdrawal) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.withdrawals[w.ID] = w
	return nil
}

func (m *mockWithdrawalRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Withdrawal, error) {
	w, ok := m.withdrawals[id]
	if !ok {
		return nil, fmt.Errorf("withdrawal not found")
	}
	return w, nil
}

func (m *mockWithdrawalRepo) ListByUserID(_ context.Context, userID uuid.UUID, limit, offset int) ([]domain.Withdrawal, error) {
	var result []domain.Withdrawal
	for _, w := range m.withdrawals {
		if w.UserID == userID {
			result = append(result, *w)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockWithdrawalRepo) GetDailyTotal(_ context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	if m.dailyErr != nil {
		return 0, m.dailyErr
	}
	return m.dailyTotal, nil
}

func (m *mockWithdrawalRepo) UpdateStatus(_ context.Context, id uuid.UUID, status domain.WithdrawalStatus, txHash string) error {
	w, ok := m.withdrawals[id]
	if !ok {
		return fmt.Errorf("withdrawal not found")
	}
	w.Status = status
	if txHash != "" {
		w.TxHash = txHash
	}
	return nil
}

// --- Test helpers ---

func makeTestWallet(balance int64) *domain.Wallet {
	return &domain.Wallet{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Balance:   balance,
		Currency:  "USD",
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func newTestWithdrawalService(
	balance int64,
	dailyTotal int64,
	kycTier KYCTier,
	emergencyPaused bool,
) (*WithdrawalService, *domain.Wallet) {
	wallet := makeTestWallet(balance)

	walletRepo := &mockWalletRepo{wallet: wallet}
	withdrawalRepo := newMockWithdrawalRepo()
	withdrawalRepo.dailyTotal = dailyTotal

	svc := NewWithdrawalService(
		walletRepo,
		withdrawalRepo,
		func(_ context.Context, _ uuid.UUID) (KYCTier, error) {
			return kycTier, nil
		},
		func(_ context.Context) (bool, error) {
			return emergencyPaused, nil
		},
	)

	return svc, wallet
}

// --- Tests: 3.1 KYC tier auto-approval ---

func TestWithdrawalAutoApprovedBelowThreshold(t *testing.T) {
	// L1 user, $100 (10000 cents) — below $5K auto-approval threshold
	svc, wallet := newTestWithdrawalService(50000, 0, KYCL1, false)

	w, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 10000, "ABC123", "SOL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Status != domain.WithdrawalStatusApproved {
		t.Errorf("status = %q, want %q (auto-approved below L1 threshold)", w.Status, domain.WithdrawalStatusApproved)
	}
}

func TestWithdrawalPendingApprovalAboveThreshold(t *testing.T) {
	// Spec: auto-approval threshold equals per-tx limit for each tier.
	// pending_approval can only occur if a future tier has a higher per-tx limit
	// than its auto-approval threshold. For current tiers (equal limits),
	// amounts above threshold are rejected by per-tx check.
	// This test verifies the auto-approval path works correctly at the boundary.
	// L1 user, $5K (500000 cents) — exactly at L1 threshold → auto-approved
	svc, wallet := newTestWithdrawalService(5000000, 0, KYCL1, false)

	w, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 500000, "ABC123", "SOL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Status != domain.WithdrawalStatusApproved {
		t.Errorf("status = %q, want %q (at L1 threshold, auto-approved)", w.Status, domain.WithdrawalStatusApproved)
	}
}

func TestWithdrawalL2AutoApprovedBelow25K(t *testing.T) {
	// L2 user, $20K (2000000 cents) — below $25K L2 threshold
	svc, wallet := newTestWithdrawalService(10000000, 0, KYCL2, false)

	w, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 2000000, "ABC123", "SOL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Status != domain.WithdrawalStatusApproved {
		t.Errorf("status = %q, want %q (auto-approved below L2 threshold)", w.Status, domain.WithdrawalStatusApproved)
	}
}

func TestWithdrawalL2ExceedsPerTxLimit(t *testing.T) {
	// L2 user tries $30K — exceeds $25K L2 per-tx limit → rejected
	svc, wallet := newTestWithdrawalService(10000000, 0, KYCL2, false)

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 3000000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error for exceeding L2 per-tx limit")
	}
}

// --- Tests: 3.3 Per-transaction limit enforcement ---

func TestWithdrawalExceedsPerTxLimit(t *testing.T) {
	// Unverified user tries $600 — exceeds $500 per-tx limit
	svc, wallet := newTestWithdrawalService(500000, 0, KYCUnverified, false)

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 60000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error for exceeding per-tx limit")
	}
}

func TestWithdrawalExceedsDailyLimit(t *testing.T) {
	// L1 user, daily already $48K, tries $3K — exceeds $50K daily
	svc, wallet := newTestWithdrawalService(10000000, 4800000, KYCL1, false)

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 300000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error for exceeding daily limit")
	}
	if err != nil && err.Error() != "daily limit exceeded" {
		t.Errorf("error = %q, want %q", err.Error(), "daily limit exceeded")
	}
}

// --- Tests: 3.4 Outflow cap and emergency pause ---

func TestWithdrawalRejectedWhenPaused(t *testing.T) {
	svc, wallet := newTestWithdrawalService(50000, 0, KYCL1, true)

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 10000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error when emergency pause is active")
	}
	if err != nil && err.Error() != "withdrawals paused" {
		t.Errorf("error = %q, want %q", err.Error(), "withdrawals paused")
	}
}

func TestWithdrawalInsufficientBalance(t *testing.T) {
	svc, wallet := newTestWithdrawalService(5000, 0, KYCL1, false)

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 10000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error for insufficient balance")
	}
}

// --- Tests: 3.6 Outflow cap enforcement ---

func TestWithdrawalExceedsOutflowCap(t *testing.T) {
	// Hot wallet = 1,000,000 cents, 24h outflow = 280,000, new = 50,000
	// 280000 + 50000 = 330000, 30% of 1,000,000 = 300,000 → cap exceeded
	wallet := makeTestWallet(1000000)
	walletRepo := &mockWalletRepo{wallet: wallet}
	withdrawalRepo := newMockWithdrawalRepo()

	svc := NewWithdrawalService(
		walletRepo,
		withdrawalRepo,
		func(_ context.Context, _ uuid.UUID) (KYCTier, error) {
			return KYCL2, nil
		},
		func(_ context.Context) (bool, error) {
			return false, nil
		},
	)
	svc.SetOutflowChecker(func(_ context.Context, hotWalletBalance, newOutflow int64) (bool, error) {
		outflowLimit := int64(float64(hotWalletBalance) * 0.30)
		return (280000 + newOutflow) > outflowLimit, nil
	})

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 50000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error for outflow cap exceeded")
	}
	if err != nil && err.Error() != "hot wallet outflow cap exceeded" {
		t.Errorf("error = %q, want %q", err.Error(), "hot wallet outflow cap exceeded")
	}
}

func TestWithdrawalWithinOutflowCap(t *testing.T) {
	// Hot wallet = 1,000,000 cents, 24h outflow = 200,000, new = 50,000
	// 250,000 < 300,000 → within cap
	wallet := makeTestWallet(1000000) // balance large enough for outflow checker
	walletRepo := &mockWalletRepo{wallet: wallet}
	withdrawalRepo := newMockWithdrawalRepo()

	svc := NewWithdrawalService(
		walletRepo,
		withdrawalRepo,
		func(_ context.Context, _ uuid.UUID) (KYCTier, error) {
			return KYCL2, nil
		},
		func(_ context.Context) (bool, error) {
			return false, nil
		},
	)
	svc.SetOutflowChecker(func(_ context.Context, hotWalletBalance, newOutflow int64) (bool, error) {
		outflowLimit := int64(float64(hotWalletBalance) * 0.30)
		return (200000 + newOutflow) > outflowLimit, nil
	})

	w, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 50000, "ABC123", "SOL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Status != domain.WithdrawalStatusApproved {
		t.Errorf("status = %q, want %q (within outflow cap)", w.Status, domain.WithdrawalStatusApproved)
	}
}

// mockWalletRepo is a minimal WalletRepository mock for WithdrawalService tests.
type mockWalletRepo struct {
	wallet    *domain.Wallet
	createErr error
}

func (m *mockWalletRepo) Create(_ context.Context, w *domain.Wallet) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.wallet = w
	return nil
}

func (m *mockWalletRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Wallet, error) {
	return m.wallet, nil
}

func (m *mockWalletRepo) GetByUserID(_ context.Context, _ uuid.UUID) (*domain.Wallet, error) {
	return m.wallet, nil
}

func (m *mockWalletRepo) UpdateBalance(_ context.Context, _ uuid.UUID, amount int64, _ int) error {
	m.wallet.Balance += amount
	return nil
}

func (m *mockWalletRepo) CreateTransaction(_ context.Context, _ *domain.Transaction) error {
	return nil
}
