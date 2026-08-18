package com.casino.app.data.api

import androidx.datastore.preferences.core.PreferenceDataStoreFactory
import com.casino.app.data.api.dto.TokenPair
import com.casino.app.data.token.TokenStore
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import java.io.File

/**
 * RED (5.4): AuthInterceptor attaches the bearer token to every request and,
 * on a 401 (design D10), clears the token store so the auth gate returns to
 * login. Auto-refresh is deferred — no retry happens here.
 *
 * The interceptor is exercised against a real temp-file DataStore-backed
 * [TokenStore] and a real OkHttp client, so this test proves the full
 * interceptor/store wiring, not a mock of it.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class AuthInterceptorTest {

    private lateinit var server: MockWebServer
    private lateinit var file: File
    private lateinit var scope: CoroutineScope
    private lateinit var store: TokenStore
    private lateinit var client: OkHttpClient

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        file = File.createTempFile("auth_interceptor_test", ".preferences_pb").apply { delete() }
        scope = CoroutineScope(UnconfinedTestDispatcher())
        store = TokenStore(PreferenceDataStoreFactory.create(scope = scope, produceFile = { file }))
        client = OkHttpClient.Builder()
            .addInterceptor(AuthInterceptor(store))
            .build()
    }

    @After
    fun tearDown() {
        scope.cancel()
        server.shutdown()
        file.delete()
    }

    private fun newCall() = client.newCall(Request.Builder().url(server.url("/")).build())

    @Test
    fun `adds the bearer header when tokens are saved`() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("ok"))
        runBlocking { store.saveTokens(TokenPair("access-1", "refresh-1")) }

        newCall().execute().use { assertEquals(200, it.code) }

        assertEquals("Bearer access-1", server.takeRequest().getHeader("Authorization"))
    }

    @Test
    fun `omits the authorization header when no session exists`() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("ok"))

        newCall().execute().use { assertEquals(200, it.code) }

        assertNull(server.takeRequest().getHeader("Authorization"))
    }

    @Test
    fun `401 clears the stored tokens and propagates the status`() {
        server.enqueue(MockResponse().setResponseCode(401))
        runBlocking { store.saveTokens(TokenPair("access-2", "refresh-2")) }

        newCall().execute().use { assertEquals(401, it.code) }

        assertNull(runBlocking { store.accessToken.first() })
        assertNull(runBlocking { store.refreshToken.first() })
    }

    @Test
    fun `non-401 responses leave the tokens untouched`() {
        server.enqueue(MockResponse().setResponseCode(200).setBody("ok"))
        runBlocking { store.saveTokens(TokenPair("access-3", "refresh-3")) }

        newCall().execute().use { assertEquals(200, it.code) }

        assertEquals("access-3", runBlocking { store.accessToken.first() })
    }
}
