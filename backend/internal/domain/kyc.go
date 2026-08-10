package domain

import (
	"time"

	"github.com/google/uuid"
)

// KycStatus is the lifecycle state of a KYC submission. Admin review lands in a
// later change (casino-admin), so until then every submission stays pending.
type KycStatus string

const (
	KycStatusPending  KycStatus = "pending"
	KycStatusApproved KycStatus = "approved"
	KycStatusRejected KycStatus = "rejected"
)

// Valid reports whether s is one of the three documented KYC statuses. The
// values are pinned to the kyc_submissions.status column (migrations/002_kyc.sql).
func (s KycStatus) Valid() bool {
	switch s {
	case KycStatusPending, KycStatusApproved, KycStatusRejected:
		return true
	default:
		return false
	}
}

// KycSubmission represents a player's identity verification submission. The
// identity fields are submitted in one request (Phase 4) and later editable by
// admin review; the status column is NOT NULL DEFAULT 'pending'.
type KycSubmission struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	FullName       string     `json:"full_name,omitempty"`
	DateOfBirth    *time.Time `json:"date_of_birth,omitempty"`
	Nationality    string     `json:"nationality,omitempty"`
	DocumentType   string     `json:"document_type,omitempty"`
	DocumentNumber string     `json:"document_number,omitempty"`
	Status         KycStatus  `json:"status"`
	SubmittedAt    time.Time  `json:"submitted_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy     *uuid.UUID `json:"reviewed_by,omitempty"`
	AdminNotes     string     `json:"admin_notes,omitempty"`
}

// KycDocument represents an uploaded identity document. Uploads may precede a
// submission, so SubmissionID is nullable; files are stored under generated
// keys (storage layer concern), never the user-supplied filename.
type KycDocument struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	SubmissionID *uuid.UUID `json:"submission_id,omitempty"`
	FileName     string     `json:"file_name"`
	StoredName   string     `json:"stored_name"`
	ContentType  string     `json:"content_type"`
	SizeBytes    int64      `json:"size_bytes"`
	CreatedAt    time.Time  `json:"created_at"`
}
