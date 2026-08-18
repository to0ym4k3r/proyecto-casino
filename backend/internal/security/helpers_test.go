// Package security contains the defense-oriented security test suite for the
// casino backend. All files in this package are _test.go files: they only add
// tests and never modify production code.
package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/handler"
	"casino-backend/internal/repository"
	"casino-backend/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
)

// hashPassword hashes with the minimum bcrypt cost: the hash cost itself is
// asserted elsewhere (bcrypt cost 12 in service/auth.go:60), these tests only
// need a working hash to exercise login paths.
func hashPassword(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(h)
}

// testSecret is a throwaway HMAC secret used only by these tests. It is never
// used by production code and is not a real credential.
const testSecret = "unit-test-secret-not-for-production"

// ---------------------------------------------------------------------------
// In-memory repository implementations (fakes)
// ---------------------------------------------------------------------------

// memUserRepo implements repository.UserRepository in memory.
type memUserRepo struct {
	mu         sync.Mutex
	byEmail    map[string]*domain.User
	byUsername map[string]*domain.User
	byID       map[uuid.UUID]*domain.User
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{
		byEmail:    make(map[string]*domain.User),
		byUsername: make(map[string]*domain.User),
		byID:       make(map[uuid.UUID]*domain.User),
	}
}

func (r *memUserRepo) Create(_ context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byEmail[u.Email]; ok {
		return errors.New("duplicate email")
	}
	if _, ok := r.byUsername[u.Username]; ok {
		return errors.New("duplicate username")
	}
	r.byEmail[u.Email] = u
	r.byUsername[u.Username] = u
	r.byID[u.ID] = u
	return nil
}

func (r *memUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, errors.New("user not found")
}

func (r *memUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (r *memUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u, ok := r.byUsername[username]; ok {
		return u, nil
	}
	return nil, repository.ErrUserNotFound
}

func (r *memUserRepo) Update(_ context.Context, _ *domain.User) error { return nil }

// UpdateProfile mirrors the production repository: it persists exactly the
// three mutable profile fields and keeps the username index coherent.
func (r *memUserRepo) UpdateProfile(_ context.Context, userID uuid.UUID, username, firstName, lastName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	if u.Username != username {
		delete(r.byUsername, u.Username)
		u.Username = username
		r.byUsername[username] = u
	}
	u.FirstName = firstName
	u.LastName = lastName
	u.UpdatedAt = time.Now().UTC()
	return nil
}
func (r *memUserRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (r *memUserRepo) DeletePermanent(_ context.Context, _ uuid.UUID) error {
	return nil
}

// memWalletRepo implements repository.WalletRepository in memory.
type memWalletRepo struct {
	mu       sync.Mutex
	byUserID map[uuid.UUID]*domain.Wallet
	byID     map[uuid.UUID]*domain.Wallet
}

func newMemWalletRepo() *memWalletRepo {
	return &memWalletRepo{
		byUserID: make(map[uuid.UUID]*domain.Wallet),
		byID:     make(map[uuid.UUID]*domain.Wallet),
	}
}

func (r *memWalletRepo) Create(_ context.Context, w *domain.Wallet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byUserID[w.UserID] = w
	r.byID[w.ID] = w
	return nil
}

func (r *memWalletRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Wallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if w, ok := r.byID[id]; ok {
		return w, nil
	}
	return nil, errors.New("wallet not found")
}

func (r *memWalletRepo) GetByUserID(_ context.Context, userID uuid.UUID) (*domain.Wallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if w, ok := r.byUserID[userID]; ok {
		return w, nil
	}
	return nil, errors.New("wallet not found")
}

func (r *memWalletRepo) UpdateBalance(_ context.Context, id uuid.UUID, amount int64, version int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.byID[id]
	if !ok {
		return errors.New("wallet not found")
	}
	if w.Version != version {
		return errors.New("version conflict")
	}
	w.Balance += amount
	w.Version++
	return nil
}

func (r *memWalletRepo) CreateTransaction(_ context.Context, _ *domain.Transaction) error {
	return nil
}

// memKycRepo implements repository.KycRepository in memory with the same
// semantics as the production repository: one submission per user (second
// submit → ErrKycAlreadyExists), status lookups by user ID, and document rows
// recorded for inspection.
type memKycRepo struct {
	mu          sync.Mutex
	submissions map[uuid.UUID]*domain.KycSubmission
	docs        []domain.KycDocument
}

func newMemKycRepo() *memKycRepo {
	return &memKycRepo{submissions: make(map[uuid.UUID]*domain.KycSubmission)}
}

func (r *memKycRepo) Submit(_ context.Context, sub *domain.KycSubmission) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.submissions[sub.UserID]; ok {
		return repository.ErrKycAlreadyExists
	}
	r.submissions[sub.UserID] = sub
	return nil
}

func (r *memKycRepo) GetByUserID(_ context.Context, userID uuid.UUID) (*domain.KycSubmission, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sub, ok := r.submissions[userID]
	if !ok {
		return nil, repository.ErrKycNotFound
	}
	return sub, nil
}

func (r *memKycRepo) CreateDocument(_ context.Context, doc *domain.KycDocument) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.docs = append(r.docs, *doc)
	return nil
}

