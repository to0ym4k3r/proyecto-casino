package com.casino.app.data.api.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for the profile endpoints (GET/PUT /api/v1/users/me).
 *
 * Field names and JSON keys mirror the frozen backend contract
 * (`internal/domain/user.go`).
 */

/** The authenticated user's profile (response data of GET and PUT /users/me). */
@Serializable
data class UserDto(
    val id: String,
    val username: String,
    val email: String,
    @SerialName("first_name") val firstName: String,
    @SerialName("last_name") val lastName: String,
    /** RFC 3339 timestamp; the backend stores the birth date as a time value. */
    @SerialName("date_of_birth") val dateOfBirth: String,
    @SerialName("is_active") val isActive: Boolean,
    @SerialName("email_verified_at") val emailVerifiedAt: String? = null,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("deleted_at") val deletedAt: String? = null,
)

/**
 * Payload for PUT /users/me. Only username, first_name and last_name are
 * mutable (design D6); email is immutable. Empty defaults are omitted on the
 * wire (encodeDefaults = false), and the backend merges absent fields as
 * "unchanged" — sending only the fields the user actually edited is the
 * intended contract.
 */
@Serializable
data class UpdateProfileRequest(
    val username: String = "",
    @SerialName("first_name") val firstName: String = "",
    @SerialName("last_name") val lastName: String = "",
)
