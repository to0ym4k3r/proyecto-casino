package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// In-memory fakes (coherent — like one database behind the repository seams)
// ---------------------------------------------------------------------------

type fakeUserRepo struct {
	byEmail    map[string]*domain.User
	byUsername map[string]*domain.User
	byID       map[uuid.UUID]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		byEmail:    make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
		byID:       make(map[uuid.UUID]*domain.User),
	}
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	// Mirror the Postgres plain UNIQUE constraints on users.email/username:
	// the identity is occupied even when the existing row is soft-deleted.
	if _, ok := f.byEmail[u.Email]; ok {
		return repository.ErrUserAlreadyExists
	}
	if _, ok := f.byUsername[u.Username]; ok {
		return repository.ErrUserAlreadyExists
	}
	f.byEmail[u.Email] = u
	f.byUsername[u.Username] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := f.byID[id]
	if !ok || u.DeletedAt != nil {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := f.byEmail[email]
	if !ok || u.DeletedAt != nil {
		return nil, repository.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	u, ok := f.byUsername[username]
	if !ok || u.DeletedAt != nil {
		return nil, repository.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) Update(_ context.Context, _ *domain.User) error { return nil }

// UpdateProfile persists the three mutable profile fields, keeping the
// username identity index coherent.
func (f *fakeUserRepo) UpdateProfile(_ context.Context, id uuid.UUID, username, firstName, lastName string) error {
	u, ok := f.byID[id]
	if !ok {
		return repository.ErrUserNotFound
	}
	delete(f.byUsername, u.Username)
	u.Username = username
	u.FirstName = firstName
	u.LastName = lastName
	f.byUsername[username] = u
	return nil
}

// Delete is a SOFT delete: the row stays in the maps (still occupying the
// email/username identities) but every lookup treats it as gone — exactly how
// the Postgres deleted_at IS NULL filters behave.
func (f *fakeUserRepo) Delete(_ context.Context, id uuid.UUID) error {
	u, ok := f.byID[id]
	if !ok {
		return nil
	}
	now := time.Now()
	u.DeletedAt = &now
	return nil
}

// DeletePermanent physically removes the row and frees its email/username.
func (f *fakeUserRepo) DeletePermanent(_ context.Context, id uuid.UUID) error {
	u, ok := f.byID[id]
	if !ok {
		return nil
	}
	delete(f.byID, id)
	delete(f.byEmail, u.Email)
	delete(f.byUsername, u.Username)
	return nil
}

type fakeVerificationRepo struct {
	byHash   map[string]*domain.EmailVerification
	userRepo *fakeUserRepo
}

func newFakeVerificationRepo(userRepo *fakeUserRepo) *fakeVerificationRepo {
	return &fakeVerificationRepo{byHash: make(map[string]*domain.EmailVerification), userRepo: userRepo}
}

func (f *fakeVerificationRepo) Verify(_ context.Context, tokenHash string) error {
	ev, ok := f.byHash[tokenHash]
	if !ok || ev.UsedAt != nil || !ev.ExpiresAt.After(time.Now()) {
		return repository.ErrInvalidVerificationToken
	}
	now := time.Now()
	ev.UsedAt = &now
	if u, ok := f.userRepo.byID[ev.UserID]; ok {
		u.EmailVerifiedAt = &now
	}
	return nil
}

type fakeRefreshRepo struct {
	byID   map[uuid.UUID]*domain.RefreshToken
	byHash map[string]*domain.RefreshToken
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{
		byID:   make(map[uuid.UUID]*domain.RefreshToken),
		byHash: make(map[string]*domain.RefreshToken),
	}
}

func (f *fakeRefreshRepo) Save(_ context.Context, rt *domain.RefreshToken) error {
	f.byID[rt.ID] = rt
	f.byHash[rt.TokenHash] = rt
	return nil
}

func (f *fakeRefreshRepo) GetByHash(_ context.Context, tokenHash string) (*domain.RefreshToken, error) {
	if rt, ok := f.byHash[tokenHash]; ok {
		return rt, nil
	}
	return nil, repository.ErrRefreshTokenNotFound
}

func (f *fakeRefreshRepo) Rotate(_ context.Context, oldID uuid.UUID, replacement *domain.RefreshToken) error {
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

func (f *fakeRefreshRepo) RevokeFamily(_ context.Context, tokenID uuid.UUID) error {
	now := time.Now()
	for _, id := range f.familyIDs(tokenID) {
		if rt, ok := f.byID[id]; ok && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

// familyIDs walks the replaced_by chain in both directions, mirroring the
// recursive CTE in the production repository.
func (f *fakeRefreshRepo) familyIDs(id uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	stack := []uuid.UUID{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		rt, ok := f.byID[cur]
		if !ok {
			continue
		}
		seen[cur] = true
		if rt.ReplacedBy != nil {
			stack = append(stack, *rt.ReplacedBy)
		}
		for _, other := range f.byID {
			if other.ReplacedBy != nil && *other.ReplacedBy == cur {
				stack = append(stack, other.ID)
			}
		}
	}
	out := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

type fakeRegistrationRepo struct {
	calls       []domain.RegistrationInput
	userRepo    *fakeUserRepo
	verifyRepo  *fakeVerificationRepo
	refreshRepo *fakeRefreshRepo
}

func newFakeRegistrationRepo(userRepo *fakeUserRepo, verifyRepo *fakeVerificationRepo, refreshRepo *fakeRefreshRepo) *fakeRegistrationRepo {
	return &fakeRegistrationRepo{calls: []domain.RegistrationInput{}, userRepo: userRepo, verifyRepo: verifyRepo, refreshRepo: refreshRepo}
}

// CreateUserWithWallet mirrors the production atomic transaction: every record
// lands in the shared in-memory "database", so a later login/verify/refresh
// sees the user the same way a real Postgres would.
func (f *fakeRegistrationRepo) CreateUserWithWallet(_ context.Context, in domain.RegistrationInput) error {
	f.calls = append(f.calls, in)
	if err := f.userRepo.Create(context.Background(), in.User); err != nil {
		return err
	}
	f.verifyRepo.byHash[in.Verification.TokenHash] = in.Verification
	f.refreshRepo.byID[in.RefreshToken.ID] = in.RefreshToken
	f.refreshRepo.byHash[in.RefreshToken.TokenHash] = in.RefreshToken
	return nil
}

type mailCall struct {
	to    string
	token string
}

type stubMailer struct {
	calls []mailCall
	err   error
}

func (m *stubMailer) SendVerificationEmail(_ context.Context, to, token string) error {
	m.calls = append(m.calls, mailCall{to: to, token: token})
	return m.err
}

// ---------------------------------------------------------------------------
// Construction helpers
// ---------------------------------------------------------------------------

func newTestAuth(t *testing.T) (*AuthService, *fakeUserRepo, *fakeRegistrationRepo, *fakeRefreshRepo, *fakeVerificationRepo, *stubMailer) {
	t.Helper()
	userRepo := newFakeUserRepo()
	verifyRepo := newFakeVerificationRepo(userRepo)
	refreshRepo := newFakeRefreshRepo()
	regRepo := newFakeRegistrationRepo(userRepo, verifyRepo, refreshRepo)
	mailer := &stubMailer{}
	svc := NewAuthService(
		userRepo, regRepo, refreshRepo, verifyRepo, mailer,
		&config.JWTConfig{
			Secret:        "unit-test-secret-not-for-production",
			AccessExpiry:  15 * time.Minute,
			RefreshExpiry: 7 * 24 * time.Hour,
		},
		24*time.Hour,
	)
	return svc, userRepo, regRepo, refreshRepo, verifyRepo, mailer
}

func registerPlayer(t *testing.T, svc *AuthService, email string) *domain.User {
	t.Helper()
	u, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username:    "player_" + uuid.New().String()[:8],
		Email:       email,
		Password:    "password123",
		FirstName:   "Fn",
		LastName:    "Ln",
		DateOfBirth: time.Now().AddDate(-25, 0, 0).Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return u
}

func verifyPlayer(t *testing.T, svc *AuthService, mailer *stubMailer) {
	t.Helper()
	if len(mailer.calls) == 0 {
		t.Fatal("no verification email was dispatched")
	}
	if err := svc.VerifyEmail(context.Background(), mailer.calls[len(mailer.calls)-1].token); err != nil {
		t.Fatalf("verify email: %v", err)
	}
}

func signRefreshJWT(t *testing.T, secret string, userID uuid.UUID, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "refresh",
		"exp":  exp.Unix(),
		"iat":  time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign refresh jwt: %v", err)
	}
	return s
}

// ---------------------------------------------------------------------------
// 2.2 Register — atomic, lowercase-normalized, age-gated
// ---------------------------------------------------------------------------

func TestRegisterNormalizesEmailToLowercase(t *testing.T) {
	svc, _, regRepo, _, _, mailer := newTestAuth(t)
	u, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "Player_One", Email: "Player@Example.COM", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: time.Now().AddDate(-25, 0, 0).Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if u.Email != "player@example.com" {
		t.Errorf("stored email = %q, want lowercase %q", u.Email, "player@example.com")
	}
	if regRepo.calls[0].User.Email != "player@example.com" {
		t.Errorf("registration input email = %q, want lowercase", regRepo.calls[0].User.Email)
	}
	if mailer.calls[0].to != "player@example.com" {
		t.Errorf("verification email sent to %q, want lowercase address", mailer.calls[0].to)
	}
}

func TestRegisterRejectsUnderageDOB(t *testing.T) {
	svc, _, regRepo, _, _, _ := newTestAuth(t)
	// 18 years and one day ago: still 17 today.
	dob := time.Now().AddDate(-18, 0, 1).Format("2006-01-02")
	_, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "underage", Email: "kid@example.com", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: dob,
	})
	if !errors.Is(err, ErrUnderage) {
		t.Fatalf("err = %v, want ErrUnderage", err)
	}
	if len(regRepo.calls) != 0 {
		t.Errorf("registration repo called %d times, want 0 (no partial rows for underage)", len(regRepo.calls))
	}
}

func TestRegisterAcceptsExactly18Boundary(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuth(t)
	dob := time.Now().AddDate(-18, 0, 0).Format("2006-01-02")
	_, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "adult", Email: "adult@example.com", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: dob,
	})
	if err != nil {
		t.Fatalf("18-year-old must be accepted, got error: %v", err)
	}
}

func TestRegisterRejectsMalformedDOB(t *testing.T) {
	svc, _, regRepo, _, _, _ := newTestAuth(t)
	_, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "bad", Email: "bad@example.com", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: "2006-13-99",
	})
	if !errors.Is(err, ErrInvalidDOB) {
		t.Fatalf("err = %v, want ErrInvalidDOB", err)
	}
	if len(regRepo.calls) != 0 {
		t.Errorf("registration repo called %d times, want 0", len(regRepo.calls))
	}
}

