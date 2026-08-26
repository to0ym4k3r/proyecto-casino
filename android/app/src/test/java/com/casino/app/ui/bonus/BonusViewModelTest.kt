package com.casino.app.ui.bonus

import com.casino.app.MainDispatcherRule
import com.casino.app.data.api.dto.ActiveBonusDto
import com.casino.app.data.api.dto.BonusClaimResponse
import com.casino.app.data.api.dto.BonusDefinitionDto
import com.casino.app.data.bonus.BonusRepository
import com.casino.app.ui.util.formatCentsAsDollars
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

/**
 * BonusViewModel unit tests (task 6.10).
 *
 * Spec scenarios covered:
 * - Load available bonuses on init → list renders
 * - Load available bonuses error → error state
 * - Claim bonus success → refreshes lists
 * - Claim bonus error → error state
 * - Load active bonuses on init → list renders
 * - Load active bonuses error → error state
 * - Forfeit bonus success → refreshes active list
 * - Forfeit bonus error → error state
 * - Format cents as dollars helper
 */
@OptIn(ExperimentalCoroutinesApi::class)
class BonusViewModelTest {

    @get:Rule
    val mainDispatcherRule = MainDispatcherRule()

    private lateinit var repository: BonusRepository
    private lateinit var viewModel: BonusViewModel

    private val testDefinition = BonusDefinitionDto(
        id = "def-1",
        code = "welcome",
        name = "Welcome Bonus",
        description = "100% match on first deposit",
        wageringMultiplier = 30,
        expiryDays = 30,
        maxBonusAmountCents = 50000L,
        minDepositCents = 0L,
        isActive = true,
        createdAt = "2026-08-01T00:00:00Z",
        updatedAt = "2026-08-01T00:00:00Z",
    )

    private val testDefinition2 = BonusDefinitionDto(
        id = "def-2",
        code = "reload",
        name = "Reload Bonus",
        description = "50% match on deposits over $50",
        wageringMultiplier = 25,
        expiryDays = 14,
        maxBonusAmountCents = 25000L,
        minDepositCents = 5000L,
        isActive = true,
        createdAt = "2026-08-01T00:00:00Z",
        updatedAt = "2026-08-01T00:00:00Z",
    )

    private val testClaimResponse = BonusClaimResponse(
        grantId = "grant-1",
        bonusAmountCents = 25000L,
        wageringRequiredCents = 750000L,
        expiresAt = "2026-09-01T00:00:00Z",
    )

    private val testActiveBonus = ActiveBonusDto(
        grantId = "grant-1",
        code = "welcome",
        name = "Welcome Bonus",
        bonusAmountCents = 25000L,
        wageringRequiredCents = 750000L,
        totalWageredCents = 150000L,
        wageringProgressPct = 20.0,
        expiresAt = "2026-09-01T00:00:00Z",
    )

    @Before
    fun setup() {
        repository = mockk()
    }

    private fun initViewModel() {
        viewModel = BonusViewModel(repository)
    }

    // --- Available bonuses ---

