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
	adminDefs       []domain.BonusDefinition
	adminGrants     []domain.BonusGrant
	adminTotal      int
	receivedParams  map[string]interface{}
	adminGrantCalls int
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

// fakeBonusService admin stubs for the grant/revoke endpoints. They live here
// (not in bonus_handler_test.go) so the base fake's file stays untouched while
// *fakeBonusService — shared by both the user and admin handler tests via
// embedding — still satisfies the extended BonusServicer interface.
func (f *fakeBonusService) AdminGrant(_ context.Context, _, _, _ uuid.UUID, _ map[string]interface{}) (*domain.BonusClaimResult, error) {
	if e, ok := f.errMap["AdminGrant"]; ok {
		return nil, e
	}
	return f.claim, f.err
}

// AdminGrant overrides the embedded *fakeBonusService method via method
// promotion, letting the admin handler tests capture exactly which params map
// the handler forwards to the service. Calls made through *fakeAdminBonusService
// resolve to this method, not the embedded one.
func (f *fakeAdminBonusService) AdminGrant(_ context.Context, _, _, _ uuid.UUID, params map[string]interface{}) (*domain.BonusClaimResult, error) {
	f.adminGrantCalls++
	f.receivedParams = params
	if e, ok := f.errMap["AdminGrant"]; ok {
		return nil, e
	}
	return f.claim, f.err
}

func (f *fakeBonusService) AdminRevoke(_ context.Context, _, _ uuid.UUID) error {
	if e, ok := f.errMap["AdminRevoke"]; ok {
		return e
	}
	return f.err
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
	r.Post("/admin/bonuses/grant", h.AdminGrant)
	r.Post("/admin/bonuses/revoke", h.AdminRevoke)
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

// ---------------------------------------------------------------------------
// Test: AdminGrant — admin → 201 (spec D4 S2)
// ---------------------------------------------------------------------------

func TestAdminGrant201(t *testing.T) {
	fake := newFakeAdminBonusService()
	grantID := uuid.New()
	fake.claim = &domain.BonusClaimResult{
		GrantID:               grantID,
		BonusAmountCents:      10000,
		WageringRequiredCents: 300000,
		ExpiresAt:             "2026-11-15T00:00:00Z",
	}
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminGrantRequest{UserID: uuid.New().String(), DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
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
// Test: AdminGrant — params forwarded to the service (R2-001)
// ---------------------------------------------------------------------------

func TestAdminGrantForwardsParamsToService(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := map[string]interface{}{
		"user_id":       uuid.New().String(),
		"definition_id": uuid.New().String(),
		"params":        map[string]interface{}{"deposit_cents": 5000},
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	// The handler must forward req.Params verbatim, not a literal nil.
	assert.Equal(t, 1, fake.adminGrantCalls, "service AdminGrant must be called exactly once")
	if assert.Contains(t, fake.receivedParams, "deposit_cents") {
		assert.Equal(t, float64(5000), fake.receivedParams["deposit_cents"])
	}
}

// ---------------------------------------------------------------------------
// Test: AdminGrant — validation failure → 400 (not 500)
// ---------------------------------------------------------------------------

func TestAdminGrant400NotClaimable(t *testing.T) {
	fake := newFakeAdminBonusService()
	fake.errMap["AdminGrant"] = domain.ErrBonusNotClaimable
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	// No params, mirroring a definition whose ValidateClaim rejects the request.
	body := adminGrantRequest{UserID: uuid.New().String(), DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: AdminGrant — non-admin → 403
// ---------------------------------------------------------------------------

func TestAdminGrant403NonAdmin(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminGrantRequest{UserID: uuid.New().String(), DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// Test: AdminGrant — duplicate active grant → 409
// ---------------------------------------------------------------------------

func TestAdminGrant409Duplicate(t *testing.T) {
	fake := newFakeAdminBonusService()
	fake.errMap["AdminGrant"] = domain.ErrActiveBonusExists
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminGrantRequest{UserID: uuid.New().String(), DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// ---------------------------------------------------------------------------
// Test: AdminGrant — missing / invalid fields → 400
// ---------------------------------------------------------------------------

func TestAdminGrant400MissingFields(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminGrantRequest{DefinitionID: uuid.New().String()} // missing UserID
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminGrant400InvalidUserID(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminGrantRequest{UserID: "not-a-uuid", DefinitionID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/grant", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------------------------------------------------------------------------
// Test: AdminRevoke — admin → 200 (spec D4 S3)
// ---------------------------------------------------------------------------

func TestAdminRevoke200(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminRevokeRequest{GrantID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/revoke", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

// ---------------------------------------------------------------------------
// Test: AdminRevoke — non-admin → 403
// ---------------------------------------------------------------------------

func TestAdminRevoke403NonAdmin(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminRevokeRequest{GrantID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/revoke", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "user"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// ---------------------------------------------------------------------------
// Test: AdminRevoke — grant not found → 404
// ---------------------------------------------------------------------------

func TestAdminRevoke404NotFound(t *testing.T) {
	fake := newFakeAdminBonusService()
	fake.errMap["AdminRevoke"] = domain.ErrGrantNotFound
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminRevokeRequest{GrantID: uuid.New().String()}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/revoke", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// Test: AdminRevoke — missing / invalid grant_id → 400
// ---------------------------------------------------------------------------

func TestAdminRevoke400InvalidID(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminRevokeRequest{GrantID: "not-a-uuid"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/revoke", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminRevoke400MissingGrantID(t *testing.T) {
	fake := newFakeAdminBonusService()
	h := NewAdminBonusHandler(fake)
	router := adminBonusRouter(h)

	body := adminRevokeRequest{GrantID: ""}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/admin/bonuses/revoke", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx(uuid.New(), "admin"))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
