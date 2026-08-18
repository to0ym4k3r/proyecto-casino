package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// adminGameRepo is a simple in-memory game repo for admin handler tests.
type adminGameRepo struct {
	games map[uuid.UUID]*domain.Game
}

func newAdminGameRepo() *adminGameRepo {
	return &adminGameRepo{games: make(map[uuid.UUID]*domain.Game)}
}

func (r *adminGameRepo) List(_ context.Context, _, _ int) ([]domain.Game, int, error) {
	result := make([]domain.Game, 0, len(r.games))
	for _, g := range r.games {
		result = append(result, *g)
	}
	return result, len(r.games), nil
}
func (r *adminGameRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, fmt.Errorf("game not found")
	}
	return g, nil
}
func (r *adminGameRepo) Create(_ context.Context, g *domain.Game) error {
	r.games[g.ID] = g
	return nil
}
func (r *adminGameRepo) Update(_ context.Context, g *domain.Game) error {
	r.games[g.ID] = g
	return nil
}
func (r *adminGameRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.games[id]; !ok {
		return fmt.Errorf("game not found")
	}
	delete(r.games, id)
	return nil
}

func setupAdminHandler(t *testing.T) (*AdminHandler, *adminGameRepo) {
	t.Helper()
	repo := newAdminGameRepo()
	svc := service.NewGameService(repo)
	h := NewAdminHandler(svc)
	return h, repo
}

func adminCtx(userID uuid.UUID, role string) context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.RoleKey, role)
	return ctx
}

func TestCreateGameAdmin201(t *testing.T) {
	h, repo := setupAdminHandler(t)

	body := CreateGameRequest{
		Name:        "Roulette",
		Description: "Spin the wheel",
		Category:    "table",
		MinBet:      50,
		MaxBet:      5000,
		RTP:         97,
		CoverImage:  "roulette.png",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/" + "admin" + "/games", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	h.CreateGameHandler(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Len(t, repo.games, 1)
}

func TestCreateGameNonAdmin403(t *testing.T) {
	h, _ := setupAdminHandler(t)

	body := CreateGameRequest{Name: "Roulette", Category: "table", MinBet: 50, MaxBet: 5000, RTP: 97}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/" + "admin" + "/games", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	h.CreateGameHandler(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteGameNonExistent404(t *testing.T) {
	h, _ := setupAdminHandler(t)

	fakeID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/" + "admin" + "/games/"+fakeID.String(), nil)
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("gameID", fakeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.DeleteGameHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteGameNonAdmin403(t *testing.T) {
	h, _ := setupAdminHandler(t)

	fakeID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/" + "admin" + "/games/"+fakeID.String(), nil)
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("gameID", fakeID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.DeleteGameHandler(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateGameAdmin200(t *testing.T) {
	h, repo := setupAdminHandler(t)

	gameID := uuid.New()
	existing := &domain.Game{
		ID:     gameID,
		Name:   "Poker",
		Status: domain.GameStatusActive,
		RTP:    95,
	}
	repo.games[gameID] = existing

	body := CreateGameRequest{
		Name:        "Texas Hold'em",
		Description: "Professional poker",
		Category:    "card",
		MinBet:      200,
		MaxBet:      20000,
		RTP:         97,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/" + "admin" + "/games/"+gameID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("gameID", gameID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.UpdateGameHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Texas Hold'em", repo.games[gameID].Name)
	assert.Equal(t, 97, repo.games[gameID].RTP)
}
