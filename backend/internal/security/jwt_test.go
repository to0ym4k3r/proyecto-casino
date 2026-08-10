package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newAuthSvc(secret string) *service.AuthService {
	return service.NewAuthService(nil, nil, nil, nil, nil, &config.JWTConfig{
		Secret:        secret,
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}, time.Hour)
}

// newRefreshSeededSvc builds an auth service whose refresh repository already
// holds the given refresh JWT (hashed) for the user — the state a token has
// right after a real login, so the Refresh control can exercise rotation.
func newRefreshSeededSvc(secret, refreshJWT string, userID uuid.UUID) *service.AuthService {
	userRepo := newMemUserRepo()
	refreshRepo := newMemRefreshRepo(userRepo)
	now := time.Now().UTC()
	_ = refreshRepo.Save(context.Background(), &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hashToken(refreshJWT),
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		CreatedAt: now,
	})
	return service.NewAuthService(userRepo, nil, refreshRepo, nil, nil, &config.JWTConfig{
		Secret:        secret,
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
	}, time.Hour)
}

// TestJWTForgeryRejected attacks the JWT validation boundary
// (service/auth.go ValidateToken/Refresh, HMAC keyfunc) with every classic
// forgery vector. All of them must be rejected; the control tokens must be
// accepted.
func TestJWTForgeryRejected(t *testing.T) {
	const secret = "correct-horse-battery-staple-unit-test"
	userID := uuid.New()
	attacker := uuid.New()

	validAccess := signAccess(t, secret, userID)
	validRefresh := signRefresh(t, secret, userID)

	// Control: valid tokens must pass. The refresh control runs against a
	// service whose repository already holds the token (post-login state).
	svc := newRefreshSeededSvc(secret, validRefresh, userID)
	if _, err := svc.ValidateToken(validAccess); err != nil {
		t.Fatalf("control: valid access token rejected: %v", err)
	}
	if _, err := svc.Refresh(context.Background(), validRefresh); err != nil {
		t.Fatalf("control: valid refresh token rejected: %v", err)
	}

	// Tamper the "sub" claim of a valid token without re-signing it.
	tamperSub := func(tokenStr, newSub string) string {
		t.Helper()
		parts := strings.Split(tokenStr, ".")
		if len(parts) != 3 {
			t.Fatalf("not a JWT: %q", tokenStr)
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		var claims map[string]interface{}
		if err := json.Unmarshal(payload, &claims); err != nil {
			t.Fatalf("parse claims: %v", err)
		}
		claims["sub"] = newSub
		raw, err := json.Marshal(claims)
		if err != nil {
			t.Fatalf("marshal claims: %v", err)
		}
		return parts[0] + "." + base64.RawURLEncoding.EncodeToString(raw) + "." + parts[2]
	}

	// alg=none token (the keyfunc must reject the non-HMAC method).
	noneTok, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": userID.String(), "type": "access", "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build alg=none token: %v", err)
	}

	// RSA-signed token (keyfunc must reject the non-HMAC method).
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	rsaTok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": userID.String(), "type": "access", "exp": time.Now().Add(time.Minute).Unix(),
	}).SignedString(rsaKey)
	if err != nil {
		t.Fatalf("build RS256 token: %v", err)
	}

	checkAccess := func(_ *service.AuthService, tok string) error {
		_, err := svc.ValidateToken(tok)
		return err
	}
	checkRefresh := func(_ *service.AuthService, tok string) error {
		_, err := svc.Refresh(context.Background(), tok)
		return err
	}

	tests := []struct {
		name  string
		token string
		check func(*service.AuthService, string) error
	}{
		{"signed with wrong secret", signAccess(t, "attacker-secret", userID), checkAccess},
		{"tampered sub without re-signature", tamperSub(validAccess, attacker.String()), checkAccess},
		{"alg=none token", noneTok, checkAccess},
		{"RSA-signed token (RS256)", rsaTok, checkAccess},
		{"expired access token", signToken(t, secret, "access", userID, time.Now().Add(-time.Hour)), checkAccess},
		{"expired refresh token", signToken(t, secret, "refresh", userID, time.Now().Add(-time.Hour)), checkRefresh},
		{"refresh token used as access", validRefresh, checkAccess},
		{"access token used as refresh", validAccess, checkRefresh},
		{"garbage string", "not-a-jwt", checkAccess},
		{"empty token", "", checkAccess},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.check(svc, tt.token); err == nil {
				t.Errorf("forged token accepted, want rejection")
			}
		})
	}
}

// TestJWTAllZeroSubjectAccepted documents a minor gap found by this suite:
// a correctly signed access token whose subject is the all-zero UUID passes
// ValidateToken and yields uuid.Nil. The Auth middleware then stores uuid.Nil,
// so no wallet/game lookup can ever succeed for it — it is not a privilege
// escalation, but the service should reject a nil subject outright. Fix would
// be trivial: reject uuid.Nil after Parse in ValidateToken.
func TestJWTAllZeroSubjectAccepted(t *testing.T) {
	svc := newAuthSvc(testSecret)
	tok := signToken(t, testSecret, "access", uuid.Nil, time.Now().Add(time.Minute))

	id, err := svc.ValidateToken(tok)
	if err != nil {
		t.Logf("nil subject now rejected — minor gap closed: %v", err)
		return
	}
	if id == uuid.Nil {
		t.Log("CONFIRMED minor gap: ValidateToken accepts an all-zero subject (returns uuid.Nil, nil)")
	} else {
		t.Errorf("unexpected id %v", id)
	}
}

// TestJWTLacksIssAudJti documents finding #12 (MEDIA): generated tokens carry
// only sub/type/exp/iat and no iss, aud or jti claims.
func TestJWTLacksIssAudJti(t *testing.T) {
	th := newTestHandlers(t, testSecret)
	uid := uuid.New()
	now := time.Now().UTC()
	u := &domain.User{
		ID: uid, Username: "claimsprobe", Email: "claimsprobe@example.com",
		PasswordHash:   hashPassword(t, "password123"),
		FirstName:      "F", LastName: "L", IsActive: true,
		EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := th.userRepo.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	pair, err := th.authSvc.Login(context.Background(), domain.LoginRequest{Email: "claimsprobe@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	for _, tok := range []string{pair.AccessToken, pair.RefreshToken} {
		parsed, err := jwt.Parse(tok, func(_ *jwt.Token) (interface{}, error) {
			return []byte(testSecret), nil
		})
		if err != nil {
			t.Fatalf("parse token: %v", err)
		}
		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			t.Fatal("claims are not MapClaims")
		}
		if _, hasISS := claims["iss"]; hasISS {
			t.Error("token carries iss claim")
		}
		if _, hasAUD := claims["aud"]; hasAUD {
			t.Error("token carries aud claim")
		}
		if _, hasJTI := claims["jti"]; hasJTI {
			t.Error("token carries jti claim")
		}
	}
}
