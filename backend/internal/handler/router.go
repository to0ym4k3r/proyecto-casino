package handler

import (
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
)

// Router holds all dependencies for HTTP route configuration.
type Router struct {
	authHandler        *AuthHandler
	gameHandler        *GameHandler
	walletHandler      *WalletHandler
	userHandler        *UserHandler
	kycHandler         *KycHandler
	adminHandler       *AdminHandler
	bonusHandler       *BonusHandler
	adminBonusHandler  *AdminBonusHandler
	authService        *service.AuthService
	logger             zerolog.Logger
	corsOrigins        string
	trustedHeader      string
}

// NewRouter creates a new Router with all handlers and middleware.
func NewRouter(
	authHandler *AuthHandler,
	gameHandler *GameHandler,
	walletHandler *WalletHandler,
	userHandler *UserHandler,
	kycHandler *KycHandler,
	adminHandler *AdminHandler,
	authService *service.AuthService,
	log zerolog.Logger,
	corsOrigins string,
	trustedHeader string,
	bonusHandler *BonusHandler,
	adminBonusHandler *AdminBonusHandler,
) *Router {
	return &Router{
		authHandler:       authHandler,
		gameHandler:       gameHandler,
		walletHandler:     walletHandler,
		userHandler:       userHandler,
		kycHandler:        kycHandler,
		adminHandler:      adminHandler,
		bonusHandler:      bonusHandler,
		adminBonusHandler: adminBonusHandler,
		authService:       authService,
		logger:            log,
		corsOrigins:       corsOrigins,
		trustedHeader:     trustedHeader,
	}
}

// Setup configures and returns the chi router with all routes and middleware.
//
// Global middleware applied in order:
//  1. RequestID — injects a unique request ID into each request.
//  2. RequestLogger — logs every completed request with zerolog.
//  3. Logger — basic request/response logging wrapper.
//  4. Recoverer — catches panics and returns 500.
//  5. SecurityHeaders — sets restrictive security headers on every response.
//  6. CORS — handles cross-origin requests based on configured origins.
//
// Route groups under /api/v1:
//   - /auth  — public endpoints (register, verify, login, refresh, logout)
//     with a strict 5 req/s rate limit.
//   - /games — public list and detail endpoints; /start is protected by Auth middleware.
//   - /wallet — all endpoints require a valid JWT (Auth middleware applied at group level).
//   - /users  — all endpoints require a valid JWT (Auth middleware applied at group level).
//   - /kyc    — all endpoints require a valid JWT (Auth middleware applied at group level).
//
// A general 100 req/s rate limit is applied to all /api/v1 routes.
func (rt *Router) Setup() *chi.Mux {
	router := chi.NewRouter()

	authH := rt.authHandler
	gameH := rt.gameHandler
	walletH := rt.walletHandler
	userH := rt.userHandler
	kycH := rt.kycHandler
	authSvc := rt.authService

	router.Use(chimw.RequestID)
	router.Use(middleware.RequestLogger(rt.logger))
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.SecurityHeaders)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{rt.corsOrigins},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authRateLimiter := middleware.NewRateLimiter(5, 10, rt.trustedHeader)
	generalRateLimiter := middleware.NewRateLimiter(100, 200, rt.trustedHeader)

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(generalRateLimiter.Limit)

		r.Route("/auth", func(r chi.Router) {
			r.Use(authRateLimiter.Limit)
			r.Post("/register", authH.Register)
			r.Post("/verify", authH.VerifyEmail)
			r.Post("/login", authH.Login)
			r.Post("/refresh", authH.Refresh)
			r.Post("/logout", authH.Logout)
		})

		r.Route("/games", func(r chi.Router) {
			r.Get("/", gameH.ListGames)
			r.Get("/{gameID}", gameH.GetGame)

			r.Group(func(r chi.Router) {
				r.Use(middleware.Auth(authSvc))
				r.Post("/start", gameH.StartGame)
				r.Post("/{gameID}/bet", gameH.PlaceBetHandler)
				r.Post("/{gameID}/end", gameH.EndSessionHandler)
			})
		})

		r.Route("/wallet", func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Get("/balance", walletH.GetBalance)
			r.Post("/deposit", walletH.Deposit)
			r.Post("/withdraw", walletH.Withdraw)
			r.Post("/deposit/sol", walletH.DepositSOL)
			r.Post("/deposit/usdc", walletH.DepositUSDC)
			r.Post("/withdraw/sol", walletH.WithdrawSOL)
			r.Post("/withdraw/usdc", walletH.WithdrawUSDC)
			r.Get("/tx-history", walletH.GetTxHistory)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Post("/emergency/pause", walletH.AdminEmergencyPause)
			r.Post("/games", rt.adminHandler.CreateGameHandler)
			r.Put("/games/{gameID}", rt.adminHandler.UpdateGameHandler)
			r.Delete("/games/{gameID}", rt.adminHandler.DeleteGameHandler)

			// Bonus admin endpoints (nil-safe)
			if rt.adminBonusHandler != nil {
				ah := rt.adminBonusHandler
				r.Get("/bonuses", ah.ListDefinitions)
				r.Post("/bonuses", ah.CreateDefinition)
				r.Put("/bonuses/{id}", ah.UpdateDefinition)
				r.Get("/bonuses/grants", ah.ListGrants)
				r.Post("/bonuses/{id}/expire", ah.ExpireGrant)
			}
		})

		// Bonus user endpoints (nil-safe)
		if rt.bonusHandler != nil {
			bh := rt.bonusHandler
			r.Route("/bonuses", func(r chi.Router) {
				r.Use(middleware.Auth(authSvc))
				r.Get("/available", bh.ListAvailable)
				r.Post("/claim", bh.Claim)
				r.Get("/active", bh.ListActive)
				r.Post("/forfeit/{grantID}", bh.Forfeit)
			})
		}

		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Get("/me", userH.GetMe)
			r.Put("/me", userH.UpdateMe)
		})

		r.Route("/kyc", func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Post("/submit", kycH.SubmitKYC)
			r.Get("/status", kycH.GetKYCStatus)
			r.Post("/documents", kycH.UploadDocument)
		})
	})

	return router
}
