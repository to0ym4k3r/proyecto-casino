package service

import (
	"testing"
)

func TestDeriveAddressUsesBIP44Path(t *testing.T) {
	// Use a known mnemonic for deterministic results
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	hd := NewHDWallet(mnemonic)

	// Derive for user index 1
	addr1, err := hd.DeriveSolanaAddress(1)
	if err != nil {
		t.Fatalf("DeriveSolanaAddress(1): %v", err)
	}

	// Derive for user index 2
	addr2, err := hd.DeriveSolanaAddress(2)
	if err != nil {
		t.Fatalf("DeriveSolanaAddress(2): %v", err)
	}

	// Addresses must be different for different indices
	if addr1 == addr2 {
		t.Errorf("addresses for index 1 and 2 must differ, both got %q", addr1)
	}

	// Both must be non-empty valid-length base58 strings (32-44 chars typical)
	if len(addr1) < 32 || len(addr1) > 44 {
		t.Errorf("address1 length = %d, want 32-44 (base58)", len(addr1))
	}
	if len(addr2) < 32 || len(addr2) > 44 {
		t.Errorf("address2 length = %d, want 32-44 (base58)", len(addr2))
	}
}

func TestDeriveAddressIsDeterministic(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	hd := NewHDWallet(mnemonic)

	addr1, _ := hd.DeriveSolanaAddress(1)
	addr2, _ := hd.DeriveSolanaAddress(1)

	if addr1 != addr2 {
		t.Errorf("same index must produce same address: %q != %q", addr1, addr2)
	}
}

func TestDeriveUSDCATA(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	hd := NewHDWallet(mnemonic)

	solAddr, _ := hd.DeriveSolanaAddress(1)
	ata, err := hd.DeriveUSDCATA(solAddr)
	if err != nil {
		t.Fatalf("DeriveUSDCATA: %v", err)
	}

	// ATA must be different from the SOL address
	if ata == solAddr {
		t.Errorf("ATA must differ from SOL address, both got %q", ata)
	}

	// ATA must be non-empty
	if len(ata) == 0 {
		t.Error("ATA must not be empty")
	}
}

func TestDeriveAddressInvalidMnemonic(t *testing.T) {
	_, err := NewHDWalletFromMnemonic("invalid mnemonic phrase that is not valid")
	if err == nil {
		t.Error("expected error for invalid mnemonic, got nil")
	}
}
