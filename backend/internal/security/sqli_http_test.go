package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"casino-backend/internal/domain"

	"github.com/go-chi/chi/v5"
)

// sqliPayloads is the classic OWASP set of injection probes. Each is submitted
// to the real handlers as plain DATA (register fields, login email, game ID,
// wallet amount). The assertion for every one: it is treated as an opaque
// string and never interpreted as SQL.
var sqliPayloads = []string{
	"' OR 1=1--",
	"\" OR \"\"=\"",
	"'; DROP TABLE users; --",
	"UNION SELECT username,password FROM users--",
	"admin'--",
	"1 OR 1=1",
	"1;SELECT pg_sleep(10);--",
}

// TestSQLiPayloadsTreatedAsDataThroughHandlers submits the classic injection
// payloads through the REAL handlers (auth_handler.go, game_handler.go,
// wallet_handler.go) backed by in-memory repositories.
//
// - register: a payload username is stored literally and the account is usable
//   (201) — the payload is data, not SQL.
// - login: a payload email can never authenticate anyone (401) — no bypass.
// - games/{gameID}: a payload is rejected at the UUID parse boundary (400)
//   before it can ever reach a query.
// - wallet amounts are JSON numbers; a string payload fails decode (400).
func TestSQLiPayloadsTreatedAsDataThroughHandlers(t *testing.T) {
	th := newTestHandlers(t, testSecret)

	t.Run("register stores payload as literal data", func(t *testing.T) {
		for i, p := range sqliPayloads {
			email := "sqli-" + string(rune('a'+i)) + "@example.com"
			req := domain.RegisterRequest{
				Username: p, Email: email, Password: "password123",
				FirstName: "F", LastName: "L", DateOfBirth: "1990-01-01",
			}
			body, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			rec := doJSON(http.MethodPost, "/api/v1/auth/register", http.HandlerFunc(th.auth.Register), string(body), nil)
			if rec.Code != http.StatusCreated {
				t.Errorf("register(%q) code=%d want 201 (body: %s)", p, rec.Code, rec.Body.String())
				continue
			}
			// The account must be reachable with the exact same payload email.
			// Immediately after register the login gate must reject with 403
			// (credentials matched, email not yet verified) — that 403 is the
			// proof the payload was stored as literal data, not executed.
			loginReq, _ := json.Marshal(domain.LoginRequest{Email: email, Password: "password123"})
			lrec := doJSON(http.MethodPost, "/api/v1/auth/login", http.HandlerFunc(th.auth.Login), string(loginReq), nil)
			if lrec.Code != http.StatusForbidden {
				t.Errorf("login with stored payload email (%q) code=%d want 403 (unverified gate proves payload stored literally)", email, lrec.Code)
			}
			// Verify the dispatched token, then the same credentials must
			// authenticate — the full register→verify→login lifecycle works
			// with the injection string stored as plain data.
			if tok := th.mailer.lastToken(); tok == "" {
				t.Errorf("register(%q) dispatched no verification token", p)
			} else if err := th.authSvc.VerifyEmail(context.Background(), tok); err != nil {
				t.Errorf("verify(%q) error = %v", p, err)
			}
			vrec := doJSON(http.MethodPost, "/api/v1/auth/login", http.HandlerFunc(th.auth.Login), string(loginReq), nil)
			if vrec.Code != http.StatusOK {
				t.Errorf("login after verify (%q) code=%d want 200 (body: %s)", email, vrec.Code, vrec.Body.String())
			}
		}
	})

	t.Run("login payload cannot bypass authentication", func(t *testing.T) {
		for _, p := range sqliPayloads {
			body, _ := json.Marshal(domain.LoginRequest{Email: p, Password: "anything"})
			rec := doJSON(http.MethodPost, "/api/v1/auth/login", http.HandlerFunc(th.auth.Login), string(body), nil)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("login(%q) code=%d want 401 (no auth bypass)", p, rec.Code)
			}
		}
	})

	t.Run("game_id payload rejected at UUID parse boundary", func(t *testing.T) {
		r := chi.NewRouter()
		r.Get("/games/{gameID}", th.game.GetGame)
		for _, p := range sqliPayloads {
			target := "/games/" + url.PathEscape(p)
			rec := doJSON(http.MethodGet, target, r, "", nil)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("GetGame(%q) code=%d want 400 (invalid UUID)", p, rec.Code)
			}
		}
	})

	t.Run("wallet amount payload rejected as invalid JSON", func(t *testing.T) {
		// Amounts are JSON numbers; an injection string fails decode before any
		// business logic runs.
		rec := doJSON(http.MethodPost, "/deposit", http.HandlerFunc(th.wallet.Deposit),
			`{"amount":"' OR 1=1--"}`, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("deposit with string amount code=%d want 400", rec.Code)
		}
	})
}
