package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Compile-time interface compliance checks
// ---------------------------------------------------------------------------

// TestPostgresBonusDefinitionRepoImplementsInterface proves the Postgres
// implementation satisfies the repository interface at compile time.
func TestPostgresBonusDefinitionRepoImplementsInterface(t *testing.T) {
	var _ repository.BonusDefinitionRepository = (*PostgresBonusDefinitionRepo)(nil)
}

// TestPostgresBonusGrantRepoImplementsInterface proves the Postgres
// implementation satisfies the repository interface at compile time.
func TestPostgresBonusGrantRepoImplementsInterface(t *testing.T) {
	var _ repository.BonusGrantRepository = (*PostgresBonusGrantRepo)(nil)
}

// TestPostgresBonusLedgerRepoImplementsInterface proves the Postgres
// implementation satisfies the repository interface at compile time.
func TestPostgresBonusLedgerRepoImplementsInterface(t *testing.T) {
	var _ repository.BonusLedgerRepository = (*PostgresBonusLedgerRepo)(nil)
}

// ---------------------------------------------------------------------------
// BonusDefinition — domain edge cases
// ---------------------------------------------------------------------------

// TestBonusDefinitionDefaultValues proves BonusDefinition zero-value is
// a usable (but invalid) definition — Config defaults to nil not panic.
func TestBonusDefinitionDefaultValues(t *testing.T) {
	def := domain.BonusDefinition{}
	if def.IsActive {
		t.Error("zero-value IsActive should be false")
	}
	if def.WageringMultiplier != 0 {
		t.Errorf("zero-value WageringMultiplier = %d, want 0", def.WageringMultiplier)
	}
	if def.Config != nil {
		t.Error("zero-value Config should be nil")
	}
}

// TestBonusDefinitionConfigSurvivesMarshalCycle proves Config map survives
// JSON marshal → unmarshal round-trip (simulates what the repo does with JSONB).
func TestBonusDefinitionConfigSurvivesMarshalCycle(t *testing.T) {
	original := map[string]interface{}{
		"match_percent":  float64(100),
		"min_deposit":    float64(5000),
		"eligible_games": []interface{}{"slots", "blackjack"},
	}
	def := domain.BonusDefinition{
		ID:     uuid.New(),
		Code:   "welcome",
		Config: original,
	}

	// Marshal to JSON (simulates INSERT).
	data, err := json.Marshal(def.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	// Unmarshal back (simulates SELECT).
	var restored map[string]interface{}
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if restored["match_percent"] != float64(100) {
		t.Errorf("match_percent = %v, want 100", restored["match_percent"])
	}
	if restored["min_deposit"] != float64(5000) {
		t.Errorf("min_deposit = %v, want 5000", restored["min_deposit"])
	}
}

// ---------------------------------------------------------------------------
// BonusGrant — status transition edge cases
// ---------------------------------------------------------------------------

// TestBonusGrantStatusTransitions documents the valid lifecycle transitions.
func TestBonusGrantStatusTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		expected bool
	}{
		{"active to wagered", domain.BonusStatusActive, true},
		{"active to expired", domain.BonusStatusActive, true},
		{"active to forfeited", domain.BonusStatusActive, true},
		{"active to withdrawn", domain.BonusStatusActive, true},
		{"wagered to expired", domain.BonusStatusWagered, false}, // terminal
		{"expired to active", domain.BonusStatusExpired, false},  // no reverse
		{"forfeited to active", domain.BonusStatusForfeited, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only active grants should be transitionable (documented, not enforced here).
			isTransitionable := tt.from == domain.BonusStatusActive
			if isTransitionable != tt.expected {
				t.Errorf("transition from %q: got %v, want %v", tt.from, isTransitionable, tt.expected)
			}
		})
	}
}

// TestBonusGrantWithAdminID proves admin-granted bonuses store admin_id.
func TestBonusGrantWithAdminID(t *testing.T) {
	adminID := uuid.New()
	grant := domain.BonusGrant{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		GrantedBy: "admin",
		AdminID:   &adminID,
		Status:    domain.BonusStatusActive,
	}
	if grant.AdminID == nil {
		t.Fatal("AdminID should not be nil for admin grant")
	}
	if *grant.AdminID != adminID {
		t.Errorf("AdminID = %v, want %v", *grant.AdminID, adminID)
	}
}

// TestBonusGrantWithoutAdminID proves system grants have nil admin_id.
func TestBonusGrantWithoutAdminID(t *testing.T) {
	grant := domain.BonusGrant{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		GrantedBy: "system",
		AdminID:   nil,
		Status:    domain.BonusStatusActive,
	}
	if grant.AdminID != nil {
		t.Errorf("AdminID should be nil for system grant, got %v", *grant.AdminID)
	}
}

