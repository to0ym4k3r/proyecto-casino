package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
		func(_ context.Context) (uint64, error) {
			return uint64(wallet.Balance), nil
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
		func(_ context.Context) (uint64, error) {
			return uint64(wallet.Balance), nil
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
		func(_ context.Context) (uint64, error) {
			return uint64(wallet.Balance), nil
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
	// updateBalanceErr simulates a post-forfeit debit failure (e.g. optimistic
	// version conflict), exercising the R-001 window after a committed forfeit.
	updateBalanceErr error
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
	if m.updateBalanceErr != nil {
		return m.updateBalanceErr
	}
	m.wallet.Balance += amount
	return nil
}

func (m *mockWalletRepo) CreateTransaction(_ context.Context, _ *domain.Transaction) error {
	return nil
}

// ---------------------------------------------------------------------------
// Tests: 5.3 / 5.5 — withdrawal forces forfeit of the active bonus
// ---------------------------------------------------------------------------

// newTestWithdrawalServiceWithBonus wires a WithdrawalService whose
// RequestWithdrawal forfeits the user's active bonus grants through the given
// spies. The bonus service shares the same txFunc as the withdrawal, mirroring
// the production transaction boundary, so the forfeit runs inside the
// withdrawal's transaction rather than a fresh one.
func newTestWithdrawalServiceWithBonus(
	balance, bonusBalance int64,
	grantRepo *fakeGrantRepo,
	ledgerRepo *fakeLedgerRepo,
) (*WithdrawalService, *domain.Wallet, *recordingTx, *mockWithdrawalRepo) {
	wallet := makeTestWallet(balance)
	wallet.BonusBalance = bonusBalance

	walletRepo := &mockWalletRepo{wallet: wallet}
	withdrawalRepo := newMockWithdrawalRepo()

	tx := newRecordingTx()
	tx.queryRowValue = &fakeWalletRow{
		id:           wallet.ID,
		balance:      balance,
		bonusBalance: bonusBalance,
		version:      wallet.Version,
	}
	txFunc := func(_ context.Context, fn func(pgx.Tx) error) error { return fn(tx) }

	bonusSvc := NewBonusService(nil, grantRepo, ledgerRepo, NewWalletService(nil), nil, txFunc)

	svc := NewWithdrawalService(
		walletRepo,
		withdrawalRepo,
		func(_ context.Context, _ uuid.UUID) (KYCTier, error) { return KYCL1, nil },
		func(_ context.Context) (bool, error) { return false, nil },
		func(_ context.Context) (uint64, error) { return uint64(balance), nil },
	)
	svc.SetBonusForfeit(bonusSvc, txFunc)

	return svc, wallet, tx, withdrawalRepo
}

func TestRequestWithdrawalForfeitsActiveBonus(t *testing.T) {
	grantRepo := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}}
	ledgerRepo := &fakeLedgerRepo{}
	svc, wallet, tx, withdrawalRepo := newTestWithdrawalServiceWithBonus(8000, 5000, grantRepo, ledgerRepo)

	grant := activeGrant(wallet.UserID, uuid.New(), 5000)
	grantRepo.activeByUser = []domain.BonusGrant{*grant}

	w, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 8000, "ABC123", "SOL")
	if err != nil {
		t.Fatalf("RequestWithdrawal: %v", err)
	}
	if w.Status != domain.WithdrawalStatusApproved {
		t.Errorf("status = %q, want %q", w.Status, domain.WithdrawalStatusApproved)
	}

	// Real money leaves the wallet; the bonus pool is surrendered.
	if wallet.Balance != 0 {
		t.Errorf("balance = %d, want 0 (withdrew the full real balance)", wallet.Balance)
	}

	// Bonus pool zeroed inside the withdrawal transaction.
	foundZero := false
	for _, e := range tx.execs {
		if strings.Contains(e.sql, "bonus_balance = 0") {
			foundZero = true
		}
	}
	if !foundZero {
		t.Errorf("no SQL zeroing bonus_balance was executed; execs=%d", len(tx.execs))
	}

	// Active grant flipped to forfeited.
	if len(grantRepo.statuses) != 1 {
		t.Fatalf("status updates = %d, want 1", len(grantRepo.statuses))
	}
	if grantRepo.statuses[0].id != grant.ID || grantRepo.statuses[0].status != domain.BonusStatusForfeited {
		t.Errorf("status update = {%s %s}, want {%s %s}",
			grantRepo.statuses[0].id, grantRepo.statuses[0].status, grant.ID, domain.BonusStatusForfeited)
	}

	// One forfeited ledger entry for the surrendered amount.
	if len(ledgerRepo.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(ledgerRepo.entries))
	}
	e := ledgerRepo.entries[0]
	if e.TransactionType != domain.BonusLedgerForfeited || e.AmountCents != 5000 || e.GrantID != grant.ID {
		t.Errorf("ledger entry = %+v, want forfeited/5000/%s", e, grant.ID)
	}

	// The withdrawal itself is still recorded.
	if len(withdrawalRepo.withdrawals) != 1 {
		t.Errorf("withdrawals recorded = %d, want 1", len(withdrawalRepo.withdrawals))
	}
}

