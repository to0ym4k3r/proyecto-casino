package com.casino.app.ui.screens.game

import androidx.lifecycle.SavedStateHandle
import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.BetResultResponse
import com.casino.app.data.api.dto.GameDto
import com.casino.app.data.api.dto.StartGameResponse
import com.casino.app.data.game.GameRepository
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

@OptIn(ExperimentalCoroutinesApi::class)
class GameViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var repository: GameRepository
    private lateinit var savedStateHandle: SavedStateHandle

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

    private val testStartSession = StartGameResponse(
        sessionId = "session-1",
        serverSeedHash = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
    )

    private val testBetWin = BetResultResponse(
        betId = "bet-1",
        amount = 500L,
        payout = 750L,
        multiplier = 1.5,
        won = true,
    )

    private val testBetLoss = BetResultResponse(
        betId = "bet-2",
        amount = 500L,
        payout = 0L,
        multiplier = 0.5,
        won = false,
    )

    @Before
    fun setup() {
        repository = mockk()
        savedStateHandle = SavedStateHandle(mapOf("gameId" to "game-1"))
    }

    private fun initViewModel(): GameViewModel {
        return GameViewModel(repository, savedStateHandle)
    }

    // --- loadGame tests ---

    @Test
    fun `loadGame success sets game state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession

            val viewModel = initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoading)
            assertNull(state.error)
            assertNotNull(state.game)
            assertEquals("Lucky Slots", state.game?.name)
            assertEquals("slots", state.game?.category)
        }

    @Test
    fun `loadGame error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } throws IOException("connection refused")
            coEvery { repository.startGame("game-1") } returns testStartSession

            val viewModel = initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoading)
            assertNotNull(state.error)
            assertTrue(state.error!!.contains("connection refused"))
        }

    @Test
    fun `loadGame http error surfaces backend message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } throws HttpException(
                Response.error<Any>(404, """{"success":false,"error":"game not found"}""".toResponseBody("application/json".toMediaType()))
            )
            coEvery { repository.startGame("game-1") } returns testStartSession

            val viewModel = initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoading)
            assertNotNull(state.error)
            assertEquals("game not found", state.error)
        }

    // --- startSession tests ---

    @Test
    fun `startSession success sets session info`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession

            val viewModel = initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertEquals("session-1", state.sessionId)
            assertNotNull(state.serverSeedHash)
        }

    @Test
    fun `startSession error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } throws IOException("server unavailable")

            val viewModel = initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertNull(state.sessionId)
            assertNotNull(state.error)
        }

    // --- placeBet tests ---

    @Test
    fun `placeBet success updates lastBet`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.placeBet("game-1", 500L) } returns testBetWin

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.placeBet(500L)
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isPlacingBet)
            assertNull(state.betError)
            assertNotNull(state.lastBet)
            assertTrue(state.lastBet!!.won)
            assertEquals(750L, state.lastBet!!.payout)
            assertEquals(1.5, state.lastBet!!.multiplier, 0.001)
        }

    @Test
    fun `placeBet loss updates lastBet with zero payout`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.placeBet("game-1", 500L) } returns testBetLoss

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.placeBet(500L)
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isPlacingBet)
            assertNotNull(state.lastBet)
            assertFalse(state.lastBet!!.won)
            assertEquals(0L, state.lastBet!!.payout)
        }

    @Test
    fun `placeBet error sets betError`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.placeBet("game-1", 500L) } throws IOException("network error")

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.placeBet(500L)
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isPlacingBet)
            assertNull(state.lastBet)
            assertNotNull(state.betError)
            assertTrue(state.betError!!.contains("network error"))
        }

    @Test
    fun `placeBet http error surfaces backend message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.placeBet("game-1", 500L) } throws HttpException(
                Response.error<Any>(400, """{"success":false,"error":"insufficient balance"}""".toResponseBody("application/json".toMediaType()))
            )

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.placeBet(500L)
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isPlacingBet)
            assertEquals("insufficient balance", state.betError)
        }

    @Test
    fun `placeBet guards against double tap`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.placeBet("game-1", 500L) } coAnswers {
                kotlinx.coroutines.delay(1000)
                testBetWin
            }

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.placeBet(500L)
            viewModel.placeBet(500L) // second tap should be ignored
            advanceUntilIdle()

            coVerify(exactly = 1) { repository.placeBet("game-1", 500L) }
        }

    // --- endSession tests ---

    @Test
    fun `endSession success clears session state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.endSession("game-1") } returns mockk()

            val viewModel = initViewModel()
            advanceUntilIdle()

            assertNotNull(viewModel.uiState.value.sessionId)

            viewModel.endSession()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertNull(state.sessionId)
            assertNull(state.serverSeedHash)
            assertNull(state.lastBet)
        }

    @Test
    fun `endSession error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession
            coEvery { repository.endSession("game-1") } throws IOException("server down")

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.endSession()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertNotNull(state.error)
        }

    // --- clearBetResult tests ---

    @Test
    fun `clearBetResult clears lastBet and betError`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getGame("game-1") } returns testGame
            coEvery { repository.startGame("game-1") } returns testStartSession

            val viewModel = initViewModel()
            advanceUntilIdle()

            viewModel.clearBetResult()

            val state = viewModel.uiState.value
            assertNull(state.lastBet)
            assertNull(state.betError)
        }

    // --- init requires gameId ---

    @Test(expected = IllegalArgumentException::class)
    fun `init without gameId throws`() {
        val emptyHandle = SavedStateHandle()
        GameViewModel(repository, emptyHandle)
    }
}
