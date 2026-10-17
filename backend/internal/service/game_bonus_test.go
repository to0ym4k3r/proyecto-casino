package service

import (
	"context"
	"strings"
	"testing"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Test doubles for bonus integration
// ---------------------------------------------------------------------------

// spyGrantRepo implements BonusGrantRepository. ListActiveByUserTx returns
// pre-configured grants; IncrementWagered and UpdateStatus record calls.
// It deliberately does NOT mutate in-memory grant state on IncrementWagered
// because production Postgres does that server-side and the caller's struct
// snapshot would not reflect it.
type spyGrantRepo struct {
	grants       map[uuid.UUID]*domain.BonusGrant
	wageredCalls []wageredCall
	statusCalls  []statusCall
}

type wageredCall struct {
	grantID uuid.UUID
	amount  int64
}

type statusCall struct {
	grantID uuid.UUID
	status  string
}

func newSpyGrantRepo() *spyGrantRepo {
	return &spyGrantRepo{grants: make(map[uuid.UUID]*domain.BonusGrant)}
}

func (r *spyGrantRepo) Create(_ context.Context, _ pgx.Tx, grant *domain.BonusGrant) error {
	r.grants[grant.ID] = grant
	return nil
}

func (r *spyGrantRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.BonusGrant, error) {
	g, ok := r.grants[id]
	if !ok {
		return nil, repository.ErrBonusGrantNotFound
	}
	return g, nil
}

func (r *spyGrantRepo) GetActiveByUserAndDefinition(_ context.Context, _ pgx.Tx, userID, defID uuid.UUID) (*domain.BonusGrant, error) {
	for _, g := range r.grants {
		if g.UserID == userID && g.BonusDefinitionID == defID && g.Status == domain.BonusStatusActive {
			return g, nil
		}
	}
	return nil, repository.ErrBonusGrantNotFound
}

func (r *spyGrantRepo) ListActiveByUser(_ context.Context, userID uuid.UUID) ([]domain.BonusGrant, error) {
	var result []domain.BonusGrant
	for _, g := range r.grants {
		if g.UserID == userID && g.Status == domain.BonusStatusActive {
			result = append(result, *g)
		}
	}
	return result, nil
}

func (r *spyGrantRepo) ListActiveByUserTx(_ context.Context, _ pgx.Tx, userID uuid.UUID) ([]domain.BonusGrant, error) {
	var result []domain.BonusGrant
	for _, g := range r.grants {
		if g.UserID == userID && g.Status == domain.BonusStatusActive {
			result = append(result, *g)
		}
	}
	return result, nil
}

func (r *spyGrantRepo) ListByUser(_ context.Context, userID uuid.UUID, status string) ([]domain.BonusGrant, error) {
	return nil, nil
}

func (r *spyGrantRepo) ListAll(_ context.Context, status string, userID *uuid.UUID, limit, offset int) ([]domain.BonusGrant, int, error) {
	return nil, 0, nil
}

func (r *spyGrantRepo) UpdateStatus(_ context.Context, _ pgx.Tx, id uuid.UUID, status string) error {
	r.statusCalls = append(r.statusCalls, statusCall{grantID: id, status: status})
	return nil
}

func (r *spyGrantRepo) IncrementWagered(_ context.Context, _ pgx.Tx, id uuid.UUID, amount int64) error {
	r.wageredCalls = append(r.wageredCalls, wageredCall{grantID: id, amount: amount})
	return nil
}

func (r *spyGrantRepo) ExpireStale(_ context.Context, _ pgx.Tx) ([]domain.BonusGrant, error) {
	return nil, nil
}

// spyLedgerRepo records ledger entries for assertions.
type spyLedgerRepo struct {
	entries []domain.BonusLedgerEntry
}

func (r *spyLedgerRepo) Create(_ context.Context, _ pgx.Tx, entry *domain.BonusLedgerEntry) error {
	r.entries = append(r.entries, *entry)
	return nil
}

func (r *spyLedgerRepo) ListByGrant(_ context.Context, grantID uuid.UUID) ([]domain.BonusLedgerEntry, error) {
	return nil, nil
}

// newWalletStore builds a fakeTxStore holding a single wallet for userID with
// version 1. bonusBalance lives on the fake wallet row itself, so both the
// split-deduction UPDATE and ConvertBonusToReal operate on one shared state.
func newWalletStore(userID uuid.UUID, balance, bonusBalance int64) (*fakeTxStore, uuid.UUID) {
	walletID := uuid.New()
	store := &fakeTxStore{
		wallets: map[uuid.UUID]*fakeWallet{
			walletID: {id: walletID, userID: userID, balance: balance, bonusBalance: bonusBalance, version: 1},
		},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	return store, walletID
}

// makeBonusService constructs a real BonusService wired with the given spy
// repos and wallet service. This lets us test the full call chain from
// PlaceBet → findActiveGrantForBet → ProcessBetBonus → convertCompletedBonus.
func makeBonusService(grantRepo *spyGrantRepo, ledgerRepo *spyLedgerRepo, wallets *WalletService) *BonusService {
	return NewBonusService(nil, grantRepo, ledgerRepo, wallets, nil, nil)
}

// newTestGrant creates a BonusGrant for tests.
func newTestGrant(userID uuid.UUID, bonusCents, wageringRequired, totalWagered int64) *domain.BonusGrant {
	return &domain.BonusGrant{
		ID:                    uuid.New(),
		UserID:                userID,
		BonusDefinitionID:     uuid.New(),
		BonusAmountCents:      bonusCents,
		WageringRequiredCents: wageringRequired,
		TotalWageredCents:     totalWagered,
		Status:                domain.BonusStatusActive,
	}
}

// findLedger returns the first ledger entry of the given type, or nil.
func findLedger(entries []domain.BonusLedgerEntry, transactionType string) *domain.BonusLedgerEntry {
	for i := range entries {
		if entries[i].TransactionType == transactionType {
			return &entries[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests: PlaceBet wallet deduction (spec Domain 2, amended)
// ---------------------------------------------------------------------------

// TestPlaceBetFullyFromRealMoney covers spec Domain 2 Scenario 1: with no
// bonus money, the whole bet comes from real balance and the ONE two-column
// UPDATE still runs (single version bump, single conflict check — design §5).
func TestPlaceBetFullyFromRealMoney(t *testing.T) {
	userID := uuid.New()
	store, walletID := newWalletStore(userID, 10000, 0)
	svc := NewGameServiceWithBonus(nil, nil, nil, nil, nil, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 5000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	w := store.wallets[walletID]
	if w.balance != 5000 {
		t.Errorf("balance = %d, want 5000", w.balance)
	}
	if w.bonusBalance != 0 {
		t.Errorf("bonus_balance = %d, want 0", w.bonusBalance)
	}
	if w.version != 2 {
		t.Errorf("version = %d, want 2 (exactly one bump)", w.version)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1 (single two-column UPDATE)", store.splitDeducts)
	}
	if len(store.txLog) != 1 {
		t.Fatalf("txLog entries = %d, want 1", len(store.txLog))
	}
	if store.txLog[0].Balance != 5000 {
		t.Errorf("balance_after = %d, want 5000 (real balance after deduction)", store.txLog[0].Balance)
	}
	if store.txLog[0].Amount != -5000 {
		t.Errorf("transaction amount = %d, want -5000", store.txLog[0].Amount)
	}
}

// TestPlaceBetSplitsRealThenBonus covers spec Domain 2 Scenario 2: a bet
// larger than real balance consumes all real money first, then the bonus.
// balance=3000 + bonus=7000, bet 5000 → balance=0, bonus=5000.
func TestPlaceBetSplitsRealThenBonus(t *testing.T) {
	userID := uuid.New()
	store, walletID := newWalletStore(userID, 3000, 7000)
	svc := NewGameServiceWithBonus(nil, nil, nil, nil, nil, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 5000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	w := store.wallets[walletID]
	if w.balance != 0 {
		t.Errorf("balance = %d, want 0 (real 3000 fully spent)", w.balance)
	}
	if w.bonusBalance != 5000 {
		t.Errorf("bonus_balance = %d, want 5000 (7000 - 2000 bonus spent)", w.bonusBalance)
	}
	if w.version != 2 {
		t.Errorf("version = %d, want 2 (ONE bump for the two-column UPDATE)", w.version)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1", store.splitDeducts)
	}
	if len(store.txLog) != 1 {
		t.Fatalf("txLog entries = %d, want 1", len(store.txLog))
	}
	// balance_after is the REAL balance after the split: 3000 - 3000 = 0,
	// not the naive 3000 - 5000 = -2000.
	if store.txLog[0].Balance != 0 {
		t.Errorf("balance_after = %d, want 0 (real balance after split)", store.txLog[0].Balance)
	}
}

// TestPlaceBetInsufficientTotalBalanceRejected covers spec Domain 2
// Scenario 3: rejection happens only when balance + bonus_balance < amount,
// and the reported "have" value is the TOTAL available money.
func TestPlaceBetInsufficientTotalBalanceRejected(t *testing.T) {
	userID := uuid.New()
	store, walletID := newWalletStore(userID, 2000, 1000)
	svc := NewGameServiceWithBonus(nil, nil, nil, nil, nil, newFakeTxFunc(store))

	_, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 5000)
	if err == nil {
		t.Fatal("expected insufficient balance error, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Errorf("error = %v, want \"insufficient balance\" family", err)
	}
	// The guard must account for the TOTAL: 2000 + 1000 = 3000 available.
	if !strings.Contains(err.Error(), "have 3000, want 5000") {
		t.Errorf("error = %v, want \"have 3000, want 5000\" (total balance)", err)
	}

	w := store.wallets[walletID]
	if w.balance != 2000 || w.bonusBalance != 1000 || w.version != 1 {
		t.Errorf("wallet = {balance %d, bonus %d, version %d}, want {2000, 1000, 1} (untouched)", w.balance, w.bonusBalance, w.version)
	}
	if len(store.bets) != 0 || len(store.txLog) != 0 {
		t.Errorf("side effects recorded: bets=%d txLog=%d, want 0/0", len(store.bets), len(store.txLog))
	}
	if store.splitDeducts != 0 {
		t.Errorf("split deductions = %d, want 0", store.splitDeducts)
	}
}

// ---------------------------------------------------------------------------
// Tests: PlaceBet + bonus integration (re-derived from the amended spec/design)
// ---------------------------------------------------------------------------

// TestPlaceBetNoBonusSvc: no bonus wiring — the bet still runs and the split
// UPDATE handles the (zero) bonus column; deduction must not depend on the
// bonus service being configured.
func TestPlaceBetNoBonusSvc(t *testing.T) {
	userID := uuid.New()
	store, walletID := newWalletStore(userID, 5000, 0)
	svc := NewGameServiceWithBonus(nil, nil, nil, nil, nil, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet.Amount != 1000 {
		t.Errorf("bet.Amount = %d, want 1000", bet.Amount)
	}

	w := store.wallets[walletID]
	if w.balance != 4000 {
		t.Errorf("balance = %d, want 4000", w.balance)
	}
	if w.bonusBalance != 0 {
		t.Errorf("bonus_balance = %d, want 0", w.bonusBalance)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1 (two-column UPDATE even without bonus svc)", store.splitDeducts)
	}
}

// TestPlaceBetWithActiveBonus: an active grant, a bet that splits across
// real and bonus money, and wagering that advances by the FULL bet amount
// (2000 — not just the 1000 bonus-funded portion).
func TestPlaceBetWithActiveBonus(t *testing.T) {
	userID := uuid.New()
	grant := newTestGrant(userID, 1000, 10000, 0)

	store, walletID := newWalletStore(userID, 1000, 2000)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 2000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	// Split deduction: real 1000 exhausted + 1000 from bonus.
	w := store.wallets[walletID]
	if w.balance != 0 {
		t.Errorf("balance = %d, want 0", w.balance)
	}
	if w.bonusBalance != 1000 {
		t.Errorf("bonus_balance = %d, want 1000 (2000 - 1000 spent)", w.bonusBalance)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1", store.splitDeducts)
	}

	// Wagering advanced by the FULL bet amount (amended design §5).
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	call := grantRepo.wageredCalls[0]
	if call.grantID != grant.ID {
		t.Errorf("wagered on grant %s, want %s", call.grantID, grant.ID)
	}
	if call.amount != 2000 {
		t.Errorf("wagered amount = %d, want 2000 (full bet, not the bonus portion)", call.amount)
	}

	// No status change (wagering not completed: 2000 < 10000).
	if len(grantRepo.statusCalls) != 0 {
		t.Errorf("unexpected status calls: %d, want 0", len(grantRepo.statusCalls))
	}

	// One "wagered" ledger entry for the audited advance.
	if len(ledgerRepo.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(ledgerRepo.entries))
	}
	if ledgerRepo.entries[0].TransactionType != domain.BonusLedgerWagered {
		t.Errorf("ledger type = %s, want %s", ledgerRepo.entries[0].TransactionType, domain.BonusLedgerWagered)
	}
	if ledgerRepo.entries[0].AmountCents != 2000 {
		t.Errorf("ledger amount = %d, want 2000", ledgerRepo.entries[0].AmountCents)
	}
}

// TestPlaceBetWageringAdvancesByFullBetAmount covers spec Domain 1
// Scenario 3 (amended): required=5000, balance=1000, bonus=7000,
// PlaceBet(2500) → real 1000 + bonus 1500; total_wagered += 2500 (the FULL
// bet, not the 1500 bonus-funded portion); bonus_balance=5500.
func TestPlaceBetWageringAdvancesByFullBetAmount(t *testing.T) {
	userID := uuid.New()
	grant := newTestGrant(userID, 2000, 5000, 0)

	store, walletID := newWalletStore(userID, 1000, 7000)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	if _, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 2500); err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	w := store.wallets[walletID]
	if w.balance != 0 {
		t.Errorf("balance = %d, want 0 (real 1000 fully spent)", w.balance)
	}
	if w.bonusBalance != 5500 {
		t.Errorf("bonus_balance = %d, want 5500 (7000 - 1500)", w.bonusBalance)
	}
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	if grantRepo.wageredCalls[0].amount != 2500 {
		t.Errorf("wagered amount = %d, want 2500 (full bet, not the 1500 bonus portion)", grantRepo.wageredCalls[0].amount)
	}
	if len(grantRepo.statusCalls) != 0 {
		t.Errorf("unexpected status calls: %d, want 0 (2500 < 5000 required)", len(grantRepo.statusCalls))
	}
	led := findLedger(ledgerRepo.entries, domain.BonusLedgerWagered)
	if led == nil {
		t.Fatal("missing wagered ledger entry")
	}
	if led.AmountCents != 2500 {
		t.Errorf("ledger amount = %d, want 2500", led.AmountCents)
	}
}

// TestPlaceBetCompletesWagering covers the SPLIT completion path: the
// completing bet itself spends bonus money, so the conversion must move the
// REMAINING bonus balance (1300), not the original grant amount (1500).
// required=5000, wagered=4000, grant=1500, balance=800, bonus=1500,
// PlaceBet(1000) → real 800 + bonus 200 → total_wagered=5000.
func TestPlaceBetCompletesWagering(t *testing.T) {
	userID := uuid.New()
	grant := newTestGrant(userID, 1500, 5000, 4000)

	store, walletID := newWalletStore(userID, 800, 1500)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	// Split: real 800 + bonus 200; pool left for conversion = 1300.
	w := store.wallets[walletID]
	if w.bonusBalance != 0 {
		t.Errorf("bonus_balance = %d, want 0 (1300 remaining converted)", w.bonusBalance)
	}
	if w.balance != 1300 {
		t.Errorf("balance = %d, want 1300 (0 after bet + 1300 converted)", w.balance)
	}

	// Wagering advanced by the full bet and completed (4000 + 1000 = 5000).
	if len(grantRepo.wageredCalls) != 1 || grantRepo.wageredCalls[0].amount != 1000 {
		t.Fatalf("wagered calls = %+v, want one call of 1000", grantRepo.wageredCalls)
	}
	if len(grantRepo.statusCalls) != 1 {
		t.Fatalf("status calls = %d, want 1", len(grantRepo.statusCalls))
	}
	if grantRepo.statusCalls[0].status != domain.BonusStatusWagered {
		t.Errorf("grant status = %s, want %s", grantRepo.statusCalls[0].status, domain.BonusStatusWagered)
	}

	// Completion ledger records what was ACTUALLY converted (1300 remaining,
	// not the original 1500 — the bet spent 200 of bonus money).
	completed := findLedger(ledgerRepo.entries, domain.BonusLedgerCompleted)
	if completed == nil {
		t.Fatal("missing BonusLedgerCompleted entry")
	}
	if completed.AmountCents != 1300 {
		t.Errorf("completion ledger amount = %d, want 1300 (remaining bonus converted)", completed.AmountCents)
	}
}

// TestPlaceBetWageringCompletionConvertsRemainingBonus covers spec Domain 1
// Scenario 4 verbatim: required=5000, wagered=4000, bonus_balance=1500,
// PlaceBet(1000) fully funded by real money → status becomes 'wagered',
// the remaining 1500 bonus converts to real balance, ledger 'wagered_completed'.
func TestPlaceBetWageringCompletionConvertsRemainingBonus(t *testing.T) {
	userID := uuid.New()
	grant := newTestGrant(userID, 1500, 5000, 4000)

	store, walletID := newWalletStore(userID, 2000, 1500)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	if _, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000); err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	// Real money paid the bet (2000 → 1000), then 1500 bonus converted.
	w := store.wallets[walletID]
	if w.balance != 2500 {
		t.Errorf("balance = %d, want 2500 (1000 after bet + 1500 converted)", w.balance)
	}
	if w.bonusBalance != 0 {
		t.Errorf("bonus_balance = %d, want 0 (fully converted)", w.bonusBalance)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1 (single two-column UPDATE)", store.splitDeducts)
	}

	if len(grantRepo.wageredCalls) != 1 || grantRepo.wageredCalls[0].amount != 1000 {
		t.Fatalf("wagered calls = %+v, want one call of 1000", grantRepo.wageredCalls)
	}
	if len(grantRepo.statusCalls) != 1 || grantRepo.statusCalls[0].status != domain.BonusStatusWagered {
		t.Fatalf("status calls = %+v, want one %s", grantRepo.statusCalls, domain.BonusStatusWagered)
	}

	completed := findLedger(ledgerRepo.entries, domain.BonusLedgerCompleted)
	if completed == nil {
		t.Fatal("missing BonusLedgerCompleted entry")
	}
	if completed.AmountCents != 1500 {
		t.Errorf("completion ledger amount = %d, want 1500", completed.AmountCents)
	}
}

// TestPlaceBetMultipleGrantsPicksMostRemaining: wagering still targets the
// grant with the most remaining need, and the bet itself deducts from the
// wallet via the split UPDATE.
func TestPlaceBetMultipleGrantsPicksMostRemaining(t *testing.T) {
	userID := uuid.New()
	// grant1: remaining = 10000 - 5000 = 5000
	grant1 := newTestGrant(userID, 1000, 10000, 5000)
	// grant2: remaining = 20000 - 5000 = 15000 — should be picked
	grant2 := newTestGrant(userID, 2000, 20000, 5000)

	store, walletID := newWalletStore(userID, 10000, 0)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant1.ID] = grant1
	grantRepo.grants[grant2.ID] = grant2
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	if _, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000); err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	if grantRepo.wageredCalls[0].grantID != grant2.ID {
		t.Errorf("wagered on grant %s, want grant %s (most remaining)", grantRepo.wageredCalls[0].grantID, grant2.ID)
	}
	if store.wallets[walletID].balance != 9000 {
		t.Errorf("balance = %d, want 9000", store.wallets[walletID].balance)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1", store.splitDeducts)
	}
}

// TestPlaceBetBonusPortionCappedAtRemaining: the wagering advance is capped
// at the grant's remaining requirement — 500, not the full 1000 bet — and
// hitting the cap completes the grant.
func TestPlaceBetBonusPortionCappedAtRemaining(t *testing.T) {
	userID := uuid.New()
	// Grant has only 500 remaining wagering — bet is 1000 but the advance
	// is capped at 500 (which completes it: 500 + 500 = 1000 required).
	grant := newTestGrant(userID, 500, 1000, 500)

	store, walletID := newWalletStore(userID, 500, 500)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	if _, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000); err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	// Split: real 500 + bonus 500 — both pools drained by this bet.
	w := store.wallets[walletID]
	if w.balance != 0 || w.bonusBalance != 0 {
		t.Errorf("wallet = {balance %d, bonus %d}, want {0, 0}", w.balance, w.bonusBalance)
	}

	// Advance capped at remaining wagering (500), not the full 1000 bet.
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	if grantRepo.wageredCalls[0].amount != 500 {
		t.Errorf("wagered amount = %d, want 500 (capped at remaining)", grantRepo.wageredCalls[0].amount)
	}

	// Wagering completed → grant flips to 'wagered'.
	if len(grantRepo.statusCalls) != 1 {
		t.Fatalf("status calls = %d, want 1", len(grantRepo.statusCalls))
	}
	if grantRepo.statusCalls[0].status != domain.BonusStatusWagered {
		t.Errorf("status = %s, want %s", grantRepo.statusCalls[0].status, domain.BonusStatusWagered)
	}

	// The bonus pool was fully spent betting, so conversion moves 0 — but
	// the completion ledger entry is still written (honest, actual amount).
	completed := findLedger(ledgerRepo.entries, domain.BonusLedgerCompleted)
	if completed == nil {
		t.Fatal("missing BonusLedgerCompleted entry")
	}
	if completed.AmountCents != 0 {
		t.Errorf("completion ledger amount = %d, want 0 (no bonus left to convert)", completed.AmountCents)
	}
}

// TestPlaceBetNoActiveGrantsProceedsNormally: no active grant → the bet
// proceeds with zero bonus work (no wagering, no status change, no ledger).
func TestPlaceBetNoActiveGrantsProceedsNormally(t *testing.T) {
	userID := uuid.New()
	store, walletID := newWalletStore(userID, 5000, 0)

	grantRepo := newSpyGrantRepo() // empty — no active grants
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	if store.wallets[walletID].balance != 4000 {
		t.Errorf("balance = %d, want 4000", store.wallets[walletID].balance)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1", store.splitDeducts)
	}
	// No bonus activity
	if len(grantRepo.wageredCalls) != 0 {
		t.Errorf("unexpected wagered calls: %d", len(grantRepo.wageredCalls))
	}
	if len(grantRepo.statusCalls) != 0 {
		t.Errorf("unexpected status calls: %d", len(grantRepo.statusCalls))
	}
	if len(ledgerRepo.entries) != 0 {
		t.Errorf("unexpected ledger entries: %d", len(ledgerRepo.entries))
	}
}

// TestPlaceBetAllGrantsCompletedIgnoresBonus: a grant with zero remaining
// wagering is skipped entirely — no wagering, no conversion — while the bet
// still splits against the (untouched-here) bonus pool.
func TestPlaceBetAllGrantsCompletedIgnoresBonus(t *testing.T) {
	userID := uuid.New()
	// Grant already has all wagering done (remaining = 0)
	grant := newTestGrant(userID, 500, 1000, 1000)
	grant.Status = domain.BonusStatusActive // still active in DB but fully wagered

	store, walletID := newWalletStore(userID, 5000, 500)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, NewWalletService(nil))

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	// Fully-real bet; bonus pool untouched because there is nothing to advance.
	w := store.wallets[walletID]
	if w.balance != 4000 {
		t.Errorf("balance = %d, want 4000", w.balance)
	}
	if w.bonusBalance != 500 {
		t.Errorf("bonus_balance = %d, want 500 (untouched)", w.bonusBalance)
	}
	if store.splitDeducts != 1 {
		t.Errorf("split deductions = %d, want 1", store.splitDeducts)
	}
	// No bonus processing — all grants have zero remaining
	if len(grantRepo.wageredCalls) != 0 {
		t.Errorf("unexpected wagered calls: %d", len(grantRepo.wageredCalls))
	}
	if len(ledgerRepo.entries) != 0 {
		t.Errorf("unexpected ledger entries: %d", len(ledgerRepo.entries))
	}
}
