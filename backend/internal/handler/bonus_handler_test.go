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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Fake BonusService for handler tests
// ---------------------------------------------------------------------------

type fakeBonusService struct {
	defs   []domain.BonusDefinition
	grants []domain.BonusActiveResponse
	claim  *domain.BonusClaimResult
	err    error
	errMap map[string]error // keyed by method name for fine-grained control
}

func newFakeBonusService() *fakeBonusService {
	return &fakeBonusService{errMap: make(map[string]error)}
}

func (f *fakeBonusService) ListAvailableBonuses(_ context.Context, _ uuid.UUID) ([]domain.BonusDefinition, error) {
	if e, ok := f.errMap["ListAvailableBonuses"]; ok {
		return nil, e
	}
	return f.defs, f.err
}

func (f *fakeBonusService) ClaimBonus(_ context.Context, _, _ uuid.UUID, _ map[string]interface{}) (*domain.BonusClaimResult, error) {
	if e, ok := f.errMap["ClaimBonus"]; ok {
		return nil, e
	}
	return f.claim, f.err
}

func (f *fakeBonusService) GetActiveGrants(_ context.Context, _ uuid.UUID) ([]domain.BonusActiveResponse, error) {
	if e, ok := f.errMap["GetActiveGrants"]; ok {
		return nil, e
	}
	return f.grants, f.err
}

func (f *fakeBonusService) ForfeitBonus(_ context.Context, _, _ uuid.UUID) error {
	if e, ok := f.errMap["ForfeitBonus"]; ok {
		return e
	}
	return f.err
}

// Admin stubs — satisfy BonusServicer interface for user handler tests.
func (f *fakeBonusService) ListAllBonuses(_ context.Context) ([]domain.BonusDefinition, error) {
	return f.defs, f.err
}
func (f *fakeBonusService) CreateOrUpdateBonus(_ context.Context, _ *domain.BonusDefinition) error {
	return f.err
}
func (f *fakeBonusService) AdminListGrants(_ context.Context, _ string, _ *uuid.UUID, _, _ int) ([]domain.BonusGrant, int, error) {
	return nil, 0, f.err
}
func (f *fakeBonusService) AdminExpireGrant(_ context.Context, _ uuid.UUID) error {
	return f.err
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func userCtx(userID uuid.UUID) context.Context {
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	return ctx
}

func bonusRouter(h *BonusHandler) chi.Router {
	r := chi.NewRouter()
	r.Get("/bonuses/available", h.ListAvailable)
	r.Post("/bonuses/claim", h.Claim)
	r.Get("/bonuses/active", h.ListActive)
	r.Post("/bonuses/forfeit/{grantID}", h.Forfeit)
	return r
}

// ---------------------------------------------------------------------------
// Test: ListAvailableBonuses — happy path
// ---------------------------------------------------------------------------

func TestListAvailableBonuses200(t *testing.T) {
	fake := newFakeBonusService()
	fake.defs = []domain.BonusDefinition{
		{
			ID:                  uuid.New(),
			Code:                "welcome",
			Name:                "Welcome Bonus",
			MaxBonusAmountCents: 50000,
			IsActive:            true,
		},
		{
			ID:                  uuid.New(),
			Code:                "reload",
			Name:                "Reload Bonus",
			MaxBonusAmountCents: 25000,
			IsActive:            true,
		},
	}
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/bonuses/available", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
		Data    []domain.BonusDefinition
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, "welcome", resp.Data[0].Code)
}

// ---------------------------------------------------------------------------
// Test: ListAvailableBonuses — empty list
// ---------------------------------------------------------------------------

func TestListAvailableBonuses200Empty(t *testing.T) {
	fake := newFakeBonusService()
	fake.defs = []domain.BonusDefinition{}
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/bonuses/available", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
		Data    []domain.BonusDefinition
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Data)
}

// ---------------------------------------------------------------------------
// Test: ListAvailableBonuses — service error → 500
// ---------------------------------------------------------------------------