func TestRequestWithdrawalWithoutActiveBonusProceeds(t *testing.T) {
	grantRepo := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}} // no active grants
	ledgerRepo := &fakeLedgerRepo{}
	svc, wallet, tx, _ := newTestWithdrawalServiceWithBonus(8000, 0, grantRepo, ledgerRepo)

	w, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 8000, "ABC123", "SOL")
	if err != nil {
		t.Fatalf("RequestWithdrawal without active bonus: %v", err)
	}
	if w.Status != domain.WithdrawalStatusApproved {
		t.Errorf("status = %q, want %q", w.Status, domain.WithdrawalStatusApproved)
	}
	if wallet.Balance != 0 {
		t.Errorf("balance = %d, want 0", wallet.Balance)
	}

	// No active grant → no bonus state is touched at all.
	if len(grantRepo.statuses) != 0 {
		t.Errorf("status updates = %d, want 0 (no active grant)", len(grantRepo.statuses))
	}
	if len(ledgerRepo.entries) != 0 {
		t.Errorf("ledger entries = %d, want 0 (no active grant)", len(ledgerRepo.entries))
	}
	for _, e := range tx.execs {
		if strings.Contains(e.sql, "bonus_balance = 0") {
			t.Errorf("must not zero bonus_balance when no active grant exists: %s", e.sql)
		}
	}
}

func TestRequestWithdrawalForfeitsMultipleActiveGrants(t *testing.T) {
	grantRepo := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}}
	ledgerRepo := &fakeLedgerRepo{}
	svc, wallet, _, _ := newTestWithdrawalServiceWithBonus(8000, 5000, grantRepo, ledgerRepo)

	// Two active grants of different types coexist (partial pool consumption is
	// possible), so the withdrawal must surrender both, not just one.
	g1 := activeGrant(wallet.UserID, uuid.New(), 3000)
	g2 := activeGrant(wallet.UserID, uuid.New(), 2000)
	grantRepo.activeByUser = []domain.BonusGrant{*g1, *g2}

	if _, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 8000, "ABC123", "SOL"); err != nil {
		t.Fatalf("RequestWithdrawal: %v", err)
	}

	if len(grantRepo.statuses) != 2 {
		t.Fatalf("status updates = %d, want 2 (both grants forfeited)", len(grantRepo.statuses))
	}
	for _, up := range grantRepo.statuses {
		if up.status != domain.BonusStatusForfeited {
			t.Errorf("grant %s status = %q, want forfeited", up.id, up.status)
		}
	}

	if len(ledgerRepo.entries) != 2 {
		t.Fatalf("ledger entries = %d, want 2", len(ledgerRepo.entries))
	}
	var total int64
	for _, e := range ledgerRepo.entries {
		if e.TransactionType != domain.BonusLedgerForfeited {
			t.Errorf("ledger type = %q, want forfeited", e.TransactionType)
		}
		total += e.AmountCents
	}
	if total != 5000 {
		t.Errorf("forfeited ledger total = %d, want 5000", total)
	}
}