func TestRegisterDuplicateEmailCaseInsensitive(t *testing.T) {
	svc, _, regRepo, _, _, _ := newTestAuth(t)
	registerPlayer(t, svc, "dup@example.com")
	_, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "other", Email: "DUP@EXAMPLE.COM", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: time.Now().AddDate(-25, 0, 0).Format("2006-01-02"),
	})
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("err = %v, want ErrUserAlreadyExists (case-insensitive)", err)
	}
	if len(regRepo.calls) != 1 {
		t.Errorf("registration repo called %d times, want 1 (duplicate never persisted)", len(regRepo.calls))
	}
}

// TestRegisterAtomicInputComplete proves the service hands the registration
// repository a complete, coherent RegistrationInput: hashed password (cost 12),
// unverified user, wallet, hashed verification token and a persisted refresh
// token row — all in one atomic call.
func TestRegisterAtomicInputComplete(t *testing.T) {
	svc, _, regRepo, refreshRepo, _, mailer := newTestAuth(t)
	dob := time.Now().AddDate(-25, 0, 0)
	u, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "atomic", Email: "atomic@example.com", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: dob.Format("2006-01-02"),
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(regRepo.calls) != 1 {
		t.Fatalf("CreateUserWithWallet called %d times, want exactly 1 atomic call", len(regRepo.calls))
	}
	in := regRepo.calls[0]

	if in.User.PasswordHash == "password123" || !strings.HasPrefix(in.User.PasswordHash, "$2a$12$") {
		t.Errorf("password not bcrypt cost 12: %q", in.User.PasswordHash)
	}
	if in.User.EmailVerifiedAt != nil {
		t.Error("new user must start unverified (email_verified_at nil)")
	}
	if !in.User.IsActive {
		t.Error("new user must be active")
	}
	if in.User.DateOfBirth.Format("2006-01-02") != dob.Format("2006-01-02") {
		t.Errorf("DateOfBirth = %v, want calendar date %v", in.User.DateOfBirth.Format("2006-01-02"), dob.Format("2006-01-02"))
	}
	if in.Wallet.UserID != u.ID || in.Wallet.Balance != 0 {
		t.Errorf("wallet = %+v, want zero-balance wallet for user %v", in.Wallet, u.ID)
	}

	if len(mailer.calls) != 1 {
		t.Fatalf("mailer called %d times, want 1", len(mailer.calls))
	}
	raw := mailer.calls[0].token
	if len(raw) < 43 { // 32 random bytes in base64url
		t.Errorf("raw verification token %q too short (%d chars)", raw, len(raw))
	}
	if in.Verification.TokenHash != hashToken(raw) {
		t.Error("stored verification token hash does not match the dispatched raw token")
	}

	if _, err := refreshRepo.GetByHash(context.Background(), in.RefreshToken.TokenHash); err != nil {
		t.Errorf("refresh token row not persisted: %v", err)
	}
}

