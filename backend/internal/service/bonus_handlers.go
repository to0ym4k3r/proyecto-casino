package service

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Type-specific claim inputs
//
// Handlers read claim parameters from the params map posted by the client:
//   - welcome / reload / cashback read "deposit_cents" / "loss_amount_cents"
//   - free_spins optionally reads "game_id" when config pins a game
// ---------------------------------------------------------------------------

const (
	paramDepositCents    = "deposit_cents"
	paramLossAmountCents = "loss_amount_cents"
	paramGameID          = "game_id"

	configMatchPercent    = "match_percent"
	configCashbackPercent = "cashback_percent"
	configMinDepositCents = "min_deposit_cents"
	configGameID          = "game_id"
)

// paramInt64 extracts a non-negative integer amount from params. JSONB
// configs unmarshal numbers as float64, so numeric variants are normalized.
func paramInt64(params map[string]interface{}, key string) (int64, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if n < 0 || n != float64(int64(n)) {
			return 0, false
		}
		return int64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return int64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// configPercent reads a percentage value from a definition's JSONB config.
func configPercent(config map[string]interface{}, key string) (float64, bool) {
	v, ok := config[key]
	if !ok {
		return 0, false
	}
	pct, ok := v.(float64)
	if !ok || pct < 0 {
		return 0, false
	}
	return pct, true
}

// percentOfCents returns amount*pct/100 rounded half-up to whole cents.
// Percentages come from admin-managed JSONB config as float64; amounts stay
// in integer cents so results remain exact for realistic magnitudes.
func percentOfCents(amountCents int64, pct float64) int64 {
	return int64(float64(amountCents)*pct/100 + 0.5)
}

// cappedMatch computes min(amount*pct/100, maxBonusCents).
func cappedMatch(amountCents int64, pct float64, maxBonusCents int64) int64 {
	bonus := percentOfCents(amountCents, pct)
	if bonus > maxBonusCents {
		return maxBonusCents
	}
	return bonus
}

// effectiveMinDeposit returns the largest minimum-deposit requirement between
// the definition column and its JSONB config override.
func effectiveMinDeposit(def *domain.BonusDefinition) int64 {
	minDeposit := def.MinDepositCents
	if cfgMin, ok := paramInt64(def.Config, configMinDepositCents); ok && cfgMin > minDeposit {
		minDeposit = cfgMin
	}
	return minDeposit
}

// ---------------------------------------------------------------------------
// WelcomeHandler — first-deposit match bonus
// ---------------------------------------------------------------------------

// WelcomeHandler implements the welcome bonus: a percentage match on the
// player's deposit, capped by the definition maximum.
type WelcomeHandler struct{}

// ValidateClaim requires a positive deposit meeting the definition minimum.
// The one-active-grant rule is enforced centrally by BonusService.ClaimBonus.
func (WelcomeHandler) ValidateClaim(_ context.Context, _ uuid.UUID, def *domain.BonusDefinition, params map[string]interface{}) error {
	deposit, ok := paramInt64(params, paramDepositCents)
	if !ok || deposit <= 0 {
		return fmt.Errorf("%w: positive deposit_cents required", domain.ErrBonusNotClaimable)
	}
	if min := effectiveMinDeposit(def); deposit < min {
		return fmt.Errorf("%w: deposit %d below minimum %d", domain.ErrBonusNotClaimable, deposit, min)
	}
	return nil
}

// CalculateBonus returns min(deposit*match_percent/100, max_bonus_amount_cents).
func (WelcomeHandler) CalculateBonus(_ context.Context, def *domain.BonusDefinition, params map[string]interface{}) (int64, error) {
	deposit, _ := paramInt64(params, paramDepositCents)
	pct, ok := configPercent(def.Config, configMatchPercent)
	if !ok {
		return 0, fmt.Errorf("welcome bonus config missing %s", configMatchPercent)
	}
	return cappedMatch(deposit, pct, def.MaxBonusAmountCents), nil
}

// ---------------------------------------------------------------------------
// ReloadHandler — repeat-deposit match bonus
// ---------------------------------------------------------------------------

// ReloadHandler implements the reload bonus: like welcome but typically a
// lower match on later deposits, gated by a higher minimum deposit.
type ReloadHandler struct{}

// ValidateClaim applies the same deposit rules as the welcome handler; the
// effective minimum usually comes from the definition config.
func (ReloadHandler) ValidateClaim(ctx context.Context, userID uuid.UUID, def *domain.BonusDefinition, params map[string]interface{}) error {
	return WelcomeHandler{}.ValidateClaim(ctx, userID, def, params)
}

// CalculateBonus mirrors the welcome computation with the reload definition's
// own match_percent and cap.
func (ReloadHandler) CalculateBonus(ctx context.Context, def *domain.BonusDefinition, params map[string]interface{}) (int64, error) {
	return WelcomeHandler{}.CalculateBonus(ctx, def, params)
}

// ---------------------------------------------------------------------------
// CashbackHandler — periodic loss rebate
// ---------------------------------------------------------------------------

// CashbackHandler implements the weekly cashback: a percentage of the
// player's recorded net losses over the configured period, capped by the
// definition maximum. Loss totals are computed by the caller and passed in
// params as "loss_amount_cents"; wagering_multiplier is typically 0 for
// cashback, so grants are immediately withdrawable once wagered through.
type CashbackHandler struct{}

// ValidateClaim requires a positive loss amount for the period.
func (CashbackHandler) ValidateClaim(_ context.Context, _ uuid.UUID, _ *domain.BonusDefinition, params map[string]interface{}) error {
	losses, ok := paramInt64(params, paramLossAmountCents)
	if !ok || losses <= 0 {
		return fmt.Errorf("%w: positive loss_amount_cents required", domain.ErrBonusNotClaimable)
	}
	return nil
}

// CalculateBonus returns min(losses*cashback_percent/100, max_bonus_amount_cents).
func (CashbackHandler) CalculateBonus(_ context.Context, def *domain.BonusDefinition, params map[string]interface{}) (int64, error) {
	losses, _ := paramInt64(params, paramLossAmountCents)
	pct, ok := configPercent(def.Config, configCashbackPercent)
	if !ok {
		return 0, fmt.Errorf("cashback config missing %s", configCashbackPercent)
	}
	return cappedMatch(losses, pct, def.MaxBonusAmountCents), nil
}

// ---------------------------------------------------------------------------
// FreeSpinsHandler — fixed-value spins grant
// ---------------------------------------------------------------------------

// FreeSpinsHandler implements free spins: a fixed bonus worth the definition
// maximum. When the definition config pins a game_id, claims must target that
// game; unpinned definitions are claimable from any eligible game context.
type FreeSpinsHandler struct{}

// ValidateClaim enforces the pinned game when config specifies one.
func (FreeSpinsHandler) ValidateClaim(_ context.Context, _ uuid.UUID, def *domain.BonusDefinition, params map[string]interface{}) error {
	pinnedGame, pinned := def.Config[configGameID].(string)
	if !pinned || pinnedGame == "" {
		return nil
	}
	gameID, _ := params[paramGameID].(string)
	if gameID != pinnedGame {
		return fmt.Errorf("%w: free spins restricted to game %q", domain.ErrBonusNotClaimable, pinnedGame)
	}
	return nil
}

// CalculateBonus returns the fixed value: max_bonus_amount_cents.
func (FreeSpinsHandler) CalculateBonus(_ context.Context, def *domain.BonusDefinition, _ map[string]interface{}) (int64, error) {
	return def.MaxBonusAmountCents, nil
}
