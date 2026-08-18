package com.casino.app.data.auth

import app.cash.turbine.test
import com.casino.app.data.api.AuthApi
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.LoginRequest
import com.casino.app.data.api.dto.RefreshRequest
import com.casino.app.data.api.dto.RegisterRequest
import com.casino.app.data.api.dto.TokenPair
import com.casino.app.data.api.dto.UserDto
import com.casino.app.data.api.dto.VerifyEmailRequest
import com.casino.app.data.api.dto.VerifyEmailResponse
import com.casino.app.data.token.TokenStore
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Before
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response
import java.io.IOException

/**
 * RED (5.4): AuthRepository keeps the authenticated state in sync with the
 * token store and drives login/logout against [AuthApi].
 *
 * Design D9: tokens are the single source of truth — `authed` derives from
 * [TokenStore.tokens]. Design D10: a 401 clears the store (via the
 * interceptor), which flips `authed` back to false and sends the app to login.
 *
 * The repository owns its [CoroutineScope] exactly as Hilt supplies it in
 * production; tests cancel it after each case so the `stateIn` sharing
 * coroutine never leaks into the test scope.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class AuthRepositoryTest {

    private val tokenStore = mockk<TokenStore>()
    private val authApi = mockk<AuthApi>()
    private lateinit var appScope: CoroutineScope

    @Before
    fun setUp() {
        // Unconfined: every emission is applied synchronously, keeping the
        // authed flow's value assertions deterministic without virtual time.
        appScope = CoroutineScope(UnconfinedTestDispatcher())
    }

    @After
    fun tearDown() {
        appScope.cancel()
    }

    private fun newRepository() = AuthRepository(tokenStore, authApi, appScope)

    @Test
    fun `authed follows the stored token pair`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(TokenPair("access-1", "refresh-1"))
        every { tokenStore.tokens } returns tokens

        newRepository().authed.test {
            assertEquals(true, awaitItem())
            tokens.value = null
            assertEquals(false, awaitItem())
            tokens.value = TokenPair("access-2", "refresh-2")
            assertEquals(true, awaitItem())
        }
    }

    @Test
    fun `authed starts false when no session exists`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens

        assertEquals(false, newRepository().authed.value)
    }

    @Test
    fun `login saves the returned pair and authenticates`() = runTest {
        val pair = TokenPair("access-9", "refresh-9")
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens
        coEvery {
            authApi.login(LoginRequest("alice@casino.dev", "secret"))
        } returns ApiEnvelope(success = true, data = pair)
        coEvery { tokenStore.saveTokens(pair) } answers { tokens.value = pair }

        val repository = newRepository()
        repository.authed.test {
            assertEquals(false, awaitItem())
            repository.login("alice@casino.dev", "secret")
            assertEquals(true, awaitItem())
        }

        coVerify(exactly = 1) { authApi.login(LoginRequest("alice@casino.dev", "secret")) }
        coVerify(exactly = 1) { tokenStore.saveTokens(pair) }
    }

    @Test
    fun `login failure surfaces the error and saves nothing`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens
        coEvery { authApi.login(any()) } throws HttpException(
            Response.error<ApiEnvelope<TokenPair>>(
                401,
                """{"success":false,"error":"invalid credentials"}""".toResponseBody("application/json".toMediaType()),
            ),
        )

        val repository = newRepository()

        try {
            repository.login("alice@casino.dev", "wrong")
            fail("expected HttpException")
        } catch (e: HttpException) {
            assertEquals(401, e.code())
        }

        coVerify(exactly = 0) { tokenStore.saveTokens(any()) }
        assertEquals(false, repository.authed.value)
    }

    @Test
    fun `logout clears the store even when the server call fails`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(TokenPair("access-1", "refresh-1"))
        every { tokenStore.tokens } returns tokens
        every { tokenStore.refreshToken } returns MutableStateFlow("refresh-1")
        coEvery { authApi.logout(RefreshRequest("refresh-1")) } throws IOException("network down")
        coEvery { tokenStore.clear() } answers { tokens.value = null }

        newRepository().logout()

        coVerify(exactly = 1) { authApi.logout(RefreshRequest("refresh-1")) }
        coVerify(exactly = 1) { tokenStore.clear() }
    }

    @Test
    fun `session first emission is the real store state, not a synthetic initial value`() = runTest {
        // The navigation gate (5.5) must be able to distinguish "not resolved
        // yet" from "authenticated". Unlike `authed` (stateIn initial false),
        // `session` must emit the true state of the store on its first emission:
        // a pre-existing pair resolves straight to true with no login flash.
        val tokens = MutableStateFlow<TokenPair?>(TokenPair("access-1", "refresh-1"))
        every { tokenStore.tokens } returns tokens

        newRepository().session.test {
            assertEquals(true, awaitItem())
        }
    }

    @Test
    fun `session tracks a cleared store back to false`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(TokenPair("access-1", "refresh-1"))
        every { tokenStore.tokens } returns tokens

        newRepository().session.test {
            assertEquals(true, awaitItem())
            tokens.value = null
            assertEquals(false, awaitItem())
        }
    }

    @Test
    fun `register delegates the request and saves no tokens`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens
        val request = RegisterRequest(
            username = "alice",
            email = "alice@casino.dev",
            password = "secret-pass",
            firstName = "Alice",
            lastName = "Smith",
            dateOfBirth = "1998-04-02",
        )
        coEvery { authApi.register(request) } returns ApiEnvelope(
            success = true,
            data = UserDto(
                id = "u1",
                username = "alice",
                email = "alice@casino.dev",
                firstName = "Alice",
                lastName = "Smith",
                dateOfBirth = "1998-04-02",
                isActive = false,
                emailVerifiedAt = null,
                createdAt = "2026-01-01T00:00:00Z",
                updatedAt = "2026-01-01T00:00:00Z",
            ),
        )

        newRepository().register(request)

        coVerify(exactly = 1) { authApi.register(request) }
        // Register returns the user, never a token pair (backend contract: no
        // auto-login while unverified) — nothing may be persisted.
        coVerify(exactly = 0) { tokenStore.saveTokens(any()) }
    }

    @Test
    fun `register failure propagates and saves nothing`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens
        coEvery { authApi.register(any()) } throws HttpException(
            Response.error<ApiEnvelope<UserDto>>(
                409,
                """{"success":false,"error":"username already taken"}"""
                    .toResponseBody("application/json".toMediaType()),
            ),
        )

        val repository = newRepository()

        try {
            repository.register(
                RegisterRequest("alice", "alice@casino.dev", "secret-pass", "Alice", "Smith", "1998-04-02"),
            )
            fail("expected HttpException")
        } catch (e: HttpException) {
            assertEquals(409, e.code())
        }

        coVerify(exactly = 0) { tokenStore.saveTokens(any()) }
    }

    @Test
    fun `verifyEmail delegates the token`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens
        coEvery { authApi.verify(VerifyEmailRequest("tok-123")) } returns ApiEnvelope(
            success = true,
            data = VerifyEmailResponse(verified = true),
        )

        newRepository().verifyEmail("tok-123")

        coVerify(exactly = 1) { authApi.verify(VerifyEmailRequest("tok-123")) }
    }

    @Test
    fun `verifyEmail failure propagates`() = runTest {
        val tokens = MutableStateFlow<TokenPair?>(null)
        every { tokenStore.tokens } returns tokens
        coEvery { authApi.verify(any()) } throws HttpException(
            Response.error<ApiEnvelope<VerifyEmailResponse>>(
                400,
                """{"success":false,"error":"invalid or expired token"}"""
                    .toResponseBody("application/json".toMediaType()),
            ),
        )

        val repository = newRepository()

        try {
            repository.verifyEmail("expired")
            fail("expected HttpException")
        } catch (e: HttpException) {
            assertEquals(400, e.code())
        }
    }
}
