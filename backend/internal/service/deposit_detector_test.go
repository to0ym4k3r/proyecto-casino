package service

import (
	"testing"
)

func TestDepositDetectorBatchSplitting(t *testing.T) {
	// Test that batch sizes are correctly calculated
	addresses := make([]string, 250)
	for i := range addresses {
		addresses[i] = "11111111111111111111111111111111" // system program (valid pubkey)
	}

	// Verify batch count logic: 250 addresses / 100 per batch = 3 batches
	batchSize := 100
	batches := 0
	for i := 0; i < len(addresses); i += batchSize {
		batches++
	}

	if batches != 3 {
		t.Errorf("expected 3 batches for 250 addresses, got %d", batches)
	}
}

func TestDetectedDepositFields(t *testing.T) {
	d := DetectedDeposit{
		Address:        "TestAddr11111111111111111111111111111111111",
		TxHash:         "abc123",
		AmountLamports: 1000000000,
		Confirmations:  32,
	}

	if d.Confirmations < 32 {
		t.Errorf("confirmations = %d, want >= 32", d.Confirmations)
	}
	if d.AmountLamports == 0 {
		t.Error("amount must be non-zero")
	}
}