// memStore implements storage.Store in memory: it records every Save and never
// touches the filesystem, so full-router tests can exercise the upload path.
type memStore struct {
	mu    sync.Mutex
	saves []string
}

func (s *memStore) Save(_ context.Context, key string, r io.Reader) (int64, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = append(s.saves, key)
	return n, nil
}

func (s *memStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = append(s.saves, "delete:"+key)
	return nil
}

// memGameRepo implements repository.GameRepository in memory.
type memGameRepo struct {
	mu    sync.Mutex
	games []domain.Game
}

func newMemGameRepo() *memGameRepo {
	return &memGameRepo{games: make([]domain.Game, 0)}
}

func (r *memGameRepo) Create(_ context.Context, g *domain.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.games = append(r.games, *g)
	return nil
}

func (r *memGameRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Game, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.games {
		if r.games[i].ID == id {
			g := r.games[i]
			return &g, nil
		}
	}
	return nil, errors.New("game not found")
}

func (r *memGameRepo) List(_ context.Context, limit, offset int) ([]domain.Game, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if offset > len(r.games) {
		offset = len(r.games)
	}
	end := offset + limit
	if end > len(r.games) {
		end = len(r.games)
	}
	return r.games[offset:end], len(r.games), nil
}

func (r *memGameRepo) Update(_ context.Context, _ *domain.Game) error { return nil }
func (r *memGameRepo) Delete(_ context.Context, _ uuid.UUID) error    { return nil }

// memRefreshRepo implements repository.RefreshTokenRepository in memory with
// the same family semantics as the production repository (only hashes stored,
// rotate marks replaced_by + revoked, family revocation walks both directions).
type memRefreshRepo struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]*domain.RefreshToken
	byHash   map[string]*domain.RefreshToken
	userRepo *memUserRepo
}

func newMemRefreshRepo(userRepo *memUserRepo) *memRefreshRepo {
	return &memRefreshRepo{
		byID:     make(map[uuid.UUID]*domain.RefreshToken),
		byHash:   make(map[string]*domain.RefreshToken),
		userRepo: userRepo,
	}
}

func (r *memRefreshRepo) Save(_ context.Context, rt *domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[rt.ID] = rt
	r.byHash[rt.TokenHash] = rt
	return nil
}

func (r *memRefreshRepo) GetByHash(_ context.Context, tokenHash string) (*domain.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rt, ok := r.byHash[tokenHash]; ok {
		return rt, nil
	}
	return nil, repository.ErrRefreshTokenNotFound
}

