package com.casino.app.data.bonus

import com.casino.app.data.api.BonusApi
import com.casino.app.data.api.dto.ActiveBonusDto
import com.casino.app.data.api.dto.ApiEnvelope
import com.casino.app.data.api.dto.BonusClaimResponse
import com.casino.app.data.api.dto.BonusDefinitionDto
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.mockk
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.ResponseBody.Companion.toResponseBody
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.HttpException
import retrofit2.Response

/**
 * BonusRepository unit tests (task 6.10).
 *
 * Tests cover:
 * - getAvailableBonuses: success, null data, network error, http error
 * - claimBonus: success with forwarding, null data, http error
 * - getActiveBonuses: success, null data, network error
 * - forfeitBonus: success, network error
 */
@OptIn(ExperimentalCoroutinesApi::class)
class BonusRepositoryTest {

    private lateinit var bonusApi: BonusApi
    private lateinit var repository: BonusRepository

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
        bonusApi = mockk()
        repository = BonusRepository(bonusApi)
    }

    // --- getAvailableBonuses ---

    @Test
    fun `getAvailableBonuses returns list on success`() =
        runTest {
            coEvery { bonusApi.getAvailableBonuses() } returns ApiEnvelope(
                success = true,
                data = listOf(testDefinition),
            )

            val result = repository.getAvailableBonuses()

            assertEquals(1, result.size)
            assertEquals("def-1", result[0].id)
            assertEquals("welcome", result[0].code)
            assertEquals("Welcome Bonus", result[0].name)
        }

    @Test
    fun `getAvailableBonuses returns empty list`() =
        runTest {
            coEvery { bonusApi.getAvailableBonuses() } returns ApiEnvelope(
                success = true,
                data = emptyList(),
            )

            val result = repository.getAvailableBonuses()

            assertEquals(0, result.size)
        }

    @Test
    fun `getAvailableBonuses null data throws`() =
        runTest {
            coEvery { bonusApi.getAvailableBonuses() } returns ApiEnvelope(success = true, data = null)

            try {
                repository.getAvailableBonuses()
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `getAvailableBonuses network error propagates`() =
        runTest {
            coEvery { bonusApi.getAvailableBonuses() } throws IOException("connection refused")

            try {
                repository.getAvailableBonuses()
                throw AssertionError("expected exception")
            } catch (e: IOException) {
                assertEquals("connection refused", e.message)
            }
        }

    @Test
    fun `getAvailableBonuses http error propagates`() =
        runTest {
            coEvery { bonusApi.getAvailableBonuses() } throws HttpException(
                Response.error<Any>(500, """{"success":false,"error":"internal error"}""".toResponseBody("application/json".toMediaType()))
            )

            try {
                repository.getAvailableBonuses()
                throw AssertionError("expected exception")
            } catch (e: HttpException) {
                assertEquals(500, e.code())
            }
        }

    // --- claimBonus ---

    @Test
    fun `claimBonus returns claim response on success`() =
        runTest {
            coEvery { bonusApi.claimBonus(any()) } returns ApiEnvelope(
                success = true,
                data = testClaimResponse,
            )

            val result = repository.claimBonus("def-1")

            assertEquals("grant-1", result.grantId)
            assertEquals(25000L, result.bonusAmountCents)
            assertEquals(750000L, result.wageringRequiredCents)
        }

    @Test
    fun `claimBonus forwards definition ID`() =
        runTest {
            coEvery { bonusApi.claimBonus(any()) } returns ApiEnvelope(
                success = true,
                data = testClaimResponse,
            )

            repository.claimBonus("def-1")

            coVerify {
                bonusApi.claimBonus(with { it.definitionId == "def-1" })
            }
        }

    @Test
    fun `claimBonus null data throws`() =
        runTest {
            coEvery { bonusApi.claimBonus(any()) } returns ApiEnvelope(success = true, data = null)

            try {
                repository.claimBonus("def-1")
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `claimBonus http error propagates`() =
        runTest {
            coEvery { bonusApi.claimBonus(any()) } throws HttpException(
                Response.error<Any>(409, """{"success":false,"error":"active bonus of this type already exists"}""".toResponseBody("application/json".toMediaType()))
            )

            try {
                repository.claimBonus("def-1")
                throw AssertionError("expected exception")
            } catch (e: HttpException) {
                assertEquals(409, e.code())
            }
        }

    // --- getActiveBonuses ---

    @Test
    fun `getActiveBonuses returns list on success`() =
        runTest {
            coEvery { bonusApi.getActiveBonuses() } returns ApiEnvelope(
                success = true,
                data = listOf(testActiveBonus),
            )

            val result = repository.getActiveBonuses()

            assertEquals(1, result.size)
            assertEquals("grant-1", result[0].grantId)
            assertEquals(20.0, result[0].wageringProgressPct, 0.01)
        }

    @Test
    fun `getActiveBonuses null data throws`() =
        runTest {
            coEvery { bonusApi.getActiveBonuses() } returns ApiEnvelope(success = true, data = null)

            try {
                repository.getActiveBonuses()
                throw AssertionError("expected error")
            } catch (e: IllegalStateException) {
                assertTrue(e.message!!.contains("succeeded without data"))
            }
        }

    @Test
    fun `getActiveBonuses network error propagates`() =
        runTest {
            coEvery { bonusApi.getActiveBonuses() } throws IOException("timeout")

            try {
                repository.getActiveBonuses()
                throw AssertionError("expected exception")
            } catch (e: IOException) {
                assertEquals("timeout", e.message)
            }
        }

    // --- forfeitBonus ---

    @Test
    fun `forfeitBonus calls API with correct grant ID`() =
        runTest {
            coEvery { bonusApi.forfeitBonus(any()) } returns ApiEnvelope(success = true, data = Unit)

            repository.forfeitBonus("grant-1")

            coVerify {
                bonusApi.forfeitBonus("grant-1")
            }
        }

    @Test
    fun `forfeitBonus network error propagates`() =
        runTest {
            coEvery { bonusApi.forfeitBonus(any()) } throws IOException("connection reset")

            try {
                repository.forfeitBonus("grant-1")
                throw AssertionError("expected exception")
            } catch (e: IOException) {
                assertEquals("connection reset", e.message)
            }
        }
}
