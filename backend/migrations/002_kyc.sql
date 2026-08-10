-- KYC schema (migration 002)
-- Adds player identity columns to users plus the KYC submission and document
-- tables. Statements are idempotent so the runner can apply this file on a
-- fresh database or one already initialized by the docker entrypoint.

-- Player identity fields (legal age gate + email verification marker).
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS date_of_birth DATE NOT NULL DEFAULT '1900-01-01',
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

-- KYC submissions: one row per player; admin review lands in a later change,
-- so status stays 'pending' until then.
CREATE TABLE IF NOT EXISTS kyc_submissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id),
    full_name VARCHAR(200),
    date_of_birth DATE,
    nationality VARCHAR(3),
    document_type VARCHAR(20),
    document_number VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    submitted_at TIMESTAMPTZ DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID,
    admin_notes TEXT
);

-- KYC documents: uploads may precede a submission, so submission_id is
-- nullable. Files are stored under generated keys (storage layer concern).
CREATE TABLE IF NOT EXISTS kyc_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    submission_id UUID REFERENCES kyc_submissions(id),
    file_name VARCHAR(255),
    stored_name VARCHAR(255),
    content_type VARCHAR(100),
    size_bytes BIGINT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_kyc_documents_user_id ON kyc_documents(user_id);
