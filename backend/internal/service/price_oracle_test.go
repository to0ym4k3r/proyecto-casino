package service

import (
	"testing"
)

func TestPriceOracleSnapshotCents(t *testing.T) {
	po := NewPriceOracle()
	po.UpdateSOLPrice(150.00) // $150.00 per SOL

	// 1.5 SOL = 1,500,000,000 lamports
	snapshot := po.SnapshotUsdCents(1_500_000_000)
	// 1.5 SOL * $150 = $225.00 = 22500 cents
	if snapshot != 22500 {
		t.Errorf("SnapshotUsdCents(1.5 SOL @ $150) = %d, want 22500", snapshot)
	}
}

func TestPriceOracleSnapshotZeroPrice(t *testing.T) {
	po := NewPriceOracle()
	// No price set — price = 0
	snapshot := po.SnapshotUsdCents(1_000_000_000)
	if snapshot != 0 {
		t.Errorf("SnapshotUsdCents with zero price = %d, want 0", snapshot)
	}
}

func TestPriceOracleLamportsToCents(t *testing.T) {
	po := NewPriceOracle()
	po.UpdateSOLPrice(100.00) // $100 per SOL

	// 1 SOL = 1,000,000,000 lamports → $100 = 10000 cents
	cents := po.LamportsToCents(1_000_000_000)
	if cents != 10000 {
		t.Errorf("LamportsToCents(1 SOL @ $100) = %d, want 10000", cents)
	}
}

func TestPriceOracleUSDToLamports(t *testing.T) {
	po := NewPriceOracle()
	po.UpdateSOLPrice(100.00) // $100 per SOL

	// $50 = 5000 cents → 0.5 SOL = 500,000,000 lamports
	lamports := po.USDToLamports(5000)
	if lamports != 500_000_000 {
		t.Errorf("USDToLamports($50 @ $100/SOL) = %d, want 500000000", lamports)
	}
}
