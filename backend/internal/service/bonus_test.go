package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Fakes for the three bonus repositories
// ---------------------------------------------------------------------------

// unexpectedCall is returned by fake methods a test did not configure; it
// makes accidental extra calls fail loudly instead of passing vacuously.
var unexpectedCall = errors.New("fake repo: unexpected call")

// fakeDefRepo implements repository.BonusDefinitionRepository.
type fakeDefRepo struct {
	byID       map[uuid.UUID]*domain.BonusDefinition
	listActive []domain.BonusDefinition
	getByIDErr error

	gotByID []uuid.UUID
}

func (f *fakeDefRepo) Create(_ context.Context, _ *domain.BonusDefinition) error {
	return unexpectedCall
}

func (f *fakeDefRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.BonusDefinition, error) {
	f.gotByID = append(f.gotByID, id)
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	if d, ok := f.byID[id]; ok {
		return d, nil
	}
	return nil, repository.ErrBonusDefinitionNotFound
}

func (f *fakeDefRepo) GetByCode(_ context.Context, _ string) (*domain.BonusDefinition, error) {
	return nil, unexpectedCall
}

func (f *fakeDefRepo) ListActive(_ context.Context) ([]domain.BonusDefinition, error) {
	return f.listActive, nil
}

func (f *fakeDefRepo) ListAll(_ context.Context) ([]domain.BonusDefinition, error) {
	return nil, unexpectedCall
}

func (f *fakeDefRepo) Update(_ context.Context, _ *domain.BonusDefinition) error {
	return unexpectedCall
}

// statusUpdate records one UpdateStatus call.
type statusUpdate struct {
	id     uuid.UUID
	status string
	tx     pgx.Tx
}

// incrementRecord records one IncrementWagered call.
type incrementRecord struct {
	id     uuid.UUID
	amount int64
}

// fakeGetByID scripts a single GetByID result. Tests that need to model a
// TOCTOU race (a stale pre-transaction read vs. the committed in-transaction
// re-read) push a sequence of these onto fakeGrantRepo.getByIDQueue.
type fakeGetByID struct {
	grant *domain.BonusGrant
	err   error
}

// fakeGrantRepo implements repository.BonusGrantRepository.
type fakeGrantRepo struct {
	byID            map[uuid.UUID]*domain.BonusGrant
	activeByUserDef *domain.BonusGrant // returned by GetActiveByUserAndDefinition when set
	activeByUser    []domain.BonusGrant
	expireStaleOut  []domain.BonusGrant
	createErr       error

	// getByIDQueue, when non-empty, is consumed one entry per GetByID call
	// before falling back to byID. It lets a test replay the exact per-call
	// reads of a concurrent/stale sequence.
	getByIDQueue []fakeGetByID

	created    []*domain.BonusGrant
	createdTxs []pgx.Tx
	statuses   []statusUpdate
	increments []incrementRecord
}

func (f *fakeGrantRepo) Create(_ context.Context, tx pgx.Tx, grant *domain.BonusGrant) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, grant)
	f.createdTxs = append(f.createdTxs, tx)
	return nil
}

func (f *fakeGrantRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.BonusGrant, error) {
	if len(f.getByIDQueue) > 0 {
		next := f.getByIDQueue[0]
		f.getByIDQueue = f.getByIDQueue[1:]
		return next.grant, next.err
	}
	if g, ok := f.byID[id]; ok {
		return g, nil
	}
	return nil, repository.ErrBonusGrantNotFound
}

func (f *fakeGrantRepo) GetActiveByUserAndDefinition(_ context.Context, _ pgx.Tx, _, _ uuid.UUID) (*domain.BonusGrant, error) {
	if f.activeByUserDef != nil {
		return f.activeByUserDef, nil
	}
	return nil, repository.ErrBonusGrantNotFound
}

func (f *fakeGrantRepo) ListActiveByUser(_ context.Context, _ uuid.UUID) ([]domain.BonusGrant, error) {
	return f.activeByUser, nil
}

func (f *fakeGrantRepo) ListActiveByUserTx(_ context.Context, _ pgx.Tx, _ uuid.UUID) ([]domain.BonusGrant, error) {
	return f.activeByUser, nil
}

func (f *fakeGrantRepo) ListByUser(_ context.Context, _ uuid.UUID, _ string) ([]domain.BonusGrant, error) {
	return nil, unexpectedCall
}

func (f *fakeGrantRepo) ListAll(_ context.Context, _ string, _ *uuid.UUID, _, _ int) ([]domain.BonusGrant, int, error) {
	return nil, 0, unexpectedCall
}

func (f *fakeGrantRepo) UpdateStatus(_ context.Context, tx pgx.Tx, id uuid.UUID, status string) error {
	f.statuses = append(f.statuses, statusUpdate{id: id, status: status, tx: tx})
	return nil
}

func (f *fakeGrantRepo) IncrementWagered(_ context.Context, tx pgx.Tx, id uuid.UUID, amount int64) error {
	f.increments = append(f.increments, incrementRecord{id: id, amount: amount})
	return nil
}

func (f *fakeGrantRepo) ExpireStale(_ context.Context, _ pgx.Tx) ([]domain.BonusGrant, error) {
	return f.expireStaleOut, nil
}

// fakeLedgerRepo implements repository.BonusLedgerRepository.
type fakeLedgerRepo struct {
	createErr error
	entries   []*domain.BonusLedgerEntry
	entryTxs  []pgx.Tx
}

func (f *fakeLedgerRepo) Create(_ context.Context, tx pgx.Tx, entry *domain.BonusLedgerEntry) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.entries = append(f.entries, entry)
	f.entryTxs = append(f.entryTxs, tx)
	return nil
}

