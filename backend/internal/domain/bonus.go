package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Bonus type constants
// ---------------------------------------------------------------------------

const (
	BonusTypeWelcome   = "welcome"
	BonusTypeReload    = "reload"
	BonusTypeCashback  = "cashback"
	BonusTypeFreeSpins = "free_spins"
)

// ---------------------------------------------------------------------------
// Bonus grant status constants
// ---------------------------------------------------------------------------

const (
	BonusStatusActive    = "active"
	BonusStatusWagered   = "wagered"
	BonusStatusExpired   = "expired"
	BonusStatusForfeited = "forfeited"
	BonusStatusWithdrawn = "withdrawn"
)

// ---------------------------------------------------------------------------
// Bonus grant origin constants (granted_by column values)
// ---------------------------------------------------------------------------

const (
	GrantedBySystem = "system"
	GrantedByAdmin  = "admin"
)

// ---------------------------------------------------------------------------
// Bonus ledger transaction types
// ---------------------------------------------------------------------------

const (
	BonusLedgerGranted   = "granted"
	BonusLedgerWagered   = "wagered"
	BonusLedgerCompleted = "wagered_completed"
	BonusLedgerExpired   = "expired"
	BonusLedgerForfeited = "forfeited"
	BonusLedgerWithdrawn = "withdrawn"
)

// ---------------------------------------------------------------------------
// Bonus sentinel errors
// ---------------------------------------------------------------------------

var (
	ErrBonusNotFound       = errors.New("bonus definition not found")
	ErrBonusNotActive      = errors.New("bonus is not active")
	ErrActiveBonusExists   = errors.New("active bonus of this type already exists")
	ErrBonusExpired        = errors.New("bonus has expired")
	ErrBonusNotClaimable   = errors.New("bonus is not claimable")
	ErrGrantNotFound       = errors.New("bonus grant not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// ---------------------------------------------------------------------------
// BonusDefinition — what bonuses can be offered
// ---------------------------------------------------------------------------

// BonusDefinition represents a configurable bonus offer.
// MaxBonusAmountCents and MinDepositCents are in integer cents.
type BonusDefinition struct {
	ID                  uuid.UUID              `json:"id"`
	Code                string                 `json:"code"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	WageringMultiplier  int                    `json:"wagering_multiplier"`
	ExpiryDays          int                    `json:"expiry_days"`
	MaxBonusAmountCents int64                  `json:"max_bonus_amount_cents"`
	MinDepositCents     int64                  `json:"min_deposit_cents"`
	IsActive            bool                   `json:"is_active"`
	Config              map[string]interface{} `json:"config"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// BonusGrant — a user's bonus instance
// ---------------------------------------------------------------------------

// BonusGrant represents a user's active or historical bonus.
// All monetary amounts are in integer cents.
type BonusGrant struct {
	ID                    uuid.UUID  `json:"id"`
	UserID                uuid.UUID  `json:"user_id"`
	BonusDefinitionID     uuid.UUID  `json:"bonus_definition_id"`
	BonusDefinitionCode   string     `json:"bonus_definition_code,omitempty"` // joined from definition
	BonusAmountCents      int64      `json:"bonus_amount_cents"`
	WageringRequiredCents int64      `json:"wagering_required_cents"`
	TotalWageredCents     int64      `json:"total_wagered_cents"`
	Status                string     `json:"status"`
	GrantedBy             string     `json:"granted_by"`
	AdminID               *uuid.UUID `json:"admin_id,omitempty"`
	ExpiresAt             time.Time  `json:"expires_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// WageringProgressPct returns 0..100 based on total_wagered / wagering_required.
// Returns 100 when wagering is zero (no wagering required) or when progress
// exceeds 100%.
func (g *BonusGrant) WageringProgressPct() float64 {
	if g.WageringRequiredCents == 0 {
		return 100.0
	}
	pct := float64(g.TotalWageredCents) / float64(g.WageringRequiredCents) * 100.0
	if pct > 100 {
		pct = 100
	}
	return pct
}

// ---------------------------------------------------------------------------
// BonusLedgerEntry — immutable audit trail record
// ---------------------------------------------------------------------------

// BonusLedgerEntry represents a single event in the bonus lifecycle audit trail.
type BonusLedgerEntry struct {
	ID              uuid.UUID `json:"id"`
	GrantID         uuid.UUID `json:"grant_id"`
	TransactionType string    `json:"transaction_type"`
	AmountCents     int64     `json:"amount_cents"`
	ReferenceType   string    `json:"reference_type"`
	ReferenceID     uuid.UUID `json:"reference_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// BonusHandler — interface for type-specific bonus logic
// ---------------------------------------------------------------------------

// BonusHandler defines the contract for type-specific bonus validation and
// calculation. Each bonus type (welcome, reload, cashback, free_spins)
// implements this interface.
type BonusHandler interface {
	ValidateClaim(ctx context.Context, userID uuid.UUID, def *BonusDefinition, params map[string]interface{}) error
	CalculateBonus(ctx context.Context, def *BonusDefinition, params map[string]interface{}) (int64, error)
}

// ---------------------------------------------------------------------------
// Bonus response types (API layer uses these)
// ---------------------------------------------------------------------------

// BonusClaimResult is returned after a successful bonus claim.
type BonusClaimResult struct {
	GrantID               uuid.UUID `json:"grant_id"`
	BonusAmountCents      int64     `json:"bonus_amount_cents"`
	WageringRequiredCents int64     `json:"wagering_required_cents"`
	ExpiresAt             string    `json:"expires_at"`
}

// BonusActiveResponse is the response for GET /bonuses/active.
type BonusActiveResponse struct {
	GrantID               uuid.UUID `json:"grant_id"`
	Code                  string    `json:"code"`
	Name                  string    `json:"name"`
	BonusAmountCents      int64     `json:"bonus_amount_cents"`
	WageringRequiredCents int64     `json:"wagering_required_cents"`
	TotalWageredCents     int64     `json:"total_wagered_cents"`
	WageringProgressPct   float64   `json:"wagering_progress_pct"`
	ExpiresAt             string    `json:"expires_at"`
}
