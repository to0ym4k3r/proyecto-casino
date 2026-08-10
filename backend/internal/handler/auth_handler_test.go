package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/repository"
	"casino-backend/internal/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// In-memory auth fakes (coherent, one shared "database")
// ---------------------------------------------------------------------------

type fakeAuthUserRepo struct {
	byEmail    map[string]*domain.User
	byUsername map[string]*domain.User
	byID       map[uuid.UUID]*domain.User
}

func newFakeAuthUserRepo() *fakeAuthUserRepo {
	return &fakeAuthUserRepo{
		byEmail:    make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
		byID:       make(map[uuid.UUID]*domain.User),
	}
}

func (f *fakeAuthUserRepo) Create(_ context.Context, u *domain.User) error {
	f.byEmail[u.Email] = u
	f.byUsername[u.Username] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeAuthUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
}

func (f *fakeAuthUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (f *fakeAuthUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	if u, ok := f.byUsername[username]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (f *fakeAuthUserRepo) Update(_ context.Context, _ *domain.User) error { return nil }
func (f *fakeAuthUserRepo) Delete(_ context.Context, _ uuid.UUID) error    { return nil }
func (f *fakeAuthUserRepo) DeletePermanent(_ context.Context, _ uuid.UUID) error {
	return nil
}

type fakeAuthRefreshRepo struct {
	byID   map[uuid.UUID]*domain.RefreshToken
	byHash map[string]*domain.RefreshToken
}

func newFakeAuthRefreshRepo() *fakeAuthRefreshRepo {
	return &fakeAuthRefreshRepo{
		byID:   make(map[uuid.UUID]*domain.RefreshToken),
		byHash: make(map[string]*domain.RefreshToken),
	}
}

func (f *fakeAuthRefreshRepo) Save(_ context.Context, rt *domain.RefreshToken) error {
	f.byID[rt.ID] = rt
	f.byHash[rt.TokenHash] = rt
	return nil
}

func (f *fakeAuthRefreshRepo) GetByHash(_ context.Context, tokenHash string) (*domain.RefreshToken, error) {
	if rt, ok := f.byHash[tokenHash]; ok {
		return rt, nil
	}
	return nil, repository.ErrRefreshTokenNotFound
}

func (f *fakeAuthRefreshRepo) Rotate(_ context.Context, oldID uuid.UUID, replacement *domain.RefreshToken) error {
	old, ok := f.byID[oldID]
	if !ok || old.RevokedAt != nil || old.ReplacedBy != nil {
		return repository.ErrReplayDetected
	}
	f.byID[replacement.ID] = replacement
	f.byHash[replacement.TokenHash] = replacement
	now := time.Now()
	old.RevokedAt = &now
	old.ReplacedBy = &replacement.ID
	return nil
}

func (f *fakeAuthRefreshRepo) RevokeFamily(_ context.Context, tokenID uuid.UUID) error {
	now := time.Now()
	for _, rt := range f.byID {
		if rt.ID == tokenID && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

type fakeAuthVerifyRepo struct {
	byHash map[string]*domain.EmailVerification
	users  *fakeAuthUserRepo
}

func newFakeAuthVerifyRepo(users *fakeAuthUserRepo) *fakeAuthVerifyRepo {
	return &fakeAuthVerifyRepo{byHash: make(map[string]*domain.EmailVerification), users: users}
}

func (f *fakeAuthVerifyRepo) Verify(_ context.Context, tokenHash string) error {
	ev, ok := f.byHash[tokenHash]
	if !ok || ev.UsedAt != nil || !ev.ExpiresAt.After(time.Now()) {
		return repository.ErrInvalidVerificationToken
	}
	now := time.Now()
	ev.UsedAt = &now
	if u, ok := f.users.byID[ev.UserID]; ok {
		u.EmailVerifiedAt = &now
	}
	return nil
}

type fakeAuthRegRepo struct {
	users     *fakeAuthUserRepo
	verify    *fakeAuthVerifyRepo
	refresh   *fakeAuthRefreshRepo
	wallet    *fakeWalletRepo
}

func newFakeAuthRegRepo(users *fakeAuthUserRepo, verify *fakeAuthVerifyRepo, refresh *fakeAuthRefreshRepo, wallet *fakeWalletRepo) *fakeAuthRegRepo {
	return &fakeAuthRegRepo{users: users, verify: verify, refresh: refresh, wallet: wallet}
}

func (f *fakeAuthRegRepo) CreateUserWithWallet(ctx context.Context, in domain.RegistrationInput) error {
	if err := f.users.Create(ctx, in.User); err != nil {
		return err
	}
	f.wallet.Create(ctx, in.Wallet)
	f.verify.byHash[in.Verification.TokenHash] = in.Verification
	f.refresh.byID[in.RefreshToken.ID] = in.RefreshToken
	f.refresh.byHash[in.RefreshToken.TokenHash] = in.RefreshToken
	return nil
}

type fakeAuthMailer struct {
	to    string
	token string
}

func (m *fakeAuthMailer) SendVerificationEmail(_ context.Context, to, token string) error {
	m.to = to
	m.token = token
	return nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Stack construction
// ---------------------------------------------------------------------------

type authTestStack struct {
	auth *AuthHandler
	svc  *service.AuthService
	mail *fakeAuthMailer
}

func newAuthTestStack(t *testing.T) *authTestStack {
	t.Helper()
	users := newFakeAuthUserRepo()
	refresh := newFakeAuthRefreshRepo()
	verify := newFakeAuthVerifyRepo(users)
	wallet := &fakeWalletRepo{wallet: &domain.Wallet{}}
	reg := newFakeAuthRegRepo(users, verify, refresh, wallet)
	mail := &fakeAuthMailer{}
	cfg := &config.JWTConfig{Secret: "handler-test-secret-not-for-prod", AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour}
	svc := service.NewAuthService(users, reg, refresh, verify, mail, cfg, 24*time.Hour)
	return &authTestStack{auth: NewAuthHandler(svc), svc: svc, mail: mail}
}

func doReq(t *testing.T, h http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const validDOB = "1990-01-01"

func registerReq(username, email, password, dob string) string {
	return `{"username":"` + username + `","email":"` + email + `","password":"` + password +
		`","first_name":"Fn","last_name":"Ln","date_of_birth":"` + dob + `"}`
}

// ---------------------------------------------------------------------------
// Register — 201 with the user, NO auto-issued tokens
// ---------------------------------------------------------------------------

func TestRegisterReturnsUserWithoutAutoLoginTokens(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("player1", "player1@example.com", "password123", validDOB))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"tokens"`) {
		t.Errorf("register response must not auto-issue tokens: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id"`) || !strings.Contains(rec.Body.String(), `"player1"`) {
		t.Errorf("register response missing user payload: %s", rec.Body.String())
	}
	if st.mail.to != "player1@example.com" {
		t.Errorf("verification email sent to %q, want player1@example.com", st.mail.to)
	}
	if st.mail.token == "" {
		t.Error("verification token not dispatched")
	}
}

func TestRegisterRejectsMalformedDOBWith400(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("baddob", "baddob@example.com", "password123", "2006-13-99"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsUnderageWith400(t *testing.T) {
	st := newAuthTestStack(t)
	underage := time.Now().AddDate(-18, 0, 1).Format("2006-01-02") // still 17 today
	rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("kid", "kid@example.com", "password123", underage))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestRegisterDuplicateEmail409(t *testing.T) {
	st := newAuthTestStack(t)
	first := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("first", "dup@example.com", "password123", validDOB))
	if first.Code != http.StatusCreated {
		t.Fatalf("first register status = %d, want 201", first.Code)
	}
	rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("second", "DUP@example.com", "password123", validDOB))
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsWhitespaceEmailAndOversizedPasswordWith400(t *testing.T) {
	st := newAuthTestStack(t)
	if rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("spaces", "   ", "password123", validDOB)); rec.Code != http.StatusBadRequest {
		t.Fatalf("whitespace-only email: status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	long := strings.Repeat("a", 73)
	if rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("longpw", "long@example.com", long, validDOB)); rec.Code != http.StatusBadRequest {
		t.Fatalf("73-byte password: status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Verify email
// ---------------------------------------------------------------------------

func TestVerifyEmail200(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("v1", "v1@example.com", "password123", validDOB))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", rec.Code)
	}
	body := `{"token":"` + st.mail.token + `"}`
	vrec := doReq(t, st.auth.VerifyEmail, http.MethodPost, "/api/v1/auth/verify", body)
	if vrec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200 (body: %s)", vrec.Code, vrec.Body.String())
	}
}

func TestVerifyEmailInvalidToken400(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.VerifyEmail, http.MethodPost, "/api/v1/auth/verify", `{"token":"bogus"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestVerifyEmailMissingToken400(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.VerifyEmail, http.MethodPost, "/api/v1/auth/verify", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verify status = %d, want 400", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Login gate + logout
// ---------------------------------------------------------------------------

func TestLoginUnverified403(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("gate", "gate@example.com", "password123", validDOB))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", rec.Code)
	}
	lrec := doReq(t, st.auth.Login, http.MethodPost, "/api/v1/auth/login",
		`{"email":"gate@example.com","password":"password123"}`)
	if lrec.Code != http.StatusForbidden {
		t.Fatalf("login before verify status = %d, want 403 (body: %s)", lrec.Code, lrec.Body.String())
	}
}

func TestLogoutRevokesAndReturns204(t *testing.T) {
	st := newAuthTestStack(t)
	if rec := doReq(t, st.auth.Register, http.MethodPost, "/api/v1/auth/register",
		registerReq("out", "out@example.com", "password123", validDOB)); rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201", rec.Code)
	}
	if vrec := doReq(t, st.auth.VerifyEmail, http.MethodPost, "/api/v1/auth/verify",
		`{"token":"`+st.mail.token+`"}`); vrec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", vrec.Code)
	}
	lrec := doReq(t, st.auth.Login, http.MethodPost, "/api/v1/auth/login",
		`{"email":"out@example.com","password":"password123"}`)
	if lrec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %s)", lrec.Code, lrec.Body.String())
	}
	var envelope struct {
		Data domain.TokenPair `json:"data"`
	}
	if err := json.Unmarshal(lrec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal login body: %v", err)
	}
	pair := envelope.Data
	orec := doReq(t, st.auth.Logout, http.MethodPost, "/api/v1/auth/logout",
		`{"refresh_token":"`+pair.RefreshToken+`"}`)
	if orec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204 (body: %s)", orec.Code, orec.Body.String())
	}
	if got := orec.Body.Len(); got != 0 {
		t.Fatalf("logout body length = %d, want 0 (204 must be empty)", got)
	}
	rrec := doReq(t, st.auth.Refresh, http.MethodPost, "/api/v1/auth/refresh",
		`{"refresh_token":"`+pair.RefreshToken+`"}`)
	if rrec.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status = %d, want 401", rrec.Code)
	}
}

func TestLogoutUnknownToken401(t *testing.T) {
	st := newAuthTestStack(t)
	rec := doReq(t, st.auth.Logout, http.MethodPost, "/api/v1/auth/logout", `{"refresh_token":"not-a-token"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}
