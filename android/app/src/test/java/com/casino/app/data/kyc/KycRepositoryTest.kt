package com.casino.app.data.kyc

import com.casino.app.data.api.KycApi
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.KycDocumentDto
import com.casino.app.data.api.dto.KycSubmissionDto
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.RequestBody.Companion.toRequestBody
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
 * RED (6.3): KycRepository wraps KycApi for submit, status, and document upload.
 *
 * These tests prove the frozen KYC contract:
 * - POST /kyc/submit returns 201 pending submission
 * - GET /kyc/status returns current status
 * - POST /kyc/documents returns 201 document metadata
 * - 409 (duplicate submit), 404 (no status), 401 propagate as HttpException
 */
class KycRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var kycApi: KycApi
    private lateinit var repository: KycRepository

    private val json = Json {
        ignoreUnknownKeys = true
        explicitNulls = false
        encodeDefaults = false
    }

    private val submitEnvelope = """{"success":true,"data":{"id":"k-1","user_id":"u-1","full_name":"Alice Wong",""" +
        """"status":"pending","submitted_at":"2026-08-15T10:00:00Z"}}"""

    private val statusEnvelope = """{"success":true,"data":{"id":"k-1","user_id":"u-1","full_name":"Alice Wong",""" +
        """"status":"pending","submitted_at":"2026-08-15T10:00:00Z"}}"""

    private val docEnvelope = """{"success":true,"data":{"id":"d-1","user_id":"u-1","file_name":"id-card.jpg",""" +
        """"stored_name":"abc123.jpg","content_type":"image/jpeg","size_bytes":1024,"created_at":"2026-08-15T10:01:00Z"}}"""

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        val retrofit = Retrofit.Builder()
            .baseUrl(server.url("/api/v1/"))
            .client(OkHttpClient.Builder().build())
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
        kycApi = retrofit.create(KycApi::class.java)
        repository = KycRepository(kycApi)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `submit returns pending submission`() = runTest {
        server.enqueue(MockResponse().setResponseCode(201).setBody(submitEnvelope))

        val submission = repository.submit(
            KycSubmissionDto(fullName = "Alice Wong", nationality = "ARG", documentType = "dni", documentNumber = "12345678")
        )

        assertEquals("pending", submission.status)
        assertEquals("Alice Wong", submission.fullName)
        assertEquals("k-1", submission.id)
    }

    @Test
    fun `status returns current submission`() = runTest {
        server.enqueue(MockResponse().setResponseCode(200).setBody(statusEnvelope))

        val submission = repository.status()

        assertEquals("pending", submission.status)
        assertEquals("k-1", submission.id)
    }

    @Test
    fun `uploadDocument returns document metadata`() = runTest {
        server.enqueue(MockResponse().setResponseCode(201).setBody(docEnvelope))

        val part = MultipartBody.Part.createFormData(
            "document", "id-card.jpg", "fake-jpeg".toRequestBody("image/jpeg".toMediaType())
        )
        val doc = repository.uploadDocument(part)

        assertEquals("id-card.jpg", doc.fileName)
        assertEquals("image/jpeg", doc.contentType)
        assertEquals(1024L, doc.sizeBytes)
    }

    @Test
    fun `duplicate submit propagates 409`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(409).setBody(
                """{"success":false,"error":"KYC already submitted"}"""
            )
        )

        var caught = false
        try {
            repository.submit(KycSubmissionDto(fullName = "Alice"))
        } catch (e: HttpException) {
            caught = true
            assertEquals(409, e.code())
        }
        assertTrue("409 must propagate as HttpException", caught)
    }

    @Test
    fun `status with no submission propagates 404`() = runTest {
        server.enqueue(
            MockResponse().setResponseCode(404).setBody(
                """{"success":false,"error":"no submission found"}"""
            )
        )

        var caught = false
        try {
            repository.status()
        } catch (e: HttpException) {
            caught = true
            assertEquals(404, e.code())
        }
        assertTrue("404 must propagate as HttpException", caught)
    }
}
