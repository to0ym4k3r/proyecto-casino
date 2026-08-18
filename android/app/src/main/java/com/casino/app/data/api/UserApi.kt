package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.UpdateProfileRequest
import com.casino.app.data.api.dto.UserDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.PUT

/**
 * Retrofit interface for the frozen profile contract (GET/PUT /api/v1/users/me).
 * Both endpoints require a valid Bearer token (added by [AuthInterceptor]).
 */
interface UserApi {

    /** GET /api/v1/users/me — 200 with the authenticated user's profile. */
    @GET("users/me")
    suspend fun me(): ApiEnvelope<UserDto>

    /** PUT /api/v1/users/me — 200 with the updated profile. */
    @PUT("users/me")
    suspend fun updateMe(@Body body: UpdateProfileRequest): ApiEnvelope<UserDto>
}
