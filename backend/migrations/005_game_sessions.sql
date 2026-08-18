-- Casino Games: session tracking, bet enhancements, and game RTP migration (005)

-- ============================================================
-- PART 1: Game sessions table
-- ============================================================

CREATE TABLE IF NOT EXISTS game_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    game_id UUID NOT NULL REFERENCES games(id),
    server_seed VARCHAR(64) NOT NULL,
    server_seed_hash VARCHAR(64) NOT NULL,
    status VARCHAR(20) DEFAULT 'active' NOT NULL,
    balance_snapshot BIGINT DEFAULT 0,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_game_sessions_user_id ON game_sessions(user_id, started_at);
CREATE INDEX IF NOT EXISTS idx_game_sessions_status ON game_sessions(status);

-- ============================================================
-- PART 2: Enhance bets table with session link, multiplier, payout
-- ============================================================

ALTER TABLE bets ADD COLUMN IF NOT EXISTS session_id UUID REFERENCES game_sessions(id);
ALTER TABLE bets ADD COLUMN IF NOT EXISTS multiplier DECIMAL(10,4);
ALTER TABLE bets ADD COLUMN IF NOT EXISTS payout BIGINT DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_bets_session_id ON bets(session_id) WHERE session_id IS NOT NULL;

-- ============================================================
-- PART 3: Migrate games table columns
-- ============================================================

-- min_bet / max_bet: NUMERIC(10,2) → BIGINT (integer cents)
ALTER TABLE games ALTER COLUMN min_bet DROP DEFAULT;
ALTER TABLE games ALTER COLUMN min_bet TYPE BIGINT USING (min_bet * 100)::BIGINT;
ALTER TABLE games ALTER COLUMN min_bet SET DEFAULT 0;

ALTER TABLE games ALTER COLUMN max_bet DROP DEFAULT;
ALTER TABLE games ALTER COLUMN max_bet TYPE BIGINT USING (max_bet * 100)::BIGINT;
ALTER TABLE games ALTER COLUMN max_bet SET DEFAULT 0;

-- rtp: NUMERIC(5,2) → INTEGER (0–100)
ALTER TABLE games ALTER COLUMN rtp DROP DEFAULT;
ALTER TABLE games ALTER COLUMN rtp TYPE INTEGER USING ROUND(rtp)::INTEGER;
ALTER TABLE games ALTER COLUMN rtp SET DEFAULT 95;

-- ============================================================
-- ROLLBACK (down migration)
-- ============================================================

-- DROP INDEX IF EXISTS idx_bets_session_id;
-- ALTER TABLE bets DROP COLUMN IF EXISTS payout;
-- ALTER TABLE bets DROP COLUMN IF EXISTS multiplier;
-- ALTER TABLE bets DROP COLUMN IF EXISTS session_id;

-- ALTER TABLE games ALTER COLUMN rtp TYPE NUMERIC(5,2) USING rtp::NUMERIC;
-- ALTER TABLE games ALTER COLUMN rtp SET DEFAULT 96.50;

-- ALTER TABLE games ALTER COLUMN max_bet TYPE NUMERIC(10,2) USING (max_bet / 100.0);
-- ALTER TABLE games ALTER COLUMN max_bet SET DEFAULT 100.00;

-- ALTER TABLE games ALTER COLUMN min_bet TYPE NUMERIC(10,2) USING (min_bet / 100.0);
-- ALTER TABLE games ALTER COLUMN min_bet SET DEFAULT 1.00;

-- DROP INDEX IF EXISTS idx_game_sessions_status;
-- DROP INDEX IF EXISTS idx_game_sessions_user_id;
-- DROP TABLE IF EXISTS game_sessions;
