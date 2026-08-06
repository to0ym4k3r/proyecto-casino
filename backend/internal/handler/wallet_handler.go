package handler

import (
	"encoding/json"
	"net/http"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"
)

// WalletHandler handles HTTP requests for wallet operations.
type WalletHandler struct {
	walletService *service.WalletService
}

// NewWalletHandler creates a new wallet handler.
func NewWalletHandler(walletService *service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: walletService}
}

// GetBalance returns the authenticated user's wallet via GET /api/v1/wallet/balance.
//
// Requires a valid JWT. The user ID is extracted from the token claims.
// Returns 404 if no wallet exists for the user.
//
// Response (200): Wallet { id, user_id, balance, currency, version, ... }
func (h *WalletHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	wallet, err := h.walletService.GetBalance(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "wallet not found")
		return
	}

	response.JSON(w, http.StatusOK, wallet)
}

// Deposit adds funds to the authenticated user's wallet via POST /api/v1/wallet/deposit.
//
// Requires a valid JWT. Decodes a domain.DepositRequest; the amount must be > 0.
// The deposit is recorded as a transaction with optimistic-lock versioning.
// Returns 400 if the amount is invalid or the wallet cannot be updated.
//
// Response (200): Wallet with updated balance
func (h *WalletHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	var req domain.DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	wallet, err := h.walletService.Deposit(r.Context(), userID, req.Amount)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, wallet)
}

// Withdraw removes funds from the authenticated user's wallet via POST /api/v1/wallet/withdraw.
//
// Requires a valid JWT. Decodes a domain.WithdrawRequest; the amount must be > 0.
// Returns 400 if the amount is invalid, the balance is insufficient, or the
// wallet update fails (e.g. version conflict).
//
// Response (200): Wallet with updated balance
func (h *WalletHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	var req domain.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	wallet, err := h.walletService.Withdraw(r.Context(), userID, req.Amount)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, wallet)
}
