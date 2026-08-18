package com.casino.app.ui.screens.auth

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.TokenPair
import java.io.IOException
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

/**
 * RED (5.6, extract-before-mock): the shared Throwable → user-facing message
 * mapping used by the login, register and verify screens.
 */
class AuthErrorMessagesTest {

    @Test
    fun `http error surfaces the backend envelope message`() {
        val error = HttpException(
            Response.error<ApiEnvelope<TokenPair>>(
                401,
                """{"success":false,"error":"invalid credentials"}"""
                    .toResponseBody("application/json".toMediaType()),
            ),
        )

        assertEquals("invalid credentials", AuthErrorMessages.from(error, "fallback"))
    }

    @Test
    fun `http error without a parseable envelope falls back`() {
        val error = HttpException(
            Response.error<ApiEnvelope<TokenPair>>(
                500,
                "gateway exploded".toResponseBody("text/plain".toMediaType()),
            ),
        )

        assertEquals("fallback", AuthErrorMessages.from(error, "fallback"))
    }

    @Test
    fun `io error maps to a connection message regardless of fallback`() {
        assertEquals(
            "Network error. Check your connection and try again.",
            AuthErrorMessages.from(IOException("boom"), "fallback"),
        )
    }

    @Test
    fun `unknown failure uses the fallback`() {
        assertEquals("fallback", AuthErrorMessages.from(IllegalStateException("nope"), "fallback"))
    }
}