func TestRegisterRollsBackWhenVerificationEmailFails(t *testing.T) {
	svc, userRepo, _, _, _, mailer := newTestAuth(t)
	mailer.err = errors.New("smtp server unreachable")
	_, err := svc.Register(context.Background(), domain.RegisterRequest{
		Username: "offline", Email: "offline@example.com", Password: "password123",
		FirstName: "Fn", LastName: "Ln", DateOfBirth: time.Now().AddDate(-25, 0, 0).Format("2006-01-02"),
	})
	if !errors.Is(err, ErrVerificationEmailFailed) {
		t.Fatalf("Register() error = %v, want ErrVerificationEmailFailed", err)
	}
	if _, err := userRepo.GetByEmail(context.Background(), "offline@example.com"); err == nil {
		t.Error("user not rolled back after verification email dispatch failure")
	}
}

// TestRegisterRetrySucceedsAfterVerificationEmailFailure is the regression test
// for the soft-delete brick: the rollback compensation MUST physically delete
// the just-created row, otherwise the plain UNIQUE constraints keep the
// email/username occupied and every retry of the same registration fails with
// ErrUserAlreadyExists forever. Fails on the old code (soft Delete), passes on
// the fixed code (DeletePermanent).
func TestRegisterRetrySucceedsAfterVerificationEmailFailure(t *testing.T) {
	svc, userRepo, _, _, _, mailer := newTestAuth(t)
	req := domain.RegisterRequest{
		Username:    "retry",
		Email:       "retry@example.com",
		Password:    "password123",
		FirstName:   "Fn",
		LastName:    "Ln",
		DateOfBirth: time.Now().AddDate(-25, 0, 0).Format("2006-01-02"),
	}

	mailer.err = errors.New("smtp server unreachable")
	if _, err := svc.Register(context.Background(), req); !errors.Is(err, ErrVerificationEmailFailed) {
		t.Fatalf("first Register() error = %v, want ErrVerificationEmailFailed", err)
	}
	// Compensation must be a PHYSICAL delete: the identity must be free again.
	if _, err := userRepo.GetByEmail(context.Background(), "retry@example.com"); !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("after rollback GetByEmail = %v, want ErrUserNotFound (physical delete freed the identity)", err)
	}

	mailer.err = nil
	if _, err := svc.Register(context.Background(), req); err != nil {
		t.Fatalf("retry Register() after rollback = %v, want success (was %v with a soft-delete brick)", err, ErrUserAlreadyExists)
	}
}

