package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.LoginRequest
import com.casino.app.data.api.dto.RefreshRequest
import com.casino.app.data.api.dto.RegisterRequest
import com.casino.app.data.api.dto.TokenPair
import com.casino.app.data.api.dto.UserDto
import com.casino.app.data.api.dto.VerifyEmailRequest
import com.casino.app.data.api.dto.VerifyEmailResponse
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.POST

/**
 * Retrofit interface for the frozen auth contract
 * (POST /api/v1/auth/{register,verify,login,refresh,logout}).
 */
interface AuthApi {

    /** POST /api/v1/auth/register — 201 with the created user, no tokens. */
    @POST("auth/register")
    suspend fun register(@Body body: RegisterRequest): ApiEnvelope<UserDto>

    /** POST /api/v1/auth/verify — 200 with `{verified: true}`. */
    @POST("auth/verify")
    suspend fun verify(@Body body: VerifyEmailRequest): ApiEnvelope<VerifyEmailResponse>

    /** POST /api/v1/auth/login — 200 with the access/refresh token pair. */
    @POST("auth/login")
    suspend fun login(@Body body: LoginRequest): ApiEnvelope<TokenPair>

    /** POST /api/v1/auth/refresh — 200 with a rotated token pair. */
    @POST("auth/refresh")
    suspend fun refresh(@Body body: RefreshRequest): ApiEnvelope<TokenPair>

    /**
     * POST /api/v1/auth/logout — 204 No Content with an empty body (the only
     * endpoint that does NOT return the JSON envelope). [Response] is used so
     * non-2xx statuses do not throw; the repository treats logout as best-effort.
     */
    @POST("auth/logout")
    suspend fun logout(@Body body: RefreshRequest): Response<Unit>
}