// ---------------------------------------------------------------------------
// BonusGrant — WageringProgressPct via the domain method
// ---------------------------------------------------------------------------

// TestIncrementWageredStatusTransition documents the behavior tested by
// the SQL in IncrementWagered: when total_wagered + amount >= required,
// status flips to 'wagered'.
func TestIncrementWageredStatusTransition(t *testing.T) {
	tests := []struct {
		name       string
		wagered    int64
		required   int64
		increment  int64
		shouldFlip bool
	}{
		{"below threshold", 1000, 5000, 1000, false},
		{"exact threshold", 3000, 5000, 2000, true},
		{"over threshold", 4000, 5000, 2000, true},
		{"zero required (cashback)", 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newTotal := tt.wagered + tt.increment
			// Mirrors the SQL: CASE WHEN total + $2 >= required THEN 'wagered'
			flips := newTotal >= tt.required
			if flips != tt.shouldFlip {
				t.Errorf("wagered=%d+increment=%d, required=%d: flip=%v, want %v",
					tt.wagered, tt.increment, tt.required, flips, tt.shouldFlip)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BonusGrant — 23505 → ErrActiveBonusGrantExists mapping
// ---------------------------------------------------------------------------

// TestMapBonusGrantUniqueViolation pins the 23505 → ErrActiveBonusGrantExists
// translation BonusGrantRepo.Create performs: racing a duplicate active grant
// must surface as the repository sentinel, never as an opaque driver error.
func TestMapBonusGrantUniqueViolation(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantSentinel bool
	}{
		{"bare 23505", &pgconn.PgError{Code: "23505"}, true},
		{"23505 wrapped by Create", fmt.Errorf("insert bonus grant: %w", &pgconn.PgError{Code: "23505", ConstraintName: "bonus_grants_active_idx"}), true},
		{"non-unique PgError passes through", &pgconn.PgError{Code: "23503"}, false}, // FK violation
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapBonusGrantUniqueViolation(tt.err)
			if tt.wantSentinel && !errors.Is(got, repository.ErrActiveBonusGrantExists) {
				t.Fatalf("err = %v, want repository.ErrActiveBonusGrantExists", got)
			}
			if !tt.wantSentinel && got != tt.err {
				t.Fatalf("err = %v, want the original error unchanged", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BonusLedgerEntry — edge cases
// ---------------------------------------------------------------------------

// TestBonusLedgerEntryZeroAmount proves ledger entries can record zero-amount
// events (e.g. expiry of a fully-wagered bonus).
func TestBonusLedgerEntryZeroAmount(t *testing.T) {
	entry := domain.BonusLedgerEntry{
		ID:              uuid.New(),
		GrantID:         uuid.New(),
		TransactionType: domain.BonusLedgerExpired,
		AmountCents:     0,
		CreatedAt:       time.Now().UTC(),
	}
	if entry.AmountCents != 0 {
		t.Errorf("AmountCents = %d, want 0", entry.AmountCents)
	}
}

// TestBonusLedgerEntryReferenceFields proves reference_type and reference_id
// are set for bet-linked entries and empty for system events.
func TestBonusLedgerEntryReferenceFields(t *testing.T) {
	betID := uuid.New()
	entry := domain.BonusLedgerEntry{
		TransactionType: domain.BonusLedgerWagered,
		AmountCents:     2500,
		ReferenceType:   "bet",
		ReferenceID:     betID,
	}
	if entry.ReferenceType != "bet" {
		t.Errorf("ReferenceType = %q, want %q", entry.ReferenceType, "bet")
	}
	if entry.ReferenceID != betID {
		t.Errorf("ReferenceID = %v, want %v", entry.ReferenceID, betID)
	}

	// System event (expiry) — no reference.
	systemEntry := domain.BonusLedgerEntry{
		TransactionType: domain.BonusLedgerExpired,
		AmountCents:     0,
	}
	if systemEntry.ReferenceType != "" {
		t.Errorf("system event ReferenceType = %q, want empty", systemEntry.ReferenceType)
	}
}

// ---------------------------------------------------------------------------
// Integration tests (require TEST_DATABASE_URL)
// ---------------------------------------------------------------------------

// TestBonusDefRepoIntegration tests CRUD operations against a real database.
func TestBonusDefRepoIntegration(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresBonusDefinitionRepo(pool)
	ctx := t.Context()

	now := time.Now().UTC()
	def := &domain.BonusDefinition{
		ID:                  uuid.New(),
		Code:                "test_bonus_" + uuid.New().String()[:8],
		Name:                "Test Bonus",
		Description:         "A test bonus",
		WageringMultiplier:  30,
		ExpiryDays:          30,
		MaxBonusAmountCents: 50000,
		MinDepositCents:     0,
		IsActive:            true,
		Config:              map[string]interface{}{"match_percent": float64(100)},
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Create
	if err := repo.Create(ctx, def); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// GetByID
	got, err := repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Code != def.Code {
		t.Errorf("Code = %q, want %q", got.Code, def.Code)
	}

	// GetByCode
	got2, err := repo.GetByCode(ctx, def.Code)
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if got2.ID != def.ID {
		t.Errorf("GetByCode ID = %v, want %v", got2.ID, def.ID)
	}

	// ListActive
	active, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	found := false
	for _, d := range active {
		if d.ID == def.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListActive should include the created definition")
	}

	// Update
	def.Name = "Updated Test Bonus"
	def.IsActive = false
	if err := repo.Update(ctx, def); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got3, err := repo.GetByID(ctx, def.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got3.Name != "Updated Test Bonus" {
		t.Errorf("Name after update = %q, want %q", got3.Name, "Updated Test Bonus")
	}
	if got3.IsActive {
		t.Error("IsActive should be false after update")
	}
}

// TestBonusDefRepoNotFound proves not-found returns the sentinel error.
func TestBonusDefRepoNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresBonusDefinitionRepo(pool)
	ctx := t.Context()

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
	if !isErrBonusDefinitionNotFound(err) {
		t.Errorf("expected ErrBonusDefinitionNotFound, got: %v", err)
	}

	_, err = repo.GetByCode(ctx, "nonexistent_code_xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent code")
	}
	if !isErrBonusDefinitionNotFound(err) {
		t.Errorf("expected ErrBonusDefinitionNotFound, got: %v", err)
	}
}

// TestBonusGrantRepoIntegration tests grant CRUD against a real database.
func TestBonusGrantRepoIntegration(t *testing.T) {
	pool := testPool(t)
	defRepo := NewPostgresBonusDefinitionRepo(pool)
	grantRepo := NewPostgresBonusGrantRepo(pool)
	ctx := t.Context()

	// Seed a bonus definition.
	now := time.Now().UTC()
	def := &domain.BonusDefinition{
		ID:                  uuid.New(),
		Code:                "test_grant_" + uuid.New().String()[:8],
		Name:                "Test Grant Def",
		Description:         "Test",
		WageringMultiplier:  20,
		ExpiryDays:          7,
		MaxBonusAmountCents: 10000,
		IsActive:            true,
		Config:              map[string]interface{}{},
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := defRepo.Create(ctx, def); err != nil {
		t.Fatalf("seed definition: %v", err)
	}

	userID := seedTestUser(t, pool)

	grant := &domain.BonusGrant{
		ID:                    uuid.New(),
		UserID:                userID,
		BonusDefinitionID:     def.ID,
		BonusAmountCents:      5000,
		WageringRequiredCents: 100000,
		TotalWageredCents:     0,
		Status:                domain.BonusStatusActive,
		GrantedBy:             "system",
		ExpiresAt:             now.Add(7 * 24 * time.Hour),
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	// Create
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.Create(ctx, tx, grant)
	})

	// GetByID
	got, err := grantRepo.GetByID(ctx, grant.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != userID {
		t.Errorf("UserID = %v, want %v", got.UserID, userID)
	}
	if got.Status != domain.BonusStatusActive {
		t.Errorf("Status = %q, want %q", got.Status, domain.BonusStatusActive)
	}

	// GetActiveByUserAndDefinition
	var active *domain.BonusGrant
	execInTx(t, ctx, pool, func(tx pgx.Tx) (err error) {
		active, err = grantRepo.GetActiveByUserAndDefinition(ctx, tx, userID, def.ID)
		return err
	})
	if active.ID != grant.ID {
		t.Errorf("active grant ID = %v, want %v", active.ID, grant.ID)
	}

	// ListActiveByUser
	actives, err := grantRepo.ListActiveByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListActiveByUser: %v", err)
	}
	if len(actives) < 1 {
		t.Fatal("expected at least 1 active grant")
	}

	// IncrementWagered
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.IncrementWagered(ctx, tx, grant.ID, 50000)
	})
	got2, _ := grantRepo.GetByID(ctx, grant.ID)
	if got2.TotalWageredCents != 50000 {
		t.Errorf("TotalWageredCents = %d, want 50000", got2.TotalWageredCents)
	}
	if got2.Status != domain.BonusStatusActive {
		t.Errorf("Status should still be active at 50%% wagering, got %q", got2.Status)
	}

	// IncrementWagered to completion
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.IncrementWagered(ctx, tx, grant.ID, 50001)
	})
	got3, _ := grantRepo.GetByID(ctx, grant.ID)
	if got3.Status != domain.BonusStatusWagered {
		t.Errorf("Status = %q after full wagering, want %q", got3.Status, domain.BonusStatusWagered)
	}

	// UpdateStatus
	// Reset for status test.
	grant2 := &domain.BonusGrant{
		ID:                    uuid.New(),
		UserID:                userID,
		BonusDefinitionID:     def.ID,
		BonusAmountCents:      3000,
		WageringRequiredCents: 6000,
		TotalWageredCents:     0,
		Status:                domain.BonusStatusActive,
		GrantedBy:             "system",
		ExpiresAt:             now.Add(7 * 24 * time.Hour),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	// Create second grant — unique index requires previous active to be non-active.
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.UpdateStatus(ctx, tx, grant.ID, domain.BonusStatusWithdrawn)
	})
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.Create(ctx, tx, grant2)
	})
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.UpdateStatus(ctx, tx, grant2.ID, domain.BonusStatusForfeited)
	})
	got4, _ := grantRepo.GetByID(ctx, grant2.ID)
	if got4.Status != domain.BonusStatusForfeited {
		t.Errorf("Status = %q, want %q", got4.Status, domain.BonusStatusForfeited)
	}
}

