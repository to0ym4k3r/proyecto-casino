package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/repository"
	"casino-backend/internal/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// In-memory user repository (profile slice)
// ---------------------------------------------------------------------------

// profileUpdateCall records one UpdateProfile invocation so tests can prove the
// merged values actually crossed the repository boundary.
type profileUpdateCall struct {
	userID    uuid.UUID
	username  string
	firstName string
	lastName  string
}

// fakeUserRepo implements repository.UserRepository in memory for the profile
// handler tests.
type fakeUserRepo struct {
	users       map[uuid.UUID]*domain.User
	updateCalls []profileUpdateCall
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[uuid.UUID]*domain.User)}
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	f.users[u.ID] = u
	return nil
}

func (f *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok || u.DeletedAt != nil {
		return nil, repository.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, repository.ErrUserNotFound
}

func (f *fakeUserRepo) GetByUsername(_ context.Context, _ string) (*domain.User, error) {
	return nil, repository.ErrUserNotFound
}

func (f *fakeUserRepo) Update(_ context.Context, _ *domain.User) error { return nil }
func (f *fakeUserRepo) Delete(_ context.Context, _ uuid.UUID) error    { return nil }
func (f *fakeUserRepo) DeletePermanent(_ context.Context, _ uuid.UUID) error {
	return nil
}

// UpdateProfile mirrors the production repository: it persists exactly the
// three mutable profile fields and never touches email or credentials.
func (f *fakeUserRepo) UpdateProfile(_ context.Context, userID uuid.UUID, username, firstName, lastName string) error {
	u, ok := f.users[userID]
	if !ok {
		return repository.ErrUserNotFound
	}
	f.updateCalls = append(f.updateCalls, profileUpdateCall{
		userID:    userID,
		username:  username,
		firstName: firstName,
		lastName:  lastName,
	})
	u.Username = username
	u.FirstName = firstName
	u.LastName = lastName
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// ---------------------------------------------------------------------------
// Stack construction
// ---------------------------------------------------------------------------

// userTestSecret is a throwaway HMAC secret used only by these tests.
const userTestSecret = "user-handler-test-secret-not-for-prod"

type userTestStack struct {
	h       *UserHandler
	authSvc *service.AuthService
	repo    *fakeUserRepo
}

func newUserTestStack(t *testing.T) *userTestStack {
	t.Helper()
	repo := newFakeUserRepo()
	h := NewUserHandler(service.NewUserService(repo))
	cfg := &config.JWTConfig{Secret: userTestSecret, AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour}
	authSvc := service.NewAuthService(nil, nil, nil, nil, nil, cfg, time.Hour)
	return &userTestStack{h: h, authSvc: authSvc, repo: repo}
}

// seedProfileUser stores a fully-populated user (including a password hash that
// MUST never cross the HTTP boundary) and returns it with its ID.
func seedProfileUser(t *testing.T, st *userTestStack) (*domain.User, uuid.UUID) {
	t.Helper()
	userID := uuid.New()
	verifiedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	u := &domain.User{
		ID:              userID,
		Username:        "player_1",
		Email:           "player1@example.com",
		PasswordHash:    "SUPER-SECRET-HASH-MUST-NOT-LEAK",
		FirstName:       "First",
		LastName:        "Last",
		DateOfBirth:     time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC),
		IsActive:        true,
		EmailVerifiedAt: &verifiedAt,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	st.repo.users[userID] = u
	return u, userID
}

// authedRoute wraps a handler in the real Auth middleware, exercising the full
// protected path: JWT validation + typed context key resolution.
func authedRoute(h http.HandlerFunc, authSvc *service.AuthService) http.Handler {
	return middleware.Auth(authSvc)(http.HandlerFunc(h))
}

// bearerReq performs a request against h, optionally with a Bearer token.
func bearerReq(t *testing.T, h http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// GET /users/me
// ---------------------------------------------------------------------------

func TestGetProfileWithValidToken200(t *testing.T) {
	st := newUserTestStack(t)
	user, userID := seedProfileUser(t, st)
	route := authedRoute(st.h.GetMe, st.authSvc)

	rec := bearerReq(t, route, http.MethodGet, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool        `json:"success"`
		Data    domain.User `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Success {
		t.Error("success = false, want true")
	}
	got := envelope.Data
	if got.ID != userID || got.Username != user.Username || got.Email != user.Email ||
		got.FirstName != user.FirstName || got.LastName != user.LastName ||
		!got.DateOfBirth.Equal(user.DateOfBirth) {
		t.Errorf("profile mismatch: got %+v, want %+v", got, user)
	}
	if got.EmailVerifiedAt == nil || !got.EmailVerifiedAt.Equal(*user.EmailVerifiedAt) {
		t.Errorf("email_verified_at = %v, want %v", got.EmailVerifiedAt, user.EmailVerifiedAt)
	}

	// Credentials must never cross the HTTP boundary.
	if strings.Contains(rec.Body.String(), user.PasswordHash) {
		t.Errorf("response leaks password hash: %s", rec.Body.String())
	}
}

func TestGetProfileWithoutToken401(t *testing.T) {
	st := newUserTestStack(t)
	seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.GetMe, st.authSvc), http.MethodGet, "/api/v1/users/me", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Errorf("body = %s, want success=false envelope", rec.Body.String())
	}
}

func TestGetProfileWithInvalidToken401(t *testing.T) {
	st := newUserTestStack(t)
	seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.GetMe, st.authSvc), http.MethodGet, "/api/v1/users/me", "not-a-real-token", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestGetProfileForMissingUser401(t *testing.T) {
	st := newUserTestStack(t)
	// Deliberately no seed: the token's subject does not exist in the repository.
	userID := uuid.New()

	rec := bearerReq(t, authedRoute(st.h.GetMe, st.authSvc), http.MethodGet, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID), "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// PUT /users/me
// ---------------------------------------------------------------------------

func TestUpdateProfileValidFields200(t *testing.T) {
	st := newUserTestStack(t)
	_, userID := seedProfileUser(t, st)
	route := authedRoute(st.h.UpdateMe, st.authSvc)

	rec := bearerReq(t, route, http.MethodPut, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID),
		`{"username":"new_name","first_name":"New","last_name":"Name"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"new_name"`) ||
		!strings.Contains(rec.Body.String(), `"New"`) ||
		!strings.Contains(rec.Body.String(), `"Name"`) {
		t.Errorf("body = %s, want updated profile values", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"player1@example.com"`) {
		t.Errorf("body = %s, email must remain unchanged", rec.Body.String())
	}
	if len(st.repo.updateCalls) != 1 {
		t.Fatalf("update calls = %d, want 1", len(st.repo.updateCalls))
	}
	if st.repo.updateCalls[0].username != "new_name" ||
		st.repo.updateCalls[0].firstName != "New" ||
		st.repo.updateCalls[0].lastName != "Name" {
		t.Errorf("repo received %+v, want merged new values", st.repo.updateCalls[0])
	}
}

func TestUpdateProfileOmittedFieldsStayUnchanged(t *testing.T) {
	st := newUserTestStack(t)
	user, userID := seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.UpdateMe, st.authSvc), http.MethodPut, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID), `{"username":"only_name"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	updated := st.repo.users[userID]
	if updated.Username != "only_name" {
		t.Errorf("username = %q, want only_name", updated.Username)
	}
	if updated.FirstName != user.FirstName || updated.LastName != user.LastName {
		t.Errorf("omitted fields changed: got first=%q last=%q, want first=%q last=%q",
			updated.FirstName, updated.LastName, user.FirstName, user.LastName)
	}
}

func TestUpdateProfileEmailImmutable400(t *testing.T) {
	st := newUserTestStack(t)
	user, userID := seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.UpdateMe, st.authSvc), http.MethodPut, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID), `{"email":"hacker@example.com"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "email") {
		t.Errorf("body = %s, want email-immutable error message", rec.Body.String())
	}
	// A 400 must leave the profile unchanged and must not reach the repository.
	if st.repo.users[userID].Username != user.Username {
		t.Errorf("profile changed despite 400: username = %q", st.repo.users[userID].Username)
	}
	if len(st.repo.updateCalls) != 0 {
		t.Errorf("repository write happened on rejected payload: %+v", st.repo.updateCalls)
	}
}

func TestUpdateProfileInvalidUsername400(t *testing.T) {
	st := newUserTestStack(t)
	user, userID := seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.UpdateMe, st.authSvc), http.MethodPut, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID), `{"username":"ab"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if st.repo.users[userID].Username != user.Username {
		t.Errorf("profile changed despite 400: username = %q", st.repo.users[userID].Username)
	}
	if len(st.repo.updateCalls) != 0 {
		t.Errorf("repository write happened on invalid payload: %+v", st.repo.updateCalls)
	}
}

func TestUpdateProfileOversizedName400(t *testing.T) {
	st := newUserTestStack(t)
	user, userID := seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.UpdateMe, st.authSvc), http.MethodPut, "/api/v1/users/me",
		signAccessToken(t, userTestSecret, userID),
		`{"first_name":"`+strings.Repeat("a", 101)+`"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if st.repo.users[userID].FirstName != user.FirstName {
		t.Errorf("profile changed despite 400: first_name = %q", st.repo.users[userID].FirstName)
	}
}

func TestUpdateProfileWithoutToken401(t *testing.T) {
	st := newUserTestStack(t)
	seedProfileUser(t, st)

	rec := bearerReq(t, authedRoute(st.h.UpdateMe, st.authSvc), http.MethodPut, "/api/v1/users/me", "", `{"username":"x"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}
