package service

import (
	"context"
	"fmt"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
)

// WalletService handles wallet and balance operations.
type WalletService struct {
	walletRepo repository.WalletRepository
}

// NewWalletService creates a new wallet service.
func NewWalletService(walletRepo repository.WalletRepository) *WalletService {
	return &WalletService{walletRepo: walletRepo}
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
func (s *WalletService) Deposit(ctx context.Context, userID uuid.UUID, amount float64) (*domain.Wallet, error) {
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
func (s *WalletService) Withdraw(ctx context.Context, userID uuid.UUID, amount float64) (*domain.Wallet, error) {
	wallet, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("wallet not found: %w", err)
	}

	if wallet.Balance < amount {
		return nil, fmt.Errorf("insufficient balance: have %.2f, want %.2f", wallet.Balance, amount)
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
