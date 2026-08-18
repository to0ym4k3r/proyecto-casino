package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"
	"casino-backend/internal/rng"
	"casino-backend/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGameRepo for handler tests — returns a fixed game.
type stubGameRepo struct {
	game domain.Game
}

func (s *stubGameRepo) List(_ context.Context, _, _ int) ([]domain.Game, int, error) {
	return []domain.Game{s.game}, 1, nil
}
func (s *stubGameRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Game, error) {
	return &s.game, nil
}
func (s *stubGameRepo) Create(_ context.Context, g *domain.Game) error  { s.game = *g; return nil }
func (s *stubGameRepo) Update(_ context.Context, g *domain.Game) error  { s.game = *g; return nil }
func (s *stubGameRepo) Delete(_ context.Context, _ uuid.UUID) error     { return nil }

func setupGameHandler(t *testing.T) (*GameHandler, *domain.Game) {
	t.Helper()
	game := domain.Game{
		ID:          uuid.New(),
		Name:        "Blackjack",
		Description: "Classic card game",
		Category:    "card",
		MinBet:      100,
		MaxBet:      10000,
		RTP:         99,
		Status:      domain.GameStatusActive,
	}
	repo := &stubGameRepo{game: game}
	svc := service.NewGameService(repo)
	h := NewGameHandler(svc)
	return h, &game
}

// decodeEnvelope unwraps the APIResponse envelope and returns the "data" field.
func decodeEnvelope(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var envelope response.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &envelope)
	require.NoError(t, err, "failed to decode response envelope")
	require.True(t, envelope.Success, "expected success=true, got error: %s", envelope.Error)
	require.NotNil(t, envelope.Data, "response data should not be nil")

	data, ok := envelope.Data.(map[string]interface{})
	require.True(t, ok, "response data should be a map, got %T", envelope.Data)
	return data
}

func TestListGames(t *testing.T) {
	h, game := setupGameHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/games?limit=5&offset=0", nil)
	w := httptest.NewRecorder()

	h.ListGames(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	data := decodeEnvelope(t, w)
	games := data["games"].([]interface{})
	assert.Len(t, games, 1)
	assert.Equal(t, game.Name, games[0].(map[string]interface{})["name"])
	assert.Equal(t, float64(1), data["total"])
}

func TestListGames_DefaultParams(t *testing.T) {
	h, _ := setupGameHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	w := httptest.NewRecorder()

	h.ListGames(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	decodeEnvelope(t, w) // just verify it succeeds
}

func TestGetGame(t *testing.T) {
	h, game := setupGameHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/"+game.ID.String(), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("gameID", game.ID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetGame(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var envelope response.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &envelope)
	require.NoError(t, err)
	require.True(t, envelope.Success)

	gameData, ok := envelope.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, game.Name, gameData["name"])
}

func TestGetGame_InvalidID(t *testing.T) {
	h, _ := setupGameHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/games/not-a-uuid", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("gameID", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetGame(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartGame(t *testing.T) {
	h, game := setupGameHandler(t)

	body := domain.PlaceBetRequest{GameID: game.ID}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/start", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
	req = req.WithContext(ctx)

	h.StartGame(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	data := decodeEnvelope(t, w)
	assert.NotEmpty(t, data["server_seed_hash"])
	assert.NotEmpty(t, data["session_id"])
}

func TestStartGame_MissingGameID(t *testing.T) {
	h, _ := setupGameHandler(t)

	body := domain.PlaceBetRequest{}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/start", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
	req = req.WithContext(ctx)

	h.StartGame(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartGame_InvalidBody(t *testing.T) {
	h, _ := setupGameHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/start", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
	req = req.WithContext(ctx)

	h.StartGame(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartGame_NonActiveGame(t *testing.T) {
	game := domain.Game{
		ID:     uuid.New(),
		Name:   "Closed Game",
		Status: domain.GameStatusInactive,
		RTP:    99,
	}
	repo := &stubGameRepo{game: game}
	svc := service.NewGameService(repo)
	h := NewGameHandler(svc)

	body := domain.PlaceBetRequest{GameID: game.ID}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/start", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
	req = req.WithContext(ctx)

	h.StartGame(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartGame_CryptoSeedHash(t *testing.T) {
	game := domain.Game{
		ID:          uuid.New(),
		Name:        "RNG Game",
		Description: "Crypto RNG",
		Category:    "random",
		MinBet:      100,
		MaxBet:      50000,
		RTP:         96,
		Status:      domain.GameStatusActive,
	}
	repo := &stubGameRepo{game: game}
	svc := service.NewGameService(repo)
	h := NewGameHandler(svc)

	body := domain.PlaceBetRequest{GameID: game.ID}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/games/start", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ctx := context.WithValue(req.Context(), middleware.UserIDKey, uuid.New())
	req = req.WithContext(ctx)

	h.StartGame(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	data := decodeEnvelope(t, w)
	hash, ok := data["server_seed_hash"].(string)
	require.True(t, ok, "server_seed_hash should be a string")
	assert.Len(t, hash, 64, "SHA-256 hex digest should be 64 chars")
}

func TestRNG_CryptoSecure(t *testing.T) {
	v1 := rng.SecureRandom()
	v2 := rng.SecureRandom()
	assert.NotEqual(t, v1, v2, "two SecureRandom() calls should return different values")
	assert.Greater(t, v1, 0.0)
	assert.Less(t, v1, 1.0)
	assert.Greater(t, v2, 0.0)
	assert.Less(t, v2, 1.0)
}

func TestRNG_Distribution(t *testing.T) {
	total := 10000
	underHalf := 0
	for i := 0; i < total; i++ {
		if rng.SecureRandom() < 0.5 {
			underHalf++
		}
	}
	ratio := float64(underHalf) / float64(total)
	assert.InDelta(t, 0.5, ratio, 0.05, "distribution should be roughly uniform")
}
