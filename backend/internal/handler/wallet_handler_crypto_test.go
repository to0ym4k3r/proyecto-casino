package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/repository/postgres"
	"casino-backend/internal/service"

	"github.com/google/uuid"
)

// --- Mocks for new handler dependencies ---

type fakeDepositAddressRepo struct {
	addr  string
	ata   string
	exist bool
}

func (f *fakeDepositAddressRepo) Create(_ context.Context, _ *postgres.DepositAddress) error { return nil }
func (f *fakeDepositAddressRepo) GetByUserID(_ context.Context, _ uuid.UUID) (*postgres.DepositAddress, error) {
	if f.exist {
		return &postgres.DepositAddress{
			UserID:    uuid.New(),
			UserIndex: 1,
			SolAddr:   f.addr,
			USDCAta:   f.ata,
		}, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeDepositAddressRepo) AllocateIndex(_ context.Context, _ uuid.UUID) (int, error) {
	return 1, nil
}

type fakeWithdrawalRepoForHandler struct{}

func (f *fakeWithdrawalRepoForHandler) Create(_ context.Context, _ *domain.Withdrawal) error {
	return nil
}
func (f *fakeWithdrawalRepoForHandler) GetByID(_ context.Context, _ uuid.UUID) (*domain.Withdrawal, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeWithdrawalRepoForHandler) ListByUserID(_ context.Context, _ uuid.UUID, _, _ int) ([]domain.Withdrawal, error) {
	return nil, nil
}
func (f *fakeWithdrawalRepoForHandler) GetDailyTotal(_ context.Context, _ uuid.UUID, _ time.Time) (int64, error) {
	return 0, nil
}
func (f *fakeWithdrawalRepoForHandler) UpdateStatus(_ context.Context, _ uuid.UUID, _ domain.WithdrawalStatus, _ string) error {
	return nil
}

type fakeSolanaTxRepoForHandler struct{}

func (f *fakeSolanaTxRepoForHandler) Create(_ context.Context, _ *domain.SolanaTx) error { return nil }
func (f *fakeSolanaTxRepoForHandler) ListByWalletID(_ context.Context, _ uuid.UUID, _ string, _, _ *time.Time, _, _ int) ([]domain.SolanaTx, int, error) {
	return nil, 0, nil
}
func (f *fakeSolanaTxRepoForHandler) GetByTxHash(_ context.Context, _ string) (*domain.SolanaTx, error) {
	return nil, domain.ErrNotFound
}

// --- Helper to build handler with full deps ---

func newTestWalletHandlerWithDeps(secret string) (*WalletHandler, *service.AuthService) {
	wallet := &domain.Wallet{
		ID: uuid.New(), UserID: uuid.New(), Balance: 50000,
		Currency: "USD", Version: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}

	repo := &fakeWalletRepo{wallet: wallet}
	walletSvc := service.NewWalletService(repo)
	withdrawalSvc := service.NewWithdrawalService(
		repo, &fakeWithdrawalRepoForHandler{},
		func(_ context.Context, _ uuid.UUID) (service.KYCTier, error) {
			return service.KYCL1, nil
		},
		func(_ context.Context) (bool, error) { return false, nil },
		func(_ context.Context) (uint64, error) { return uint64(wallet.Balance), nil },
	)
	txHistorySvc := service.NewTxHistoryService(&fakeSolanaTxRepoForHandler{})
	depositAddrRepo := &fakeDepositAddressRepo{
		addr:  "FakeSOLAddress123",
		ata:   "FakeUSDCAta456",
		exist: true,
	}
	authSvc := service.NewAuthService(nil, nil, nil, nil, nil, &config.JWTConfig{
		Secret: secret, AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour,
	}, time.Hour)

	h := NewWalletHandler(walletSvc, withdrawalSvc, txHistorySvc, depositAddrRepo)
	return h, authSvc
}

// --- Tests ---

func TestDepositSOLReturnsAddress(t *testing.T) {
	const secret = "test-secret"
	h, authSvc := newTestWalletHandlerWithDeps(secret)
	userID := uuid.New()

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.DepositSOL))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/deposit/sol", nil)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "FakeSOLAddress123") {
		t.Errorf("body should contain deposit address, got: %s", body)
	}
	if !strings.Contains(body, "solana:FakeSOLAddress123") {
		t.Errorf("body should contain solana: URI with address, got: %s", body)
	}
}

func TestDepositUSDCReturnsATA(t *testing.T) {
	const secret = "test-secret"
	h, authSvc := newTestWalletHandlerWithDeps(secret)
	userID := uuid.New()

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.DepositUSDC))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/deposit/usdc", nil)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "FakeUSDCAta456") {
		t.Errorf("body should contain USDC ATA, got: %s", body)
	}
	if !strings.Contains(body, "solana:FakeUSDCAta456") {
		t.Errorf("body should contain solana: URI with ATA, got: %s", body)
	}
}

func TestWithdrawSOLRequiresAmount(t *testing.T) {
	const secret = "test-secret"
	h, authSvc := newTestWalletHandlerWithDeps(secret)
	userID := uuid.New()

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.WithdrawSOL))

	body := `{"amount": 0, "to_address": "ABC123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/withdraw/sol", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for zero amount (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestWithdrawSOLSuccess(t *testing.T) {
	const secret = "test-secret"
	h, authSvc := newTestWalletHandlerWithDeps(secret)
	userID := uuid.New()

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.WithdrawSOL))

	body := `{"amount": 10000, "to_address": "ABC123def456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/withdraw/sol", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "withdrawal_id") {
		t.Errorf("body should contain withdrawal_id, got: %s", respBody)
	}
}

func TestTxHistoryReturnsEmpty(t *testing.T) {
	const secret = "test-secret"
	h, authSvc := newTestWalletHandlerWithDeps(secret)
	userID := uuid.New()

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.GetTxHistory))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/tx-history?page=1&limit=50", nil)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "transactions") {
		t.Errorf("body should contain transactions array, got: %s", rec.Body.String())
	}
}

func TestEmergencyPauseRequiresAdmin(t *testing.T) {
	const secret = "test-secret"
	h, authSvc := newTestWalletHandlerWithDeps(secret)
	userID := uuid.New()

	route := middleware.Auth(authSvc)(http.HandlerFunc(h.AdminEmergencyPause))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/emergency/pause", nil)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	rec := httptest.NewRecorder()

	route.ServeHTTP(rec, req)

	// Without admin role, should return 403
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for non-admin (body: %s)", rec.Code, rec.Body.String())
	}
}
