package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func signAccessToken(t *testing.T, secret string, userID uuid.UUID) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"type": "access",
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func newAuthService(t *testing.T, secret string) *service.AuthService {
	t.Helper()
	// Auth middleware only ever calls ValidateToken, which touches no
	// repositories — nil fakes are safe here.
	return service.NewAuthService(nil, nil, nil, nil, nil, &config.JWTConfig{
		Secret:        secret,
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}, time.Hour)
}

// TestAuthMiddlewareStoresUserIDUnderTypedKey proves the write side: a valid
// JWT is accepted and the user ID is stored under UserIDKey, retrievable with
// the same key and via UserIDFromContext.
func TestAuthMiddlewareStoresUserIDUnderTypedKey(t *testing.T) {
	const secret = "test-secret"
	userID := uuid.New()
	svc := newAuthService(t, secret)

	var gotViaKey, gotViaHelper uuid.UUID
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotViaKey, _ = r.Context().Value(UserIDKey).(uuid.UUID)
		gotViaHelper = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(svc)(probe)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, secret, userID))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotViaKey != userID {
		t.Errorf("UserIDKey value = %v, want %v", gotViaKey, userID)
	}
	if gotViaHelper != userID {
		t.Errorf("UserIDFromContext = %v, want %v", gotViaHelper, userID)
	}
}

// TestAuthMiddlewareRejectsUnauthorized covers every 401 failure mode of the
// middleware: missing header, malformed header, and invalid token.
func TestAuthMiddlewareRejectsUnauthorized(t *testing.T) {
	svc := newAuthService(t, "test-secret")
	probe := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := Auth(svc)(probe)

	tests := []struct {
		name string
		auth string
	}{
		{name: "missing header", auth: ""},
		{name: "malformed scheme", auth: "Basic abc"},
		{name: "missing token", auth: "Bearer"},
		{name: "invalid token", auth: "Bearer not-a-jwt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

// TestUserIDFromContext asserts the helper only reads the typed UserIDKey — a
// plain string key must not match, otherwise the read/write sides would disagree.
func TestUserIDFromContext(t *testing.T) {
	userID := uuid.New()

	t.Run("empty context returns uuid.Nil", func(t *testing.T) {
		if got := UserIDFromContext(context.Background()); got != uuid.Nil {
			t.Errorf("got %v, want uuid.Nil", got)
		}
	})

	t.Run("plain string key is not readable (type mismatch)", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "user_id", userID)
		if got := UserIDFromContext(ctx); got != uuid.Nil {
			t.Errorf("got %v, want uuid.Nil (string key must not match typed key)", got)
		}
	})

	t.Run("typed key returns the user ID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserIDKey, userID)
		if got := UserIDFromContext(ctx); got != userID {
			t.Errorf("got %v, want %v", got, userID)
		}
	})
}
