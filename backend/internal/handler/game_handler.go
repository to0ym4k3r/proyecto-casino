package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"
	"casino-backend/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GameHandler handles HTTP requests for games.
type GameHandler struct {
	gameService *service.GameService
}

// NewGameHandler creates a new game handler.
func NewGameHandler(gameService *service.GameService) *GameHandler {
	return &GameHandler{gameService: gameService}
}

// ListGames returns a paginated list of games via GET /api/v1/games.
//
// Query parameters:
//   - limit — max number of games to return (default 20, capped at 100 by the service).
//   - offset — number of games to skip (default 0).
//
// Invalid or missing query parameters are silently ignored and fall back to defaults.
//
// Response (200): { "games": []Game, "total": int, "limit": int, "offset": int }
func (h *GameHandler) ListGames(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0

	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			limit = v
		}
	}
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			offset = v
		}
	}

	games, total, err := h.gameService.ListGames(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list games")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"games":  games,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetGame returns a single game by its UUID via GET /api/v1/games/{gameID}.
//
// The gameID path parameter must be a valid UUID; otherwise returns 400.
// Returns 404 if no game with that ID exists.
//
// Response (200): Game
func (h *GameHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	gameIDStr := chi.URLParam(r, "gameID")
	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid game ID")
		return
	}

	game, err := h.gameService.GetGame(r.Context(), gameID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "game not found")
		return
	}

	response.JSON(w, http.StatusOK, game)
}

// StartGame begins a new game session for the authenticated user via POST /api/v1/games/start.
//
// Requires a valid JWT (Auth middleware). Decodes a domain.PlaceBetRequest; the
// game_id field is required (must not be uuid.Nil). Returns 400 if the game does
// not exist or is not active. On success returns the newly created GameSession.
//
// Response (200): GameSession
func (h *GameHandler) StartGame(w http.ResponseWriter, r *http.Request) {
	var req domain.PlaceBetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.GameID == uuid.Nil {
		response.Error(w, http.StatusBadRequest, "game_id is required")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	session, err := h.gameService.StartGame(r.Context(), userID, req.GameID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, session)
}
