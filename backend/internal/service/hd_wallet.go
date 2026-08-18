package service

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/tyler-smith/go-bip39"
)

// HDWallet derives per-user Solana deposit addresses from a mnemonic
// using BIP44 path m/44'/501'/{userIndex}'/0'.
type HDWallet struct {
	mnemonic string
}

// NewHDWallet creates an HDWallet from a mnemonic string.
func NewHDWallet(mnemonic string) *HDWallet {
	return &HDWallet{mnemonic: mnemonic}
}

// NewHDWalletFromMnemonic validates and creates an HDWallet.
func NewHDWalletFromMnemonic(mnemonic string) (*HDWallet, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic phrase")
	}
	return &HDWallet{mnemonic: mnemonic}, nil
}

// DeriveSolanaAddress derives a Solana public key for the given user index
// using BIP44 path m/44'/501'/{userIndex}'/0'.
//
// Solana uses ED25519 key pairs. The derivation path follows the standard
// BIP44 convention for Solana (coin type 501).
func (h *HDWallet) DeriveSolanaAddress(userIndex int) (string, error) {
	path := fmt.Sprintf("m/44'/501'/%d'/0'", userIndex)

	key, err := solana.PrivateKeyFromMnemonicAtPath(h.mnemonic, "", path)
	if err != nil {
		return "", fmt.Errorf("derive key for index %d: %w", userIndex, err)
	}

	return key.PublicKey().String(), nil
}

// DeriveUSDCATA derives the Associated Token Account address for USDC
// (mint EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v) given the owner address.
//
// ATA is deterministic from (owner address, mint, token program).
func (h *HDWallet) DeriveUSDCATA(ownerAddress string) (string, error) {
	owner, err := solana.PublicKeyFromBase58(ownerAddress)
	if err != nil {
		return "", fmt.Errorf("parse owner address: %w", err)
	}

	// USDC mint on all networks
	usdcMint := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	ata, _, err := solana.FindAssociatedTokenAddress(owner, usdcMint)
	if err != nil {
		return "", fmt.Errorf("derive ATA: %w", err)
	}

	return ata.String(), nil
}

// GetSolanaClient creates a new Solana RPC client for the given endpoint.
func GetSolanaClient(rpcURL string) *rpc.Client {
	return rpc.New(rpcURL)
}
