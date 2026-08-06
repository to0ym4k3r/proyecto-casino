package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// fakeWalletRepo is an in-memory WalletRepository used to test handlers without
// a database.
type fakeWalletRepo struct {
	gotUserID uuid.UUID
	wallet    *domain.Wallet
	err       error
}

func (f *fakeWalletRepo) Create(_ context.Context, _ *domain.Wallet) error { return nil }

func (f *fakeWalletRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Wallet, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.wallet, nil
}

func (f *fakeWalletRepo) GetByUserID(_ context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	f.gotUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	return f.wallet, nil
}

func (f *fakeWalletRepo) UpdateBalance(_ context.Context, _ uuid.UUID, _ float64, _ int) error {
	return nil
}

func (f *fakeWalletRepo) CreateTransaction(_ context.Context, _ *domain.Transaction) error {
	return nil
}

func signAccessToken(t *testing.T, secret string, userID uuid.UUID) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "access",
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// TestGetBalanceWithValidJWT exercises the full protected path: JWT middleware
// stores the user ID under the typed key, then GetBalance reads it back. It
// asserts the request succeeds and that the service received the correct user ID.
func TestGetBalanceWithValidJWT(t *testing.T) {
	const secret = "test-secret"
	userID := uuid.New()
	wantWallet := &domain.Wallet{
		ID:        uuid.New(),
		UserID:    userID,
		Balance:   250.5,
		Currency:  "USD",
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	repo := &fakeWalletRepo{wallet: wantWallet}
	authSvc := service.NewAuthService(nil, &config.JWTConfig{Secret: secret, AccessExpiry: 15 * time.Minute})
	h := NewWalletHandler(service.NewWalletService(repo))

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.GetBalance))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance", nil)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if repo.gotUserID != userID {
		t.Errorf("service received userID %v, want %v", repo.gotUserID, userID)
	}
	if !strings.Contains(rec.Body.String(), `"balance":250.5`) {
		t.Errorf("body = %s, want balance 250.5 in response", rec.Body.String())
	}
}

// TestGetBalanceWithoutUserIDDoesNotPanic asserts the handler degrades to a
// 404 instead of panicking when the context carries no user ID under the typed key.
func TestGetBalanceWithoutUserIDDoesNotPanic(t *testing.T) {
	repo := &fakeWalletRepo{err: errors.New("wallet not found")}
	h := NewWalletHandler(service.NewWalletService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance", nil)
	rec := httptest.NewRecorder()

	h.GetBalance(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}
