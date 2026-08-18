package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// DepositDetector polls Solana RPC for new deposits on registered addresses.
type DepositDetector struct {
	rpcClient             *rpc.Client
	hdWallet              *HDWallet
	confirmationsRequired int
	pollInterval          time.Duration
}

// NewDepositDetector creates a new deposit detector.
func NewDepositDetector(rpcClient *rpc.Client, hdWallet *HDWallet, confirmationsRequired int) *DepositDetector {
	return &DepositDetector{
		rpcClient:             rpcClient,
		hdWallet:              hdWallet,
		confirmationsRequired: confirmationsRequired,
		pollInterval:          10 * time.Second,
	}
}

// DetectedDeposit represents a confirmed deposit detected on-chain.
type DetectedDeposit struct {
	Address        string
	TxHash         string
	AmountLamports uint64
	Confirmations  int
}

// PollSignatures fetches recent confirmed signatures for the given address
// and returns deposits that meet the confirmation threshold.
func (d *DepositDetector) PollSignatures(ctx context.Context, address string, lastSeenSignature *string) ([]DetectedDeposit, error) {
	pubkey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("parse address: %w", err)
	}

	limit := 100
	opts := &rpc.GetSignaturesForAddressOpts{
		Limit: &limit,
	}
	if lastSeenSignature != nil {
		sig, err := solana.SignatureFromBase58(*lastSeenSignature)
		if err != nil {
			return nil, fmt.Errorf("parse last seen signature: %w", err)
		}
		opts.Until = sig
	}

	sigs, err := d.rpcClient.GetSignaturesForAddressWithOpts(ctx, pubkey, opts)
	if err != nil {
		return nil, fmt.Errorf("get signatures for %s: %w", address, err)
	}

	var deposits []DetectedDeposit
	for _, sig := range sigs {
		if sig.ConfirmationStatus == rpc.ConfirmationStatusFinalized ||
			sig.ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
			deposits = append(deposits, DetectedDeposit{
				Address:        address,
				TxHash:         sig.Signature.String(),
				Confirmations:  int(sig.Slot),
			})
		}
	}

	return deposits, nil
}

// GetBalance fetches the SOL balance for an address in lamports.
func (d *DepositDetector) GetBalance(ctx context.Context, address string) (uint64, error) {
	pubkey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return 0, fmt.Errorf("parse address: %w", err)
	}

	balance, err := d.rpcClient.GetBalance(ctx, pubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get balance for %s: %w", address, err)
	}

	return uint64(balance.Value), nil
}

// GetTokenBalance fetches the USDC (or other SPL token) balance for an ATA.
func (d *DepositDetector) GetTokenBalance(ctx context.Context, ataAddress string) (uint64, error) {
	ata, err := solana.PublicKeyFromBase58(ataAddress)
	if err != nil {
		return 0, fmt.Errorf("parse ATA address: %w", err)
	}

	balance, err := d.rpcClient.GetTokenAccountBalance(ctx, ata, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get token balance for %s: %w", ataAddress, err)
	}

	var rawAmount uint64
	_, scanErr := fmt.Sscanf(balance.Value.Amount, "%d", &rawAmount)
	if scanErr != nil {
		return 0, fmt.Errorf("parse token amount %q: %w", balance.Value.Amount, scanErr)
	}

	return rawAmount, nil
}

// GetMultipleAccountsBatch fetches multiple accounts in a single RPC call.
// Splits into batches of maxBatchSize (default 100 per Solana RPC limit).
func (d *DepositDetector) GetMultipleAccountsBatch(ctx context.Context, addresses []string, maxBatchSize int) ([][]byte, error) {
	if maxBatchSize <= 0 {
		maxBatchSize = 100
	}

	var allAccounts [][]byte

	for i := 0; i < len(addresses); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(addresses) {
			end = len(addresses)
		}

		batch := addresses[i:end]
		pubkeys := make([]solana.PublicKey, len(batch))
		for j, addr := range batch {
			pk, err := solana.PublicKeyFromBase58(addr)
			if err != nil {
				return nil, fmt.Errorf("parse address %s: %w", addr, err)
			}
			pubkeys[j] = pk
		}

		result, err := d.rpcClient.GetMultipleAccounts(ctx, pubkeys...)
		if err != nil {
			return nil, fmt.Errorf("get multiple accounts batch %d: %w", i/maxBatchSize, err)
		}

		for _, acct := range result.Value {
			if acct != nil {
				allAccounts = append(allAccounts, acct.Data.GetBinary())
			} else {
				allAccounts = append(allAccounts, nil)
			}
		}
	}

	log.Printf("fetched %d accounts in %d batches", len(allAccounts), (len(addresses)+maxBatchSize-1)/maxBatchSize)
	return allAccounts, nil
}
