package com.casino.app.data.api

import com.casino.app.data.token.TokenStore
import java.net.HttpURLConnection
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Attaches the bearer token to every outgoing request and enforces design D10.
 *
 * A 401 (access token expired or revoked server-side) clears the token store —
 * no automatic retry and no auto-refresh (deferred) — which flips
 * [com.casino.app.data.auth.AuthRepository.authed] to false and returns the
 * app to the login gate. The interceptor runs on OkHttp background threads, so
 * the short blocking read of the DataStore flow is safe here.
 */
class AuthInterceptor(private val tokenStore: TokenStore) : Interceptor {

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val token = runBlocking { tokenStore.accessToken.first() }
        val authorizedRequest = if (token != null) {
            request.newBuilder().header("Authorization", "Bearer $token").build()
        } else {
            request
        }
        val response = chain.proceed(authorizedRequest)
        if (response.code == HttpURLConnection.HTTP_UNAUTHORIZED) {
            runBlocking { tokenStore.clear() }
        }
        return response
    }
}
