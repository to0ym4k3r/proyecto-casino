package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
	Security  SecurityConfig
	Mailer    MailerConfig
	Storage   StorageConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig holds PostgreSQL connection parameters.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// JWTConfig holds JSON Web Token settings.
type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// CORSConfig holds CORS allowed origins.
type CORSConfig struct {
	AllowedOrigins string
}

// RateLimitConfig holds rate limiter settings.
type RateLimitConfig struct {
	// TrustedHeader names the header used to derive the client IP for rate
	// limiting. Empty means X-Forwarded-For / Forwarded are ignored and the
	// peer RemoteAddr is used (anti-spoofing default).
	TrustedHeader string
}

// SecurityConfig holds HTTP security header settings.
type SecurityConfig struct {
	HSTSEnabled bool
}

// MailerConfig holds email transport settings.
type MailerConfig struct {
	Driver        string
	SMTPHost      string
	SMTPPort      string
	SMTPUser      string
	SMTPPassword  string
	SMTPFrom      string
	SMTPTLS       bool
	VerifyBaseURL string
	// EmailVerifyTTLMin bounds how long an issued email verification token
	// stays valid (EMAIL_VERIFY_TTL_MIN, minutes).
	EmailVerifyTTLMin time.Duration
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	// KycDir is the directory where KYC documents are stored
	// (KYC_STORAGE_DIR, design D1). Files live under generated keys, never
	// user-supplied filenames.
	KycDir string
}

// Load reads configuration from environment variables and .env file.
//
// It never returns an error so cmd/migrate keeps working without a JWT secret;
// the server binary validates the secret separately via ValidateJWTSecret.
// JWT_SECRET has no default: a missing or weak value only fails at server
// startup, never at config load.
func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "casino_user"),
			Password: getEnv("DB_PASSWORD", "casino_pass"),
			Name:     getEnv("DB_NAME", "casino"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", ""),
			AccessExpiry:  getDurationMinutes("JWT_ACCESS_EXPIRY", 15),
			RefreshExpiry: getDurationMinutes("JWT_REFRESH_EXPIRY", 10080),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000"),
		},
		RateLimit: RateLimitConfig{
			TrustedHeader: getEnv("RATE_LIMIT_TRUSTED_HEADER", ""),
		},
		Security: SecurityConfig{
			HSTSEnabled: getBool("HSTS_ENABLED", true),
		},
		Mailer: MailerConfig{
			Driver:            getEnv("MAILER_DRIVER", "console"),
			SMTPHost:          getEnv("SMTP_HOST", ""),
			SMTPPort:          getEnv("SMTP_PORT", "587"),
			SMTPUser:          getEnv("SMTP_USER", ""),
			SMTPPassword:      getEnv("SMTP_PASSWORD", ""),
			SMTPFrom:          getEnv("SMTP_FROM", ""),
			SMTPTLS:           getBool("SMTP_TLS", true),
			VerifyBaseURL:     getEnv("VERIFY_BASE_URL", "http://localhost:8080"),
			EmailVerifyTTLMin: getDurationMinutes("EMAIL_VERIFY_TTL_MIN", 1440),
		},
		Storage: StorageConfig{
			KycDir: getEnv("KYC_STORAGE_DIR", "./data/kyc"),
		},
	}
}

// ValidateJWTSecret reports whether the JWT signing secret is strong enough to
// run the server. cmd/server fails fast at startup when it returns an error;
// cmd/migrate never calls it, so migrations keep working without a secret.
func (c *Config) ValidateJWTSecret() error {
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes long (got %d): set a strong secret before starting the server", len(c.JWT.Secret))
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

func getDurationMinutes(key string, fallback int) time.Duration {
	return time.Duration(getInt(key, fallback)) * time.Minute
}
