-- Bonus system tables (migration 006)

-- ============================================================
-- PART 1: Bonus definitions — configurable bonus offers
-- ============================================================

CREATE TABLE IF NOT EXISTS bonus_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    wagering_multiplier INT NOT NULL DEFAULT 0,
    expiry_days INT NOT NULL DEFAULT 30,
    max_bonus_amount_cents BIGINT NOT NULL,
    min_deposit_cents BIGINT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- PART 2: Bonus grants — per-user bonus instances
-- ============================================================

CREATE TABLE IF NOT EXISTS bonus_grants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    bonus_definition_id UUID NOT NULL REFERENCES bonus_definitions(id),
    bonus_amount_cents BIGINT NOT NULL,
    wagering_required_cents BIGINT NOT NULL,
    total_wagered_cents BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    granted_by VARCHAR(20) NOT NULL DEFAULT 'system',
    admin_id UUID,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- One active grant per definition per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_bonus_grants_active_unique
    ON bonus_grants(user_id, bonus_definition_id) WHERE status = 'active';

CREATE INDEX IF NOT EXISTS idx_bonus_grants_user ON bonus_grants(user_id, status);
CREATE INDEX IF NOT EXISTS idx_bonus_grants_expires ON bonus_grants(expires_at) WHERE status = 'active';

-- ============================================================
-- PART 3: Bonus ledger — immutable audit trail
-- ============================================================

CREATE TABLE IF NOT EXISTS bonus_ledger (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    grant_id UUID NOT NULL REFERENCES bonus_grants(id),
    transaction_type VARCHAR(30) NOT NULL,
    amount_cents BIGINT NOT NULL,
    reference_type VARCHAR(30),
    reference_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bonus_ledger_grant ON bonus_ledger(grant_id);

-- ============================================================
-- PART 4: Add bonus_balance to wallets
-- ============================================================

ALTER TABLE wallets ADD COLUMN IF NOT EXISTS bonus_balance BIGINT NOT NULL DEFAULT 0;

-- ============================================================
-- PART 5: Seed default bonus definitions
-- ============================================================

INSERT INTO bonus_definitions (code, name, description, wagering_multiplier, expiry_days, max_bonus_amount_cents, config)
VALUES
    ('welcome', 'Welcome Bonus', '100% match on your first deposit up to $500', 30, 30, 50000, '{"match_percent": 100}'),
    ('reload', 'Reload Bonus', '50% match on deposits over $50', 25, 14, 25000, '{"match_percent": 50, "min_deposit_cents": 5000}'),
    ('cashback', 'Weekly Cashback', '10%% cashback on weekly losses', 0, 7, 10000, '{"cashback_percent": 10, "loss_period_hours": 168}'),
    ('free_spins', 'Free Spins', '50 free spins on selected slots', 20, 7, 5000, '{"spins_count": 50, "game_id": null}')
ON CONFLICT (code) DO NOTHING;
