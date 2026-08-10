package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"casino-backend/internal/domain"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"
)

// maxAuthBodyBytes caps the request body for all auth endpoints so oversized
// payloads are rejected before JSON decoding (finding RR-02).
const maxAuthBodyBytes = 1 << 20

// AuthHandler handles HTTP requests for authentication.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register handles user registration via POST /api/v1/auth/register.
//
// It decodes a domain.RegisterRequest from the request body and validates that
// all required fields are present and the password is at least 8 characters.
// The service atomically creates the user, wallet, single-use email
// verification record and initial refresh token, then dispatches the
// verification email; a dispatch failure rolls the account back (503). The player is NOT auto-authenticated —
// they must verify their email before the first login.
//
// Responses: 201 with the created user (no tokens), 400 on invalid/underage
// date of birth, 409 on duplicate email or username.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.RegisterRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" || req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		response.Error(w, http.StatusBadRequest, "all fields are required")
		return
	}

	if len(req.Password) < 8 || len(req.Password) > 72 {
		response.Error(w, http.StatusBadRequest, "password must be between 8 and 72 characters")
		return
	}

	user, err := h.authService.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserAlreadyExists):
			response.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Error(w, http.StatusBadRequest, "invalid email or password")
		case errors.Is(err, service.ErrVerificationEmailFailed):
			w.Header().Set("Retry-After", "60")
			response.Error(w, http.StatusServiceUnavailable, "registration failed, please retry")
		case errors.Is(err, service.ErrInvalidDOB), errors.Is(err, service.ErrUnderage):
			response.Error(w, http.StatusBadRequest, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}

	response.JSON(w, http.StatusCreated, user)
}

// VerifyEmail handles email verification via POST /api/v1/auth/verify.
//
// It consumes the single-use token dispatched at registration. The token is
// URL-safe base64url; it arrives in the JSON body (the emailed link embeds it
// as a query parameter pointing at this same route, but the endpoint only
// reads the body).
//
// Responses: 200 on success, 400 when the token is missing, unknown, expired
// or already consumed.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req domain.VerifyEmailRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		response.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	if err := h.authService.VerifyEmail(r.Context(), req.Token); err != nil {
		if errors.Is(err, service.ErrInvalidVerificationToken) {
			response.Error(w, http.StatusBadRequest, "invalid or expired verification token")
			return
		}
		response.Error(w, http.StatusInternalServerError, "verification failed")
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"verified": true})
}

// Login handles user login via POST /api/v1/auth/login.
//
// It decodes a domain.LoginRequest (email + password) from the request body.
// Unverified accounts are rejected with 403 before any token is issued.
//
// Responses: 200 with a JWT token pair, 401 on invalid credentials, 403 when
// the email address has not been verified yet.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "email and password are required")
		return
	}

	tokens, err := h.authService.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			response.Error(w, http.StatusUnauthorized, "invalid credentials")
		case errors.Is(err, service.ErrEmailNotVerified):
			response.Error(w, http.StatusForbidden, "email verification required")
		default:
			response.Error(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

// Refresh handles JWT token refresh via POST /api/v1/auth/refresh.
//
// The presented refresh token is validated (signature, expiry, type claim,
// persisted hash) and rotated: the old token is invalidated and replaced by a
// fresh pair. A replayed or revoked token fails closed with 401 and revokes
// the whole token family.
//
// Response (200): TokenPair { access_token, refresh_token }.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		response.Error(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	tokens, err := h.authService.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRefreshToken) {
			response.Error(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		response.Error(w, http.StatusInternalServerError, "refresh failed")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

// Logout handles session termination via POST /api/v1/auth/logout.
//
// It revokes the refresh token family the presented token belongs to, so all
// outstanding refresh tokens in that chain stop working immediately.
//
// Responses: 204 No Content on success (idempotent, empty body), 401 when the
// token is unknown or already revoked.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		response.Error(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		if errors.Is(err, service.ErrInvalidRefreshToken) {
			response.Error(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		response.Error(w, http.StatusInternalServerError, "logout failed")
		return
	}

	// 204 No Content with an empty body (spec-freeze): the response envelope
	// helper cannot emit a bodyless 204, so the header is written directly —
	// same convention as middleware/cors.go OPTIONS handling.
	w.WriteHeader(http.StatusNoContent)
}
