package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/handler"
	"casino-backend/internal/repository"
	"casino-backend/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// --- Fakes: minimal production-shaped dependencies for buildWalletHandler. ---

type fakeWiringWalletRepo struct{ debit int64 }

func (f *fakeWiringWalletRepo) Create(context.Context, *domain.Wallet) error { return nil }
func (f *fakeWiringWalletRepo) GetByID(context.Context, uuid.UUID) (*domain.Wallet, error) {
	return f.wallet(), nil
}
func (f *fakeWiringWalletRepo) GetByUserID(context.Context, uuid.UUID) (*domain.Wallet, error) {
	return f.wallet(), nil
}
func (f *fakeWiringWalletRepo) wallet() *domain.Wallet {
	return &domain.Wallet{ID: uuid.New(), Balance: 50000, Version: 1}
}
func (f *fakeWiringWalletRepo) UpdateBalance(_ context.Context, _ uuid.UUID, amount int64, _ int) error {
	f.debit = amount
	return nil
}
func (f *fakeWiringWalletRepo) CreateTransaction(context.Context, *domain.Transaction) error {
	return nil
}

type fakeWiringKycRepo struct{}

func (f *fakeWiringKycRepo) Submit(context.Context, *domain.KycSubmission) error { return nil }
func (f *fakeWiringKycRepo) GetByUserID(context.Context, uuid.UUID) (*domain.KycSubmission, error) {
	return nil, repository.ErrKycNotFound
}
func (f *fakeWiringKycRepo) CreateDocument(context.Context, *domain.KycDocument) error { return nil }

type fakeWiringWithdrawalRepo struct{}

func (f *fakeWiringWithdrawalRepo) Create(context.Context, *domain.Withdrawal) error { return nil }
func (f *fakeWiringWithdrawalRepo) GetByID(context.Context, uuid.UUID) (*domain.Withdrawal, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeWiringWithdrawalRepo) ListByUserID(context.Context, uuid.UUID, int, int) ([]domain.Withdrawal, error) {
	return nil, nil
}
func (f *fakeWiringWithdrawalRepo) GetDailyTotal(context.Context, uuid.UUID, time.Time) (int64, error) {
	return 0, nil
}
func (f *fakeWiringWithdrawalRepo) UpdateStatus(context.Context, uuid.UUID, domain.WithdrawalStatus, string) error {
	return nil
}

// newWiringHandler builds the wallet handler through the production seam with
// production-shaped fakes. forfeitTxCalls counts transactions opened for the
// bonus-forfeit step of RequestWithdrawal.
func newWiringHandler() (*handler.WalletHandler, *fakeWiringWalletRepo, *int) {
	walletRepo := &fakeWiringWalletRepo{}
	walletSvc := service.NewWalletService(walletRepo)
	forfeitTxCalls := 0
	txFunc := func(_ context.Context, _ func(pgx.Tx) error) error {
		forfeitTxCalls++
		return nil // records the forfeit tx; the forfeit SQL itself is covered by service tests
	}
	bonusSvc := service.NewBonusService(nil, nil, nil, walletSvc, service.DefaultBonusHandlers(), txFunc)
	h := buildWalletHandler(
		walletSvc,
		walletRepo,
		&fakeWiringWithdrawalRepo{},
		service.NewKycService(&fakeWiringKycRepo{}),
		bonusSvc,
		txFunc,
		nil, // depositAddrRepo: not exercised by the withdraw paths
	)
	return h, walletRepo, &forfeitTxCalls
}

// TestBuildWalletHandlerWiresWithdrawalService pins the CRITICAL-3 regression:
// the production wallet handler must carry a real WithdrawalService plus the
// bonus-forfeit wiring, never nil — the registered crypto withdraw routes
// dereference the service on every request.
func TestBuildWalletHandlerWiresWithdrawalService(t *testing.T) {
	h, _, _ := newWiringHandler()

	svc := reflect.ValueOf(h).Elem().FieldByName("withdrawalSvc")
	if !svc.IsValid() || svc.IsNil() {
		t.Fatal("walletHandler.withdrawalSvc is nil: crypto withdraw routes would panic (CRITICAL-3)")
	}
	if bonusSvc := svc.Elem().FieldByName("bonusSvc"); !bonusSvc.IsValid() || bonusSvc.IsNil() {
		t.Fatal("WithdrawalService.bonusSvc is nil: bonus forfeit on withdrawal unreachable")
	}
	if txFunc := svc.Elem().FieldByName("txFunc"); !txFunc.IsValid() || txFunc.IsNil() {
		t.Fatal("WithdrawalService.txFunc is nil: bonus forfeit on withdrawal unreachable")
	}
}

// TestBuildWalletHandlerWithdrawSOLReachesService proves a valid withdraw
// request reaches the real service pipeline instead of panicking: KYC tier
// limit, daily limit, bonus-forfeit transaction and the balance debit all run.
func TestBuildWalletHandlerWithdrawSOLReachesService(t *testing.T) {
	h, walletRepo, forfeitTxCalls := newWiringHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/withdraw/sol",
		strings.NewReader(`{"amount": 1000, "to_address": "ABC123def456"}`))
	rec := httptest.NewRecorder()
	h.WithdrawSOL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "withdrawal_id") {
		t.Errorf("body should contain withdrawal_id, got: %s", rec.Body.String())
	}
	if walletRepo.debit != -1000 {
		t.Errorf("wallet debit = %d, want -1000 (real money must move)", walletRepo.debit)
	}
	if *forfeitTxCalls != 1 {
		t.Errorf("forfeit transactions = %d, want 1 (bonus forfeit must run on withdrawal)", *forfeitTxCalls)
	}
}

// TestBuildWalletHandlerWithdrawEnforcesKYCLimit triangulates the wired
// service: an unverified user is rejected at the per-transaction limit —
// proof the request flowed through real KYC tier logic, not a stub.
func TestBuildWalletHandlerWithdrawEnforcesKYCLimit(t *testing.T) {
	h, _, _ := newWiringHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wallet/withdraw/sol",
		strings.NewReader(`{"amount": 99999999, "to_address": "ABC123def456"}`))
	rec := httptest.NewRecorder()
	h.WithdrawSOL(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "per-transaction limit exceeded") {
		t.Errorf("body should report the per-transaction limit, got: %s", rec.Body.String())
	}
}
