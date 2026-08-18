package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/google/uuid"
)

// --- Mock SolanaTxRepository ---

type mockSolanaTxRepo struct {
	txs   []domain.SolanaTx
	total int
}

func (m *mockSolanaTxRepo) Create(_ context.Context, tx *domain.SolanaTx) error {
	m.txs = append(m.txs, *tx)
	return nil
}

func (m *mockSolanaTxRepo) ListByWalletID(_ context.Context, walletID uuid.UUID, txType string, after, before *time.Time, limit, offset int) ([]domain.SolanaTx, int, error) {
	var filtered []domain.SolanaTx
	for _, tx := range m.txs {
		if tx.WalletID != walletID {
			continue
		}
		if txType != "" && string(tx.Type) != txType {
			continue
		}
		if after != nil && tx.CreatedAt.Before(*after) {
			continue
		}
		if before != nil && tx.CreatedAt.After(*before) {
			continue
		}
		filtered = append(filtered, tx)
	}

	total := len(filtered)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

func (m *mockSolanaTxRepo) GetByTxHash(_ context.Context, txHash string) (*domain.SolanaTx, error) {
	for _, tx := range m.txs {
		if tx.TxHash == txHash {
			return &tx, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// --- Transaction History Tests ---

func TestTxHistoryShowsMetadata(t *testing.T) {
	walletID := uuid.New()
	now := time.Now().UTC()

	txs := []domain.SolanaTx{
		{
			ID: uuid.New(), WalletID: walletID, TxHash: "abc123def",
			Type: domain.SolanaTxTypeDeposit, Amount: 50000,
			FromAddress: "SenderAddr111", ToAddress: "DepositAddr111",
			Confirmations: 35, NetworkFee: 5000, UsdValue: 50000,
			CreatedAt: now,
		},
	}

	repo := &mockSolanaTxRepo{txs: txs}
	svc := NewTxHistoryService(repo)

	result, err := svc.GetHistory(context.Background(), walletID, domain.TxHistoryFilter{
		Page:  1,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(result.Transactions))
	}

	tx := result.Transactions[0]
	if tx.TxHash != "abc123def" {
		t.Errorf("tx_hash = %q, want %q", tx.TxHash, "abc123def")
	}
	if tx.Confirmations != 35 {
		t.Errorf("confirmations = %d, want 35", tx.Confirmations)
	}
	if tx.NetworkFee != 5000 {
		t.Errorf("network_fee = %d, want 5000", tx.NetworkFee)
	}
	if tx.Type != domain.SolanaTxTypeDeposit {
		t.Errorf("type = %q, want %q", tx.Type, domain.SolanaTxTypeDeposit)
	}
}

func TestTxHistoryPagination(t *testing.T) {
	walletID := uuid.New()
	now := time.Now().UTC()

	// Create 150 transactions
	txs := make([]domain.SolanaTx, 150)
	for i := range txs {
		txs[i] = domain.SolanaTx{
			ID: uuid.New(), WalletID: walletID, TxHash: uuid.New().String()[:8],
			Type: domain.SolanaTxTypeDeposit, Amount: int64(i * 100),
			Confirmations: 32, NetworkFee: 5000,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		}
	}

	repo := &mockSolanaTxRepo{txs: txs}
	svc := NewTxHistoryService(repo)

	// Page 2, limit 50 → should get transactions 51-100
	result, err := svc.GetHistory(context.Background(), walletID, domain.TxHistoryFilter{
		Page:  2,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 150 {
		t.Errorf("total = %d, want 150", result.Total)
	}
	if len(result.Transactions) != 50 {
		t.Errorf("page 2 count = %d, want 50", len(result.Transactions))
	}
	if result.Page != 2 {
		t.Errorf("page = %d, want 2", result.Page)
	}
	if result.Limit != 50 {
		t.Errorf("limit = %d, want 50", result.Limit)
	}
}

func TestTxHistoryFilterByType(t *testing.T) {
	walletID := uuid.New()
	now := time.Now().UTC()

	txs := []domain.SolanaTx{
		{ID: uuid.New(), WalletID: walletID, TxHash: "dep1", Type: domain.SolanaTxTypeDeposit, Amount: 1000, Confirmations: 32, CreatedAt: now},
		{ID: uuid.New(), WalletID: walletID, TxHash: "wit1", Type: domain.SolanaTxTypeWithdraw, Amount: 500, Confirmations: 32, CreatedAt: now},
		{ID: uuid.New(), WalletID: walletID, TxHash: "dep2", Type: domain.SolanaTxTypeDeposit, Amount: 2000, Confirmations: 32, CreatedAt: now},
	}

	repo := &mockSolanaTxRepo{txs: txs}
	svc := NewTxHistoryService(repo)

	result, err := svc.GetHistory(context.Background(), walletID, domain.TxHistoryFilter{
		Type:  "deposit",
		Page:  1,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("total = %d, want 2 (deposit only)", result.Total)
	}
	if len(result.Transactions) != 2 {
		t.Errorf("count = %d, want 2", len(result.Transactions))
	}
	for _, tx := range result.Transactions {
		if tx.Type != domain.SolanaTxTypeDeposit {
			t.Errorf("unexpected type %q in filtered results", tx.Type)
		}
	}
}

func TestTxHistoryFilterByDate(t *testing.T) {
	walletID := uuid.New()
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	txs := []domain.SolanaTx{
		{ID: uuid.New(), WalletID: walletID, TxHash: "old1", Type: domain.SolanaTxTypeDeposit, Amount: 1000, Confirmations: 32, CreatedAt: twoDaysAgo},
		{ID: uuid.New(), WalletID: walletID, TxHash: "new1", Type: domain.SolanaTxTypeDeposit, Amount: 2000, Confirmations: 32, CreatedAt: now},
	}

	repo := &mockSolanaTxRepo{txs: txs}
	svc := NewTxHistoryService(repo)

	result, err := svc.GetHistory(context.Background(), walletID, domain.TxHistoryFilter{
		After: &yesterday,
		Page:  1,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1 (after yesterday)", result.Total)
	}
	if len(result.Transactions) != 1 {
		t.Fatalf("count = %d, want 1", len(result.Transactions))
	}
	if result.Transactions[0].TxHash != "new1" {
		t.Errorf("tx_hash = %q, want %q", result.Transactions[0].TxHash, "new1")
	}
}

func TestTxHistoryEmptyResult(t *testing.T) {
	walletID := uuid.New()

	repo := &mockSolanaTxRepo{txs: []domain.SolanaTx{}}
	svc := NewTxHistoryService(repo)

	result, err := svc.GetHistory(context.Background(), walletID, domain.TxHistoryFilter{
		Page:  1,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
	if len(result.Transactions) != 0 {
		t.Errorf("count = %d, want 0", len(result.Transactions))
	}
}
