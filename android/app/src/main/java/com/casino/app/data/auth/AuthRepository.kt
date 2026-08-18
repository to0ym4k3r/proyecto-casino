package com.casino.app.data.auth

import com.casino.app.data.api.AuthApi
import com.casino.app.data.api.dto.LoginRequest
import com.casino.app.data.api.dto.RefreshRequest
import com.casino.app.data.api.dto.RegisterRequest
import com.casino.app.data.api.dto.VerifyEmailRequest
import com.casino.app.data.token.TokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn

/**
 * Drives the auth session from the token store and the auth API (design D9).
 *
 * [authed] derives directly from [TokenStore.tokens]: saving a pair (login, or
 * a future refresh) flips it to true; clearing it (logout, or an
 * interceptor-initiated 401 clear per design D10) flips it back to false. The
 * navigation gate observes this flow to switch start destinations.
 */
class AuthRepository(
    private val tokenStore: TokenStore,
    private val authApi: AuthApi,
    scope: CoroutineScope,
) {

    /**
     * True while a token pair exists. The initial value is false; the flow
     * converges to the store's current state as soon as DataStore first emits.
     */
    val authed: StateFlow<Boolean> = tokenStore.tokens
        .map { it != null }
        .stateIn(scope, SharingStarted.Eagerly, false)

    /**
     * Cold view of the same session state, WITHOUT the synthetic initial value
     * [authed] carries. Its first emission is always the store's real state, so
     * the navigation gate can hold a "resolving" state until that first
     * emission and then land on the correct graph — a pre-existing session
     * resolves straight to the main tabs with no login flash (deviation #12).
     */
    val session: Flow<Boolean> = tokenStore.tokens.map { it != null }

    /**
     * POST /auth/register and discards the response: registration returns the
     * created (unverified) user, never tokens, so nothing is persisted here.
     * Non-2xx failures (e.g. 409 duplicate, 400 invalid) propagate.
     */
    suspend fun register(request: RegisterRequest) {
        authApi.register(request)
    }

    /**
     * POST /auth/verify with the email token from the verification email. A 200
     * activates the account; invalid/expired tokens surface as
     * [retrofit2.HttpException] 400 and propagate to the UI.
     */
    suspend fun verifyEmail(token: String) {
        authApi.verify(VerifyEmailRequest(token))
    }

    /**
     * POST /auth/login and persists the returned pair on success. Failures
     * (network, or Retrofit [retrofit2.HttpException] for non-2xx) propagate to
     * the caller so the UI can surface the backend's message.
     */
    suspend fun login(email: String, password: String) {
        val envelope = authApi.login(LoginRequest(email, password))
        val pair = requireNotNull(envelope.data) { "login succeeded without a token pair" }
        tokenStore.saveTokens(pair)
    }

    /**
     * Revokes the refresh token server-side (best-effort) and always clears the
     * local store, returning the gate to login regardless of network state.
     */
    suspend fun logout() {
        val refreshToken = tokenStore.refreshToken.first()
        if (refreshToken != null) {
            try {
                authApi.logout(RefreshRequest(refreshToken))
            } catch (_: Exception) {
                // Best-effort: revocation must never block local sign-out.
            }
        }
        tokenStore.clear()
    }
}
