package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/database"
	"casino-backend/internal/domain"
	"casino-backend/internal/repository/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// connectTestDB connects to the disposable Postgres named by TEST_DATABASE_URL
// (skipping when unset) and ensures the schema is migrated. The migrations
// directory lives at backend/migrations, one level up from internal/service.
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
	migs, err := database.LoadMigrations(filepath.Join("..", "migrations"))
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if _, err := database.ApplyMigrations(ctx, pool, migs); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return pool
}

// scriptedMailer fails the first dispatch and succeeds afterwards, letting one
// AuthService instance reproduce the failing-email rollback followed by a
// successful retry.
type scriptedMailer struct {
	calls     int
	failFirst bool
}

func (m *scriptedMailer) SendVerificationEmail(_ context.Context, _ string, _ string) error {
	m.calls++
	if m.failFirst && m.calls == 1 {
		return errors.New("smtp server unreachable")
	}
	return nil
}

// TestRegisterRetrySucceedsAfterVerificationEmailFailureLive proves the
// soft-delete brick is gone against a real Postgres: a verification-email
// dispatch failure must PHYSICALLY remove the just-created user row (freeing
// the plain UNIQUE email/username), so a retry with the same identity succeeds
// instead of failing with a permanent unique violation.
func TestRegisterRetrySucceedsAfterVerificationEmailFailureLive(t *testing.T) {
	pool := connectTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	suffix := uuid.New().String()[:8]
	email := "retry-" + suffix + "@example.com"
	username := "retry_" + suffix

	// Clean any leftover rows from a previous crashed run (cascade removes
	// wallet/verification/refresh children).
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE email = $1 OR username = $2`, email, username); err != nil {
		t.Fatalf("pre-cleanup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1 OR username = $2`, email, username)
	})

	userRepo := postgres.NewPostgresUserRepo(pool)
	regRepo := postgres.NewPostgresRegistrationRepo(pool)
	refreshRepo := postgres.NewPostgresRefreshTokenRepo(pool)
	verifyRepo := postgres.NewPostgresEmailVerificationRepo(pool)

	cfg := &config.JWTConfig{
		Secret:        "integration-test-secret-not-for-production",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	mailer := &scriptedMailer{failFirst: true}
	svc := NewAuthService(userRepo, regRepo, refreshRepo, verifyRepo, mailer, cfg, 24*time.Hour)

	req := domain.RegisterRequest{
		Username:    username,
		Email:       email,
		Password:    "password123",
		FirstName:   "Fn",
		LastName:    "Ln",
		DateOfBirth: time.Now().AddDate(-25, 0, 0).Format("2006-01-02"),
	}

	if _, err := svc.Register(ctx, req); !errors.Is(err, ErrVerificationEmailFailed) {
		t.Fatalf("first Register() error = %v, want ErrVerificationEmailFailed", err)
	}

	// The rollback compensation must have PHYSICALLY deleted the row.
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count); err != nil {
		t.Fatalf("count users after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("users row still present after rollback: want physical delete, got %d row(s)", count)
	}

	if _, err := svc.Register(ctx, req); err != nil {
		t.Fatalf("retry Register() after rollback = %v, want success (unique identities must be free)", err)
	}
}
