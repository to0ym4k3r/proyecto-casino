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
// not exist or is not active. On success returns the newly created GameSession
// including server_seed_hash.
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

	// Persist the session if persistence is configured
	if h.gameService.HasPersistence() {
		if err := h.gameService.PersistSession(r.Context(), session); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to create session")
			return
		}
	}

	// Return only the hash to the client, not the seed
	response.JSON(w, http.StatusOK, map[string]interface{}{
		"session_id":       session.ID,
		"server_seed_hash": session.ServerSeedHash,
	})
}

// PlaceBetRequest is the payload for placing a bet within a session.
type PlaceBetRequest struct {
	Amount int64 `json:"amount" validate:"required,gt=0"`
}

// PlaceBetHandler places a bet within an active game session via POST /api/v1/games/{gameID}/bet.
//
// Requires a valid JWT. The bet amount is deducted from the wallet atomically.
// Returns 200 with the bet result (win/loss + payout).
func (h *GameHandler) PlaceBetHandler(w http.ResponseWriter, r *http.Request) {
	gameIDStr := chi.URLParam(r, "gameID")
	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid game ID")
		return
	}

	var req PlaceBetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	// Get active session for this user and game
	session, err := h.gameService.GetActiveSession(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "no active session")
		return
	}

	// Get game for RTP
	game, err := h.gameService.GetGame(r.Context(), gameID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "game not found")
		return
	}

	// Place bet atomically
	bet, err := h.gameService.PlaceBet(r.Context(), userID, gameID, session.ID, req.Amount)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve the bet
	payout, multiplier, won := service.ResolveBet(req.Amount, game.RTP)

	if won {
		if err := h.gameService.CreditWin(r.Context(), bet, payout, multiplier); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to credit win")
			return
		}
	} else {
		if err := h.gameService.CreditLoss(r.Context(), bet, multiplier); err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to resolve bet")
			return
		}
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"bet_id":    bet.ID,
		"amount":    req.Amount,
		"payout":    payout,
		"multiplier": multiplier,
		"won":       won,
	})
}

// EndSessionHandler ends an active game session via POST /api/v1/games/{gameID}/end.
//
// Requires a valid JWT. Marks the session as completed.
func (h *GameHandler) EndSessionHandler(w http.ResponseWriter, r *http.Request) {
	gameIDStr := chi.URLParam(r, "gameID")
	gameID, err := uuid.Parse(gameIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid game ID")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	session, err := h.gameService.GetActiveSession(r.Context(), userID, gameID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "no active session")
		return
	}

	if err := h.gameService.EndSession(r.Context(), session.ID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to end session")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"session_id": session.ID,
		"status":     "completed",
	})
}