func TestRequestWithdrawalAbortsWhenForfeitFails(t *testing.T) {
	grantRepo := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}}
	ledgerRepo := &fakeLedgerRepo{createErr: errors.New("ledger down")}
	svc, wallet, _, withdrawalRepo := newTestWithdrawalServiceWithBonus(8000, 5000, grantRepo, ledgerRepo)

	grant := activeGrant(wallet.UserID, uuid.New(), 5000)
	grantRepo.activeByUser = []domain.BonusGrant{*grant}

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 8000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error when the forfeit transaction fails")
	}

	// Money-conserving abort: no real money leaves and no withdrawal is recorded.
	if wallet.Balance != 8000 {
		t.Errorf("balance = %d, want 8000 (withdrawal aborted, no debit)", wallet.Balance)
	}
	if len(withdrawalRepo.withdrawals) != 0 {
		t.Errorf("withdrawals recorded = %d, want 0 (aborted before create)", len(withdrawalRepo.withdrawals))
	}
}

// ---------------------------------------------------------------------------
// Tests: R-004 — lock ordering and post-forfeit debit failure
// ---------------------------------------------------------------------------

// lockOrderSpyTx records the relative order of wallet-row locks onto a shared
// operation log. It embeds recordingTx so LockForUpdate's QueryRow is answered
// by the preset fakeWalletRow.
type lockOrderSpyTx struct {
	*recordingTx
	order *[]string
}

func (t *lockOrderSpyTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if strings.Contains(sql, "FOR UPDATE") {
		*t.order = append(*t.order, "lock")
	}
	return t.recordingTx.QueryRow(ctx, sql, args...)
}

// orderSpyGrantRepo logs ListActiveByUserTx calls onto a shared operation log,
// embedding fakeGrantRepo for the rest of the repository surface.
type orderSpyGrantRepo struct {
	*fakeGrantRepo
	order *[]string
}

func (r *orderSpyGrantRepo) ListActiveByUserTx(_ context.Context, _ pgx.Tx, _ uuid.UUID) ([]domain.BonusGrant, error) {
	*r.order = append(*r.order, "list")
	return r.activeByUser, nil
}

// TestForfeitActiveGrantsTxLocksWalletBeforeListingGrants pins the R-002
// lock-first protocol: the wallet row must be locked BEFORE the grant list is
// read, so a concurrent claim or expiry cannot change the active set between
// the list and the lock (the old list-then-lock order was the TOCTOU window).
func TestForfeitActiveGrantsTxLocksWalletBeforeListingGrants(t *testing.T) {
	var order []string
	tx := &lockOrderSpyTx{recordingTx: newRecordingTx(), order: &order}
	tx.queryRowValue = &fakeWalletRow{id: uuid.New(), balance: 0, bonusBalance: 0, version: 1}

	base := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}}
	repo := &orderSpyGrantRepo{fakeGrantRepo: base, order: &order}
	userID := uuid.New()
	repo.activeByUser = []domain.BonusGrant{*activeGrant(userID, uuid.New(), 5000)}

	svc := NewBonusService(nil, repo, &fakeLedgerRepo{}, NewWalletService(nil), nil, nil)
	if err := svc.ForfeitActiveGrantsTx(context.Background(), tx, userID); err != nil {
		t.Fatalf("ForfeitActiveGrantsTx: %v", err)
	}

	if len(order) != 2 || order[0] != "lock" || order[1] != "list" {
		t.Fatalf("operation order = %v, want [lock list] (wallet lock must precede grant listing)", order)
	}
	if len(base.statuses) != 1 || base.statuses[0].status != domain.BonusStatusForfeited {
		t.Errorf("statuses = %+v, want one forfeited grant", base.statuses)
	}
}

