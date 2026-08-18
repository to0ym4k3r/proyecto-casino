package com.casino.app.data.profile

import com.casino.app.data.api.UserApi
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.UpdateProfileRequest
import com.casino.app.data.api.dto.UserDto
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.ResponseBody.Companion.toResponseBody
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import kotlinx.serialization.json.Json

/**
 * RED (6.1): ProfileRepository wraps UserApi and exposes getProfile / updateProfile.
 *
 * These tests prove the frozen profile contract (GET/PUT /api/v1/users/me):
 * - GET returns a full UserDto on 200
 * - PUT returns the updated profile
 * - 401 propagates as HttpException
 */
class ProfileRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var userApi: UserApi
    private lateinit var repository: ProfileRepository

    private val json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
        encodeDefaults = false
    }

    private val successEnvelope = """{"success":true,"data":{"id":"u-1","username":"alice","email":"alice@casino.dev",""" +
        """"first_name":"Alice","last_name":"Wong","date_of_birth":"1998-03-12T00:00:00Z",""" +
        """"is_active":true,"created_at":"2026-08-01T10:00:00Z","updated_at":"2026-08-01T10:00:00Z"}}"""

    private val updatedEnvelope = """{"success":true,"data":{"id":"u-1","username":"neo_alice","email":"alice@casino.dev",""" +
        """"first_name":"Alice","last_name":"Wong","date_of_birth":"1998-03-12T00:00:00Z",""" +
        """"is_active":true,"created_at":"2026-08-01T10:00:00Z","updated_at":"2026-08-05T12:00:00Z"}}"""

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        val retrofit = Retrofit.Builder()
            .baseUrl(server.url("/api/v1/"))
            .client(OkHttpClient.Builder().build())
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
        userApi = retrofit.create(UserApi::class.java)
        repository = ProfileRepository(userApi)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `getProfile returns a full UserDto on success`() = runTest {
        server.enqueue(MockResponse().setResponseCode(200).setBody(successEnvelope))

        val user = repository.getProfile()

        assertEquals("u-1", user.id)
        assertEquals("alice", user.username)
        assertEquals("alice@casino.dev", user.email)
        assertEquals("Alice", user.firstName)
        assertEquals("Wong", user.lastName)
        assertEquals("1998-03-12T00:00:00Z", user.dateOfBirth)

        val recorded = server.takeRequest()
        assertEquals("GET", recorded.method)
        assertEquals("/api/v1/users/me", recorded.path)
    }

    @Test
    fun `updateProfile returns the updated profile`() = runTest {
        server.enqueue(MockResponse().setResponseCode(200).setBody(updatedEnvelope))

        val user = repository.updateProfile(UpdateProfileRequest(username = "neo_alice"))

        assertEquals("neo_alice", user.username)

        val recorded = server.takeRequest()
        assertEquals("PUT", recorded.method)
        assertEquals("/api/v1/users/me", recorded.path)
        assertTrue(recorded.body.readUtf8().contains("\"username\":\"neo_alice\""))
    }

    @Test
    fun `getProfile propagates 401 as HttpException`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(401).setBody(
                """{"success":false,"error":"unauthorized"}"""
            )
        )

        var caught = false
        try {
            repository.getProfile()
        } catch (e: HttpException) {
            caught = true
            assertEquals(401, e.code())
        }
        assertTrue("401 must propagate as HttpException", caught)
    }
}
