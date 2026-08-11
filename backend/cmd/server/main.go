package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/handler"
	"casino-backend/internal/mailer"
	"casino-backend/internal/repository/postgres"
	"casino-backend/internal/service"
	"casino-backend/internal/storage"
	"casino-backend/pkg/logger"
)

// main is the entry point for the Casino backend server.
//
// It performs the following startup sequence:
//  1. Initializes structured logging with the "casino-backend" service name.
//  2. Loads configuration from environment variables via config.Load().
//  3. Establishes a PostgreSQL connection pool with a 10-second timeout.
//  4. Constructs all repository, service, and handler dependencies (manual DI).
//  5. Configures the HTTP server with the chi router and timeout settings.
//  6. Starts the server in a background goroutine.
//  7. Waits for OS interrupt signals (SIGINT/SIGTERM) for graceful shutdown.
//  8. Drains in-flight requests with a 30-second shutdown deadline.
//
// The server listens on the port specified by the PORT environment variable (default: 8080).
func main() {
	log := logger.New("casino-backend")

	cfg := config.Load()

	// Fail fast (GOV-3): the server must never boot with a missing or weak
	// JWT signing secret. cmd/migrate keeps working without one.
	if err := cfg.ValidateJWTSecret(); err != nil {
		log.Fatal().Err(err).Msg("invalid JWT configuration")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	userRepo := postgres.NewPostgresUserRepo(pool)
	walletRepo := postgres.NewPostgresWalletRepo(pool)
	gameRepo := postgres.NewPostgresGameRepo(pool)
	refreshRepo := postgres.NewPostgresRefreshTokenRepo(pool)
	verificationRepo := postgres.NewPostgresEmailVerificationRepo(pool)
	registrationRepo := postgres.NewPostgresRegistrationRepo(pool)
	kycRepo := postgres.NewPostgresKycRepo(pool)

	// Email transport seam (design D3 / spec "SMTP seam"): console in dev,
	// SMTP in production, selected by MAILER_DRIVER.
	var mail mailer.Mailer
	switch cfg.Mailer.Driver {
	case "smtp":
		mail = mailer.NewSMTPMailer(cfg.Mailer)
	default:
		mail = mailer.ConsoleMailer{VerifyBaseURL: cfg.Mailer.VerifyBaseURL}
	}

	authService := service.NewAuthService(userRepo, registrationRepo, refreshRepo, verificationRepo, mail, &cfg.JWT, cfg.Mailer.EmailVerifyTTLMin)
	walletService := service.NewWalletService(walletRepo)
	gameService := service.NewGameService(gameRepo)
	userService := service.NewUserService(userRepo)
	kycService := service.NewKycService(kycRepo)

	authHandler := handler.NewAuthHandler(authService)
	gameHandler := handler.NewGameHandler(gameService)
	walletHandler := handler.NewWalletHandler(walletService)
	userHandler := handler.NewUserHandler(userService)
	kycStore := storage.NewLocalStore(cfg.Storage.KycDir)
	kycHandler := handler.NewKycHandler(kycService, kycStore)

	router := handler.NewRouter(authHandler, gameHandler, walletHandler, userHandler, kycHandler, authService, log, cfg.CORS.AllowedOrigins, cfg.RateLimit.TrustedHeader)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router.Setup(),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info().Str("port", cfg.Server.Port).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	<-done
	log.Info().Msg("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("server stopped gracefully")
}
