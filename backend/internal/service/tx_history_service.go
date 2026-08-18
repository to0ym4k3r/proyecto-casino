package service

import (
	"context"
	"fmt"

	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/google/uuid"
)

// TxHistoryService provides enriched transaction history queries
// combining on-chain Solana transaction records with wallet transactions.
type TxHistoryService struct {
	solanaTxRepo repository.SolanaTxRepository
}

// NewTxHistoryService creates a new transaction history service.
func NewTxHistoryService(solanaTxRepo repository.SolanaTxRepository) *TxHistoryService {
	return &TxHistoryService{solanaTxRepo: solanaTxRepo}
}

// GetHistory returns paginated, filterable Solana transaction history for a wallet.
func (s *TxHistoryService) GetHistory(
	ctx context.Context,
	walletID uuid.UUID,
	filter domain.TxHistoryFilter,
) (*domain.TxHistoryResult, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	txs, total, err := s.solanaTxRepo.ListByWalletID(
		ctx, walletID, filter.Type, filter.After, filter.Before, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list solana txs: %w", err)
	}

	return &domain.TxHistoryResult{
		Transactions: txs,
		Total:        total,
		Page:         page,
		Limit:        limit,
	}, nil
}
