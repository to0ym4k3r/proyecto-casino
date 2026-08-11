package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"

	"github.com/google/uuid"
)

// maxUserBodyBytes caps the request body for profile endpoints so oversized
// payloads are rejected before JSON decoding.
const maxUserBodyBytes = 1 << 20

// UserHandler handles the authenticated user's own profile endpoints.
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler creates a new profile handler.
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// GetMe handles GET /users/me.
//
// Auth required: the middleware stores the user ID under the single typed
// context key and the handler reads it via middleware.UserIDFromContext —
// never a string key. On success the profile is returned inside the standard
// {success,data,error} envelope; the password hash is never serialized
// (json:"-" on domain.User.PasswordHash).
//
// Responses: 200 with the profile, 401 without a valid token.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrProfileNotFound) {
			response.Error(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to load profile")
		return
	}

	response.JSON(w, http.StatusOK, user)
}

// UpdateMe handles PUT /users/me.
//
// Only username, first_name and last_name are mutable (design D6); email is
// immutable in this change, so any payload carrying an "email" key is rejected
// with 400 before the repository is touched — a rejected update leaves the
// profile unchanged.
//
// Responses: 200 with the updated profile, 400 on invalid payload or fields,
// 401 without a valid token.
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUserBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Decode into a raw map first so a forbidden "email" key is detectable:
	// the typed request type deliberately has no Email field.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if _, ok := raw["email"]; ok {
		response.Error(w, http.StatusBadRequest, "email cannot be updated")
		return
	}

	var req domain.UpdateProfileRequest
	if err := json.Unmarshal(body, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.userService.UpdateProfile(r.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUsername), errors.Is(err, service.ErrInvalidProfileName):
			response.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrProfileNotFound):
			response.Error(w, http.StatusUnauthorized, "invalid or expired token")
		default:
			response.Error(w, http.StatusInternalServerError, "failed to update profile")
		}
		return
	}

	response.JSON(w, http.StatusOK, user)
}
