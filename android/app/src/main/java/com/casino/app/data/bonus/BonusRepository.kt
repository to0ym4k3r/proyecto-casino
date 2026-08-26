package com.casino.app.data.bonus

import com.casino.app.data.api.BonusApi
import com.casino.app.data.api.dto.ActiveBonusDto
import com.casino.app.data.api.dto.BonusClaimRequest
import com.casino.app.data.api.dto.BonusClaimResponse
import com.casino.app.data.api.dto.BonusDefinitionDto

/**
 * Repository for the bonus API endpoints (task 6.3).
 *
 * Wraps [BonusApi] and unwraps the [ApiEnvelope][com.casino.app.data.api.dto.ApiEnvelope]
 * on success; non-2xx responses propagate as [retrofit2.HttpException].
 *
 * Design decisions:
 * - All monetary amounts are in integer cents (e.g. 50000 = $500.00).
 * - The backend returns empty data arrays/lists as success with empty list.
 * - Forfeit uses a path parameter (grantId in the URL path).
 */
class BonusRepository(private val bonusApi: BonusApi) {

    /** GET /bonuses/available — returns all claimable bonus definitions. */
    suspend fun getAvailableBonuses(): List<BonusDefinitionDto> {
        return bonusApi.getAvailableBonuses().data
            ?: error("available bonuses succeeded without data")
    }

    /** POST /bonuses/claim — claim a bonus by its definition ID. */
    suspend fun claimBonus(definitionId: String, params: Map<String, String>? = null): BonusClaimResponse {
        return bonusApi.claimBonus(BonusClaimRequest(definitionId, params)).data
            ?: error("claim bonus succeeded without data")
    }

    /** GET /bonuses/active — returns the user's active bonus grants with wagering progress. */
    suspend fun getActiveBonuses(): List<ActiveBonusDto> {
        return bonusApi.getActiveBonuses().data
            ?: error("active bonuses succeeded without data")
    }

    /** POST /bonuses/forfeit/{grantId} — forfeit an active bonus grant. */
    suspend fun forfeitBonus(grantId: String) {
        bonusApi.forfeitBonus(grantId)
    }
}
