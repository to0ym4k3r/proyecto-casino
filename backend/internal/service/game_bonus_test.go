package service

import (
	"context"
	"strings"
	"testing"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// bonusFakeTx extends fakeTx (from game_test.go) to handle the
// ConvertBonusToReal SQL: UPDATE wallets SET bonus_balance = GREATEST(...).
type bonusFakeTx struct {
	*fakeTx
	bonusBalance *int64
}

func (tx *bonusFakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "GREATEST(bonus_balance -") {
		amount := args[1].(int64)
		*tx.bonusBalance -= amount
		if *tx.bonusBalance < 0 {
			*tx.bonusBalance = 0
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	return tx.fakeTx.Exec(ctx, sql, args...)
}

// newBonusFakeTxFunc creates a TxFunc backed by a fakeTx that also handles
// bonus balance SQL. bonusBalance is a shared pointer so the caller can assert
// the final value after PlaceBet completes.
func newBonusFakeTxFunc(store *fakeTxStore, bonusBalance *int64) TxFunc {
	return func(_ context.Context, fn func(pgx.Tx) error) error {
		return fn(&bonusFakeTx{fakeTx: &fakeTx{store: store}, bonusBalance: bonusBalance})
	}
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

// ---------------------------------------------------------------------------
// Tests: PlaceBet + bonus integration
// ---------------------------------------------------------------------------

func TestPlaceBetNoBonusSvc(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	svc := NewGameServiceWithBonus(nil, nil, nil, nil, nil, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if store.wallets[walletID].balance != 4000 {
		t.Errorf("balance = %d, want 4000", store.wallets[walletID].balance)
	}
	if bet.Amount != 1000 {
		t.Errorf("bet.Amount = %d, want 1000", bet.Amount)
	}
}

func TestPlaceBetWithActiveBonus(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	grant := newTestGrant(userID, 1000, 10000, 0)

	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}

	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	// Wagering progress should be recorded
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	call := grantRepo.wageredCalls[0]
	if call.grantID != grant.ID {
		t.Errorf("wagered on grant %s, want %s", call.grantID, grant.ID)
	}
	if call.amount != 1000 {
		t.Errorf("wagered amount = %d, want 1000", call.amount)
	}

	// No status change (wagering not completed)
	if len(grantRepo.statusCalls) != 0 {
		t.Errorf("unexpected status calls: %d, want 0", len(grantRepo.statusCalls))
	}

	// Ledger entry written for the wagered portion
	if len(ledgerRepo.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(ledgerRepo.entries))
	}
	if ledgerRepo.entries[0].TransactionType != domain.BonusLedgerWagered {
		t.Errorf("ledger type = %s, want %s", ledgerRepo.entries[0].TransactionType, domain.BonusLedgerWagered)
	}
}

func TestPlaceBetCompletesWagering(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	// Grant needs 1000 more wagering — a bet of 1000 will complete it
	grant := newTestGrant(userID, 500, 1000, 0)

	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}

	bonusBalance := int64(500) // user has 500 in bonus balance
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newBonusFakeTxFunc(store, &bonusBalance))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	// Wagering should be recorded
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	if grantRepo.wageredCalls[0].amount != 1000 {
		t.Errorf("wagered amount = %d, want 1000", grantRepo.wageredCalls[0].amount)
	}

	// Grant should be marked as wagered (completed)
	if len(grantRepo.statusCalls) != 1 {
		t.Fatalf("status calls = %d, want 1", len(grantRepo.statusCalls))
	}
	if grantRepo.statusCalls[0].status != domain.BonusStatusWagered {
		t.Errorf("grant status = %s, want %s", grantRepo.statusCalls[0].status, domain.BonusStatusWagered)
	}

	// Bonus balance should be converted to real balance
	if bonusBalance != 0 {
		t.Errorf("bonusBalance = %d, want 0 (converted to real)", bonusBalance)
	}

	// Completion ledger entry written
	found := false
	for _, e := range ledgerRepo.entries {
		if e.TransactionType == domain.BonusLedgerCompleted {
			found = true
			if e.AmountCents != 500 {
				t.Errorf("completion ledger amount = %d, want 500", e.AmountCents)
			}
		}
	}
	if !found {
		t.Error("missing BonusLedgerCompleted entry")
	}
}

func TestPlaceBetMultipleGrantsPicksMostRemaining(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	// grant1: remaining = 10000 - 5000 = 5000
	grant1 := newTestGrant(userID, 1000, 10000, 5000)
	// grant2: remaining = 20000 - 5000 = 15000 — should be picked
	grant2 := newTestGrant(userID, 2000, 20000, 5000)

	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 10000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}

	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant1.ID] = grant1
	grantRepo.grants[grant2.ID] = grant2
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	_, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	// Should have wagered on grant2 (most remaining)
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	if grantRepo.wageredCalls[0].grantID != grant2.ID {
		t.Errorf("wagered on grant %s, want grant %s (most remaining)", grantRepo.wageredCalls[0].grantID, grant2.ID)
	}
}

func TestPlaceBetBonusPortionCappedAtRemaining(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	// Grant has only 500 remaining wagering — bet is 1000 but bonus portion capped at 500
	grant := newTestGrant(userID, 500, 1000, 500)

	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}

	bonusBalance := int64(500)
	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newBonusFakeTxFunc(store, &bonusBalance))

	_, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	// Bonus portion should be capped at remaining wagering (500)
	if len(grantRepo.wageredCalls) != 1 {
		t.Fatalf("wagered calls = %d, want 1", len(grantRepo.wageredCalls))
	}
	if grantRepo.wageredCalls[0].amount != 500 {
		t.Errorf("wagered amount = %d, want 500 (capped at remaining)", grantRepo.wageredCalls[0].amount)
	}

	// Wagering should complete (remaining was exactly 500)
	if len(grantRepo.statusCalls) != 1 {
		t.Fatalf("status calls = %d, want 1", len(grantRepo.statusCalls))
	}
	if grantRepo.statusCalls[0].status != domain.BonusStatusWagered {
		t.Errorf("status = %s, want %s", grantRepo.statusCalls[0].status, domain.BonusStatusWagered)
	}
}

func TestPlaceBetNoActiveGrantsProceedsNormally(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()

	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}

	grantRepo := newSpyGrantRepo() // empty — no active grants
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
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

func TestPlaceBetAllGrantsCompletedIgnoresBonus(t *testing.T) {
	userID := uuid.New()
	walletID := uuid.New()
	// Grant already has all wagering done (remaining = 0)
	grant := newTestGrant(userID, 500, 1000, 1000)
	grant.Status = domain.BonusStatusActive // still active in DB but fully wagered

	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}

	grantRepo := newSpyGrantRepo()
	grantRepo.grants[grant.ID] = grant
	ledgerRepo := &spyLedgerRepo{}
	wallets := NewWalletService(nil)
	bonusSvc := makeBonusService(grantRepo, ledgerRepo, wallets)

	svc := NewGameServiceWithBonus(nil, nil, nil, nil, bonusSvc, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}
	if bet == nil {
		t.Fatal("bet is nil")
	}

	// No bonus processing — all grants have zero remaining
	if len(grantRepo.wageredCalls) != 0 {
		t.Errorf("unexpected wagered calls: %d", len(grantRepo.wageredCalls))
	}
	if len(ledgerRepo.entries) != 0 {
		t.Errorf("unexpected ledger entries: %d", len(ledgerRepo.entries))
	}
}
