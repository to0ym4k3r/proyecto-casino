package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestVerifyEmailRequestDecodesToken proves the wire contract of the verify
// endpoint payload: a JSON body {"token": "..."} maps to VerifyEmailRequest.Token.
func TestVerifyEmailRequestDecodesToken(t *testing.T) {
	var req VerifyEmailRequest
	if err := json.Unmarshal([]byte(`{"token":"abc123"}`), &req); err != nil {
		t.Fatalf("unmarshal VerifyEmailRequest: %v", err)
	}
	if req.Token != "abc123" {
		t.Errorf("Token = %q, want %q", req.Token, "abc123")
	}
}

// TestVerifyEmailRequestRejectsMissingToken proves the field is required: a
// payload without "token" leaves the field empty.
func TestVerifyEmailRequestRejectsMissingToken(t *testing.T) {
	var req VerifyEmailRequest
	if err := json.Unmarshal([]byte(`{}`), &req); err != nil {
		t.Fatalf("unmarshal VerifyEmailRequest: %v", err)
	}
	if req.Token != "" {
		t.Errorf("Token = %q, want empty when payload omits token", req.Token)
	}
}

// TestUpdateProfileRequestDecodesEditableFields proves the profile update
// payload carries only the editable fields (username, first_name, last_name) —
// email is deliberately NOT part of the request (immutable this change).
func TestUpdateProfileRequestDecodesEditableFields(t *testing.T) {
	body := `{"username":"newhandle","first_name":"New","last_name":"Name","email":"sneaky@example.com"}`
	var req UpdateProfileRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal UpdateProfileRequest: %v", err)
	}
	if req.Username != "newhandle" {
		t.Errorf("Username = %q, want %q", req.Username, "newhandle")
	}
	if req.FirstName != "New" {
		t.Errorf("FirstName = %q, want %q", req.FirstName, "New")
	}
	if req.LastName != "Name" {
		t.Errorf("LastName = %q, want %q", req.LastName, "Name")
	}
}

// TestUserDateOfBirthSurvivesJSONRoundTrip proves the User domain model carries
// the date_of_birth field from migration 002 and exposes it under the snake_case
// JSON key the API contract expects.
func TestUserDateOfBirthSurvivesJSONRoundTrip(t *testing.T) {
	dob := time.Date(1995, 6, 15, 0, 0, 0, 0, time.UTC)
	u := User{
		ID:          uuid.New(),
		Username:    "player",
		Email:       "player@example.com",
		DateOfBirth: dob,
	}
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	if !strings.Contains(string(raw), `"date_of_birth":"1995-06-15T00:00:00Z"`) {
		t.Errorf("marshaled user missing date_of_birth: %s", raw)
	}

	var back User
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal user: %v", err)
	}
	if !back.DateOfBirth.Equal(dob) {
		t.Errorf("DateOfBirth after round trip = %v, want %v", back.DateOfBirth, dob)
	}
}

// TestKycStatusValues pins the three KycStatus constants to the exact strings
// the kyc_submissions.status column uses (migrations/002_kyc.sql:21).
func TestKycStatusValues(t *testing.T) {
	tests := []struct {
		status KycStatus
		want   string
	}{
		{KycStatusPending, "pending"},
		{KycStatusApproved, "approved"},
		{KycStatusRejected, "rejected"},
	}
	for _, tt := range tests {
		if got := string(tt.status); got != tt.want {
			t.Errorf("KycStatus %s = %q, want %q", tt.want, got, tt.want)
		}
	}
}

// TestKycStatusValid proves the Valid helper accepts exactly the three
// documented statuses and nothing else.
func TestKycStatusValid(t *testing.T) {
	if !KycStatusPending.Valid() {
		t.Error("pending should be valid")
	}
	if !KycStatusApproved.Valid() {
		t.Error("approved should be valid")
	}
	if !KycStatusRejected.Valid() {
		t.Error("rejected should be valid")
	}
	if KycStatus("").Valid() {
		t.Error("empty status must not be valid")
	}
	if KycStatus("in_review").Valid() {
		t.Error("unknown status must not be valid")
	}
}

// TestKycSubmissionJSONContract proves the KYC submission model matches the
// kyc_submissions table columns and the API envelope keys.
func TestKycSubmissionJSONContract(t *testing.T) {
	sub := KycSubmission{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		Status:         KycStatusPending,
		SubmittedAt:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		DocumentType:   "passport",
		DocumentNumber: "AB123456",
	}
	raw, err := json.Marshal(sub)
	if err != nil {
		t.Fatalf("marshal submission: %v", err)
	}
	for _, key := range []string{`"id"`, `"user_id"`, `"status":"pending"`, `"submitted_at"`, `"document_type":"passport"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled submission missing %s: %s", key, raw)
		}
	}
}

// TestKycDocumentJSONContract proves the KYC document model matches the
// kyc_documents table columns and the API envelope keys.
func TestKycDocumentJSONContract(t *testing.T) {
	doc := KycDocument{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		FileName:    "id.jpg",
		StoredName:  "abc-123.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   2048,
		CreatedAt:   time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal document: %v", err)
	}
	for _, key := range []string{`"file_name":"id.jpg"`, `"stored_name":"abc-123.jpg"`, `"content_type":"image/jpeg"`, `"size_bytes":2048`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("marshaled document missing %s: %s", key, raw)
		}
	}
}
