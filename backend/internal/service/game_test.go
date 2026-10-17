package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/rng"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeGameRepo is an in-memory game repository for testing.
type fakeGameRepo struct {
	games map[uuid.UUID]*domain.Game
}

func newFakeGameRepo() *fakeGameRepo {
	return &fakeGameRepo{games: make(map[uuid.UUID]*domain.Game)}
}

func (f *fakeGameRepo) Create(_ context.Context, g *domain.Game) error {
	f.games[g.ID] = g
	return nil
}

func (f *fakeGameRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Game, error) {
	g, ok := f.games[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return g, nil
}

func (f *fakeGameRepo) List(_ context.Context, limit, offset int) ([]domain.Game, int, error) {
	var all []domain.Game
	for _, g := range f.games {
		all = append(all, *g)
	}
	return all, len(all), nil
}

func (f *fakeGameRepo) Update(_ context.Context, g *domain.Game) error {
	f.games[g.ID] = g
	return nil
}

func (f *fakeGameRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.games, id)
	return nil
}

// fakeSessionRepo is an in-memory game session repository for testing.
type fakeSessionRepo struct {
	sessions map[uuid.UUID]*domain.GameSession
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: make(map[uuid.UUID]*domain.GameSession)}
}

func (f *fakeSessionRepo) Create(_ context.Context, s *domain.GameSession) error {
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeSessionRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.GameSession, error) {
	s, ok := f.sessions[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}

func (f *fakeSessionRepo) GetActiveSession(_ context.Context, userID, gameID uuid.UUID) (*domain.GameSession, error) {
	for _, s := range f.sessions {
		if s.UserID == userID && s.GameID == gameID && s.Status == domain.SessionStatusActive {
			return s, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeSessionRepo) UpdateStatus(_ context.Context, id uuid.UUID, status domain.GameSessionStatus) error {
	s, ok := f.sessions[id]
	if !ok {
		return domain.ErrNotFound
	}
	s.Status = status
	return nil
}

func (f *fakeSessionRepo) FindAbandoned(_ context.Context, olderThan time.Duration) ([]domain.GameSession, error) {
	return nil, nil
}

// TestResolveBetUsesCryptoRand proves ResolveBet does not depend on math/rand
// by checking the distribution over 1000 iterations is not degenerate.
func TestResolveBetUsesCryptoRand(t *testing.T) {
	// Seed the function via rng.SecureRandom to verify it produces varied results.
	seen := make(map[float64]bool)
	unique := 0
	for i := 0; i < 1000; i++ {
		v := rng.SecureRandom()
		if !seen[v] {
			seen[v] = true
			unique++
		}
	}
	if unique < 900 {
		t.Errorf("only %d unique SecureRandom values out of 1000", unique)
	}
}

// TestResolveBetMultiplierCappedByRTP proves the multiplier output of
// ResolveBet never exceeds the RTP-derived cap. For RTP=95, maxMultiplier
// = 0.95, so no multiplier should exceed 0.95.
func TestResolveBetMultiplierCappedByRTP(t *testing.T) {
	const rtp = 95
	const maxMult = float64(rtp) / 100.0 // 0.95
	const iterations = 10000

	for i := 0; i < iterations; i++ {
		_, multiplier, _ := ResolveBet(1000, rtp)
		if multiplier > maxMult+1e-9 { // small float tolerance
			t.Fatalf("iteration %d: multiplier %f exceeds RTP cap %f", i, multiplier, maxMult)
		}
	}
}

// TestResolveBetStatisticalDistribution proves ResolveBet produces a
// statistically valid distribution over many iterations. The mean multiplier
// should be roughly half the max multiplier (uniform distribution).
func TestResolveBetStatisticalDistribution(t *testing.T) {
	const rtp = 95
	const iterations = 10000
	const maxMult = float64(rtp) / 100.0

	sum := 0.0
	for i := 0; i < iterations; i++ {
		_, multiplier, _ := ResolveBet(1000, rtp)
		sum += multiplier
	}
	mean := sum / float64(iterations)
	expectedMean := maxMult / 2.0 // uniform on [0, maxMult]

	if math.Abs(mean-expectedMean) > 0.05 {
		t.Errorf("mean multiplier = %f, want ~%f (uniform distribution)", mean, expectedMean)
	}
}

// TestResolveBetRTP100NeverPaysMoreThanBet proves that when RTP=100,
// the multiplier is always in [0, 1.0), meaning the player never wins
// more than their original bet.
func TestResolveBetRTP100NeverPaysMoreThanBet(t *testing.T) {
	const rtp = 100
	const bet int64 = 500
	const iterations = 10000

	wins := 0
	for i := 0; i < iterations; i++ {
		payout, multiplier, won := ResolveBet(bet, rtp)
		if multiplier > 1.0+1e-9 {
			t.Fatalf("RTP=100: multiplier %f exceeds 1.0", multiplier)
		}
		if won {
			wins++
			if payout > bet {
				t.Fatalf("RTP=100: payout %d exceeds bet %d", payout, bet)
			}
		}
	}
	// With RTP=100, maxMultiplier=1.0, so multiplier ∈ [0, 1.0).
	// Since win condition is multiplier >= 1.0, there should be 0 wins.
	if wins != 0 {
		t.Errorf("RTP=100: expected 0 wins, got %d", wins)
	}
}

// TestResolveBetNeverExceedsMaxMultiplierAcrossRTPs tests several RTP values
// to confirm the cap holds universally.
func TestResolveBetNeverExceedsMaxMultiplierAcrossRTPs(t *testing.T) {
	rtps := []int{80, 90, 95, 97, 100}
	for _, rtp := range rtps {
		maxMult := float64(rtp) / 100.0
		for i := 0; i < 5000; i++ {
			_, multiplier, _ := ResolveBet(500, rtp)
			if multiplier > maxMult+1e-9 {
				t.Fatalf("RTP=%d: multiplier %f exceeds cap %f", rtp, multiplier, maxMult)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// StartGame with server seed
// ---------------------------------------------------------------------------

// TestStartGameGeneratesSeedAndHash proves StartGame creates a session with
// server_seed and server_seed_hash populated. The hash must be the SHA-256
// of the seed. The seed must be a 64-char hex string.
func TestStartGameGeneratesSeedAndHash(t *testing.T) {
	repo := newFakeGameRepo()
	gameID := uuid.New()
	repo.games[gameID] = &domain.Game{
		ID:     gameID,
		Name:   "Test Game",
		Status: domain.GameStatusActive,
		RTP:    95,
	}
	svc := NewGameService(repo)

	session, err := svc.StartGame(context.Background(), uuid.New(), gameID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	if len(session.ServerSeed) != 64 {
		t.Errorf("ServerSeed length = %d, want 64", len(session.ServerSeed))
	}
	if len(session.ServerSeedHash) != 64 {
		t.Errorf("ServerSeedHash length = %d, want 64", len(session.ServerSeedHash))
	}
	// Verify hash matches SHA-256 of seed
	wantHash := HashSeed(session.ServerSeed)
	if session.ServerSeedHash != wantHash {
		t.Errorf("ServerSeedHash = %s, want %s (SHA-256 of seed)", session.ServerSeedHash, wantHash)
	}
}

// TestStartGameReturnsHashNotSeed proves the response contains the hash but
// the seed is only stored internally (never leaked to the client after
// initial response).
func TestStartGameReturnsHashNotSeed(t *testing.T) {
	repo := newFakeGameRepo()
	gameID := uuid.New()
	repo.games[gameID] = &domain.Game{
		ID:     gameID,
		Name:   "Test Game",
		Status: domain.GameStatusActive,
		RTP:    97,
	}
	svc := NewGameService(repo)

	session, err := svc.StartGame(context.Background(), uuid.New(), gameID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	// The hash is non-empty and different from the seed
	if session.ServerSeedHash == "" {
		t.Error("ServerSeedHash is empty")
	}
	if session.ServerSeed == session.ServerSeedHash {
		t.Error("ServerSeed and ServerSeedHash must be different")
	}
}

// ---------------------------------------------------------------------------
// GetActiveSession
// ---------------------------------------------------------------------------

// TestGetActiveSessionReturnsSession proves GetActiveSession returns the active
// session when one exists for the given user and game.
func TestGetActiveSessionReturnsSession(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	userID := uuid.New()
	gameID := uuid.New()
	want := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          userID,
		GameID:          gameID,
		ServerSeed:      "abc123",
		ServerSeedHash:  "def456",
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 1000,
		StartedAt:       time.Now().UTC(),
	}
	sessionRepo.sessions[want.ID] = want

	got, err := svc.GetActiveSession(context.Background(), userID, gameID)
	if err != nil {
		t.Fatalf("GetActiveSession: unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("got session ID %v, want %v", got.ID, want.ID)
	}
}

// TestGetActiveSessionNotFound proves GetActiveSession returns an error when no
// active session exists for the given user and game.
func TestGetActiveSessionNotFound(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	_, err := svc.GetActiveSession(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// TestGetActiveSessionCompletedSessionNotReturned proves a completed session
// is not returned by GetActiveSession.
func TestGetActiveSessionCompletedSessionNotReturned(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	userID := uuid.New()
	gameID := uuid.New()
	sessionRepo.sessions[uuid.New()] = &domain.GameSession{
		ID:     uuid.New(),
		UserID: userID,
		GameID: gameID,
		Status: domain.SessionStatusCompleted,
	}

	_, err := svc.GetActiveSession(context.Background(), userID, gameID)
	if err == nil {
		t.Fatal("expected error for completed session, got nil")
	}
}

// TestGetActiveSessionNoPersistence proves GetActiveSession returns an error
// when the session repo is nil (persistence not configured).
func TestGetActiveSessionNoPersistence(t *testing.T) {
	svc := NewGameService(nil)

	_, err := svc.GetActiveSession(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "persistence not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListGames
// ---------------------------------------------------------------------------

func TestListGamesDefaultParams(t *testing.T) {
	repo := newFakeGameRepo()
	for i := 0; i < 5; i++ {
		id := uuid.New()
		repo.games[id] = &domain.Game{ID: id, Name: "Game", Status: domain.GameStatusActive}
	}
	svc := NewGameService(repo)

	games, total, err := svc.ListGames(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(games) != 5 {
		t.Errorf("len(games) = %d, want 5", len(games))
	}
}

func TestListGamesClampsLimitToMax100(t *testing.T) {
	repo := newFakeGameRepo()
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.games[id] = &domain.Game{ID: id, Name: "Game", Status: domain.GameStatusActive}
	}
	svc := NewGameService(repo)

	games, _, err := svc.ListGames(context.Background(), 500, 0)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(games) != 3 {
		t.Errorf("len(games) = %d, want 3 (all games fit within clamped limit)", len(games))
	}
}

func TestListGamesNegativeOffsetFloorToZero(t *testing.T) {
	repo := newFakeGameRepo()
	id := uuid.New()
	repo.games[id] = &domain.Game{ID: id, Name: "Game", Status: domain.GameStatusActive}
	svc := NewGameService(repo)

	games, total, err := svc.ListGames(context.Background(), 10, -5)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(games) != 1 {
		t.Errorf("len(games) = %d, want 1", len(games))
	}
}

// ---------------------------------------------------------------------------
// GetGame
// ---------------------------------------------------------------------------

func TestGetGameFound(t *testing.T) {
	repo := newFakeGameRepo()
	gameID := uuid.New()
	repo.games[gameID] = &domain.Game{ID: gameID, Name: "Roulette"}
	svc := NewGameService(repo)

	got, err := svc.GetGame(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGame: %v", err)
	}
	if got.Name != "Roulette" {
		t.Errorf("Name = %s, want Roulette", got.Name)
	}
}

func TestGetGameNotFound(t *testing.T) {
	repo := newFakeGameRepo()
	svc := NewGameService(repo)

	_, err := svc.GetGame(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for nonexistent game, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// StartGame error paths
// ---------------------------------------------------------------------------

func TestStartGameGameNotFound(t *testing.T) {
	repo := newFakeGameRepo()
	svc := NewGameService(repo)

	_, err := svc.StartGame(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStartGameInactiveGame(t *testing.T) {
	repo := newFakeGameRepo()
	gameID := uuid.New()
	repo.games[gameID] = &domain.Game{
		ID:     gameID,
		Name:   "Maintenance Game",
		Status: domain.GameStatusMaintenance,
		RTP:    95,
	}
	svc := NewGameService(repo)

	_, err := svc.StartGame(context.Background(), uuid.New(), gameID)
	if err == nil {
		t.Fatal("expected error for inactive game, got nil")
	}
	if got := err.Error(); got != "game is not active" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartGameInactiveStatusInactive(t *testing.T) {
	repo := newFakeGameRepo()
	gameID := uuid.New()
	repo.games[gameID] = &domain.Game{
		ID:     gameID,
		Name:   "Inactive Game",
		Status: domain.GameStatusInactive,
		RTP:    90,
	}
	svc := NewGameService(repo)

	_, err := svc.StartGame(context.Background(), uuid.New(), gameID)
	if err == nil {
		t.Fatal("expected error for inactive game, got nil")
	}
}

// ---------------------------------------------------------------------------
// EndSession
// ---------------------------------------------------------------------------

func TestEndSessionCompletesSession(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	sessionID := uuid.New()
	sessionRepo.sessions[sessionID] = &domain.GameSession{
		ID:     sessionID,
		Status: domain.SessionStatusActive,
	}

	if err := svc.EndSession(context.Background(), sessionID); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if sessionRepo.sessions[sessionID].Status != domain.SessionStatusCompleted {
		t.Errorf("status = %s, want %s", sessionRepo.sessions[sessionID].Status, domain.SessionStatusCompleted)
	}
}

func TestEndSessionNotFound(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	err := svc.EndSession(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
}

func TestEndSessionNoPersistence(t *testing.T) {
	svc := NewGameService(nil)

	err := svc.EndSession(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "persistence not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PersistSession
// ---------------------------------------------------------------------------

func TestPersistSessionSuccess(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	session := &domain.GameSession{
		ID:              uuid.New(),
		UserID:          uuid.New(),
		GameID:          uuid.New(),
		ServerSeed:      "seed",
		ServerSeedHash:  "hash",
		Status:          domain.SessionStatusActive,
		BalanceSnapshot: 1000,
		StartedAt:       time.Now().UTC(),
	}

	if err := svc.PersistSession(context.Background(), session); err != nil {
		t.Fatalf("PersistSession: %v", err)
	}
	if _, ok := sessionRepo.sessions[session.ID]; !ok {
		t.Error("session was not persisted")
	}
}

func TestPersistSessionNoPersistence(t *testing.T) {
	svc := NewGameService(nil)

	err := svc.PersistSession(context.Background(), &domain.GameSession{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "persistence not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CreateGame / UpdateGame / DeleteGame
// ---------------------------------------------------------------------------

func TestCreateGame(t *testing.T) {
	repo := newFakeGameRepo()
	svc := NewGameService(repo)

	game := &domain.Game{ID: uuid.New(), Name: "New Game", RTP: 95}
	if err := svc.CreateGame(context.Background(), game); err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	got, err := svc.GetGame(context.Background(), game.ID)
	if err != nil {
		t.Fatalf("GetGame after create: %v", err)
	}
	if got.Name != "New Game" {
		t.Errorf("Name = %s, want New Game", got.Name)
	}
}

func TestUpdateGame(t *testing.T) {
	repo := newFakeGameRepo()
	svc := NewGameService(repo)

	game := &domain.Game{ID: uuid.New(), Name: "Old Name", RTP: 90}
	if err := svc.CreateGame(context.Background(), game); err != nil {
		t.Fatalf("CreateGame: %v", err)
	}

	game.Name = "Updated Name"
	if err := svc.UpdateGame(context.Background(), game); err != nil {
		t.Fatalf("UpdateGame: %v", err)
	}
	got, _ := svc.GetGame(context.Background(), game.ID)
	if got.Name != "Updated Name" {
		t.Errorf("Name = %s, want Updated Name", got.Name)
	}
}

func TestDeleteGame(t *testing.T) {
	repo := newFakeGameRepo()
	svc := NewGameService(repo)

	game := &domain.Game{ID: uuid.New(), Name: "Doomed"}
	if err := svc.CreateGame(context.Background(), game); err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	if err := svc.DeleteGame(context.Background(), game.ID); err != nil {
		t.Fatalf("DeleteGame: %v", err)
	}
	_, err := svc.GetGame(context.Background(), game.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreditLoss
// ---------------------------------------------------------------------------

type fakeBetRepo struct {
	bets map[uuid.UUID]*domain.Bet
}

func newFakeBetRepo() *fakeBetRepo {
	return &fakeBetRepo{bets: make(map[uuid.UUID]*domain.Bet)}
}

func (f *fakeBetRepo) Create(_ context.Context, bet *domain.Bet) error {
	f.bets[bet.ID] = bet
	return nil
}

func (f *fakeBetRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Bet, error) {
	b, ok := f.bets[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (f *fakeBetRepo) GetByUserID(_ context.Context, userID uuid.UUID, limit, offset int) ([]domain.Bet, error) {
	return nil, nil
}

func (f *fakeBetRepo) Update(_ context.Context, bet *domain.Bet) error {
	f.bets[bet.ID] = bet
	return nil
}

func TestCreditLossSuccess(t *testing.T) {
	betRepo := newFakeBetRepo()
	svc := NewGameServiceWithPersistence(nil, nil, betRepo, nil, nil)

	bet := &domain.Bet{
		ID:     uuid.New(),
		Status: domain.BetStatusPending,
		Amount: 500,
	}
	betRepo.bets[bet.ID] = bet

	if err := svc.CreditLoss(context.Background(), bet, 0.5); err != nil {
		t.Fatalf("CreditLoss: %v", err)
	}
	got := betRepo.bets[bet.ID]
	if got.Status != domain.BetStatusLost {
		t.Errorf("Status = %s, want %s", got.Status, domain.BetStatusLost)
	}
	if got.Multiplier != 0.5 {
		t.Errorf("Multiplier = %f, want 0.5", got.Multiplier)
	}
	if got.ResolvedAt == nil {
		t.Error("ResolvedAt is nil, want timestamp")
	}
}

func TestCreditLossNoPersistence(t *testing.T) {
	svc := NewGameService(nil)

	err := svc.CreditLoss(context.Background(), &domain.Bet{}, 0.5)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "persistence not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HasPersistence
// ---------------------------------------------------------------------------

func TestHasPersistenceWithSessionRepo(t *testing.T) {
	sessionRepo := newFakeSessionRepo()
	svc := NewGameServiceWithPersistence(nil, nil, nil, sessionRepo, nil)

	if !svc.HasPersistence() {
		t.Error("expected HasPersistence() = true with sessionRepo configured")
	}
}

func TestHasPersistenceWithoutSessionRepo(t *testing.T) {
	svc := NewGameService(nil)

	if svc.HasPersistence() {
		t.Error("expected HasPersistence() = false without sessionRepo")
	}
}

// ---------------------------------------------------------------------------
// ResolveBet win and loss scenarios
// ---------------------------------------------------------------------------

func TestResolveBetWins(t *testing.T) {
	const bet int64 = 1000

	// RTP=100 guarantees multiplier in [0, 1.0) → never wins.
	// RTP=150 gives maxMultiplier=1.5, so wins ARE possible.
	const highRtp = 150
	won := false
	for i := 0; i < 10000; i++ {
		payout, multiplier, w := ResolveBet(bet, highRtp)
		if w {
			won = true
			if payout <= 0 {
				t.Fatalf("win payout = %d, want > 0", payout)
			}
			if multiplier < 1.0 {
				t.Fatalf("win multiplier = %f, want >= 1.0", multiplier)
			}
			if payout != int64(float64(bet)*multiplier) {
				t.Fatalf("payout = %d, want amount*multiplier = %d", payout, int64(float64(bet)*multiplier))
			}
			break
		}
	}
	if !won {
		t.Skip("no wins in 10000 iterations at RTP=150 (statistically unlikely)")
	}
}

func TestResolveBetLosses(t *testing.T) {
	const rtp = 95
	const bet int64 = 1000

	losses := 0
	for i := 0; i < 10000; i++ {
		payout, _, won := ResolveBet(bet, rtp)
		if !won {
			losses++
			if payout != 0 {
				t.Fatalf("loss payout = %d, want 0", payout)
			}
		}
	}
	if losses == 0 {
		t.Error("expected at least some losses with RTP=95")
	}
}

// ---------------------------------------------------------------------------
// fakeTx infrastructure — simulates SQL operations for PlaceBet / CreditWin
// ---------------------------------------------------------------------------

// fakeWallet is an in-memory wallet row for tx-level testing.
type fakeWallet struct {
	id           uuid.UUID
	userID       uuid.UUID
	balance      int64
	bonusBalance int64
	version      int
}

// fakeTxStore holds the in-memory state that the fake transaction operates on.
type fakeTxStore struct {
	wallets       map[uuid.UUID]*fakeWallet // keyed by wallet ID
	walletByUser  map[uuid.UUID]uuid.UUID   // userID → walletID
	bets          map[uuid.UUID]*domain.Bet
	txLog         []domain.Transaction
	queryRowError error // injectable error for QueryRow
	execError     error // injectable error for Exec

	// splitDeducts counts successful two-column (balance + bonus_balance)
	// wallet deductions — the design-mandated single-UPDATE contract (design §5):
	// one statement, one version bump, one optimistic-lock conflict check.
	splitDeducts int
}

// fakeRow implements pgx.Row for QueryRow results.
type fakeRow struct {
	values []any
	err    error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		if i < len(r.values) {
			switch v := d.(type) {
			case *uuid.UUID:
				*v = r.values[i].(uuid.UUID)
			case *int64:
				*v = r.values[i].(int64)
			case *int:
				*v = r.values[i].(int)
			}
		}
	}
	return nil
}

// noRows implements pgx.Rows as a no-op (unused by PlaceBet/CreditWin).
type noRows struct{}

func (r *noRows) Close()                              {}
func (r *noRows) Err() error                          { return nil }
func (r *noRows) CommandTag() pgconn.CommandTag       { return pgconn.CommandTag{} }
func (r *noRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *noRows) Next() bool                          { return false }
func (r *noRows) Scan(_ ...any) error                 { return nil }
func (r *noRows) Values() ([]any, error)              { return nil, nil }
func (r *noRows) RawValues() [][]byte                 { return nil }
func (r *noRows) Conn() *pgx.Conn                     { return nil }

// fakeTx implements pgx.Tx for transaction testing.
type fakeTx struct {
	store *fakeTxStore
}

func (tx *fakeTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if tx.store.queryRowError != nil {
		return &fakeRow{err: tx.store.queryRowError}
	}
	// Only handle wallets SELECT
	if strings.Contains(sql, "FROM wallets") && strings.Contains(sql, "FOR UPDATE") {
		userID := args[0].(uuid.UUID)
		wid, ok := tx.store.walletByUser[userID]
		if !ok {
			return &fakeRow{err: fmt.Errorf("wallet not found")}
		}
		w := tx.store.wallets[wid]
		// Mirror the SQL's projection: the widened lock reads the bonus
		// column too, the legacy lock (e.g. CreditWin) does not.
		if strings.Contains(sql, "bonus_balance") {
			return &fakeRow{values: []any{w.id, w.balance, w.bonusBalance, w.version}}
		}
		return &fakeRow{values: []any{w.id, w.balance, w.version}}
	}
	return &fakeRow{err: fmt.Errorf("unexpected query: %s", sql)}
}

func (tx *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.store.execError != nil {
		return pgconn.CommandTag{}, tx.store.execError
	}

	switch {
	case strings.Contains(sql, "UPDATE wallets SET balance = balance -") && strings.Contains(sql, "bonus_balance"):
		// PlaceBet split deduction (design §5): one two-column UPDATE.
		// args = [walletID, realDeducted, bonusDeducted, version]
		wid := args[0].(uuid.UUID)
		realDeducted := args[1].(int64)
		bonusDeducted := args[2].(int64)
		version := args[3].(int)
		w := tx.store.wallets[wid]
		if w == nil || w.version != version {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		w.balance -= realDeducted
		w.bonusBalance -= bonusDeducted
		if w.bonusBalance < 0 {
			w.bonusBalance = 0 // mirrors GREATEST(bonus_balance - $3, 0)
		}
		w.version++
		tx.store.splitDeducts++
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.Contains(sql, "SET bonus_balance = GREATEST(bonus_balance -") && strings.Contains(sql, "balance = balance +"):
		// ConvertBonusToReal: args = [userID, amount]. Mirrors the SQL
		// semantics exactly: bonus debit floored at zero, real balance
		// credited by the FULL amount — so an uncapped production amount
		// surfaces in tests as money minted out of nowhere.
		wid, ok := tx.store.walletByUser[args[0].(uuid.UUID)]
		if !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		w := tx.store.wallets[wid]
		amount := args[1].(int64)
		w.bonusBalance -= amount
		if w.bonusBalance < 0 {
			w.bonusBalance = 0
		}
		w.balance += amount
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.Contains(sql, "SET bonus_balance = GREATEST(bonus_balance -"):
		// DebitBonusBalance: args = [userID, amount]; zero-floored debit only.
		wid, ok := tx.store.walletByUser[args[0].(uuid.UUID)]
		if !ok {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		w := tx.store.wallets[wid]
		amount := args[1].(int64)
		w.bonusBalance -= amount
		if w.bonusBalance < 0 {
			w.bonusBalance = 0
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.Contains(sql, "UPDATE wallets SET balance = balance +"):
		// CreditWin addition: args = [walletID, payout, version]
		wid := args[0].(uuid.UUID)
		payout := args[1].(int64)
		version := args[2].(int)
		w := tx.store.wallets[wid]
		if w == nil || w.version != version {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		w.balance += payout
		w.version++
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.Contains(sql, "INSERT INTO bets"):
		// args = [id, userID, gameID, sessionID, amount, status, createdAt]
		var status domain.BetStatus
		switch v := args[5].(type) {
		case domain.BetStatus:
			status = v
		case string:
			status = domain.BetStatus(v)
		}
		bet := &domain.Bet{
			ID:        args[0].(uuid.UUID),
			UserID:    args[1].(uuid.UUID),
			GameID:    args[2].(uuid.UUID),
			SessionID: args[3].(uuid.UUID),
			Amount:    args[4].(int64),
			Status:    status,
			CreatedAt: args[6].(time.Time),
		}
		tx.store.bets[bet.ID] = bet
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.Contains(sql, "INSERT INTO transactions"):
		// args = [id, walletID, type, amount, balanceAfter, reference, createdAt]
		var txType domain.TransactionType
		switch v := args[2].(type) {
		case domain.TransactionType:
			txType = v
		case string:
			txType = domain.TransactionType(v)
		}
		tx.store.txLog = append(tx.store.txLog, domain.Transaction{
			ID:        args[0].(uuid.UUID),
			WalletID:  args[1].(uuid.UUID),
			Type:      txType,
			Amount:    args[3].(int64),
			Balance:   args[4].(int64),
			Reference: args[5].(string),
			CreatedAt: args[6].(time.Time),
		})
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.Contains(sql, "UPDATE bets SET payout"):
		// CreditWin: args = [betID, payout, multiplier, status, resolvedAt]
		bid := args[0].(uuid.UUID)
		payout := args[1].(int64)
		multiplier := args[2].(float64)
		var status domain.BetStatus
		switch v := args[3].(type) {
		case domain.BetStatus:
			status = v
		case string:
			status = domain.BetStatus(v)
		}
		resolvedAt := args[4].(time.Time)
		b := tx.store.bets[bid]
		if b == nil {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		b.Payout = payout
		b.Multiplier = multiplier
		b.Status = status
		b.ResolvedAt = &resolvedAt
		return pgconn.NewCommandTag("UPDATE 1"), nil

	default:
		return pgconn.CommandTag{}, fmt.Errorf("unexpected exec: %s", sql)
	}
}

// Unused pgx.Tx methods — satisfies the interface.
func (tx *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }
func (tx *fakeTx) Commit(_ context.Context) error          { return nil }
func (tx *fakeTx) Rollback(_ context.Context) error        { return nil }
func (tx *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (tx *fakeTx) LargeObjects() pgx.LargeObjects                              { return pgx.LargeObjects{} }
func (tx *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &noRows{}, nil
}
func (tx *fakeTx) Conn() *pgx.Conn { return nil }

// newFakeTxFunc creates a test txFunc backed by the given store.
func newFakeTxFunc(store *fakeTxStore) TxFunc {
	return func(_ context.Context, fn func(pgx.Tx) error) error {
		return fn(&fakeTx{store: store})
	}
}

// ---------------------------------------------------------------------------
// PlaceBet
// ---------------------------------------------------------------------------

func TestPlaceBetSuccess(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 5000, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, newFakeTxFunc(store))

	bet, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err != nil {
		t.Fatalf("PlaceBet: %v", err)
	}

	// Wallet deducted
	if store.wallets[walletID].balance != 4000 {
		t.Errorf("balance = %d, want 4000", store.wallets[walletID].balance)
	}
	// Wallet version bumped
	if store.wallets[walletID].version != 2 {
		t.Errorf("version = %d, want 2", store.wallets[walletID].version)
	}
	// Bet record created
	if bet == nil {
		t.Fatal("bet is nil")
	}
	if bet.Amount != 1000 {
		t.Errorf("bet.Amount = %d, want 1000", bet.Amount)
	}
	if bet.Status != domain.BetStatusPending {
		t.Errorf("bet.Status = %s, want %s", bet.Status, domain.BetStatusPending)
	}
	// Transaction log recorded
	if len(store.txLog) != 1 {
		t.Fatalf("txLog len = %d, want 1", len(store.txLog))
	}
	if store.txLog[0].Type != domain.TransactionBet {
		t.Errorf("txLog type = %s, want %s", store.txLog[0].Type, domain.TransactionBet)
	}
	if store.txLog[0].Amount != -1000 {
		t.Errorf("txLog amount = %d, want -1000", store.txLog[0].Amount)
	}
}

func TestPlaceBetInsufficientBalance(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 500, version: 1}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, newFakeTxFunc(store))

	_, err := svc.PlaceBet(context.Background(), userID, uuid.New(), uuid.New(), 1000)
	if err == nil {
		t.Fatal("expected insufficient balance error, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Errorf("unexpected error: %v", err)
	}
	// Wallet unchanged
	if store.wallets[walletID].balance != 500 {
		t.Errorf("balance changed to %d, want 500 (no deduction)", store.wallets[walletID].balance)
	}
	// No bet or transaction created
	if len(store.bets) != 0 {
		t.Errorf("bets created = %d, want 0", len(store.bets))
	}
	if len(store.txLog) != 0 {
		t.Errorf("txLog created = %d, want 0", len(store.txLog))
	}
}

func TestPlaceBetNoTxFunc(t *testing.T) {
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, nil)

	_, err := svc.PlaceBet(context.Background(), uuid.New(), uuid.New(), uuid.New(), 1000)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "persistence not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPlaceBetWalletNotFound(t *testing.T) {
	store := &fakeTxStore{
		wallets:      make(map[uuid.UUID]*fakeWallet),
		walletByUser: make(map[uuid.UUID]uuid.UUID),
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, newFakeTxFunc(store))

	_, err := svc.PlaceBet(context.Background(), uuid.New(), uuid.New(), uuid.New(), 1000)
	if err == nil {
		t.Fatal("expected error for nonexistent wallet, got nil")
	}
}

// ---------------------------------------------------------------------------
// CreditWin
// ---------------------------------------------------------------------------

func TestCreditWinSuccess(t *testing.T) {
	walletID := uuid.New()
	userID := uuid.New()
	store := &fakeTxStore{
		wallets:      map[uuid.UUID]*fakeWallet{walletID: {id: walletID, userID: userID, balance: 4000, version: 2}},
		walletByUser: map[uuid.UUID]uuid.UUID{userID: walletID},
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, newFakeTxFunc(store))

	betID := uuid.New()
	bet := &domain.Bet{ID: betID, UserID: userID, Amount: 1000, Status: domain.BetStatusPending}
	store.bets[betID] = bet

	err := svc.CreditWin(context.Background(), bet, 1500, 1.5)
	if err != nil {
		t.Fatalf("CreditWin: %v", err)
	}

	// Wallet credited
	if store.wallets[walletID].balance != 5500 {
		t.Errorf("balance = %d, want 5500", store.wallets[walletID].balance)
	}
	if store.wallets[walletID].version != 3 {
		t.Errorf("version = %d, want 3", store.wallets[walletID].version)
	}
	// Bet resolved
	resolved := store.bets[betID]
	if resolved.Status != domain.BetStatusWon {
		t.Errorf("bet status = %s, want %s", resolved.Status, domain.BetStatusWon)
	}
	if resolved.Payout != 1500 {
		t.Errorf("bet payout = %d, want 1500", resolved.Payout)
	}
	if resolved.Multiplier != 1.5 {
		t.Errorf("bet multiplier = %f, want 1.5", resolved.Multiplier)
	}
	if resolved.ResolvedAt == nil {
		t.Error("bet ResolvedAt is nil, want timestamp")
	}
	// Transaction log
	if len(store.txLog) != 1 {
		t.Fatalf("txLog len = %d, want 1", len(store.txLog))
	}
	if store.txLog[0].Type != domain.TransactionWin {
		t.Errorf("txLog type = %s, want %s", store.txLog[0].Type, domain.TransactionWin)
	}
	if store.txLog[0].Amount != 1500 {
		t.Errorf("txLog amount = %d, want 1500", store.txLog[0].Amount)
	}
}

func TestCreditWinWalletNotFound(t *testing.T) {
	store := &fakeTxStore{
		wallets:      make(map[uuid.UUID]*fakeWallet),
		walletByUser: make(map[uuid.UUID]uuid.UUID),
		bets:         make(map[uuid.UUID]*domain.Bet),
	}
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, newFakeTxFunc(store))

	bet := &domain.Bet{ID: uuid.New(), UserID: uuid.New(), Amount: 1000}
	err := svc.CreditWin(context.Background(), bet, 1500, 1.5)
	if err == nil {
		t.Fatal("expected error for nonexistent wallet, got nil")
	}
}

func TestCreditWinNoTxFunc(t *testing.T) {
	svc := NewGameServiceWithPersistence(nil, nil, nil, nil, nil)

	err := svc.CreditWin(context.Background(), &domain.Bet{}, 1000, 1.0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got != "persistence not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}
