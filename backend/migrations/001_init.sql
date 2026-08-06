-- Casino Backend Initial Schema (migration 001)
-- Applied automatically by `make migrate` (cmd/migrate) and by the postgres
-- docker entrypoint on a fresh volume. Statements are idempotent so re-running
-- against an already-initialized database is safe.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE deleted_at IS NULL;

-- Wallets table
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance NUMERIC(15,2) DEFAULT 0.00 NOT NULL CHECK (balance >= 0),
    currency VARCHAR(3) DEFAULT 'USD' NOT NULL,
    version INTEGER DEFAULT 1 NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);

-- Games table
CREATE TABLE IF NOT EXISTS games (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(100) NOT NULL,
    min_bet NUMERIC(10,2) NOT NULL CHECK (min_bet > 0),
    max_bet NUMERIC(10,2) NOT NULL CHECK (max_bet > 0),
    rtp NUMERIC(5,2) NOT NULL CHECK (rtp >= 0 AND rtp <= 100),
    status VARCHAR(20) DEFAULT 'active' NOT NULL,
    cover_image VARCHAR(500),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_games_category ON games(category);
CREATE INDEX IF NOT EXISTS idx_games_status ON games(status);

-- Bets table
CREATE TABLE IF NOT EXISTS bets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    game_id UUID NOT NULL REFERENCES games(id),
    amount NUMERIC(10,2) NOT NULL CHECK (amount > 0),
    result NUMERIC(10,2),
    status VARCHAR(20) DEFAULT 'pending' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_bets_user_id ON bets(user_id);
CREATE INDEX IF NOT EXISTS idx_bets_game_id ON bets(game_id);
CREATE INDEX IF NOT EXISTS idx_bets_status ON bets(status);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    type VARCHAR(20) NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    balance_after NUMERIC(15,2) NOT NULL,
    reference VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);

-- Seed sample games only when the table is empty, so re-running is safe.
INSERT INTO games (name, description, category, min_bet, max_bet, rtp, status)
SELECT * FROM (VALUES
    ('Lucky Slots', 'Classic 5-reel slot machine with bonus rounds', 'slots', 0.10, 100.00, 96.50, 'active'),
    ('Blackjack Pro', 'Professional blackjack with side bets', 'card_games', 1.00, 500.00, 99.50, 'active'),
    ('Roulette Royale', 'European roulette with progressive jackpot', 'table_games', 0.50, 1000.00, 97.30, 'active'),
    ('Poker Hold''em', 'Texas Hold''em poker with tournaments', 'card_games', 5.00, 2000.00, 97.80, 'active'),
    ('Crash', 'Multiplied crash game with auto-cashout', 'instant', 0.10, 500.00, 97.00, 'active'),
    ('Dice Roll', 'Simple dice game with instant payouts', 'instant', 0.05, 100.00, 98.00, 'active')
) AS seed(name, description, category, min_bet, max_bet, rtp, status)
WHERE NOT EXISTS (SELECT 1 FROM games);