// ---------------------------------------------------------------------------
// 2.5 Verify email
// ---------------------------------------------------------------------------

func TestVerifyEmailValidTokenActivates(t *testing.T) {
	svc, userRepo, _, _, _, mailer := newTestAuth(t)
	u := registerPlayer(t, svc, "verify@example.com")
	if err := svc.VerifyEmail(context.Background(), mailer.calls[0].token); err != nil {
		t.Fatalf("verify: %v", err)
	}
	got, _ := userRepo.GetByID(context.Background(), u.ID)
	if got.EmailVerifiedAt == nil {
		t.Error("user not marked email-verified after valid token")
	}
}

func TestVerifyEmailInvalidTokenRejected(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuth(t)
	registerPlayer(t, svc, "v2@example.com")
	if err := svc.VerifyEmail(context.Background(), "bogus-token"); !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("err = %v, want ErrInvalidVerificationToken", err)
	}
	for _, u := range userRepo.byEmail {
		if u.EmailVerifiedAt != nil {
			t.Errorf("user %s verified by an invalid token", u.Email)
		}
	}
}

func TestVerifyEmailReplayRejected(t *testing.T) {
	svc, _, _, _, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "replay@example.com")
	raw := mailer.calls[0].token
	if err := svc.VerifyEmail(context.Background(), raw); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if err := svc.VerifyEmail(context.Background(), raw); !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("second verify of same token = %v, want ErrInvalidVerificationToken (single-use)", err)
	}
}