func (r *memRefreshRepo) Rotate(_ context.Context, oldID uuid.UUID, replacement *domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.byID[oldID]
	if !ok || old.RevokedAt != nil || old.ReplacedBy != nil {
		return repository.ErrReplayDetected
	}
	r.byID[replacement.ID] = replacement
	r.byHash[replacement.TokenHash] = replacement
	now := time.Now()
	old.RevokedAt = &now
	old.ReplacedBy = &replacement.ID
	return nil
}

func (r *memRefreshRepo) RevokeFamily(_ context.Context, tokenID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, id := range r.familyIDs(tokenID) {
		if rt, ok := r.byID[id]; ok && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

// familyIDs walks the replaced_by chain in both directions, mirroring the
// recursive CTE in the production repository.
func (r *memRefreshRepo) familyIDs(id uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	stack := []uuid.UUID{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[cur] {
			continue
		}
		rt, ok := r.byID[cur]
		if !ok {
			continue
		}
		seen[cur] = true
		if rt.ReplacedBy != nil {
			stack = append(stack, *rt.ReplacedBy)
		}
		for _, other := range r.byID {
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

// memVerificationRepo implements repository.EmailVerificationRepository in
// memory. Consuming a valid token marks the matching user's email verified so
// the subsequent login gate behaves like the real database.
type memVerificationRepo struct {
	byHash   map[string]*domain.EmailVerification
	userRepo *memUserRepo
}

func newMemVerificationRepo(userRepo *memUserRepo) *memVerificationRepo {
	return &memVerificationRepo{byHash: make(map[string]*domain.EmailVerification), userRepo: userRepo}
}

func (r *memVerificationRepo) Verify(_ context.Context, tokenHash string) error {
	ev, ok := r.byHash[tokenHash]
	if !ok || ev.UsedAt != nil || !ev.ExpiresAt.After(time.Now()) {
		return repository.ErrInvalidVerificationToken
	}
	now := time.Now()
	ev.UsedAt = &now
	if u, ok := r.userRepo.byID[ev.UserID]; ok {
		u.EmailVerifiedAt = &now
	}
	return nil
}

// memRegistrationRepo implements repository.RegistrationRepository in memory,
// mirroring the production atomic transaction: user, wallet, verification and
// refresh rows all land in the shared fakes.
type memRegistrationRepo struct {
	userRepo    *memUserRepo
	walletRepo  *memWalletRepo
	verifyRepo  *memVerificationRepo
	refreshRepo *memRefreshRepo
}

func newMemRegistrationRepo(userRepo *memUserRepo, walletRepo *memWalletRepo, verifyRepo *memVerificationRepo, refreshRepo *memRefreshRepo) *memRegistrationRepo {
	return &memRegistrationRepo{userRepo: userRepo, walletRepo: walletRepo, verifyRepo: verifyRepo, refreshRepo: refreshRepo}
}

func (r *memRegistrationRepo) CreateUserWithWallet(ctx context.Context, in domain.RegistrationInput) error {
	if err := r.userRepo.Create(ctx, in.User); err != nil {
		return err
	}
	if err := r.walletRepo.Create(ctx, in.Wallet); err != nil {
		return err
	}
	r.verifyRepo.byHash[in.Verification.TokenHash] = in.Verification
	r.refreshRepo.byID[in.RefreshToken.ID] = in.RefreshToken
	r.refreshRepo.byHash[in.RefreshToken.TokenHash] = in.RefreshToken
	return nil
}

// stubMailer captures the dispatched verification emails (to + raw token).
type stubMailer struct {
	mu    sync.Mutex
	calls []mailCall
}

type mailCall struct {
	to    string
	token string
}

func (m *stubMailer) SendVerificationEmail(_ context.Context, to, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mailCall{to: to, token: token})
	return nil
}

func (m *stubMailer) lastToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return ""
	}
	return m.calls[len(m.calls)-1].token
}

// hashToken mirrors the service's token hashing so tests can correlate the raw
// token the mailer dispatched with the hash the repositories persisted.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ---------------------------------------------------------------------------
// Test stack construction
// ---------------------------------------------------------------------------

// testHandlers wires the real handlers and services over the in-memory fakes,
// mirroring the dependency graph built in cmd/server/main.go.
type testHandlers struct {
	auth       *handler.AuthHandler
	wallet     *handler.WalletHandler
	game       *handler.GameHandler
	user       *handler.UserHandler
	kyc        *handler.KycHandler
	authSvc    *service.AuthService
	userRepo   *memUserRepo
	walletRepo *memWalletRepo
	gameRepo   *memGameRepo
	kycRepo    *memKycRepo
	mailer     *stubMailer
	jwtCfg     *config.JWTConfig
}

func newTestHandlers(t *testing.T, secret string) *testHandlers {
	t.Helper()
	jwtCfg := &config.JWTConfig{
		Secret:        secret,
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}
	userRepo := newMemUserRepo()
	walletRepo := newMemWalletRepo()
	gameRepo := newMemGameRepo()
	kycRepo := newMemKycRepo()
	refreshRepo := newMemRefreshRepo(userRepo)
	verifyRepo := newMemVerificationRepo(userRepo)
	regRepo := newMemRegistrationRepo(userRepo, walletRepo, verifyRepo, refreshRepo)
	mailer := &stubMailer{}

	authSvc := service.NewAuthService(userRepo, regRepo, refreshRepo, verifyRepo, mailer, jwtCfg, 24*time.Hour)
	walletSvc := service.NewWalletService(walletRepo)
	gameSvc := service.NewGameService(gameRepo)
	userSvc := service.NewUserService(userRepo)
	kycSvc := service.NewKycService(kycRepo)

	return &testHandlers{
		auth:       handler.NewAuthHandler(authSvc),
		wallet:     handler.NewWalletHandler(walletSvc, nil, nil, nil),
		game:       handler.NewGameHandler(gameSvc),
		user:       handler.NewUserHandler(userSvc),
		kyc:        handler.NewKycHandler(kycSvc, &memStore{}),
		authSvc:    authSvc,
		userRepo:   userRepo,
		walletRepo: walletRepo,
		gameRepo:   gameRepo,
		kycRepo:    kycRepo,
		mailer:     mailer,
		jwtCfg:     jwtCfg,
	}
}

// newTestRouter builds the full production chi router (all middleware: RequestID,
// logging, recoverer, security headers, CORS, rate limiters) over the fakes.
func newTestRouter(t *testing.T, secret string) http.Handler {
	t.Helper()
	th := newTestHandlers(t, secret)
	return handler.NewRouter(th.auth, th.game, th.wallet, th.user, th.kyc, nil, th.authSvc, zerolog.Nop(), "http://localhost:3000", "").Setup()
}

// ---------------------------------------------------------------------------
// Token and HTTP helpers
// ---------------------------------------------------------------------------

func signToken(t *testing.T, secret, tokenType string, sub uuid.UUID, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  sub.String(),
		"type": tokenType,
		"exp":  exp.Unix(),
		"iat":  time.Now().Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func signAccess(t *testing.T, secret string, userID uuid.UUID) string {
	t.Helper()
	return signToken(t, secret, "access", userID, time.Now().Add(15*time.Minute))
}

func signRefresh(t *testing.T, secret string, userID uuid.UUID) string {
	t.Helper()
	return signToken(t, secret, "refresh", userID, time.Now().Add(7*24*time.Hour))
}

// doJSON performs an HTTP request against h with a fixed client IP so
// per-IP rate limiters see a deterministic RemoteAddr.
func doJSON(method, target string, h http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// registerBody builds a plain-JSON register payload. Values must be JSON-safe;
// use json.Marshal where payloads may contain quotes (see sqli_http_test.go).
func registerBody(username, email, password string) string {
	return `{"username":"` + username + `","email":"` + email + `","password":"` + password +
		`","first_name":"Fn","last_name":"Ln","date_of_birth":"1990-01-01"}`
}
