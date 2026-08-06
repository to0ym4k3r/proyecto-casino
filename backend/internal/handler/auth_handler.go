package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"casino-backend/internal/domain"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"
)

// AuthHandler handles HTTP requests for authentication.
type AuthHandler struct {
	authService   *service.AuthService
	walletService *service.WalletService
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(authService *service.AuthService, walletService *service.WalletService) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		walletService: walletService,
	}
}

// Register handles user registration via POST /api/v1/auth/register.
//
// It decodes a domain.RegisterRequest from the request body, validates that all
// required fields (username, email, password, first_name, last_name) are present
// and that the password is at least 8 characters. On success it:
//  1. Creates the user in the repository (returns 409 if email or username already exists).
//  2. Creates a zero-balance wallet for the new user (wallet creation errors are non-fatal).
//  3. Issues a JWT token pair (access + refresh) so the user is immediately authenticated.
//
// Response (201): { "user": User, "tokens": TokenPair }
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req domain.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		response.Error(w, http.StatusBadRequest, "all fields are required")
		return
	}

	if len(req.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	user, err := h.authService.Register(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			response.Error(w, http.StatusConflict, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "registration failed")
		return
	}

	_, _ = h.walletService.CreateWallet(r.Context(), user.ID)

	tokens, err := h.authService.Login(r.Context(), domain.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"user":   user,
		"tokens": tokens,
	})
}

// Login handles user login via POST /api/v1/auth/login.
//
// It decodes a domain.LoginRequest (email + password) from the request body.
// Returns 401 if the credentials are invalid, or 200 with a JWT token pair on success.
//
// Response (200): TokenPair { access_token, refresh_token }
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
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
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Error(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		response.Error(w, http.StatusInternalServerError, "login failed")
		return
	}

	response.JSON(w, http.StatusOK, tokens)
}

// Refresh handles JWT token refresh via POST /api/v1/auth/refresh.
//
// It decodes a domain.RefreshRequest containing a refresh token. The token is
// validated (signature, expiry, and type claim must be "refresh"). On success a
// new token pair is returned. Returns 401 if the refresh token is invalid or expired.
//
// Response (200): TokenPair { access_token, refresh_token }
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshRequest
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
