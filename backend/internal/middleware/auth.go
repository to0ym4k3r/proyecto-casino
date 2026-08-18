package middleware

import (
	"context"
	"net/http"
	"strings"

	"casino-backend/internal/service"
	"casino-backend/pkg/response"

	"github.com/google/uuid"
)

type contextKey string

// UserIDKey is the context key under which the authenticated user's UUID is stored.
// Handlers retrieve it via middleware.UserIDFromContext(ctx).
const UserIDKey contextKey = "user_id"

// RoleKey is the context key under which the user's role is stored (e.g. "admin", "user").
const RoleKey contextKey = "role"

// UserIDFromContext returns the authenticated user's UUID stored in the request
// context by the Auth middleware. It returns uuid.Nil when the context carries
// no user ID (e.g. on unauthenticated requests).
func UserIDFromContext(ctx context.Context) uuid.UUID {
	userID, _ := ctx.Value(UserIDKey).(uuid.UUID)
	return userID
}

// RoleFromContext returns the user's role stored in the request context.
// Returns "user" (default) when no role is set.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(RoleKey).(string)
	if role == "" {
		return "user"
	}
	return role
}

// Auth returns a middleware that validates JWT access tokens from the
// Authorization header.
//
// The header must follow the format "Bearer <token>". On success the user ID
// extracted from the token claims is stored in the request context under
// UserIDKey and the request is forwarded. Returns 401 with a specific message
// for each failure mode:
//   - Missing Authorization header
//   - Malformed header (not "Bearer <token>")
//   - Invalid or expired token
func Auth(authService *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, http.StatusUnauthorized, "authorization header required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Error(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			userID, err := authService.ValidateToken(parts[1])
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			// Extract role from JWT claims if present
			if role, ok := authService.GetClaim(parts[1], "role"); ok {
				ctx = context.WithValue(ctx, RoleKey, role)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
