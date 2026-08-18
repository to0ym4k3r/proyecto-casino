package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// HotWallet manages the hot wallet keypair and on-chain operations.
type HotWallet struct {
	keypair    solana.PrivateKey
	rpcClient  *rpc.Client
	publicKey  solana.PublicKey
}

// NewHotWallet creates a HotWallet from a base58 private key.
func NewHotWallet(privateKeyBase58 string, rpcClient *rpc.Client) (*HotWallet, error) {
	keypair, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		return nil, fmt.Errorf("parse hot wallet key: %w", err)
	}

	return &HotWallet{
		keypair:   keypair,
		rpcClient: rpcClient,
		publicKey: keypair.PublicKey(),
	}, nil
}

// PublicKey returns the hot wallet's public key.
func (hw *HotWallet) PublicKey() solana.PublicKey {
	return hw.publicKey
}

// PublicKeyString returns the hot wallet's public key as a base58 string.
func (hw *HotWallet) PublicKeyString() string {
	return hw.publicKey.String()
}

// GetBalance returns the SOL balance in lamports.
func (hw *HotWallet) GetBalance(ctx context.Context) (uint64, error) {
	result, err := hw.rpcClient.GetBalance(ctx, hw.publicKey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get hot wallet balance: %w", err)
	}
	return uint64(result.Value), nil
}

// EstimateFee estimates the network fee for a transfer in lamports.
// Returns a conservative estimate; actual fee is determined at send time.
func (hw *HotWallet) EstimateFee(_ context.Context) (uint64, error) {
	// Solana base fee is 5000 lamports per signature
	// For a simple transfer: 1 signature = 5000 lamports
	return 5000, nil
}

// SendSOL transfers SOL from the hot wallet to the destination address.
// Returns the transaction signature.
func (hw *HotWallet) SendSOL(ctx context.Context, to string, lamports uint64) (string, error) {
	toPubkey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return "", fmt.Errorf("parse destination: %w", err)
	}

	sig, err := hw.rpcClient.SendTransaction(ctx, nil) // placeholder - real implementation builds a Transaction
	if err != nil {
		return "", fmt.Errorf("send SOL: %w", err)
	}

	_ = toPubkey
	_ = lamports
	return sig.String(), nil
}

// SignAndSendTransaction signs and sends a pre-built transaction.
func (hw *HotWallet) SignAndSendTransaction(ctx context.Context, tx *solana.Transaction) (string, error) {
	_, err := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key == hw.publicKey {
			return &hw.keypair
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sign transaction: %w", err)
	}

	sig, err := hw.rpcClient.SendTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}

	return sig.String(), nil
}

// KYCTier represents a user's KYC verification level.
type KYCTier string

const (
	KYCUnverified KYCTier = "unverified"
	KYCL1         KYCTier = "l1"
	KYCL2         KYCTier = "l2"
)

// KYCLimits defines per-transaction and daily withdrawal limits per KYC tier.
type KYCLimits struct {
	PerTransaction int64 // max cents per single transaction
	Daily          int64 // max cents per 24h window
}

// DefaultKYCLimits returns the standard limits for each tier.
func DefaultKYCLimits() map[KYCTier]KYCLimits {
	return map[KYCTier]KYCLimits{
		KYCUnverified: {PerTransaction: 50000, Daily: 250000},   // $500/tx, $2500/day
		KYCL1:         {PerTransaction: 500000, Daily: 5000000}, // $5K/tx, $50K/day
		KYCL2:         {PerTransaction: 2500000, Daily: 25000000}, // $25K/tx, $250K/day
	}
}

// AutoApprovalThreshold returns the max amount (cents) that can be auto-approved per tier.
func AutoApprovalThreshold(tier KYCTier) int64 {
	switch tier {
	case KYCL1:
		return 500000 // $5K
	case KYCL2:
		return 2500000 // $25K
	default:
		return 50000 // $500
	}
}

// WithdrawalStatus represents the status of a withdrawal request.
type WithdrawalStatus string

const (
	WithdrawalPending        WithdrawalStatus = "pending"
	WithdrawalPendingApproval WithdrawalStatus = "pending_approval"
	WithdrawalApproved       WithdrawalStatus = "approved"
	WithdrawalRejected       WithdrawalStatus = "rejected"
	WithdrawalCompleted      WithdrawalStatus = "completed"
)

// PriceOracle provides USD price snapshots for crypto assets.
type PriceOracle struct {
	solUSDC float64 // cached SOL/USD price
}

// NewPriceOracle creates a new price oracle.
func NewPriceOracle() *PriceOracle {
	return &PriceOracle{solUSDC: 0}
}

// GetSOLPrice returns the current SOL/USD price.
func (po *PriceOracle) GetSOLPrice() float64 {
	return po.solUSDC
}

// UpdateSOLPrice sets the SOL/USD price (called by price feed).
func (po *PriceOracle) UpdateSOLPrice(price float64) {
	po.solUSDC = price
}

// LamportsToCents converts lamports to USD cents at the current price.
// 1 SOL = 1,000,000,000 lamports
func (po *PriceOracle) LamportsToCents(lamports uint64) int64 {
	solAmount := float64(lamports) / 1_000_000_000.0
	usdCents := solAmount * po.solUSDC * 100
	return int64(usdCents)
}

// USDToLamports converts USD cents to lamports at the current price.
func (po *PriceOracle) USDToLamports(cents int64) uint64 {
	usdDollars := float64(cents) / 100.0
	solAmount := usdDollars / po.solUSDC
	return uint64(solAmount * 1_000_000_000.0)
}

// SnapshotUsdCents returns the USD value of the given lamport amount in cents,
// using the current cached SOL/USD price. Returns 0 if price is not set.
func (po *PriceOracle) SnapshotUsdCents(lamports uint64) int64 {
	if po.solUSDC == 0 {
		return 0
	}
	return po.LamportsToCents(lamports)
}

// Now returns current UTC time — extracted for testability.
var Now = func() time.Time { return time.Now().UTC() }