// ---------------------------------------------------------------------------
// 2.6 Login gate
// ---------------------------------------------------------------------------

func TestLoginUnverifiedForbidden(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuth(t)
	registerPlayer(t, svc, "gate@example.com")
	_, err := svc.Login(context.Background(), domain.LoginRequest{Email: "gate@example.com", Password: "password123"})
	if !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("err = %v, want ErrEmailNotVerified", err)
	}
}

func TestLoginVerifiedSucceedsAndPersistsRefresh(t *testing.T) {
	svc, _, _, refreshRepo, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "ok@example.com")
	verifyPlayer(t, svc, mailer)
	pair, err := svc.Login(context.Background(), domain.LoginRequest{Email: "ok@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("login returned empty token pair")
	}
	if _, err := refreshRepo.GetByHash(context.Background(), hashToken(pair.RefreshToken)); err != nil {
		t.Errorf("issued refresh token not persisted: %v", err)
	}
}

func TestLoginWrongPasswordInvalidCredentials(t *testing.T) {
	svc, _, _, _, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "pw@example.com")
	verifyPlayer(t, svc, mailer)
	_, err := svc.Login(context.Background(), domain.LoginRequest{Email: "pw@example.com", Password: "wrongpass"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginUnknownEmailInvalidCredentials(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuth(t)
	_, err := svc.Login(context.Background(), domain.LoginRequest{Email: "ghost@example.com", Password: "password123"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

// ---------------------------------------------------------------------------
// 2.7 Refresh rotation/revocation + logout
// ---------------------------------------------------------------------------

func TestRefreshRotatesAndReturnsNewPair(t *testing.T) {
	svc, _, _, refreshRepo, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "rot@example.com")
	verifyPlayer(t, svc, mailer)
	pair, err := svc.Login(context.Background(), domain.LoginRequest{Email: "rot@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	oldRow, err := refreshRepo.GetByHash(context.Background(), hashToken(pair.RefreshToken))
	if err != nil {
		t.Fatalf("old refresh row: %v", err)
	}

	newPair, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if newPair.RefreshToken == pair.RefreshToken {
		t.Error("refresh returned the same token — rotation did not happen")
	}
	if newPair.AccessToken == "" {
		t.Error("refresh returned empty access token")
	}

	rotated, err := refreshRepo.GetByHash(context.Background(), hashToken(pair.RefreshToken))
	if err != nil {
		t.Fatalf("re-fetch old row: %v", err)
	}
	if rotated.RevokedAt == nil {
		t.Error("old refresh token not marked revoked")
	}
	if rotated.ReplacedBy == nil {
		t.Error("old refresh token not marked replaced_by the new token")
	}
	// Rotation must persist a DISTINCT row for the successor token, never
	// overwrite the old one.
	newRow, err := refreshRepo.GetByHash(context.Background(), hashToken(newPair.RefreshToken))
	if err != nil {
		t.Fatalf("fetch new refresh row: %v", err)
	}
	if oldRow.ID == newRow.ID {
		t.Error("old and new tokens share one row — rotation must create a distinct row")
	}
	if rotated.ReplacedBy == nil || *rotated.ReplacedBy != newRow.ID {
		t.Errorf("old row replaced_by = %v, want the new row %v", rotated.ReplacedBy, newRow.ID)
	}
}

func TestRefreshReplayRejectedAndFamilyRevoked(t *testing.T) {
	svc, _, _, refreshRepo, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "rep@example.com")
	verifyPlayer(t, svc, mailer)
	pair, err := svc.Login(context.Background(), domain.LoginRequest{Email: "rep@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	first, err := svc.Refresh(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// Replaying the already-rotated token is a credential theft signal: the
	// whole family must die (fail-safe forced re-login).
	if _, err := svc.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("replay refresh err = %v, want ErrInvalidRefreshToken", err)
	}

	if _, err := svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("family member (new token) still usable after replay: %v", err)
	}
	// The presented token and its rotated successor are the family: both must
	// be revoked. (The unlinked registration seed row is not part of this
	// family — its hash is never handed to any client.)
	for _, hash := range []string{hashToken(pair.RefreshToken), hashToken(first.RefreshToken)} {
		row, err := refreshRepo.GetByHash(context.Background(), hash)
		if err != nil {
			t.Fatalf("fetch family member: %v", err)
		}
		if row.RevokedAt == nil {
			t.Errorf("family member %s not revoked after replay", row.ID)
		}
	}
}

func TestRefreshRevokedTokenRejected(t *testing.T) {
	svc, _, _, _, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "rev@example.com")
	verifyPlayer(t, svc, mailer)
	pair, err := svc.Login(context.Background(), domain.LoginRequest{Email: "rev@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("refresh after logout = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshUnknownTokenRejected(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuth(t)
	// Well-signed refresh JWT that was never persisted: must be rejected.
	foreign := signRefreshJWT(t, "unit-test-secret-not-for-production", uuid.New(), time.Now().Add(7*24*time.Hour))
	if _, err := svc.Refresh(context.Background(), foreign); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("err = %v, want ErrInvalidRefreshToken for unpersisted token", err)
	}
}

func TestRefreshExpiredTokenRejected(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuth(t)
	expired := signRefreshJWT(t, "unit-test-secret-not-for-production", uuid.New(), time.Now().Add(-time.Hour))
	if _, err := svc.Refresh(context.Background(), expired); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("err = %v, want ErrInvalidRefreshToken for expired token", err)
	}
}

func TestRefreshAccessTokenRejected(t *testing.T) {
	svc, _, _, _, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "acc@example.com")
	verifyPlayer(t, svc, mailer)
	pair, err := svc.Login(context.Background(), domain.LoginRequest{Email: "acc@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), pair.AccessToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("access token used as refresh = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestLogoutRevokesFamily(t *testing.T) {
	svc, _, _, refreshRepo, _, mailer := newTestAuth(t)
	registerPlayer(t, svc, "out@example.com")
	verifyPlayer(t, svc, mailer)
	pair, err := svc.Login(context.Background(), domain.LoginRequest{Email: "out@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := svc.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	row, err := refreshRepo.GetByHash(context.Background(), hashToken(pair.RefreshToken))
	if err != nil {
		t.Fatalf("fetch row after logout: %v", err)
	}
	if row.RevokedAt == nil {
		t.Error("refresh token not revoked by logout")
	}
	if _, err := svc.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("refresh after logout = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestLogoutUnknownTokenRejected(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuth(t)
	if err := svc.Logout(context.Background(), "not-a-real-token"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("err = %v, want ErrInvalidRefreshToken for unknown token", err)
	}
}