    @Test
    fun `load available bonuses on init succeeds`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns listOf(testDefinition, testDefinition2)
            coEvery { repository.getActiveBonuses() } returns emptyList()

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingAvailable)
            assertNull(state.availableError)
            assertEquals(2, state.availableBonuses.size)
            assertEquals("Welcome Bonus", state.availableBonuses[0].name)
            assertEquals("Reload Bonus", state.availableBonuses[1].name)
        }

    @Test
    fun `load available bonuses error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } throws IOException("connection reset")
            coEvery { repository.getActiveBonuses() } returns emptyList()

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingAvailable)
            assertNotNull(state.availableError)
            assertTrue(state.availableError!!.contains("Network"))
        }

    @Test
    fun `load available bonuses http error surfaces backend message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } throws HttpException(
                Response.error<Any>(500, """{"success":false,"error":"server error"}""".toResponseBody("application/json".toMediaType()))
            )
            coEvery { repository.getActiveBonuses() } returns emptyList()

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingAvailable)
            assertEquals("server error", state.availableError)
        }

    // --- Claim bonus ---

    @Test
    fun `claim bonus success refreshes lists`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns listOf(testDefinition)
            coEvery { repository.getActiveBonuses() } returns emptyList()
            coEvery { repository.claimBonus("def-1", null) } returns testClaimResponse

            initViewModel()
            advanceUntilIdle()

            viewModel.claimBonus("def-1")
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isClaiming)
            assertNull(state.claimError)
            assertEquals("grant-1", state.lastClaimedGrantId)
            // Verify both lists were refreshed
            coVerify(atLeast = 2) { repository.getAvailableBonuses() }
            coVerify(atLeast = 2) { repository.getActiveBonuses() }
        }

    @Test
    fun `claim bonus error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns listOf(testDefinition)
            coEvery { repository.getActiveBonuses() } returns emptyList()
            coEvery { repository.claimBonus(any(), any()) } throws IOException("timeout")

            initViewModel()
            advanceUntilIdle()

            viewModel.claimBonus("def-1")
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isClaiming)
            assertNotNull(state.claimError)
        }

    @Test
    fun `claim bonus conflict error surfaces backend message`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns listOf(testDefinition)
            coEvery { repository.getActiveBonuses() } returns emptyList()
            coEvery { repository.claimBonus(any(), any()) } throws HttpException(
                Response.error<Any>(409, """{"success":false,"error":"active bonus of this type already exists"}""".toResponseBody("application/json".toMediaType()))
            )

            initViewModel()
            advanceUntilIdle()

            viewModel.claimBonus("def-1")
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isClaiming)
            assertEquals("active bonus of this type already exists", state.claimError)
        }

    // --- Active bonuses ---

    @Test
    fun `load active bonuses on init succeeds`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns emptyList()
            coEvery { repository.getActiveBonuses() } returns listOf(testActiveBonus)

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingActive)
            assertNull(state.activeError)
            assertEquals(1, state.activeBonuses.size)
            assertEquals("grant-1", state.activeBonuses[0].grantId)
            assertEquals(20.0, state.activeBonuses[0].wageringProgressPct, 0.01)
        }

    @Test
    fun `load active bonuses error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns emptyList()
            coEvery { repository.getActiveBonuses() } throws IOException("connection refused")

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingActive)
            assertNotNull(state.activeError)
        }

    @Test
    fun `load active bonuses empty list shows empty state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns emptyList()
            coEvery { repository.getActiveBonuses() } returns emptyList()

            initViewModel()
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isLoadingActive)
            assertEquals(0, state.activeBonuses.size)
        }

    // --- Forfeit bonus ---

    @Test
    fun `forfeit bonus success refreshes active list`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns emptyList()
            coEvery { repository.getActiveBonuses() } returns listOf(testActiveBonus)
            coEvery { repository.forfeitBonus("grant-1") } returns Unit

            initViewModel()
            advanceUntilIdle()

            viewModel.forfeitBonus("grant-1")
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isForfeiting)
            assertNull(state.forfeitError)
            assertEquals("grant-1", state.forfeitedGrantId)
            // Verify active list was refreshed
            coVerify(atLeast = 2) { repository.getActiveBonuses() }
        }

    @Test
    fun `forfeit bonus error sets error state`() =
        runTest(mainDispatcherRule.testDispatcher) {
            coEvery { repository.getAvailableBonuses() } returns emptyList()
            coEvery { repository.getActiveBonuses() } returns listOf(testActiveBonus)
            coEvery { repository.forfeitBonus(any()) } throws IOException("connection reset")

            initViewModel()
            advanceUntilIdle()

            viewModel.forfeitBonus("grant-1")
            advanceUntilIdle()

            val state = viewModel.uiState.value
            assertFalse(state.isForfeiting)
            assertNotNull(state.forfeitError)
        }

    // --- Format helper ---

    @Test
    fun `formatCentsAsDollars formats correctly`() {
        assertEquals("$0.00", formatCentsAsDollars(0L))
        assertEquals("$0.01", formatCentsAsDollars(1L))
        assertEquals("$1.00", formatCentsAsDollars(100L))
        assertEquals("$150.00", formatCentsAsDollars(15000L))
        assertEquals("$500.00", formatCentsAsDollars(50000L))
        assertEquals("$123.45", formatCentsAsDollars(12345L))
    }
}
