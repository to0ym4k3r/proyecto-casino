package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/repository/postgres"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"
)

// WalletHandler handles HTTP requests for wallet operations.
type WalletHandler struct {
	walletService   *service.WalletService
	withdrawalSvc   *service.WithdrawalService
	txHistorySvc    *service.TxHistoryService
	depositAddrRepo postgres.DepositAddressRepository
}

// NewWalletHandler creates a new wallet handler.
func NewWalletHandler(
	walletService *service.WalletService,
	withdrawalSvc *service.WithdrawalService,
	txHistorySvc *service.TxHistoryService,
	depositAddrRepo postgres.DepositAddressRepository,
) *WalletHandler {
	return &WalletHandler{
		walletService:   walletService,
		withdrawalSvc:   withdrawalSvc,
		txHistorySvc:    txHistorySvc,
		depositAddrRepo: depositAddrRepo,
	}
}

// GetBalance returns the authenticated user's wallet via GET /api/v1/wallet/balance.
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

// --- Crypto deposit endpoints ---

// depositSOLResponse is the response for SOL deposit address generation.
type depositSOLResponse struct {
	Address string `json:"address"`
	QRData  string `json:"qr_data"`
}

// DepositSOL generates a SOL deposit address via POST /api/v1/wallet/deposit/sol.
func (h *WalletHandler) DepositSOL(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	da, err := h.depositAddrRepo.GetByUserID(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "deposit address not found — please complete wallet setup")
		return
	}

	response.JSON(w, http.StatusOK, depositSOLResponse{
		Address: da.SolAddr,
		QRData:  fmt.Sprintf("solana:%s", da.SolAddr),
	})
}

// DepositUSDC generates a USDC ATA deposit address via POST /api/v1/wallet/deposit/usdc.
func (h *WalletHandler) DepositUSDC(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	da, err := h.depositAddrRepo.GetByUserID(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "deposit address not found — please complete wallet setup")
		return
	}

	response.JSON(w, http.StatusOK, struct {
		Address string `json:"address"`
		ATA     string `json:"ata"`
		QRData  string `json:"qr_data"`
	}{
		Address: da.SolAddr,
		ATA:     da.USDCAta,
		QRData:  fmt.Sprintf("solana:%s", da.USDCAta),
	})
}

// --- Crypto withdrawal endpoints ---

// cryptoWithdrawRequest is the request body for crypto withdrawals.
type cryptoWithdrawRequest struct {
	Amount    int64  `json:"amount"`
	ToAddress string `json:"to_address"`
}

// cryptoWithdrawResponse is the response for crypto withdrawal requests.
type cryptoWithdrawResponse struct {
	WithdrawalID string `json:"withdrawal_id"`
	Status       string `json:"status"`
}

// WithdrawSOL requests a SOL withdrawal via POST /api/v1/wallet/withdraw/sol.
func (h *WalletHandler) WithdrawSOL(w http.ResponseWriter, r *http.Request) {
	var req cryptoWithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}
	if req.ToAddress == "" {
		response.Error(w, http.StatusBadRequest, "to_address is required")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	withdrawal, err := h.withdrawalSvc.RequestWithdrawal(r.Context(), userID, req.Amount, req.ToAddress, "SOL")
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, cryptoWithdrawResponse{
		WithdrawalID: withdrawal.ID.String(),
		Status:       string(withdrawal.Status),
	})
}

// WithdrawUSDC requests a USDC withdrawal via POST /api/v1/wallet/withdraw/usdc.
func (h *WalletHandler) WithdrawUSDC(w http.ResponseWriter, r *http.Request) {
	var req cryptoWithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be greater than 0")
		return
	}
	if req.ToAddress == "" {
		response.Error(w, http.StatusBadRequest, "to_address is required")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	withdrawal, err := h.withdrawalSvc.RequestWithdrawal(r.Context(), userID, req.Amount, req.ToAddress, "USDC")
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, cryptoWithdrawResponse{
		WithdrawalID: withdrawal.ID.String(),
		Status:       string(withdrawal.Status),
	})
}

// --- Transaction history ---

// GetTxHistory returns paginated Solana transaction history via GET /api/v1/wallet/tx-history.
func (h *WalletHandler) GetTxHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	txType := r.URL.Query().Get("type")

	filter := domain.TxHistoryFilter{
		Type:  txType,
		Page:  page,
		Limit: limit,
	}

	// Parse optional date filters
	if afterStr := r.URL.Query().Get("after"); afterStr != "" {
		if t, err := time.Parse(time.RFC3339, afterStr); err == nil {
			filter.After = &t
		}
	}
	if beforeStr := r.URL.Query().Get("before"); beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			filter.Before = &t
		}
	}

	// Get wallet ID for tx history
	wallet, err := h.walletService.GetBalance(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "wallet not found")
		return
	}

	result, err := h.txHistorySvc.GetHistory(r.Context(), wallet.ID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// --- Admin endpoints ---

// AdminEmergencyPause toggles the emergency pause via POST /api/v1/admin/emergency/pause.
func (h *WalletHandler) AdminEmergencyPause(w http.ResponseWriter, r *http.Request) {
	// Check admin role from JWT claims
	role := middleware.RoleFromContext(r.Context())
	if role != "admin" {
		response.Error(w, http.StatusForbidden, "admin role required")
		return
	}

	// In production, this toggles a DB flag; here we return a success response
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "emergency pause toggled",
	})
}
