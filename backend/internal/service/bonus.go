package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BonusService orchestrates the bonus lifecycle: claiming, wagering
// progression, expiry and forfeit. Every multi-step money operation runs
// inside ONE pgx.Tx (via txFunc) composing the tx-scoped WalletService
// helpers with the tx-scoped bonus repositories, so a failure mid-flow rolls
// back grants, wallet balances and ledger entries together.
type BonusService struct {
	defs     repository.BonusDefinitionRepository
	grants   repository.BonusGrantRepository
	ledger   repository.BonusLedgerRepository
	wallets  *WalletService
	handlers map[string]domain.BonusHandler
	txFunc   TxFunc
}

// NewBonusService creates a new bonus service. handlers maps bonus
// definition codes (e.g. domain.BonusTypeWelcome) to type-specific claim
// logic; codes without a registered handler are not claimable.
func NewBonusService(
	defs repository.BonusDefinitionRepository,
	grants repository.BonusGrantRepository,
	ledger repository.BonusLedgerRepository,
	wallets *WalletService,
	handlers map[string]domain.BonusHandler,
	txFunc TxFunc,
) *BonusService {
	return &BonusService{
		defs:     defs,
		grants:   grants,
		ledger:   ledger,
		wallets:  wallets,
		handlers: handlers,
		txFunc:   txFunc,
	}
}

// DefaultBonusHandlers returns the built-in handler set for the four seeded
// bonus types. Register it via NewBonusService; custom definitions can reuse
// one of these behaviors by code or add their own handler at wiring time.
func DefaultBonusHandlers() map[string]domain.BonusHandler {
	return map[string]domain.BonusHandler{
		domain.BonusTypeWelcome:   WelcomeHandler{},
		domain.BonusTypeReload:    ReloadHandler{},
		domain.BonusTypeCashback:  CashbackHandler{},
		domain.BonusTypeFreeSpins: FreeSpinsHandler{},
	}
}

// ---------------------------------------------------------------------------
// User-facing operations
// ---------------------------------------------------------------------------

// ListAvailableBonuses returns the active bonus definitions a user can see.
func (s *BonusService) ListAvailableBonuses(ctx context.Context, _ uuid.UUID) ([]domain.BonusDefinition, error) {
	defs, err := s.defs.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list available bonuses: %w", err)
	}
	return defs, nil
}

