package com.casino.app.data.api.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for the KYC endpoints (POST /api/v1/kyc/submit, GET /kyc/status,
 * POST /kyc/documents).
 *
 * Field names and JSON keys mirror the frozen backend contract
 * (`internal/domain/kyc.go`).
 */

/**
 * A KYC identity submission. Doubles as the request payload (only the identity
 * fields are relevant when submitting) and the response data of submit/status
 * (server-populated id, user_id, status, submitted_at).
 */
@Serializable
data class KycSubmissionDto(
    val id: String = "",
    @SerialName("user_id") val userId: String = "",
    @SerialName("full_name") val fullName: String = "",
    /** RFC 3339 timestamp or null; optional at submit time. */
    @SerialName("date_of_birth") val dateOfBirth: String? = null,
    val nationality: String = "",
    @SerialName("document_type") val documentType: String = "",
    @SerialName("document_number") val documentNumber: String = "",
    /** pending | approved | rejected — admin review is a later change. */
    val status: String = "pending",
    @SerialName("submitted_at") val submittedAt: String = "",
    @SerialName("reviewed_at") val reviewedAt: String? = null,
    @SerialName("reviewed_by") val reviewedBy: String? = null,
    @SerialName("admin_notes") val adminNotes: String = "",
)

/** Response data of POST /kyc/documents — metadata of a stored upload. */
@Serializable
data class KycDocumentDto(
    val id: String,
    @SerialName("user_id") val userId: String,
    @SerialName("submission_id") val submissionId: String? = null,
    /** The original user-supplied filename (never used as a storage key). */
    @SerialName("file_name") val fileName: String,
    /** Generated storage key: UUID + sanitized extension (design D1). */
    @SerialName("stored_name") val storedName: String,
    @SerialName("content_type") val contentType: String,
    @SerialName("size_bytes") val sizeBytes: Long,
    @SerialName("created_at") val createdAt: String,
)
