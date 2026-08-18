package config

import (
	"os"
	"testing"
)

func TestSolanaConfigLoadsFromEnv(t *testing.T) {
	t.Setenv("SOLANA_RPC_URL", "https://api.testnet.solana.com")
	t.Setenv("SOLANA_HOT_WALLET_KEY", "test-hot-wallet-key-32chars!!")
	t.Setenv("SOLANA_NETWORK", "testnet")
	t.Setenv("SOLANA_COMMITMENT", "finalized")
	t.Setenv("SOLANA_CONFIRMATIONS", "32")

	cfg := Load()

	if cfg.Solana.RPCURL != "https://api.testnet.solana.com" {
		t.Errorf("RPCURL = %q, want %q", cfg.Solana.RPCURL, "https://api.testnet.solana.com")
	}
	if cfg.Solana.HotWalletKey != "test-hot-wallet-key-32chars!!" {
		t.Errorf("HotWalletKey not loaded correctly")
	}
	if cfg.Solana.Network != "testnet" {
		t.Errorf("Network = %q, want %q", cfg.Solana.Network, "testnet")
	}
	if cfg.Solana.CommitmentLevel != "finalized" {
		t.Errorf("CommitmentLevel = %q, want %q", cfg.Solana.CommitmentLevel, "finalized")
	}
	if cfg.Solana.ConfirmationsRequired != 32 {
		t.Errorf("ConfirmationsRequired = %d, want 32", cfg.Solana.ConfirmationsRequired)
	}
}

func TestSolanaConfigDefaults(t *testing.T) {
	// Ensure no Solana env vars are set
	os.Unsetenv("SOLANA_RPC_URL")
	os.Unsetenv("SOLANA_HOT_WALLET_KEY")
	os.Unsetenv("SOLANA_NETWORK")
	os.Unsetenv("SOLANA_COMMITMENT")
	os.Unsetenv("SOLANA_CONFIRMATIONS")

	cfg := Load()

	if cfg.Solana.RPCURL != "" {
		t.Errorf("RPCURL = %q, want empty default", cfg.Solana.RPCURL)
	}
	if cfg.Solana.Network != "" {
		t.Errorf("Network = %q, want empty default", cfg.Solana.Network)
	}
	if cfg.Solana.CommitmentLevel != "" {
		t.Errorf("CommitmentLevel = %q, want empty default", cfg.Solana.CommitmentLevel)
	}
	if cfg.Solana.ConfirmationsRequired != 32 {
		t.Errorf("ConfirmationsRequired = %d, want default 32", cfg.Solana.ConfirmationsRequired)
	}
}

func TestValidateSolanaConfigMissingHotWalletKey(t *testing.T) {
	cfg := &Config{
		Solana: SolanaConfig{
			RPCURL: "https://api.testnet.solana.com",
			// HotWalletKey intentionally empty
		},
	}

	err := cfg.ValidateSolanaConfig()
	if err == nil {
		t.Error("expected error for missing SOLANA_HOT_WALLET_KEY, got nil")
	}
}

func TestValidateSolanaConfigMissingRPCURL(t *testing.T) {
	cfg := &Config{
		Solana: SolanaConfig{
			HotWalletKey: "some-key",
			// RPCURL intentionally empty
		},
	}

	err := cfg.ValidateSolanaConfig()
	if err == nil {
		t.Error("expected error for missing SOLANA_RPC_URL, got nil")
	}
}

func TestValidateSolanaConfigValid(t *testing.T) {
	cfg := &Config{
		Solana: SolanaConfig{
			RPCURL:               "https://api.testnet.solana.com",
			HotWalletKey:         "some-base58-key",
			Network:              "testnet",
			CommitmentLevel:      "finalized",
			ConfirmationsRequired: 32,
		},
	}

	err := cfg.ValidateSolanaConfig()
	if err != nil {
		t.Errorf("expected no error for valid config, got %v", err)
	}
}
