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

// TestRegisterInputValidation checks that the register handler rejects empty
// fields, short passwords, and malformed JSON with 400.
func TestRegisterInputValidation(t *testing.T) {
	th := newTestHandlers(t, testSecret)
	reg := http.HandlerFunc(th.auth.Register)

	cases := []struct {
		name string
		body string
	}{
		{"empty username", registerBody("", "a@example.com", "password123")},
		{"empty email", registerBody("user1", "", "password123")},
		{"empty password", registerBody("user1", "a@example.com", "")},
		{"short password", registerBody("user1", "a@example.com", "short")},
		{"empty payload", `{}`},
		{"malformed json", `{"username":`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if rec := doJSON(http.MethodPost, "/api/v1/auth/register", reg, tt.body, nil); rec.Code != http.StatusBadRequest {
				t.Errorf("code=%d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestWalletAmountValidation checks that deposit/withdraw amounts <= 0 and
// malformed bodies are rejected with 400 (wallet_handler.go:55-58, 85-88).
func TestWalletAmountValidation(t *testing.T) {
	th := newTestHandlers(t, testSecret)
	uid := uuid.New()
	tok := signAccess(t, testSecret, uid)
	now := time.Now().UTC()
	wallet := &domain.Wallet{ID: uuid.New(), UserID: uid, Balance: 100, Currency: "USD", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := th.walletRepo.Create(context.Background(), wallet); err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	auth := middleware.Auth(th.authSvc)
	deposit := auth(http.HandlerFunc(th.wallet.Deposit))
	withdraw := auth(http.HandlerFunc(th.wallet.Withdraw))

	cases := []struct {
		name string
		h    http.Handler
		body string
	}{
		{"deposit amount 0", deposit, `{"amount":0}`},
		{"deposit negative amount", deposit, `{"amount":-5}`},
		{"deposit malformed body", deposit, `{"amount":`},
		{"withdraw amount 0", withdraw, `{"amount":0}`},
		{"withdraw negative amount", withdraw, `{"amount":-5}`},
		{"withdraw non-numeric amount", withdraw, `{"amount":"abc"}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := doJSON(http.MethodPost, "/", tt.h, tt.body, map[string]string{"Authorization": "Bearer " + tok})
			if rec.Code != http.StatusBadRequest {
				t.Errorf("code=%d want 400 (body: %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestOversizedUsernameAcceptedByHandler documents finding #9 (MEDIA): the
// `validate` tags on domain/user.go:24-30 are decorative — no validation
// library ever runs them. The handler (auth_handler.go:44-52) only checks
// non-empty fields and minimum password length, so a 200-char username is
// accepted at the HTTP boundary (it would later fail the VARCHAR(50) column
// in a real database, surfacing as a 500, not a clean 400).
func TestOversizedUsernameAcceptedByHandler(t *testing.T) {
	th := newTestHandlers(t, testSecret)
	body := registerBody(strings.Repeat("x", 200), "oversize@example.com", "password123")
	rec := doJSON(http.MethodPost, "/register", http.HandlerFunc(th.auth.Register), body, nil)
	if rec.Code == http.StatusBadRequest {
		t.Log("handler rejected oversized username — gap now closed")
	} else {
		t.Logf("CONFIRMED GAP: 200-char username accepted (code %d) — no length validation at handler", rec.Code)
	}
}

// TestOversizedBodyNoMaxBytesReader documents finding #8 (MEDIA): there is no
// http.MaxBytesReader anywhere in the tree (grep-confirmed), so request bodies
// are read unbounded. A 5 MB JSON body is fully buffered and processed instead
// of being rejected with 413.
func TestOversizedBodyNoMaxBytesReader(t *testing.T) {
	th := newTestHandlers(t, testSecret)
	huge := strings.Repeat("a", 5<<20) // 5 MB
	body := registerBody(huge, "huge@example.com", "password123")
	rec := doJSON(http.MethodPost, "/register", http.HandlerFunc(th.auth.Register), body, nil)
	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Log("413 returned — MaxBytesReader now present")
	} else {
		t.Logf("CONFIRMED GAP: 5MB request body accepted (code %d) — no MaxBytesReader", rec.Code)
	}
}
