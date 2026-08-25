package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// BonusServicer is the interface that BonusHandler and AdminBonusHandler
// depend on. It captures the user-facing and admin methods of BonusService
// so the handlers are testable with fakes.
type BonusServicer interface {
	// User-facing methods
	ListAvailableBonuses(ctx context.Context, userID uuid.UUID) ([]domain.BonusDefinition, error)
	ClaimBonus(ctx context.Context, userID, defID uuid.UUID, params map[string]interface{}) (*domain.BonusClaimResult, error)
	GetActiveGrants(ctx context.Context, userID uuid.UUID) ([]domain.BonusActiveResponse, error)
	ForfeitBonus(ctx context.Context, userID, grantID uuid.UUID) error

	// Admin methods
	ListAllBonuses(ctx context.Context) ([]domain.BonusDefinition, error)
	CreateOrUpdateBonus(ctx context.Context, def *domain.BonusDefinition) error
	AdminListGrants(ctx context.Context, status string, userID *uuid.UUID, limit, offset int) ([]domain.BonusGrant, int, error)
	AdminExpireGrant(ctx context.Context, grantID uuid.UUID) error
}

// BonusHandler handles HTTP requests for user-facing bonus endpoints.
type BonusHandler struct {
	svc BonusServicer
}

// NewBonusHandler creates a new bonus handler.
func NewBonusHandler(svc BonusServicer) *BonusHandler {
	return &BonusHandler{svc: svc}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// claimBonusRequest is the request body for POST /bonuses/claim.
type claimBonusRequest struct {
	DefinitionID string                 `json:"definition_id"`
	Params       map[string]interface{} `json:"params,omitempty"`
}

// ---------------------------------------------------------------------------
// GET /bonuses/available — ListAvailable
// ---------------------------------------------------------------------------

// ListAvailable returns the bonus definitions the authenticated user can claim.
func (h *BonusHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	defs, err := h.svc.ListAvailableBonuses(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list bonuses")
		return
	}

	response.JSON(w, http.StatusOK, defs)
}

// ---------------------------------------------------------------------------
// POST /bonuses/claim — Claim
// ---------------------------------------------------------------------------

// Claim creates a bonus grant for the authenticated user.
func (h *BonusHandler) Claim(w http.ResponseWriter, r *http.Request) {
	var req claimBonusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.DefinitionID) == "" {
		response.Error(w, http.StatusBadRequest, "definition_id is required")
		return
	}

	defID, err := uuid.Parse(req.DefinitionID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid definition_id")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	result, err := h.svc.ClaimBonus(r.Context(), userID, defID, req.Params)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBonusNotFound):
			response.Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrActiveBonusExists):
			response.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrBonusNotActive):
			response.Error(w, http.StatusBadRequest, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to claim bonus")
		}
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

// ---------------------------------------------------------------------------
// GET /bonuses/active — ListActive
// ---------------------------------------------------------------------------

// ListActive returns the authenticated user's active bonus grants with progress.
func (h *BonusHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	grants, err := h.svc.GetActiveGrants(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list active grants")
		return
	}

	response.JSON(w, http.StatusOK, grants)
}

// ---------------------------------------------------------------------------
// POST /bonuses/forfeit/{grantID} — Forfeit
// ---------------------------------------------------------------------------

// Forfeit surrenders an active bonus grant.
func (h *BonusHandler) Forfeit(w http.ResponseWriter, r *http.Request) {
	grantIDStr := chi.URLParam(r, "grantID")
	grantID, err := uuid.Parse(grantIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid grant ID")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	if err := h.svc.ForfeitBonus(r.Context(), userID, grantID); err != nil {
		switch {
		case errors.Is(err, domain.ErrGrantNotFound):
			response.Error(w, http.StatusNotFound, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to forfeit bonus")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"success": true})
}
