package com.casino.app.ui.screens.auth

import com.casino.app.data.api.dto.errorMessageFrom
import java.io.IOException
import retrofit2.HttpException

/**
 * Shared mapping from the exceptions the auth layer throws to user-facing
 * messages (design: login/register/verify screens expose error states).
 *
 * - [HttpException]: the backend's envelope `error` message when the body
 *   parses, otherwise the caller's fallback.
 * - [IOException]: a connectivity problem, always a connection message.
 * - Anything else: the caller's fallback.
 */
object AuthErrorMessages {

    fun from(throwable: Throwable, fallback: String): String {
        if (throwable is HttpException) {
            val body = throwable.response()?.errorBody()?.string()
            return errorMessageFrom(body) ?: fallback
        }
        if (throwable is IOException) {
            return "Network error. Check your connection and try again."
        }
        return fallback
    }
}
