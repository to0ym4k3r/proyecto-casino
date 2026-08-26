package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.BonusDefinitionDto
import com.casino.app.data.api.dto.BonusClaimRequest
import com.casino.app.data.api.dto.BonusClaimResponse
import com.casino.app.data.api.dto.ActiveBonusDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path

/**
 * Retrofit interface for the bonus API (tasks 6.1).
 *
 * Covers:
 * - GET  /api/v1/bonuses/available — list claimable bonus definitions
 * - POST /api/v1/bonuses/claim     — claim a bonus by definition ID
 * - GET  /api/v1/bonuses/active    — list active bonus grants with wagering progress
 * - POST /api/v1/bonuses/forfeit   — forfeit an active bonus grant
 */
interface BonusApi {

    /** GET /api/v1/bonuses/available — returns all claimable bonus definitions. */
    @GET("bonuses/available")
    suspend fun getAvailableBonuses(): ApiEnvelope<List<BonusDefinitionDto>>

    /** POST /api/v1/bonuses/claim — claim a bonus by its definition ID. */
    @POST("bonuses/claim")
    suspend fun claimBonus(@Body body: BonusClaimRequest): ApiEnvelope<BonusClaimResponse>

    /** GET /api/v1/bonuses/active — returns the user's active bonus grants with wagering progress. */
    @GET("bonuses/active")
    suspend fun getActiveBonuses(): ApiEnvelope<List<ActiveBonusDto>>

    /** POST /api/v1/bonuses/forfeit/{grantId} — forfeit an active bonus grant. */
    @POST("bonuses/forfeit/{grantId}")
    suspend fun forfeitBonus(@Path("grantId") grantId: String): ApiEnvelope<Unit>
}
