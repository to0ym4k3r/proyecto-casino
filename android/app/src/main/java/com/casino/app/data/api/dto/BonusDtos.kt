package com.casino.app.data.api.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Bonus DTOs matching the frozen backend contract (internal/domain/bonus.go).
 *
 * All monetary amounts are in integer cents (e.g. 50000 = $500.00).
 */

/** Bonus definition — a claimable bonus offer from GET /api/v1/bonuses/available. */
@Serializable
data class BonusDefinitionDto(
    val id: String,
    val code: String,
    val name: String,
    val description: String = "",
    @SerialName("wagering_multiplier") val wageringMultiplier: Int,
    @SerialName("expiry_days") val expiryDays: Int,
    @SerialName("max_bonus_amount_cents") val maxBonusAmountCents: Long,
    @SerialName("min_deposit_cents") val minDepositCents: Long,
    @SerialName("is_active") val isActive: Boolean,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
)

/** Request body for POST /api/v1/bonuses/claim. */
@Serializable
data class BonusClaimRequest(
    @SerialName("definition_id") val definitionId: String,
    val params: Map<String, String>? = null,
)

/** Response body for POST /api/v1/bonuses/claim. */
@Serializable
data class BonusClaimResponse(
    @SerialName("grant_id") val grantId: String,
    @SerialName("bonus_amount_cents") val bonusAmountCents: Long,
    @SerialName("wagering_required_cents") val wageringRequiredCents: Long,
    @SerialName("expires_at") val expiresAt: String,
)

/** Active bonus grant from GET /api/v1/bonuses/active. */
@Serializable
data class ActiveBonusDto(
    @SerialName("grant_id") val grantId: String,
    val code: String,
    val name: String,
    @SerialName("bonus_amount_cents") val bonusAmountCents: Long,
    @SerialName("wagering_required_cents") val wageringRequiredCents: Long,
    @SerialName("total_wagered_cents") val totalWageredCents: Long,
    @SerialName("wagering_progress_pct") val wageringProgressPct: Double,
    @SerialName("expires_at") val expiresAt: String,
)
