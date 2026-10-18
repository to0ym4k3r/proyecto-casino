package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/handler"
	"casino-backend/internal/mailer"
	"casino-backend/internal/repository"
	"casino-backend/internal/repository/postgres"
	"casino-backend/internal/service"
	"casino-backend/internal/storage"
	"casino-backend/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	depositAddrRepo := postgres.NewPostgresDepositAddressRepo(pool)
	withdrawalRepo := postgres.NewPostgresWithdrawalRepo(pool)
	betRepo := postgres.NewPostgresBetRepo(pool)
	sessionRepo := postgres.NewPostgresGameSessionRepo(pool)
	bonusDefRepo := postgres.NewPostgresBonusDefinitionRepo(pool)
	bonusGrantRepo := postgres.NewPostgresBonusGrantRepo(pool)
	bonusLedgerRepo := postgres.NewPostgresBonusLedgerRepo(pool)

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
	txFunc := func(ctx context.Context, fn func(pgx.Tx) error) error {
		return postgres.WithTransaction(ctx, pool, fn)
	}
	bonusService := service.NewBonusService(
		bonusDefRepo,
		bonusGrantRepo,
		bonusLedgerRepo,
		walletService,
		service.DefaultBonusHandlers(),
		txFunc,
	)
	gameService := service.NewGameServiceWithBonus(gameRepo, walletRepo, betRepo, sessionRepo, bonusService, txFunc)
	userService := service.NewUserService(userRepo)
	kycService := service.NewKycService(kycRepo)

	expiryScheduler := service.NewBonusExpiryScheduler(bonusService, time.Hour, log)
	expiryScheduler.Start(context.Background())
	defer expiryScheduler.Stop()

	authHandler := handler.NewAuthHandler(authService)
	gameHandler := handler.NewGameHandler(gameService)
	walletHandler := buildWalletHandler(walletService, walletRepo, withdrawalRepo, kycService, bonusService, txFunc, depositAddrRepo)
	userHandler := handler.NewUserHandler(userService)
	kycStore := storage.NewLocalStore(cfg.Storage.KycDir)
	kycHandler := handler.NewKycHandler(kycService, kycStore)
	adminHandler := handler.NewAdminHandler(gameService)
	bonusHandler := handler.NewBonusHandler(bonusService)
	adminBonusHandler := handler.NewAdminBonusHandler(bonusService)

	router := handler.NewRouter(authHandler, gameHandler, walletHandler, userHandler, kycHandler, adminHandler, authService, log, cfg.CORS.AllowedOrigins, cfg.RateLimit.TrustedHeader, bonusHandler, adminBonusHandler)

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

// buildWalletHandler assembles the wallet handler with its production
// dependencies. The WithdrawalService must not be nil: the registered crypto
// withdrawal routes call RequestWithdrawal on it, and the bonus forfeit on
// withdrawal (spec Domain 2, scenario 4) needs the bonus service and txFunc.
// Known stubs until their stores land: withdrawals are never reported paused,
// and the hot-wallet balance fails closed (consulted only once an outflow
// checker is configured, which production does not set).
func buildWalletHandler(
	walletService *service.WalletService,
	walletRepo repository.WalletRepository,
	withdrawalRepo service.WithdrawalRepository,
	kycService *service.KycService,
	bonusService *service.BonusService,
	txFunc service.TxFunc,
	depositAddrRepo postgres.DepositAddressRepository,
) *handler.WalletHandler {
	withdrawalSvc := service.NewWithdrawalService(
		walletRepo,
		withdrawalRepo,
		func(ctx context.Context, userID uuid.UUID) (service.KYCTier, error) {
			sub, err := kycService.Status(ctx, userID)
			if err != nil {
				if errors.Is(err, service.ErrKycNotFound) {
					return service.KYCUnverified, nil
				}
				return "", err
			}
			if sub.Status == domain.KycStatusApproved {
				return service.KYCL1, nil
			}
			return service.KYCUnverified, nil
		},
		func(_ context.Context) (bool, error) { return false, nil },
		func(_ context.Context) (uint64, error) { return 0, errors.New("hot wallet balance not wired") },
	)
	withdrawalSvc.SetBonusForfeit(bonusService, txFunc)
	return handler.NewWalletHandler(walletService, withdrawalSvc, nil, depositAddrRepo)
}
