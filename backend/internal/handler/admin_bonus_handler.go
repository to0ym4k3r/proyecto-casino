package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"casino-backend/internal/domain"
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