func (f *fakeLedgerRepo) ListByGrant(_ context.Context, _ uuid.UUID) ([]domain.BonusLedgerEntry, error) {
	return nil, unexpectedCall
}

// bonusTestHarness bundles a BonusService with recording fakes and a txFunc
// handing out a recordingTx preset with a wallet row for LockForUpdate.
type bonusTestHarness struct {
	defs   *fakeDefRepo
	grants *fakeGrantRepo
	ledger *fakeLedgerRepo
	tx     *recordingTx
	svc    *BonusService
}

func newBonusTestHarness(t *testing.T) *bonusTestHarness {
	t.Helper()
	h := &bonusTestHarness{
		defs:   &fakeDefRepo{byID: map[uuid.UUID]*domain.BonusDefinition{}},
		grants: &fakeGrantRepo{byID: map[uuid.UUID]*domain.BonusGrant{}},
		ledger: &fakeLedgerRepo{},
		tx:     newRecordingTx(),
	}
	h.tx.queryRowValue = &fakeWalletRow{id: uuid.New(), balance: 0, bonusBalance: 0, version: 1}
	h.svc = NewBonusService(h.defs, h.grants, h.ledger, NewWalletService(nil), DefaultBonusHandlers(), func(_ context.Context, fn func(pgx.Tx) error) error {
		return fn(h.tx)
	})
	if h.svc == nil {
		t.Fatal("NewBonusService returned nil")
	}
	return h
}

// welcomeDefinition returns a canonical welcome bonus definition for tests.
func welcomeDefinition() *domain.BonusDefinition {
	return &domain.BonusDefinition{
		ID:                  uuid.New(),
		Code:                domain.BonusTypeWelcome,
		Name:                "Welcome Bonus",
		WageringMultiplier:  30,
		ExpiryDays:          30,
		MaxBonusAmountCents: 50000,
		IsActive:            true,
		Config:              map[string]interface{}{"match_percent": float64(100)},
	}
}

// --- ListAvailableBonuses ---------------------------------------------------

func TestListAvailableBonusesReturnsActiveDefinitions(t *testing.T) {
	h := newBonusTestHarness(t)
	want := []domain.BonusDefinition{*welcomeDefinition(), {ID: uuid.New(), Code: domain.BonusTypeReload, IsActive: true}}
	h.defs.listActive = want

	got, err := h.svc.ListAvailableBonuses(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("ListAvailableBonuses: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d definitions, want 2", len(got))
	}
	if got[0].Code != domain.BonusTypeWelcome || got[1].Code != domain.BonusTypeReload {
		t.Errorf("codes = (%s, %s), want (welcome, reload)", got[0].Code, got[1].Code)
	}
}

func TestListAvailableBonusesPropagatesRepoError(t *testing.T) {
	h := newBonusTestHarness(t)
	h.svc = NewBonusService(errDefs{}, h.grants, h.ledger, NewWalletService(nil), DefaultBonusHandlers(), func(_ context.Context, fn func(pgx.Tx) error) error {
		return fn(h.tx)
	})

	if _, err := h.svc.ListAvailableBonuses(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected error when definition repo fails")
	}
}

// errDefs always fails, to exercise repository error propagation.
type errDefs struct{}

func (errDefs) Create(_ context.Context, _ *domain.BonusDefinition) error { return unexpectedCall }
func (errDefs) GetByID(_ context.Context, _ uuid.UUID) (*domain.BonusDefinition, error) {
	return nil, unexpectedCall
}
func (errDefs) GetByCode(_ context.Context, _ string) (*domain.BonusDefinition, error) {
	return nil, unexpectedCall
}
func (errDefs) ListActive(_ context.Context) ([]domain.BonusDefinition, error) {
	return nil, errors.New("db down")
}
func (errDefs) ListAll(_ context.Context) ([]domain.BonusDefinition, error) {
	return nil, unexpectedCall
}
func (errDefs) Update(_ context.Context, _ *domain.BonusDefinition) error { return unexpectedCall }

// --- ClaimBonus -------------------------------------------------------------

func TestClaimBonusWelcomeCreatesGrantCreditsBalanceWritesLedger(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	h.defs.byID[def.ID] = def
	userID := uuid.New()
	deposit := int64(10000)

	before := time.Now().UTC()
	res, err := h.svc.ClaimBonus(context.Background(), userID, def.ID, map[string]interface{}{"deposit_cents": float64(deposit)})
	if err != nil {
		t.Fatalf("ClaimBonus: %v", err)
	}

	// Result payload mirrors the created grant terms.
	if res.GrantID == uuid.Nil {
		t.Error("result GrantID is nil UUID")
	}
	if res.BonusAmountCents != 10000 {
		t.Errorf("BonusAmountCents = %d, want 10000 (100%% of 10000)", res.BonusAmountCents)
	}
	if res.WageringRequiredCents != 300000 {
		t.Errorf("WageringRequiredCents = %d, want 300000 (30x)", res.WageringRequiredCents)
	}
	if _, err := time.Parse(time.RFC3339, res.ExpiresAt); err != nil {
		t.Errorf("ExpiresAt %q is not RFC3339: %v", res.ExpiresAt, err)
	}

	// Exactly one grant persisted, inside the harness transaction.
	if len(h.grants.created) != 1 {
		t.Fatalf("created %d grants, want 1", len(h.grants.created))
	}
	g := h.grants.created[0]
	if h.grants.createdTxs[0] != pgx.Tx(h.tx) {
		t.Error("grant was not created inside the service transaction")
	}
	if g.UserID != userID || g.BonusDefinitionID != def.ID {
		t.Errorf("grant identity = (%s, %s), want (%s, %s)", g.UserID, g.BonusDefinitionID, userID, def.ID)
	}
	if g.Status != domain.BonusStatusActive {
		t.Errorf("grant Status = %q, want %q", g.Status, domain.BonusStatusActive)
	}
	if g.GrantedBy != domain.GrantedBySystem {
		t.Errorf("GrantedBy = %q, want %q", g.GrantedBy, domain.GrantedBySystem)
	}
	if !g.ExpiresAt.After(before.Add(29 * 24 * time.Hour)) {
		t.Errorf("ExpiresAt = %v, want ~30 days out", g.ExpiresAt)
	}

	// Wallet credited with the bonus amount inside the same tx.
	foundCredit := false
	for _, e := range h.tx.execs {
		if strings.Contains(e.sql, "bonus_balance = bonus_balance +") &&
			len(e.args) == 2 && e.args[0] == userID && e.args[1] == int64(10000) {
			foundCredit = true
		}
	}
	if !foundCredit {
		t.Errorf("wallet credit for %+v amount 10000 not recorded; execs=%d", userID, len(h.tx.execs))
	}

	// One granted ledger entry, same tx.
	if len(h.ledger.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(h.ledger.entries))
	}
	if h.ledger.entryTxs[0] != pgx.Tx(h.tx) {
		t.Error("ledger entry was not written inside the service transaction")
	}
	if h.ledger.entries[0].TransactionType != domain.BonusLedgerGranted ||
		h.ledger.entries[0].AmountCents != 10000 ||
		h.ledger.entries[0].GrantID != g.ID {
		t.Errorf("ledger entry = %+v, want granted/10000/grant %s", h.ledger.entries[0], g.ID)
	}
}

