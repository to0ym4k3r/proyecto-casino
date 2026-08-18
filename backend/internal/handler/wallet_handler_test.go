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

func (f *fakeWalletRepo) UpdateBalance(_ context.Context, _ uuid.UUID, _ int64, _ int) error {
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
		Balance:   25050, // 25050 cents = $250.50
		Currency:  "USD",
		Version:   1,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	repo := &fakeWalletRepo{wallet: wantWallet}
	authSvc := service.NewAuthService(nil, nil, nil, nil, nil, &config.JWTConfig{Secret: secret, AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour}, time.Hour)
	h := NewWalletHandler(service.NewWalletService(repo), nil, nil)

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
	if !strings.Contains(rec.Body.String(), `"balance":25050`) {
		t.Errorf("body = %s, want balance:25050 in response", rec.Body.String())
	}
}

// TestDepositRejectsDecimalCents asserts that deposit with decimal amount (150.50)
// is rejected when Balance is int64 cents. This test WILL FAIL with current float64
// implementation because float64 passes decimal validation — proving the type needs
// to change.
func TestDepositRejectsDecimalCents(t *testing.T) {
	const secret = "test-secret"
	userID := uuid.New()
	wallet := &domain.Wallet{
		ID:       uuid.New(),
		UserID:   userID,
		Balance:  15000,
		Currency: "USD",
		Version:  1,
	}

	repo := &fakeWalletRepo{wallet: wallet}
	authSvc := service.NewAuthService(nil, nil, nil, nil, nil, &config.JWTConfig{Secret: secret, AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour}, time.Hour)
	h := NewWalletHandler(service.NewWalletService(repo), nil, nil)

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.Deposit))

	// Decimal cents are invalid — should be rejected
	body := `{"amount": 150.50}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/deposit", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	// With int64 this MUST return 400 (decimal not allowed in cents).
	// With current float64 it returns 200 (passes validation) — proving the type
	// needs to change.
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for decimal cents (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestGetBalanceWithoutUserIDDoesNotPanic asserts the handler degrades to a
// 404 instead of panicking when the context carries no user ID under the typed key.
func TestGetBalanceWithoutUserIDDoesNotPanic(t *testing.T) {
	repo := &fakeWalletRepo{err: errors.New("wallet not found")}
	h := NewWalletHandler(service.NewWalletService(repo), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance", nil)
	rec := httptest.NewRecorder()

	h.GetBalance(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}
