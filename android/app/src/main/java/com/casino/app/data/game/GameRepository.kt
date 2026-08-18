package com.casino.app.data.game

import com.casino.app.data.api.GameApi
import com.casino.app.data.api.dto.BetResultResponse
import com.casino.app.data.api.dto.EndSessionResponse
import com.casino.app.data.api.dto.GameDto
import com.casino.app.data.api.dto.GameListResponse
import com.casino.app.data.api.dto.PlaceBetRequest
import com.casino.app.data.api.dto.StartGameRequest
import com.casino.app.data.api.dto.StartGameResponse

/**
 * Repository for the game API endpoints.
 *
 * Wraps [GameApi] and unwraps the [ApiEnvelope][com.casino.app.data.api.dto.ApiEnvelope]
 * on success; non-2xx responses propagate as [retrofit2.HttpException].
 */
class GameRepository(private val gameApi: GameApi) {

    /** GET /games — returns a paginated list of active games. */
    suspend fun listGames(limit: Int = 20, offset: Int = 0): GameListResponse {
        return gameApi.listGames(limit, offset).data
            ?: error("list games succeeded without data")
    }

    /** GET /games/{gameId} — returns a single game. */
    suspend fun getGame(gameId: String): GameDto {
        return gameApi.getGame(gameId).data
            ?: error("get game succeeded without data")
    }

    /** POST /games/start — starts a new game session and returns the seed hash. */
    suspend fun startGame(gameId: String): StartGameResponse {
        return gameApi.startGame(StartGameRequest(gameId)).data
            ?: error("start game succeeded without data")
    }

    /** POST /games/{gameId}/bet — places a bet and returns the result. */
    suspend fun placeBet(gameId: String, amount: Long): BetResultResponse {
        return gameApi.placeBet(gameId, PlaceBetRequest(amount)).data
            ?: error("place bet succeeded without data")
    }

    /** POST /games/{gameId}/end — ends the active session. */
    suspend fun endSession(gameId: String): EndSessionResponse {
        return gameApi.endSession(gameId).data
            ?: error("end session succeeded without data")
    }
}
