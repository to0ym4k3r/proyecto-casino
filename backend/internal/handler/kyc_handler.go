package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"casino-backend/internal/domain"
	"casino-backend/internal/middleware"
	"casino-backend/internal/service"
	"casino-backend/internal/storage"
	"casino-backend/pkg/response"

	"github.com/google/uuid"
)

// maxKycBodyBytes caps the JSON payload for the KYC identity endpoints so
// oversized bodies are rejected before JSON decoding.
const maxKycBodyBytes = 1 << 20

// maxKycUploadBytes caps the multipart body for document uploads at 5 MB
// (design D2); the multipart boundary and field overhead count toward the cap.
const maxKycUploadBytes = 5 << 20

// KycHandler handles the authenticated user's KYC identity and document
// endpoints.
type KycHandler struct {
	kycService *service.KycService
	store      storage.Store
}

// NewKycHandler creates a new KYC handler.
func NewKycHandler(kycService *service.KycService, store storage.Store) *KycHandler {
	return &KycHandler{kycService: kycService, store: store}
}

// SubmitKYC handles POST /kyc/submit.
//
// Auth required: the middleware stores the user ID under the single typed
// context key and the handler reads it via middleware.UserIDFromContext —
// never a string key. On success the submission is returned inside the
// standard {success,data,error} envelope with status pending (admin review is
// a later change).
//
// Responses: 201 with the submission, 400 on an invalid payload, 401 without
// a valid token, 409 when the user already has a submission.
func (h *KycHandler) SubmitKYC(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxKycBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req domain.KycSubmission
	if err := json.Unmarshal(body, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sub, err := h.kycService.Submit(r.Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidKycSubmission):
			response.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrKycAlreadyExists):
			response.Error(w, http.StatusConflict, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to submit KYC")
		}
		return
	}

	response.JSON(w, http.StatusCreated, sub)
}

// GetKYCStatus handles GET /kyc/status.
//
// Auth required; reads the user ID from the typed context key. Returns the
// current submission with its status, or 404 when the user has not submitted
// KYC yet.
//
// Responses: 200 with the submission, 401 without a valid token, 404 when no
// submission exists.
func (h *KycHandler) GetKYCStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sub, err := h.kycService.Status(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrKycNotFound):
			response.Error(w, http.StatusNotFound, "no KYC submission found")
		default:
			response.Error(w, http.StatusInternalServerError, "failed to load KYC status")
		}
		return
	}

	response.JSON(w, http.StatusOK, sub)
}

// UploadDocument handles POST /kyc/documents.
//
// The multipart body is capped with http.MaxBytesReader (413 when exceeded).
// The "document" part must be present and non-empty (400), and its extension
// plus sniffed content must both be in the jpg/png/pdf whitelist and agree
// with each other (415). On success the file is written to the storage layer
// under a generated key — UUID + sanitized extension, never the user
// filename (design D1) — and a kyc_documents metadata row is persisted.
// Nothing is stored on any rejection path (design D2).
//
// Responses: 201 with document metadata, 400, 401, 413, 415, 500.
func (h *KycHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		response.Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxKycUploadBytes)
	if err := r.ParseMultipartForm(maxKycUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.Error(w, http.StatusRequestEntityTooLarge, "file exceeds the 5 MB limit")
			return
		}
		response.Error(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "document file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "failed to read document")
		return
	}
	if len(data) == 0 {
		response.Error(w, http.StatusBadRequest, "document is empty")
		return
	}

	ext, contentType, err := storage.ValidateKycDocument(header.Filename, data)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidKycType) {
			response.Error(w, http.StatusUnsupportedMediaType, err.Error())
			return
		}
		response.Error(w, http.StatusBadRequest, "invalid document")
		return
	}

	key := uuid.New().String() + "." + ext
	size, err := h.store.Save(r.Context(), key, bytes.NewReader(data))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to store document")
		return
	}

	doc := &domain.KycDocument{
		ID:          uuid.New(),
		UserID:      userID,
		FileName:    header.Filename,
		StoredName:  key,
		ContentType: contentType,
		SizeBytes:   size,
		CreatedAt:   time.Now().UTC(),
	}
	if err := h.kycService.SaveDocument(r.Context(), doc); err != nil {
		// Design D2: nothing may remain stored on a rejected request. The file
		// was already written; remove it so a failed metadata write cannot
		// leave an orphaned document on disk (DB outage or constraint error).
		// Delete is idempotent, so a partially-landed file is still cleaned.
		if delErr := h.store.Delete(r.Context(), key); delErr != nil {
			response.Error(w, http.StatusInternalServerError, "failed to record document")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to record document")
		return
	}

	response.JSON(w, http.StatusCreated, doc)
}
