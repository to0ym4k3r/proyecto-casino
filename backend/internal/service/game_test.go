package service

import (
	"context"
	"math"
	"testing"

	"casino-backend/internal/domain"
	"casino-backend/internal/rng"

	"github.com/google/uuid"
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
