package security

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"

	"github.com/google/uuid"
)

// TestAuthMiddlewareDefense exercises the Auth middleware (middleware/auth.go:38-62):
// every malformed/forged request must get 401, a valid token must pass and the
// user ID extracted from claims must be readable via UserIDFromContext.
func TestAuthMiddlewareDefense(t *testing.T) {
	th := newTestHandlers(t, testSecret)

	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if middleware.UserIDFromContext(r.Context()) == uuid.Nil {
			http.Error(w, "no user id in context", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.Auth(th.authSvc)(probe)

	t.Run("no Authorization header -> 401", func(t *testing.T) {
		if rec := doJSON(http.MethodGet, "/protected", h, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", rec.Code)
		}
	})
	t.Run("non-Bearer scheme -> 401", func(t *testing.T) {
		rec := doJSON(http.MethodGet, "/protected", h, "", map[string]string{"Authorization": "Basic dXNlcjpwYXNz"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", rec.Code)
		}
	})
	t.Run("Bearer without token -> 401", func(t *testing.T) {
		rec := doJSON(http.MethodGet, "/protected", h, "", map[string]string{"Authorization": "Bearer"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", rec.Code)
		}
	})
	t.Run("garbage token -> 401", func(t *testing.T) {
		rec := doJSON(http.MethodGet, "/protected", h, "", map[string]string{"Authorization": "Bearer not-a-jwt"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", rec.Code)
		}
	})
	t.Run("token signed with wrong secret -> 401", func(t *testing.T) {
		tok := signAccess(t, "attacker-secret", uuid.New())
		rec := doJSON(http.MethodGet, "/protected", h, "", map[string]string{"Authorization": "Bearer " + tok})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", rec.Code)
		}
	})
	t.Run("expired token -> 401", func(t *testing.T) {
		tok := signToken(t, testSecret, "access", uuid.New(), time.Now().Add(-time.Hour))
		rec := doJSON(http.MethodGet, "/protected", h, "", map[string]string{"Authorization": "Bearer " + tok})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code=%d want 401", rec.Code)
		}
	})
	t.Run("valid token passes and UserIDFromContext is correct", func(t *testing.T) {
		uid := uuid.New()
		tok := signAccess(t, testSecret, uid)
		var got uuid.UUID
		probe2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = middleware.UserIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
		rec := doJSON(http.MethodGet, "/protected", middleware.Auth(th.authSvc)(probe2), "",
			map[string]string{"Authorization": "Bearer " + tok})
		if rec.Code != http.StatusOK {
			t.Fatalf("code=%d want 200", rec.Code)
		}
		if got != uid {
			t.Errorf("UserIDFromContext = %v, want %v", got, uid)
		}
	})
}

// TestWalletIDORPreventedByClaimsBoundary confirms the "userID from claims ->
// no IDOR" strength: a token for user A can only ever address A's wallet,
// because the wallet service is fed the claim-derived ID, never a client value.
func TestWalletIDORPreventedByClaimsBoundary(t *testing.T) {
	th := newTestHandlers(t, testSecret)
	userA := uuid.New()
	userB := uuid.New()
	now := time.Now().UTC()
	walletA := &domain.Wallet{ID: uuid.New(), UserID: userA, Balance: 500, Currency: "USD", Version: 1, CreatedAt: now, UpdatedAt: now}
	walletB := &domain.Wallet{ID: uuid.New(), UserID: userB, Balance: 99999, Currency: "USD", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := th.walletRepo.Create(context.Background(), walletA); err != nil {
		t.Fatalf("create wallet A: %v", err)
	}
	if err := th.walletRepo.Create(context.Background(), walletB); err != nil {
		t.Fatalf("create wallet B: %v", err)
	}

	tokA := signAccess(t, testSecret, userA)
	h := middleware.Auth(th.authSvc)(http.HandlerFunc(th.wallet.GetBalance))
	rec := doJSON(http.MethodGet, "/api/v1/wallet/balance", h, "", map[string]string{"Authorization": "Bearer " + tokA})

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "500") || strings.Contains(body, "99999") {
		t.Errorf("user A token saw B's balance — IDOR. body=%s", body)
	}
}