func TestClaimBonusUnknownDefinitionRejected(t *testing.T) {
	h := newBonusTestHarness(t)
	_, err := h.svc.ClaimBonus(context.Background(), uuid.New(), uuid.New(), nil)
	if !errors.Is(err, domain.ErrBonusNotFound) {
		t.Fatalf("err = %v, want domain.ErrBonusNotFound", err)
	}
	if len(h.grants.created) != 0 || len(h.ledger.entries) != 0 {
		t.Error("no grant or ledger work may happen for unknown definition")
	}
}

func TestClaimBonusInactiveDefinitionRejected(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	def.IsActive = false
	h.defs.byID[def.ID] = def

	_, err := h.svc.ClaimBonus(context.Background(), uuid.New(), def.ID, nil)
	if !errors.Is(err, domain.ErrBonusNotActive) {
		t.Fatalf("err = %v, want domain.ErrBonusNotActive", err)
	}
	if len(h.grants.created) != 0 {
		t.Error("inactive bonus must not create grants")
	}
}

func TestClaimBonusDuplicateActiveGrantRejectedBeforeAnyMoneyMoves(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	h.defs.byID[def.ID] = def
	h.grants.activeByUserDef = &domain.BonusGrant{ID: uuid.New(), Status: domain.BonusStatusActive}

	_, err := h.svc.ClaimBonus(context.Background(), uuid.New(), def.ID, map[string]interface{}{"deposit_cents": float64(10000)})
	if !errors.Is(err, domain.ErrActiveBonusExists) {
		t.Fatalf("err = %v, want domain.ErrActiveBonusExists", err)
	}
	if len(h.grants.created) != 0 || len(h.ledger.entries) != 0 {
		t.Error("duplicate claim must not create grants or ledger entries")
	}
	if len(h.tx.execs) != 0 {
		t.Errorf("duplicate claim must not move money, got %d wallet execs", len(h.tx.execs))
	}
}

func TestClaimBonusMapsUniqueViolationToActiveExists(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	h.defs.byID[def.ID] = def
	h.grants.createErr = repository.ErrActiveBonusGrantExists

	_, err := h.svc.ClaimBonus(context.Background(), uuid.New(), def.ID, map[string]interface{}{"deposit_cents": float64(10000)})
	if !errors.Is(err, domain.ErrActiveBonusExists) {
		t.Fatalf("err = %v, want domain.ErrActiveBonusExists (23505 belt-and-braces path)", err)
	}
}

func TestClaimBonusReloadBelowMinDepositRejected(t *testing.T) {
	h := newBonusTestHarness(t)
	def := &domain.BonusDefinition{
		ID:                  uuid.New(),
		Code:                domain.BonusTypeReload,
		WageringMultiplier:  25,
		ExpiryDays:          14,
		MaxBonusAmountCents: 25000,
		IsActive:            true,
		Config:              map[string]interface{}{"match_percent": float64(50)},
		MinDepositCents:     5000,
	}
	h.defs.byID[def.ID] = def

	_, err := h.svc.ClaimBonus(context.Background(), uuid.New(), def.ID, map[string]interface{}{"deposit_cents": float64(4999)})
	if !errors.Is(err, domain.ErrBonusNotClaimable) {
		t.Fatalf("err = %v, want domain.ErrBonusNotClaimable", err)
	}
	if len(h.grants.created) != 0 {
		t.Error("rejected claim must not create grants")
	}
}

func TestClaimBonusUnregisteredCodeRejected(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	def.Code = "mystery_box"
	h.defs.byID[def.ID] = def

	_, err := h.svc.ClaimBonus(context.Background(), uuid.New(), def.ID, nil)
	if !errors.Is(err, domain.ErrBonusNotClaimable) {
		t.Fatalf("err = %v, want domain.ErrBonusNotClaimable", err)
	}
}

// --- GetActiveGrants --------------------------------------------------------

