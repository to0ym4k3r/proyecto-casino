package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"casino-backend/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Admin fake — extends fakeBonusService with admin methods
// ---------------------------------------------------------------------------

type fakeAdminBonusService struct {
	fakeBonusService
	adminDefs   []domain.BonusDefinition
	adminGrants []domain.BonusGrant
	adminTotal  int
}

func newFakeAdminBonusService() *fakeAdminBonusService {
	return &fakeAdminBonusService{
		fakeBonusService: *newFakeBonusService(),
	}
}

func (f *fakeAdminBonusService) ListAllBonuses(_ context.Context) ([]domain.BonusDefinition, error) {
	if e, ok := f.errMap["ListAllBonuses"]; ok {
		return nil, e
	}
	return f.adminDefs, nil
}

func (f *fakeAdminBonusService) CreateOrUpdateBonus(_ context.Context, def *domain.BonusDefinition) error {
	if e, ok := f.errMap["CreateOrUpdateBonus"]; ok {
		return e
	}
	if def.ID == uuid.Nil {
		def.ID = uuid.New()
	}
	if def.CreatedAt.IsZero() {
		def.CreatedAt = time.Now()
	}
	def.UpdatedAt = time.Now()
	return nil
}

func (f *fakeAdminBonusService) AdminListGrants(_ context.Context, _ string, _ *uuid.UUID, _, _ int) ([]domain.BonusGrant, int, error) {
	if e, ok := f.errMap["AdminListGrants"]; ok {
		return nil, 0, e
	}
	return f.adminGrants, f.adminTotal, nil
}

func (f *fakeAdminBonusService) AdminExpireGrant(_ context.Context, grantID uuid.UUID) error {
	if e, ok := f.errMap["AdminExpireGrant"]; ok {
		return e
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func adminBonusRouter(h *AdminBonusHandler) chi.Router {
	r := chi.NewRouter()
	r.Get("/admin/bonuses", h.ListDefinitions)
	r.Post("/admin/bonuses", h.CreateDefinition)
	r.Put("/admin/bonuses/{id}", h.UpdateDefinition)
	r.Get("/admin/bonuses/grants", h.ListGrants)
	r.Post("/admin/bonuses/{id}/expire", h.ExpireGrant)
	return r
}

// ---------------------------------------------------------------------------
// Test: ListDefinitions — admin → 200
// ---------------------------------------------------------------------------

func TestAdminListDefinitions200(t *testing.T) {
	fake := newFakeAdminBonusService()
	fake.adminDefs = []domain.BonusDefinition{
		{ID: uuid.New(), Code: "welcome", Name: "Welcome Bonus", IsActive: true},
		{ID: uuid.New(), Code: "reload", Name: "Reload Bonus", IsActive: false},
	}
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/admin/bonuses", nil)
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
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
	// Should include both active and inactive
	assert.Equal(t, "welcome", resp.Data[0].Code)
	assert.Equal(t, "reload", resp.Data[1].Code)
}

// ---------------------------------------------------------------------------
// Test: ListDefinitions — non-admin → 403
// ---------------------------------------------------------------------------

func TestAdminListDefinitions403(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/admin/bonuses", nil)
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// Test: CreateDefinition — admin → 201
// ---------------------------------------------------------------------------

func TestAdminCreateDefinition201(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminCreateDefRequest{
		Code:                "vip",
		Name:                "VIP Bonus",
		Description:         "Exclusive for VIPs",
		WageringMultiplier:  20,
		ExpiryDays:          30,
		MaxBonusAmountCents: 100000,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		Success bool
		Data    domain.BonusDefinition
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "vip", resp.Data.Code)
	assert.Equal(t, "VIP Bonus", resp.Data.Name)
}

// ---------------------------------------------------------------------------
// Test: CreateDefinition — non-admin → 403
// ---------------------------------------------------------------------------

func TestAdminCreateDefinition403(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminCreateDefRequest{Code: "vip", Name: "VIP Bonus"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// Test: CreateDefinition — missing required fields → 400
// ---------------------------------------------------------------------------

func TestAdminCreateDefinition400MissingFields(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminCreateDefRequest{Name: "VIP Bonus"} // missing Code
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: UpdateDefinition — admin → 200
// ---------------------------------------------------------------------------

func TestAdminUpdateDefinition200(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	defID := uuid.New().String()
	body := adminCreateDefRequest{
		Code:                "welcome",
		Name:                "Welcome Bonus Updated",
		WageringMultiplier:  35,
		ExpiryDays:          30,
		MaxBonusAmountCents: 75000,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/admin/bonuses/"+defID, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", defID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
		Data    domain.BonusDefinition
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, "welcome", resp.Data.Code)
}

// ---------------------------------------------------------------------------
// Test: UpdateDefinition — invalid ID → 400
// ---------------------------------------------------------------------------

func TestAdminUpdateDefinition400InvalidID(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminCreateDefRequest{Code: "welcome", Name: "Updated"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/admin/bonuses/not-a-uuid", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ListGrants — admin with filters → 200
// ---------------------------------------------------------------------------

func TestAdminListGrants200(t *testing.T) {
	fake := newFakeAdminBonusService()
	userID := uuid.New()
	fake.adminGrants = []domain.BonusGrant{
		{
			ID:                uuid.New(),
			UserID:            userID,
			BonusDefinitionID: uuid.New(),
			Status:            "active",
			BonusAmountCents:  10000,
		},
	}
	fake.adminTotal = 1
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/admin/bonuses/grants?status=active&limit=10&offset=0", nil)
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
		Data    struct {
			Grants []domain.BonusGrant `json:"grants"`
			Total  int                 `json:"total"`
		}
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data.Grants, 1)
	assert.Equal(t, 1, resp.Data.Total)
}

// ---------------------------------------------------------------------------
// Test: ListGrants — non-admin → 403
// ---------------------------------------------------------------------------

func TestAdminListGrants403(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/admin/bonuses/grants", nil)
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ExpireGrant — admin → 200
// ---------------------------------------------------------------------------

func TestAdminExpireGrant200(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	grantID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/"+grantID+"/expire", nil)
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grantID)
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
// Test: ExpireGrant — non-admin → 403
// ---------------------------------------------------------------------------

func TestAdminExpireGrant403(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	grantID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/"+grantID+"/expire", nil)
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grantID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ExpireGrant — invalid ID → 400
// ---------------------------------------------------------------------------

func TestAdminExpireGrant400InvalidID(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/not-a-uuid/expire", nil)
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: ExpireGrant — grant not found → 404
// ---------------------------------------------------------------------------

func TestAdminExpireGrant404NotFound(t *testing.T) {
	fake := newFakeAdminBonusService()
	fake.errMap["AdminExpireGrant"] = domain.ErrGrantNotFound
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	grantID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/"+grantID+"/expire", nil)
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", grantID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
