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

// WalletService handles wallet and balance operations.
type WalletService struct {
	walletRepo repository.WalletRepository
}

// NewWalletService creates a new wallet service.
func NewWalletService(walletRepo repository.WalletRepository) *WalletService {
	return &WalletService{walletRepo: walletRepo}
}

// ---------------------------------------------------------------------------
// Transaction-scoped bonus-balance helpers
//
// These run inside a caller-owned pgx.Tx so multi-step money operations
// (claim, forfeit, expiry, bet deduction) stay atomic: every helper below is
// composed by BonusService inside ONE transaction. (GameService still uses
// its own inline locking; unifying is a follow-up.)
// ---------------------------------------------------------------------------

// LockForUpdate selects the user's wallet row FOR UPDATE inside the caller's
// transaction, serializing concurrent money operations on that wallet until
// the transaction commits or rolls back. Only the columns needed for balance
// decisions are read; the returned Wallet carries the queried UserID.
func (s *WalletService) LockForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*domain.Wallet, error) {
	w := &domain.Wallet{}
	err := tx.QueryRow(ctx,
		`SELECT id, balance, bonus_balance, version FROM wallets WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&w.ID, &w.Balance, &w.BonusBalance, &w.Version)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("lock wallet for user %s: %w", userID, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("lock wallet for user %s: %w", userID, err)
	}
	w.UserID = userID
	return w, nil
}

// CreditBonusBalance adds amount to wallets.bonus_balance inside the caller's
// transaction. Negative amounts are rejected: credits flow through dedicated
// call sites so an accidental sign flip cannot mint bonus money.
func (s *WalletService) CreditBonusBalance(ctx context.Context, tx pgx.Tx, userID uuid.UUID, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("credit bonus balance: negative amount %d", amount)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE wallets SET bonus_balance = bonus_balance + $2, updated_at = NOW()
		 WHERE user_id = $1`,
		userID, amount,
	)
	if err != nil {
		return fmt.Errorf("credit bonus balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("credit bonus balance for user %s: %w", userID, domain.ErrNotFound)
	}
	return nil
}

// DebitBonusBalance subtracts amount from wallets.bonus_balance inside the
// caller's transaction, floored at zero via GREATEST. The floor keeps the
// shared column consistent when several grants coexist and one of them was
// already partially spent; callers attribute per-grant amounts.
func (s *WalletService) DebitBonusBalance(ctx context.Context, tx pgx.Tx, userID uuid.UUID, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("debit bonus balance: negative amount %d", amount)
	}
	tag, err := tx.Exec(ctx,
		`UPDATE wallets SET bonus_balance = GREATEST(bonus_balance - $2, 0), updated_at = NOW()
		 WHERE user_id = $1`,
		userID, amount,
	)
	if err != nil {
		return fmt.Errorf("debit bonus balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("debit bonus balance for user %s: %w", userID, domain.ErrNotFound)
	}
	return nil
}

// ZeroBonusBalance sets wallets.bonus_balance to 0 inside the caller's
// transaction. Intended for forced forfeit paths (e.g. whole-pool surrender
// on withdrawal) once such callers land.
func (s *WalletService) ZeroBonusBalance(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`UPDATE wallets SET bonus_balance = 0, updated_at = NOW() WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("zero bonus balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("zero bonus balance for user %s: %w", userID, domain.ErrNotFound)
	}
	return nil
}

// ConvertBonusToReal atomically moves amount from bonus_balance to balance
// inside the caller's transaction. Used when wagering requirements are met.
// The debit is zero-floored via GREATEST so partial spends across grants stay
// consistent.
func (s *WalletService) ConvertBonusToReal(ctx context.Context, tx pgx.Tx, userID uuid.UUID, amount int64) error {
	if amount <= 0 {
		return nil // nothing to convert
	}
	// Debit bonus_balance (zero-floored)
	tag, err := tx.Exec(ctx,
		`UPDATE wallets 
		 SET bonus_balance = GREATEST(bonus_balance - $2, 0),
		     balance = balance + $2,
		     updated_at = NOW()
		 WHERE user_id = $1`,
		userID, amount,
	)
	if err != nil {
		return fmt.Errorf("convert bonus to real: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("convert bonus to real for user %s: %w", userID, domain.ErrNotFound)
	}
	return nil
}

// GetBalance returns the wallet for the given user.
//
// Delegates to the repository. Returns an error (typically wrapping a not-found)
// if no wallet exists for the user ID.
func (s *WalletService) GetBalance(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	return s.walletRepo.GetByUserID(ctx, userID)
}

// Deposit adds funds to the user's wallet and records a deposit transaction.
//
// Uses optimistic concurrency (wallet.Version) to prevent lost updates.
// Returns an error if the wallet is not found or the version has changed.
// On success a Transaction of type TransactionDeposit is persisted and the
// refreshed wallet is returned.
func (s *WalletService) Deposit(ctx context.Context, userID uuid.UUID, amount int64) (*domain.Wallet, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	if err := s.walletRepo.UpdateBalance(ctx, wallet.ID, amount, wallet.Version); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	tx := &domain.Transaction{
		ID:        uuid.New(),
		WalletID:  wallet.ID,
		Type:      domain.TransactionDeposit,
		Amount:    amount,
		Balance:   wallet.Balance + amount,
		Reference: fmt.Sprintf("deposit-%s", uuid.New().String()[:8]),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.walletRepo.CreateTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("record transaction: %w", err)
	}

	return s.walletRepo.GetByID(ctx, wallet.ID)
}

// Withdraw removes funds from the user's wallet and records a withdrawal transaction.
//
// Returns an error if the wallet is not found, the balance is insufficient, or
// the optimistic version lock fails. On success a Transaction of type
// TransactionWithdraw is persisted and the refreshed wallet is returned.
func (s *WalletService) Withdraw(ctx context.Context, userID uuid.UUID, amount int64) (*domain.Wallet, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	if wallet.Balance < amount {
		return nil, fmt.Errorf("insufficient balance: have %d, want %d", wallet.Balance, amount)
	}

	if err := s.walletRepo.UpdateBalance(ctx, wallet.ID, -amount, wallet.Version); err != nil {
		return nil, fmt.Errorf("update balance: %w", err)
	}

	tx := &domain.Transaction{
		ID:        uuid.New(),
		WalletID:  wallet.ID,
		Type:      domain.TransactionWithdraw,
		Amount:    -amount,
		Balance:   wallet.Balance - amount,
		Reference: fmt.Sprintf("withdraw-%s", uuid.New().String()[:8]),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.walletRepo.CreateTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("record transaction: %w", err)
	}

	return s.walletRepo.GetByID(ctx, wallet.ID)
}

// CreateWallet creates a new zero-balance wallet for the given user.
//
// The wallet is created with currency "USD" and version 1. This is typically
// called during user registration. Returns an error if the repository insert fails.
func (s *WalletService) CreateWallet(ctx context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	now := time.Now().UTC()
	wallet := &domain.Wallet{
		ID:        uuid.New(),
		UserID:    userID,
		Balance:   0,
		Currency:  "USD",
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.walletRepo.Create(ctx, wallet); err != nil {
		return nil, fmt.Errorf("create wallet: %w", err)
	}

	return wallet, nil
}