func TestListAvailableBonuses500(t *testing.T) {
	fake := newFakeBonusService()
	fake.errMap["ListAvailableBonuses"] = assert.AnError
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/bonuses/available", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ClaimBonus — success → 201
// ---------------------------------------------------------------------------

func TestClaimBonus201(t *testing.T) {
	fake := newFakeBonusService()
	grantID := uuid.New()
	fake.claim = &domain.BonusClaimResult{
		GrantID:               grantID,
		BonusAmountCents:      10000,
		WageringRequiredCents: 300000,
		ExpiresAt:             "2026-09-23T00:00:00Z",
	}
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	body := claimBonusRequest{DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bonuses/claim", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		Success bool
		Data    domain.BonusClaimResult
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, grantID, resp.Data.GrantID)
	assert.Equal(t, int64(10000), resp.Data.BonusAmountCents)
}

// ---------------------------------------------------------------------------
// Test: ClaimBonus — invalid body → 400
// ---------------------------------------------------------------------------

func TestClaimBonus400InvalidBody(t *testing.T) {
	fake := newFakeBonusService()
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/bonuses/claim", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ClaimBonus — missing definition_id → 400
// ---------------------------------------------------------------------------

func TestClaimBonus400MissingDefID(t *testing.T) {
	fake := newFakeBonusService()
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	body := claimBonusRequest{DefinitionID: ""}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bonuses/claim", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ClaimBonus — definition not found → 404
// ---------------------------------------------------------------------------

func TestClaimBonus404NotFound(t *testing.T) {
	fake := newFakeBonusService()
	fake.errMap["ClaimBonus"] = domain.ErrBonusNotFound
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	body := claimBonusRequest{DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bonuses/claim", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ClaimBonus — active bonus already exists → 409
// ---------------------------------------------------------------------------

func TestClaimBonus409Conflict(t *testing.T) {
	fake := newFakeBonusService()
	fake.errMap["ClaimBonus"] = domain.ErrActiveBonusExists
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	body := claimBonusRequest{DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bonuses/claim", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ListActiveGrants — happy path
// ---------------------------------------------------------------------------

func TestListActiveGrants200(t *testing.T) {
	fake := newFakeBonusService()
	fake.grants = []domain.BonusActiveResponse{
		{
			GrantID:               uuid.New(),
			Code:                  "welcome",
			Name:                  "Welcome Bonus",
			BonusAmountCents:      10000,
			WageringRequiredCents: 300000,
			TotalWageredCents:     90000,
			WageringProgressPct:   30.0,
			ExpiresAt:             "2026-09-23T00:00:00Z",
		},
	}
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/bonuses/active", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
		Data    []domain.BonusActiveResponse
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "welcome", resp.Data[0].Code)
	assert.Equal(t, 30.0, resp.Data[0].WageringProgressPct)
}

// ---------------------------------------------------------------------------
// Test: ListActiveGrants — empty
// ---------------------------------------------------------------------------

func TestListActiveGrants200Empty(t *testing.T) {
	fake := newFakeBonusService()
	fake.grants = []domain.BonusActiveResponse{}
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/bonuses/active", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
		Data    []domain.BonusActiveResponse
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Data)
}

// ---------------------------------------------------------------------------
// Test: ListActiveGrants — service error → 500
// ---------------------------------------------------------------------------

func TestListActiveGrants500(t *testing.T) {
	fake := newFakeBonusService()
	fake.errMap["GetActiveGrants"] = assert.AnError
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/bonuses/active", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ForfeitBonus — success → 200
// ---------------------------------------------------------------------------

func TestForfeitBonus200(t *testing.T) {
	fake := newFakeBonusService()
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	grantID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/bonuses/forfeit/"+grantID, nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	// chi needs RouteContext for URL params
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("grantID", grantID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

// ---------------------------------------------------------------------------
// Test: ForfeitBonus — grant not found → 404
// ---------------------------------------------------------------------------

func TestForfeitBonus404NotFound(t *testing.T) {
	fake := newFakeBonusService()
	fake.errMap["ForfeitBonus"] = domain.ErrGrantNotFound
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	grantID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/bonuses/forfeit/"+grantID, nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("grantID", grantID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ForfeitBonus — invalid grant ID → 400
// ---------------------------------------------------------------------------

func TestForfeitBonus400InvalidID(t *testing.T) {
	fake := newFakeBonusService()
	h := NewBonusHandler(fake)
	router := bonusRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/bonuses/forfeit/not-a-uuid", nil)
	req = req.WithContext(userCtx(uuid.New()))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("grantID", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
