package com.casino.app.data.game

import com.casino.app.data.api.GameApi
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.BetResultResponse
import com.casino.app.data.api.dto.EndSessionResponse
import com.casino.app.data.api.dto.GameDto
import com.casino.app.data.api.dto.GameListResponse
import com.casino.app.data.api.dto.PlaceBetRequest
import com.casino.app.data.api.dto.StartGameRequest
import com.casino.app.data.api.dto.StartGameResponse
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
class GameRepositoryTest {

    private lateinit var gameApi: GameApi
    private lateinit var repository: GameRepository

    private val testGame = GameDto(
        id = "game-1",
        name = "Lucky Slots",
        description = "A fun slot game",
        category = "slots",
        minBet = 100L,
        maxBet = 50000L,
        rtp = 95,
        status = "active",
        coverImage = "https://example.com/slots.png",
    )

    private val testGame2 = GameDto(
        id = "game-2",
        name = "Poker Royal",
        description = "Classic poker",
        category = "poker",
        minBet = 500L,
        maxBet = 100000L,
        rtp = 97,
        status = "active",
    )

    private val testListResponse = GameListResponse(
        games = listOf(testGame, testGame2),
        total = 2,
        limit = 20,
        offset = 0,
    )

    private val testStartResponse = StartGameResponse(
        sessionId = "session-1",
        serverSeedHash = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
    )

    private val testBetResponse = BetResultResponse(
        betId = "bet-1",
        amount = 500L,
        payout = 750L,
        multiplier = 1.5,
        won = true,
    )

    private val testEndResponse = EndSessionResponse(
        sessionId = "session-1",
        status = "completed",
    )

    @Before
    fun setup() {
        gameApi = mockk()
        repository = GameRepository(gameApi)
    }

    // --- listGames ---

    @Test
    fun `listGames returns game list on success`() =
        runTest {
            coEvery { gameApi.listGames(20, 0) } returns ApiEnvelope(success = true, data = testListResponse)

            val result = repository.listGames()

            assertEquals(2, result.games.size)
            assertEquals(2, result.total)
            assertEquals("Lucky Slots", result.games[0].name)
            assertEquals("Poker Royal", result.games[1].name)
        }

    @Test
    fun `listGames forwards limit and offset`() =
        runTest {
            coEvery { gameApi.listGames(10, 5) } returns ApiEnvelope(success = true, data = testListResponse)

            repository.listGames(limit = 10, offset = 5)

            coVerify { gameApi.listGames(10, 5) }
        }

    @Test
    fun `listGames null data throws`() =
        runTest {
            coEvery { gameApi.listGames(any(), any()) } returns ApiEnvelope(success = true, data = null)

            try {
                repository.listGames()
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `listGames network error propagates`() =
        runTest {
            coEvery { gameApi.listGames(any(), any()) } throws IOException("connection refused")

            try {
                repository.listGames()
                throw AssertionError("expected exception")
            } catch (e: IOException) {
                assertEquals("connection refused", e.message)
            }
        }

    @Test
    fun `listGames http error propagates`() =
        runTest {
            coEvery { gameApi.listGames(any(), any()) } throws HttpException(
                Response.error<Any>(500, """{"success":false,"error":"internal error"}""".toResponseBody("application/json".toMediaType()))
            )

            try {
                repository.listGames()
                throw AssertionError("expected exception")
            } catch (e: HttpException) {
                assertEquals(500, e.code())
            }
        }

    // --- getGame ---

    @Test
    fun `getGame returns game on success`() =
        runTest {
            coEvery { gameApi.getGame("game-1") } returns ApiEnvelope(success = true, data = testGame)

            val result = repository.getGame("game-1")

            assertEquals("game-1", result.id)
            assertEquals("Lucky Slots", result.name)
            assertEquals("slots", result.category)
            assertEquals(95, result.rtp)
        }

    @Test
    fun `getGame null data throws`() =
        runTest {
            coEvery { gameApi.getGame(any()) } returns ApiEnvelope(success = true, data = null)

            try {
                repository.getGame("game-1")
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `getGame network error propagates`() =
        runTest {
            coEvery { gameApi.getGame(any()) } throws IOException("timeout")

            try {
                repository.getGame("game-1")
                throw AssertionError("expected exception")
            } catch (e: IOException) {
                assertEquals("timeout", e.message)
            }
        }

    // --- startGame ---

    @Test
    fun `startGame returns session info on success`() =
        runTest {
            coEvery { gameApi.startGame(any()) } returns ApiEnvelope(success = true, data = testStartResponse)

            val result = repository.startGame("game-1")

            assertEquals("session-1", result.sessionId)
            assertNotNull(result.serverSeedHash)
            assertEquals(64, result.serverSeedHash.length)
        }

    @Test
    fun `startGame forwards correct request body`() =
        runTest {
            coEvery { gameApi.startGame(any()) } returns ApiEnvelope(success = true, data = testStartResponse)

            repository.startGame("game-1")

            coVerify { gameApi.startGame(StartGameRequest("game-1")) }
        }

    @Test
    fun `startGame null data throws`() =
        runTest {
            coEvery { gameApi.startGame(any()) } returns ApiEnvelope(success = true, data = null)

            try {
                repository.startGame("game-1")
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    // --- placeBet ---

    @Test
    fun `placeBet returns bet result on success`() =
        runTest {
            coEvery { gameApi.placeBet("game-1", any()) } returns ApiEnvelope(success = true, data = testBetResponse)

            val result = repository.placeBet("game-1", 500L)

            assertEquals("bet-1", result.betId)
            assertEquals(500L, result.amount)
            assertEquals(750L, result.payout)
            assertEquals(1.5, result.multiplier, 0.001)
            assertTrue(result.won)
        }

    @Test
    fun `placeBet forwards correct request body`() =
        runTest {
            coEvery { gameApi.placeBet("game-1", any()) } returns ApiEnvelope(success = true, data = testBetResponse)

            repository.placeBet("game-1", 500L)

            coVerify { gameApi.placeBet("game-1", PlaceBetRequest(500L)) }
        }

    @Test
    fun `placeBet null data throws`() =
        runTest {
            coEvery { gameApi.placeBet(any(), any()) } returns ApiEnvelope(success = true, data = null)

            try {
                repository.placeBet("game-1", 500L)
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `placeBet http error propagates`() =
        runTest {
            coEvery { gameApi.placeBet(any(), any()) } throws HttpException(
                Response.error<Any>(400, """{"success":false,"error":"insufficient balance"}""".toResponseBody("application/json".toMediaType()))
            )

            try {
                repository.placeBet("game-1", 500L)
                throw AssertionError("expected exception")
            } catch (e: HttpException) {
                assertEquals(400, e.code())
            }
        }

    // --- endSession ---

    @Test
    fun `endSession returns completion response on success`() =
        runTest {
            coEvery { gameApi.endSession("game-1") } returns ApiEnvelope(success = true, data = testEndResponse)

            val result = repository.endSession("game-1")

            assertEquals("session-1", result.sessionId)
            assertEquals("completed", result.status)
        }

    @Test
    fun `endSession null data throws`() =
        runTest {
            coEvery { gameApi.endSession(any()) } returns ApiEnvelope(success = true, data = null)

            try {
                repository.endSession("game-1")
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `endSession network error propagates`() =
        runTest {
            coEvery { gameApi.endSession(any()) } throws IOException("connection reset")

            try {
                repository.endSession("game-1")
                throw AssertionError("expected exception")
            } catch (e: IOException) {
                assertEquals("connection reset", e.message)
            }
        }
}
