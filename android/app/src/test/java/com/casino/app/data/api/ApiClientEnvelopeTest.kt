package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.LoginRequest
import com.casino.app.data.api.dto.TokenPair
import com.casino.app.data.api.dto.UpdateProfileRequest
import com.casino.app.data.api.dto.UserDto
import com.casino.app.data.api.dto.VerifyEmailResponse
import com.casino.app.data.api.dto.errorMessageFrom
import kotlinx.serialization.json.Json
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

/**
 * MockWebServer contract tests for the Retrofit + kotlinx.serialization network
 * layer (task 5.2).
 *
 * These prove, against the frozen backend contract (casino-core-users), that:
 *  - the `{success, data, error}` envelope deserializes into DTOs on success,
 *  - the error envelope parses into a readable backend message on failure,
 *  - an oversized KYC upload surfaces the server 413 error,
 *  - the request wire format matches the backend (snake_case, defaults omitted).
 *
 * The Retrofit client here mirrors [com.casino.app.di.AppModule]'s wiring: the
 * tests exercise the REAL OkHttp + Retrofit + kotlinx-serialization stack with
 * the same Json configuration, so a green suite proves the production pipeline.
 */
class ApiClientEnvelopeTest {

    private lateinit var server: MockWebServer
    private lateinit var authApi: AuthApi
    private lateinit var userApi: UserApi
    private lateinit var kycApi: KycApi

    /** Mirrors the Json configuration provided in [com.casino.app.di.AppModule]. */
    private val json: Json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
        encodeDefaults = false
    }

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        val retrofit = Retrofit.Builder()
            .baseUrl(server.url("/api/v1/"))
            .client(OkHttpClient.Builder().build())
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
        authApi = retrofit.create(AuthApi::class.java)
        userApi = retrofit.create(UserApi::class.java)
        kycApi = retrofit.create(KycApi::class.java)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `login success envelope maps to a token pair`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(200).setBody(
                """{"success":true,"data":{"access_token":"access-abc","refresh_token":"refresh-xyz"}}"""
            )
        )

        val envelope = authApi.login(LoginRequest(email = "alice@casino.dev", password = "secret123"))

        assertTrue("envelope must be marked successful", envelope.success)
        val pair = envelope.data
        assertEquals("access-abc", pair?.accessToken)
        assertEquals("refresh-xyz", pair?.refreshToken)

        val recorded = server.takeRequest()
        assertEquals("POST", recorded.method)
        assertEquals("/api/v1/auth/login", recorded.path)
        assertTrue(recorded.body.readUtf8().contains("\"email\""))
    }

    @Test
    fun `login failure envelope surfaces the backend error message`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(401).setBody(
                """{"success":false,"error":"invalid credentials"}"""
            )
        )

        val ex = try {
            authApi.login(LoginRequest(email = "alice@casino.dev", password = "wrong"))
            null
        } catch (e: HttpException) {
            e
        }

        assertNotNull("login with wrong credentials must throw HttpException", ex)
        assertEquals(401, ex?.code())
        val backendMessage = errorMessageFrom(ex?.response()?.errorBody()?.string())
        assertEquals("invalid credentials", backendMessage)
    }

    @Test
    fun `oversized kyc upload surfaces the 413 error`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(413).setBody(
                """{"success":false,"error":"file exceeds the 5 MB limit"}"""
            )
        )
        val part = MultipartBody.Part.createFormData(
            "document", "id-card.jpg", "fake-jpeg".toRequestBody("image/jpeg".toMediaType())
        )

        val ex = try {
            kycApi.uploadDocument(part)
            null
        } catch (e: HttpException) {
            e
        }

        assertNotNull("oversized upload must throw HttpException", ex)
        assertEquals(413, ex?.code())
        assertEquals("file exceeds the 5 MB limit", errorMessageFrom(ex?.response()?.errorBody()?.string()))
    }

    @Test
    fun `verify success envelope maps to verified true`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(200).setBody(
                """{"success":true,"data":{"verified":true}}"""
            )
        )

        val envelope = authApi.verify(com.casino.app.data.api.dto.VerifyEmailRequest(token = "tok-123"))

        assertTrue(envelope.success)
        assertEquals(VerifyEmailResponse(verified = true), envelope.data)
    }

    @Test
    fun `profile success envelope maps to a full user dto`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(200).setBody(
                """{"success":true,"data":{"id":"u-1","username":"alice","email":"alice@casino.dev",""" +
                    """"first_name":"Alice","last_name":"Wong","date_of_birth":"1998-03-12T00:00:00Z",""" +
                    """"is_active":true,"created_at":"2026-08-01T10:00:00Z","updated_at":"2026-08-01T10:00:00Z"}}"""
            )
        )

        val envelope = userApi.me()

        assertTrue(envelope.success)
        val user = envelope.data
        assertEquals("u-1", user?.id)
        assertEquals("alice", user?.username)
        assertEquals("Alice", user?.firstName)
        assertEquals("Wong", user?.lastName)
        assertEquals("1998-03-12T00:00:00Z", user?.dateOfBirth)
        assertEquals(true, user?.isActive)
        assertNull("email_verified_at is omitempty on the backend and absent here", user?.emailVerifiedAt)
    }

    @Test
    fun `update profile request omits unchanged defaults on the wire`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(200).setBody(
                """{"success":true,"data":{"id":"u-1","username":"neo_alice","email":"alice@casino.dev",""" +
                    """"first_name":"Alice","last_name":"Wong","date_of_birth":"1998-03-12T00:00:00Z",""" +
                    """"is_active":true,"created_at":"2026-08-01T10:00:00Z","updated_at":"2026-08-01T10:00:00Z"}}"""
            )
        )

        userApi.updateMe(UpdateProfileRequest(username = "neo_alice"))

        val recorded = server.takeRequest()
        assertEquals("PUT", recorded.method)
        assertEquals("/api/v1/users/me", recorded.path)
        val body = recorded.body.readUtf8()
        assertTrue("request must carry the changed field", body.contains("\"username\":\"neo_alice\""))
        assertTrue(
            "unchanged empty defaults must be omitted (backend merges absent fields)",
            !body.contains("first_name")
        )
    }

    @Test
    fun `blank error body yields no backend message`() = runTest {
        assertNull(errorMessageFrom(null))
        assertNull(errorMessageFrom(""))
    }
}
