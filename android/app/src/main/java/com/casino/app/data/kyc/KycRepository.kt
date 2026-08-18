package com.casino.app.data.kyc

import com.casino.app.data.api.KycApi
import com.casino.app.data.api.dto.KycDocumentDto
import com.casino.app.data.api.dto.KycSubmissionDto
import okhttp3.MultipartBody

/**
 * Repository for the KYC endpoints (submit, status, document upload).
 *
 * Wraps [KycApi] and unwraps the [ApiEnvelope][com.casino.app.data.api.dto.ApiEnvelope]
 * on success; non-2xx responses propagate as [retrofit2.HttpException].
 */
class KycRepository(private val kycApi: KycApi) {

    /** POST /kyc/submit — returns the newly created pending submission. */
    suspend fun submit(submission: KycSubmissionDto): KycSubmissionDto {
        return kycApi.submit(submission).data ?: error("submit succeeded without data")
    }

    /** GET /kyc/status — returns the current submission status. */
    suspend fun status(): KycSubmissionDto {
        return kycApi.status().data ?: error("status succeeded without data")
    }

    /** POST /kyc/documents — uploads a document and returns its metadata. */
    suspend fun uploadDocument(file: MultipartBody.Part): KycDocumentDto {
        return kycApi.uploadDocument(file).data ?: error("upload succeeded without data")
    }
}
