-- Solana wallet infrastructure (migration 004)
-- Migrates monetary columns from NUMERIC to BIGINT (integer cents),
-- adds Solana-specific tables for HD derivation, deposits, withdrawals,
-- and on-chain transaction tracking.

-- ============================================================
-- PART 1: Migrate existing monetary columns to BIGINT
-- ============================================================

-- Wallets: balance NUMERIC(15,2) → BIGINT (integer cents)
ALTER TABLE wallets ALTER COLUMN balance DROP DEFAULT;
ALTER TABLE wallets ALTER COLUMN balance TYPE BIGINT USING (balance * 100)::BIGINT;
ALTER TABLE wallets ALTER COLUMN balance SET DEFAULT 0;
ALTER TABLE wallets ADD CONSTRAINT wallets_balance_non_negative CHECK (balance >= 0);

-- Transactions: amount and balance_after NUMERIC(15,2) → BIGINT
ALTER TABLE transactions ALTER COLUMN amount TYPE BIGINT USING (amount * 100)::BIGINT;
ALTER TABLE transactions ALTER COLUMN balance_after TYPE BIGINT USING (balance_after * 100)::BIGINT;

-- Bets: amount and result NUMERIC(10,2) → BIGINT
ALTER TABLE bets ALTER COLUMN amount TYPE BIGINT USING (amount * 100)::BIGINT;
ALTER TABLE bets ALTER COLUMN result TYPE BIGINT USING (result * 100)::BIGINT;

-- ============================================================
-- PART 2: Solana wallet columns on existing wallets table
-- ============================================================

ALTER TABLE wallets ADD COLUMN IF NOT EXISTS solana_address TEXT;
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS usdc_ata TEXT;

-- ============================================================
-- PART 3: New tables for Solana integration
-- ============================================================

-- Sequence for per-user HD derivation index (atomic allocation)
CREATE SEQUENCE IF NOT EXISTS user_deposit_index_seq START WITH 1 INCREMENT BY 1;

-- Deposit addresses: deterministic per-user Solana addresses
CREATE TABLE IF NOT EXISTS deposit_addresses (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    user_index INT UNIQUE NOT NULL DEFAULT nextval('user_deposit_index_seq'),
    sol_address TEXT NOT NULL,
    usdc_ata TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deposit_addresses_user_index ON deposit_addresses(user_index);

-- Withdrawals: crypto withdrawal requests with approval workflow
CREATE TABLE IF NOT EXISTS withdrawals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(10) NOT NULL,
    to_address TEXT NOT NULL,
    tx_hash TEXT,
    status VARCHAR(20) DEFAULT 'pending' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_withdrawals_user_id ON withdrawals(user_id);
CREATE INDEX IF NOT EXISTS idx_withdrawals_status ON withdrawals(status);

-- Solana transactions: on-chain transaction tracking with metadata
CREATE TABLE IF NOT EXISTS solana_txs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    tx_hash TEXT NOT NULL,
    type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    from_address TEXT,
    to_address TEXT,
    confirmations INT DEFAULT 0,
    network_fee BIGINT DEFAULT 0,
    usd_value BIGINT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_solana_txs_wallet_id ON solana_txs(wallet_id);
CREATE INDEX IF NOT EXISTS idx_solana_txs_tx_hash ON solana_txs(tx_hash);

-- Emergency pause flag (runtime config via DB)
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS emergency_pause BOOLEAN DEFAULT FALSE;
