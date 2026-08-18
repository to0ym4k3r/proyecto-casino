package handler

import (
	"encoding/json"
	"net/http"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AdminHandler handles admin game management endpoints.
type AdminHandler struct {
	gameService *service.GameService
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(gameService *service.GameService) *AdminHandler {
	return &AdminHandler{gameService: gameService}
}

// requireAdmin checks the JWT role claim and returns 403 if not admin.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	role := middleware.RoleFromContext(r.Context())
	if role != "admin" {
		response.Error(w, http.StatusForbidden, "admin role required")
		return false
	}
	return true
}

// CreateGameRequest is the payload for creating a game.
type CreateGameRequest struct {
	Name        string `json:"name" validate:"required,max=200"`
	Description string `json:"description" validate:"max=1000"`
	Category    string `json:"category" validate:"required,max=100"`
	MinBet      int64  `json:"min_bet" validate:"required,gt=0"`
	MaxBet      int64  `json:"max_bet" validate:"required,gt=0"`
	RTP         int    `json:"rtp" validate:"required,gte=0,lte=100"`
	CoverImage  string `json:"cover_image" validate:"max=500"`
}

// CreateGameHandler creates a new game via the admin games endpoint.
//
// Requires a valid JWT with admin role. Returns 201 with the created game.
func (h *AdminHandler) CreateGameHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req CreateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Category == "" {
		response.Error(w, http.StatusBadRequest, "name and category are required")
		return
	}

	game := &domain.Game{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		MinBet:      req.MinBet,
		MaxBet:      req.MaxBet,
		RTP:         req.RTP,
		Status:      domain.GameStatusActive,
		CoverImage:  req.CoverImage,
	}

	if err := h.gameService.CreateGame(r.Context(), game); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create game")
		return
	}

	response.JSON(w, http.StatusCreated, game)
}

// UpdateGameHandler updates an existing game via the admin games endpoint.
//
// Requires a valid JWT with admin role. Returns 200 with the updated game.
func (h *AdminHandler) UpdateGameHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	gameIDStr := chi.URLParam(r, "gameID")
	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid game ID")
		return
	}

	var req CreateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	game, err := h.gameService.GetGame(r.Context(), gameID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "game not found")
		return
	}

	game.Name = req.Name
	game.Description = req.Description
	game.Category = req.Category
	game.MinBet = req.MinBet
	game.MaxBet = req.MaxBet
	game.RTP = req.RTP
	game.CoverImage = req.CoverImage

	if err := h.gameService.UpdateGame(r.Context(), game); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to update game")
		return
	}

	response.JSON(w, http.StatusOK, game)
}

// DeleteGameHandler deletes a game via the admin games endpoint.
//
// Requires a valid JWT with admin role. Returns 200 on success, 404 if not found.
func (h *AdminHandler) DeleteGameHandler(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	gameIDStr := chi.URLParam(r, "gameID")
	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid game ID")
		return
	}

	if err := h.gameService.DeleteGame(r.Context(), gameID); err != nil {
		response.Error(w, http.StatusNotFound, "game not found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
