package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.KycDocumentDto
import com.casino.app.data.api.dto.KycSubmissionDto
import okhttp3.MultipartBody
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Multipart
import retrofit2.http.POST
import retrofit2.http.Part

/**
 * Retrofit interface for the frozen KYC contract
 * (POST /api/v1/kyc/submit, GET /kyc/status, POST /kyc/documents).
 * All endpoints require a valid Bearer token (added by [AuthInterceptor]).
 */
interface KycApi {

    /** POST /api/v1/kyc/submit — 201 with the pending submission. */
    @POST("kyc/submit")
    suspend fun submit(@Body body: KycSubmissionDto): ApiEnvelope<KycSubmissionDto>

    /** GET /api/v1/kyc/status — 200 with the submission, 404 when none exists. */
    @GET("kyc/status")
    suspend fun status(): ApiEnvelope<KycSubmissionDto>

    /**
     * POST /api/v1/kyc/documents — multipart upload of the "document" part.
     * The server caps the body at 5 MB (413) and rejects disallowed types
     * (415); a failed response surfaces through [ApiEnvelope.errorMessageFrom].
     */
    @Multipart
    @POST("kyc/documents")
    suspend fun uploadDocument(@Part file: MultipartBody.Part): ApiEnvelope<KycDocumentDto>
}
