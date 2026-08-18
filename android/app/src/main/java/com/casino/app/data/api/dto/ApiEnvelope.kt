package com.casino.app.data.api.dto

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement

/**
 * Standard response envelope returned by every casino API endpoint.
 *
 * Matches the frozen backend contract (`pkg/response.APIResponse`): on success
 * `success` is true and `data` carries the payload; on failure `success` is
 * false and `error` carries the human-readable message. `data` and `error` are
 * omitted on the opposite branch (Go `omitempty`), so both default to null.
 *
 * @param T the DTO type carried by `data` on a successful response.
 */
@Serializable
data class ApiEnvelope<T>(
    val success: Boolean,
    val data: T? = null,
    val error: String? = null,
)

private val errorEnvelopeJson = Json { ignoreUnknownKeys = true }

/**
 * Extracts the backend's human-readable error message from a failed response
 * body, or null when the body is absent, blank, or not an error envelope.
 *
 * Non-2xx Retrofit responses throw [retrofit2.HttpException] before the body
 * converter runs, so callers parse the raw error body with this helper to
 * surface the message the backend actually sent (e.g. "invalid credentials",
 * "file exceeds the 5 MB limit").
 */
fun errorMessageFrom(errorBody: String?): String? {
    if (errorBody.isNullOrBlank()) return null
    return runCatching {
        errorEnvelopeJson.decodeFromString<ApiEnvelope<JsonElement>>(errorBody).error
    }.getOrNull()
}
