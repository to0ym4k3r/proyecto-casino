package com.casino.app.data.profile

import com.casino.app.data.api.UserApi
import com.casino.app.data.api.dto.UpdateProfileRequest
import com.casino.app.data.api.dto.UserDto

/**
 * Repository for the profile endpoints (GET/PUT /api/v1/users/me).
 *
 * Wraps [UserApi] and unwraps the [ApiEnvelope][com.casino.app.data.api.dto.ApiEnvelope]
 * on success; non-2xx responses propagate as [retrofit2.HttpException].
 *
 * Design D6: only username, first_name, and last_name are mutable; email is
 * immutable in this change.
 */
class ProfileRepository(private val userApi: UserApi) {

    /** GET /users/me — returns the authenticated user's profile. */
    suspend fun getProfile(): UserDto {
        return userApi.me().data ?: error("profile succeeded without data")
    }

    /** PUT /users/me — returns the updated profile. */
    suspend fun updateProfile(request: UpdateProfileRequest): UserDto {
        return userApi.updateMe(request).data ?: error("update profile succeeded without data")
    }
}
