package com.casino.app.data.api

import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.BetResultResponse
import com.casino.app.data.api.dto.EndSessionResponse
import com.casino.app.data.api.dto.GameDto
import com.casino.app.data.api.dto.GameListResponse
import com.casino.app.data.api.dto.PlaceBetRequest
import com.casino.app.data.api.dto.StartGameRequest
import com.casino.app.data.api.dto.StartGameResponse
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Retrofit interface for the game API endpoints.
 *
 * Covers:
 * - GET /api/v1/games — paginated game listing
 * - GET /api/v1/games/{gameId} — single game detail
 * - POST /api/v1/games/start — start a new game session
 * - POST /api/v1/games/{gameId}/bet — place a bet in a session
 * - POST /api/v1/games/{gameId}/end — end a game session
 */
interface GameApi {

    /** GET /api/v1/games — returns a paginated list of active games. */
    @GET("games")
    suspend fun listGames(
        @Query("limit") limit: Int = 20,
        @Query("offset") offset: Int = 0,
    ): ApiEnvelope<GameListResponse>

    /** GET /api/v1/games/{gameId} — returns a single game. */
    @GET("games/{gameId}")
    suspend fun getGame(
        @Path("gameId") gameId: String,
    ): ApiEnvelope<GameDto>

    /** POST /api/v1/games/start — starts a new game session. */
    @POST("games/start")
    suspend fun startGame(
        @Body body: StartGameRequest,
    ): ApiEnvelope<StartGameResponse>

    /** POST /api/v1/games/{gameId}/bet — places a bet in the active session. */
    @POST("games/{gameId}/bet")
    suspend fun placeBet(
        @Path("gameId") gameId: String,
        @Body body: PlaceBetRequest,
    ): ApiEnvelope<BetResultResponse>

    /** POST /api/v1/games/{gameId}/end — ends the active session. */
    @POST("games/{gameId}/end")
    suspend fun endSession(
        @Path("gameId") gameId: String,
    ): ApiEnvelope<EndSessionResponse>
}
