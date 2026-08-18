# Arquitectura del Sistema - Casino Online

## Vision General

El sistema sigue una arquitectura hexagonal (ports & adapters) en el backend Go, con Clean Architecture en el cliente Android.

## Componentes

### Backend Go

#### Domain Layer (core)
- `internal/domain/` — Entidades de negocio puras, sin dependencias externas
  - User, Wallet, Bet, Game, GameSession, Transaction
  - Monetary values: int64 cents (NOT float64 dollars)

#### Service Layer (use cases)
- `internal/service/` — Logica de negocio
  - AuthService: register, login, refresh, logout
  - ProfileService: get/update profile, KYC submit/status
  - WalletService: balance, deposit, withdraw (internal balance)
  - GameService: list, start, place bet, resolve bet

#### Handler Layer (adapters in)
- `internal/handler/` — HTTP handlers (chi router)
  - Auth, Profile, KYC, Wallet, Game handlers
  - Middleware: JWT auth, rate limiting, CORS, security headers

#### Repository Layer (adapters out)
- `internal/repository/postgres/` — PostgreSQL implementations
  - UserRepo, WalletRepo, BetRepo, GameRepo, SessionRepo, ProfileRepo, KycRepo
  - All use parameterized queries (SQL injection safe)

#### Solana Integration (NEW)
- `internal/service/` — Solana blockchain integration
  - `hot_wallet.go` — Hot wallet keypair management + SOL transfers
  - `hd_wallet.go` — BIP44 HD derivation for per-user deposit addresses
  - `deposit_detector.go` — Polling service for SOL/USDC deposits
  - `withdrawal_service.go` — Withdrawal execution with rate limiting

### Android Client

#### Presentation Layer
- `ui/screens/` — Compose screens
  - auth/ — Login, Register, Verify
  - profile/ — Profile, KYC
  - wallet/ — Balance, Deposit, Withdraw, TransactionHistory (NEW)
  - games/ — Game list, Game play

#### Data Layer
- `data/` — Repositories + API
  - AuthRepository, ProfileRepository, KycRepository
  - WalletRepository, CryptoRepository (NEW)
  - ApiService (Retrofit)

#### Domain Layer
- `domain/` — Use cases + models
  - Models: User, Wallet, Transaction, Bet, Game

## Flujo de Datos

### Deposit Flow (SOL/USDC)
1. User requests deposit address -> Backend derives via HD wallet (m/44'/501'/{userId}'/0')
2. Address returned to Android -> Displayed as QR code
3. User sends crypto from external wallet
4. Deposit detector polls every 10s -> Detects incoming transaction
5. Wait for Finalized commitment (~30s) -> Credit internal balance
6. User sees updated balance in Android app

### Withdrawal Flow
1. User requests withdrawal -> Backend validates:
   - Sufficient balance
   - Rate limits (3-tier KYC)
   - Daily outflow cap (30%)
   - Emergency pause not active
2. If auto-approved (below threshold) -> Execute immediately
3. If manual required -> Queue for admin approval
4. Build Solana transaction -> Sign via hot wallet key
5. Broadcast -> Wait for confirmation
6. Update transaction history

### Bet Flow
1. User places bet -> WalletService.Debit(amount)
2. Game resolves -> ResolveBet(amount) returns payout (integer math)
3. If win -> WalletService.Credit(payout)
4. Transaction recorded with full audit trail

## Seguridad

### Autenticacion
- JWT access tokens (15 min) + refresh tokens (7 days)
- bcrypt cost factor 12
- Rate limiting: 5 req/min auth, 100 req/min general

### Crypto Security
- Hot wallet key: env var (MVP), HSM (production)
- HD derivation: no per-user private keys stored
- Withdrawal: 3-tier rate limiting, emergency pause
- Sweep: PDA-based (Solana-native), batch atomic

## Base de Datos

### Tablas Principales
- `users` — User accounts + KYC status
- `wallets` — Balance (int64 cents), currency, version
- `transactions` — Full audit trail (amount, balance_after, type, tx_hash)
- `bets` — Bet history (amount, result, game_session_id)
- `games` — Game catalog (min_bet, max_bet, rtp)
- `game_sessions` — Active sessions
- `user_deposit_addresses` — HD-derived deposit addresses (NEW)
- `crypto_transactions` — Blockchain tx details (NEW)
- `withdrawal_requests` — Withdrawal queue (NEW)
- `system_config` — Emergency pause flag (NEW)

## Endpoints API

### Auth
- POST /api/v1/auth/register
- POST /api/v1/auth/login
- POST /api/v1/auth/refresh

### Profile
- GET /api/v1/users/me
- PUT /api/v1/users/me

### KYC
- POST /api/v1/kyc/submit
- GET /api/v1/kyc/status
- POST /api/v1/kyc/upload

### Wallet (Internal)
- GET /api/v1/wallet/balance
- POST /api/v1/wallet/deposit
- POST /api/v1/wallet/withdraw

### Wallet (Crypto) — NEW
- POST /api/v1/wallet/deposit/sol
- POST /api/v1/wallet/deposit/usdc
- POST /api/v1/wallet/withdraw/sol
- POST /api/v1/wallet/withdraw/usdc
- GET /api/v1/wallet/tx-history

### Games
- GET /api/v1/games/
- GET /api/v1/games/{gameID}
- POST /api/v1/games/start
- POST /api/v1/games/bet

### Admin
- POST /api/admin/emergency/pause

## Infraestructura

### Desarrollo
- Backend: `go run cmd/server/main.go`
- DB: PostgreSQL (Docker)
- Solana: Testnet (primary), Devnet (prototyping)

### Testing
- Backend: `go test ./...` (unit + integration)
- Android: `.\gradlew.bat testDebugUnitTest`
- TDD: Red -> Green -> Refactor

### Deployment
- Backend: Docker container
- Android: APK/AAB via Gradle
- Solana: Testnet -> Mainnet-beta