// ClaimBonus validates, prices and materializes a bonus grant for the user:
// one active grant per user+definition, enforced by an in-tx duplicate check
// plus the partial unique index (23505 mapped to ErrActiveBonusExists as the
// belt-and-braces defense). On success it credits wallets.bonus_balance and
// writes a "granted" ledger entry — all inside a single transaction.
//
// params carries type-specific claim inputs, e.g. {"deposit_cents": 10000}.
func (s *BonusService) ClaimBonus(ctx context.Context, userID, defID uuid.UUID, params map[string]interface{}) (*domain.BonusClaimResult, error) {
	// Pure pre-checks outside the transaction: fail fast before locking rows.
	def, err := s.defs.GetByID(ctx, defID)
	if err != nil {
		if errors.Is(err, repository.ErrBonusDefinitionNotFound) {
			return nil, fmt.Errorf("claim bonus: %w", domain.ErrBonusNotFound)
		}
		return nil, fmt.Errorf("claim bonus: %w", err)
	}
	if !def.IsActive {
		return nil, fmt.Errorf("claim bonus %s: %w", def.Code, domain.ErrBonusNotActive)
	}

	handler, ok := s.handlers[def.Code]
	if !ok {
		return nil, fmt.Errorf("claim bonus %s: %w", def.Code, domain.ErrBonusNotClaimable)
	}
	if err := handler.ValidateClaim(ctx, userID, def, params); err != nil {
		return nil, fmt.Errorf("claim bonus %s: %w", def.Code, err)
	}
	amount, err := handler.CalculateBonus(ctx, def, params)
	if err != nil {
		return nil, fmt.Errorf("claim bonus %s: %w", def.Code, err)
	}

	var result *domain.BonusClaimResult
	err = s.txFunc(ctx, func(tx pgx.Tx) error {
		// Serialize money operations on this wallet row.
		if _, err := s.wallets.LockForUpdate(ctx, tx, userID); err != nil {
			return err
		}

		// Secondary duplicate check under the row lock; the partial unique
		// index on (user_id, definition_id) WHERE status='active' is primary.
		if _, err := s.grants.GetActiveByUserAndDefinition(ctx, tx, userID, defID); err == nil {
			return fmt.Errorf("claim bonus %s: %w", def.Code, domain.ErrActiveBonusExists)
		} else if !errors.Is(err, repository.ErrBonusGrantNotFound) {
			return fmt.Errorf("claim bonus %s: %w", def.Code, err)
		}

		now := time.Now().UTC()
		expiresAt := now.AddDate(0, 0, def.ExpiryDays)
		grant := &domain.BonusGrant{
			ID:                    uuid.New(),
			UserID:                userID,
			BonusDefinitionID:     def.ID,
			BonusAmountCents:      amount,
			WageringRequiredCents: amount * int64(def.WageringMultiplier),
			Status:                domain.BonusStatusActive,
			GrantedBy:             domain.GrantedBySystem,
			ExpiresAt:             expiresAt,
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		if err := s.grants.Create(ctx, tx, grant); err != nil {
			if errors.Is(err, repository.ErrActiveBonusGrantExists) {
				return fmt.Errorf("claim bonus %s: %w", def.Code, domain.ErrActiveBonusExists)
			}
			return fmt.Errorf("create grant: %w", err)
		}

		if err := s.wallets.CreditBonusBalance(ctx, tx, userID, amount); err != nil {
			return err
		}

		if err := s.writeLedger(ctx, tx, grant.ID, domain.BonusLedgerGranted, amount); err != nil {
			return err
		}

		result = &domain.BonusClaimResult{
			GrantID:               grant.ID,
			BonusAmountCents:      amount,
			WageringRequiredCents: grant.WageringRequiredCents,
			ExpiresAt:             expiresAt.Format(time.RFC3339),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetActiveGrants returns the user's active grants joined with their
// definition code/name and wagering progress, for GET /bonuses/active.
func (s *BonusService) GetActiveGrants(ctx context.Context, userID uuid.UUID) ([]domain.BonusActiveResponse, error) {
	grants, err := s.grants.ListActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active grants: %w", err)
	}

	responses := make([]domain.BonusActiveResponse, 0, len(grants))
	for _, g := range grants {
		def, err := s.defs.GetByID(ctx, g.BonusDefinitionID)
		if err != nil {
			return nil, fmt.Errorf("join definition %s for grant %s: %w", g.BonusDefinitionID, g.ID, err)
		}
		responses = append(responses, domain.BonusActiveResponse{
			GrantID:               g.ID,
			Code:                  def.Code,
			Name:                  def.Name,
			BonusAmountCents:      g.BonusAmountCents,
			WageringRequiredCents: g.WageringRequiredCents,
			TotalWageredCents:     g.TotalWageredCents,
			WageringProgressPct:   g.WageringProgressPct(),
			ExpiresAt:             g.ExpiresAt.Format(time.RFC3339),
		})
	}
	return responses, nil
}

// ForfeitBonus surrenders one ACTIVE grant owned by the user: the grant's
// bonus amount is debited from wallets.bonus_balance (per-grant debit with a
// zero floor, so coexisting grants keep their own money), the grant flips to
// 'forfeited' and a "forfeited" ledger entry is written — all inside ONE
// transaction. Missing, foreign or non-active grants all surface as
// domain.ErrGrantNotFound so callers cannot probe other users' grants.
func (s *BonusService) ForfeitBonus(ctx context.Context, userID, grantID uuid.UUID) error {
	grant, err := s.grants.GetByID(ctx, grantID)
	if err != nil {
		if errors.Is(err, repository.ErrBonusGrantNotFound) {
			return fmt.Errorf("forfeit bonus: %w", domain.ErrGrantNotFound)
		}
		return fmt.Errorf("forfeit bonus: %w", err)
	}
	if grant.UserID != userID || grant.Status != domain.BonusStatusActive {
		return fmt.Errorf("forfeit bonus: %w", domain.ErrGrantNotFound)
	}

	return s.txFunc(ctx, func(tx pgx.Tx) error {
		if _, err := s.wallets.LockForUpdate(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.wallets.DebitBonusBalance(ctx, tx, userID, grant.BonusAmountCents); err != nil {
			return err
		}
		if err := s.grants.UpdateStatus(ctx, tx, grant.ID, domain.BonusStatusForfeited); err != nil {
			return fmt.Errorf("mark grant forfeited: %w", err)
		}
		return s.writeLedger(ctx, tx, grant.ID, domain.BonusLedgerForfeited, grant.BonusAmountCents)
	})
}

// ForfeitActiveGrantsTx surrenders EVERY active grant the user holds and zeroes
// wallets.bonus_balance, writing one "forfeited" ledger entry per grant — all
// inside the caller's pgx.Tx. It backs the forced-forfeit-on-withdrawal rule
// (spec Domain 2): a withdrawal strips the whole bonus pool before any real
// money moves. The wallet row is locked first and the grant list is read under
// that lock, matching ClaimBonus/ForfeitBonus/AdminExpireGrant; a user with no
// active grant takes the lock but moves no money, leaving ordinary withdrawals
// untouched.
func (s *BonusService) ForfeitActiveGrantsTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	// Lock the wallet row BEFORE listing grants. Listing under the lock closes
	// the TOCTOU window where a concurrent claim or conversion changes the
	// active set between the list and the lock.
	if _, err := s.wallets.LockForUpdate(ctx, tx, userID); err != nil {
		return err
	}

	grants, err := s.grants.ListActiveByUserTx(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("list active grants for forfeit: %w", err)
	}
	if len(grants) == 0 {
		return nil
	}
	if err := s.wallets.ZeroBonusBalance(ctx, tx, userID); err != nil {
		return err
	}

	for _, g := range grants {
		if err := s.grants.UpdateStatus(ctx, tx, g.ID, domain.BonusStatusForfeited); err != nil {
			return fmt.Errorf("mark grant %s forfeited: %w", g.ID, err)
		}
		if err := s.writeLedger(ctx, tx, g.ID, domain.BonusLedgerForfeited, g.BonusAmountCents); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Game integration (runs inside the caller's PlaceBet transaction)
// ---------------------------------------------------------------------------

// ProcessBetBonus advances wagering on an active grant by wageredAmount — the
// FULL bet amount, which the caller caps at the grant's remaining requirement.
// It MUST be called with the transaction that already settled the bonus debit
// (PlaceBet's real-first/bonus-second split UPDATE), so the wagering increment
// commits or rolls back atomically with the bet. This method only advances the
// counter and audits the advance; it never moves money. IncrementWagered flips
// the grant to 'wagered' when the requirement is met; every call appends a
// "wagered" ledger entry for the audit trail.
func (s *BonusService) ProcessBetBonus(ctx context.Context, tx pgx.Tx, grantID uuid.UUID, wageredAmount int64) error {
	if wageredAmount < 0 {
		return fmt.Errorf("process bet bonus: negative wagered amount %d", wageredAmount)
	}
	if wageredAmount == 0 {
		return nil
	}
	if err := s.grants.IncrementWagered(ctx, tx, grantID, wageredAmount); err != nil {
		return fmt.Errorf("increment wagering for grant %s: %w", grantID, err)
	}
	return s.writeLedger(ctx, tx, grantID, domain.BonusLedgerWagered, wageredAmount)
}

// findActiveGrantForBet returns the user's active grant with the highest
// remaining wagering need, or nil if none exists. Runs inside the caller's tx.
func (s *BonusService) findActiveGrantForBet(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*domain.BonusGrant, error) {
	grants, err := s.grants.ListActiveByUserTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if len(grants) == 0 {
		return nil, nil
	}
	// Pick the grant with the most remaining wagering (prioritize completing one)
	var best *domain.BonusGrant
	for i := range grants {
		remaining := grants[i].WageringRequiredCents - grants[i].TotalWageredCents
		if remaining <= 0 {
			continue // already completed
		}
		if best == nil || remaining > (best.WageringRequiredCents-best.TotalWageredCents) {
			best = &grants[i]
		}
	}
	return best, nil
}

// convertCompletedBonus moves the grant's remaining bonus money from
// bonus_balance to real balance and flips the grant status to 'wagered' —
// all inside the caller's transaction.
//
// The conversion is capped at the wallet's CURRENT bonus_balance: the
// completing bet may itself have spent part of the bonus pool (real-first/
// bonus-second split), so converting the original grant amount would mint
// real money out of an already-spent pool. The wallet row is already locked
// by PlaceBet's SELECT ... FOR UPDATE, so this re-read observes the post-split
// balance; the ledger records what was ACTUALLY converted.
func (s *BonusService) convertCompletedBonus(ctx context.Context, tx pgx.Tx, userID uuid.UUID, grant *domain.BonusGrant) error {
	// Re-read the locked wallet to learn the post-split bonus pool.
	wallet, err := s.wallets.LockForUpdate(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("lock wallet for conversion: %w", err)
	}

	// Never convert more than this grant granted, nor more than the pool holds.
	converted := grant.BonusAmountCents
	if converted > wallet.BonusBalance {
		converted = wallet.BonusBalance
	}

	if converted > 0 {
		if err := s.wallets.ConvertBonusToReal(ctx, tx, userID, converted); err != nil {
			return err
		}
	}
	// Mark grant as wagered (completed)
	if err := s.grants.UpdateStatus(ctx, tx, grant.ID, domain.BonusStatusWagered); err != nil {
		return fmt.Errorf("mark grant wagered: %w", err)
	}
	// Write completion ledger entry with the honest, actual converted amount
	return s.writeLedger(ctx, tx, grant.ID, domain.BonusLedgerCompleted, converted)
}

// ---------------------------------------------------------------------------
// Background expiry
// ---------------------------------------------------------------------------

// ExpireStaleBonuses marks grants past expires_at as expired and settles
// their money in ONE transaction: per-grant debits (zero-floored) plus one
// "expired" ledger entry per grant. Returns how many grants expired so the
// scheduler can log it.
func (s *BonusService) ExpireStaleBonuses(ctx context.Context) (int, error) {
	var count int
	err := s.txFunc(ctx, func(tx pgx.Tx) error {
		// ExpireStale must run first: it is what discovers the affected
		// users, so the wallet locks cannot precede it (see below).
		stale, err := s.grants.ExpireStale(ctx, tx)
		if err != nil {
			return err
		}

		// Lock every affected wallet BEFORE settling money, so the debit and
		// ledger writes run under the same row lock the claim/forfeit paths
		// take. Locks are deduplicated per user in first-seen grant order;
		// they are not globally sorted, so a concurrent flow taking locks in
		// a different order can deadlock — Postgres resolves that by aborting
		// one transaction rather than corrupting state, and the scheduler
		// retries on its next tick.
		seen := make(map[uuid.UUID]struct{}, len(stale))
		for _, g := range stale {
			if _, ok := seen[g.UserID]; ok {
				continue
			}
			seen[g.UserID] = struct{}{}
			if _, err := s.wallets.LockForUpdate(ctx, tx, g.UserID); err != nil {
				return err
			}
		}

		for _, g := range stale {
			if err := s.wallets.DebitBonusBalance(ctx, tx, g.UserID, g.BonusAmountCents); err != nil {
				return err
			}
			if err := s.writeLedger(ctx, tx, g.ID, domain.BonusLedgerExpired, g.BonusAmountCents); err != nil {
				return err
			}
		}
		count = len(stale)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// writeLedger appends one audit entry inside the caller's transaction.
func (s *BonusService) writeLedger(ctx context.Context, tx pgx.Tx, grantID uuid.UUID, transactionType string, amountCents int64) error {
	entry := &domain.BonusLedgerEntry{
		ID:              uuid.New(),
		GrantID:         grantID,
		TransactionType: transactionType,
		AmountCents:     amountCents,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.ledger.Create(ctx, tx, entry); err != nil {
		return fmt.Errorf("write bonus ledger %s entry: %w", transactionType, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Admin operations
// ---------------------------------------------------------------------------

// ListAllBonuses returns every bonus definition (active and inactive) for
// admin management.
func (s *BonusService) ListAllBonuses(ctx context.Context) ([]domain.BonusDefinition, error) {
	defs, err := s.defs.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all bonuses: %w", err)
	}
	return defs, nil
}

// CreateOrUpdateBonus upserts a bonus definition. If def.ID is uuid.Nil the
// repo creates a new row; otherwise it updates the existing one.
func (s *BonusService) CreateOrUpdateBonus(ctx context.Context, def *domain.BonusDefinition) error {
	if def.ID == uuid.Nil {
		if err := s.defs.Create(ctx, def); err != nil {
			return fmt.Errorf("create bonus definition: %w", err)
		}
		return nil
	}
	if err := s.defs.Update(ctx, def); err != nil {
		return fmt.Errorf("update bonus definition: %w", err)
	}
	return nil
}

// AdminListGrants returns grants filtered by status and optional userID, with
// pagination, for the admin grant list endpoint.
func (s *BonusService) AdminListGrants(ctx context.Context, status string, userID *uuid.UUID, limit, offset int) ([]domain.BonusGrant, int, error) {
	grants, total, err := s.grants.ListAll(ctx, status, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("admin list grants: %w", err)
	}
	return grants, total, nil
}

// AdminExpireGrant manually expires a single grant: flips status to expired,
// debits the bonus_balance and writes a ledger entry — all atomically.
func (s *BonusService) AdminExpireGrant(ctx context.Context, grantID uuid.UUID) error {
	grant, err := s.grants.GetByID(ctx, grantID)
	if err != nil {
		if errors.Is(err, repository.ErrBonusGrantNotFound) {
			return fmt.Errorf("admin expire grant: %w", domain.ErrGrantNotFound)
		}
		return fmt.Errorf("admin expire grant: %w", err)
	}
	if grant.Status != domain.BonusStatusActive {
		return fmt.Errorf("admin expire grant: %w", domain.ErrBonusNotActive)
	}

	return s.txFunc(ctx, func(tx pgx.Tx) error {
		if _, err := s.wallets.LockForUpdate(ctx, tx, grant.UserID); err != nil {
			return err
		}
		if err := s.wallets.DebitBonusBalance(ctx, tx, grant.UserID, grant.BonusAmountCents); err != nil {
			return err
		}
		if err := s.grants.UpdateStatus(ctx, tx, grant.ID, domain.BonusStatusExpired); err != nil {
			return fmt.Errorf("mark grant expired: %w", err)
		}
		return s.writeLedger(ctx, tx, grant.ID, domain.BonusLedgerExpired, grant.BonusAmountCents)
	})
}

// AdminGrant materializes a bonus grant on behalf of an admin. It mirrors
// ClaimBonus exactly — definition lookup, active-flag and type-handler
// validation/pricing, the in-tx duplicate-active check plus the partial unique
// index (belt-and-braces), crediting wallets.bonus_balance and writing a
// "granted" ledger entry — but stamps the grant with granted_by='admin' and
// the acting admin_id. Everything runs in ONE transaction with the wallet row
// locked first, so a failure mid-flow rolls back the grant, the balance and
// the ledger together.
func (s *BonusService) AdminGrant(ctx context.Context, adminID, userID, defID uuid.UUID, params map[string]interface{}) (*domain.BonusClaimResult, error) {
	// Pure pre-checks outside the transaction: fail fast before locking rows.
	def, err := s.defs.GetByID(ctx, defID)
	if err != nil {
		if errors.Is(err, repository.ErrBonusDefinitionNotFound) {
			return nil, fmt.Errorf("admin grant: %w", domain.ErrBonusNotFound)
		}
		return nil, fmt.Errorf("admin grant: %w", err)
	}
	if !def.IsActive {
		return nil, fmt.Errorf("admin grant %s: %w", def.Code, domain.ErrBonusNotActive)
	}

	handler, ok := s.handlers[def.Code]
	if !ok {
		return nil, fmt.Errorf("admin grant %s: %w", def.Code, domain.ErrBonusNotClaimable)
	}
	if err := handler.ValidateClaim(ctx, userID, def, params); err != nil {
		return nil, fmt.Errorf("admin grant %s: %w", def.Code, err)
	}
	amount, err := handler.CalculateBonus(ctx, def, params)
	if err != nil {
		return nil, fmt.Errorf("admin grant %s: %w", def.Code, err)
	}

	var result *domain.BonusClaimResult
	err = s.txFunc(ctx, func(tx pgx.Tx) error {
		// Serialize money operations on this wallet row.
		if _, err := s.wallets.LockForUpdate(ctx, tx, userID); err != nil {
			return err
		}

		// Secondary duplicate check under the row lock; the partial unique
		// index on (user_id, definition_id) WHERE status='active' is primary.
		if _, err := s.grants.GetActiveByUserAndDefinition(ctx, tx, userID, defID); err == nil {
			return fmt.Errorf("admin grant %s: %w", def.Code, domain.ErrActiveBonusExists)
		} else if !errors.Is(err, repository.ErrBonusGrantNotFound) {
			return fmt.Errorf("admin grant %s: %w", def.Code, err)
		}

		now := time.Now().UTC()
		expiresAt := now.AddDate(0, 0, def.ExpiryDays)
		admin := adminID
		grant := &domain.BonusGrant{
			ID:                    uuid.New(),
			UserID:                userID,
			BonusDefinitionID:     def.ID,
			BonusAmountCents:      amount,
			WageringRequiredCents: amount * int64(def.WageringMultiplier),
			Status:                domain.BonusStatusActive,
			GrantedBy:             domain.GrantedByAdmin,
			AdminID:               &admin,
			ExpiresAt:             expiresAt,
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		if err := s.grants.Create(ctx, tx, grant); err != nil {
			if errors.Is(err, repository.ErrActiveBonusGrantExists) {
				return fmt.Errorf("admin grant %s: %w", def.Code, domain.ErrActiveBonusExists)
			}
			return fmt.Errorf("create grant: %w", err)
		}

		if err := s.wallets.CreditBonusBalance(ctx, tx, userID, amount); err != nil {
			return err
		}

		if err := s.writeLedger(ctx, tx, grant.ID, domain.BonusLedgerGranted, amount); err != nil {
			return err
		}

		result = &domain.BonusClaimResult{
			GrantID:               grant.ID,
			BonusAmountCents:      amount,
			WageringRequiredCents: grant.WageringRequiredCents,
			ExpiresAt:             expiresAt.Format(time.RFC3339),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AdminRevoke forfeits ONE active grant on behalf of an admin. Unlike
// ForfeitBonus it does NOT require the grant to belong to the caller — an
// admin may revoke any user's grant. It debits that grant's bonus amount from
// the OWNER's wallets.bonus_balance (per-grant debit with a zero floor, so
// coexisting grants keep their own money), flips the grant to 'forfeited' and
// writes a "forfeited" ledger entry — all in ONE transaction with the wallet
// row locked first. Missing or non-active grants are rejected.
func (s *BonusService) AdminRevoke(ctx context.Context, _ uuid.UUID, grantID uuid.UUID) error {
	grant, err := s.grants.GetByID(ctx, grantID)
	if err != nil {
		if errors.Is(err, repository.ErrBonusGrantNotFound) {
			return fmt.Errorf("admin revoke: %w", domain.ErrGrantNotFound)
		}
		return fmt.Errorf("admin revoke: %w", err)
	}
	if grant.Status != domain.BonusStatusActive {
		return fmt.Errorf("admin revoke: %w", domain.ErrBonusNotActive)
	}

	return s.txFunc(ctx, func(tx pgx.Tx) error {
		if _, err := s.wallets.LockForUpdate(ctx, tx, grant.UserID); err != nil {
			return err
		}
		// Authoritative status guard INSIDE the transaction, AFTER the wallet
		// row lock. The pre-tx check above is only a fast path: two concurrent
		// revokes can both pass it (stale reads) and the wallet FOR UPDATE lock
		// merely serializes them. Re-reading here under READ COMMITTED (pgx's
		// default isolation) observes the latest committed state, so the loser
		// of the race sees a non-active status and aborts WITHOUT debiting
		// bonus_balance or writing a duplicate ledger row. This also prevents
		// regressing a grant that completed to 'wagered' between the pre-check
		// and the lock.
		reloaded, err := s.grants.GetByID(ctx, grantID)
		if err != nil {
			if errors.Is(err, repository.ErrBonusGrantNotFound) {
				return fmt.Errorf("admin revoke: %w", domain.ErrGrantNotFound)
			}
			return fmt.Errorf("admin revoke: %w", err)
		}
		if reloaded.Status != domain.BonusStatusActive {
			return fmt.Errorf("admin revoke: %w", domain.ErrBonusNotActive)
		}
		if err := s.wallets.DebitBonusBalance(ctx, tx, grant.UserID, reloaded.BonusAmountCents); err != nil {
			return err
		}
		if err := s.grants.UpdateStatus(ctx, tx, reloaded.ID, domain.BonusStatusForfeited); err != nil {
			return fmt.Errorf("mark grant forfeited: %w", err)
		}
		return s.writeLedger(ctx, tx, reloaded.ID, domain.BonusLedgerForfeited, reloaded.BonusAmountCents)
	})
}
