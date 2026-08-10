package security

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"casino-backend/internal/database"
	"casino-backend/internal/domain"
	"casino-backend/internal/repository/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// connectTestDB connects to the disposable Postgres named by TEST_DATABASE_URL
// (skipping when unset) and ensures the schema is migrated. It is the same
// pattern used by internal/database/migrations_test.go.
func connectTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping live-database integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	migs, err := database.LoadMigrations(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if _, err := database.ApplyMigrations(ctx, pool, migs); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return pool
}

// insertUserRaw creates a minimal user row so wallet tests can satisfy the
// wallets_user_id_fkey foreign key without going through the auth flow.
func insertUserRaw(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, email, password_hash, first_name, last_name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, $7)`,
		id, "wallet-test-"+id.String()[:8], "wallet-test-"+id.String()[:8]+"@example.com",
		"$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		"Wallet", "Test", now)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
}

func countUsers(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	return n
}

// TestSQLiPayloadsTreatedAsDataAgainstRealPostgres runs the injection probes
// through the REAL parameterized queries (user_repo.go:72-119). Every query
// uses a bind parameter, so each payload is compared literally:
//
//   - a literal-match lookup returns only the exact payload row,
//   - classic probes ('admin'--, ' OR 1=1) never match any row,
//   - a "DROP TABLE users" probe leaves the schema and row count intact,
//   - a stacked "SELECT pg_sleep(10)" probe returns instantly (never executes).
func TestSQLiPayloadsTreatedAsDataAgainstRealPostgres(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	repo := postgres.NewPostgresUserRepo(pool)

	suffix := uuid.New().String()[:8]
	payloadUser := "' OR 1=1--"
	payloadEmail := "' OR '1'='1 " + suffix + "@example.com"

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE username=$1 OR email=$2`, payloadUser, payloadEmail); err != nil {
		t.Fatalf("pre-cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE username=$1 OR email=$2`, payloadUser, payloadEmail)
		pool.Close()
	})

	now := time.Now().UTC()
	u := &domain.User{
		ID: uuid.New(), Username: payloadUser, Email: payloadEmail,
		PasswordHash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
		FirstName:    "SQLi", LastName: "Probe", IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create payload user: %v", err)
	}

	t.Run("literal lookup matches only the payload row", func(t *testing.T) {
		got, err := repo.GetByUsername(ctx, payloadUser)
		if err != nil {
			t.Fatalf("GetByUsername(payload) errored: %v", err)
		}
		if got.Username != payloadUser {
			t.Errorf("got username %q, want the literal payload %q", got.Username, payloadUser)
		}
		for _, probe := range []string{"admin'--", "' OR 1=1", "admin'}--", "x' OR '1'='1"} {
			if got, err := repo.GetByUsername(ctx, probe); err == nil {
				t.Errorf("probe %q matched user %q — injection-like match", probe, got.Username)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("probe %q returned unexpected error: %v", probe, err)
			}
		}
	})

	t.Run("DROP TABLE probe leaves schema and rows intact", func(t *testing.T) {
		before := countUsers(t, ctx, pool)
		_, err := repo.GetByEmail(ctx, "x'; DROP TABLE users; --")
		if err == nil {
			t.Error("DROP probe returned a user")
		} else if !errors.Is(err, pgx.ErrNoRows) {
			t.Logf("DROP probe returned non-ErrNoRows error (still safe): %v", err)
		}

		var reg interface{}
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.users')::text`).Scan(&reg); err != nil || reg == nil {
			t.Fatalf("users table missing after DROP probe (reg=%v err=%v)", reg, err)
		}
		if after := countUsers(t, ctx, pool); after != before {
			t.Errorf("users count changed: before=%d after=%d", before, after)
		}
	})

	t.Run("stacked pg_sleep probe is never executed", func(t *testing.T) {
		start := time.Now()
		_, err := repo.GetByEmail(ctx, "1;SELECT pg_sleep(10); --")
		elapsed := time.Since(start)
		if err == nil {
			t.Error("sleep probe returned a user")
		}
		if elapsed > 2*time.Second {
			t.Errorf("query took %v — stacked SELECT pg_sleep(10) likely executed (injection)", elapsed)
		} else {
			t.Logf("pg_sleep probe handled as data in %v", elapsed)
		}
	})
}

// TestWalletOverdrawBlockedByCheckConstraint confirms the database-level
// defense: wallets.balance has CHECK (balance >= 0) (migrations/001_init.sql:30),
// so an overdraw attempt fails even though the UPDATE SQL has no explicit
// guard clause.
func TestWalletOverdrawBlockedByCheckConstraint(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	walletRepo := postgres.NewPostgresWalletRepo(pool)

	uid := uuid.New()
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, uid); err != nil {
		t.Fatalf("pre-cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM wallets WHERE user_id=$1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, uid)
		pool.Close()
	})
	insertUserRaw(t, ctx, pool, uid)

	now := time.Now().UTC()
	w := &domain.Wallet{ID: uuid.New(), UserID: uid, Balance: 100, Currency: "USD", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := walletRepo.Create(ctx, w); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	if err := walletRepo.UpdateBalance(ctx, w.ID, -500, w.Version); err == nil {
		t.Error("overdraw succeeded — CHECK (balance >= 0) did not fire")
	} else {
		t.Logf("overdraw rejected by DB (expected): %v", err)
	}

	got, err := walletRepo.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance != 100 {
		t.Errorf("balance after failed overdraw = %v, want 100", got.Balance)
	}
	if got.Version != 1 {
		t.Errorf("version after failed overdraw = %v, want 1 (unchanged)", got.Version)
	}
}

// TestWalletOptimisticLockVersionConflict confirms optimistic locking
// (wallet_repo.go:91-106): when two concurrent updates start from the same
// version, exactly one wins and the loser gets a version conflict. No balance
// is lost.
func TestWalletOptimisticLockVersionConflict(t *testing.T) {
	pool := connectTestDB(t)
	ctx := context.Background()
	walletRepo := postgres.NewPostgresWalletRepo(pool)

	uid := uuid.New()
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, uid); err != nil {
		t.Fatalf("pre-cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM wallets WHERE user_id=$1`, uid)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, uid)
		pool.Close()
	})
	insertUserRaw(t, ctx, pool, uid)

	now := time.Now().UTC()
	w := &domain.Wallet{ID: uuid.New(), UserID: uid, Balance: 100, Currency: "USD", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := walletRepo.Create(ctx, w); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = walletRepo.UpdateBalance(ctx, w.ID, 10, 1) // both start from version 1
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		} else {
			t.Logf("conflicting update rejected (expected): %v", err)
		}
	}
	if wins != 1 {
		t.Errorf("exactly one optimistic update should win, got %d", wins)
	}
	got, err := walletRepo.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance != 110 {
		t.Errorf("balance = %v, want 110 (exactly one +10 applied)", got.Balance)
	}
}
