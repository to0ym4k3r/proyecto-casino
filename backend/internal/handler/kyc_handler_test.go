package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/domain"
	"casino-backend/internal/repository"
	"casino-backend/internal/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// In-memory KYC repository (Phase 4 slice)
// ---------------------------------------------------------------------------

// fakeKycRepo implements repository.KycRepository in memory. It mirrors the
// production semantics: one submission per user (second submit → duplicate),
// status lookups by user ID, and document rows recorded for inspection.
type fakeKycRepo struct {
	submissions         map[uuid.UUID]*domain.KycSubmission // keyed by user ID
	docs                []domain.KycDocument
	failCreateDocument  bool
}

func newFakeKycRepo() *fakeKycRepo {
	return &fakeKycRepo{submissions: make(map[uuid.UUID]*domain.KycSubmission)}
}

func (f *fakeKycRepo) Submit(_ context.Context, sub *domain.KycSubmission) error {
	if _, ok := f.submissions[sub.UserID]; ok {
		return repository.ErrKycAlreadyExists
	}
	f.submissions[sub.UserID] = sub
	return nil
}

func (f *fakeKycRepo) GetByUserID(_ context.Context, userID uuid.UUID) (*domain.KycSubmission, error) {
	sub, ok := f.submissions[userID]
	if !ok {
		return nil, repository.ErrKycNotFound
	}
	return sub, nil
}

func (f *fakeKycRepo) CreateDocument(_ context.Context, doc *domain.KycDocument) error {
	if f.failCreateDocument {
		return errors.New("simulated metadata write failure")
	}
	f.docs = append(f.docs, *doc)
	return nil
}

// fakeStore implements storage.Store in memory: it records every Save key and
// never touches the filesystem, so tests can prove nothing is stored when an
// upload is rejected (413/415/400).
type fakeStore struct {
	saves   []fakeSaveCall
	deletes []string
}

type fakeSaveCall struct {
	key  string
	size int64
}

func (f *fakeStore) Save(_ context.Context, key string, r io.Reader) (int64, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, err
	}
	f.saves = append(f.saves, fakeSaveCall{key: key, size: n})
	return n, nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	return nil
}

// ---------------------------------------------------------------------------
// Stack construction
// ---------------------------------------------------------------------------

// kycTestSecret is a throwaway HMAC secret used only by these tests.
const kycTestSecret = "kyc-handler-test-secret-not-for-prod"

type kycTestStack struct {
	h       *KycHandler
	authSvc *service.AuthService
	repo    *fakeKycRepo
	store   *fakeStore
}

func newKycTestStack(t *testing.T) *kycTestStack {
	t.Helper()
	repo := newFakeKycRepo()
	store := &fakeStore{}
	h := NewKycHandler(service.NewKycService(repo), store)
	cfg := &config.JWTConfig{Secret: kycTestSecret, AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour}
	authSvc := service.NewAuthService(nil, nil, nil, nil, nil, cfg, time.Hour)
	return &kycTestStack{h: h, authSvc: authSvc, repo: repo, store: store}
}

// kycSubmitBody builds a valid KYC identity payload.
func kycSubmitBody() string {
	return `{"full_name":"Ada Lovelace","nationality":"GB","document_type":"passport","document_number":"P1234567","date_of_birth":"1990-01-02T00:00:00Z"}`
}

// ---------------------------------------------------------------------------
// POST /kyc/submit
// ---------------------------------------------------------------------------

