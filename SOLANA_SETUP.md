# Solana Setup Guide — Casino Online

## Prerequisites
- Go 1.21+
- Solana CLI tools
- Testnet SOL (faucet)

## 1. Install Solana CLI
```bash
sh -c "$(curl -sSfL https://release.solana.com/v1.18.4/install)"
solana --version
```

## 2. Configure for Testnet
```bash
solana config set --url https://api.testnet.solana.com
```

## 3. Generate Hot Wallet
```bash
# Generate new keypair
solana-keygen new --outfile ~/.config/solana/casino-hot-wallet.json

# Get public key
solana address -k ~/.config/solana/casino-hot-wallet.json

# Get Base58 private key for env var
solana-keygen recover --outfile ~/.config/solana/casino-hot-wallet.json
```

## 4. Fund Hot Wallet
```bash
# Request SOL from faucet (2 SOL per request)
solana airdrop 2 --url https://api.testnet.solana.com

# Check balance
solana balance --url https://api.testnet.solana.com
```

## 5. Configure Backend
Create `.env` in `backend/`:
```env
SOLANA_RPC_URL=https://api.testnet.solana.com
SOLANA_HOT_WALLET_KEY=<your-base58-private-key>
SOLANA_NETWORK=testnet
SOLANA_COMMITMENT=finalized
SOLANA_CONFIRMATIONS=32
SOLANA_MAX_OUTFLOW_PCT=30
SOLANA_USDC_MINT=EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v
```

## 6. Get Testnet USDC (if available)
```bash
# Check if USDC faucet is available on testnet
# Note: USDC on testnet may not have a faucet
# For testing, use mock USDC balance in integration tests
```

## 7. Verify Setup
```bash
cd backend
go test ./internal/service/... -v
```

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| SOLANA_RPC_URL | Solana RPC endpoint | https://api.testnet.solana.com |
| SOLANA_HOT_WALLET_KEY | Hot wallet private key (Base58) | (required) |
| SOLANA_NETWORK | Network: testnet, mainnet-beta | testnet |
| SOLANA_COMMITMENT | Commitment level: processed, confirmed, finalized | finalized |
| SOLANA_CONFIRMATIONS | Min confirmations for deposit | 32 |
| SOLANA_MAX_OUTFLOW_PCT | Max hot wallet outflow % per 24h | 30 |
| SOLANA_USDC_MINT | USDC mint address | EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v |

## Production Upgrade Path

### From Env Var to HSM
1. Provision AWS KMS key (Ed25519)
2. Replace `keypair.FromBase58()` with KMS signing client
3. Implement `solana.Signer` interface wrapping KMS
4. Update hot_wallet.go constructor

### From Testnet to Mainnet
1. Change `SOLANA_RPC_URL` to paid provider (Helius, QuickNode, Triton)
2. Change `SOLANA_NETWORK` to mainnet-beta
3. Fund hot wallet with real SOL
4. Test with small amounts first
5. Monitor hot wallet balance continuously
