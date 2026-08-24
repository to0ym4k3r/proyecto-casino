package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationsDir is the repository migrations folder relative to this package.
var migrationsDir = filepath.Join("..", "..", "migrations")

// migrationByVersion loads the real migrations directory and returns the
// migration with the given version, failing the test when it is missing.
func migrationByVersion(t *testing.T, version string) Migration {
	t.Helper()
	migrations, err := LoadMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("LoadMigrations(%s): %v", migrationsDir, err)
	}
	for _, m := range migrations {
		if m.Version == version {
			return m
		}
	}
	t.Fatalf("migration %s not found in %s (loaded %d files)", version, migrationsDir, len(migrations))
	return Migration{}
}

func TestMigrationsDirLoadsKycAndAuthInOrder(t *testing.T) {
	migrations, err := LoadMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("LoadMigrations(%s): %v", migrationsDir, err)
	}

	wantVersions := []string{"001", "002", "003", "004", "005", "006"}
	wantPaths := []string{"001_init.sql", "002_kyc.sql", "003_auth.sql", "004_solana_wallet.sql", "005_game_sessions.sql", "006_bonuses.sql"}
	if len(migrations) != len(wantVersions) {
		t.Fatalf("loaded %d migrations, want %d (%v); got %v", len(migrations), len(wantVersions), wantVersions, migrationPaths(migrations))
	}
	for i, m := range migrations {
		if m.Version != wantVersions[i] {
			t.Errorf("migrations[%d].Version = %q, want %q", i, m.Version, wantVersions[i])
		}
		if m.Path != wantPaths[i] {
			t.Errorf("migrations[%d].Path = %q, want %q", i, m.Path, wantPaths[i])
		}
	}
}

func migrationPaths(migrations []Migration) []string {
	paths := make([]string, 0, len(migrations))
	for _, m := range migrations {
		paths = append(paths, m.Path)
	}
	return paths
}

func TestMigration002KycSchemaContract(t *testing.T) {
	// contract pieces that production 002_kyc.sql MUST contain; each maps to a
	// spec/design requirement (users DOB + email_verified_at, submissions,
	// documents with FK to submissions, idempotent style of 001).
	required := []string{
		"ADD COLUMN IF NOT EXISTS date_of_birth",
		"ADD COLUMN IF NOT EXISTS email_verified_at",
		"CREATE TABLE IF NOT EXISTS kyc_submissions",
		"CREATE TABLE IF NOT EXISTS kyc_documents",
		"user_id UUID UNIQUE NOT NULL REFERENCES users(id)",
		"REFERENCES kyc_submissions(id)",
		"status VARCHAR(20) NOT NULL DEFAULT 'pending'",
	}
	missing := checkSQLContains(t, "002", required)
	if len(missing) > 0 {
		t.Errorf("002_kyc.sql missing required fragments: %v", missing)
	}
}

func TestMigration003AuthSchemaContract(t *testing.T) {
	required := []string{
		"CREATE TABLE IF NOT EXISTS refresh_tokens",
		"token_hash TEXT NOT NULL UNIQUE",
		"expires_at TIMESTAMPTZ NOT NULL",
		"ON DELETE CASCADE",
		"CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user",
		"CREATE TABLE IF NOT EXISTS email_verifications",
	}
	missing := checkSQLContains(t, "003", required)
	if len(missing) > 0 {
		t.Errorf("003_auth.sql missing required fragments: %v", missing)
	}
}

func checkSQLContains(t *testing.T, version string, fragments []string) []string {
	t.Helper()
	m := migrationByVersion(t, version)
	var missing []string
	for _, fragment := range fragments {
		if !strings.Contains(m.SQL, fragment) {
			missing = append(missing, fragment)
		}
	}
	return missing
}

// TestApplyMigrationsFreshExistingAndPartial exercises ApplyMigrations against
// a real Postgres 16 instance: fresh apply, idempotent re-run on an existing
// volume, and the "already at 001" upgrade path from the spec. It is skipped
// unless TEST_DATABASE_URL points at a disposable database.
func TestApplyMigrationsFreshExistingAndPartial(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping live database migration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	// Reset to a pristine database, mirroring an empty Postgres 16 volume.
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}

	migrations, err := LoadMigrations(migrationsDir)
	if err != nil {
		t.Fatalf("LoadMigrations: %v", err)
	}

	// Fresh volume: every migration applies in order and is recorded.
	if n, err := ApplyMigrations(ctx, pool, migrations); err != nil {
		t.Fatalf("fresh apply: %v", err)
	} else if n != 3 {
		t.Errorf("fresh apply applied %d, want 3", n)
	}
	assertTrackedVersions(t, ctx, pool, "001", "002", "003")
	assertObjectsExist(t, ctx, pool)

	// Existing volume: a second run is a no-op.
	if n, err := ApplyMigrations(ctx, pool, migrations); err != nil {
		t.Fatalf("second apply: %v", err)
	} else if n != 0 {
		t.Errorf("existing volume re-applied %d migrations, want 0", n)
	}

	// Database already at 001 (spec scenario): only 002 and 003 apply.
	if _, err := pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version <> '001'`); err != nil {
		t.Fatalf("simulate at-001 state: %v", err)
	}
	if n, err := ApplyMigrations(ctx, pool, migrations); err != nil {
		t.Fatalf("at-001 apply: %v", err)
	} else if n != 2 {
		t.Errorf("at-001 apply applied %d, want 2 (only 002 and 003)", n)
	}
	assertTrackedVersions(t, ctx, pool, "001", "002", "003")
	assertObjectsExist(t, ctx, pool)
}

func assertTrackedVersions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want ...string) {
	t.Helper()
	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		t.Fatalf("appliedVersions: %v", err)
	}
	for _, version := range want {
		if _, ok := applied[version]; !ok {
			t.Errorf("version %s missing from schema_migrations (have %v)", version, applied)
		}
	}
	if len(applied) != len(want) {
		t.Errorf("schema_migrations has %d versions, want %d (%v)", len(applied), len(want), applied)
	}
}

// assertObjectsExist proves the migrations created real schema objects, not
// just version rows: the KYC columns on users, the four new tables, and the
// refresh-token index.
func assertObjectsExist(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	for _, table := range []string{"kyc_submissions", "kyc_documents", "refresh_tokens", "email_verifications"} {
		var oid string
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.'||$1)::text`, table).Scan(&oid); err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		if oid == "" {
			t.Errorf("table %s does not exist after migrations", table)
		}
	}

	var columns int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'users' AND column_name IN ('date_of_birth', 'email_verified_at')`,
	).Scan(&columns); err != nil {
		t.Fatalf("query user columns: %v", err)
	}
	if columns != 2 {
		t.Errorf("users has %d/2 expected KYC columns (date_of_birth, email_verified_at)", columns)
	}

	var hasIndex bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'refresh_tokens' AND indexname = 'idx_refresh_tokens_user'
		)`).Scan(&hasIndex); err != nil {
		t.Fatalf("query refresh_tokens index: %v", err)
	}
	if !hasIndex {
		t.Error("index idx_refresh_tokens_user does not exist")
	}
}