func TestKycSubmitValid201(t *testing.T) {
	st := newKycTestStack(t)
	userID := uuid.New()
	route := authedRoute(st.h.SubmitKYC, st.authSvc)

	rec := bearerReq(t, route, http.MethodPost, "/api/v1/kyc/submit",
		signAccessToken(t, kycTestSecret, userID), kycSubmitBody())

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool                 `json:"success"`
		Data    domain.KycSubmission `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Success {
		t.Error("success = false, want true")
	}
	if envelope.Data.Status != domain.KycStatusPending {
		t.Errorf("status = %q, want pending", envelope.Data.Status)
	}
	if envelope.Data.UserID != userID {
		t.Errorf("user_id = %s, want %s", envelope.Data.UserID, userID)
	}
	// The repository must have persisted the submission with the identity fields.
	got, ok := st.repo.submissions[userID]
	if !ok {
		t.Fatal("submission not persisted in repository")
	}
	if got.FullName != "Ada Lovelace" || got.Nationality != "GB" || got.DocumentType != "passport" || got.DocumentNumber != "P1234567" {
		t.Errorf("persisted submission = %+v, want identity fields", got)
	}
}

func TestKycSubmitDuplicate409(t *testing.T) {
	st := newKycTestStack(t)
	userID := uuid.New()
	route := authedRoute(st.h.SubmitKYC, st.authSvc)

	first := bearerReq(t, route, http.MethodPost, "/api/v1/kyc/submit",
		signAccessToken(t, kycTestSecret, userID), kycSubmitBody())
	if first.Code != http.StatusCreated {
		t.Fatalf("first submit status = %d, want 201 (body: %s)", first.Code, first.Body.String())
	}

	second := bearerReq(t, route, http.MethodPost, "/api/v1/kyc/submit",
		signAccessToken(t, kycTestSecret, userID), kycSubmitBody())
	if second.Code != http.StatusConflict {
		t.Fatalf("second submit status = %d, want 409 (body: %s)", second.Code, second.Body.String())
	}
	if !strings.Contains(strings.ToLower(second.Body.String()), "already") {
		t.Errorf("body = %s, want already-exists message", second.Body.String())
	}
}

func TestKycSubmitInvalidMissingFullName400(t *testing.T) {
	st := newKycTestStack(t)
	userID := uuid.New()

	rec := bearerReq(t, authedRoute(st.h.SubmitKYC, st.authSvc), http.MethodPost, "/api/v1/kyc/submit",
		signAccessToken(t, kycTestSecret, userID), `{"nationality":"GB"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if _, ok := st.repo.submissions[userID]; ok {
		t.Error("invalid submission was persisted")
	}
}

func TestKycSubmitWithoutToken401(t *testing.T) {
	st := newKycTestStack(t)

	rec := bearerReq(t, authedRoute(st.h.SubmitKYC, st.authSvc), http.MethodPost, "/api/v1/kyc/submit", "", kycSubmitBody())

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Errorf("body = %s, want success=false envelope", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// GET /kyc/status
// ---------------------------------------------------------------------------

func TestKycStatusExistingPending200(t *testing.T) {
	st := newKycTestStack(t)
	userID := uuid.New()
	// Seed the submission directly in the fake repo: it already exists.
	st.repo.submissions[userID] = &domain.KycSubmission{
		ID:          uuid.New(),
		UserID:      userID,
		FullName:    "Ada Lovelace",
		Status:      domain.KycStatusPending,
		SubmittedAt: time.Now().UTC(),
	}

	rec := bearerReq(t, authedRoute(st.h.GetKYCStatus, st.authSvc), http.MethodGet, "/api/v1/kyc/status",
		signAccessToken(t, kycTestSecret, userID), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool                 `json:"success"`
		Data    domain.KycSubmission `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Success {
		t.Error("success = false, want true")
	}
	if envelope.Data.Status != domain.KycStatusPending {
		t.Errorf("status = %q, want pending", envelope.Data.Status)
	}
	if envelope.Data.UserID != userID {
		t.Errorf("user_id = %s, want %s", envelope.Data.UserID, userID)
	}
}

func TestKycStatusNoSubmission404(t *testing.T) {
	st := newKycTestStack(t)
	userID := uuid.New()
	// No seeded submission: the status endpoint must report the absence.

	rec := bearerReq(t, authedRoute(st.h.GetKYCStatus, st.authSvc), http.MethodGet, "/api/v1/kyc/status",
		signAccessToken(t, kycTestSecret, userID), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestKycStatusWithoutToken401(t *testing.T) {
	st := newKycTestStack(t)

	rec := bearerReq(t, authedRoute(st.h.GetKYCStatus, st.authSvc), http.MethodGet, "/api/v1/kyc/status", "", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /kyc/documents (upload boundary, design D2)
// ---------------------------------------------------------------------------

// kycUploadLimit is the maximum accepted upload size in bytes (5 MB).
const kycUploadLimit = 5 << 20

// sniffable content for the three whitelisted types. The JPEG/PNG/PDF magic
// prefixes are what http.DetectContentType / the %PDF- check key on.
var (
	jpegContent = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	pngContent  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R'}
	pdfContent  = []byte{'%', 'P', 'D', 'F', '-', '1', '.', '4', '\n', '%', 0xE2, 0xE3, 0xCF, 0xD3}
)

// multipartBody builds a multipart request body with one part named "document"
// and returns its Content-Type plus raw bytes.
func multipartBody(t *testing.T, fileName string, content []byte) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("document", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return mw.FormDataContentType(), buf.Bytes()
}

// uploadReq performs a multipart upload against the authed upload route.
func uploadReq(t *testing.T, st *kycTestStack, fileName string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	userID := uuid.New()
	ct, body := multipartBody(t, fileName, content)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/kyc/documents", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, kycTestSecret, userID))
	rec := httptest.NewRecorder()
	authedRoute(st.h.UploadDocument, st.authSvc).ServeHTTP(rec, req)
	return rec
}

func TestKycUploadValidJpg201(t *testing.T) {
	st := newKycTestStack(t)

	rec := uploadReq(t, st, "id_card.jpg", jpegContent)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool               `json:"success"`
		Data    domain.KycDocument `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Success {
		t.Error("success = false, want true")
	}
	doc := envelope.Data
	if doc.ContentType != "image/jpeg" {
		t.Errorf("content_type = %q, want image/jpeg", doc.ContentType)
	}
	if doc.SizeBytes != int64(len(jpegContent)) {
		t.Errorf("size_bytes = %d, want %d", doc.SizeBytes, len(jpegContent))
	}
	if doc.FileName != "id_card.jpg" {
		t.Errorf("file_name = %q, want original id_card.jpg", doc.FileName)
	}
	// The stored name is a generated key, NEVER the user filename.
	if doc.StoredName == "id_card.jpg" || !strings.HasSuffix(doc.StoredName, ".jpg") {
		t.Errorf("stored_name = %q, want generated key ending in .jpg", doc.StoredName)
	}
	// The file reached the store under exactly the key the metadata references.
	if len(st.store.saves) != 1 {
		t.Fatalf("store saves = %d, want 1", len(st.store.saves))
	}
	if st.store.saves[0].key != doc.StoredName {
		t.Errorf("stored key = %q, want %q", st.store.saves[0].key, doc.StoredName)
	}
	// The metadata row was persisted.
	if len(st.repo.docs) != 1 {
		t.Fatalf("document rows = %d, want 1", len(st.repo.docs))
	}
	if st.repo.docs[0].StoredName != doc.StoredName {
		t.Errorf("repo stored_name = %q, want %q", st.repo.docs[0].StoredName, doc.StoredName)
	}
}

func TestKycUploadMetadataFailureCompensatesDelete(t *testing.T) {
	st := newKycTestStack(t)
	// Simulate the metadata write failing AFTER the file was stored
	// (design D2: nothing may remain stored on a rejected request).
	st.repo.failCreateDocument = true

	rec := uploadReq(t, st, "id_card.jpg", jpegContent)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
	}
	// The file reached the store...
	if len(st.store.saves) != 1 {
		t.Fatalf("store saves = %d, want 1 (file stored before metadata write)", len(st.store.saves))
	}
	// ...and the handler must compensate by deleting the stored key.
	if len(st.store.deletes) != 1 {
		t.Fatalf("store deletes = %d, want 1 — orphaned file must be removed", len(st.store.deletes))
	}
	if st.store.deletes[0] != st.store.saves[0].key {
		t.Errorf("deleted key = %q, want stored key %q", st.store.deletes[0], st.store.saves[0].key)
	}
	// No metadata row may be recorded for the rejected request.
	if len(st.repo.docs) != 0 {
		t.Errorf("document rows = %d, want 0", len(st.repo.docs))
	}
}

func TestKycUploadValidPng201(t *testing.T) {
	st := newKycTestStack(t)

	rec := uploadReq(t, st, "proof.png", pngContent)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"image/png"`) {
		t.Errorf("body = %s, want image/png content type", rec.Body.String())
	}
	if len(st.store.saves) != 1 || len(st.repo.docs) != 1 {
		t.Errorf("png upload not persisted: saves=%d docs=%d", len(st.store.saves), len(st.repo.docs))
	}
}

func TestKycUploadValidPdf201(t *testing.T) {
	st := newKycTestStack(t)

	rec := uploadReq(t, st, "statement.pdf", pdfContent)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"application/pdf"`) {
		t.Errorf("body = %s, want application/pdf content type", rec.Body.String())
	}
	if len(st.store.saves) != 1 || len(st.repo.docs) != 1 {
		t.Errorf("pdf upload not persisted: saves=%d docs=%d", len(st.store.saves), len(st.repo.docs))
	}
}

func TestKycUploadOversized413(t *testing.T) {
	st := newKycTestStack(t)

	// 5 MB + 1 KB: the multipart body exceeds the 5 MB MaxBytesReader cap.
	content := make([]byte, kycUploadLimit+1024)
	copy(content, jpegContent)
	rec := uploadReq(t, st, "big.jpg", content)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(st.store.saves) != 0 {
		t.Errorf("store saves = %d, want 0 — nothing stored on 413", len(st.store.saves))
	}
	if len(st.repo.docs) != 0 {
		t.Errorf("document rows = %d, want 0 on 413", len(st.repo.docs))
	}
}

func TestKycUploadSniffExtMismatch415(t *testing.T) {
	st := newKycTestStack(t)

	// PNG magic bytes smuggled inside a .jpg filename: the sniffed type and
	// the whitelisted extension disagree.
	rec := uploadReq(t, st, "fake.jpg", pngContent)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(st.store.saves) != 0 {
		t.Errorf("store saves = %d, want 0 — nothing stored on 415", len(st.store.saves))
	}
	if len(st.repo.docs) != 0 {
		t.Errorf("document rows = %d, want 0 on 415", len(st.repo.docs))
	}
}

func TestKycUploadDisallowedExtension415(t *testing.T) {
	st := newKycTestStack(t)

	// Valid JPEG content under a non-whitelisted extension.
	rec := uploadReq(t, st, "note.txt", jpegContent)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(st.store.saves) != 0 {
		t.Errorf("store saves = %d, want 0 — nothing stored on 415", len(st.store.saves))
	}
}

func TestKycUploadEmpty400(t *testing.T) {
	st := newKycTestStack(t)

	// A zero-byte part passes the size and type checks but is an empty file.
	rec := uploadReq(t, st, "empty.jpg", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(st.store.saves) != 0 {
		t.Errorf("store saves = %d, want 0 — nothing stored on 400", len(st.store.saves))
	}
	if len(st.repo.docs) != 0 {
		t.Errorf("document rows = %d, want 0 on 400", len(st.repo.docs))
	}
}

func TestKycUploadMissingPart400(t *testing.T) {
	st := newKycTestStack(t)

	// Multipart form without the "document" part.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("note", "hello"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kyc/documents", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+signAccessToken(t, kycTestSecret, uuid.New()))
	rec := httptest.NewRecorder()
	authedRoute(st.h.UploadDocument, st.authSvc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(st.store.saves) != 0 {
		t.Errorf("store saves = %d, want 0 — nothing stored on 400", len(st.store.saves))
	}
}

func TestKycUploadWithoutToken401(t *testing.T) {
	st := newKycTestStack(t)

	ct, body := multipartBody(t, "id.jpg", jpegContent)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/kyc/documents", bytes.NewReader(body))
	req.Header.Set("Content-Type", ct)
	rec := httptest.NewRecorder()
	authedRoute(st.h.UploadDocument, st.authSvc).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}