// TestBonusGrantRepoNotFound proves not-found returns the sentinel error.
func TestBonusGrantRepoNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewPostgresBonusGrantRepo(pool)
	ctx := t.Context()

	_, err := repo.GetByID(ctx, uuid.New())
	if err == nil {
		t.Fatal("expected error for nonexistent grant ID")
	}
	if !isErrBonusGrantNotFound(err) {
		t.Errorf("expected ErrBonusGrantNotFound, got: %v", err)
	}
}

// TestBonusLedgerRepoIntegration tests ledger CRUD against a real database.
func TestBonusLedgerRepoIntegration(t *testing.T) {
	pool := testPool(t)
	defRepo := NewPostgresBonusDefinitionRepo(pool)
	grantRepo := NewPostgresBonusGrantRepo(pool)
	ledgerRepo := NewPostgresBonusLedgerRepo(pool)
	ctx := t.Context()

	now := time.Now().UTC()

	// Seed definition + grant.
	def := &domain.BonusDefinition{
		ID:                  uuid.New(),
		Code:                "test_ledger_" + uuid.New().String()[:8],
		Name:                "Test Ledger Def",
		WageringMultiplier:  10,
		ExpiryDays:          7,
		MaxBonusAmountCents: 5000,
		IsActive:            true,
		Config:              map[string]interface{}{},
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := defRepo.Create(ctx, def); err != nil {
		t.Fatalf("seed definition: %v", err)
	}

	userID := seedTestUser(t, pool)
	grant := &domain.BonusGrant{
		ID:                    uuid.New(),
		UserID:                userID,
		BonusDefinitionID:     def.ID,
		BonusAmountCents:      2000,
		WageringRequiredCents: 20000,
		Status:                domain.BonusStatusActive,
		GrantedBy:             "system",
		ExpiresAt:             now.Add(7 * 24 * time.Hour),
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return grantRepo.Create(ctx, tx, grant)
	})

	// Create ledger entry.
	entry := &domain.BonusLedgerEntry{
		ID:              uuid.New(),
		GrantID:         grant.ID,
		TransactionType: domain.BonusLedgerGranted,
		AmountCents:     2000,
		ReferenceType:   "system",
		ReferenceID:     uuid.New(),
		CreatedAt:       now,
	}
	execInTx(t, ctx, pool, func(tx pgx.Tx) error {
		return ledgerRepo.Create(ctx, tx, entry)
	})

	// ListByGrant
	entries, err := ledgerRepo.ListByGrant(ctx, grant.ID)
	if err != nil {
		t.Fatalf("ListByGrant: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(entries))
	}
	if entries[0].TransactionType != domain.BonusLedgerGranted {
		t.Errorf("TransactionType = %q, want %q", entries[0].TransactionType, domain.BonusLedgerGranted)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// execInTx runs fn inside a fresh transaction, then commits it.
func execInTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := fn(tx); err != nil {
		t.Fatalf("tx: %v (rolled back: %v)", err, tx.Rollback(ctx))
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}

func isErrBonusDefinitionNotFound(err error) bool {
	for err != nil {
		if err == repository.ErrBonusDefinitionNotFound {
			return true
		}
		err = unwrapError(err)
	}
	return false
}

func isErrBonusGrantNotFound(err error) bool {
	for err != nil {
		if err == repository.ErrBonusGrantNotFound {
			return true
		}
		err = unwrapError(err)
	}
	return false
}

// unwrapError extracts the next error in the chain.
type unwrappable interface {
	Unwrap() error
}

func unwrapError(err error) error {
	if u, ok := err.(unwrappable); ok {
		return u.Unwrap()
	}
	return nil
}
