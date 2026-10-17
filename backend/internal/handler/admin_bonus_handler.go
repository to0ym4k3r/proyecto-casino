package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AdminBonusHandler handles HTTP requests for admin bonus management endpoints.
type AdminBonusHandler struct {
	svc BonusServicer
}

// NewAdminBonusHandler creates a new admin bonus handler.
func NewAdminBonusHandler(svc BonusServicer) *AdminBonusHandler {
	return &AdminBonusHandler{svc: svc}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// adminCreateDefRequest is the request body for the create and update bonus
// definition endpoints.
type adminCreateDefRequest struct {
	Code                string                 `json:"code"`
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	WageringMultiplier  int                    `json:"wagering_multiplier"`
	ExpiryDays          int                    `json:"expiry_days"`
	MaxBonusAmountCents int64                  `json:"max_bonus_amount_cents"`
	MinDepositCents     int64                  `json:"min_deposit_cents"`
	IsActive            bool                   `json:"is_active"`
	Config              map[string]interface{} `json:"config"`
}

// adminGrantRequest is the request body for the manual bonus grant endpoint.
type adminGrantRequest struct {
	UserID       string                 `json:"user_id"`
	DefinitionID string                 `json:"definition_id"`
	Params       map[string]interface{} `json:"params,omitempty"`
}

// adminRevokeRequest is the request body for the manual grant revoke endpoint.
type adminRevokeRequest struct {
	GrantID string `json:"grant_id"`
}

// ---------------------------------------------------------------------------
// ListDefinitions — returns all bonus definitions (including inactive).
// ---------------------------------------------------------------------------

// ListDefinitions returns all bonus definitions (including inactive) for admin.
func (h *AdminBonusHandler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	defs, err := h.svc.ListAllBonuses(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list bonus definitions")
		return
	}

	response.JSON(w, http.StatusOK, defs)
}

// ---------------------------------------------------------------------------
// CreateDefinition — creates a new bonus definition.
// ---------------------------------------------------------------------------

// CreateDefinition creates a new bonus definition.
func (h *AdminBonusHandler) CreateDefinition(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req adminCreateDefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		response.Error(w, http.StatusBadRequest, "code and name are required")
		return
	}

	def := &domain.BonusDefinition{
		Code:                req.Code,
		Name:                req.Name,
		Description:         req.Description,
		WageringMultiplier:  req.WageringMultiplier,
		ExpiryDays:          req.ExpiryDays,
		MaxBonusAmountCents: req.MaxBonusAmountCents,
		MinDepositCents:     req.MinDepositCents,
		IsActive:            req.IsActive,
		Config:              req.Config,
	}

	if err := h.svc.CreateOrUpdateBonus(r.Context(), def); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create bonus definition")
		return
	}

	response.JSON(w, http.StatusCreated, def)
}

// ---------------------------------------------------------------------------
// UpdateDefinition — updates an existing bonus definition.
// ---------------------------------------------------------------------------

// UpdateDefinition updates an existing bonus definition.
func (h *AdminBonusHandler) UpdateDefinition(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid definition ID")
		return
	}

	var req adminCreateDefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
		response.Error(w, http.StatusBadRequest, "code and name are required")
		return
	}

	def := &domain.BonusDefinition{
		ID:                  id,
		Code:                req.Code,
		Name:                req.Name,
		Description:         req.Description,
		WageringMultiplier:  req.WageringMultiplier,
		ExpiryDays:          req.ExpiryDays,
		MaxBonusAmountCents: req.MaxBonusAmountCents,
		MinDepositCents:     req.MinDepositCents,
		IsActive:            req.IsActive,
		Config:              req.Config,
	}

	if err := h.svc.CreateOrUpdateBonus(r.Context(), def); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update bonus definition")
		return
	}

	response.JSON(w, http.StatusOK, def)
}

// ---------------------------------------------------------------------------
// ListGrants — returns grants with optional filters.
// ---------------------------------------------------------------------------

// ListGrants returns grants with optional filters: status, user_id, limit, offset.
func (h *AdminBonusHandler) ListGrants(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	status := r.URL.Query().Get("status")

	var userID *uuid.UUID
	if uidStr := r.URL.Query().Get("user_id"); uidStr != "" {
		uid, err := uuid.Parse(uidStr)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		userID = &uid
	}

	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	grants, total, err := h.svc.AdminListGrants(r.Context(), status, userID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list grants")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"grants": grants,
		"total":  total,
	})
}

// ---------------------------------------------------------------------------
// ExpireGrant — manually expires a specific grant.
// ---------------------------------------------------------------------------

// ExpireGrant manually expires a specific grant (sets status to expired,
// zeros bonus_balance, writes ledger entry).
func (h *AdminBonusHandler) ExpireGrant(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	grantID, err := uuid.Parse(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid grant ID")
		return
	}

	if err := h.svc.AdminExpireGrant(r.Context(), grantID); err != nil {
		switch {
		case err == domain.ErrGrantNotFound:
			response.Error(w, http.StatusNotFound, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to expire grant")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ---------------------------------------------------------------------------
// AdminGrant — manually grants a bonus to a user (spec D4 S2).
// ---------------------------------------------------------------------------

// AdminGrant manually grants a bonus definition to a target user. The acting
// admin is taken from the JWT context; the grant is stamped with
// granted_by='admin' by the service. Returns 201 with the created grant terms.
func (h *AdminBonusHandler) AdminGrant(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req adminGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.DefinitionID) == "" {
		response.Error(w, http.StatusBadRequest, "user_id and definition_id are required")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	defID, err := uuid.Parse(req.DefinitionID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid definition_id")
		return
	}

	adminID := middleware.UserIDFromContext(r.Context())

	result, err := h.svc.AdminGrant(r.Context(), adminID, userID, defID, req.Params)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBonusNotFound):
			response.Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrActiveBonusExists):
			response.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, domain.ErrBonusNotActive):
			response.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, domain.ErrBonusNotClaimable):
			response.Error(w, http.StatusBadRequest, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to grant bonus")
		}
		return
	}

	response.JSON(w, http.StatusCreated, result)
}

// ---------------------------------------------------------------------------
// AdminRevoke — revokes (forfeits) a grant (spec D4 S3).
// ---------------------------------------------------------------------------

// AdminRevoke forfeits a single active grant. The grant need not belong to the
// calling admin (admins may revoke any user's grant). Returns 200 on success.
func (h *AdminBonusHandler) AdminRevoke(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req adminRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.GrantID) == "" {
		response.Error(w, http.StatusBadRequest, "grant_id is required")
		return
	}

	grantID, err := uuid.Parse(req.GrantID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid grant_id")
		return
	}

	adminID := middleware.UserIDFromContext(r.Context())

	if err := h.svc.AdminRevoke(r.Context(), adminID, grantID); err != nil {
		switch {
		case errors.Is(err, domain.ErrGrantNotFound):
			response.Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrBonusNotActive):
			response.Error(w, http.StatusBadRequest, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to revoke grant")
		}
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"success": true})
}