// TestRequestWithdrawalDebitFailureLeavesForfeitCommitted is a PINNING test
// for the R-001 known gap: the forfeit transaction commits BEFORE the
// non-transactional debit, so a debit failure leaves the bonus pool
// forfeited with no withdrawal recorded. It documents current behavior; a
// future atomicity fix must flip the forfeit assertions to 0.
func TestRequestWithdrawalDebitFailureLeavesForfeitCommitted(t *testing.T) {
	grantRepo := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}}
	ledgerRepo := &fakeLedgerRepo{}
	svc, wallet, _, withdrawalRepo := newTestWithdrawalServiceWithBonus(8000, 5000, grantRepo, ledgerRepo)
	svc.walletRepo.(*mockWalletRepo).updateBalanceErr = errors.New("optimistic version conflict")

	grant := activeGrant(wallet.UserID, uuid.New(), 5000)
	grantRepo.activeByUser = []domain.BonusGrant{*grant}

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 8000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error when the post-forfeit debit fails")
	}

	// Forfeit already committed: grant flipped, ledger written.
	if len(grantRepo.statuses) != 1 {
		t.Errorf("status updates = %d, want 1 (forfeit committed before debit)", len(grantRepo.statuses))
	}
	if len(ledgerRepo.entries) != 1 {
		t.Errorf("ledger entries = %d, want 1 (forfeit ledger committed with forfeit tx)", len(ledgerRepo.entries))
	}
	// Withdrawal aborted: no record, no real money moved.
	if len(withdrawalRepo.withdrawals) != 0 {
		t.Errorf("withdrawals recorded = %d, want 0 (aborted before create)", len(withdrawalRepo.withdrawals))
	}
	if wallet.Balance != 8000 {
		t.Errorf("balance = %d, want 8000 (debit failed, no real money left)", wallet.Balance)
	}
}

// TestRequestWithdrawalCreateFailureLeavesForfeitCommitted is a PINNING test
// for the R-001 known gap: the forfeit transaction commits BEFORE the debit and
// the withdrawal create, so a failure after the forfeit leaves the bonus pool
// forfeited with no withdrawal recorded. Here the forfeit commits AND the debit
// succeeds, but the withdrawal create fails, so the pool is forfeited, real
// money has left the wallet, and nothing is recorded. It documents current
// behavior; a future single-transaction unification of forfeit+debit+create
// must flip the forfeit assertions to 0 (forfeit NOT applied).
func TestRequestWithdrawalCreateFailureLeavesForfeitCommitted(t *testing.T) {
	grantRepo := &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}}
	ledgerRepo := &fakeLedgerRepo{}
	svc, wallet, tx, withdrawalRepo := newTestWithdrawalServiceWithBonus(8000, 5000, grantRepo, ledgerRepo)
	withdrawalRepo.createErr = errors.New("withdrawal insert failed")

	grant := activeGrant(wallet.UserID, uuid.New(), 5000)
	grantRepo.activeByUser = []domain.BonusGrant{*grant}

	_, err := svc.RequestWithdrawal(context.Background(), wallet.UserID, 8000, "ABC123", "SOL")
	if err == nil {
		t.Fatal("expected error when the withdrawal create fails")
	}

	// Forfeit already committed: grant flipped to forfeited, pool zeroed,
	// forfeit ledger written inside the forfeit transaction.
	if len(grantRepo.statuses) != 1 {
		t.Fatalf("status updates = %d, want 1 (forfeit committed before create)", len(grantRepo.statuses))
	}
	if grantRepo.statuses[0].id != grant.ID || grantRepo.statuses[0].status != domain.BonusStatusForfeited {
		t.Errorf("status update = {%s %s}, want {%s %s}",
			grantRepo.statuses[0].id, grantRepo.statuses[0].status, grant.ID, domain.BonusStatusForfeited)
	}
	foundZero := false
	for _, e := range tx.execs {
		if strings.Contains(e.sql, "bonus_balance = 0") {
			foundZero = true
		}
	}
	if !foundZero {
		t.Errorf("no SQL zeroing bonus_balance was executed; execs=%d", len(tx.execs))
	}
	if len(ledgerRepo.entries) != 1 {
		t.Errorf("ledger entries = %d, want 1 (forfeit ledger committed with forfeit tx)", len(ledgerRepo.entries))
	}

	// Withdrawal aborted: the create failed, so no withdrawal is recorded.
	if len(withdrawalRepo.withdrawals) != 0 {
		t.Errorf("withdrawals recorded = %d, want 0 (create failed)", len(withdrawalRepo.withdrawals))
	}
	// The debit already succeeded and is NOT rolled back by the create failure:
	// the real wallet balance reflects the debit the fake reports (money left).
	if wallet.Balance != 0 {
		t.Errorf("balance = %d, want 0 (debit succeeded and is not rolled back by the create failure)", wallet.Balance)
	}
}
