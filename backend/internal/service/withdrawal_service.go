package service

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WithdrawalService handles crypto withdrawal requests with KYC limits,
// emergency pause, outflow cap, and hot wallet integration.
type WithdrawalService struct {
	walletRepo      repository.WalletRepository
	withdrawalRepo  WithdrawalRepository
	getKYCTier      func(ctx context.Context, userID uuid.UUID) (KYCTier, error)
	emergencyPaused func(ctx context.Context) (bool, error)
	outflowChecker  func(ctx context.Context, hotWalletBalance, newOutflow int64) (bool, error)
	getHotBalance   func(ctx context.Context) (uint64, error)
	// bonusSvc/txFunc are optional. When set, RequestWithdrawal forfeits the
	// user's active bonus before debiting real money (spec Domain 2).
	bonusSvc *BonusService
	txFunc   TxFunc
}

// WithdrawalRepository defines persistence operations for withdrawals.
type WithdrawalRepository interface {
	Create(ctx context.Context, w *domain.Withdrawal) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Withdrawal, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Withdrawal, error)
	GetDailyTotal(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.WithdrawalStatus, txHash string) error
}

// NewWithdrawalService creates a WithdrawalService with the given dependencies.
func NewWithdrawalService(
	walletRepo repository.WalletRepository,
	withdrawalRepo WithdrawalRepository,
	getKYCTier func(ctx context.Context, userID uuid.UUID) (KYCTier, error),
	emergencyPaused func(ctx context.Context) (bool, error),
	getHotBalance func(ctx context.Context) (uint64, error),
) *WithdrawalService {
	return &WithdrawalService{
		walletRepo:      walletRepo,
		withdrawalRepo:  withdrawalRepo,
		getKYCTier:      getKYCTier,
		emergencyPaused: emergencyPaused,
		getHotBalance:   getHotBalance,
	}
}

// SetOutflowChecker configures the outflow cap check function.
func (s *WithdrawalService) SetOutflowChecker(fn func(ctx context.Context, hotWalletBalance, newOutflow int64) (bool, error)) {
	s.outflowChecker = fn
}

// SetBonusForfeit configures the optional forced-forfeit-on-withdrawal
// dependency. When set, RequestWithdrawal surrenders every active bonus grant
// (bonus_balance → 0, grants → forfeited, ledger entries written) through
// txFunc before any real money is debited. The forfeit commits in its own
// transaction, separate from the debit and withdrawal create that follow (see
// the step-7 comment in RequestWithdrawal for the known atomicity gap). It is
// nil-safe: leaving it unset keeps the pre-existing withdrawal behavior
// unchanged.
func (s *WithdrawalService) SetBonusForfeit(bonusSvc *BonusService, txFunc TxFunc) {
	s.bonusSvc = bonusSvc
	s.txFunc = txFunc
}

// RequestWithdrawal processes a crypto withdrawal request through the full pipeline:
// emergency pause check → KYC tier limit check → daily limit check →
// outflow cap check → balance debit → on-chain send.
func (s *WithdrawalService) RequestWithdrawal(
	ctx context.Context,
	userID uuid.UUID,
	amount int64,
	toAddress string,
	currency string,
) (*domain.Withdrawal, error) {
	// 1. Emergency pause check
	paused, err := s.emergencyPaused(ctx)
	if err != nil {
		return nil, fmt.Errorf("check emergency pause: %w", err)
	}
	if paused {
		return nil, fmt.Errorf("withdrawals paused")
	}

	// 2. Get KYC tier
	tier, err := s.getKYCTier(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get KYC tier: %w", err)
	}

	// 3. Per-transaction limit check
	limits := DefaultKYCLimits()[tier]
	if amount > limits.PerTransaction {
		return nil, fmt.Errorf("per-transaction limit exceeded for tier %s: max %d cents", tier, limits.PerTransaction)
	}

	// 4. Daily limit check
	dailyTotal, err := s.withdrawalRepo.GetDailyTotal(ctx, userID, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("get daily total: %w", err)
	}
	if dailyTotal+amount > limits.Daily {
		return nil, fmt.Errorf("daily limit exceeded")
	}

	// 5. Outflow cap check (if configured)
	if s.outflowChecker != nil {
		hotLamports, err := s.getHotBalance(ctx)
		if err != nil {
			return nil, fmt.Errorf("get hot wallet balance: %w", err)
		}
		hotWalletBalance := int64(hotLamports)
		exceeded, err := s.outflowChecker(ctx, hotWalletBalance, amount)
		if err != nil {
			return nil, fmt.Errorf("check outflow cap: %w", err)
		}
		if exceeded {
			return nil, fmt.Errorf("hot wallet outflow cap exceeded")
		}
	}

	// 6. Balance check
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}
	if wallet.Balance < amount {
		return nil, fmt.Errorf("insufficient balance: have %d, want %d", wallet.Balance, amount)
	}

	// 7. Forfeit any active bonus BEFORE real money moves (spec Domain 2,
	// scenario 4): withdrawing always surrenders the whole bonus pool. The
	// forfeit commits in its OWN transaction, separate from the debit and
	// withdrawal create that follow: a failure after the forfeit (e.g. debit
	// or create error) leaves the pool forfeited with no withdrawal recorded,
	// and a bonus claimed between the forfeit commit and the debit can
	// survive the withdrawal. Known atomicity gap (R-001); single-transaction
	// unification of forfeit+debit+create is deferred to a follow-up change.
	if s.bonusSvc != nil && s.txFunc != nil {
		if err := s.txFunc(ctx, func(tx pgx.Tx) error {
			return s.bonusSvc.ForfeitActiveGrantsTx(ctx, tx, userID)
		}); err != nil {
			return nil, fmt.Errorf("forfeit active bonus: %w", err)
		}
	}

	// 8. Determine approval status
	status := domain.WithdrawalStatusApproved
	threshold := AutoApprovalThreshold(tier)
	if amount > threshold {
		status = domain.WithdrawalStatusPendingApproval
	}

	// 9. Debit wallet
	if err := s.walletRepo.UpdateBalance(ctx, wallet.ID, -amount, wallet.Version); err != nil {
		return nil, fmt.Errorf("debit balance: %w", err)
	}

	// 10. Create withdrawal record
	now := time.Now().UTC()
	withdrawal := &domain.Withdrawal{
		ID:        uuid.New(),
		UserID:    userID,
		Amount:    amount,
		Currency:  currency,
		ToAddress: toAddress,
		Status:    status,
		CreatedAt: now,
	}

	if err := s.withdrawalRepo.Create(ctx, withdrawal); err != nil {
		return nil, fmt.Errorf("create withdrawal: %w", err)
	}

	return withdrawal, nil
}