func TestGetActiveGrantsBuildsResponsesWithProgressAndNames(t *testing.T) {
	h := newBonusTestHarness(t)
	welcome := welcomeDefinition()
	reload := &domain.BonusDefinition{ID: uuid.New(), Code: domain.BonusTypeReload, Name: "Reload Bonus"}
	h.defs.byID[welcome.ID] = welcome
	h.defs.byID[reload.ID] = reload
	userID := uuid.New()

	inProgress := domain.BonusGrant{
		ID: uuid.New(), UserID: userID, BonusDefinitionID: welcome.ID,
		BonusAmountCents: 10000, WageringRequiredCents: 10000, TotalWageredCents: 3000,
		Status: domain.BonusStatusActive,
	}
	noWagering := domain.BonusGrant{
		ID: uuid.New(), UserID: userID, BonusDefinitionID: reload.ID,
		BonusAmountCents: 2500, WageringRequiredCents: 0, TotalWageredCents: 0,
		Status: domain.BonusStatusActive,
	}
	h.grants.activeByUser = []domain.BonusGrant{inProgress, noWagering}

	got, err := h.svc.GetActiveGrants(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetActiveGrants: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d responses, want 2", len(got))
	}

	first := got[0]
	if first.GrantID != inProgress.ID || first.Code != domain.BonusTypeWelcome || first.Name != "Welcome Bonus" {
		t.Errorf("first = {%s %s %s}, want grant joined to welcome definition", first.GrantID, first.Code, first.Name)
	}
	if first.WageringProgressPct != 30.0 {
		t.Errorf("progress = %v, want 30.0 (3000/10000)", first.WageringProgressPct)
	}
	if _, err := time.Parse(time.RFC3339, first.ExpiresAt); err != nil {
		t.Errorf("ExpiresAt %q not RFC3339: %v", first.ExpiresAt, err)
	}

	if got[1].WageringProgressPct != 100.0 {
		t.Errorf("zero-requirement progress = %v, want 100.0", got[1].WageringProgressPct)
	}
	if got[1].Code != domain.BonusTypeReload || got[1].BonusAmountCents != 2500 {
		t.Errorf("second response = {%s %d}, want reload/2500", got[1].Code, got[1].BonusAmountCents)
	}
}

func TestGetActiveGrantsEmptyForUserWithoutGrants(t *testing.T) {
	h := newBonusTestHarness(t)

	got, err := h.svc.GetActiveGrants(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetActiveGrants: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d responses, want 0 for user without grants", len(got))
	}
}

// --- ForfeitBonus -----------------------------------------------------------

func activeGrant(userID uuid.UUID, defID uuid.UUID, amount int64) *domain.BonusGrant {
	return &domain.BonusGrant{
		ID:                uuid.New(),
		UserID:            userID,
		BonusDefinitionID: defID,
		BonusAmountCents:  amount,
		Status:            domain.BonusStatusActive,
		GrantedBy:         domain.GrantedBySystem,
	}
}

func TestForfeitBonusDebitsGrantAmountAndWritesLedger(t *testing.T) {
	h := newBonusTestHarness(t)
	userID := uuid.New()
	grant := activeGrant(userID, uuid.New(), 5000)
	h.grants.byID[grant.ID] = grant

	if err := h.svc.ForfeitBonus(context.Background(), userID, grant.ID); err != nil {
		t.Fatalf("ForfeitBonus: %v", err)
	}

	// Wallet debited by exactly the surrendered grant amount.
	foundDebit := false
	for _, e := range h.tx.execs {
		if strings.Contains(e.sql, "GREATEST(bonus_balance -") &&
			len(e.args) == 2 && e.args[0] == userID && e.args[1] == int64(5000) {
			foundDebit = true
		}
	}
	if !foundDebit {
		t.Errorf("per-grant debit of 5000 not recorded; execs=%d", len(h.tx.execs))
	}

	// Grant flipped to forfeited inside the same tx.
	if len(h.grants.statuses) != 1 {
		t.Fatalf("status updates = %d, want 1", len(h.grants.statuses))
	}
	up := h.grants.statuses[0]
	if up.id != grant.ID || up.status != domain.BonusStatusForfeited || up.tx != pgx.Tx(h.tx) {
		t.Errorf("status update = {%s %s tx-match:%v}, want forfeited on harness tx",
			up.id, up.status, up.tx == pgx.Tx(h.tx))
	}

	// One forfeited ledger entry with the surrendered amount.
	if len(h.ledger.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(h.ledger.entries))
	}
	e := h.ledger.entries[0]
	if e.TransactionType != domain.BonusLedgerForfeited || e.AmountCents != 5000 || e.GrantID != grant.ID {
		t.Errorf("ledger entry = %+v, want forfeited/5000/%s", e, grant.ID)
	}
}

func TestForfeitBonusRejectsForeignOrMissingGrant(t *testing.T) {
	h := newBonusTestHarness(t)
	grant := activeGrant(uuid.New(), uuid.New(), 5000) // owned by someone else
	h.grants.byID[grant.ID] = grant

	err := h.svc.ForfeitBonus(context.Background(), uuid.New(), grant.ID)
	if !errors.Is(err, domain.ErrGrantNotFound) {
		t.Fatalf("err = %v, want domain.ErrGrantNotFound for foreign grant", err)
	}
	if len(h.tx.execs) != 0 || len(h.grants.statuses) != 0 || len(h.ledger.entries) != 0 {
		t.Error("foreign forfeit must not move money or state")
	}

	err = h.svc.ForfeitBonus(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrGrantNotFound) {
		t.Fatalf("err = %v, want domain.ErrGrantNotFound for missing grant", err)
	}
}

func TestForfeitBonusRejectsNonActiveGrant(t *testing.T) {
	h := newBonusTestHarness(t)
	userID := uuid.New()
	grant := activeGrant(userID, uuid.New(), 5000)
	grant.Status = domain.BonusStatusExpired
	h.grants.byID[grant.ID] = grant

	err := h.svc.ForfeitBonus(context.Background(), userID, grant.ID)
	if !errors.Is(err, domain.ErrGrantNotFound) {
		t.Fatalf("err = %v, want domain.ErrGrantNotFound for expired grant", err)
	}
	if len(h.grants.statuses) != 0 {
		t.Error("expired grant must stay untouched")
	}
}

