package domain

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Bonus type constants — pin exact string values
// ---------------------------------------------------------------------------

// TestBonusTypeConstants proves the four bonus type constants match the
// codes stored in bonus_definitions.code and used throughout the system.
func TestBonusTypeConstants(t *testing.T) {
	tests := []struct {
		constant string
		want     string
	}{
		{BonusTypeWelcome, "welcome"},
		{BonusTypeReload, "reload"},
		{BonusTypeCashback, "cashback"},
		{BonusTypeFreeSpins, "free_spins"},
	}
	for _, tt := range tests {
		if tt.constant != tt.want {
			t.Errorf("constant = %q, want %q", tt.constant, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Bonus status constants — pin exact string values
// ---------------------------------------------------------------------------

// TestBonusStatusConstants proves the five grant lifecycle status constants.
func TestBonusStatusConstants(t *testing.T) {
	tests := []struct {
		constant string
		want     string
	}{
		{BonusStatusActive, "active"},
		{BonusStatusWagered, "wagered"},
		{BonusStatusExpired, "expired"},
		{BonusStatusForfeited, "forfeited"},
		{BonusStatusWithdrawn, "withdrawn"},
	}
	for _, tt := range tests {
		if tt.constant != tt.want {
			t.Errorf("status constant = %q, want %q", tt.constant, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Bonus ledger transaction type constants
// ---------------------------------------------------------------------------

// TestBonusLedgerConstants proves the six ledger event type constants.
func TestBonusLedgerConstants(t *testing.T) {
	tests := []struct {
		constant string
		want     string
	}{
		{BonusLedgerGranted, "granted"},
		{BonusLedgerWagered, "wagered"},
		{BonusLedgerCompleted, "wagered_completed"},
		{BonusLedgerExpired, "expired"},
		{BonusLedgerForfeited, "forfeited"},
		{BonusLedgerWithdrawn, "withdrawn"},
	}
	for _, tt := range tests {
		if tt.constant != tt.want {
			t.Errorf("ledger constant = %q, want %q", tt.constant, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// BonusDefinition — struct initialization
// ---------------------------------------------------------------------------

// TestBonusDefinitionInitialization proves BonusDefinition fields are
// correctly assigned and the Config map survives assignment.
func TestBonusDefinitionInitialization(t *testing.T) {
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	def := BonusDefinition{
		ID:                  uuid.New(),
		Code:                BonusTypeWelcome,
		Name:                "Welcome Bonus",
		Description:         "100% match on first deposit",
		WageringMultiplier:  30,
		ExpiryDays:          30,
		MaxBonusAmountCents: 50000,
		MinDepositCents:     0,
		IsActive:            true,
		Config:              map[string]interface{}{"match_percent": 100},
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if def.Code != "welcome" {
		t.Errorf("Code = %q, want %q", def.Code, "welcome")
	}
	if def.WageringMultiplier != 30 {
		t.Errorf("WageringMultiplier = %d, want 30", def.WageringMultiplier)
	}
	if def.MaxBonusAmountCents != 50000 {
		t.Errorf("MaxBonusAmountCents = %d, want 50000", def.MaxBonusAmountCents)
	}
	mp, ok := def.Config["match_percent"]
	if !ok || mp != 100 {
		t.Errorf("Config[match_percent] = %v, want 100", mp)
	}
}

// ---------------------------------------------------------------------------
// BonusDefinition — JSON contract
// ---------------------------------------------------------------------------

// TestBonusDefinitionJSONContract proves the JSON wire format uses snake_case
// keys and preserves the Config map.
func TestBonusDefinitionJSONContract(t *testing.T) {
	def := BonusDefinition{
		ID:                  uuid.New(),
		Code:                BonusTypeReload,
		Name:                "Reload Bonus",
		Description:         "50% match",
		WageringMultiplier:  25,
		ExpiryDays:          14,
		MaxBonusAmountCents: 25000,
		MinDepositCents:     5000,
		IsActive:            true,
		Config:              map[string]interface{}{"match_percent": 50},
		CreatedAt:           time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:           time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	raw, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal BonusDefinition: %v", err)
	}
	for _, key := range []string{
		`"code":"reload"`,
		`"wagering_multiplier":25`,
		`"max_bonus_amount_cents":25000`,
		`"min_deposit_cents":5000`,
		`"is_active":true`,
		`"match_percent":50`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled BonusDefinition missing %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// BonusGrant — struct initialization
// ---------------------------------------------------------------------------

// TestBonusGrantInitialization proves BonusGrant fields are correctly assigned.
func TestBonusGrantInitialization(t *testing.T) {
	now := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	expires := now.Add(30 * 24 * time.Hour)
	grant := BonusGrant{
		ID:                    uuid.New(),
		UserID:                uuid.New(),
		BonusDefinitionID:     uuid.New(),
		BonusDefinitionCode:   "welcome",
		BonusAmountCents:      50000,
		WageringRequiredCents: 1500000,
		TotalWageredCents:     0,
		Status:                BonusStatusActive,
		GrantedBy:             "system",
		AdminID:               nil,
		ExpiresAt:             expires,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if grant.Status != BonusStatusActive {
		t.Errorf("Status = %q, want %q", grant.Status, BonusStatusActive)
	}
	if grant.BonusAmountCents != 50000 {
		t.Errorf("BonusAmountCents = %d, want 50000", grant.BonusAmountCents)
	}
	if grant.AdminID != nil {
		t.Errorf("AdminID = %v, want nil for system grant", grant.AdminID)
	}
}

// ---------------------------------------------------------------------------
// BonusGrant — JSON contract
// ---------------------------------------------------------------------------

// TestBonusGrantJSONContract proves the JSON wire format matches the API contract.
func TestBonusGrantJSONContract(t *testing.T) {
	grant := BonusGrant{
		ID:                    uuid.New(),
		UserID:                uuid.New(),
		BonusDefinitionID:     uuid.New(),
		BonusDefinitionCode:   "welcome",
		BonusAmountCents:      50000,
		WageringRequiredCents: 1500000,
		TotalWageredCents:     300000,
		Status:                BonusStatusActive,
		GrantedBy:             "system",
		ExpiresAt:             time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
		CreatedAt:             time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		UpdatedAt:             time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		t.Fatalf("marshal BonusGrant: %v", err)
	}
	for _, key := range []string{
		`"user_id"`,
		`"bonus_definition_id"`,
		`"bonus_definition_code":"welcome"`,
		`"bonus_amount_cents":50000`,
		`"wagering_required_cents":1500000`,
		`"total_wagered_cents":300000`,
		`"status":"active"`,
		`"granted_by":"system"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled BonusGrant missing %s: %s", key, raw)
		}
	}
}

// TestBonusGrantAdminGrantSerializesAdminID proves admin_id is serialized
// when present and omitted when nil.
func TestBonusGrantAdminGrantSerializesAdminID(t *testing.T) {
	adminID := uuid.New()
	grant := BonusGrant{
		AdminID:   &adminID,
		GrantedBy: "admin",
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"admin_id"`) {
		t.Error("admin_id should be present when non-nil")
	}

	grant.AdminID = nil
	raw, err = json.Marshal(grant)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), `"admin_id"`) {
		t.Error("admin_id should be omitted when nil (omitempty)")
	}
}

// ---------------------------------------------------------------------------
// WageringProgressPct — edge cases
// ---------------------------------------------------------------------------

// TestWageringProgressPctZeroRequired proves that when wagering_required is 0
// (e.g. cashback bonuses), progress returns 100%.
func TestWageringProgressPctZeroRequired(t *testing.T) {
	grant := BonusGrant{WageringRequiredCents: 0, TotalWageredCents: 0}
	if got := grant.WageringProgressPct(); got != 100.0 {
		t.Errorf("WageringProgressPct() = %f, want 100.0 (zero wagering required)", got)
	}
}

// TestWageringProgressPctZeroWagered proves that with wagering required but
// nothing wagered yet, progress is 0%.
func TestWageringProgressPctZeroWagered(t *testing.T) {
	grant := BonusGrant{WageringRequiredCents: 10000, TotalWageredCents: 0}
	if got := grant.WageringProgressPct(); got != 0.0 {
		t.Errorf("WageringProgressPct() = %f, want 0.0", got)
	}
}

// TestWageringProgressPctPartialProgress proves a partial wagering progress
// calculation (e.g. 3000/10000 = 30%).
func TestWageringProgressPctPartialProgress(t *testing.T) {
	grant := BonusGrant{WageringRequiredCents: 10000, TotalWageredCents: 3000}
	want := 30.0
	got := grant.WageringProgressPct()
	if math.Abs(got-want) > 0.001 {
		t.Errorf("WageringProgressPct() = %f, want %f", got, want)
	}
}

// TestWageringProgressPctExactCompletion proves that when total_wagered
// equals wagering_required, progress is exactly 100%.
func TestWageringProgressPctExactCompletion(t *testing.T) {
	grant := BonusGrant{WageringRequiredCents: 5000, TotalWageredCents: 5000}
	if got := grant.WageringProgressPct(); got != 100.0 {
		t.Errorf("WageringProgressPct() = %f, want 100.0", got)
	}
}

// TestWageringProgressPctOverflowCapped proves that when total_wagered
// exceeds wagering_required, progress is capped at 100% (never exceeds).
func TestWageringProgressPctOverflowCapped(t *testing.T) {
	grant := BonusGrant{WageringRequiredCents: 5000, TotalWageredCents: 99999}
	if got := grant.WageringProgressPct(); got != 100.0 {
		t.Errorf("WageringProgressPct() = %f, want 100.0 (capped at 100%%)", got)
	}
}

// TestWageringProgressPctHighPrecision proves precision for large values
// (e.g. 1/3000 wagering).
func TestWageringProgressPctHighPrecision(t *testing.T) {
	grant := BonusGrant{WageringRequiredCents: 3000, TotalWageredCents: 1}
	got := grant.WageringProgressPct()
	want := 100.0 / 3000.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("WageringProgressPct() = %f, want ~%f", got, want)
	}
}

// TestWageringProgressPctTableDriven consolidates all edge cases in one
// table-driven test for easy scanning.
func TestWageringProgressPctTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		required int64
		wagered  int64
		want     float64
	}{
		{"zero required", 0, 0, 100.0},
		{"zero wagered", 10000, 0, 0.0},
		{"partial 30%", 10000, 3000, 30.0},
		{"partial 50%", 10000, 5000, 50.0},
		{"partial 99%", 10000, 9900, 99.0},
		{"exact 100%", 5000, 5000, 100.0},
		{"overflow capped", 5000, 99999, 100.0},
		{"one cent wagered", 3000, 1, 100.0 / 3000.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grant := BonusGrant{
				WageringRequiredCents: tt.required,
				TotalWageredCents:     tt.wagered,
			}
			got := grant.WageringProgressPct()
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("WageringProgressPct(%d, %d) = %f, want %f",
					tt.wagered, tt.required, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// BonusLedgerEntry — struct initialization and JSON contract
// ---------------------------------------------------------------------------

// TestBonusLedgerEntryInitialization proves BonusLedgerEntry fields are
// correctly assigned.
func TestBonusLedgerEntryInitialization(t *testing.T) {
	entry := BonusLedgerEntry{
		ID:              uuid.New(),
		GrantID:         uuid.New(),
		TransactionType: BonusLedgerGranted,
		AmountCents:     50000,
		ReferenceType:   "system",
		ReferenceID:     uuid.New(),
		CreatedAt:       time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	}

	if entry.TransactionType != BonusLedgerGranted {
		t.Errorf("TransactionType = %q, want %q", entry.TransactionType, BonusLedgerGranted)
	}
	if entry.AmountCents != 50000 {
		t.Errorf("AmountCents = %d, want 50000", entry.AmountCents)
	}
}

// TestBonusLedgerEntryJSONContract proves the JSON wire format matches the API.
func TestBonusLedgerEntryJSONContract(t *testing.T) {
	entry := BonusLedgerEntry{
		ID:              uuid.New(),
		GrantID:         uuid.New(),
		TransactionType: BonusLedgerWagered,
		AmountCents:     2500,
		ReferenceType:   "bet",
		ReferenceID:     uuid.New(),
		CreatedAt:       time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal BonusLedgerEntry: %v", err)
	}
	for _, key := range []string{
		`"grant_id"`,
		`"transaction_type":"wagered"`,
		`"amount_cents":2500`,
		`"reference_type":"bet"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled BonusLedgerEntry missing %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// BonusClaimResult — JSON contract
// ---------------------------------------------------------------------------

// TestBonusClaimResultJSONContract proves the claim response format.
func TestBonusClaimResultJSONContract(t *testing.T) {
	result := BonusClaimResult{
		GrantID:               uuid.New(),
		BonusAmountCents:      50000,
		WageringRequiredCents: 1500000,
		ExpiresAt:             "2026-09-18T00:00:00Z",
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal BonusClaimResult: %v", err)
	}
	for _, key := range []string{
		`"grant_id"`,
		`"bonus_amount_cents":50000`,
		`"wagering_required_cents":1500000`,
		`"expires_at":"2026-09-18T00:00:00Z"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled BonusClaimResult missing %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// BonusActiveResponse — JSON contract
// ---------------------------------------------------------------------------

// TestBonusActiveResponseJSONContract proves the active bonuses response format
// includes wagering_progress_pct.
func TestBonusActiveResponseJSONContract(t *testing.T) {
	resp := BonusActiveResponse{
		GrantID:               uuid.New(),
		Code:                  "welcome",
		Name:                  "Welcome Bonus",
		BonusAmountCents:      50000,
		WageringRequiredCents: 1500000,
		TotalWageredCents:     450000,
		WageringProgressPct:   30.0,
		ExpiresAt:             "2026-09-18T00:00:00Z",
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal BonusActiveResponse: %v", err)
	}
	for _, key := range []string{
		`"code":"welcome"`,
		`"wagering_progress_pct":30`,
		`"total_wagered_cents":450000`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled BonusActiveResponse missing %s: %s", key, raw)
		}
	}
}

// ---------------------------------------------------------------------------
// BonusHandler interface — compile-time compliance check
// ---------------------------------------------------------------------------

// stubHandler is a minimal implementation of BonusHandler used only to
// verify the interface contract compiles.
type stubHandler struct{}

func (stubHandler) ValidateClaim(_ context.Context, _ uuid.UUID, _ *BonusDefinition, _ map[string]interface{}) error {
	return nil
}
func (stubHandler) CalculateBonus(_ context.Context, _ *BonusDefinition, _ map[string]interface{}) (int64, error) {
	return 0, nil
}

// TestBonusHandlerInterfaceCompliance proves that stubHandler satisfies the
// BonusHandler interface at compile time.
func TestBonusHandlerInterfaceCompliance(t *testing.T) {
	var _ BonusHandler = stubHandler{}
}
