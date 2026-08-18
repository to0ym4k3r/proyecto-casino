package com.casino.app.data.api.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * DTOs for the auth endpoints (POST /api/v1/auth/register, /verify, /login,
 * /refresh, /logout).
 *
 * Field names and JSON keys mirror the frozen backend contract exactly
 * (`internal/domain/user.go`); snake_case keys are mapped via [SerialName].
 */

/** Payload for POST /api/v1/auth/register. */
@Serializable
data class RegisterRequest(
    val username: String,
    val email: String,
    val password: String,
    @SerialName("first_name") val firstName: String,
    @SerialName("last_name") val lastName: String,
    /** Birth date as "2006-01-02"; parsed and age-gated (>= 18) server-side. */
    @SerialName("date_of_birth") val dateOfBirth: String,
)

/** Payload for POST /api/v1/auth/login. */
@Serializable
data class LoginRequest(
    val email: String,
    val password: String,
)

/** Payload for POST /api/v1/auth/refresh and /auth/logout. */
@Serializable
data class RefreshRequest(
    @SerialName("refresh_token") val refreshToken: String,
)

/** Payload for POST /api/v1/auth/verify. */
@Serializable
data class VerifyEmailRequest(
    val token: String,
)

/** Response data of POST /api/v1/auth/login and /auth/refresh. */
@Serializable
data class TokenPair(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String,
)

/** Response data of POST /api/v1/auth/verify. */
@Serializable
data class VerifyEmailResponse(
    val verified: Boolean,
)
