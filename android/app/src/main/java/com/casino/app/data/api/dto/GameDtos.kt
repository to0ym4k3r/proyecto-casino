package com.casino.app.data.api.dto

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Game DTOs matching the frozen backend contract (internal/domain/game.go).
 *
 * All monetary amounts are in integer cents (e.g. 15000 = $150.00).
 * RTP is an integer 0–100 representing the return-to-player percentage.
 */

/** A game returned by GET /api/v1/games. */
@Serializable
data class GameDto(
    val id: String,
    val name: String,
    val description: String = "",
    val category: String = "",
    @SerialName("min_bet") val minBet: Long,
    @SerialName("max_bet") val maxBet: Long,
    val rtp: Int,
    val status: String = "",
    @SerialName("cover_image") val coverImage: String? = null,
)

/** Paginated game list response from GET /api/v1/games. */
@Serializable
data class GameListResponse(
    val games: List<GameDto>,
    val total: Int,
    val limit: Int,
    val offset: Int,
)

/** Start game request body. */
@Serializable
data class StartGameRequest(
    @SerialName("game_id") val gameId: String,
)

/** Start game response — contains session info and hashed server seed. */
@Serializable
data class StartGameResponse(
    @SerialName("session_id") val sessionId: String,
    @SerialName("server_seed_hash") val serverSeedHash: String,
)

/** Place bet request body. */
@Serializable
data class PlaceBetRequest(
    val amount: Long,
)

/** Bet result response from POST /api/v1/games/{gameId}/bet. */
@Serializable
data class BetResultResponse(
    @SerialName("bet_id") val betId: String,
    val amount: Long,
    val payout: Long,
    val multiplier: Double,
    val won: Boolean,
)

/** End session response. */
@Serializable
data class EndSessionResponse(
    @SerialName("session_id") val sessionId: String,
    val status: String,
)