// --- ProcessBetBonus --------------------------------------------------------

func TestProcessBetBonusIncrementsWageredAndWritesLedger(t *testing.T) {
	h := newBonusTestHarness(t)
	grantID := uuid.New()

	if err := h.svc.ProcessBetBonus(context.Background(), h.tx, grantID, 2500); err != nil {
		t.Fatalf("ProcessBetBonus: %v", err)
	}

	// Wagering incremented inside the caller's (PlaceBet) transaction.
	if len(h.grants.increments) != 1 {
		t.Fatalf("increments = %d, want 1", len(h.grants.increments))
	}
	inc := h.grants.increments[0]
	if inc.id != grantID || inc.amount != 2500 {
		t.Errorf("increment = {%s %d}, want {%s 2500}", inc.id, inc.amount, grantID)
	}

	// Wagering progression audited on the ledger.
	if len(h.ledger.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(h.ledger.entries))
	}
	e := h.ledger.entries[0]
	if e.TransactionType != domain.BonusLedgerWagered || e.AmountCents != 2500 || e.GrantID != grantID {
		t.Errorf("ledger entry = %+v, want wagered/2500/%s", e, grantID)
	}
}

func TestProcessBetBonusRejectsNegativePortion(t *testing.T) {
	h := newBonusTestHarness(t)

	err := h.svc.ProcessBetBonus(context.Background(), h.tx, uuid.New(), -1)
	if err == nil {
		t.Fatal("expected error for negative bonus portion")
	}
	if len(h.grants.increments) != 0 || len(h.ledger.entries) != 0 {
		t.Error("negative portion must not touch grants or ledger")
	}
}

// --- ExpireStaleBonuses -----------------------------------------------------

