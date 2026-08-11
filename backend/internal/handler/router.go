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
	authHandler   *AuthHandler
	gameHandler   *GameHandler
	walletHandler *WalletHandler
	userHandler   *UserHandler
	authService   *service.AuthService
	logger        zerolog.Logger
	corsOrigins   string
	trustedHeader string
}

// NewRouter creates a new Router with all handlers and middleware.
func NewRouter(
	authHandler *AuthHandler,
	gameHandler *GameHandler,
	walletHandler *WalletHandler,
	userHandler *UserHandler,
	authService *service.AuthService,
	log zerolog.Logger,
	corsOrigins string,
	trustedHeader string,
) *Router {
	return &Router{
		authHandler:   authHandler,
		gameHandler:   gameHandler,
		walletHandler: walletHandler,
		userHandler:   userHandler,
		authService:   authService,
		logger:        log,
		corsOrigins:   corsOrigins,
		trustedHeader: trustedHeader,
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
//
// A general 100 req/s rate limit is applied to all /api/v1 routes.
func (rt *Router) Setup() *chi.Mux {
	router := chi.NewRouter()

	authH := rt.authHandler
	gameH := rt.gameHandler
	walletH := rt.walletHandler
	userH := rt.userHandler
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
			})
		})

		r.Route("/wallet", func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Get("/balance", walletH.GetBalance)
			r.Post("/deposit", walletH.Deposit)
			r.Post("/withdraw", walletH.Withdraw)
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Get("/me", userH.GetMe)
			r.Put("/me", userH.UpdateMe)
		})
	})

	return router
}