func TestExpireStaleBonusesDebitsBalancesAndWritesExpiredEntries(t *testing.T) {
	h := newBonusTestHarness(t)
	userA, userB := uuid.New(), uuid.New()
	stale := []domain.BonusGrant{
		{ID: uuid.New(), UserID: userA, BonusAmountCents: 5000, Status: domain.BonusStatusActive},
		{ID: uuid.New(), UserID: userB, BonusAmountCents: 2500, Status: domain.BonusStatusActive},
	}
	h.grants.expireStaleOut = stale

	count, err := h.svc.ExpireStaleBonuses(context.Background())
	if err != nil {
		t.Fatalf("ExpireStaleBonuses: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}

	// One per-grant debit per expired grant, each inside the same tx.
	debits := map[uuid.UUID]int64{}
	for _, e := range h.tx.execs {
		if strings.Contains(e.sql, "GREATEST(bonus_balance -") && len(e.args) == 2 {
			debits[e.args[0].(uuid.UUID)] = e.args[1].(int64)
		}
	}
	if debits[userA] != 5000 || debits[userB] != 2500 {
		t.Errorf("debits = %v, want [%v:5000 %v:2500]", debits, userA, userB)
	}

	// Ledger records every expiration.
	if len(h.ledger.entries) != 2 {
		t.Fatalf("ledger entries = %d, want 2", len(h.ledger.entries))
	}
	for i, wantType := range []string{domain.BonusLedgerExpired, domain.BonusLedgerExpired} {
		e := h.ledger.entries[i]
		if e.TransactionType != wantType || e.GrantID != stale[i].ID || e.AmountCents != stale[i].BonusAmountCents {
			t.Errorf("entry[%d] = %+v, want expired/%s/%d", i, e, stale[i].ID, stale[i].BonusAmountCents)
		}
		if h.ledger.entryTxs[i] != pgx.Tx(h.tx) {
			t.Errorf("entry[%d] written outside harness tx", i)
		}
	}
}

func TestExpireStaleBonusesNoGrantsIsNoOp(t *testing.T) {
	h := newBonusTestHarness(t)

	count, err := h.svc.ExpireStaleBonuses(context.Background())
	if err != nil {
		t.Fatalf("ExpireStaleBonuses: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
	if len(h.tx.execs) != 0 || len(h.ledger.entries) != 0 {
		t.Error("no-op expiry must not move money or write entries")
	}
}

// --- Pure calculation helpers ------------------------------------------------

func TestPercentOfCents(t *testing.T) {
	cases := []struct {
		name        string
		amountCents int64
		pct         float64
		want        int64
	}{
		{"full match", 10000, 100, 10000},
		{"half match rounds half up", 9999, 50, 5000},
		{"ten percent cashback", 20000, 10, 2000},
		{"sub-cent result floors to zero", 1, 1, 0},
		{"zero percent", 50000, 0, 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentOfCents(tt.amountCents, tt.pct); got != tt.want {
				t.Errorf("percentOfCents(%d, %v) = %d, want %d", tt.amountCents, tt.pct, got, tt.want)
			}
		})
	}
}

func TestCappedMatchEnforcesMaximum(t *testing.T) {
	if got := cappedMatch(100000, 100, 50000); got != 50000 {
		t.Errorf("cappedMatch over cap = %d, want 50000", got)
	}
	if got := cappedMatch(10000, 50, 25000); got != 5000 {
		t.Errorf("cappedMatch under cap = %d, want 5000", got)
	}
}

func TestEffectiveMinDepositTakesLargest(t *testing.T) {
	columnOnly := &domain.BonusDefinition{MinDepositCents: 3000}
	if got := effectiveMinDeposit(columnOnly); got != 3000 {
		t.Errorf("column only = %d, want 3000", got)
	}
	configOverrides := &domain.BonusDefinition{
		MinDepositCents: 3000,
		Config:          map[string]interface{}{configMinDepositCents: float64(5000)},
	}
	if got := effectiveMinDeposit(configOverrides); got != 5000 {
		t.Errorf("config override = %d, want 5000", got)
	}
	configLower := &domain.BonusDefinition{
		MinDepositCents: 5000,
		Config:          map[string]interface{}{configMinDepositCents: float64(1000)},
	}
	if got := effectiveMinDeposit(configLower); got != 5000 {
		t.Errorf("lower config ignored = %d, want 5000", got)
	}
}

// --- CashbackHandler --------------------------------------------------------

func TestCashbackHandlerValidateAndCalculate(t *testing.T) {
	cashbackDef := func() *domain.BonusDefinition {
		return &domain.BonusDefinition{
			ID:                  uuid.New(),
			Code:                domain.BonusTypeCashback,
			WageringMultiplier:  0,
			ExpiryDays:          7,
			MaxBonusAmountCents: 10000,
			IsActive:            true,
			Config:              map[string]interface{}{"cashback_percent": float64(10)},
		}
	}

	t.Run("accepts positive losses and prices ten percent", func(t *testing.T) {
		def := cashbackDef()
		params := map[string]interface{}{paramLossAmountCents: float64(20000)}
		h := CashbackHandler{}
		if err := h.ValidateClaim(context.Background(), uuid.New(), def, params); err != nil {
			t.Fatalf("ValidateClaim: %v", err)
		}
		got, err := h.CalculateBonus(context.Background(), def, params)
		if err != nil || got != 2000 {
			t.Errorf("CalculateBonus = (%d, %v), want (2000, nil)", got, err)
		}
	})

	t.Run("caps cashback at definition maximum", func(t *testing.T) {
		def := cashbackDef()
		got, err := CashbackHandler{}.CalculateBonus(context.Background(), def, map[string]interface{}{paramLossAmountCents: float64(200000)})
		if err != nil || got != 10000 {
			t.Errorf("CalculateBonus = (%d, %v), want capped (10000, nil)", got, err)
		}
	})

	t.Run("rejects missing zero and negative losses", func(t *testing.T) {
		h := CashbackHandler{}
		for name, params := range map[string]map[string]interface{}{
			"missing":  {},
			"zero":     {paramLossAmountCents: float64(0)},
			"negative": {paramLossAmountCents: float64(-5)},
		} {
			err := h.ValidateClaim(context.Background(), uuid.New(), cashbackDef(), params)
			if !errors.Is(err, domain.ErrBonusNotClaimable) {
				t.Errorf("%s: err = %v, want domain.ErrBonusNotClaimable", name, err)
			}
		}
	})
}

// --- FreeSpinsHandler -------------------------------------------------------

func TestFreeSpinsHandlerFixedAmountAndGameEligibility(t *testing.T) {
	freeSpinsDef := func(configGame interface{}) *domain.BonusDefinition {
		cfg := map[string]interface{}{"spins_count": float64(50)}
		if configGame != nil {
			cfg[configGameID] = configGame
		}
		return &domain.BonusDefinition{
			ID:                  uuid.New(),
			Code:                domain.BonusTypeFreeSpins,
			WageringMultiplier:  20,
			ExpiryDays:          7,
			MaxBonusAmountCents: 5000,
			IsActive:            true,
			Config:              cfg,
		}
	}

	t.Run("calculates fixed maximum bonus", func(t *testing.T) {
		def := freeSpinsDef(nil)
		got, err := FreeSpinsHandler{}.CalculateBonus(context.Background(), def, nil)
		if err != nil || got != 5000 {
			t.Errorf("CalculateBonus = (%d, %v), want fixed (5000, nil)", got, err)
		}
	})

	t.Run("any game accepted when config does not pin one", func(t *testing.T) {
		def := freeSpinsDef(nil)
		err := FreeSpinsHandler{}.ValidateClaim(context.Background(), uuid.New(), def,
			map[string]interface{}{paramGameID: "some-game"})
		if err != nil {
			t.Fatalf("ValidateClaim with unpinned config: %v", err)
		}
		err = FreeSpinsHandler{}.ValidateClaim(context.Background(), uuid.New(), def, nil)
		if err != nil {
			t.Fatalf("ValidateClaim without params: %v", err)
		}
	})

	t.Run("pinned game id enforced", func(t *testing.T) {
		def := freeSpinsDef("game-123")
		if err := (FreeSpinsHandler{}).ValidateClaim(context.Background(), uuid.New(), def,
			map[string]interface{}{paramGameID: "other-game"}); !errors.Is(err, domain.ErrBonusNotClaimable) {
			t.Errorf("mismatched game err = %v, want domain.ErrBonusNotClaimable", err)
		}
		if err := (FreeSpinsHandler{}).ValidateClaim(context.Background(), uuid.New(), def, nil); !errors.Is(err, domain.ErrBonusNotClaimable) {
			t.Errorf("missing game err = %v, want domain.ErrBonusNotClaimable", err)
		}
		if err := (FreeSpinsHandler{}).ValidateClaim(context.Background(), uuid.New(), def,
			map[string]interface{}{paramGameID: "game-123"}); err != nil {
			t.Errorf("matching game err = %v, want nil", err)
		}
	})
}

// --- AdminGrant -------------------------------------------------------------

func TestAdminGrantMirrorsClaimAndStampsAdminOrigin(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	h.defs.byID[def.ID] = def
	adminID := uuid.New()
	userID := uuid.New()
	deposit := int64(10000)

	res, err := h.svc.AdminGrant(context.Background(), adminID, userID, def.ID, map[string]interface{}{"deposit_cents": float64(deposit)})
	if err != nil {
		t.Fatalf("AdminGrant: %v", err)
	}
	if res == nil || res.GrantID == uuid.Nil {
		t.Fatalf("result = %+v, want a non-nil grant id", res)
	}
	if res.BonusAmountCents != 10000 || res.WageringRequiredCents != 300000 {
		t.Errorf("pricing = (%d, %d), want (10000, 300000)", res.BonusAmountCents, res.WageringRequiredCents)
	}

	// Exactly one grant persisted, inside the harness transaction.
	if len(h.grants.created) != 1 {
		t.Fatalf("created %d grants, want 1", len(h.grants.created))
	}
	g := h.grants.created[0]
	if g.UserID != userID || g.BonusDefinitionID != def.ID {
		t.Errorf("grant identity = (%s, %s), want (%s, %s)", g.UserID, g.BonusDefinitionID, userID, def.ID)
	}
	if g.Status != domain.BonusStatusActive {
		t.Errorf("grant Status = %q, want %q", g.Status, domain.BonusStatusActive)
	}
	// The whole point of AdminGrant vs ClaimBonus: provenance.
	if g.GrantedBy != domain.GrantedByAdmin {
		t.Errorf("GrantedBy = %q, want %q", g.GrantedBy, domain.GrantedByAdmin)
	}
	if g.AdminID == nil || *g.AdminID != adminID {
		t.Errorf("AdminID = %v, want %s", g.AdminID, adminID)
	}

	// Bonus balance credited with the priced amount inside the same tx.
	foundCredit := false
	for _, e := range h.tx.execs {
		if strings.Contains(e.sql, "bonus_balance = bonus_balance +") &&
			len(e.args) == 2 && e.args[0] == userID && e.args[1] == int64(10000) {
			foundCredit = true
		}
	}
	if !foundCredit {
		t.Errorf("wallet credit of 10000 for %s not recorded; execs=%d", userID, len(h.tx.execs))
	}

	// One 'granted' ledger entry for the new grant, same tx.
	if len(h.ledger.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(h.ledger.entries))
	}
	e := h.ledger.entries[0]
	if e.TransactionType != domain.BonusLedgerGranted || e.AmountCents != 10000 || e.GrantID != g.ID {
		t.Errorf("ledger entry = %+v, want granted/10000/%s", e, g.ID)
	}
}

func TestAdminGrantDuplicateActiveGrantRejectedBeforeAnyMoneyMoves(t *testing.T) {
	h := newBonusTestHarness(t)
	def := welcomeDefinition()
	h.defs.byID[def.ID] = def
	h.grants.activeByUserDef = &domain.BonusGrant{ID: uuid.New(), Status: domain.BonusStatusActive}

	_, err := h.svc.AdminGrant(context.Background(), uuid.New(), uuid.New(), def.ID, map[string]interface{}{"deposit_cents": float64(10000)})
	if !errors.Is(err, domain.ErrActiveBonusExists) {
		t.Fatalf("err = %v, want domain.ErrActiveBonusExists", err)
	}
	if len(h.grants.created) != 0 || len(h.ledger.entries) != 0 {
		t.Error("duplicate admin grant must not create grants or ledger entries")
	}
	if len(h.tx.execs) != 0 {
		t.Errorf("duplicate admin grant must not move money, got %d wallet execs", len(h.tx.execs))
	}
}

func TestAdminGrantUnknownDefinitionRejected(t *testing.T) {
	h := newBonusTestHarness(t)

	_, err := h.svc.AdminGrant(context.Background(), uuid.New(), uuid.New(), uuid.New(), nil)
	if !errors.Is(err, domain.ErrBonusNotFound) {
		t.Fatalf("err = %v, want domain.ErrBonusNotFound", err)
	}
	if len(h.grants.created) != 0 {
		t.Error("unknown definition must not create grants")
	}
}

// --- AdminRevoke ------------------------------------------------------------

func TestAdminRevokeForfeitsGrantWithoutOwnership(t *testing.T) {
	h := newBonusTestHarness(t)
	owner := uuid.New()
	grant := activeGrant(owner, uuid.New(), 5000)
	h.grants.byID[grant.ID] = grant

	// A DIFFERENT admin (not the owner) revokes the grant — no ownership rule.
	adminID := uuid.New()
	if err := h.svc.AdminRevoke(context.Background(), adminID, grant.ID); err != nil {
		t.Fatalf("AdminRevoke: %v", err)
	}

	// Bonus balance debited by the grant amount for the OWNER, zero-floored.
	foundDebit := false
	for _, e := range h.tx.execs {
		if strings.Contains(e.sql, "GREATEST(bonus_balance -") &&
			len(e.args) == 2 && e.args[0] == owner && e.args[1] == int64(5000) {
			foundDebit = true
		}
	}
	if !foundDebit {
		t.Errorf("revoke debit of 5000 for %s not recorded; execs=%d", owner, len(h.tx.execs))
	}

	// Grant flipped to forfeited inside the same tx.
	if len(h.grants.statuses) != 1 {
		t.Fatalf("status updates = %d, want 1", len(h.grants.statuses))
	}
	up := h.grants.statuses[0]
	if up.id != grant.ID || up.status != domain.BonusStatusForfeited || up.tx != pgx.Tx(h.tx) {
		t.Errorf("status update = {%s %s tx-match:%v}, want forfeited on harness tx",
			up.id, up.status, up.tx == pgx.Tx(h.tx))
	}

	// One forfeited ledger entry with the surrendered amount.
	if len(h.ledger.entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(h.ledger.entries))
	}
	e := h.ledger.entries[0]
	if e.TransactionType != domain.BonusLedgerForfeited || e.AmountCents != 5000 || e.GrantID != grant.ID {
		t.Errorf("ledger entry = %+v, want forfeited/5000/%s", e, grant.ID)
	}
}

func TestAdminRevokeRejectsMissingGrant(t *testing.T) {
	h := newBonusTestHarness(t)

	err := h.svc.AdminRevoke(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, domain.ErrGrantNotFound) {
		t.Fatalf("err = %v, want domain.ErrGrantNotFound", err)
	}
	if len(h.tx.execs) != 0 || len(h.grants.statuses) != 0 || len(h.ledger.entries) != 0 {
		t.Error("missing grant revoke must not move money or state")
	}
}

// countBonusDebits counts per-grant bonus_balance debits for one user.
func countBonusDebits(tx *recordingTx, userID uuid.UUID) int {
	n := 0
	for _, e := range tx.execs {
		if strings.Contains(e.sql, "GREATEST(bonus_balance -") &&
			len(e.args) == 2 && e.args[0] == userID {
			n++
		}
	}
	return n
}

// countLedgerOfType counts ledger entries of a given type.
func countLedgerOfType(l *fakeLedgerRepo, entryType string) int {
	n := 0
	for _, e := range l.entries {
		if e.TransactionType == entryType {
			n++
		}
	}
	return n
}

// TestAdminRevokeSecondCallDoesNotDoubleDebit models the RR-1 race: two revokes
// of the SAME grant both slip past the pre-transaction status check (the second
// read is stale), but the post-lock re-read inside the transaction observes the
// already-committed 'forfeited' status, so the second call must fail WITHOUT
// debiting bonus_balance again or writing a duplicate ledger row.
func TestAdminRevokeSecondCallDoesNotDoubleDebit(t *testing.T) {
	h := newBonusTestHarness(t)
	owner := uuid.New()
	grant := activeGrant(owner, uuid.New(), 5000)
	forfeited := *grant
	forfeited.Status = domain.BonusStatusForfeited

	// Request 1: pre-tx read and in-tx re-read both see 'active'.
	// Request 2: pre-tx read is stale ('active') but the in-tx re-read, taken
	// after the wallet lock, observes the committed 'forfeited' state.
	h.grants.getByIDQueue = []fakeGetByID{
		{grant: grant},      // req1 pre-tx
		{grant: grant},      // req1 in-tx
		{grant: grant},      // req2 pre-tx (stale)
		{grant: &forfeited}, // req2 in-tx (committed)
	}

	if err := h.svc.AdminRevoke(context.Background(), uuid.New(), grant.ID); err != nil {
		t.Fatalf("first AdminRevoke: %v", err)
	}
	err := h.svc.AdminRevoke(context.Background(), uuid.New(), grant.ID)
	if err == nil {
		t.Fatal("second AdminRevoke must fail, got nil")
	}
	if !errors.Is(err, domain.ErrBonusNotActive) {
		t.Fatalf("second AdminRevoke err = %v, want domain.ErrBonusNotActive", err)
	}

	if got := countBonusDebits(h.tx, owner); got != 1 {
		t.Errorf("bonus_balance debited %d times, want exactly 1", got)
	}
	if got := countLedgerOfType(h.ledger, domain.BonusLedgerForfeited); got != 1 {
		t.Errorf("forfeited ledger rows = %d, want exactly 1", got)
	}
	if len(h.grants.statuses) != 1 {
		t.Errorf("status updates = %d, want 1", len(h.grants.statuses))
	}
}

// TestAdminRevokeStaleActiveReadDoesNotForfeitWageredGrant covers the other
// half of RR-1: a grant that completed to 'wagered' between the pre-tx check
// and the row lock must NOT be regressed to 'forfeited' nor debited.
func TestAdminRevokeStaleActiveReadDoesNotForfeitWageredGrant(t *testing.T) {
	h := newBonusTestHarness(t)
	owner := uuid.New()
	grant := activeGrant(owner, uuid.New(), 5000)
	wagered := *grant
	wagered.Status = domain.BonusStatusWagered

	// Pre-tx read sees stale 'active'; in-tx re-read sees committed 'wagered'.
	h.grants.getByIDQueue = []fakeGetByID{
		{grant: grant},
		{grant: &wagered},
	}

	err := h.svc.AdminRevoke(context.Background(), uuid.New(), grant.ID)
	if !errors.Is(err, domain.ErrBonusNotActive) {
		t.Fatalf("err = %v, want domain.ErrBonusNotActive", err)
	}
	if len(h.tx.execs) != 0 {
		t.Errorf("wagered grant must not be debited, got %d wallet execs", len(h.tx.execs))
	}
	if len(h.grants.statuses) != 0 {
		t.Error("wagered grant status must not change")
	}
	if len(h.ledger.entries) != 0 {
		t.Error("wagered grant must not produce a ledger row")
	}
}

// TestAdminRevokeRejectsNonActiveGrant exercises the kept pre-tx fast path: a
// grant already non-active (e.g. completed to 'wagered') is rejected before any
// transaction is opened.
func TestAdminRevokeRejectsNonActiveGrant(t *testing.T) {
	h := newBonusTestHarness(t)
	grant := activeGrant(uuid.New(), uuid.New(), 5000)
	grant.Status = domain.BonusStatusWagered // completed: real money, not forfeitable
	h.grants.byID[grant.ID] = grant

	err := h.svc.AdminRevoke(context.Background(), uuid.New(), grant.ID)
	if !errors.Is(err, domain.ErrBonusNotActive) {
		t.Fatalf("err = %v, want domain.ErrBonusNotActive", err)
	}
	if len(h.grants.statuses) != 0 || len(h.tx.execs) != 0 {
		t.Error("non-active grant must stay untouched")
	}
}
